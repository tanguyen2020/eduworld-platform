package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ---------- TEST DOUBLES (R17): in-memory stand-ins of the ports of command.Profiles ----------

// nickUoW is a TEST DOUBLE of write.UnitOfWork: fn runs without a transaction.
type nickUoW struct{}

func (nickUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
func (nickUoW) InTenantTx(ctx context.Context, _ tenancy.Scope, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// nickRepo is a TEST DOUBLE of write.ProfileRepository holding one profile.
type nickRepo struct {
	write.ProfileRepository
	p        *profile.Profile
	saved    int
	history  []write.NicknameChange
	taken    map[string]bool // nickname keys held by other students of the tenant
	changes  int             // player_change rows this month
	log      *[]string
	saveErrs error
}

func (r *nickRepo) Get(context.Context, uuid.UUID, uuid.UUID) (*profile.Profile, error) {
	if r.p == nil {
		return nil, nil
	}
	c := *r.p
	return &c, nil
}

func (r *nickRepo) Save(_ context.Context, p *profile.Profile) error {
	*r.log = append(*r.log, "save")
	if r.saveErrs != nil {
		return r.saveErrs
	}
	if p.Nickname != nil && r.taken[profile.NicknameKey(*p.Nickname)] {
		return write.ErrNicknameTaken
	}
	c := *p
	c.Version++
	r.p = &c
	r.saved++
	return nil
}

func (r *nickRepo) AppendNicknameChange(_ context.Context, c write.NicknameChange) error {
	r.history = append(r.history, c)
	return nil
}

func (r *nickRepo) PlayerChangesSince(context.Context, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	*r.log = append(*r.log, "limit")
	return r.changes, nil
}

// nickSchools is a TEST DOUBLE of write.SchoolPolicy (no school limit).
type nickSchools struct{}

func (nickSchools) NicknameChangeLimit(context.Context, uuid.UUID, uuid.UUID) (*int, error) {
	return nil, nil
}

// nickOptions is a TEST DOUBLE of port.NicknameOptionStore.
type nickOptions struct{ options map[uuid.UUID]string }

func (o nickOptions) Save(context.Context, uuid.UUID, map[uuid.UUID]string, time.Duration) error {
	return nil
}
func (o nickOptions) Get(_ context.Context, _, option uuid.UUID) (string, bool, error) {
	n, ok := o.options[option]
	return n, ok, nil
}
func (o nickOptions) Clear(context.Context, uuid.UUID) error { return nil }

// nickEvents is a TEST DOUBLE of write.EventPublisher.
type nickEvents struct{ events []write.Event }

func (e *nickEvents) Publish(_ context.Context, ev write.Event) error {
	e.events = append(e.events, ev)
	return nil
}

// nickAudit is a TEST DOUBLE of write.Auditor.
type nickAudit struct{ events []audit.Event }

func (a *nickAudit) Record(_ context.Context, e audit.Event) error {
	a.events = append(a.events, e)
	return nil
}

// nickScreen is a TEST DOUBLE of port.NicknameModeration and port.NicknamePersonalDataScreen.
type nickScreen struct {
	name    string
	allowed bool
	err     error
	slow    bool // waits for the deadline of ctx
	texts   []string
	log     *[]string
}

func (s *nickScreen) check(ctx context.Context, text string) (bool, error) {
	*s.log = append(*s.log, s.name)
	s.texts = append(s.texts, text)
	if s.slow {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return s.allowed, s.err
}

func (s *nickScreen) CheckNickname(ctx context.Context, _, _ uuid.UUID, text string) (bool, error) {
	return s.check(ctx, text)
}

func (s *nickScreen) ScreenNickname(ctx context.Context, _, _ uuid.UUID, text string) (bool, error) {
	return s.check(ctx, text)
}

// nickAttempts is a TEST DOUBLE of port.NicknameAttemptCounter.
type nickAttempts struct {
	hits, limit int
	err         error
	log         *[]string
}

func (a *nickAttempts) Hit(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, time.Duration, error) {
	*a.log = append(*a.log, "attempt")
	if a.err != nil {
		return false, 0, a.err
	}
	a.hits++
	if a.hits > a.limit {
		return false, 7 * time.Minute, nil
	}
	return true, 0, nil
}

// nickMetrics is a TEST DOUBLE of port.NicknameMetrics.
type nickMetrics struct{ results []string }

func (m *nickMetrics) NicknameCheck(result string)                 { m.results = append(m.results, result) }
func (m *nickMetrics) NicknameScreenLatency(string, time.Duration) {}

// ---------- fixture ----------

type nickFixture struct {
	h        Profiles
	repo     *nickRepo
	mod      *nickScreen
	personal *nickScreen
	attempts *nickAttempts
	events   *nickEvents
	audit    *nickAudit
	metrics  *nickMetrics
	log      *[]string
	ctx      context.Context
	student  uuid.UUID
}

var nickNow = time.Date(2026, 11, 28, 3, 0, 0, 0, time.UTC)

func newNickFixture(t *testing.T, current *string) *nickFixture {
	t.Helper()
	tenant, student, school := uuid.New(), uuid.New(), uuid.New()
	log := &[]string{}
	p := profile.New(tenant, student, school, nickNow)
	p.ID, p.Nickname = uuid.New(), current
	f := &nickFixture{log: log, student: student,
		repo:     &nickRepo{p: p, taken: map[string]bool{}, log: log},
		mod:      &nickScreen{name: "moderation", allowed: true, log: log},
		personal: &nickScreen{name: "personal", allowed: true, log: log},
		attempts: &nickAttempts{limit: 10, log: log},
		events:   &nickEvents{}, audit: &nickAudit{}, metrics: &nickMetrics{}}
	f.h = Profiles{UoW: nickUoW{}, Repo: f.repo, Schools: nickSchools{}, Options: nickOptions{options: map[uuid.UUID]string{}},
		Events: f.events, Audit: f.audit, Moderation: f.mod, Personal: f.personal, Attempts: f.attempts, NicknameMetrics: f.metrics,
		Now: func() time.Time { return nickNow },
		Config: ProfileConfig{Nickname: profile.NicknamePolicy{MonthlyLimit: 1, Location: time.UTC},
			NicknameRules: profile.NicknameRules{MinLength: 3, MaxDigits: 4}, ScreenDeadline: 50 * time.Millisecond}}
	pr := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &tenant})
	f.ctx = authz.WithPrincipal(context.Background(), pr)
	return f
}

func typed(s string) ProfileUpdate { return ProfileUpdate{Nickname: &s, SetNickname: true} }

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// noLeak fails when the problem of err carries the typed text or one of its words (F136-AC-16).
func noLeak(t *testing.T, err error, typedText string) {
	t.Helper()
	b, _ := json.Marshal(problem.From(err, ""))
	body := strings.ToLower(string(b))
	for _, w := range strings.Fields(strings.ToLower(typedText)) {
		if len(w) >= 3 && strings.Contains(body, w) {
			t.Fatalf("problem %s leaks %q of the refused nickname", b, w)
		}
	}
}

func (f *nickFixture) nothingSaved(t *testing.T) {
	t.Helper()
	if f.repo.saved != 0 || len(f.repo.history) != 0 || len(f.events.events) != 0 {
		t.Fatalf("saved %d, history %d, events %d: nothing may be stored", f.repo.saved, len(f.repo.history), len(f.events.events))
	}
}

// F136-AC-04, AC-11, AC-12 (TC-GAM-013): a valid typed nickname passes the attempt counter, the chat moderation and the
// identity screen (normalised text, in that order), is stored with its history (initial) and game_profile.updated.
func TestChangeNicknameTypedSaved(t *testing.T) {
	f := newNickFixture(t, nil)
	p, err := f.h.UpdateProfile(f.ctx, typed("  Siêu   Nhân Sao Băng "))
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if p.Nickname == nil || *p.Nickname != "Siêu Nhân Sao Băng" {
		t.Fatalf("nickname %v", p.Nickname)
	}
	if got := strings.Join(*f.log, ","); got != "attempt,moderation,personal,save" {
		t.Fatalf("order %s (first set: no monthly limit)", got)
	}
	if f.mod.texts[0] != "Siêu Nhân Sao Băng" || f.personal.texts[0] != "Siêu Nhân Sao Băng" {
		t.Fatalf("screened %q / %q, want the normalised text", f.mod.texts, f.personal.texts)
	}
	if len(f.repo.history) != 1 || f.repo.history[0].Reason != profile.ReasonInitial || f.repo.history[0].New != "Siêu Nhân Sao Băng" {
		t.Fatalf("history %+v", f.repo.history)
	}
	if len(f.events.events) != 1 || f.events.events[0].Type != "game_profile.updated" {
		t.Fatalf("events %+v", f.events.events)
	}
	if len(f.audit.events) != 1 || f.audit.events[0].Action != AuditNicknameChanged || f.audit.events[0].Result != audit.ResultSuccess {
		t.Fatalf("audit %+v", f.audit.events)
	}
	if strings.Join(f.metrics.results, ",") != port.NicknameAllowed {
		t.Fatalf("metrics %v", f.metrics.results)
	}
}

// F136-AC-10, AC-16: a format violation is 422 NICKNAME_NOT_ALLOWED before any facade call; the problem never echoes the
// text; the refusal is counted and audited with its result code only (AC-30).
func TestChangeNicknameFormatRefused(t *testing.T) {
	f := newNickFixture(t, nil)
	_, err := f.h.UpdateProfile(f.ctx, typed("@minh_123 zalo"))
	if code(err) != problem.NicknameNotAllowed {
		t.Fatalf("err %v, want NICKNAME_NOT_ALLOWED", err)
	}
	noLeak(t, err, "@minh_123 zalo")
	if strings.Contains(strings.Join(*f.log, ","), "moderation") {
		t.Fatalf("calls %v: no facade call for a format violation", *f.log)
	}
	f.nothingSaved(t)
	if f.attempts.hits != 1 {
		t.Fatalf("attempts %d, want the refusal counted", f.attempts.hits)
	}
	f.refusalAudited(t, port.NicknameFormat, "@minh_123 zalo")
}

func (f *nickFixture) refusalAudited(t *testing.T, result, text string) {
	t.Helper()
	if len(f.audit.events) != 1 || f.audit.events[0].Action != AuditNicknameRefused || f.audit.events[0].Result != audit.ResultDenied {
		t.Fatalf("audit %+v", f.audit.events)
	}
	b, _ := json.Marshal(f.audit.events[0])
	if !strings.Contains(string(b), result) {
		t.Fatalf("refusal audit %s: want the result code %s", b, result)
	}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if len(w) >= 4 && strings.Contains(strings.ToLower(string(b)), w) {
			t.Fatalf("refusal audit %s leaks %q: result code only, never the text", b, w)
		}
	}
	if got := f.metrics.results; len(got) != 1 || got[0] != result {
		t.Fatalf("metrics %v, want %s", got, result)
	}
}

