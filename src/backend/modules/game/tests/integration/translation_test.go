//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/translation"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// ---------- env wiring of the translation use cases (F082) ----------

func (e *env) translationsUC() command.Translations {
	if e.storage == nil {
		e.storage = newMemStorage()
	}
	return command.Translations{UoW: e.uow, Store: writerepo.TranslationRepo{}, Configs: writerepo.GameConfigRepo{}, Media: writerepo.GameMediaRepo{},
		Admin: e.adminUC(), Storage: e.storage, Codecs: map[string]port.TranslationCodec{"xliff": translation.XLIFF{}, "csv": translation.CSV{}},
		Audit: writerepo.Auditor{R: e.rec}, Now: e.clock.Now}
}

type translationProxy struct{ e *env }

func (p translationProxy) Export(ctx context.Context, in command.ExportInput) (command.JobView, error) {
	return p.e.translationsUC().Export(ctx, in)
}
func (p translationProxy) Import(ctx context.Context, in command.ImportInput) (command.JobView, error) {
	return p.e.translationsUC().Import(ctx, in)
}

type translationQueryProxy struct{ e *env }

func (p translationQueryProxy) Get(ctx context.Context, id uuid.UUID) (command.JobView, error) {
	if p.e.storage == nil {
		p.e.storage = newMemStorage()
	}
	return query.TranslationJobs{Read: p.e.read, Jobs: read.TranslationJobs{}, Storage: p.e.storage, LinkTTL: 15 * time.Minute,
		Now: p.e.clock.Now}.Get(ctx, id)
}

