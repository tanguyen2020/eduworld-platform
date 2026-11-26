package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the translation jobs (counts only, never a text — plan "Bảo mật").
const (
	ActionTranslationExported = "game_translation.exported"
	ActionTranslationImported = "game_translation.imported"
	ActionTranslationRequest  = "game_translation.requested"
)

// Translation job limits (TranslationExportRequest maxItems, Clarifications 2026-11-26).
const (
	TranslationMaxTypes    = 20
	TranslationMaxIDs      = 500
	TranslationJobLease    = 5 * time.Minute
	TranslationMaxAttempts = 3
	translationPage        = 200
)

// TranslationMetrics observes eduworld_game_translation_jobs_total{kind,status} and
// eduworld_game_translation_rows_total{result}; nil = not observed.
type TranslationMetrics interface {
	Job(kind, status string)
	Rows(result string, n int)
}

// Translations are the use cases of the translation files of game content (F082 US4): exportGameTranslations and
// importGameTranslations create a job (202), the worker runs it (RunNext). An import writes drafts through the draft
// core of the console (GameAdmin: type checks, translation state, audit) and never changes a published version (R11).
type Translations struct {
	UoW     write.UnitOfWork
	Store   write.TranslationStore
	Configs write.GameConfigStore
	Media   write.GameMediaStore
	Admin   GameAdmin
	Storage port.ObjectStorage
	Codecs  map[string]port.TranslationCodec
	Audit   write.Auditor
	Metrics TranslationMetrics
	Now     func() time.Time
}

func (h Translations) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// ExportInput is TranslationExportRequest.
type ExportInput struct {
	ContentTypes   []string
	ContentIDs     []uuid.UUID
	SourceLanguage string
	TargetLanguage string
	Format         string
	OnlyMissing    bool
}

// ImportInput is TranslationImportRequest.
type ImportInput struct {
	AssetID        uuid.UUID
	Format         string
	TargetLanguage string
	DryRun         bool
}

// JobView is TranslationJob of the contract (gameadmin.TranslationJobView).
type JobView = gameadmin.TranslationJobView

// BucketExports and TranslationKeyPrefix locate the generated files (gameadmin).
const (
	BucketExports        = gameadmin.BucketExports
	TranslationKeyPrefix = gameadmin.TranslationKeyPrefix
)

// JobViewOf is the view of a job row (without the download link).
func JobViewOf(j write.TranslationJob) JobView {
	v := JobView{JobID: j.ID, Kind: j.Kind, Status: j.Status, Format: j.Format, TargetLanguage: j.TargetLanguage, DryRun: j.DryRun,
		CreatedAt: j.CreatedAt.UTC(), FinishedAt: j.FinishedAt, ErrorMessage: j.ErrorMessage, Report: j.Report}
	if v.Report != nil && v.Report.Errors == nil {
		v.Report.Errors = []i18ntext.ReportError{}
	}
	return v
}

// translator authorizes a translation request: platform staff with game.content.translate (F082-AC-16; authz.denied on
// refusal). Game config data has no tenant (F082-AC-17).
func translator(ctx context.Context) (authz.Principal, tenancy.Scope, error) {
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return p, nil, err
	}
	if err := authz.Require(ctx, p, authz.PermGameContentTranslate, authz.Resource{}); err != nil {
		return p, nil, err
	}
	return p, sc, nil
}

func checkLanguages(source, target *string) error {
	if *source == "" {
		*source = i18ntext.SourceLanguage
	}
	valid := func(l string) bool { return l == "vi" || l == "en" }
	switch {
	case !valid(*target):
		return problem.Invalid("target_language", problem.SubInvalid, "vi or en")
	case !valid(*source):
		return problem.Invalid("source_language", problem.SubInvalid, "vi or en")
	case *source == *target:
		return problem.Invalid("target_language", problem.SubInvalid, "the target differs from the source")
	}
	return nil
}

func (h Translations) codec(format string) (port.TranslationCodec, error) {
	c, ok := h.Codecs[format]
	if !ok {
		return nil, problem.Invalid("format", problem.SubInvalid, "xliff or csv")
	}
	return c, nil
}

