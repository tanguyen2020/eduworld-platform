package devseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The assignment seed of the repository is valid, keeps the dev markers and only targets th-demo-a/DEMOA1 khối 3
// (nothing for pgd-demo, so the isolation check of F062 T018 stays meaningful).
func TestAssignmentSeedFileValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "seed", "assignments", "demoa1_g3.dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f AssignmentFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, a := range f.Assignments {
		if a.GradeLevel != 3 || !strings.HasPrefix(a.Title, "[DEV]") {
			t.Fatalf("unexpected assignment %+v", a)
		}
		kinds[a.Content] = true
	}
	if f.TenantCode != "th-demo-a" || f.SchoolCode != "DEMOA1" || !kinds["activity"] || !kinds["question_set"] {
		t.Fatalf("unexpected seed file %+v", f)
	}
	bad := f
	bad.Assignments = append([]AssignmentSeed{}, f.Assignments...)
	bad.Assignments = append(bad.Assignments, f.Assignments[0])
	if bad.Validate() == nil {
		t.Fatal("duplicate title accepted")
	}
	bad.Assignments = []AssignmentSeed{{Title: "x", Content: "assessment_blueprint", Mode: "practice", GradeLevel: 3, MaxAttempts: 1}}
	if bad.Validate() == nil {
		t.Fatal("content kind outside the content seed accepted")
	}
}

type fixedTenant string

func (f fixedTenant) Tenants(context.Context) ([]string, error) { return []string{string(f)}, nil }

// MOCK (TEST FAKE) of the public LMS API: the first run creates both assignments as drafts and publishes them with the
// ETag of the create answer, every call carries the tenant of the seed; a second run finds them by title and changes
// nothing; a draft left by an interrupted run is published with the ETag of a GET.
func TestRunAssignmentsIsIdempotent(t *testing.T) {
	var mu sync.Mutex
	tenant, school := uuid.NewString(), uuid.NewString()
	type stored struct {
		body   map[string]any
		status string
	}
	store := map[string]*stored{}
	var order []string
	creates, publishes := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("X-Tenant-Id") != tenant || r.Header.Get("Authorization") != "Bearer tok-ad" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		list := func() []map[string]any {
			out := []map[string]any{}
			for _, id := range order {
				s := store[id]
				out = append(out, map[string]any{"assignment_id": id, "title": s.body["title"], "status": s.status})
			}
			return out
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/catalog/schools":
			_, _ = w.Write([]byte(`{"items":[{"school_id":"` + school + `","code":"DEMOA1"}],"page":{"has_more":false}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/lms/assignments":
			if r.URL.Query().Get("school_id") != school || r.URL.Query().Get("grade_level") != "3" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": list()})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/lms/assignments":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if r.Header.Get("Idempotency-Key") == "" || in["school_id"] != school || in["subject_id"] != "subj" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			id := uuid.NewString()
			store[id] = &stored{body: in, status: "draft"}
			order = append(order, id)
			creates++
			w.Header().Set("ETag", `"v1-`+id+`"`)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"assignment_id": id, "title": in["title"], "status": "draft"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/lms/assignments/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/lms/assignments/")
			w.Header().Set("ETag", `"v1-`+id+`"`)
			_ = json.NewEncoder(w).Encode(map[string]any{"assignment_id": id, "status": store[id].status})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":publish"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/lms/assignments/"), ":publish")
			if r.Header.Get("If-Match") != `"v1-`+id+`"` || store[id] == nil || store[id].status != "draft" {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			store[id].status = "open"
			publishes++
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	now := time.Date(2026, 11, 24, 7, 30, 0, 0, time.UTC)
	c := AssignmentClient{BaseURL: srv.URL + "/api/v1", HTTP: srv.Client(), Token: "tok-ad", Now: func() time.Time { return now }}
	f := AssignmentFile{TenantCode: "th-demo-a", SchoolCode: "DEMOA1", Author: "ad.truong.a", WindowDays: 365, Assignments: []AssignmentSeed{
		{Title: "[DEV] A", Content: "activity", Mode: "practice", GradeLevel: 3, MaxAttempts: 3},
		{Title: "[DEV] Q", Content: "question_set", Mode: "practice", GradeLevel: 3, MaxAttempts: 3},
	}}
	res := Result{SubjectID: "subj", Activity: uuid.New(), QuestionSet: uuid.New()}
	first, err := RunAssignments(ctx, c, fixedTenant(tenant), f, res)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 2 || first.Published != 2 || first.SchoolID != school || len(first.Assignments) != 2 {
		t.Fatalf("first run %+v", first)
	}
	a := store[first.Assignments[0].String()].body
	ref, _ := a["content_ref"].(map[string]any)
	targets, _ := a["targets"].([]any)
	if ref["content_type"] != "activity" || ref["content_id"] != res.Activity.String() || len(targets) != 1 ||
		a["open_at"] != "2026-11-24T06:30:00Z" || a["due_at"] != "2027-11-24T06:30:00Z" {
		t.Fatalf("activity assignment body %v", a)
	}
	q, _ := store[first.Assignments[1].String()].body["content_ref"].(map[string]any)
	if q["content_type"] != "question_set" || q["content_id"] != res.QuestionSet.String() {
		t.Fatalf("question set assignment ref %v", q)
	}
	second, err := RunAssignments(ctx, c, fixedTenant(tenant), f, res)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created != 0 || second.Published != 0 || creates != 2 || publishes != 2 || second.Assignments[0] != first.Assignments[0] {
		t.Fatalf("second run changed something: %+v (creates %d, publishes %d)", second, creates, publishes)
	}
	// A draft left by an interrupted run is published with the ETag of a GET.
	store[first.Assignments[1].String()].status = "draft"
	third, err := RunAssignments(ctx, c, fixedTenant(tenant), f, res)
	if err != nil || third.Created != 0 || third.Published != 1 {
		t.Fatalf("third run %+v %v", third, err)
	}
	if _, err := RunAssignments(ctx, c, fixedTenant(tenant), f, Result{}); err == nil {
		t.Fatal("missing content result accepted")
	}
}
