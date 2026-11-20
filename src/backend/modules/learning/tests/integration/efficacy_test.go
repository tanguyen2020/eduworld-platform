//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F035-AC-01…AC-05 (US1, SC-001): efficacy instruments are platform, exam_only, referenced by efficacy blueprints only,
// never selected, never assigned, never an ordinary exam session.
func TestEfficacyInstruments(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	qc := k.s.qc
	// AC-01: not exam_only, tenant scope → 422 on /efficacy_instrument.
	for name, x := range map[string]resp{
		"not exam_only": c.createQ(t, cmNoidung, qc.body(qbFixture(t, "single_choice"), map[string]any{"owner_scope": "platform",
			"efficacy_instrument": true})),
		"tenant": c.createQ(t, edA, qc.body(qbFixture(t, "single_choice"), map[string]any{"efficacy_instrument": true, "exam_only": true})),
	} {
		if x.status != http.StatusUnprocessableEntity || !slices.Contains(x.errorCodes(), "/efficacy_instrument:INVALID") {
			t.Fatalf("%s: %d %s", name, x.status, x.raw)
		}
	}
	// AC-03: a static set, an ordinary blueprint referencing an instrument → 422; an efficacy blueprint over an ordinary
	// question, with a drawn rule or independent variants → 422.
	if x := c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(k.qA[0]))); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("set: %d %s", x.status, x.raw)
	}
	ordinary := efficacyBlueprint(qc, k.qA)
	delete(ordinary, "efficacy_instrument")
	if x := c.do(t, http.MethodPost, blueprintsPath, cmNoidung, idem(), ordinary); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("ordinary blueprint: %d %s", x.status, x.raw)
	}
	_, plain := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform", "exam_only": true})
	if x := c.do(t, http.MethodPost, blueprintsPath, cmNoidung, idem(), efficacyBlueprint(qc, []string{plain})); x.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("efficacy blueprint, ordinary question: %d %s", x.status, x.raw)
	}
	drawn := blueprintBody(qc, 3, []any{map[string]any{"objective_ids": []string{qc.lo}, "count": 3, "points_each": 1}},
		map[string]any{"strategy": "fixed"})
	drawn["efficacy_instrument"] = true
	if x := c.do(t, http.MethodPost, blueprintsPath, cmNoidung, idem(), drawn); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("drawn rule: %d %s", x.status, x.raw)
	}
	indep := efficacyBlueprint(qc, k.qA)
	indep["definition"].(map[string]any)["variants"] = map[string]any{"strategy": "independent_draw", "count": 2}
	if x := c.do(t, http.MethodPost, blueprintsPath, cmNoidung, idem(), indep); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("independent_draw: %d %s", x.status, x.raw)
	}
	// AC-04: an ordinary exam session on a form → 422; applyTenantAssignments of a form or an instrument → 422.
	body := c.sessionBody(examKit{asgKit: k.asgKit, version: k.formA}, nil)
	if x := c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), body); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("createExamSession on a form: %d %s", x.status, x.raw)
	}
	reg := c.realRegistry(t)
	deps := command.AssignmentDeps{UoW: c.uow, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: reg,
		Audit: write.Auditor{R: c.rec}, Metrics: observability.AssignmentCounter{}}
	f := facade.Facade{Apply: command.ApplyTenantAssignments{AssignmentDeps: deps}, Idempotency: idempotency.NewStore(c.app, 0, nil)}
	sctx, _ := systemAdmin(ctx)
	var formItem, qItem uuid.UUID
	if err := c.platform.QueryRowContext(ctx, `SELECT assessment_blueprint_id FROM content.assessment_blueprint_versions
		WHERE assessment_blueprint_version_id = $1`, k.formA).Scan(&formItem); err != nil {
		t.Fatal(err)
	}
	if err := c.platform.QueryRowContext(ctx, `SELECT question_id FROM content.question_versions WHERE question_version_id = $1`, k.qA[0]).
		Scan(&qItem); err != nil {
		t.Fatal(err)
	}
	for rt, id := range map[string]uuid.UUID{"assessment_blueprint": formItem, "question": qItem} {
		_, err := apply(sctx, f, newKey(), rt, id, c.tenantA)
		if pe, ok := problemAs(err); !ok || pe.Code != problem.ValidationFailed {
			t.Fatalf("assign %s: %v", rt, err)
		}
	}
	// AC-02, SC-001: the pool of tenant A (practice, retry_wrong, diagnostic, dynamic sets, blueprints of every kind draw
	// from it) never holds an instrument; 10 000 selections never pick one.
	delivery := query.Delivery{Session: c.read, Reader: read.Delivery{}, Registry: question.Full(), Schema: c.schema}
	sc := tenancy.WithScope(authztest.Ctx(ctx, authztest.Staff(c.tenantA, "academic_manager")), scopeOf(t, c.tenantA))
	subject := uuid.MustParse(qc.subject)
	pool, err := delivery.Pool(sc, &subject)
	if err != nil {
		t.Fatal(err)
	}
	instruments := append(slices.Clone(k.qA), k.qB...)
	for _, cand := range pool {
		if slices.Contains(instruments, cand.VersionID.String()) {
			t.Fatalf("instrument %s in the pool", cand.VersionID)
		}
	}
	if len(pool) == 0 {
		t.Fatal("empty pool")
	}
	lo := uuid.MustParse(qc.lo)
	for seed := range uint64(10_000) {
		got, err := selection.Select(pool, selection.Request{Rule: selection.Rule{Filters: selection.Filters{ObjectiveIDs: []uuid.UUID{lo}},
			Count: 1, AllowExamOnly: true}, Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range got {
			if slices.Contains(instruments, x.VersionID.String()) {
				t.Fatalf("seed %d selected an instrument", seed)
			}
		}
	}
	// AC-05: instruments are exam_only: never on the public CDN (cdnEligible), and the render payload of a delivered item
	// carries no answer key (checked on the exam paper of TestEfficacyWaves).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = ANY($1::uuid[])
		AND (NOT exam_only OR NOT efficacy_instrument OR tenant_id IS NOT NULL)`, "{"+joinIDs(instruments)+"}"); n != 0 {
		t.Fatalf("instruments not platform exam_only: %d", n)
	}
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

func problemAs(err error) (*problem.Error, bool) {
	var pe *problem.Error
	ok := errors.As(err, &pe)
	return pe, ok
}

// F035-AC-06…AC-11 (US2): agreements, the study, its classes, the activation; authorization and cross-tenant lists.
func TestEfficacyStudy(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	authztest.Cover(t, "listResearchAgreements", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createResearchAgreement", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "revokeResearchAgreement", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listEfficacyStudies", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createEfficacyStudy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getEfficacyStudy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateEfficacyStudy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listEfficacyStudyClasses", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "addEfficacyStudyClass", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "removeEfficacyStudyClass", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "activateEfficacyStudy", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "completeEfficacyStudy", authztest.CrossTenant, authztest.RoleDenied)
	// AC-06: X-Tenant-Id required; agreement recorded and audited; the same reference twice → 409.
	if x := c.do(t, http.MethodPost, agreements, saAdmin, idem(), map[string]any{"agreement_ref": "HD-01", "covers": []string{"efficacy_study"},
		"agreed_on": "2026-09-01"}); x.status != http.StatusBadRequest || x.str("code") != "TENANT_HEADER_REQUIRED" {
		t.Fatalf("no header: %d %s", x.status, x.raw)
	}
	a := c.agreement(t, c.tenantA, "HD-01", "efficacy_study")
	if a.str("tenant_id") != c.tenantA.String() || a.str("status") != "active" || c.audits(ctx, t, "research_agreement.created", &c.tenantA) != 1 {
		t.Fatalf("agreement %s", a.raw)
	}
	if x := c.do(t, http.MethodPost, agreements, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"agreement_ref": "HD-01",
		"covers": []string{"experiment"}, "agreed_on": "2026-09-01"}); x.status != http.StatusConflict {
		t.Fatalf("duplicate ref: %d %s", x.status, x.raw)
	}
	// AC-10: tenant roles, content_admin, safety_moderator → 403 PERMISSION_DENIED; no MFA → 403 MFA_REQUIRED.
	for _, sub := range []string{adA, qlcmA, cmNoidung, smAntoan} {
		if x := c.do(t, http.MethodGet, studies, sub, nil, nil); x.status != http.StatusForbidden || x.str("code") != "PERMISSION_DENIED" {
			t.Fatalf("%s list: %d %s", sub, x.status, x.raw)
		}
		if x := c.do(t, http.MethodPost, studies, sub, idem(), k.studyBody("x")); x.status != http.StatusForbidden {
			t.Fatalf("%s create: %d %s", sub, x.status, x.raw)
		}
	}
	if x := c.do(t, http.MethodGet, studies, saNoMFA, nil, nil); x.status != http.StatusForbidden || x.str("code") != "MFA_REQUIRED" {
		t.Fatalf("no MFA: %d %s", x.status, x.raw)
	}
	// AC-07: invalid study → 422 naming the field; idempotent creation; code unique.
	bad := k.studyBody("math-g3")
	bad["form_b_blueprint_version_id"] = k.formA
	if x := c.do(t, http.MethodPost, studies, saAdmin, idem(), bad); x.status != http.StatusUnprocessableEntity ||
		!slices.Contains(x.errorCodes(), "/form_b_blueprint_version_id:INVALID") {
		t.Fatalf("same forms: %d %s", x.status, x.raw)
	}
	other := k.studyBody("math-g3")
	other["objective_ids"] = []string{k.s.qc.lo2}
	if x := c.do(t, http.MethodPost, studies, saAdmin, idem(), other); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("forms outside the objectives: %d %s", x.status, x.raw)
	}
	key := idem()
	s := must(t, c.do(t, http.MethodPost, studies, saAdmin, key, k.studyBody("math-g3")), http.StatusCreated, "study")
	if again := must(t, c.do(t, http.MethodPost, studies, saAdmin, key, k.studyBody("math-g3")), http.StatusCreated, "replay"); again.str("study_id") !=
		s.str("study_id") || s.str("status") != "draft" {
		t.Fatalf("replay %s / %s", again.raw, s.raw)
	}
	if x := c.do(t, http.MethodPost, studies, saAdmin, idem(), k.studyBody("math-g3")); x.status != http.StatusConflict {
		t.Fatalf("code taken: %d %s", x.status, x.raw)
	}
	if c.audits(ctx, t, "efficacy_study.created", nil) != 1 {
		t.Fatal("audit efficacy_study.created")
	}
	study := s.str("study_id")
	classes := studies + "/" + study + "/classes"
	// AC-08: a class of tenant B under the header of tenant A → 404; tenant B without agreement → 422; added once (409 twice).
	if x := c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.classOther,
		"pair_no": 1}); x.status != http.StatusNotFound {
		t.Fatalf("class of tenant B: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantB)), map[string]any{"class_id": k.classOther,
		"pair_no": 2}); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("no agreement: %d %s", x.status, x.raw)
	}
	must(t, c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.class, "pair_no": 1}),
		http.StatusCreated, "3A")
	if x := c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.class, "pair_no": 1}); x.status !=
		http.StatusConflict {
		t.Fatalf("3A twice: %d %s", x.status, x.raw)
	}
	s2 := must(t, c.do(t, http.MethodPost, studies, saAdmin, idem(), k.studyBody("math-g3-b")), http.StatusCreated, "study 2").str("study_id")
	if x := c.do(t, http.MethodPost, studies+"/"+s2+"/classes", saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.class,
		"pair_no": 1}); x.status != http.StatusConflict {
		t.Fatalf("3A in another study: %d %s", x.status, x.raw)
	}
	// Remove then add 3B again (soft-delete, revived).
	must(t, c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.classB, "pair_no": 1}),
		http.StatusCreated, "3B")
	must(t, c.do(t, http.MethodDelete, classes+"/"+k.classB.String(), saAdmin, tenantHdr(c.tenantA), nil), http.StatusNoContent, "remove 3B")
	must(t, c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": k.classB, "pair_no": 1}),
		http.StatusCreated, "3B again")
	// AC-09: activation assigns one intervention and one comparison, a_then_b for pair 1; reproducible; locked afterwards.
	act := must(t, c.do(t, http.MethodPost, studies+"/"+study+":activate", saAdmin, idem(), nil), http.StatusOK, "activate")
	if act.str("status") != "active" || act.num("class_count") != 2 || act.num("pair_count") != 1 {
		t.Fatalf("activated %s", act.raw)
	}
	list := must(t, c.do(t, http.MethodGet, classes, saAdmin, nil, nil), http.StatusOK, "classes")
	arms := map[string]string{}
	for _, cl := range list.items("items") {
		arms[cl["arm"].(string)] = cl["class_id"].(string)
		if cl["form_order"] != "a_then_b" || cl["assignment_method"] != "random" || cl["tenant_id"] != c.tenantA.String() || cl["class_name"] == "" {
			t.Fatalf("class %v", cl)
		}
	}
	if len(arms) != 2 || arms["intervention"] == "" || arms["comparison"] == "" {
		t.Fatalf("arms %v", arms)
	}
	if x := c.do(t, http.MethodPost, studies+"/"+study+":activate", saAdmin, idem(), nil); x.status != http.StatusUnprocessableEntity ||
		x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("activate again: %d %s", x.status, x.raw)
	}
	g := must(t, c.do(t, http.MethodGet, studies+"/"+study, saAdmin, nil, nil), http.StatusOK, "get")
	if x := c.do(t, http.MethodPatch, studies+"/"+study, saAdmin, ifm(g.header.Get("ETag")), map[string]any{"title": "Khác"}); x.status !=
		http.StatusUnprocessableEntity || x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("patch active: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, classes, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"class_id": uuid.New(), "pair_no": 2}); x.status !=
		http.StatusUnprocessableEntity || x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("add after activation: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodDelete, classes+"/"+k.class.String(), saAdmin, tenantHdr(c.tenantA), nil); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("remove after activation: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, studies+"/"+study+":complete", saAdmin, idem(), nil); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("complete before post: %d %s", x.status, x.raw)
	}
	// Draft edits keep the checks (If-Match).
	d2 := must(t, c.do(t, http.MethodGet, studies+"/"+s2, saAdmin, nil, nil), http.StatusOK, "draft")
	if p := must(t, c.do(t, http.MethodPatch, studies+"/"+s2, saAdmin, ifm(d2.header.Get("ETag")), map[string]any{"title": "Toán 3 – nhóm 2"}),
		http.StatusOK, "patch draft"); p.str("title") != "Toán 3 – nhóm 2" {
		t.Fatalf("patch %s", p.raw)
	}
	// AC-11: lists across tenants take tenant_ids (CrossTenantScope, audit cross_tenant.accessed); agreements require them.
	if x := c.do(t, http.MethodGet, agreements, saAdmin, nil, nil); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("agreements without tenant_ids: %d %s", x.status, x.raw)
	}
	before := c.audits(ctx, t, "cross_tenant.accessed", nil)
	al := must(t, c.do(t, http.MethodGet, agreements+"?tenant_ids="+c.tenantA.String()+","+c.tenantB.String(), saAdmin, nil, nil), http.StatusOK,
		"agreements")
	if items := al.items("items"); len(items) != 1 || items[0]["tenant_id"] != c.tenantA.String() {
		t.Fatalf("agreements %s", al.raw)
	}
	sl := must(t, c.do(t, http.MethodGet, studies+"?tenant_ids="+c.tenantB.String(), saAdmin, nil, nil), http.StatusOK, "studies of B")
	if len(sl.items("items")) != 0 {
		t.Fatalf("studies of B %s", sl.raw)
	}
	sa := must(t, c.do(t, http.MethodGet, studies+"?status=active", saAdmin, nil, nil), http.StatusOK, "active studies")
	if items := sa.items("items"); len(items) != 1 || items[0]["study_id"] != study {
		t.Fatalf("active studies %s", sa.raw)
	}
	if after := c.audits(ctx, t, "cross_tenant.accessed", nil); after <= before {
		t.Fatalf("cross_tenant.accessed %d → %d", before, after)
	}
	// AC-20 (US4): revoking the agreement withdraws the classes of the tenant (here before any wave).
	must(t, c.do(t, http.MethodPost, agreements+"/"+a.str("agreement_id")+":revoke", saAdmin, merge(idem(), tenantHdr(c.tenantA)),
		map[string]any{"reason": "Trường rút khỏi pilot theo văn bản"}), http.StatusOK, "revoke")
	if x := c.do(t, http.MethodPost, agreements+"/"+a.str("agreement_id")+":revoke", saAdmin, merge(idem(), tenantHdr(c.tenantB)),
		map[string]any{"reason": "Trường rút khỏi pilot theo văn bản"}); x.status != http.StatusNotFound {
		t.Fatalf("revoke under tenant B: %d %s", x.status, x.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_study_classes WHERE study_id = $1 AND status = 'withdrawn'`,
		study); n != 2 {
		t.Fatalf("withdrawn classes %d", n)
	}
}

