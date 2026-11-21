//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/battlehost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// rightOption is the right answer of every question of fakePlans.
const rightOption = "o_right"

// fakePlans is a TEST DOUBLE of the learning facade for the battle engine (port.QuestionPlans, R17): single-choice
// questions whose render payload has no answer data; o_right is right; it records the calls (the learning side is
// tested against the real module in modules/learning/tests/integration/question_plan_test.go).
type fakePlans struct {
	mu       sync.Mutex
	fail     error // Create fails
	scoreErr error // Score fails
	creates  []port.PlanRequest
	scores   map[string]int // (index, student) → calls
	plans    map[uuid.UUID]uuid.UUID
}

func (f *fakePlans) Create(_ context.Context, req port.PlanRequest) (port.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, req)
	if f.fail != nil {
		return port.Plan{}, f.fail
	}
	if f.plans == nil {
		f.plans = map[uuid.UUID]uuid.UUID{}
	}
	id, ok := f.plans[req.MatchID]
	if !ok {
		id = uuid.New()
		f.plans[req.MatchID] = id
	}
	p := port.Plan{ID: id, Version: "qp1-" + id.String()[:8]}
	for i := 0; i < req.Count; i++ {
		render, _ := json.Marshal(map[string]any{"stem": map[string]any{"text": fmt.Sprintf("%d + %d = ?", i, i)},
			"options": []map[string]any{{"option_id": rightOption}, {"option_id": "o_wrong"}}})
		p.Items = append(p.Items, port.PlanItem{Index: i, QuestionVersionID: uuid.New(), Type: "single_choice", TimeS: req.TimeS, Render: render})
	}
	return p, nil
}

func (f *fakePlans) Score(_ context.Context, _ uuid.UUID, index int, student uuid.UUID, response json.RawMessage, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.scores == nil {
		f.scores = map[string]int{}
	}
	f.scores[fmt.Sprintf("%d:%s", index, student)]++
	if f.scoreErr != nil {
		return false, f.scoreErr
	}
	var r struct {
		OptionID string `json:"option_id"`
	}
	_ = json.Unmarshal(response, &r)
	return r.OptionID == rightOption, nil
}

func (f *fakePlans) Reveal(_ context.Context, _ uuid.UUID, index int, _ time.Time) (battle.Reveal, error) {
	return battle.Reveal{CorrectResponse: json.RawMessage(`{"option_id":"` + rightOption + `"}`), Explanation: "Giải thích",
		Question: json.RawMessage(fmt.Sprintf(`{"stem":{"text":"%d + %d = ?"}}`, index, index))}, nil
}

// recOut is a TEST DOUBLE of port.RoomOutput recording every message (room or user).
type recOut struct {
	mu   sync.Mutex
	msgs []outMsg
}

type outMsg struct {
	room    uuid.UUID
	user    uuid.UUID // one player (ew:rt:user)
	to      []uuid.UUID
	typ     string
	payload map[string]any
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func (o *recOut) Room(_ context.Context, room uuid.UUID, typ string, payload any, to []uuid.UUID) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.msgs = append(o.msgs, outMsg{room: room, to: to, typ: typ, payload: toMap(payload)})
	return nil
}

func (o *recOut) User(_ context.Context, user uuid.UUID, typ string, payload any) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.msgs = append(o.msgs, outMsg{user: user, typ: typ, payload: toMap(payload)})
	return nil
}

// of returns the messages of typ a student receives (room broadcast, room targeted or user channel).
func (o *recOut) of(student uuid.UUID, typ string) []map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []map[string]any
	for _, m := range o.msgs {
		if m.typ != typ {
			continue
		}
		switch {
		case m.user != uuid.Nil && m.user == student:
		case m.user == uuid.Nil && m.to == nil:
		case m.user == uuid.Nil:
			hit := false
			for _, id := range m.to {
				hit = hit || id == student
			}
			if !hit {
				continue
			}
		default:
			continue
		}
		out = append(out, m.payload)
	}
	return out
}

func (o *recOut) all() []outMsg {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]outMsg(nil), o.msgs...)
}

// benv is the battle environment (F105): the party environment (migrated eduworld_competition, httpx chain, tenants
// A/B), a real Valkey (leases, checkpoints) and NATS (operations between room hosts, presence), the battle use cases
// over the fake learning facade and the recording output; the clock is the settable test clock.
type benv struct {
	*penv
	vk      valkey.Client
	nc      *nats.Conn
	plans   *fakePlans
	out     *recOut
	battles command.Battles
	subject uuid.UUID
}

func setupBattle(t *testing.T) *benv {
	t.Helper()
	b := &benv{plans: &fakePlans{}, out: &recOut{}, subject: uuid.New()}
	_, b.vk = valkeyLobby(t)
	_, url := testkit.StartNATS(context.Background(), t)
	var err error
	if b.nc, err = nats.Connect(url); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.nc.Close)
	b.penv = setupParty(t, popts{extra: func(e *penv, h *competitionhttp.Handlers) {
		h.Battles = query.Battles{Read: e.read, Reader: persistence.BattleRead{}, Plans: b.plans}
	}})
	w, err := outbox.NewWriter("competition", "core", nil)
	if err != nil {
		t.Fatal(err)
	}
	b.battles = command.Battles{UoW: b.uow, Repo: persistence.Battle{}, Plans: b.plans, Cards: persistence.PartyDirectory{},
		Events: persistence.Publisher{W: w}, Audit: persistence.Auditor{R: b.rec}, Metrics: observability.Battle{}, Now: b.clock.Now,
		Instance: "core-api-test"}
	return b
}

