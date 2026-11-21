//go:build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	mmdomain "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// menv is the matchmaking integration environment of one test (F104): the party environment (migrated
// eduworld_competition, pkg/httpx chain, fake OIDC, tenants A/B/U) plus a real Valkey with the matchmaking store and the
// F104 use cases on the same server. The clock is the settable test clock; the matchmaker runs on demand (tick).
type menv struct {
	*penv
	vk      valkey.Client
	store   cache.MatchmakingStore
	mm      command.Matchmaking
	ratings *fakeRatings
	q       query.Matchmaking
	subject uuid.UUID
}

func setupMM(t *testing.T) *menv {
	t.Helper()
	m := &menv{subject: uuid.New(), ratings: &fakeRatings{r: map[uuid.UUID]mmdomain.Rating{}}}
	_, m.vk = valkeyLobby(t)
	m.store = cache.MatchmakingStore{C: m.vk}
	m.penv = setupParty(t, popts{extra: func(e *penv, h *competitionhttp.Handlers) {
		m.mm = command.Matchmaking{UoW: e.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Matchmaking{}, Directory: persistence.PartyDirectory{},
			Parties: persistence.Parties{}, Events: e.parties.Events, Audit: e.parties.Audit, Store: m.store, Realtime: e.push, Presence: e.online,
			Metrics: observability.Matchmaking{}, Settings: command.MatchmakingSettings{Owner: "worker-1", LockTTL: 5 * time.Second},
			Now: e.clock.Now, Tracker: command.NewTicketTracker(), Ratings: m.ratings}
		m.q = query.Matchmaking{Read: e.read, Reader: persistence.Matchmaking{}, Store: m.store, Now: e.clock.Now}
		h.Matchmaking, h.MatchmakingQueries = m.mm, m.q
	}})
	return m
}

func jobCtx() context.Context { return tenancy.WithModule(context.Background(), "competition") }

// tick runs one matchmaker pass; it fails the test on error.
func (m *menv) tick() int {
	m.t.Helper()
	n, err := m.mm.Tick(jobCtx())
	if err != nil {
		m.t.Fatalf("tick: %v", err)
	}
	return n
}

// sweep runs one pass of matchmaking-ticket-expiry.
func (m *menv) sweep() int {
	m.t.Helper()
	n, err := m.mm.ExpireTickets(jobCtx())
	if err != nil {
		m.t.Fatalf("sweep: %v", err)
	}
	return n
}

// setOnline marks students online (presence of the realtime gateway, TEST DOUBLE).
func (m *menv) setOnline(ks ...kid) {
	for _, k := range ks {
		m.penv.online.set(k.id, true)
	}
}

// ticket creates a ticket for k (battle_1v1, skill, the test subject unless body overrides).
func (m *menv) ticket(k kid, want int, body map[string]any) presp {
	m.t.Helper()
	b := map[string]any{"mode": "battle_1v1", "subject_id": m.subject, "matchmaking_type": "skill"}
	for key, v := range body {
		if v == nil {
			delete(b, key)
		} else {
			b[key] = v
		}
	}
	r := m.call(k, http.MethodPost, "/competition/matchmaking/tickets", key(), b)
	if want != 0 && r.status != want {
		m.t.Fatalf("ticket by %s: %d %s", k.nickname, r.status, r.raw)
	}
	return r
}

// proposalOf returns the last matchmaking.proposal.created pushed to k.
func (m *menv) proposalOf(k kid) map[string]any {
	m.t.Helper()
	ps := m.push.of(k.id, "matchmaking.proposal.created")
	if len(ps) == 0 {
		return nil
	}
	return ps[len(ps)-1]
}

func (m *menv) mustProposal(k kid) string {
	m.t.Helper()
	p := m.proposalOf(k)
	if p == nil {
		m.t.Fatalf("no proposal for %s", k.nickname)
	}
	id, _ := p["proposal_id"].(string)
	return id
}

func (m *menv) acceptProposal(k kid, proposal string, want int) presp {
	m.t.Helper()
	r := m.call(k, http.MethodPost, "/competition/matches/"+proposal+"/proposal:accept", "", nil)
	if want != 0 && r.status != want {
		m.t.Fatalf("accept by %s: %d %s", k.nickname, r.status, r.raw)
	}
	return r
}

func (m *menv) declineProposal(k kid, proposal string, want int) presp {
	m.t.Helper()
	r := m.call(k, http.MethodPost, "/competition/matches/"+proposal+"/proposal:decline", "", nil)
	if want != 0 && r.status != want {
		m.t.Fatalf("decline by %s: %d %s", k.nickname, r.status, r.raw)
	}
	return r
}

func (m *menv) getTicket(k kid, id string) presp {
	m.t.Helper()
	return m.call(k, http.MethodGet, "/competition/matchmaking/tickets/"+id, "", nil)
}

// updates returns the matchmaking.ticket.updated payloads pushed to k.
func (m *menv) updates(k kid) []map[string]any { return m.push.of(k.id, "matchmaking.ticket.updated") }

// lastUpdate returns the last matchmaking.ticket.updated of k (nil when none).
func (m *menv) lastUpdate(k kid) map[string]any {
	u := m.updates(k)
	if len(u) == 0 {
		return nil
	}
	return u[len(u)-1]
}

// matches counts competition.matches rows (superuser view).
func (m *menv) matches() int { return m.count(`SELECT count(*) FROM competition.matches`) }

// restrict adds a competition restriction in force for k (kind abandon_penalty or restrict_competition).
func (m *menv) restrict(k kid, kind string) {
	m.exec(`INSERT INTO competition.competition_restrictions (restriction_id, tenant_id, school_id, student_id, kind, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5, 'test', $6, $7)`, uuid.New(), k.tenant, k.school, k.id, kind, m.clock.Now().Add(-time.Minute),
		m.clock.Now().Add(15*time.Minute))
}

// fakeRatings is a TEST DOUBLE of port.RatingReader (the Glicko-2 ratings of F106, R17): ratings set by the test, the
// default rating otherwise.
type fakeRatings struct {
	mu sync.Mutex
	r  map[uuid.UUID]mmdomain.Rating
}

func (f *fakeRatings) set(k kid, mu float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.r[k.id] = mmdomain.Rating{Mu: mu, RD: 120}
}

func (f *fakeRatings) Ratings(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ string, _ uuid.UUID, def mmdomain.Rating) (map[uuid.UUID]mmdomain.Rating, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uuid.UUID]mmdomain.Rating{}
	for _, id := range ids {
		if r, ok := f.r[id]; ok {
			out[id] = r
		} else {
			out[id] = def
		}
	}
	return out, nil
}

// readSeed returns seed/matchmaking_config.json.
func readSeed(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "competition", "seed", "matchmaking_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
