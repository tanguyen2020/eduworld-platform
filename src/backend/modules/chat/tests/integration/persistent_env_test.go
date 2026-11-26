//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/policyschema"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// blockedPlaceholder is the text the TEST DOUBLE name checker refuses (never a real sensitive word, R15).
const blockedPlaceholder = "PLACEHOLDER_BLOCKED"

// nameChecker is a TEST DOUBLE (R17) of the DisplayTextChecker port (production: the F116 pipeline CheckDisplayText): it
// refuses a text containing blockedPlaceholder and records the number of checks.
type nameChecker struct {
	mu    sync.Mutex
	calls int
}

func (n *nameChecker) CheckDisplayText(_ context.Context, _ uuid.UUID, text string) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	return !strings.Contains(text, blockedPlaceholder), nil
}

// pEnv is the F120 environment: the chat core of setupChat plus the policy levels, the persistent channels (implicit
// World/Area, direct), the clubs, the search and the retention job on the real adapters.
type pEnv struct {
	*chatEnv
	policies   command.PolicyAdmin
	persistent command.Persistent
	clubs      command.Clubs
	implicit   *service.Implicit
	rooms      cache.Rooms
	names      *nameChecker
	retention  command.PersistentRetention
}

func setupPersistent(t *testing.T, o chatOpts) *pEnv {
	t.Helper()
	p := &pEnv{names: &nameChecker{}}
	o.hook = func(c *chatEnv) {
		gens := cache.PolicyGens{C: c.vk}
		c.facts.Gens = gens
		p.rooms = cache.Rooms{C: c.vk, TTL: time.Hour}
		p.implicit = &service.Implicit{Reader: persistence.ChatReader{}, Policies: persistence.Policies{}, Chat: persistence.ChatReader{},
			Rooms: p.rooms, Cache: c.facts.Cache, Facts: c.facts}
		fanout := chatrealtime.ChatFanout{P: rt.Publisher{C: c.vk}, Now: c.clock.Now}
		p.persistent = command.Persistent{UoW: c.uow, AfterCommit: entx.AfterCommit, Repo: persistence.Channels{}, Status: persistence.Channels{},
			Social: persistence.Social{}, Directory: persistence.Directory{}, Audit: persistence.Auditor{R: c.rec}, Facts: c.facts,
			Implicit: p.implicit, Push: fanout, Now: c.clock.Now}
		c.channels.Facts = c.facts
		c.send.Facts, c.send.Implicit, c.send.Ensure, c.send.Broadcast, c.send.HoursLoc = c.facts, p.implicit, p.persistent.EnsureImplicit,
			fanout, ict
		c.reads.Facts, c.reads.Implicit = c.facts, p.implicit
		c.social.Relations = p.implicit // F120 T061: a block/mute drops the cached relations of the World/Area filter
		c.chatHandlers = &chathttp.ChatHandlers{Send: c.send, Queries: c.reads}
		curfew, _ := policy.ParseWindow("21:30-06:30", ict)
		p.policies = command.PolicyAdmin{UoW: c.uow, AfterCommit: entx.AfterCommit, Store: persistence.Policies{}, Schema: policyschema.Validator{},
			Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Gens: gens, Curfew: c.facts.Curfew,
			PlatformCurfew: policy.Curfew{curfew}, Rejected: observability.Persistent{}.PolicyRejected, Now: c.clock.Now}
		p.clubs = command.Clubs{UoW: c.uow, AfterCommit: entx.AfterCommit, Store: persistence.Clubs{}, Social: persistence.Social{},
			Directory: persistence.Directory{}, Channels: persistence.Channels{}, Status: persistence.Channels{}, Names: p.names,
			Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Facts: c.facts, Push: fanout,
			Limits: club.Limits{MaxMembers: 50, PerStudent: 5, InviteTTL: 7 * 24 * time.Hour}, Now: c.clock.Now}
		codec, err := page.NewCodec([]byte(strings.Repeat("p", 32)))
		if err != nil {
			t.Fatal(err)
		}
		c.policyHandlers = &chathttp.PolicyHandlers{Commands: p.policies, Queries: query.Policies{Read: c.read, Reader: persistence.Policies{},
			Chat: persistence.ChatReader{}}}
		c.persistentHandlers = &chathttp.PersistentHandlers{Direct: p.persistent, Search: query.Search{Read: c.read, Reader: persistence.Search{},
			Cursors: codec}, Caps: c.reads.Caps}
		c.clubHandlers = &chathttp.ClubHandlers{Commands: p.clubs, Queries: query.Clubs{Read: c.read, Reader: persistence.ClubReads{},
			Directory: persistence.Directory{}, Cursors: codec}, WeekLoc: ict}
		p.retention = command.PersistentRetention{UoW: c.uow, Store: persistence.Retention{}, Audit: persistence.Auditor{R: c.rec},
			Metrics: observability.Persistent{}, Config: command.RetentionConfig{Days: 180, Batch: 2, EvidenceContext: 2}, Now: c.clock.Now}
	}
	p.chatEnv = setupChat(t, o)
	return p
}

