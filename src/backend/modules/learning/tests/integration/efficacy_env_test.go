//go:build integration

package integration

import (
	"context"
	"crypto/md5" //nolint:gosec // stable unit id of the test, same derivation as SQL md5()
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// F035 demo principals (quickstart §1): a student of grade 3 of the second school, a safety moderator and a second System Admin token
// (password only like every portal token since ADR-0094).
const (
	hsChi      = "hs.chi.3b"
	smAntoan   = "sm.antoan"
	saPwOnly   = "sa.pwonly"
	agreements = "/admin/learning/research-agreements"
	studies    = "/admin/learning/efficacy-studies"
)

// effKit is the efficacy kit (ADR-0095: cohorts school × grade): the LMS kit of tenant A (school with hs.an, hs.binh in
// grade 3), a second school of tenant A with hs.chi in grade 3, a school of tenant B with one grade-3 student, two
// published platform efficacy forms of three questions each on the LO of the kit.
type effKit struct {
	asgKit
	school2, schoolOther uuid.UUID
	qA, qB               []string // efficacy question versions of forms A and B
	formA, formB         string   // published blueprint versions
}

// tenantHdr is X-Tenant-Id of a System Admin write into a tenant.
func tenantHdr(t uuid.UUID) hdr { return hdr{"X-Tenant-Id": t.String()} }

// efficacyQuestion creates and publishes a platform efficacy instrument of the LO of qc; it returns its version id.
func (c *curEnv) efficacyQuestion(ctx context.Context, t *testing.T, qc qbCurriculum) string {
	t.Helper()
	r := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "single_choice"), map[string]any{"owner_scope": "platform",
		"efficacy_instrument": true, "exam_only": true})), http.StatusCreated, "efficacy question")
	return c.publishQuestion(ctx, t, r.str("content_id"), 1)
}

// efficacyBlueprint is a platform efficacy blueprint over fixed questions (one point each).
func efficacyBlueprint(qc qbCurriculum, qs []string) map[string]any {
	ids := make([]any, len(qs))
	for i, q := range qs {
		ids[i] = q
	}
	b := blueprintBody(qc, float64(len(qs)), []any{map[string]any{"fixed_question_version_ids": ids, "count": len(qs), "points_each": 1}},
		map[string]any{"strategy": "fixed"})
	b["efficacy_instrument"] = true
	return b
}

// publishForm creates and publishes an efficacy blueprint as cm.noidung; it returns the version id.
func (c *curEnv) publishForm(ctx context.Context, t *testing.T, qc qbCurriculum, qs []string) string {
	t.Helper()
	id := must(t, c.do(t, http.MethodPost, blueprintsPath, cmNoidung, idem(), efficacyBlueprint(qc, qs)), http.StatusCreated, "form").str("content_id")
	return c.publishDoc(ctx, t, "assessment_blueprints", "assessment_blueprint_versions", "assessment_blueprint_id", id, 1)
}

func (c *curEnv) effSetup(ctx context.Context, t *testing.T) effKit {
	t.Helper()
	k := effKit{asgKit: c.asgSetup(ctx, t), school2: uuid.New(), schoolOther: uuid.New()}
	c.user(hsChi, authztest.Student(c.tenantA).PrincipalData, false)
	c.user(smAntoan, authztest.Platform("safety_moderator").PrincipalData, true)
	d := authztest.Platform("system_admin").PrincipalData
	c.user(saPwOnly, d, true)
	c.tokens[saPwOnly] = c.idp.Token(saPwOnly, httpxtest.PortalClient, time.Now())
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.schools (school_id, tenant_id, status, source_version)
		VALUES ($1, $2, 'active', 1), ($3, $4, 'active', 1) ON CONFLICT DO NOTHING`, k.school2, c.tenantA, k.schoolOther, c.tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
		VALUES ($1, $2, $3, 3, 1)`, uuid.New(), c.tenantB, k.schoolOther); err != nil {
		t.Fatal(err)
	}
	c.placeIn(ctx, t, hsChi, k.school2, 3)
	for range 3 {
		k.qA = append(k.qA, c.efficacyQuestion(ctx, t, k.s.qc))
		k.qB = append(k.qB, c.efficacyQuestion(ctx, t, k.s.qc))
	}
	k.formA, k.formB = c.publishForm(ctx, t, k.s.qc, k.qA), c.publishForm(ctx, t, k.s.qc, k.qB)
	return k
}

