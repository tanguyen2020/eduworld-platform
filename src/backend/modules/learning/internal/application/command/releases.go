package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/release"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events and audit actions of the release (EVENT_DESIGN §4.2, specs/events/payloads/learning/content.*.v1, owner F026).
const (
	EventContentPublished        = "content.published"
	EventContentRolledBack       = "content.rolled_back"
	EventContentUnpublished      = "content.unpublished"
	EventContentPublishScheduled = "content.publish_scheduled"
	EventContentVersionArchived  = "content.version_archived"

	ActionContentRolledBack       = "content.rolled_back"
	ActionContentUnpublished      = "content.unpublished"
	ActionContentPublishScheduled = "content.publish_scheduled"
	ActionContentPublishCancelled = "content.publish_cancelled"
)

// ScheduledPublishMinLead is scheduled_publish_min_lead (CONFIG_DEFAULTS: 5 minutes, owner CONTENT_MODEL §5) until module
// learning reads platform settings.
const ScheduledPublishMinLead = 5 * time.Minute

// minClientVersion is the min_client_version of an activity_type from the Activity Registry (F030 T042: replaces the
// temporary table of F026).
func (h Releases) minClientVersion(t string) string { return h.Activities.MinClientVersion(t) }

// ReleaseMetrics counts releases (eduworld_content_releases_total{action}); nil = not counted.
type ReleaseMetrics interface {
	Released(action string)
}

// Releases holds the collaborators of the release use cases (F026): publish now or at a time, rollback, unpublish,
// cancel a schedule and apply the due schedules.
type Releases struct {
	UoW       write.UnitOfWork
	Workflow  Workflow
	Store     write.ReleaseStore
	Schedules write.ScheduleStore
	// Cache keeps snapshots and release pointers in Valkey (ew:learning:cv:*, ew:learning:current:*); nil = no cache.
	Cache port.SnapshotCache
	// CDN stores the snapshots of platform content without exam data in ew-media-public; nil = API only.
	CDN     port.ObjectStorage
	Signer  port.URLSigner
	Events  write.EventPublisher
	Audit   write.Auditor
	Clock   func() time.Time
	Metrics ReleaseMetrics
	MinLead time.Duration
	// Activities is the Activity Registry (min_client_version, publish check of the activity types, F030) and
	// ActivitySchema the JSON Schemas of their config.
	Activities     *activity.Registry
	ActivitySchema port.ActivitySchema
	// Minigames reads the projection ref.minigame_config_versions (publish check of the minigame activities, F073-AC-15);
	// nil refuses them.
	Minigames write.MinigameConfigs
}

func (h Releases) now() time.Time { return now(h.Clock) }

func (h Releases) record(ctx context.Context, s Subject, e audit.Event) error {
	e.TargetType, e.TargetID, e.Result = string(s.Type), s.ItemID.String(), audit.ResultSuccess
	return TrashDeps{Audit: h.Audit}.recordFor(ctx, s.Owner.TenantID, e)
}

func (h Releases) count(action string) {
	if h.Metrics != nil {
		h.Metrics.Released(action)
	}
}

func idemKey(ctx context.Context) string {
	if pend, ok := idempotency.PendingFrom(ctx); ok {
		return pend.Key.Key.String()
	}
	return ids.New().String()
}

// releaseErr maps the release rules.
func releaseErr(err error) error {
	switch {
	case errors.Is(err, release.ErrNotApproved), errors.Is(err, release.ErrNotScheduled):
		return problem.New(problem.InvalidStateTransition, err.Error())
	case errors.Is(err, release.ErrRollbackTarget):
		return problem.New(problem.RollbackTargetInvalid, "the target must be another published version of the item")
	case errors.Is(err, release.ErrReasonRequired):
		return problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case errors.Is(err, release.ErrReasonTooLong):
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	case errors.Is(err, release.ErrNotPublished):
		return problem.New(problem.ContentNotPublished, "the item has no current release")
	case errors.Is(err, release.ErrScheduleTooSoon):
		return problem.Invalid("publish_at", problem.SubInvalid, "must be at least scheduled_publish_min_lead after now")
	}
	return err
}

// conflict maps a lost optimistic update of the item to 409 CONTENT_VERSION_CONFLICT (F026-AC-03).
func conflict(err error) error {
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.VersionMismatch {
		return problem.New(problem.ContentVersionConflict, "the content changed; reload it")
	}
	return err
}