// host starts a battle room host (one core-api replica) with fast leases; stop it with its cancel.
func (b *benv) host(id string) (*battlehost.Host, context.CancelFunc) {
	b.t.Helper()
	h := battlehost.New(battlehost.Config{InstanceID: id, TickHz: 20, LeaseTTL: 1500 * time.Millisecond, LeaseEvery: 300 * time.Millisecond,
		WatchEvery: 300 * time.Millisecond, CheckpointEvery: time.Second, OpTimeout: 3 * time.Second}, b.nc, cache.BattleStore{C: b.vk}, b.out,
		b.battles, b.clock.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = h.Run(ctx)
	}()
	stop := func() {
		cancel()
		<-done
	}
	b.t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	time.Sleep(100 * time.Millisecond) // subscriptions in place
	return h, stop
}

// matchOpts describes a match created like matchmaking does (F104 hand-off).
type matchOpts struct {
	mode       string
	grade      int
	ranked     bool
	isBot      bool
	tournament bool
	teams      [][]kid
}

// match inserts competition.matches + match_participants in status lobby (what F104 commits before match.created).
func (b *benv) match(o matchOpts) uuid.UUID {
	b.t.Helper()
	id := uuid.New()
	typ := "skill"
	switch {
	case o.isBot:
		typ = "bot"
	case o.mode == battle.ModeTeam:
		typ = "team"
	case !o.ranked:
		typ = "random"
	}
	tenant := o.teams[0][0].tenant
	var node any
	if o.tournament {
		node = uuid.New()
		typ = "tournament"
	}
	b.exec(`INSERT INTO competition.matches (match_id, mode, matchmaking_type, subject_id, grade_level, ranked, is_bot, scope_kind,
		ruleset_version_id, tenant_ids, tournament_node_id) VALUES ($1, $2, $3, $4, $5, $6, $7, 'school', '01936a00-0000-7000-8000-000000001041',
		ARRAY[$8]::uuid[], $9)`, id, o.mode, typ, b.subject, o.grade, o.ranked, o.isBot, tenant, node)
	for team, ks := range o.teams {
		for _, k := range ks {
			b.exec(`INSERT INTO competition.match_participants (participant_id, tenant_id, match_id, student_id, school_id, team_no)
				VALUES ($1, $2, $3, $4, $5, $6)`, uuid.New(), k.tenant, id, k.id, k.school, team)
		}
	}
	return id
}

// caller is the identity of a student on the gateway (Ew-* headers) with a game session.
func caller(k kid, session uuid.UUID) rt.Caller {
	return rt.Caller{TenantID: k.tenant, UserID: k.id, SchoolID: k.school, GameSessionID: session, DeviceID: uuid.New(), ConnID: "c1", NodeID: "n1"}
}

// answer sends battle.answer.submit through the NATS handler of the gateway route for k on the current question.
func (b *benv) answer(h *battlehost.Host, match uuid.UUID, k kid, session uuid.UUID, instance, option string) rt.Reply {
	return natscmd.BattleAnswer(h, nil)(context.Background(), caller(k, session), json.RawMessage(fmt.Sprintf(
		`{"match_id":%q,"question_instance_id":%q,"response":{"option_id":%q}}`, match, instance, option)))
}

func (b *benv) state(h *battlehost.Host, match uuid.UUID, k kid, session uuid.UUID) rt.Reply {
	return natscmd.BattleState(h, nil)(context.Background(), caller(k, session), json.RawMessage(fmt.Sprintf(`{"match_id":%q}`, match)))
}

// shown waits for question index shown to k and returns its instance id.
func (b *benv) shown(k kid, index int) string {
	b.t.Helper()
	var inst string
	waitFor(b.t, 15*time.Second, fmt.Sprintf("question %d shown", index), func() bool {
		for _, m := range b.out.of(k.id, "battle.question.shown") {
			if int(m["question_index"].(float64)) == index {
				inst = m["question"].(map[string]any)["question_instance_id"].(string)
			}
		}
		return inst != ""
	})
	return inst
}

// status returns competition.matches.status.
func (b *benv) status(match uuid.UUID) string {
	var s string
	if err := b.su.QueryRowContext(context.Background(), `SELECT status FROM competition.matches WHERE match_id = $1`, match).Scan(&s); err != nil {
		b.t.Fatal(err)
	}
	return s
}

// eventsOf returns the outbox payloads of typ for match.
func (b *benv) eventsOf(typ string, match uuid.UUID) []map[string]any {
	var out []map[string]any
	for _, e := range b.events(typ) {
		if e["match_id"] == match.String() {
			out = append(out, e)
		}
	}
	return out
}

// noAnswerKeys fails when a message carries answer data before its question closed (F105-AC-02, SC-001).
func noAnswerKeys(t *testing.T, typ string, m map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(m)
	for _, k := range []string{"answer_key", "correct_response", "scoring_rule", "explanation", "correct_option"} {
		if strings.Contains(string(raw), k) {
			t.Fatalf("%s carries %s: %s", typ, k, raw)
		}
	}
}
