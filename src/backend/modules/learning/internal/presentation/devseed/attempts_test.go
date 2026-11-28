package devseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MOCK (TEST FAKE) of the school and attempt API: a student whose scripted attempts are all submitted (staff read API)
// is never logged in; the other one logs in once, resumes its open attempt, answers only the items not answered yet in
// position order (a rapid answer without waiting, the others after answer_wait_ms) with the scripted option and the
// game client headers, submits and logs out. A second run with everything submitted logs nobody in.
func TestRunAttemptsLogsInOnlyStudentsWithPendingAttempts(t *testing.T) {
	var mu sync.Mutex
	tenant, school, asgA, asgB, act := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	students := map[string]string{"hs.an.3a": uuid.NewString(), "hs.chi.3b": uuid.NewString()}
	done := map[string]map[string]bool{asgA: {students["hs.an.3a"]: true}, asgB: {students["hs.an.3a"]: true}}
	var answers []map[string]any
	submits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p := strings.TrimPrefix(r.URL.Path, "/api/v1")
		staff := r.Header.Get("Authorization") == "Bearer staff"
		student := r.Header.Get("Authorization") == "Bearer chi" && r.Header.Get("X-Client-Version") == "0.1.0" && r.Header.Get("X-Device-Id") != ""
		if r.Header.Get("X-Tenant-Id") != tenant || (!staff && !student) {
			http.Error(w, "auth", http.StatusForbidden)
			return
		}
		switch {
		case staff && p == "/catalog/schools":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"school_id": school, "code": "DEMOA1"}}})
		case staff && p == "/tenant/schools/"+school+"/grades/3/students":
			var data []any
			for u, id := range students {
				data = append(data, map[string]any{"username": u, "student_id": id})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": data, "page": map[string]any{"has_more": false}})
		case staff && p == "/lms/assignments":
			ref := map[string]any{"content_type": "activity", "content_id": act}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				map[string]any{"assignment_id": asgA, "title": "[DEV] A", "status": "open", "content_ref": ref},
				map[string]any{"assignment_id": asgB, "title": "[DEV] B", "status": "open", "content_ref": ref}}})
		case staff && strings.HasSuffix(p, "/submissions"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "/lms/assignments/"), "/submissions")
			var data []any
			for sid := range done[id] {
				data = append(data, map[string]any{"student_id": sid, "status": "graded"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "page": map[string]any{"has_more": false}})
		case student && p == "/learning/activities/"+act+"/attempts":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["mode"] != "assignment" || in["context_id"] != asgB {
				http.Error(w, "start", http.StatusUnprocessableEntity)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"attempt_id": "att", "status": "in_progress",
				"items":           []any{map[string]any{"position": 3, "question_version_id": "v3"}, map[string]any{"position": 1, "question_version_id": "v1"}, map[string]any{"position": 2, "question_version_id": "v2"}},
				"saved_responses": []any{map[string]any{"position": 2}}})
		case student && p == "/learning/activity-attempts/att/responses":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			answers = append(answers, in)
		case student && p == "/learning/activity-attempts/att:submit":
			submits++
			done[asgB][students["hs.chi.3b"]] = true
		default:
			http.Error(w, p, http.StatusNotFound)
		}
	}))
	defer srv.Close()
	f := AttemptFile{TenantCode: "th-demo-a", SchoolCode: "DEMOA1", GradeLevel: 3, Staff: "ad.truong.a", AnswerWaitMS: 4000,
		Students: []StudentScript{
			{Username: "hs.an.3a", Attempts: []AttemptScript{{Assignment: "[DEV] A", Answers: []AnswerScript{{Question: "q1", Option: "o1"}}},
				{Assignment: "[DEV] B", Answers: []AnswerScript{{Question: "q1", Option: "o1"}}}}},
			{Username: "hs.chi.3b", Attempts: []AttemptScript{{Assignment: "[DEV] B", Answers: []AnswerScript{
				{Question: "q1", Option: "o_fast", Rapid: true}, {Question: "q2", Option: "o2"}, {Question: "q3", Option: "o3"}}}}},
		}}
	lessons := LessonResult{Versions: map[string][]string{"q1": {"v1"}, "q2": {"v2"}, "q3": {"v3"}}}
	var logins []string
	logouts := 0
	var sleeps []time.Duration
	c := AttemptClient{BaseURL: srv.URL + "/api/v1", HTTP: srv.Client(), Staff: "staff",
		Login: func(_ context.Context, u string) (StudentSession, error) {
			logins = append(logins, u)
			return StudentSession{Token: "chi", TenantID: tenant, Headers: map[string]string{"X-Device-Id": "d", "X-Client-Version": "0.1.0"},
				Logout: func(context.Context) error { logouts++; return nil }}, nil
		},
		Sleep: func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }}
	ctx := context.Background()
	res, err := RunAttempts(ctx, c, fixedTenant(tenant), f, lessons)
	if err != nil {
		t.Fatal(err)
	}
	if res.Logins != 1 || len(logins) != 1 || logins[0] != "hs.chi.3b" || logouts != 1 || res.Submitted != 1 || res.Skipped != 2 || submits != 1 {
		t.Fatalf("first run: %+v logins=%v logouts=%d submits=%d", res, logins, logouts, submits)
	}
	if len(answers) != 2 || answers[0]["position"] != float64(1) || answers[0]["response"].(map[string]any)["option_id"] != "o_fast" ||
		answers[1]["position"] != float64(3) || answers[1]["response"].(map[string]any)["option_id"] != "o3" {
		t.Fatalf("answers: %v", answers)
	}
	if len(sleeps) != 1 || sleeps[0] <= 3*time.Second || sleeps[0] > 4*time.Second {
		t.Fatalf("sleeps: %v (only the non-rapid answer waits)", sleeps)
	}
	second, err := RunAttempts(ctx, c, fixedTenant(tenant), f, lessons)
	if err != nil || second.Logins != 0 || len(logins) != 1 || second.Skipped != 3 {
		t.Fatalf("second run: %+v %v logins=%v", second, err, logins)
	}
}

func TestGoldenDiff(t *testing.T) {
	three, half := 3, 0.5
	want := GoldenValues{ActiveStudents: &three, Accuracy: &half,
		Objectives: map[string]GoldenObjective{"O1": {StudentsWithEvidence: 3, LevelCounts: map[string]int{"beginning": 1, "developing": 2, "mastered": 0}}}}
	got := GoldenValues{ActiveStudents: &three, Accuracy: ptr(0.50001),
		Objectives: map[string]GoldenObjective{"O1": {StudentsWithEvidence: 3, LevelCounts: map[string]int{"beginning": 1, "developing": 2}}}}
	if d := want.Diff(got); len(d) != 0 {
		t.Fatalf("diff: %v", d)
	}
	got.Accuracy = ptr(0.6)
	got.Objectives = map[string]GoldenObjective{}
	if d := want.Diff(got); len(d) != 2 {
		t.Fatalf("diff: %v", d)
	}
	if codes := want.Codes(); len(codes) != 1 || codes[0] != "O1" {
		t.Fatal(codes)
	}
}

func ptr[T any](v T) *T { return &v }