// agreement records an agreement of tenant covering what.
func (c *curEnv) agreement(t *testing.T, tenant uuid.UUID, ref string, covers ...string) resp {
	t.Helper()
	return must(t, c.do(t, http.MethodPost, agreements, saAdmin, merge(idem(), tenantHdr(tenant)), map[string]any{"agreement_ref": ref,
		"covers": covers, "agreed_on": "2026-09-01"}), http.StatusCreated, "agreement")
}

func (k effKit) studyBody(code string) map[string]any {
	return map[string]any{"code": code, "title": "Toán 3 – học kỳ I", "subject_id": k.s.qc.subject, "grade_level": 3,
		"objective_ids": []string{k.s.qc.lo}, "form_a_blueprint_version_id": k.formA, "form_b_blueprint_version_id": k.formB,
		"min_intervention_weeks": 8}
}

// activeStudy creates a study with the pair of cohorts school × grade 3 — school2 × grade 3 (random arms) after the
// agreement of tenant A and activates it.
func (c *curEnv) activeStudy(t *testing.T, k effKit, code string) string {
	t.Helper()
	c.agreement(t, c.tenantA, "HD-"+code, "efficacy_study", "experiment")
	study := must(t, c.do(t, http.MethodPost, studies, saAdmin, idem(), k.studyBody(code)), http.StatusCreated, "study").str("study_id")
	for _, sc := range []uuid.UUID{k.school, k.school2} {
		must(t, c.do(t, http.MethodPost, studies+"/"+study+"/cohorts", saAdmin, merge(idem(), tenantHdr(c.tenantA)),
			map[string]any{"school_id": sc, "pair_no": 1}), http.StatusCreated, "cohort")
	}
	must(t, c.do(t, http.MethodPost, studies+"/"+study+":activate", saAdmin, idem(), nil), http.StatusOK, "activate")
	return study
}

// wave schedules a wave of kind starting in a minute for an hour (offset after the test clock).
func (c *curEnv) wave(t *testing.T, study, kind string, offset time.Duration, extra map[string]any) resp {
	t.Helper()
	start := c.clk.Now().Add(offset + time.Minute)
	b := map[string]any{"kind": kind, "window_start": start.Format(time.RFC3339), "window_end": start.Add(time.Hour).Format(time.RFC3339)}
	for key, v := range extra {
		b[key] = v
	}
	return c.do(t, http.MethodPost, studies+"/"+study+"/waves", saAdmin, idem(), b)
}

// runWaves runs the job efficacy-wave-monitor once with the test clock.
func (c *curEnv) runWaves(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := c.eff.RunWaveMonitor(ctx, 0); err != nil {
		t.Fatalf("efficacy-wave-monitor: %v", err)
	}
}

// sessionOfCohort returns the exam session of a cohort (school) in a wave response.
func sessionOfCohort(t *testing.T, w resp, school uuid.UUID) string {
	t.Helper()
	for _, cl := range w.items("cohorts") {
		if cl["school_id"] == school.String() {
			return cl["exam_session_id"].(string)
		}
	}
	t.Fatalf("no session for cohort %s: %s", school, w.raw)
	return ""
}

// realRegistry is the resource registry of module.go (questions, documents, curriculum; media assign-only).
func (c *curEnv) realRegistry(t *testing.T) *port.Registry {
	t.Helper()
	reg, err := port.NewRegistry(append([]port.ResourceHandler{command.CurriculumResource{Store: write.CurriculumRepo{}, Trash: write.CurriculumTrash{}},
		command.QuestionResource{Store: write.QuestionRepo{}}}, command.DocResources(write.DocRepo{}, write.QuestionRepo{})...)...)
	if err == nil {
		err = reg.RegisterAssignable(command.MediaResource{Store: write.MediaRepo{}})
	}
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// cohortUnit is the unit id of an experiment with unit school_grade (ADR-0095): md5("<school_id>:<grade>") as a UUID, the
// derivation of ExperimentRepo.StudentCohort.
func cohortUnit(school uuid.UUID, grade int) uuid.UUID {
	return uuid.UUID(md5.Sum([]byte(school.String() + ":" + strconv.Itoa(grade))))
}
