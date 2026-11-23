//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/examquality"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/examsession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/i18n"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/importer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// memAssets is a TEST DOUBLE of port.AssetReader (media store of F027, not implemented yet): files by asset id in
// memory. Never used by a binary (R17).
type memAssets struct {
	mu    sync.Mutex
	files map[uuid.UUID][]byte
	sizes map[uuid.UUID]int64 // overrides the reported size (csv_import_max test)
}

func (m *memAssets) put(b []byte) uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	m.files[id] = b
	return id
}

func (m *memAssets) Open(_ context.Context, id uuid.UUID) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[id]
	if !ok {
		return nil, 0, port.ErrAssetNotFound
	}
	size := int64(len(b))
	if s, ok := m.sizes[id]; ok {
		size = s
	}
	return io.NopCloser(bytes.NewReader(b)), size, nil
}

// curEnv is the HTTP environment of the curriculum tests: the real pkg/httpx chain (auth, tenant, guards, OpenAPI
// validation, idempotency) over the use cases of F021, with demo principals of tenants A and B.
type curEnv struct {
	*env
	srv     *httptest.Server
	idp     *httpxtest.IDP
	res     *httpxtest.Resolver
	cmd     command.Curriculum
	qry     query.Curriculum
	qcmd    command.Questions
	dcmd    command.Docs
	wf      command.Workflow
	rel     command.Releases
	schema  *qbschema.Validator
	assets  *memAssets
	users   map[string]authz.PrincipalData
	tokens  map[string]string
	media   *mediaKit               // F027 (newCurEnv(..., withMedia))
	att     command.Attempts        // F030
	attq    query.Attempts          // F030
	clk     *testClock              // F030
	lessons command.Lessons         // F031 US6
	rgd     command.Regrades        // F031 US4
	recs    command.Recommendations // F032 US6
	asg     command.Assignments     // F041
	asgq    query.SchoolAssignments // F041
	exams   command.Exams           // F042
	examq   query.Exams             // F042
	dashq   *query.Dashboards       // F043: the dashboard reads (role learning_dashboard)
	reports *command.Reports        // F044: exports (worker run by the tests)
	quality *command.Quality        // F044: item analysis, calibration, exam quality, reference levels
	reportq *query.Reports          // F044 reads
	eff     *command.Efficacy       // F035: the efficacy study (jobs run by the tests)
	exps    *command.Experiments    // F035 US5: the A/B experiments
}

// envOpt configures newCurEnv.
type envOpt func(*envOpts)

type envOpts struct {
	media      bool
	rate       command.RateLimiter // F030 T053
	answerRate command.AnswerRate
	noExams    bool // F041-AC-09: the "unavailable" ExamSessionCreator of F041 instead of the F042 adapter
}

// withoutExams wires the "unavailable" ExamSessionCreator of F041 (graded + blueprint → 422, F041-AC-09).
func withoutExams(o *envOpts) { o.noExams = true }

// withMedia adds the media pipeline and the preview sessions of F027 (floci, Valkey, clamd test double).
func withMedia(o *envOpts) { o.media = true }

// withAnswerRate limits answers and submits with r and the values cfg (F030 T053).
func withAnswerRate(r command.RateLimiter, cfg command.AnswerRate) envOpt {
	return func(o *envOpts) { o.rate, o.answerRate = r, cfg }
}

// Demo principals (quickstart §1): content admin, system admin, content reviewer (platform); academic manager and
// school admin of tenants A and B; a staff member without role; a student of tenant A.
const (
	cmNoidung = "cm.noidung"
	saAdmin   = "sa.admin"
	cmDuyet   = "cm.duyet"
	qlcmA     = "qlcm.truong.a"
	qlcmB     = "qlcm.truong.b"
	adA       = "ad.truong.a"
	adB       = "ad.truong.b"
	noRoleA   = "staff.norole.a"
	hsAn      = "hs.an.3a"
	edA       = "bt.noidung.a" // tenant_content_editor of tenant A (media.upload, F027)
	edB       = "bt.noidung.b" // tenant_content_editor of tenant B
)