// school projects a school of a tenant with its settings_subset (as chat.projection_identity writes it).
func (p *pEnv) school(tenant, school uuid.UUID, settings map[string]any) {
	p.t.Helper()
	b := []byte("{}")
	if settings != nil {
		b, _ = json.Marshal(settings)
	}
	p.exec(`INSERT INTO ref.schools (school_id, tenant_id, status, settings_subset, source_version) VALUES ($1, $2, 'active', $3, 1)
		ON CONFLICT (school_id) DO UPDATE SET settings_subset = $3`, school, tenant, string(b))
}

// staffAt registers portal staff of tenant holding role, limited to school, and returns its portal token.
func (p *pEnv) staffAt(tenant uuid.UUID, role string, school uuid.UUID) string {
	sub := "kc-staff-" + uuid.NewString()
	p.res.Set(sub, authztest.Staff(tenant, role, authztest.AtSchool(school)).PrincipalData)
	return p.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// systemAdmin registers System Admin and returns its portal token (resolved into the tenant of X-Tenant-Id).
func (p *pEnv) systemAdmin() string { return p.platformStaff("system_admin") }

// portal calls a portal route with token, X-Tenant-Id of tenant and extra headers.
func (p *pEnv) portal(token string, tenant uuid.UUID, method, path string, headers map[string]string, body any) resp {
	p.t.Helper()
	h := map[string]string{"X-Tenant-Id": tenant.String()}
	for k, v := range headers {
		h[k] = v
	}
	return p.do(method, path, token, h, body)
}

// event delivers one event to h in the transaction the outbox consumer opens, with a given event id (redelivery tests).
func (p *pEnv) event(h func(context.Context, outbox.Envelope) error, id uuid.UUID, typ string, tenant uuid.UUID, payload any) error {
	p.t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		p.t.Fatal(err)
	}
	scope, err := authz.EventScope(tenant)
	if err != nil {
		p.t.Fatal(err)
	}
	tt := tenant
	env := outbox.Envelope{EventID: id, EventType: typ, OccurredAt: p.clock.Now(), TenantID: &tt, Payload: b}
	return p.uow.InTenantTx(context.Background(), scope, func(ctx context.Context) error { return h(ctx, env) })
}

// createClub calls createClub as s and returns the response.
func (p *pEnv) createClub(s student, body map[string]any) resp {
	p.t.Helper()
	return p.call(s, http.MethodPost, "/social/clubs", newKey(), body)
}

func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

// mustClub creates an active club owned by s (school policy auto) and returns club id and channel id.
func (p *pEnv) mustClub(s student, name string) (uuid.UUID, uuid.UUID) {
	p.t.Helper()
	p.school(s.tenant, s.school, map[string]any{"student_club_creation": "auto"})
	r := p.createClub(s, map[string]any{"club_type": "interest_group", "name": name})
	if r.status != http.StatusCreated || str(r.body, "status") != "active" {
		p.t.Fatalf("create club: %d %s", r.status, r.raw)
	}
	return uuid.MustParse(str(r.body, "club_id")), uuid.MustParse(str(r.body, "chat_channel_id"))
}

// joinClub makes s a member of the club through an invitation of the owner.
func (p *pEnv) joinClub(owner, s student, clubID uuid.UUID) {
	p.t.Helper()
	r := p.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{s.id}})
	if r.status != http.StatusCreated || len(r.data()) != 1 {
		p.t.Fatalf("invite: %d %s", r.status, r.raw)
	}
	inv := str(r.data()[0], "invitation_id")
	if a := p.call(s, http.MethodPost, "/social/club-invitations/"+inv+":accept", newKey(), nil); a.status != http.StatusOK {
		p.t.Fatalf("accept invitation: %d %s", a.status, a.raw)
	}
}