// F136-AC-11, AC-16: blocked by the chat moderation → 422 NICKNAME_NOT_ALLOWED, the identity screen is not called,
// nothing stored, no leak.
func TestChangeNicknameModerationBlocked(t *testing.T) {
	f := newNickFixture(t, nil)
	f.mod.allowed = false
	_, err := f.h.UpdateProfile(f.ctx, typed("Tu Cam Mau Placeholder"))
	if code(err) != problem.NicknameNotAllowed {
		t.Fatalf("err %v", err)
	}
	noLeak(t, err, "Tu Cam Mau Placeholder")
	if len(f.personal.texts) != 0 {
		t.Fatal("identity screen called after a moderation block")
	}
	f.nothingSaved(t)
	f.refusalAudited(t, port.NicknameModerated, "Tu Cam Mau Placeholder")
}

// F136-AC-12: blocked by the identity screen (real name / school of the student) → the same 422.
func TestChangeNicknamePersonalBlocked(t *testing.T) {
	f := newNickFixture(t, nil)
	f.personal.allowed = false
	_, err := f.h.UpdateProfile(f.ctx, typed("Nguyen Van Placeholder"))
	if code(err) != problem.NicknameNotAllowed {
		t.Fatalf("err %v", err)
	}
	noLeak(t, err, "Nguyen Van Placeholder")
	f.nothingSaved(t)
	f.refusalAudited(t, port.NicknamePersonal, "Nguyen Van Placeholder")
}