func newCurEnv(ctx context.Context, t *testing.T, opts ...envOpt) *curEnv {
	t.Helper()
	var o envOpts
	for _, f := range opts {
		f(&o)
	}
	e := setup(ctx, t, true)
	authz.Configure(authz.Hooks{Recorder: e.rec})
	t.Cleanup(func() { authz.Configure(authz.Hooks{}) })
	idp := httpxtest.NewIDP(t)
	res := &httpxtest.Resolver{}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Spec: spec,
		Idempotency: idempotency.NewStore(e.app, 0, nil)})
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	reg, err := port.NewRegistry(append([]port.ResourceHandler{command.CurriculumResource{Store: write.CurriculumRepo{}, Trash: write.CurriculumTrash{}},
		command.QuestionResource{Store: write.QuestionRepo{}}}, command.DocResources(write.DocRepo{}, write.QuestionRepo{})...)...)
	if err != nil {
		t.Fatal(err)
	}
	assets := &memAssets{files: map[uuid.UUID][]byte{}, sizes: map[uuid.UUID]int64{}}
	cmd := command.Curriculum{UoW: e.uow, Store: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Imports: write.ImportJobRepo{},
		Resources: reg, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Assets: assets, Codec: importer.Codec{},
		Metrics: observability.CurriculumCounter{}}
	qry := query.Curriculum{Session: e.read, Model: read.CurriculumModel{}, Settings: read.SchoolSettings{}, Codec: importer.Codec{}, Cursors: codec}
	schema, err := qbschema.New()
	if err != nil {
		t.Fatal(err)
	}
	qcmd := command.Questions{UoW: e.uow, Store: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{}, Catalog: write.CatalogRepo{}, Schema: schema,
		Registry: question.Full(), Resources: reg, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Docs: write.DocRepo{}}
	qqry := query.Questions{Session: e.read, Model: read.QuestionModel{}, Cursors: codec}
	docSchema, err := qbschema.NewDocs()
	if err != nil {
		t.Fatal(err)
	}
	actSchema, err := qbschema.NewActivities()
	if err != nil {
		t.Fatal(err)
	}
	registry := plugins.Registry()
	dcmd := command.Docs{UoW: e.uow, Store: write.DocRepo{}, Questions: write.QuestionRepo{}, Curriculum: write.CurriculumRepo{}, Catalog: write.CatalogRepo{},
		Schema: docSchema, Read: e.read, Resources: reg, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec},
		Activities: registry, ActivitySchema: actSchema, ActivityEnabled: true, Media: write.MediaRepo{}, Minigames: write.MinigameConfigRepo{}}
	dqry := query.Docs{Session: e.read, Model: read.DocModel{}, Pool: read.Delivery{}, Cursors: codec}
	wf := command.Workflow{UoW: e.uow, Status: write.WorkflowRepo{}, Reviews: write.WorkflowRepo{}, Questions: write.QuestionRepo{}, Docs: write.DocRepo{},
		Schema: schema, Media: write.MediaRepo{}, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Metrics: observability.WorkflowCounter{}}
	handlers := learninghttp.Handlers{Commands: cmd, Queries: qry, QuestionCommands: qcmd, QuestionQueries: qqry, Docs: dcmd, DocQueries: dqry,
		Widgets: query.QuestionWidgets{Widgets: question.Full().Widgets()}, Workflow: wf,
		Reviews: query.ReviewTasks{Session: e.read, Model: read.ReviewTaskModel{}, Cursors: codec}}
	var mk *mediaKit
	if o.media {
		mk = newMediaKit(ctx, t, e, codec, schema)
		qcmd.Math, qcmd.Media = &mk.math, write.MediaRepo{}
		dcmd.Media, dcmd.Math = write.MediaRepo{}, &mk.math
		handlers.QuestionCommands, handlers.Docs = qcmd, dcmd
		handlers.Media, handlers.MediaQueries, handlers.Preview = mk.cmd, mk.qry, mk.preview
		if err := mk.s3.EnsureBucket(ctx, storage.BucketExports); err != nil {
			t.Fatal(err)
		}
	}
	rel := command.Releases{UoW: e.uow, Workflow: wf, Store: write.ReleaseRepo{}, Schedules: write.ReleaseRepo{}, Events: write.Publisher{W: e.writer},
		Audit: write.Auditor{R: e.rec}, Metrics: observability.ReleaseCounter{}, Activities: registry, ActivitySchema: actSchema,
		Minigames: write.MinigameConfigRepo{}}
	reader := query.ContentReader{Questions: read.QuestionModel{}, Docs: read.DocModel{}}
	if mk != nil {
		rel.Cache, rel.CDN, rel.Signer, reader.Cache = mk.cache, mk.s3, mk.signer, mk.cache
	}
	handlers.Releases = rel
	handlers.ReleaseQueries = query.Releases{Session: e.read, Model: read.ReleaseModel{}, Reader: reader, Cursors: codec}
	handlers.CmsActivities = query.CmsActivities{Docs: dqry}
	handlers.ActivityTypes = query.ActivityTypes{Registry: registry}
	// F030: the attempt engine with an adjustable clock (TEST clock, never in a binary).
	clk := &testClock{t: time.Now().UTC()}
	delivery := query.Delivery{Session: e.read, Reader: read.Delivery{}, Registry: question.Full(), Schema: schema}
	att := command.Attempts{UoW: e.uow, Store: write.AttemptRepo{}, Learning: write.LearningEventWriter{}, Events: write.Publisher{W: e.writer},
		Docs: write.DocRepo{}, Questions: write.QuestionRepo{}, Delivery: delivery, Students: read.SchoolSettings{}, Registry: registry,
		Clock: clk.Now, Metrics: observability.AttemptCounter{}, Rate: o.rate, AnswerRate: o.answerRate,
		// F065: media activities — progress, branch choices, ready check and signed URLs (TEST signer key).
		Progress: write.MediaProgressRepo{}, Media: write.MediaRepo{}, Signer: testSigner, MediaMetrics: observability.VideoCounter{},
		Runs: write.SimulationRunRepo{}, SimMetrics: observability.SimulationCounter{}, Minigames: write.MinigameConfigRepo{}}
	// F031 US6: lesson sessions pinned at the start of an attempt in the context of a lesson.
	lessons := command.Lessons{Store: write.LessonRepo{}, Docs: write.DocRepo{}, Learning: write.LearningEventWriter{},
		Events: write.Publisher{W: e.writer}, UoW: e.uow, Clock: clk.Now}
	att.Lessons = lessons
	// F032: review list, rapid_guess, adaptive selection, diagnostics and recommendations with the test clock.
	self, practice := write.SelfStudyRepo{}, command.DefaultPracticeConfig()
	att.Self = &command.SelfStudy{Wrong: self, Recommendations: self, Reader: self, Masteries: write.MasteryRepo{}, Config: practice,
		Metrics: observability.SelfStudyCounter{}}
	recs := command.Recommendations{UoW: e.uow, Store: self, Wrong: self, Reader: self, Masteries: write.MasteryRepo{}, Delivery: delivery,
		Students: read.SchoolSettings{}, Events: write.Publisher{W: e.writer}, Engine: recommendation.Default(), Config: practice, Clock: clk.Now,
		Metrics: observability.SelfStudyCounter{}}
	// F041: school assignments with the test clock; F042: the exam engine and its ExamSessionCreator adapter.
	asg := command.Assignments{UoW: e.uow, Store: write.SchoolAssignmentRepo{}, Read: e.read, Exams: examsession.Unavailable{},
		Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec}, Clock: clk.Now, Metrics: observability.SchoolAssignmentCounter{}}
	exams := command.Exams{UoW: e.uow, Store: write.ExamRepo{}, Events: write.Publisher{W: e.writer}, Audit: write.Auditor{R: e.rec},
		Instruments: write.EfficacyRepo{},
		Clock:       clk.Now, Metrics: observability.ExamCounter{}}
	att.ExamRelease = exams.ReleaseOf
	att.EfficacyWave = exams.WaveOf
	exps := &command.Experiments{UoW: e.uow, Store: write.ExperimentRepo{}, Efficacy: write.EfficacyRepo{}, Mastery: write.MasteryRepo{},
		Evidence: write.MasteryRepo{}, Audit: write.Auditor{R: e.rec}, Clock: clk.Now, Config: command.DefaultExperimentConfig()}
	att.Arms = exps.ArmsOf
	att.Experience, recs.Experience = exps, exps // F035-AC-24
	if !o.noExams {
		asg.Exams = &command.ExamSessionCreator{Exams: &exams}
	}
	att.Gate = asg
	exams.Asg, exams.Att = asg, att
	recs.Sources = append(recs.Sources, command.ExamFollowups{Store: write.ExamRepo{}, Clock: clk.Now})
	recs.Engine.Register(recommendation.R5{})
	recs.Sources = append(recs.Sources, command.AssignmentsDue{Store: write.SchoolAssignmentRepo{}})
	recs.Engine.Register(recommendation.R1{})
	asgq := query.SchoolAssignments{Session: e.read, Model: read.SchoolAssignmentModel{}, Cursors: codec, Clock: clk.Now}
	handlers.AssignmentCommands, handlers.Assignments = asg, asgq
	// F021 FR-CUR-008: the curriculum of a school (moved from F040, ADR-0095).
	handlers.SchoolCurriculum = command.SchoolCurriculum{UoW: e.uow, Store: write.CurriculumSettingsRepo{}, Read: e.read, Audit: write.Auditor{R: e.rec},
		Clock: clk.Now}
	handlers.SchoolCurriculumQueries = query.SchoolCurriculum{Session: e.read, Model: read.CurriculumSettingsModel{}}
	handlers.Hints = att
	handlers.SelfStudy = query.SelfStudy{Session: e.read, Model: read.SelfStudyModel{}, Reasons: i18n.MustReasons(), Cursors: codec, Clock: clk.Now}
	handlers.Mastery = query.Progress{Session: e.read, Model: read.ProgressModel{}, Details: read.SelfStudyModel{}, Clock: clk.Now, Arms: read.ExperimentModel{}}
	handlers.Lessons, handlers.LessonQueries = lessons, query.Lessons{Session: e.read, Model: read.LessonModel{}}
	attq := query.Attempts{Session: e.read, Model: read.AttemptModel{}, Docs: read.DocModel{}, Delivery: delivery, Registry: registry, Cursors: codec,
		Clock: clk.Now, Media: read.MediaModel{}, Progress: read.MediaProgressModel{}, Signer: testSigner, Assignments: asgq, Runs: read.SimulationRunModel{},
		Recommendations: read.SelfStudyModel{}, Minigames: read.MinigameConfigModel{}, Students: read.SchoolSettings{}}
	handlers.Attempts, handlers.AttemptQueries = att, attq
	examq := query.Exams{Session: e.read, Model: read.ExamModel{}, Attempts: attq, Cursors: codec, Clock: clk.Now}
	handlers.ExamCommands, handlers.ExamQueries = exams, examq
	// F031 US3: progress reads with the test clock (decay) and the profile-view audit.
	handlers.Progress = query.Progress{Session: e.read, Model: read.ProgressModel{}, Details: read.SelfStudyModel{}, Clock: clk.Now,
		Arms: read.ExperimentModel{}}
	handlers.ProfileAudit = command.ProfileViews{UoW: e.uow, Audit: write.Auditor{R: e.rec}}
	// F043: dashboards on their own session (role learning_dashboard; learning_platform for System Admin cross-tenant reads).
	dashRead, err := entx.NewReadSession("learning", entx.DBs{App: open(t, e.cl.ModuleDSN("learning", "dashboard")),
		Platform: open(t, e.cl.ModuleDSN("learning", "platform"))})
	if err != nil {
		t.Fatal(err)
	}
	dashq := &query.Dashboards{Session: dashRead, Model: read.DashboardModel{}, Config: analytics.DefaultConfig(), Cursors: codec,
		Clock: clk.Now}
	handlers.Dashboards = dashProxy{q: dashq}
	// F044: exports on the dashboard session, quality use cases, the balancer of independent_draw (pointers: tests tune them).
	reports, quality, reportq := f044Kit(e, dashRead, mk, qcmd, codec, clk)
	handlers.Reports = learninghttp.ReportHandlers{Commands: reportsProxy{reports}, Calibration: qualityProxy{quality}, Queries: reportqProxy{reportq}}
	exams.Balancer, exams.PValues = examquality.Balancer{}, write.QualityRepo{}
	handlers.ExamCommands = exams
	// F035: the efficacy study with the test clock.
	eff := &command.Efficacy{UoW: e.uow, Store: write.EfficacyRepo{}, Content: write.EfficacyRepo{}, Exams: &exams, Exclusions: write.ExamRepo{}, Data: read.EfficacyModel{},
		Read: e.read, Audit: write.Auditor{R: e.rec}, Clock: clk.Now, Config: efficacy.DefaultConfig()}
	reports.Efficacy = eff
	eff.Experiments = exps
	handlers.Experiments = learninghttp.ExperimentHandlers{Commands: exps, Queries: query.Experiments{Session: e.read, Model: read.ExperimentModel{}, Cursors: codec}}
	handlers.Efficacy = learninghttp.EfficacyHandlers{Commands: eff, Queries: query.Efficacy{Session: e.read, Model: read.EfficacyModel{}, Cursors: codec}}
	// F031 US4: re-evaluation with the test clock.
	rgd := command.Regrades{UoW: e.uow, Store: write.RegradeRepo{}, Attempts: att, Audit: write.Auditor{R: e.rec}, Clock: clk.Now, Exams: exams}
	handlers.Regrades, handlers.RegradeQueries = rgd, query.Regrades{Session: e.read, Model: read.RegradeModel{}, Cursors: codec}
	cmd.Content = &command.ContentImport{Questions: qcmd, Codec: importer.QuestionCodec{}}
	handlers.Commands = cmd
	if err := learninghttp.Register(srv, handlers); err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	ce := &curEnv{env: e, srv: hs, idp: idp, res: res, cmd: cmd, qry: qry, qcmd: qcmd, dcmd: dcmd, wf: wf, rel: rel, schema: schema, assets: assets, users: map[string]authz.PrincipalData{},
		tokens: map[string]string{}, media: mk, att: att, attq: attq, clk: clk, rgd: rgd, lessons: lessons, recs: recs,
		asg: asg, asgq: asgq, exams: exams, examq: examq, dashq: dashq,
		reports: reports, quality: quality, reportq: reportq, eff: eff, exps: exps}
	school := uuid.New()
	ce.user(cmNoidung, authztest.Platform("content_admin").PrincipalData, true)
	ce.user(saAdmin, authztest.Platform("system_admin").PrincipalData, true)
	ce.user(cmDuyet, authztest.Platform("content_reviewer").PrincipalData, true)
	ce.user(qlcmA, authztest.Staff(e.tenantA, "academic_manager").PrincipalData, true)
	ce.user(qlcmB, authztest.Staff(e.tenantB, "academic_manager").PrincipalData, true)
	ce.user(adA, authztest.Staff(e.tenantA, "school_admin", authztest.AtSchool(school)).PrincipalData, true)
	ce.user(adB, authztest.Staff(e.tenantB, "school_admin").PrincipalData, true)
	ce.user(noRoleA, authztest.NoRole(e.tenantA).PrincipalData, true)
	ce.user(hsAn, authztest.Student(e.tenantA).PrincipalData, false)
	ce.user(edA, authztest.Staff(e.tenantA, "tenant_content_editor").PrincipalData, true)
	ce.user(edB, authztest.Staff(e.tenantB, "tenant_content_editor").PrincipalData, true)
	return ce
}