// runJobs runs the queued translation jobs (the worker tick).
func (e *env) runJobs(ctx context.Context, t *testing.T) {
	t.Helper()
	for i := 0; i < 10; i++ {
		n, err := e.translationsUC().RunNext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
}

// draftQuest creates a quest draft through the console and returns its config id and the response.
func (e *env) draftQuest(t *testing.T, tok, code string, edit func(m map[string]any)) (string, resp) {
	t.Helper()
	c := e.adminCall(t, http.MethodPost, "/game-admin/quests", tok, "", map[string]any{"title": "Quest " + code, "payload": questDraft(t, code, edit)})
	if c.status != http.StatusCreated {
		t.Fatalf("create %s: %d %s", code, c.status, c.raw)
	}
	return c.body["config_id"].(string), c
}

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// uploadDocument uploads a translation file as game media of kind document and runs the media pipeline (ready).
func (e *env) uploadDocument(ctx context.Context, t *testing.T, tok, name, mime string, body []byte) string {
	t.Helper()
	r := e.adminCall(t, http.MethodPost, "/game-media:presign-upload", tok, "", map[string]any{"kind": "document", "filename": name,
		"mime_type": mime, "size_bytes": len(body), "sha256": sha(body), "license": "internal"})
	if r.status != http.StatusCreated {
		t.Fatalf("presign document: %d %s", r.status, r.raw)
	}
	id := r.body["asset_id"].(string)
	e.storage.put(pkgmedia.BucketQuarantine, pkgmedia.QuarantineKey(id), body)
	if _, err := e.mediaUC().ProcessUploads(ctx); err != nil {
		t.Fatal(err)
	}
	if e.storage.has(pkgmedia.BucketPublic, "media/"+sha(body)+"/original.csv") || e.storage.has(pkgmedia.BucketPublic, "media/"+sha(body)+"/original.xlf") {
		t.Fatal("a translation file reached the public bucket (R31)")
	}
	return id
}

// F082-AC-08, AC-09: the translation state is computed with each draft save and filters the list.
func TestTranslationState(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	// a draft without en is saved (partial, missing pointer listed) — before F082 it was refused.
	id, c := e.draftQuest(t, a.designer, "math.g3.tr.partial", func(m map[string]any) { m["name"] = map[string]any{"vi": "Bài toán phiên chợ"} })
	if c.body["translation_status"] != "partial" || strings.Join(strs(c.body["missing_fields"]), ",") != "/name/en" {
		t.Fatalf("partial: %s", c.raw)
	}
	var n int
	if err := e.migrator.QueryRowContext(ctx, `SELECT count(*) FROM game.config_translation_states WHERE config_id = $1 AND translation_status = 'partial'
		AND deleted_at IS NULL`, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("state row: %d %v", n, err)
	}
	// submit_review still refuses the missing language (TRANSLATION_INCOMPLETE, F082-AC-11 server side).
	base := "/game-admin/quests/" + id + "/versions/1"
	if r := e.adminCall(t, http.MethodPost, base+":transition", a.designer, c.header.Get("ETag"), map[string]any{"action": "submit_review"}); r.status != 422 ||
		r.code() != "TRANSLATION_INCOMPLETE" {
		t.Fatalf("submit: %d %s", r.status, r.raw)
	}
	// filled → complete.
	g := e.adminCall(t, http.MethodGet, base, a.designer, "", nil)
	payload := g.body["payload"].(map[string]any)
	payload["name"] = map[string]any{"vi": "Bài toán phiên chợ", "en": "Market problems"}
	u := e.adminCall(t, http.MethodPatch, base, a.designer, g.header.Get("ETag"), map[string]any{"payload": payload})
	if u.status != http.StatusOK || u.body["translation_status"] != "complete" {
		t.Fatalf("complete: %d %s", u.status, u.raw)
	}
	// the source vi of a translated step title changes → outdated with translated_in_version 1.
	payload["steps"].([]any)[0].(map[string]any)["title"] = map[string]any{"vi": "Gặp Ông Số ở chợ", "en": "Meet Mr. Numbers"}
	u2 := e.adminCall(t, http.MethodPatch, base, a.designer, u.header.Get("ETag"), map[string]any{"payload": payload})
	of, _ := u2.body["outdated_fields"].([]any)
	if u2.status != http.StatusOK || u2.body["translation_status"] != "outdated" || len(of) != 1 ||
		of[0].(map[string]any)["pointer"] != "/steps/0/title" || of[0].(map[string]any)["translated_in_version"] != float64(1) {
		t.Fatalf("outdated: %d %s", u2.status, u2.raw)
	}
	// list filter on the translation state of the newest version (F082-AC-09).
	other, _ := e.draftQuest(t, a.designer, "math.g3.tr.other", func(m map[string]any) { m["description"] = map[string]any{"vi": "Mô tả"} })
	for status, want := range map[string]string{"outdated": id, "partial": other} {
		l := e.adminCall(t, http.MethodGet, "/game-admin/quests?translation_status="+status, a.designer, "", nil)
		data, _ := l.body["data"].([]any)
		if l.status != http.StatusOK || len(data) != 1 || data[0].(map[string]any)["config_id"] != want {
			t.Fatalf("filter %s: %d %s", status, l.status, l.raw)
		}
	}
	if l := e.adminCall(t, http.MethodGet, "/game-admin/quests?translation_status=done", a.designer, "", nil); l.status != 422 {
		t.Fatalf("bad filter: %d", l.status)
	}
	if l := e.adminCall(t, http.MethodGet, "/game-admin/quests?tag=x", a.designer, "", nil); l.status != 422 {
		t.Fatalf("chat filter on a game type: %d", l.status)
	}
	// backfill: a version written by the release core (seed publish, no console save) gets its state from the job.
	seedID := uuid.NewString()
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(seedID, "math.g3.tr.seed", nil)})
	var before int
	if err := e.migrator.QueryRowContext(ctx, `SELECT count(*) FROM game.config_translation_states WHERE config_id = $1`, seedID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatalf("seed version already has a state: %d", before)
	}
	if _, err := (command.TranslationBackfill{UoW: e.uow, Store: writerepo.TranslationRepo{}, Required: []string{"vi", "en"}, Now: e.clock.Now}).Run(ctx); err != nil {
		t.Fatal(err)
	}
	var st string
	if err := e.migrator.QueryRowContext(ctx, `SELECT translation_status FROM game.config_translation_states WHERE config_id = $1 AND deleted_at IS NULL`,
		seedID).Scan(&st); err != nil || st != "complete" {
		t.Fatalf("backfill: %q %v", st, err)
	}
}