// F136-AC-14: an error or a timeout of either facade is 503 UNAVAILABLE and nothing is stored (fail-closed); missing
// facades are the same; the suggestion path still works while moderation is down.
func TestChangeNicknameFailClosed(t *testing.T) {
	cases := map[string]func(f *nickFixture){
		"moderation error":   func(f *nickFixture) { f.mod.err = port.ErrNicknameScreenUnavailable },
		"moderation timeout": func(f *nickFixture) { f.mod.slow = true },
		"identity error":     func(f *nickFixture) { f.personal.err = errors.New("identity down") },
		"identity timeout":   func(f *nickFixture) { f.personal.slow = true },
		"no moderation":      func(f *nickFixture) { f.h.Moderation = nil },
		"no identity":        func(f *nickFixture) { f.h.Personal = nil },
		"counter down":       func(f *nickFixture) { f.attempts.err = errors.New("valkey down") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newNickFixture(t, nil)
			setup(f)
			_, err := f.h.UpdateProfile(f.ctx, typed("Siêu Nhân Sao Băng"))
			if code(err) != problem.Unavailable {
				t.Fatalf("err %v, want 503 UNAVAILABLE", err)
			}
			f.nothingSaved(t)
		})
	}
	f := newNickFixture(t, nil)
	f.mod.err = port.ErrNicknameScreenUnavailable
	opt := uuid.New()
	f.h.Options = nickOptions{options: map[uuid.UUID]string{opt: "Cú Mèo Chăm Chỉ 27"}}
	p, err := f.h.UpdateProfile(f.ctx, ProfileUpdate{NicknameOptionID: &opt})
	if err != nil || p.Nickname == nil || *p.Nickname != "Cú Mèo Chăm Chỉ 27" || len(f.mod.texts) != 0 {
		t.Fatalf("suggestion path: %v %v (moderation calls %d)", p, err, len(f.mod.texts))
	}
}