func (c *curEnv) user(sub string, d authz.PrincipalData, portal bool) {
	d.UserID = uuid.New()
	c.res.Set(sub, d)
	c.users[sub] = d
	azp := httpxtest.PortalClient
	if !portal {
		azp = httpxtest.GameClient
	}
	c.tokens[sub] = c.idp.Token(sub, azp, time.Now())
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r resp) str(path ...string) string {
	var cur any = r.body
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	if cur == nil {
		return ""
	}
	return fmt.Sprint(cur)
}

func (r resp) items(key string) []map[string]any {
	arr, _ := r.body[key].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (r resp) errorCodes() []string {
	var out []string
	for _, x := range r.items("errors") {
		out = append(out, fmt.Sprint(x["field"])+":"+fmt.Sprint(x["code"]))
	}
	return out
}

type hdr map[string]string

func idem() hdr { return hdr{"Idempotency-Key": ids.New().String()} }

func ifm(etag string) hdr { return hdr{"If-Match": etag} }

func merge(hs ...hdr) hdr {
	out := hdr{}
	for _, h := range hs {
		for k, v := range h {
			out[k] = v
		}
	}
	return out
}

// do sends a request as user sub (with X-Tenant-Id of its tenant unless h sets it) and decodes the JSON answer.
func (c *curEnv) do(t *testing.T, method, path, sub string, h hdr, body any) resp {
	t.Helper()
	var rd io.Reader = bytes.NewReader(nil)
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+"/api/v1"+path, rd)
	req.Header.Set("Authorization", "Bearer "+c.tokens[sub])
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch {
			ct = "application/merge-patch+json"
		}
		req.Header.Set("Content-Type", ct)
	}
	if d := c.users[sub]; d.TenantID != nil {
		req.Header.Set("X-Tenant-Id", d.TenantID.String())
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: string(b)}
	_ = json.Unmarshal(b, &out.body)
	return out
}

