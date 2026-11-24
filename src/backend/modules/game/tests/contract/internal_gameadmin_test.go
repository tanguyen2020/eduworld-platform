package contract

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	portread "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// gameInternalSchema loads a schema of specs/openapi/modules/game.internal.v1.yaml (external refs to common allowed).
func gameInternalSchema(t *testing.T, name string) *openapi3.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
	l := openapi3.NewLoader()
	l.IsExternalRefsAllowed = true
	doc, err := l.LoadFromFile(filepath.Join(root, "specs", "openapi", "modules", "game.internal.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := doc.Components.Schemas[name]
	if s == nil || s.Value == nil {
		t.Fatalf("no schema %s", name)
	}
	return s.Value
}

// fakeCatalog is a TEST DOUBLE (R17) of the catalog read model behind listAssignedGameActivities.
type fakeCatalog struct{ rows []portread.AssignedActivity }

func (f fakeCatalog) Assigned(_ context.Context, _ uuid.UUID, _ string, _ int) (query.AssignedPage, error) {
	next := "c1"
	return query.AssignedPage{Data: f.rows, HasMore: true, NextCursor: &next, Limit: 20}, nil
}

func (fakeCatalog) ThumbnailURL(sha *string) string {
	if sha == nil {
		return ""
	}
	return "https://cdn.test/ew-media-public/media/" + *sha + "/render_thumbnail.png"
}

// F078-AC-23 (receiving side, AGENTS R29): listAssignedGameActivities answers GameActivityCatalogPage of the internal spec
// (bilingual title, grades, subject, objectives, minutes, thumbnail) from the published payload of a quest and a
// mini-game config.
func TestListAssignedGameActivitiesFacade(t *testing.T) {
	quest, _ := json.Marshal(map[string]any{"name": map[string]any{"vi": "Phiên chợ", "en": "Market day"}, "grade_levels": []int{3},
		"subject_id": "0193a5f4-6c1e-7b2a-9d1f-0000000720f1", "objective_ids": []string{"0193a5f4-6c1e-7b2a-9d1f-0000000a0001"}, "estimated_minutes": 15})
	mg, _ := json.Marshal(map[string]any{"name": map[string]any{"vi": "Ghép thẻ"}, "grade_levels": []int{3, 4}})
	sha := "9f2c4b1d8e7a6f5c4b3a29180716f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4"
	now := time.Now().UTC()
	f := facade.GameAdmin{Catalog: fakeCatalog{rows: []portread.AssignedActivity{
		{ItemType: "minigame_config", ConfigID: uuid.New(), VersionID: uuid.New(), VersionNo: 2, Payload: mg, PublishedAt: &now},
		{ItemType: "quest", ConfigID: uuid.New(), VersionID: uuid.New(), VersionNo: 1, Payload: quest, PublishedAt: &now, ThumbnailSHA: &sha}}}}
	res, err := f.ListAssignedGameActivities(context.Background(), api.ListAssignedGameActivitiesRequestObject{
		Params: api.ListAssignedGameActivitiesParams{TenantId: uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	out := res.(api.ListAssignedGameActivities200JSONResponse)
	b, _ := json.Marshal(out)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := gameInternalSchema(t, "GameActivityCatalogPage").VisitJSON(doc); err != nil {
		t.Fatalf("GameActivityCatalogPage: %v\n%s", err, b)
	}
	q := out.Data[1]
	if q.Title.Vi != "Phiên chợ" || q.Title.En == nil || *q.Title.En != "Market day" || q.EstimatedMinutes == nil || *q.EstimatedMinutes != 15 ||
		q.ThumbnailUrl == nil || q.SubjectId == nil || out.Data[0].ThumbnailUrl != nil || !out.Page.HasMore {
		t.Fatalf("catalog: %s", b)
	}
}

// fakeAssignments is a TEST DOUBLE (R17) of the assignment command behind the facade.
type fakeAssignments struct{}

func (fakeAssignments) Apply(_ context.Context, rt string, id uuid.UUID, tenants []uuid.UUID) ([]command.AssignmentView, error) {
	out := []command.AssignmentView{}
	for _, tn := range tenants {
		out = append(out, command.AssignmentView{AssignmentID: uuid.New(), ResourceType: rt, ResourceID: id, TenantID: tn, AssignedBy: uuid.New(),
			AssignedAt: time.Now().UTC(), Status: "active"})
	}
	return out, nil
}

func (f fakeAssignments) Revoke(ctx context.Context, rt string, id uuid.UUID, tenants []uuid.UUID, _ string) ([]command.AssignmentView, error) {
	out, _ := f.Apply(ctx, rt, id, tenants)
	at := time.Now().UTC()
	for i := range out {
		out[i].Status, out[i].RevokedAt = "revoked", &at
	}
	return out, nil
}

func (f fakeAssignments) List(ctx context.Context, rt string, id uuid.UUID, _ string, _ int) (command.AssignmentPage, error) {
	out, _ := f.Apply(ctx, rt, id, []uuid.UUID{uuid.New()})
	return command.AssignmentPage{Data: out, Page: page.Info{Limit: 20}}, nil
}

// F078-AC-21 (receiving side): applyTenantAssignments/revokeTenantAssignments/listResourceAssignments answer
// TenantAssignmentList of the shared platform-ops library.
func TestTenantAssignmentsFacade(t *testing.T) {
	f := facade.GameAdmin{Assignments: fakeAssignments{}}
	schema := gameInternalSchema(t, "GameActivityCatalogPage") // loads the document (common refs resolved)
	_ = schema
	ta := uuid.New()
	ap, err := f.ApplyTenantAssignments(context.Background(), api.ApplyTenantAssignmentsRequestObject{Body: &api.ApplyTenantAssignmentsJSONRequestBody{
		ResourceType: "quest", ResourceId: uuid.New(), TenantIds: []uuid.UUID{ta}}})
	if err != nil || len(ap.(api.ApplyTenantAssignments200JSONResponse).Data) != 1 || ap.(api.ApplyTenantAssignments200JSONResponse).Data[0].TenantId != ta {
		t.Fatalf("apply: %+v %v", ap, err)
	}
	rv, err := f.RevokeTenantAssignments(context.Background(), api.RevokeTenantAssignmentsRequestObject{Body: &api.RevokeTenantAssignmentsJSONRequestBody{
		ResourceType: "quest", ResourceId: uuid.New(), TenantIds: []uuid.UUID{ta}, Reason: "Hết học kỳ"}})
	if err != nil || rv.(api.RevokeTenantAssignments200JSONResponse).Data[0].Status != "revoked" {
		t.Fatalf("revoke: %+v %v", rv, err)
	}
	if _, err := f.ApplyTenantAssignments(context.Background(), api.ApplyTenantAssignmentsRequestObject{}); err == nil {
		t.Fatal("empty body accepted")
	}
}

// F078-AC-07 (caller side, F076 handoff): submit_review asks checkPublishedLearningContent of the learning facade with a
// LearningContentCheck valid against the internal spec, in process (caller game), and maps the answer in order.
func TestCheckPublishedLearningContentFacade(t *testing.T) {
	var got learningapi.LearningContentCheck
	var caller string
	fake := &learningapi.Fake{CheckPublishedLearningContentFunc: func(ctx context.Context, req learningapi.CheckPublishedLearningContentRequestObject) (
		learningapi.CheckPublishedLearningContentResponseObject, error) {
		got, caller = *req.Body, modcall.CallerFrom(ctx)
		out := learningapi.CheckPublishedLearningContent200JSONResponse{}
		for i, r := range req.Body.Refs {
			out.Results = append(out.Results, struct {
				ContentId   uuid.UUID                                                `json:"content_id"`
				ContentType learningapi.LearningContentCheckResultResultsContentType `json:"content_type"`
				Published   bool                                                     `json:"published"`
			}{ContentId: r.ContentId, ContentType: learningapi.LearningContentCheckResultResultsContentType(r.ContentType), Published: i == 0})
		}
		return out, nil
	}}
	v := 2
	refs := []port.LearningRef{{ContentType: "activity", ContentID: uuid.New(), Version: &v}, {ContentType: "question_set", ContentID: uuid.New()}}
	ok, err := modules.LearningAttempts{F: fake}.PublishedContent(context.Background(), refs)
	if err != nil || len(ok) != 2 || !ok[0] || ok[1] || caller != "game" {
		t.Fatalf("check: %v %v %s", ok, err, caller)
	}
	b, _ := json.Marshal(got)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := internalSchema(t, "LearningContentCheck").VisitJSON(doc); err != nil {
		t.Fatalf("LearningContentCheck: %v\n%s", err, b)
	}
	if _, err := (modules.LearningAttempts{}).PublishedContent(context.Background(), refs); err == nil {
		t.Fatal("not wired must fail")
	}
}