func jobIdempotencyKey(ctx context.Context) *string {
	if k := idempotencyKey(ctx); k != "" {
		return &k
	}
	return nil
}

// enqueue stores a new job (or returns the job of the same Idempotency-Key, F082-AC-14) and audits the request.
func (h Translations) enqueue(ctx context.Context, sc tenancy.Scope, p authz.Principal, j write.TranslationJob) (out JobView, err error) {
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		if j.IdempotencyKey != nil {
			prev, err := h.Store.JobByIdempotency(ctx, p.UserID, *j.IdempotencyKey)
			if err != nil {
				return err
			}
			if prev != nil {
				out = JobViewOf(*prev)
				return idempotency.SetResult(ctx, out)
			}
		}
		if err := h.Store.InsertJob(ctx, j); err != nil {
			if errors.Is(err, write.ErrConflict) {
				return problem.New(problem.Conflict, "the same request is being registered").Retry()
			}
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionTranslationRequest, TargetType: "translation_job", TargetID: j.ID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"kind": j.Kind, "format": j.Format, "target_language": j.TargetLanguage,
				"content_types": len(j.ContentTypes), "content_ids": len(j.ContentIDs), "dry_run": j.DryRun}}); err != nil {
			return err
		}
		out = JobViewOf(j)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.Job(j.Kind, "queued")
	}
	return out, err
}

// Export is exportGameTranslations (F082-AC-12, AC-19): content_types must be config types of module game (≤ 20),
// content_ids ≤ 500; the job is queued (202).
func (h Translations) Export(ctx context.Context, in ExportInput) (out JobView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ExportGameTranslations")
	defer func() { done(err) }()
	p, sc, err := translator(ctx)
	if err != nil {
		return out, err
	}
	if len(in.ContentTypes) > TranslationMaxTypes {
		return out, problem.Invalid("content_types", problem.SubMaxLength, "at most 20 config types")
	}
	if len(in.ContentIDs) > TranslationMaxIDs {
		return out, problem.Invalid("content_ids", problem.SubMaxLength, "at most 500 items")
	}
	for i, ct := range in.ContentTypes {
		if _, err := gameconfig.Lookup(ct); err != nil {
			return out, problem.Invalid(fmt.Sprintf("content_types/%d", i), problem.SubInvalid, "a config type of module game")
		}
	}
	if err := checkLanguages(&in.SourceLanguage, &in.TargetLanguage); err != nil {
		return out, err
	}
	if _, err := h.codec(in.Format); err != nil {
		return out, err
	}
	j := write.TranslationJob{ID: ids.New(), Kind: "export", Status: "queued", Format: in.Format, SourceLanguage: in.SourceLanguage,
		TargetLanguage: in.TargetLanguage, OnlyMissing: in.OnlyMissing, ContentTypes: in.ContentTypes, ContentIDs: in.ContentIDs,
		RequestedBy: p.UserID, IdempotencyKey: jobIdempotencyKey(ctx), CreatedAt: h.now()}
	return h.enqueue(ctx, sc, p, j)
}

// Import is importGameTranslations (F082-AC-13): the file is a ready game media asset of kind document (uploaded with
// presignGameMediaUpload); the job is queued (202), dry_run only reports.
func (h Translations) Import(ctx context.Context, in ImportInput) (out JobView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ImportGameTranslations")
	defer func() { done(err) }()
	p, sc, err := translator(ctx)
	if err != nil {
		return out, err
	}
	source := ""
	if err := checkLanguages(&source, &in.TargetLanguage); err != nil {
		return out, err
	}
	if _, err := h.codec(in.Format); err != nil {
		return out, err
	}
	if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		a, err := h.Media.Asset(ctx, in.AssetID)
		if err != nil {
			return err
		}
		switch {
		case a == nil || a.Kind != string(gameconfig.MediaDocument):
			return problem.Invalid("asset_id", problem.SubInvalid, "a translation file uploaded as game media of kind document")
		case a.Status != string(gameconfig.MediaReady):
			return problem.New(problem.MediaNotReady, "the file is "+a.Status)
		}
		return nil
	}); err != nil {
		return out, err
	}
	asset := in.AssetID
	j := write.TranslationJob{ID: ids.New(), Kind: "import", Status: "queued", Format: in.Format, SourceLanguage: source,
		TargetLanguage: in.TargetLanguage, DryRun: in.DryRun, InputAssetID: &asset, RequestedBy: p.UserID, IdempotencyKey: jobIdempotencyKey(ctx),
		CreatedAt: h.now()}
	return h.enqueue(ctx, sc, p, j)
}