// must fails unless the answer has status want.
func must(t *testing.T, r resp, want int, what string) resp {
	t.Helper()
	if r.status != want {
		t.Fatalf("%s: status %d, want %d: %s", what, r.status, want, r.raw)
	}
	return r
}

// subject creates (or reuses) subject code with grades 1–9 as the content admin and returns its id.
func (c *curEnv) subject(t *testing.T, code string) string {
	t.Helper()
	r := c.do(t, http.MethodPost, "/cms/learning/subjects", cmNoidung, idem(), map[string]any{"code": code,
		"name": map[string]string{"vi": "Môn " + code}, "grade_levels": []int{1, 2, 3, 4, 5, 6, 7, 8, 9}})
	return must(t, r, http.StatusCreated, "create subject "+code).str("subject_id")
}

// curriculum creates a curriculum as sub and returns (curriculum_id, version 1 id).
func (c *curEnv) curriculum(t *testing.T, sub, code string) (string, string) {
	t.Helper()
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/curricula", sub, idem(), map[string]any{"code": code,
		"name": map[string]string{"vi": "Chương trình " + code}}), http.StatusCreated, "create curriculum "+code)
	return r.str("curriculum_id"), r.str("latest_version", "curriculum_version_id")
}

func (c *curEnv) topic(t *testing.T, sub, version, subject, code string, grade int, parent string) string {
	t.Helper()
	body := map[string]any{"curriculum_version_id": version, "code": code, "name": "Chủ đề " + code, "subject_id": subject, "grade_level": grade}
	if parent != "" {
		body["parent_topic_id"] = parent
	}
	return must(t, c.do(t, http.MethodPost, "/cms/learning/topics", sub, idem(), body), http.StatusCreated, "create topic "+code).str("topic_id")
}

