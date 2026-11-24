package unit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
)

type seedTemplate struct {
	ConfigID        uuid.UUID       `json:"config_id"`
	ConfigVersionID uuid.UUID       `json:"config_version_id"`
	ConfigType      string          `json:"config_type"`
	Code            string          `json:"code"`
	Title           string          `json:"title"`
	Payload         json.RawMessage `json:"payload"`
}

func competitionRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func seedTemplates(t *testing.T) []seedTemplate {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(competitionRoot(), "seed", "tournament_templates.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Templates []seedTemplate `json:"templates"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f.Templates
}

// template parses the seeded template code.
func template(t *testing.T, code string) challenge.Template {
	t.Helper()
	for _, s := range seedTemplates(t) {
		if s.Code == code {
			tpl, err := challenge.ParseTemplate(s.Payload)
			if err != nil {
				t.Fatalf("%s: %v", code, err)
			}
			return tpl
		}
	}
	t.Fatalf("no seeded template %s", code)
	return challenge.Template{}
}

// T002 (F107): the seed literals of the F107 migration equal seed/tournament_templates.json (one source, R33), and every
// seeded template is a valid template.
func TestF107SeedMatchesMigration(t *testing.T) {
	mig, err := os.ReadFile(filepath.Join(competitionRoot(), "migrations", "20261231000006_f107_tournament.sql"))
	if err != nil {
		t.Fatal(err)
	}
	block := string(mig)
	start, end := strings.Index(block, "-- BEGIN seed tournament_templates.json"), strings.Index(block, "-- END seed tournament_templates.json")
	if start < 0 || end < start {
		t.Fatal("no seed block")
	}
	block = block[start:end]
	for _, s := range seedTemplates(t) {
		i := strings.Index(block, "VALUES ('"+s.ConfigVersionID.String()+"'")
		if i < 0 {
			t.Fatalf("%s: no version row", s.Code)
		}
		rest := block[i:]
		j := strings.Index(rest, "'{")
		k := strings.Index(rest[j:], "}',")
		lit := strings.ReplaceAll(rest[j+1:j+k+1], "''", "'")
		var got, want any
		if err := json.Unmarshal([]byte(lit), &got); err != nil {
			t.Fatalf("%s literal: %v", s.Code, err)
		}
		if err := json.Unmarshal(s.Payload, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: the migration literal differs from the seed file", s.Code)
		}
		if !strings.Contains(block, "'"+s.ConfigType+"', '"+s.Code+"'") {
			t.Fatalf("%s: config row", s.Code)
		}
		if _, err := challenge.ParseTemplate(s.Payload); err != nil {
			t.Fatalf("%s: %v", s.Code, err)
		}
	}
}

func intp(v int) *int { return &v }

func request(grade int, listed []uuid.UUID, starts time.Time) challenge.Request {
	r := challenge.Request{Name: "Đấu trường Toán khối 3", SchoolID: uuid.New(), StartsAt: starts,
		Targets: []challenge.Target{{Type: challenge.TargetGrade, Grade: intp(grade)}}, Params: map[string]any{"subject_id": uuid.NewString()}}
	if len(listed) > 0 {
		r.Targets = append(r.Targets, challenge.Target{Type: challenge.TargetStudents, StudentIDs: listed})
	}
	return r
}

func field(err error) string {
	var ve challenge.ValidationError
	if errors.As(err, &ve) {
		return ve.Field
	}
	return ""
}

// T008 / F107-AC-01, AC-02, AC-05 — templates and requests: the class kinds are refused, a parameter outside
// allowed_params is 422 naming the key, the request is checked against the template window and the audience limits,
// the audience is (grade ∪ listed) − excluded with exclude winning.
func TestChallengeTemplateAndRequest(t *testing.T) {
	for _, k := range []string{"class_challenge", "class_tournament", ""} {
		if err := challenge.CheckKind(k); field(err) != "kind" {
			t.Fatalf("kind %q accepted", k)
		}
		var raw map[string]any
		s := seedTemplates(t)[0]
		_ = json.Unmarshal(s.Payload, &raw)
		raw["kind"] = k
		b, _ := json.Marshal(raw)
		if _, err := challenge.ParseTemplate(b); !errors.Is(err, challenge.ErrTemplate) {
			t.Fatalf("template of kind %q accepted", k)
		}
	}
	tpl := template(t, "grade_math_single_elim")
	if _, err := challenge.FilterParams(tpl, map[string]any{"reward": "x"}); field(err) != "params.reward" {
		t.Fatalf("reward parameter: %v", err)
	}
	if _, err := challenge.FilterParams(tpl, map[string]any{"points_win": 9}); field(err) != "params.points_win" {
		t.Fatal("a scoring rule is no parameter")
	}
	v, err := challenge.FilterParams(tpl, map[string]any{"max_entries": float64(16), "auto_confirm": true})
	if err != nil || *v.MaxEntries != 16 || !*v.AutoConfirm {
		t.Fatalf("allowed parameters: %v", err)
	}
	now := trnT0
	starts := now.Add(2 * time.Hour)
	ok := request(3, []uuid.UUID{uuid.New()}, starts)
	p, err := ok.Validate(tpl, now)
	if err != nil || p.Grade == nil || *p.Grade != 3 || len(p.Listed) != 1 || !p.EndsAt.After(p.StartsAt) {
		t.Fatalf("valid request: %+v %v", p, err)
	}
	cases := map[string]func(r *challenge.Request){
		"name":                func(r *challenge.Request) { r.Name = strings.Repeat("a", 81) },
		"ends_at":             func(r *challenge.Request) { e := r.StartsAt; r.EndsAt = &e },
		"starts_at":           func(r *challenge.Request) { r.StartsAt = now.Add(time.Minute) },
		"targets":             func(r *challenge.Request) { r.Targets = nil },
		"exclude_student_ids": func(r *challenge.Request) { r.Exclude = make([]uuid.UUID, 501) },
		"participation":       func(r *challenge.Request) { r.Participation = "study_group" },
	}
	for want, edit := range cases {
		r := request(3, nil, starts)
		edit(&r)
		if _, err := r.Validate(tpl, now); field(err) != want {
			t.Errorf("%s: %v", want, err)
		}
	}
	tooMany := make([]uuid.UUID, 501)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	if _, err := request(3, tooMany, starts).Validate(tpl, now); field(err) != "targets" {
		t.Fatal("more than 500 listed students")
	}
	for _, g := range []int{0, 10} {
		if _, err := request(g, nil, starts).Validate(tpl, now); field(err) != "targets" {
			t.Fatalf("grade %d", g)
		}
	}
	rr := template(t, "grade_round_robin") // grades 4–9 only
	if _, err := request(3, nil, starts).Validate(rr, now); field(err) != "targets" {
		t.Fatal("a grade outside the grades of the template")
	}
	end := starts.Add(9 * time.Hour)
	r := request(3, nil, starts)
	r.EndsAt = &end
	if _, err := r.Validate(tpl, now); field(err) != "ends_at" {
		t.Fatal("longer than the template allows")
	}
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	aud := challenge.Audience([]uuid.UUID{a, b, c}, []uuid.UUID{d, a}, []uuid.UUID{c, d})
	if len(aud) != 2 || !contains(aud, a) || !contains(aud, b) {
		t.Fatalf("audience = grade ∪ listed − excluded, exclude wins: %v", aud)
	}
	school := uuid.New()
	if !challenge.InAudience(a, school, 3, school, intp(3), nil, nil) || challenge.InAudience(a, uuid.New(), 3, school, intp(3), nil, nil) ||
		challenge.InAudience(a, school, 3, school, intp(3), nil, []uuid.UUID{a}) || !challenge.InAudience(d, school, 4, school, intp(3), []uuid.UUID{d}, nil) {
		t.Fatal("audience membership: same school, grade or listed, not excluded")
	}
	// the tournament of a challenge: registration closes before the check-in opens.
	timers := trn.Timers{CheckIn: 15 * time.Minute, NoShow: 3 * time.Minute, Complaint: 10 * time.Minute, Reschedule: 2 * time.Minute, RescheduleMax: 2}
	tp, err := challenge.TournamentOf(tpl, p, tpl.TimersOr(timers), now)
	if err != nil || !tp.RegCloseAt.Equal(starts.Add(-15*time.Minute)) || tp.Format != trn.FormatSingleElim || tp.SeedingPolicy != "random" {
		t.Fatalf("tournament plan %+v %v", tp, err)
	}
	late := starts.Add(-time.Minute).Format(time.RFC3339)
	p.Values.RegCloseAt = nil
	if _, err := challenge.FilterParams(tpl, map[string]any{"registration_close_at": late}); err != nil {
		t.Fatal(err)
	}
	v2, _ := challenge.FilterParams(tpl, map[string]any{"registration_close_at": late})
	p.Values = v2
	if _, err := challenge.TournamentOf(tpl, p, timers, now); field(err) != "params.registration_close_at" {
		t.Fatal("registration closing inside the check-in window")
	}
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// T035 / F107-AC-28 — no template parameter can turn off the match/team chat of a tournament match: no seeded template
// allows a chat key, a template allowing one is invalid, and a request with match_chat is 422.
func TestChallengeNoChatToggle(t *testing.T) {
	for _, s := range seedTemplates(t) {
		tpl, err := challenge.ParseTemplate(s.Payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range tpl.AllowedParams {
			if strings.Contains(strings.ToLower(p), "chat") {
				t.Fatalf("%s allows %s", s.Code, p)
			}
		}
		var raw map[string]any
		_ = json.Unmarshal(s.Payload, &raw)
		raw["allowed_params"] = []string{"match_chat"}
		b, _ := json.Marshal(raw)
		if _, err := challenge.ParseTemplate(b); !errors.Is(err, challenge.ErrTemplate) {
			t.Fatalf("%s: a template allowing match_chat is invalid", s.Code)
		}
		if _, err := challenge.FilterParams(tpl, map[string]any{"match_chat": false}); field(err) != "params.match_chat" {
			t.Fatalf("%s: match_chat parameter accepted", s.Code)
		}
	}
	for _, k := range []string{"match_chat", "team_chat", "chat_enabled", "reward", "prize_item"} {
		if !challenge.Forbidden(k) {
			t.Fatalf("%s must be forbidden", k)
		}
	}
	// the complaint log of a node is its result: ids, correct answers, times and outcomes only, never chat.
	b, _ := json.Marshal(trn.Result{A: trn.Side{Correct: 3, TimeMS: 1200, Outcome: "win"}, Played: true})
	if strings.Contains(string(b), "chat") || strings.Contains(string(b), "message") {
		t.Fatal("a node result carries no chat content")
	}
}