// F082-AC-12, AC-15 (job API), AC-16, AC-17, AC-19: export jobs.
func TestTranslationExport(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	partial, _ := e.draftQuest(t, a.designer, "math.g3.tr.exp1", func(m map[string]any) { m["name"] = map[string]any{"vi": "Thiếu tên"} })
	complete, _ := e.draftQuest(t, a.designer, "math.g3.tr.exp2", nil)
	body := map[string]any{"content_types": []string{"quests"}, "target_language": "en", "format": "xliff"}
	key := newKey()
	r := e.do(t, http.MethodPost, "/game-admin/translations:export", a.designer, map[string]string{"Idempotency-Key": key}, body)
	if r.status != http.StatusAccepted || r.body["status"] != "queued" || r.body["kind"] != "export" {
		t.Fatalf("export: %d %s", r.status, r.raw)
	}
	job := r.body["job_id"].(string)
	// the same Idempotency-Key returns the same job (F082-AC-14).
	if r2 := e.do(t, http.MethodPost, "/game-admin/translations:export", a.designer, map[string]string{"Idempotency-Key": key}, body); r2.body["job_id"] != job {
		t.Fatalf("replay: %s", r2.raw)
	}
	e.runJobs(ctx, t)
	g := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+job, a.designer, "", nil)
	if g.status != http.StatusOK || g.body["status"] != "succeeded" || g.body["download_url"] == nil || g.body["expires_at"] == nil {
		t.Fatalf("job: %d %s", g.status, g.raw)
	}
	b, ok := e.storage.objects[command.BucketExports+"/translations/"+job+".xlf"]
	if !ok {
		t.Fatal("no export file in ew-exports")
	}
	f, err := translation.XLIFF{}.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	// only the missing text of the partial quest (only_missing default true); the complete quest exports nothing.
	if len(f.Units) != 1 || f.Units[0].Key != "quests/"+partial+"/1/name" || f.Units[0].Note != "missing" || f.Units[0].Source != "Thiếu tên" {
		t.Fatalf("units: %+v", f.Units)
	}
	// all texts with only_missing false, CSV, one item.
	r3 := e.adminCall(t, http.MethodPost, "/game-admin/translations:export", a.designer, "", map[string]any{"content_ids": []string{complete},
		"target_language": "en", "format": "csv", "only_missing": false})
	e.runJobs(ctx, t)
	csvb := e.storage.objects[command.BucketExports+"/translations/"+r3.body["job_id"].(string)+".csv"]
	cf, err := translation.CSV{}.Decode(bytes.NewReader(csvb))
	if err != nil || len(cf.Units) < 6 {
		t.Fatalf("csv: %v %d", err, len(cf.Units))
	}
	// F082-AC-16: another translator gets 404 for the job, a System Admin reads it; a role without the permission is 403.
	if r := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+job, e.platformToken("game_designer"), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("other requester: %d", r.status)
	}
	if r := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+job, e.platformToken("system_admin"), "", nil); r.status != http.StatusOK {
		t.Fatalf("system admin: %d %s", r.status, r.raw)
	}
	before := e.audits(ctx, t, "authz.denied")
	for name, tok := range map[string]string{"reviewer": a.reviewer, "tenant admin": e.staffToken(e.staff(e.tenantA, "tenant_admin"))} {
		if r := e.adminCall(t, http.MethodPost, "/game-admin/translations:export", tok, "", body); r.status != http.StatusForbidden {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if after := e.audits(ctx, t, "authz.denied"); after <= before {
		t.Fatal("no authz.denied")
	}
	// F082-AC-17: a tenant record id (school event) is treated as not existing.
	tenantEvent := uuid.NewString()
	r4 := e.adminCall(t, http.MethodPost, "/game-admin/translations:export", a.designer, "", map[string]any{"content_types": []string{"events"},
		"content_ids": []string{tenantEvent}, "target_language": "en", "format": "xliff"})
	e.runJobs(ctx, t)
	g4 := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+r4.body["job_id"].(string), a.designer, "", nil)
	rep := g4.body["report"].(map[string]any)
	if rep["total"] != float64(0) || len(rep["errors"].([]any)) != 1 || rep["errors"].([]any)[0].(map[string]any)["code"] != "key_not_found" {
		t.Fatalf("tenant id: %s", g4.raw)
	}
	// F082-AC-19: limits and a type outside module game are 422 naming the field.
	ids := make([]string, 501)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	types := make([]string, 21)
	for i := range types {
		types[i] = "quests"
	}
	for name, b := range map[string]map[string]any{
		"501 ids":   {"content_ids": ids, "target_language": "en", "format": "xliff"},
		"21 types":  {"content_types": types, "target_language": "en", "format": "xliff"},
		"chat type": {"content_types": []string{"sticker-packs"}, "target_language": "en", "format": "xliff"},
		"same lang": {"target_language": "vi", "format": "xliff"},
		"format":    {"target_language": "en", "format": "docx"},
	} {
		if r := e.adminCall(t, http.MethodPost, "/game-admin/translations:export", a.designer, "", b); r.status != 422 || r.code() != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
}

// F082-AC-13, AC-14: import jobs (dry run, draft written, new draft of a published version, report lines, broken file).
func TestTranslationImport(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	a := e.actors()
	draft, _ := e.draftQuest(t, a.designer, "math.g3.tr.imp1", func(m map[string]any) { m["name"] = map[string]any{"vi": "Tên nháp"} })
	pubID, etag := e.approvedQuest(t, a, "math.g3.tr.imp2")
	if p := e.adminCall(t, http.MethodPost, "/game-admin/quests/"+pubID+"/versions/1:publish", a.liveops, etag, map[string]any{}); p.status != http.StatusOK {
		t.Fatalf("publish: %d %s", p.status, p.raw)
	}
	review, rc := e.draftQuest(t, a.designer, "math.g3.tr.imp3", nil)
	if r := e.adminCall(t, http.MethodPost, "/game-admin/quests/"+review+"/versions/1:transition", a.designer, rc.header.Get("ETag"),
		map[string]any{"action": "submit_review"}); r.status != http.StatusOK {
		t.Fatalf("review: %d %s", r.status, r.raw)
	}
	csv := "key,source,target,note\n" +
		"quests/" + draft + "/1/name,Tên nháp,Draft name,missing\n" +
		"quests/" + pubID + "/1/name,Bài toán phiên chợ,Market maths,\n" +
		"quests/" + pubID + "/1/steps/0/title,Gặp Ông Số,Meet him,\n" +
		"quests/" + review + "/1/name,x,y,\n" +
		"quests/" + uuid.NewString() + "/1/name,x,y,\n" +
		"quests/" + draft + "/9/name,x,y,\n" +
		"sticker-packs/" + uuid.NewString() + "/1/name,x,y,\n" +
		"quests/" + draft + "/1/description," + "x," + strings.Repeat("a", 2001) + ",\n" +
		"not a key,x,y,\n"
	asset := e.uploadDocument(ctx, t, a.designer, "tr.csv", "text/csv", []byte(csv))
	pubBefore := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+pubID+"/versions/1", a.designer, "", nil).raw
	// dry run: a report, no write.
	d := e.adminCall(t, http.MethodPost, "/game-admin/translations:import", a.designer, "", map[string]any{"asset_id": asset, "format": "csv",
		"target_language": "en", "dry_run": true})
	if d.status != http.StatusAccepted || d.body["dry_run"] != true {
		t.Fatalf("dry run: %d %s", d.status, d.raw)
	}
	e.runJobs(ctx, t)
	dj := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+d.body["job_id"].(string), a.designer, "", nil)
	if dj.body["status"] != "succeeded" || dj.body["report"].(map[string]any)["updated"] != float64(3) {
		t.Fatalf("dry run report: %s", dj.raw)
	}
	if v := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+draft+"/versions/1", a.designer, "", nil); v.body["translation_status"] != "partial" {
		t.Fatalf("dry run wrote: %s", v.raw)
	}
	// real import.
	r := e.adminCall(t, http.MethodPost, "/game-admin/translations:import", a.designer, "", map[string]any{"asset_id": asset, "format": "csv",
		"target_language": "en"})
	e.runJobs(ctx, t)
	j := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+r.body["job_id"].(string), a.designer, "", nil)
	rep := j.body["report"].(map[string]any)
	if j.body["status"] != "succeeded" || rep["updated"] != float64(3) || rep["total"] != float64(9) || rep["skipped"] != float64(6) {
		t.Fatalf("import report: %s", j.raw)
	}
	codes := map[string]int{}
	for _, x := range rep["errors"].([]any) {
		codes[fmt.Sprint(x.(map[string]any)["code"])]++
	}
	for _, c := range []string{"version_in_review", "key_not_found", "unsupported_type", "too_long"} {
		if codes[c] == 0 {
			t.Errorf("no %s in %v", c, codes)
		}
	}
	// the draft is written in place (version 1, complete); the published version is unchanged and a new draft v2 holds the import.
	v := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+draft+"/versions/1", a.designer, "", nil)
	if v.body["translation_status"] != "complete" || v.body["payload"].(map[string]any)["name"].(map[string]any)["en"] != "Draft name" {
		t.Fatalf("draft: %s", v.raw)
	}
	if e.adminCall(t, http.MethodGet, "/game-admin/quests/"+pubID+"/versions/1", a.designer, "", nil).raw != pubBefore {
		t.Fatal("a published version changed (R11)")
	}
	v2 := e.adminCall(t, http.MethodGet, "/game-admin/quests/"+pubID+"/versions/2", a.designer, "", nil)
	if v2.status != http.StatusOK || v2.body["status"] != "draft" || v2.body["based_on_version"] != float64(1) ||
		v2.body["payload"].(map[string]any)["name"].(map[string]any)["en"] != "Market maths" {
		t.Fatalf("new draft: %d %s", v2.status, v2.raw)
	}
	// importing the same file again: the published item's lines are now stale (version 1 < newest 2).
	r2 := e.adminCall(t, http.MethodPost, "/game-admin/translations:import", a.designer, "", map[string]any{"asset_id": asset, "format": "csv",
		"target_language": "en"})
	e.runJobs(ctx, t)
	j2 := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+r2.body["job_id"].(string), a.designer, "", nil)
	if !strings.Contains(j2.raw, "stale_version") {
		t.Fatalf("stale: %s", j2.raw)
	}
	// a broken XLIFF fails the job with a message; an asset that is not a document is 422.
	bad := e.uploadDocument(ctx, t, a.designer, "bad.xlf", "application/xliff+xml", []byte(`<?xml version="1.0"?><!DOCTYPE x [<!ENTITY e "x">]><xliff/>`))
	rb := e.adminCall(t, http.MethodPost, "/game-admin/translations:import", a.designer, "", map[string]any{"asset_id": bad, "format": "xliff",
		"target_language": "en"})
	e.runJobs(ctx, t)
	jb := e.adminCall(t, http.MethodGet, "/game-admin/translations/jobs/"+rb.body["job_id"].(string), a.designer, "", nil)
	if jb.body["status"] != "failed" || jb.body["error_message"] == nil {
		t.Fatalf("broken file: %s", jb.raw)
	}
	if r := e.adminCall(t, http.MethodPost, "/game-admin/translations:import", a.designer, "", map[string]any{"asset_id": uuid.NewString(), "format": "csv",
		"target_language": "en"}); r.status != 422 {
		t.Fatalf("unknown asset: %d %s", r.status, r.raw)
	}
	// a document needs game.content.translate (the reviewer has none).
	if r := e.adminCall(t, http.MethodPost, "/game-media:presign-upload", a.reviewer, "", map[string]any{"kind": "document", "filename": "a.csv",
		"mime_type": "text/csv", "size_bytes": 3, "sha256": sha([]byte("abc")), "license": "internal"}); r.status != http.StatusForbidden {
		t.Fatalf("reviewer document: %d", r.status)
	}
	if n := e.audits(ctx, t, command.ActionTranslationImported); n < 3 {
		t.Fatalf("audits: %d", n)
	}
	_ = json.Valid
	_ = i18ntext.Complete
}