func (c *curEnv) objective(t *testing.T, sub, topic, code string, extra map[string]any) resp {
	t.Helper()
	body := map[string]any{"topic_id": topic, "code": code, "description": "Học sinh có thể " + code}
	for k, v := range extra {
		body[k] = v
	}
	return c.do(t, http.MethodPost, "/cms/learning/objectives", sub, idem(), body)
}

// assign assigns a platform curriculum to tenant (as learning_platform, like the facade of F020).
func (c *curEnv) assign(ctx context.Context, t *testing.T, curriculumID string, tenant uuid.UUID) {
	t.Helper()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'curriculum', $3, $1, now())`, ids.New(), tenant, curriculumID); err != nil {
		t.Fatal(err)
	}
}

// versionETag reads the ETag of a version.
func (c *curEnv) versionETag(t *testing.T, sub, curriculumID string, version int) string {
	t.Helper()
	r := must(t, c.do(t, http.MethodGet, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d", curriculumID, version), sub, nil, nil),
		http.StatusOK, "get version")
	return r.header.Get("ETag")
}

// transition applies action to a version as sub.
func (c *curEnv) transition(t *testing.T, sub, curriculumID string, version int, action, note string) resp {
	t.Helper()
	tag := c.versionETag(t, saAdmin, curriculumID, version)
	if d := c.users[sub]; d.TenantID != nil {
		tag = c.versionETag(t, sub, curriculumID, version)
	}
	body := map[string]any{"action": action}
	if note != "" {
		body["note"] = note
	}
	return c.do(t, http.MethodPost, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d:transition", curriculumID, version), sub,
		merge(idem(), ifm(tag)), body)
}

// publishFlow submits (author), approves (System Admin) and publishes (System Admin) a version.
func (c *curEnv) publishFlow(t *testing.T, author, curriculumID string, version int) resp {
	t.Helper()
	must(t, c.transition(t, author, curriculumID, version, "submit_review", ""), http.StatusOK, "submit_review")
	approver := saAdmin
	if c.users[author].TenantID != nil {
		approver = author + ".reviewer"
		if _, ok := c.users[approver]; !ok {
			c.user(approver, authztest.Staff(*c.users[author].TenantID, "academic_manager").PrincipalData, true)
		}
	}
	must(t, c.transition(t, approver, curriculumID, version, "approve", ""), http.StatusOK, "approve")
	tag := c.versionETag(t, approver, curriculumID, version)
	return c.do(t, http.MethodPost, fmt.Sprintf("/cms/learning/curricula/%s/versions/%d:publish", curriculumID, version), approver,
		merge(idem(), ifm(tag)), nil)
}