// RunNext is one tick of the job translation-jobs (worker profile jobs): it claims one job (SKIP LOCKED; a running job
// whose lease expired is claimed again, at most TranslationMaxAttempts times) and runs it. Returns 1 when a job ran.
func (h Translations) RunNext(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RunTranslationJob")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("translation-jobs")
	if err != nil {
		return 0, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	now := h.now()
	var job *write.TranslationJob
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		j, err := h.Store.ClaimJob(ctx, now, TranslationJobLease)
		if err != nil || j == nil {
			return err
		}
		j.Attempts++
		if j.Attempts > TranslationMaxAttempts {
			msg := "the job failed after its last attempt"
			j.Status, j.ErrorMessage, j.FinishedAt = "failed", &msg, &now
		} else {
			j.Status, j.StartedAt = "running", &now
		}
		if err := h.Store.SaveJob(ctx, *j, now); err != nil {
			return err
		}
		job = j
		return nil
	}); err != nil || job == nil {
		return 0, err
	}
	if job.Status == "failed" {
		h.finished(ctx, *job)
		return 1, nil
	}
	var runErr error
	if job.Kind == "export" {
		runErr = h.runExport(ctx, job)
	} else {
		runErr = h.runImport(ctx, job)
	}
	switch {
	case errors.Is(runErr, port.ErrMalformedFile):
		msg := truncate(runErr.Error(), 1000)
		job.Status, job.ErrorMessage = "failed", &msg
	case runErr != nil:
		// infrastructure failure: the job stays running and is claimed again when its lease expires.
		slog.WarnContext(ctx, "translation job: attempt failed", "job_id", job.ID, "attempt", job.Attempts, "error", runErr)
		return 1, nil
	default:
		job.Status = "succeeded"
	}
	end := h.now()
	job.FinishedAt = &end
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.SaveJob(ctx, *job, end); err != nil {
			return err
		}
		action := ActionTranslationExported
		if job.Kind == "import" {
			action = ActionTranslationImported
		}
		meta := map[string]any{"status": job.Status, "requested_by": job.RequestedBy.String(), "dry_run": job.DryRun}
		if job.Report != nil {
			meta["total"], meta["updated"], meta["skipped"] = job.Report.Total, job.Report.Updated, job.Report.Skipped
		}
		return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "translation_job", TargetID: job.ID.String(), Result: audit.ResultSuccess,
			Metadata: meta})
	}); err != nil {
		return 1, err
	}
	h.finished(ctx, *job)
	return 1, nil
}