// F035-AC-12…AC-17 (US3, SC-002): the waves create fixed efficacy exam sessions; the participants are snapshotted at the
// pre wave; an efficacy attempt changes no mastery, progress, outbox, recommendation; results, papers, report and
// regrade stay locked; students never learn the session is an efficacy test.
func TestEfficacyWaves(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	authztest.Cover(t, "listEfficacyWaves", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "scheduleEfficacyWave", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "withdrawEfficacyParticipants", authztest.CrossTenant, authztest.RoleDenied)
	study := c.activeStudy(t, k, "math-g3-w")
	// AC-13: post before pre, validation without form or without validation class → refused.
	if x := c.wave(t, study, "post", 0, nil); x.status != http.StatusUnprocessableEntity || x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("post first: %d %s", x.status, x.raw)
	}
	if x := c.wave(t, study, "validation", 0, nil); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("validation without form: %d %s", x.status, x.raw)
	}
	// AC-12: one session per class, fixed settings, form by form_order; idempotent.
	key := idem()
	start := c.clk.Now().Add(time.Minute)
	pb := map[string]any{"kind": "pre", "window_start": start.Format(time.RFC3339), "window_end": start.Add(time.Hour).Format(time.RFC3339)}
	pre := must(t, c.do(t, http.MethodPost, studies+"/"+study+"/waves", saAdmin, key, pb), http.StatusCreated, "pre")
	if again := must(t, c.do(t, http.MethodPost, studies+"/"+study+"/waves", saAdmin, key, pb), http.StatusCreated, "replay"); again.str("wave_id") !=
		pre.str("wave_id") {
		t.Fatalf("replay %s", again.raw)
	}
	if len(pre.items("classes")) != 2 || pre.str("status") != "scheduled" {
		t.Fatalf("pre %s", pre.raw)
	}
	for _, cl := range pre.items("classes") {
		if cl["form"] != "a" { // pair 1: a_then_b
			t.Fatalf("form %v", cl)
		}
	}
	if c.audits(ctx, t, "efficacy_wave.scheduled", nil) != 1 {
		t.Fatal("audit efficacy_wave.scheduled")
	}
	sessA := sessionOfClass(t, pre, k.class)
	sv := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+sessA, adA, nil, nil), http.StatusOK, "staff view")
	if sv.body["is_efficacy"] != true || sv.str("feedback_timing") != "never" || sv.str("result_release") != "manual" ||
		sv.num("max_attempts") != 1 || sv.body["is_official"] != false || sv.str("type") != "custom" || sv.body["show_correct_answers"] != false ||
		sv.body["show_explanations"] != false || sv.str("assessment_blueprint_version_id") != k.formA {
		t.Fatalf("session %s", sv.raw)
	}
	if x := c.wave(t, study, "pre", 0, nil); x.status != http.StatusUnprocessableEntity || x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("second pre: %d %s", x.status, x.raw)
	}
	if x := c.wave(t, study, "post", 7*24*time.Hour, nil); x.status != http.StatusUnprocessableEntity ||
		!slices.Contains(x.errorCodes(), "/window_start:INVALID") {
		t.Fatalf("early post: %d %s", x.status, x.raw)
	}
	// AC-17: the student view says nothing of the study; another tenant → 404.
	c.clk.Advance(2 * time.Minute)
	stu := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+sessA, hsAn, nil, nil), http.StatusOK, "student view")
	for _, f := range []string{"is_efficacy", "efficacy_wave_id", "study_id", "arm", "form_order"} {
		if _, ok := stu.body[f]; ok {
			t.Fatalf("student view has %s: %s", f, stu.raw)
		}
	}
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+sessA, adB, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", x.status, x.raw)
	}
	// AC-14: the pre wave opens: hs.an, hs.binh (3A) and hs.chi (3B) become participants with the arm of their class.
	c.runWaves(ctx, t)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_participants p JOIN learning.efficacy_study_classes c
		ON c.study_id = p.study_id AND c.class_id = p.class_id AND c.arm = p.arm WHERE p.study_id = $1 AND p.status = 'active'`, study); n != 3 {
		t.Fatalf("participants %d", n)
	}
	// AC-15, SC-002: hs.an takes the test; nothing but learning events with efficacy_wave_id.
	a := must(t, c.startExam(t, hsAn, sessA, devA, nil), http.StatusCreated, "start")
	for _, it := range a.items("items") {
		if _, ok := it["answer_key"]; ok {
			t.Fatalf("answer key delivered: %v", it) // AC-05
		}
	}
	attempt := a.str("attempt_id")
	sub := must(t, c.answerExam(t, hsAn, a, devA, true, idem()), http.StatusOK, "submit")
	_ = sub
	if st := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND status = 'graded'`, attempt); st != 1 {
		t.Fatal("efficacy attempt not graded at submit")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE envelope->'payload'->>'attempt_id' = $1
		OR (event_type LIKE 'exam.%' AND envelope->'payload'->>'exam_session_id' = $2)`, attempt, sessA); n != 0 {
		t.Fatalf("outbox events of the efficacy attempt: %d", n)
	}
	student := c.users[hsAn].UserID
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.objective_masteries WHERE student_id = $1`, student); n != 0 {
		t.Fatalf("mastery rows %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.progress_daily WHERE student_id = $1`, student); n != 0 {
		t.Fatalf("progress rows %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE student_id = $1 AND payload->>'attempt_id' = $2
		AND efficacy_wave_id IS NULL`, student, attempt); n != 0 {
		t.Fatalf("learning events without efficacy_wave_id: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE student_id = $1 AND payload->>'attempt_id' = $2
		AND event_type = 'question.answered' AND efficacy_wave_id = $3`, student, attempt, pre.str("wave_id")); n != 3 {
		t.Fatalf("question.answered with efficacy_wave_id: %d", n)
	}
	// The recompute of mastery skips the efficacy events (second layer).
	var evidence int
	if err := c.uow.Do(tenancy.WithScope(ctx, scopeOf(t, c.tenantA)), func(ctx context.Context) error {
		rows, err := write.MasteryRepo{}.ObjectiveEvidence(ctx, student, uuid.MustParse(k.s.qc.lo))
		evidence = len(rows)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if evidence != 0 {
		t.Fatalf("recompute evidence %d", evidence)
	}
	// AC-16: result, release, papers, report and regrade stay locked.
	if x := c.do(t, http.MethodGet, "/exams/attempts/"+attempt+"/result", hsAn, nil, nil); x.status != http.StatusForbidden ||
		x.str("code") != "EXAM_RESULTS_NOT_RELEASED" {
		t.Fatalf("result: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodGet, "/exams/attempts/"+attempt+"/analysis", hsAn, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("analysis: %d %s", x.status, x.raw)
	}
	if x := c.do(t, http.MethodPost, examSessionsPath+"/"+sessA+":release-results", qlcmA, idem(), nil); x.status != http.StatusUnprocessableEntity ||
		x.str("code") != "INVALID_STATE_TRANSITION" {
		t.Fatalf("release: %d %s", x.status, x.raw)
	}
	for name, path := range map[string]string{"papers": examSessionsPath + "/" + sessA + "/papers", "report": examSessionsPath + "/" + sessA + "/report"} {
		if x := c.do(t, http.MethodGet, path, qlcmA, nil, nil); x.status != http.StatusForbidden {
			t.Fatalf("%s: %d %s", name, x.status, x.raw)
		}
	}
	if x := c.do(t, http.MethodPost, regradePath, qlcmA, idem(), map[string]any{"scope": "exam_session", "exam_session_id": sessA,
		"question_version_id": k.qA[0], "action": "drop_item", "reason": "Đáp án câu 1 bị sai trong đề"}); x.status != http.StatusForbidden {
		t.Fatalf("regrade: %d %s", x.status, x.raw)
	}
	// listEfficacyWaves: counts only (1 invited done in 3A), never a score.
	wl := must(t, c.do(t, http.MethodGet, studies+"/"+study+"/waves", saAdmin, nil, nil), http.StatusOK, "waves")
	ws := wl.items("items")
	if len(ws) != 1 || ws[0]["status"] != "open" {
		t.Fatalf("waves %s", wl.raw)
	}
	for _, cl := range ws[0]["classes"].([]any) {
		m := cl.(map[string]any)
		if m["class_id"] == k.class.String() && (m["invited_count"] != float64(2) || m["completed_count"] != float64(1)) {
			t.Fatalf("3A counts %v", m)
		}
		if _, ok := m["score"]; ok {
			t.Fatal("score in the wave list")
		}
	}
	// The window ends: the wave closes; the post wave (8 weeks later) targets the active participants.
	c.clk.Advance(2 * time.Hour)
	c.runExamWindows(ctx, t)
	c.runWaves(ctx, t)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_waves WHERE wave_id = $1 AND status = 'closed'`, pre.str("wave_id")); n != 1 {
		t.Fatal("pre wave not closed")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type LIKE 'exam.%' AND envelope->'payload'->>'exam_session_id'
		= ANY($1::uuid[]::text[])`, "{"+sessA+","+sessionOfClass(t, pre, k.classB)+"}"); n != 0 {
		t.Fatalf("exam.* events of the efficacy sessions: %d", n)
	}
	c.clk.Advance(8 * 7 * 24 * time.Hour)
	post := must(t, c.wave(t, study, "post", 0, nil), http.StatusCreated, "post")
	for _, cl := range post.items("classes") {
		if cl["form"] != "b" {
			t.Fatalf("post form %v", cl)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1`,
		sessionOfClass(t, post, k.class)); n != 2 {
		t.Fatalf("post audience of 3A: %d", n)
	}
}