// F136-AC-15: beyond nickname_attempt_rate → 429 RATE_LIMITED with retry_after before any facade call; no sanction.
func TestChangeNicknameAttemptRate(t *testing.T) {
	f := newNickFixture(t, nil)
	f.attempts.limit = 0
	_, err := f.h.UpdateProfile(f.ctx, typed("Siêu Nhân Sao Băng"))
	var ra RetryAfter
	if code(err) != problem.RateLimited || !errors.As(err, &ra) || ra.After != 7*time.Minute {
		t.Fatalf("err %v, want 429 with Retry-After 7 min", err)
	}
	if len(f.mod.texts) != 0 || len(f.personal.texts) != 0 {
		t.Fatal("facade called beyond the attempt rate")
	}
	f.nothingSaved(t)
}

// F136-AC-08: a player change beyond nickname_change_monthly_limit is 409 NICKNAME_CHANGE_LIMIT before moderation.
func TestChangeNicknameMonthlyLimitFirst(t *testing.T) {
	old := "Cú Mèo Chăm Chỉ 27"
	f := newNickFixture(t, &old)
	f.repo.changes = 1
	_, err := f.h.UpdateProfile(f.ctx, typed("Siêu Nhân Sao Băng"))
	if code(err) != problem.NicknameChangeLimit {
		t.Fatalf("err %v", err)
	}
	if len(f.mod.texts) != 0 {
		t.Fatal("moderation called beyond the monthly limit")
	}
	f.nothingSaved(t)
	// Within the limit the change is a player_change.
	f = newNickFixture(t, &old)
	if _, err := f.h.UpdateProfile(f.ctx, typed("Siêu Nhân Sao Băng")); err != nil {
		t.Fatal(err)
	}
	if f.repo.history[0].Reason != profile.ReasonPlayerChange || *f.repo.history[0].Old != old {
		t.Fatalf("history %+v", f.repo.history)
	}
}

// F136-AC-13: a key held by another student is 409 NICKNAME_UNAVAILABLE, only after moderation and the identity screen.
func TestChangeNicknameTakenAfterScreening(t *testing.T) {
	f := newNickFixture(t, nil)
	f.repo.taken["siêu nhân sao băng"] = true
	_, err := f.h.UpdateProfile(f.ctx, typed("SIÊU NHÂN Sao Băng"))
	if code(err) != problem.NicknameUnavailable {
		t.Fatalf("err %v", err)
	}
	if got := strings.Join(*f.log, ","); got != "attempt,moderation,personal,save" {
		t.Fatalf("order %s: taken is reported after the screening", got)
	}
}

// F136-AC-03: nickname and nickname_option_id together, or nickname null, are 422 VALIDATION_FAILED.
func TestChangeNicknameExclusiveFields(t *testing.T) {
	f := newNickFixture(t, nil)
	opt := uuid.New()
	in := typed("Siêu Nhân Sao Băng")
	in.NicknameOptionID = &opt
	if _, err := f.h.UpdateProfile(f.ctx, in); code(err) != problem.ValidationFailed {
		t.Fatalf("both fields: %v", err)
	}
	if _, err := f.h.UpdateProfile(f.ctx, ProfileUpdate{SetNickname: true}); code(err) != problem.ValidationFailed {
		t.Fatalf("nickname null: %v", err)
	}
	if len(*f.log) != 0 {
		t.Fatalf("calls %v", *f.log)
	}
}

// The same nickname again changes nothing and calls no facade (F070 behaviour kept).
func TestChangeNicknameSameAgain(t *testing.T) {
	cur := "Siêu Nhân Sao Băng"
	f := newNickFixture(t, &cur)
	p, err := f.h.UpdateProfile(f.ctx, typed(" Siêu  Nhân Sao Băng"))
	if err != nil || *p.Nickname != cur {
		t.Fatalf("%v %v", p, err)
	}
	if len(f.mod.texts) != 0 {
		t.Fatal("facade called for an unchanged nickname")
	}
	f.nothingSaved(t)
}