func (h Translations) finished(ctx context.Context, j write.TranslationJob) {
	if h.Metrics == nil {
		return
	}
	h.Metrics.Job(j.Kind, j.Status)
	if j.Report != nil {
		h.Metrics.Rows("updated", j.Report.Updated)
		h.Metrics.Rows("skipped", j.Report.Skipped)
	}
	_ = ctx
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// itemTypesOf maps the config types of a job to item types (all module game types with texts when empty).
func itemTypesOf(configTypes []string) ([]string, map[string]gameconfig.Type) {
	byItem := map[string]gameconfig.Type{}
	var out []string
	if len(configTypes) == 0 {
		for _, t := range gameconfig.Types() {
			out = append(out, t.ItemType)
			byItem[t.ItemType] = t
		}
		return out, byItem
	}
	for _, ct := range configTypes {
		if t, err := gameconfig.Lookup(ct); err == nil {
			out = append(out, t.ItemType)
			byItem[t.ItemType] = t
		}
	}
	return out, byItem
}

// runExport writes the texts of the newest version of each item (only the missing/outdated ones when OnlyMissing) to a
// file in ew-exports (F082-AC-12); content ids that are not platform config items of module game are reported as not
// found (F082-AC-17).
func (h Translations) runExport(ctx context.Context, j *write.TranslationJob) error {
	codec, err := h.codec(j.Format)
	if err != nil {
		return err
	}
	if h.Storage == nil {
		return errors.New("object storage is not configured")
	}
	itemTypes, byItem := itemTypesOf(j.ContentTypes)
	file := i18ntext.File{SourceLanguage: j.SourceLanguage, TargetLanguage: j.TargetLanguage}
	report := i18ntext.Report{Errors: []i18ntext.ReportError{}}
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	if len(j.ContentIDs) > 0 {
		ids = j.ContentIDs
	}
	var after *uuid.UUID
	for {
		var page []write.LatestVersion
		if err := h.UoW.Do(ctx, func(ctx context.Context) error {
			var err error
			page, err = h.Store.LatestVersions(ctx, itemTypes, ids, after, translationPage)
			return err
		}); err != nil {
			return err
		}
		for _, lv := range page {
			seen[lv.Item.ConfigID] = true
			t := byItem[lv.Item.ItemType]
			var doc map[string]any
			if err := json.Unmarshal(lv.Version.Payload, &doc); err != nil {
				return fmt.Errorf("payload: %w", err)
			}
			missing, outdated := map[string]bool{}, map[string]bool{}
			if lv.State != nil {
				for _, m := range lv.State.State.Missing {
					missing[m] = true
				}
				for _, o := range lv.State.State.Outdated {
					outdated[o.Pointer] = true
				}
			}
			for _, tx := range i18ntext.Texts(doc) {
				note := ""
				switch {
				case missing[tx.Pointer+"/"+j.TargetLanguage] || (lv.State == nil && tx.Values[j.TargetLanguage] == ""):
					note = "missing"
				case outdated[tx.Pointer]:
					note = "outdated"
				}
				if j.OnlyMissing && note == "" {
					continue
				}
				k := i18ntext.Key{ConfigType: t.Name, ConfigID: lv.Item.ConfigID.String(), Version: lv.Version.VersionNo, Pointer: tx.Pointer}
				file.Units = append(file.Units, i18ntext.Unit{Key: k.String(), Source: tx.Values[j.SourceLanguage], Target: tx.Values[j.TargetLanguage],
					Note: note})
			}
		}
		if len(page) < translationPage {
			break
		}
		last := page[len(page)-1].Item.ConfigID
		after = &last
	}
	for _, id := range j.ContentIDs {
		if !seen[id] {
			report.Skip(id.String(), i18ntext.CodeKeyNotFound, "no game config item with this id")
		}
	}
	report.Total = len(file.Units)
	var buf bytes.Buffer
	if err := codec.Encode(&buf, file); err != nil {
		return err
	}
	key := TranslationKeyPrefix + j.ID.String() + "." + codec.Extension()
	if err := h.Storage.Put(ctx, BucketExports, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()), codec.ContentType(),
		"private, no-store"); err != nil {
		return err
	}
	j.ResultKey, j.Report = &key, &report
	return nil
}

// itemKey groups the lines of an import by item.
type itemKey struct {
	configType string
	configID   uuid.UUID
}

// importLine is one valid line of an import file.
type importLine struct {
	ref  string
	key  i18ntext.Key
	text string
}