// PublishContentInput is the request of publishContentVersion.
type PublishContentInput struct {
	Type       string
	ContentID  uuid.UUID
	VersionNo  int
	IfMatch    int64
	PublishAt  *time.Time
	ChangeNote *string
}

func (h Releases) load(ctx context.Context, p authz.Principal, typ string, id uuid.UUID, no int, perm authz.Permission) (Subject, error) {
	t, err := contentmodel.ParseContentType(typ)
	if err != nil || !slices.Contains(WorkflowTypes, t) {
		return Subject{}, problem.Invalid("content_type", problem.SubInvalid, "has no release in this release")
	}
	if no == 0 {
		no, err = h.latestNo(ctx, t, id)
		if err != nil {
			return Subject{}, err
		}
	}
	s, err := h.Workflow.Load(ctx, t, id, no)
	if err != nil {
		return s, err
	}
	return s, writable(ctx, p, perm, s.Owner)
}

func (h Releases) latestNo(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (int, error) {
	if t == contentmodel.TypeQuestion {
		q, err := h.Workflow.Questions.Question(ctx, id)
		return q.LatestVersionNo, err
	}
	d, err := h.Workflow.Docs.Doc(ctx, t, id)
	return d.LatestVersionNo, err
}

// Publish publishes an approved version now (F026-AC-01…07) or schedules it (AC-17): the readiness guard of F025 runs
// again, every referenced question version must be published (409 CONTENT_NOT_PUBLISHED), If-Match is the row version of
// the version (409 CONTENT_VERSION_CONFLICT when stale).
func (h Releases) Publish(ctx context.Context, in PublishContentInput) (out dto.ContentReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PublishContentVersion")
	defer func() { done(err) }()
	t, _ := contentmodel.ParseContentType(in.Type)
	perm := ActionPermission(t, contentmodel.ActionPublish)
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.load(ctx, p, in.Type, in.ContentID, in.VersionNo, perm)
		if err != nil {
			return err
		}
		if s.RowVersion != in.IfMatch {
			return problem.New(problem.ContentVersionConflict, "the version changed; reload it")
		}
		if err := h.ready(ctx, s); err != nil {
			return err
		}
		if in.PublishAt != nil {
			out, err = h.schedule(ctx, p.UserID, s, *in.PublishAt, in.ChangeNote, key)
		} else {
			out, err = h.apply(ctx, p.UserID, s, key, nil)
		}
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ready runs the publish conditions on an approved version: state, guard, referenced versions published.
func (h Releases) ready(ctx context.Context, s Subject) error {
	if err := release.CheckPublish(release.Version{ID: s.VersionID, ItemID: s.ItemID, Status: s.Status}); err != nil {
		return releaseErr(err)
	}
	if _, err := h.Workflow.Readiness(ctx, s); err != nil {
		return err
	}
	if s.Type == contentmodel.TypeActivity {
		if err := h.activityReady(ctx, s); err != nil {
			return err
		}
	}
	var missing []string
	for _, c := range s.Children {
		v, err := h.Workflow.Questions.QuestionVersionByID(ctx, c)
		if err != nil || v.Status != contentmodel.StatusPublished {
			missing = append(missing, c.String())
		}
	}
	if len(missing) > 0 {
		return problem.New(problem.ContentNotPublished, "referenced question versions are not published").With("items", missing)
	}
	return nil
}

func (h Releases) schedule(ctx context.Context, actor uuid.UUID, s Subject, at time.Time, note *string, key string) (dto.ContentReleaseView, error) {
	lead := h.MinLead
	if lead == 0 {
		lead = ScheduledPublishMinLead
	}
	if err := release.CheckSchedule(at, h.now(), lead); err != nil {
		return dto.ContentReleaseView{}, releaseErr(err)
	}
	sc := write.Schedule{ID: ids.New(), Owner: s.Owner, ItemType: s.Type, ItemID: s.ItemID, VersionID: s.VersionID, VersionNo: s.VersionNo,
		PublishAt: at.UTC(), Status: release.ScheduleScheduled, ChangeNote: note, RequestedBy: actor, IdempotencyKey: key}
	if err := h.Schedules.InsertSchedule(ctx, sc); err != nil {
		return dto.ContentReleaseView{}, err
	}
	if err := h.record(ctx, s, audit.Event{Action: ActionContentPublishScheduled, Metadata: map[string]any{"content_version_id": s.VersionID.String(),
		"version": s.VersionNo, "publish_at": at.UTC().Format(time.RFC3339), "schedule_id": sc.ID.String()}}); err != nil {
		return dto.ContentReleaseView{}, err
	}
	payload := map[string]any{"item_type": string(s.Type), "item_id": s.ItemID, "version_id": s.VersionID, "version": s.VersionNo,
		"schedule_id": sc.ID, "publish_at": at.UTC(), "owner_scope": string(s.Owner.Scope), "tenant_id": s.Owner.TenantID, "requested_by": actor}
	if err := h.emit(ctx, s, EventContentPublishScheduled, payload); err != nil {
		return dto.ContentReleaseView{}, err
	}
	no := s.VersionNo
	pa := at.UTC()
	return dto.ContentReleaseView{ReleaseID: sc.ID, ItemType: string(s.Type), ItemID: s.ItemID, Action: string(release.ActionPublish),
		ToVersion: &no, ScheduledFor: &pa, Status: string(release.ScheduleScheduled), ReleasedBy: actor, ReleasedAt: h.now(), Scheduled: true}, nil
}

func (h Releases) emit(ctx context.Context, s Subject, typ string, payload any) error {
	e := write.Event{Type: typ, SubjectType: string(s.Type), SubjectID: s.ItemID.String(), Payload: payload}
	if s.Owner.TenantID == nil {
		e.Platform = true
	} else {
		e.TenantID = ptrClone(s.Owner.TenantID)
	}
	return h.Events.Publish(ctx, e)
}

// cdnEligible: platform content without exam data goes to the public CDN; tenant content and exam content are served
// by the API only (F026-AC-13, R31).
func cdnEligible(s Subject) bool {
	return s.Owner.Platform() && !s.ExamOnly && s.Type != contentmodel.TypeAssessmentBlueprint
}

// SnapshotKey is the Valkey key of a snapshot: ew:learning:cv:<item_type>:<item_id>:<version_no>.
func SnapshotKey(t contentmodel.ContentType, id uuid.UUID, no int) string {
	return "ew:learning:cv:" + string(t) + ":" + id.String() + ":" + itoa(no)
}

// targets are the tenants whose students see the item: the owner tenant, or every tenant the platform item is assigned
// to (F026-AC-16, AC-23).
func (h Releases) targets(ctx context.Context, s Subject) ([]uuid.UUID, error) {
	if s.Owner.TenantID != nil {
		return []uuid.UUID{*s.Owner.TenantID}, nil
	}
	return h.Store.AssignedTenants(ctx, contentmodel.ResourceType(s.Type), s.ItemID)
}

// releasePayload is the payload of content.published, content.rolled_back and content.unpublished (v1).
type releasePayload struct {
	ItemType         string      `json:"item_type"`
	ItemID           uuid.UUID   `json:"item_id"`
	FromVersionID    *uuid.UUID  `json:"from_version_id"`
	ToVersionID      *uuid.UUID  `json:"to_version_id"`
	ToVersion        *int        `json:"to_version,omitempty"`
	ReleaseID        uuid.UUID   `json:"release_id"`
	OwnerScope       string      `json:"owner_scope"`
	TenantID         *uuid.UUID  `json:"tenant_id"`
	TenantIDs        []uuid.UUID `json:"tenant_ids"`
	ActivityType     string      `json:"activity_type,omitempty"`
	MinClientVersion string      `json:"min_client_version,omitempty"`
	GradeLevels      []int       `json:"grade_levels,omitempty"`
	SubjectID        *uuid.UUID  `json:"subject_id,omitempty"`
	ObjectiveIDs     []uuid.UUID `json:"objective_ids,omitempty"`
	SnapshotURL      string      `json:"snapshot_url,omitempty"`
	SnapshotSHA256   string      `json:"snapshot_sha256,omitempty"`
	CDNEligible      *bool       `json:"cdn_eligible,omitempty"`
	Scheduled        *bool       `json:"scheduled,omitempty"`
	PublishedBy      *uuid.UUID  `json:"published_by,omitempty"`
	Reason           string      `json:"reason,omitempty"`
	ReleasedBy       *uuid.UUID  `json:"released_by,omitempty"`
	ReleasedAt       time.Time   `json:"released_at"`
}

// apply publishes s now in the transaction of ctx: snapshot (Valkey, CDN when eligible), version published, release
// pointer, content_releases, transition, audit and outbox; sc is the schedule applied (nil = immediate publish).
func (h Releases) apply(ctx context.Context, actor uuid.UUID, s Subject, key string, sc *write.Schedule) (dto.ContentReleaseView, error) {
	var out dto.ContentReleaseView
	snap, sha, err := dto.BuildSnapshot(s.Type, s.ItemID, s.VersionID, s.VersionNo, s.ContentLanguage, s.GradeLevels, s.QuestionType, s.SnapshotDoc)
	if err != nil {
		return out, err
	}
	at := h.now()
	cacheKey := SnapshotKey(s.Type, s.ItemID, s.VersionNo)
	if h.Cache != nil {
		_ = h.Cache.Put(ctx, cacheKey, snap) // cache only (R9): a failure is served from PostgreSQL later
	}
	eligible := cdnEligible(s)
	snapshotURL := ""
	if eligible && h.CDN != nil {
		objKey := "content/" + string(s.Type) + "/" + s.ItemID.String() + "/" + itoa(s.VersionNo) + "/" + sha + ".json"
		if err := h.CDN.Put(ctx, media.BucketPublic, objKey, bytes.NewReader(snap), int64(len(snap)), "application/json",
			"public, max-age=31536000, immutable"); err != nil {
			return out, err
		}
		if h.Signer != nil {
			snapshotURL, _ = h.Signer.URL(media.BucketPublic, objKey, at)
		}
	}
	if _, err := h.Workflow.Status.SetVersionStatus(ctx, write.StatusChange{Type: s.Type, VersionID: s.VersionID, IfRow: s.RowVersion,
		Status: contentmodel.StatusPublished, PublishedAt: &at, PublishedBy: &actor, SnapshotKey: &cacheKey, SnapshotSHA: &sha}); err != nil {
		return out, conflict(err)
	}
	prev := s.CurrentVersionID
	if err := h.Store.SetCurrentVersion(ctx, s.Type, s.ItemID, &s.VersionID, s.ItemRowVersion); err != nil {
		return out, conflict(err)
	}
	rel := write.ReleaseRow{ID: ids.New(), Owner: s.Owner, ItemType: s.Type, ItemID: s.ItemID, Action: release.ActionPublish, FromVersionID: prev,
		ToVersionID: &s.VersionID, ReleasedBy: actor, ReleasedAt: at, IdempotencyKey: key}
	if sc != nil {
		rel.ScheduledFor = &sc.PublishAt
	}
	if err := h.Store.InsertRelease(ctx, rel); err != nil {
		return out, err
	}
	if err := h.Workflow.Reviews.InsertTransition(ctx, write.Transition{Owner: s.Owner, ContentType: s.Type, ContentID: s.ItemID,
		ContentVersionID: s.VersionID, Action: contentmodel.ActionPublish, From: s.Status, To: contentmodel.StatusPublished, ActorID: actor,
		IdempotencyKey: key, At: at}); err != nil {
		return out, err
	}
	if err := h.record(ctx, s, audit.Event{Action: ActionContentPublished, Metadata: map[string]any{"content_version_id": s.VersionID.String(),
		"version": s.VersionNo, "release_id": rel.ID.String(), "scheduled": sc != nil}}); err != nil {
		return out, err
	}
	tenants, err := h.targets(ctx, s)
	if err != nil {
		return out, err
	}
	scheduled := sc != nil
	no := s.VersionNo
	pl := releasePayload{ItemType: string(s.Type), ItemID: s.ItemID, FromVersionID: prev, ToVersionID: &s.VersionID, ToVersion: &no,
		ReleaseID: rel.ID, OwnerScope: string(s.Owner.Scope), TenantID: s.Owner.TenantID, TenantIDs: tenants, GradeLevels: s.GradeLevels,
		SubjectID: s.SubjectID, ObjectiveIDs: s.ObjectiveIDs, SnapshotURL: snapshotURL, SnapshotSHA256: sha, CDNEligible: &eligible,
		Scheduled: &scheduled, PublishedBy: &actor, ReleasedAt: at}
	if s.ActivityType != "" {
		pl.ActivityType, pl.MinClientVersion = s.ActivityType, h.minClientVersion(s.ActivityType)
	}
	if err := h.emit(ctx, s, EventContentPublished, pl); err != nil {
		return out, err
	}
	h.count(string(release.ActionPublish))
	out = dto.ContentReleaseView{ReleaseID: rel.ID, ItemType: string(s.Type), ItemID: s.ItemID, Action: string(release.ActionPublish),
		ToVersion: &no, Status: string(release.ScheduleApplied), ReleasedBy: actor, ReleasedAt: at, ScheduledFor: rel.ScheduledFor}
	if prev != nil {
		out.FromVersion = h.versionNo(ctx, s, *prev)
	}
	return out, nil
}

// versionNo is the number of a version of the item of s (nil when it cannot be read).
func (h Releases) versionNo(ctx context.Context, s Subject, id uuid.UUID) *int {
	if s.Type == contentmodel.TypeQuestion {
		v, err := h.Workflow.Questions.QuestionVersionByID(ctx, id)
		if err != nil {
			return nil
		}
		return &v.VersionNo
	}
	vs, err := h.Workflow.Docs.DocVersions(ctx, s.Type, s.ItemID)
	if err != nil {
		return nil
	}
	for _, v := range vs {
		if v.ID == id {
			n := v.VersionNo
			return &n
		}
	}
	return nil
}

// RollbackInput is the request of rollbackContent.
type RollbackInput struct {
	Type      string
	ContentID uuid.UUID
	ToVersion int
	Reason    string
}

// Rollback points the release at another published version of the item (F026-AC-08, AC-09, AC-11): no version is
// created; content_releases (rollback), audit content.rolled_back and outbox content.rolled_back.
func (h Releases) Rollback(ctx context.Context, in RollbackInput) (out dto.ContentReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RollbackContent")
	defer func() { done(err) }()
	if err := release.CheckReason(in.Reason); err != nil {
		return out, releaseErr(err)
	}
	p, err := authorize(ctx, authz.PermContentReleaseRollback)
	if err != nil {
		return out, err
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.load(ctx, p, in.Type, in.ContentID, 0, authz.PermContentReleaseRollback)
		if err != nil {
			return err
		}
		target, err := h.Workflow.Load(ctx, cur.Type, in.ContentID, in.ToVersion)
		if err != nil {
			return problem.New(problem.RollbackTargetInvalid, "the target version does not exist")
		}
		if err := release.CheckRollback(cur.ItemID, cur.CurrentVersionID, release.Version{ID: target.VersionID, ItemID: target.ItemID,
			Status: target.Status}, in.Reason); err != nil {
			return releaseErr(err)
		}
		out, err = h.move(ctx, p.UserID, cur, release.ActionRollback, &target.VersionID, &target.VersionNo, in.Reason, key)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UnpublishInput is the request of unpublishContent.
type UnpublishInput struct {
	Type      string
	ContentID uuid.UUID
	Reason    string
}

// Unpublish clears the release pointer (F026-AC-10, AC-11): new attempts get 409 CONTENT_NOT_PUBLISHED; pinned attempts
// keep their version.
func (h Releases) Unpublish(ctx context.Context, in UnpublishInput) (out dto.ContentReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UnpublishContent")
	defer func() { done(err) }()
	if err := release.CheckReason(in.Reason); err != nil {
		return out, releaseErr(err)
	}
	t, _ := contentmodel.ParseContentType(in.Type)
	perm := ActionPermission(t, contentmodel.ActionPublish)
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.load(ctx, p, in.Type, in.ContentID, 0, perm)
		if err != nil {
			return err
		}
		if err := release.CheckUnpublish(cur.CurrentVersionID, in.Reason); err != nil {
			return releaseErr(err)
		}
		out, err = h.move(ctx, p.UserID, cur, release.ActionUnpublish, nil, nil, in.Reason, key)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// move changes the release pointer of the item of cur (rollback or unpublish).
func (h Releases) move(ctx context.Context, actor uuid.UUID, cur Subject, action release.Action, to *uuid.UUID, toNo *int, reason, key string) (dto.ContentReleaseView, error) {
	var out dto.ContentReleaseView
	at := h.now()
	prev := cur.CurrentVersionID
	if err := h.Store.SetCurrentVersion(ctx, cur.Type, cur.ItemID, to, cur.ItemRowVersion); err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.RollbackTargetInvalid {
			return out, err
		}
		return out, conflict(err)
	}
	rel := write.ReleaseRow{ID: ids.New(), Owner: cur.Owner, ItemType: cur.Type, ItemID: cur.ItemID, Action: action, FromVersionID: prev,
		ToVersionID: to, Reason: reason, ReleasedBy: actor, ReleasedAt: at, IdempotencyKey: key}
	if err := h.Store.InsertRelease(ctx, rel); err != nil {
		return out, err
	}
	auditAction, typ := ActionContentRolledBack, EventContentRolledBack
	if action == release.ActionUnpublish {
		auditAction, typ = ActionContentUnpublished, EventContentUnpublished
	}
	meta := map[string]any{"release_id": rel.ID.String()}
	if prev != nil {
		meta["from_version_id"] = prev.String()
	}
	if to != nil {
		meta["to_version_id"] = to.String()
	}
	if err := h.record(ctx, cur, audit.Event{Action: auditAction, Reason: reason, Metadata: meta}); err != nil {
		return out, err
	}
	tenants, err := h.targets(ctx, cur)
	if err != nil {
		return out, err
	}
	pl := releasePayload{ItemType: string(cur.Type), ItemID: cur.ItemID, FromVersionID: prev, ToVersionID: to, ToVersion: toNo, ReleaseID: rel.ID,
		OwnerScope: string(cur.Owner.Scope), TenantID: cur.Owner.TenantID, TenantIDs: tenants, Reason: reason, ReleasedBy: &actor, ReleasedAt: at}
	if to != nil {
		tgt, err := h.Workflow.Load(ctx, cur.Type, cur.ItemID, *toNo)
		if err == nil {
			pl.SnapshotSHA256 = deref(tgt.SnapshotSHA)
			if tgt.ActivityType != "" {
				pl.ActivityType, pl.MinClientVersion = tgt.ActivityType, h.minClientVersion(tgt.ActivityType)
			}
		}
	}
	if err := h.emit(ctx, cur, typ, pl); err != nil {
		return out, err
	}
	h.count(string(action))
	out = dto.ContentReleaseView{ReleaseID: rel.ID, ItemType: string(cur.Type), ItemID: cur.ItemID, Action: string(action), ToVersion: toNo,
		Reason: reason, Status: string(release.ScheduleApplied), ReleasedBy: actor, ReleasedAt: at}
	if prev != nil {
		out.FromVersion = h.versionNo(ctx, cur, *prev)
	}
	return out, nil
}

// CancelScheduleInput is the request of cancelScheduledPublication.
type CancelScheduleInput struct {
	Type       string
	ContentID  uuid.UUID
	ScheduleID uuid.UUID
}

// CancelSchedule cancels a scheduled publication before it runs (F026-AC-19): an applied, failed or cancelled one is
// 422 INVALID_STATE_TRANSITION.
func (h Releases) CancelSchedule(ctx context.Context, in CancelScheduleInput) (out dto.ContentReleaseView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CancelScheduledPublication")
	defer func() { done(err) }()
	t, _ := contentmodel.ParseContentType(in.Type)
	perm := ActionPermission(t, contentmodel.ActionPublish)
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.load(ctx, p, in.Type, in.ContentID, 0, perm)
		if err != nil {
			return err
		}
		sc, err := h.Schedules.Schedule(ctx, in.ScheduleID)
		if err != nil {
			return err
		}
		if sc.ItemID != cur.ItemID || sc.ItemType != cur.Type {
			return problem.New(problem.NotFound, "")
		}
		st, err := release.Cancel(sc.Status)
		if err != nil {
			return releaseErr(err)
		}
		row := sc.RowVersion
		sc.Status, sc.CancelledBy = st, &p.UserID
		if err := h.Schedules.UpdateSchedule(ctx, sc, row); err != nil {
			return conflict(err)
		}
		if err := h.record(ctx, cur, audit.Event{Action: ActionContentPublishCancelled, Metadata: map[string]any{"schedule_id": sc.ID.String(),
			"content_version_id": sc.VersionID.String()}}); err != nil {
			return err
		}
		no, pa := sc.VersionNo, sc.PublishAt
		out = dto.ContentReleaseView{ReleaseID: sc.ID, ItemType: string(sc.ItemType), ItemID: sc.ItemID, Action: string(release.ActionPublish),
			ToVersion: &no, ScheduledFor: &pa, Status: string(st), ReleasedBy: sc.RequestedBy, ReleasedAt: h.now()}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// RunDueSchedules applies the due scheduled publications of every owner (job content-scheduled-publish, F026-AC-18):
// each in the scope of its owner and its own transaction; the guard runs again, a failure marks the schedule failed
// without moving the pointer; a schedule taken by another worker (row version moved) is skipped.
func (h Releases) RunDueSchedules(ctx context.Context, limit int) (applied int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.RunDueSchedules")
	defer func() { done(err) }()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	var due []write.Schedule
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		due, err = h.Schedules.DueSchedules(ctx, h.now(), limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, sc := range due {
		ok, err := h.runSchedule(ctx, sc)
		if err != nil {
			return applied, err
		}
		if ok {
			applied++
		}
	}
	return applied, nil
}

func (h Releases) runSchedule(ctx context.Context, sc write.Schedule) (bool, error) {
	sctx, err := jobScope(ctx, sc.Owner)
	if err != nil {
		return false, err
	}
	applied := false
	err = h.UoW.Do(sctx, func(ctx context.Context) error {
		cur, err := h.Schedules.Schedule(ctx, sc.ID)
		if err != nil || cur.Status != release.ScheduleScheduled {
			return err
		}
		var res dto.ContentReleaseView
		// The publish runs in a savepoint: a refused or failed publish leaves no partial write, only the failed schedule.
		err = h.UoW.Savepoint(ctx, func(ctx context.Context) error {
			s, err := h.Workflow.Load(ctx, sc.ItemType, sc.ItemID, sc.VersionNo)
			if err != nil {
				return err
			}
			if err := h.ready(ctx, s); err != nil {
				return err
			}
			res, err = h.apply(ctx, sc.RequestedBy, s, sc.ID.String(), &cur)
			return err
		})
		row := cur.RowVersion
		if err != nil {
			var pe *problem.Error
			if !errors.As(err, &pe) {
				return err
			}
			code := string(pe.Code)
			cur.Status, cur.FailureCode = release.ScheduleFailed, &code
			return h.Schedules.UpdateSchedule(ctx, cur, row)
		}
		cur.Status, cur.AppliedReleaseID = release.ScheduleApplied, &res.ReleaseID
		if err := h.Schedules.UpdateSchedule(ctx, cur, row); err != nil {
			return err
		}
		applied = true
		return nil
	})
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.VersionMismatch {
		return false, nil // taken by another worker
	}
	return applied, err
}

func itoa(n int) string { return strconv.Itoa(n) }

// activityReady checks an activity version again at publish (F030-AC-20, AC-21): its type has a plugin of the release and
// its config matches the JSON Schema and the rules of the plugin.
func (h Releases) activityReady(ctx context.Context, s Subject) error {
	var b struct {
		ActivityType string          `json:"activity_type"`
		Config       json.RawMessage `json:"config"`
	}
	_ = json.Unmarshal(s.Doc, &b)
	if b.ActivityType == "" {
		b.ActivityType = s.ActivityType
	}
	p, err := ActivityPlugin(h.Activities, b.ActivityType)
	if err != nil {
		return err
	}
	if h.ActivitySchema != nil {
		if vs := h.ActivitySchema.ValidateConfig(b.ActivityType, b.Config); len(vs) > 0 {
			return configProblem(vs)
		}
	}
	cfg, err := activity.ParseConfig(b.Config)
	if err != nil {
		return configProblem(contentmodel.Violations{{Field: "/config", Reason: "is invalid"}})
	}
	if vs := p.ValidateConfig(cfg); len(vs) > 0 {
		return configProblem(vs)
	}
	// Rules checked only at publish (audio_lesson: transcript required, F066-AC-01).
	if pc, ok := p.(activity.PublishCheck); ok {
		if vs := pc.ValidatePublish(cfg); len(vs) > 0 {
			return configProblem(vs)
		}
	}
	// F073-AC-15: the wrapped mini-game configuration and the question set override, checked again at publish.
	if _, ok := p.(activity.GameScored); ok {
		return minigameChecker{Minigames: h.Minigames, Docs: h.Workflow.Docs, Questions: h.Workflow.Questions}.check(ctx, b.Config)
	}
	return nil
}