// runImport reads the file, checks each line (F082-AC-13, AC-14) and writes the target texts of each item in its own
// transaction: into the newest draft, or a new draft based on the newest approved/published/archived version; a version
// under review, a stale version, an unknown key, a type outside module game or a too long text skips the line.
func (h Translations) runImport(ctx context.Context, j *write.TranslationJob) error {
	codec, err := h.codec(j.Format)
	if err != nil {
		return err
	}
	if h.Storage == nil || j.InputAssetID == nil {
		return errors.New("object storage is not configured")
	}
	var asset *write.MediaAsset
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		asset, err = h.Media.Asset(ctx, *j.InputAssetID)
		return err
	}); err != nil {
		return err
	}
	if asset == nil || asset.QuarantineKey == nil || asset.Kind != string(gameconfig.MediaDocument) || asset.Status != string(gameconfig.MediaReady) {
		return fmt.Errorf("%w: the uploaded file is not available", port.ErrMalformedFile)
	}
	rc, _, err := h.Storage.Open(ctx, pkgmedia.BucketQuarantine, *asset.QuarantineKey)
	if err != nil {
		return err
	}
	file, err := codec.Decode(rc)
	_ = rc.Close()
	if err != nil {
		return err
	}
	report := i18ntext.Report{Total: len(file.Units), Errors: []i18ntext.ReportError{}}
	if file.TargetLanguage != "" && file.TargetLanguage != j.TargetLanguage {
		for _, u := range file.Units {
			report.Skip(u.Key, i18ntext.CodeInvalidLanguage, "the file target language is not the job target language")
		}
		j.Report = &report
		return nil
	}
	groups := map[itemKey][]importLine{}
	var order []itemKey
	for _, u := range file.Units {
		k, err := i18ntext.ParseKey(u.Key)
		if err != nil {
			report.Skip(u.Key, i18ntext.CodeKeyNotFound, "malformed key")
			continue
		}
		if _, err := gameconfig.Lookup(k.ConfigType); err != nil {
			report.Skip(u.Key, i18ntext.CodeUnsupportedType, "not a config type of module game")
			continue
		}
		id, err := uuid.Parse(k.ConfigID)
		if err != nil {
			report.Skip(u.Key, i18ntext.CodeKeyNotFound, "unknown item")
			continue
		}
		if utf8.RuneCountInString(u.Target) > i18ntext.MaxTextLength {
			report.Skip(u.Key, i18ntext.CodeTooLong, fmt.Sprintf("the text is longer than %d characters", i18ntext.MaxTextLength))
			continue
		}
		if u.Target == "" {
			report.Skip(u.Key, "", "no target text")
			continue
		}
		g := itemKey{configType: k.ConfigType, configID: id}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], importLine{ref: u.Key, key: k, text: u.Target})
	}
	sort.SliceStable(order, func(a, b int) bool { return order[a].configID.String() < order[b].configID.String() })
	for _, g := range order {
		if err := h.importItem(ctx, j, g, groups[g], &report); err != nil {
			return err
		}
	}
	j.Report = &report
	return nil
}

// importItem applies the lines of one item in one transaction; a refusal of the draft core (type checks) skips the
// item's applied lines with the problem code, other items go on.
func (h Translations) importItem(ctx context.Context, j *write.TranslationJob, g itemKey, lines []importLine, report *i18ntext.Report) error {
	t, _ := gameconfig.Lookup(g.configType)
	now := h.now()
	var lineErrs []i18ntext.ReportError
	var applied []importLine
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		lineErrs, applied = nil, nil
		it, err := h.Configs.ItemByID(ctx, t.ItemType, g.configID)
		if err != nil {
			return err
		}
		if it == nil {
			return errSkipItem{code: i18ntext.CodeKeyNotFound, msg: "unknown item"}
		}
		latest, err := h.Configs.Version(ctx, it.ConfigID, it.LatestVersionNo)
		if err != nil {
			return err
		}
		if latest == nil {
			return errSkipItem{code: i18ntext.CodeKeyNotFound, msg: "unknown version"}
		}
		var doc map[string]any
		if err := json.Unmarshal(latest.Payload, &doc); err != nil {
			return fmt.Errorf("payload: %w", err)
		}
		decision := i18ntext.Skip
		for _, l := range lines {
			d, code := i18ntext.DecideImport(l.key.Version, latest.VersionNo, latest.Status)
			if d == i18ntext.Skip {
				lineErrs = append(lineErrs, i18ntext.ReportError{Ref: l.ref, Code: code, Message: "line skipped (" + code + ")"})
				continue
			}
			if err := i18ntext.Set(doc, l.key.Pointer, j.TargetLanguage, l.text); err != nil {
				lineErrs = append(lineErrs, i18ntext.ReportError{Ref: l.ref, Code: i18ntext.CodeKeyNotFound, Message: "no game text at this pointer"})
				continue
			}
			decision = d
			applied = append(applied, l)
		}
		if len(applied) == 0 || j.DryRun {
			return nil
		}
		return h.Admin.saveImportedDraft(ctx, t, *it, *latest, doc, decision, j.RequestedBy, now)
	})
	var skip errSkipItem
	var pe *problem.Error
	switch {
	case errors.As(err, &skip):
		for _, l := range lines {
			report.Skip(l.ref, skip.code, skip.msg)
		}
		return nil
	case errors.As(err, &pe):
		for _, e := range lineErrs {
			report.Skip(e.Ref, e.Code, e.Message)
		}
		for _, l := range applied {
			report.Skip(l.ref, "", "the draft was refused: "+string(pe.Code))
		}
		return nil
	case err != nil:
		return err
	}
	for _, e := range lineErrs {
		report.Skip(e.Ref, e.Code, e.Message)
	}
	report.Updated += len(applied)
	return nil
}

type errSkipItem struct{ code, msg string }

func (e errSkipItem) Error() string { return "translation import: " + e.msg }

// saveImportedDraft writes an imported translation through the draft core (F082-AC-13, research R2): decision
// WriteDraft updates the newest draft (its revision, as If-Match), NewDraft inserts a new draft based on the newest
// version; type checks, translation state and audit are those of updateGameConfigDraft/createGameConfigVersion. The
// author of a new draft is the requester of the job.
func (h GameAdmin) saveImportedDraft(ctx context.Context, t gameconfig.Type, it write.ConfigItem, latest write.ConfigVersion,
	doc map[string]any, decision i18ntext.Decision, actor uuid.UUID, now time.Time) error {
	switch decision {
	case i18ntext.WriteDraft:
		b, sha, _, err := h.prepare(ctx, t, doc, it.ItemKey, latest.ID, latest.VersionNo, gameconfig.StatusDraft, now)
		if err != nil {
			return err
		}
		next := latest
		next.Payload, next.SHA256, next.Status = b, sha, string(gameconfig.StatusDraft)
		ok, err := h.Store.UpdateDraft(ctx, next, latest.Revision, actor, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := h.record(ctx, ActionConfigChanged, t, it.ConfigID, "", map[string]any{"version": latest.VersionNo, "source": "translation_import"}); err != nil {
			return err
		}
		return h.saveTranslationState(ctx, t, next, latest.Payload, &latest.ID, now, nil)
	case i18ntext.NewDraft:
		no, err := h.Store.NextVersionNo(ctx, it)
		if err != nil {
			return err
		}
		vid := ids.New()
		b, sha, _, err := h.prepare(ctx, t, doc, it.ItemKey, vid, no, gameconfig.StatusDraft, now)
		if err != nil {
			return err
		}
		author, based := actor, latest.ID
		note := "translation import"
		v := write.ConfigVersion{ID: vid, ConfigID: it.ConfigID, ItemType: t.ItemType, VersionNo: no, Status: string(gameconfig.StatusDraft),
			Title: latest.Title, Payload: b, SHA256: sha, AuthorID: &author, ChangeNote: &note, BasedOn: &based, UpdatedAt: now, Revision: 1}
		if err := h.Store.InsertVersion(ctx, v, actor, now); err != nil {
			return err
		}
		if err := h.record(ctx, ActionVersionCreated, t, it.ConfigID, "", map[string]any{"version": no, "based_on_version": latest.VersionNo,
			"source": "translation_import"}); err != nil {
			return err
		}
		return h.saveTranslationState(ctx, t, v, latest.Payload, &latest.ID, now, nil)
	}
	return nil
}
