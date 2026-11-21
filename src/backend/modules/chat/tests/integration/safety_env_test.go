//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	chatrealtime "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// safetyEnv is the chat environment with the real moderation pipeline and the F121 use cases (Safety Team) behind the
// real pkg/httpx chain. Messages are placeholders (R15); the identity resolution of a safety_moderator into the target
// tenant of X-Tenant-Id (F012 T058) is simulated by registering an already resolved principal (as F116's tests do).
type safetyEnv struct {
	*chatEnv
	cmd      command.Safety
	reads    query.Safety
	consumer consumer.MessageHeld
}

func setupSafety(t *testing.T) *safetyEnv {
	t.Helper()
	s := &safetyEnv{}
	s.chatEnv = setupChat(t, chatOpts{real: true, hook: func(c *chatEnv) {
		codec, err := page.NewCodec([]byte(strings.Repeat("s", 32)))
		if err != nil {
			t.Fatal(err)
		}
		s.cmd = command.Safety{UoW: c.uow, Store: persistence.Safety{}, Events: persistence.Publisher{W: c.writer},
			Audit: persistence.Auditor{R: c.rec}, Cache: cache.Sanctions{C: c.vk},
			Push: chatrealtime.ChatFanout{P: rt.Publisher{C: c.vk}, Now: c.clock.Now}, Recent: cache.Recent{C: c.vk, Size: 200, TTL: 10 * time.Minute},
			Metrics: observability.Safety{}, AfterCommit: entx.AfterCommit, Now: c.clock.Now}
		s.reads = query.Safety{Read: c.read, Reader: persistence.Safety{}, Audit: c.rec, Cursors: codec, Now: c.clock.Now}
		s.consumer = consumer.MessageHeld{H: s.cmd}
		c.safetyHandlers = &chathttp.SafetyHandlers{Commands: s.cmd, Queries: s.reads}
		c.social.Holds = persistence.Safety{}
	}})
	return s
}

// moderator is a platform staff member with one user id: a token without tenant (queue, reads) and tokens resolved into a
// target tenant (writes with X-Tenant-Id).
type moderator struct {
	s     *safetyEnv
	data  authz.PrincipalData
	id    uuid.UUID
	token string
}

func (s *safetyEnv) moderator(role string, lead bool) *moderator {
	s.t.Helper()
	d := authztest.Platform(role).PrincipalData
	if lead {
		d.Grants = append(d.Grants, authz.Grant{Role: "safety_lead", ScopeKind: authz.RoleScopePlatform,
			Permissions: []authz.Permission{authz.PermModerationCaseLead}})
	}
	sub := "kc-mod-" + uuid.NewString()
	s.res.Set(sub, d)
	return &moderator{s: s, data: d, id: d.UserID, token: s.idp.Token(sub, httpxtest.PortalClient, time.Now())}
}

// in returns the token of the moderator resolved into tenant (the principal identity would build for X-Tenant-Id).
func (m *moderator) in(tenant uuid.UUID) string {
	d := m.data
	t := tenant
	d.TenantID, d.TenantType = &t, authz.TenantTypeSchool
	sub := "kc-mod-in-" + uuid.NewString()
	m.s.res.Set(sub, d)
	return m.s.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// get calls a read of the Safety Team (no X-Tenant-Id).
func (m *moderator) get(path string) resp {
	return m.s.do(http.MethodGet, path, m.token, nil, nil)
}

// post calls a write of the Safety Team in tenant with a new Idempotency-Key (and If-Match when version > 0).
func (m *moderator) post(tenant uuid.UUID, path string, body any, version int64) resp {
	h := map[string]string{"X-Tenant-Id": tenant.String(), "Idempotency-Key": newKey()}
	if version > 0 {
		h["If-Match"] = `W/"` + jsonNum(version) + `"`
	}
	return m.s.do(http.MethodPost, path, m.in(tenant), h, body)
}

func jsonNum(n int64) string { b, _ := json.Marshal(n); return string(b) }

// msg stores a placeholder message of sender in channel at seq (chat.messages, or chat.gameplay_messages when gameplay)
// as chat-persister would; created at the test clock.
func (s *safetyEnv) msg(ch uuid.UUID, gameplay bool, sender student, seq int64, body string) uuid.UUID {
	s.t.Helper()
	id := uuid.Must(uuid.NewV7())
	at := s.clock.Now().Add(time.Duration(seq) * time.Second)
	if gameplay {
		s.exec(`INSERT INTO chat.gameplay_messages (message_id, channel_id, seq, tenant_id, school_id, sender_id, kind, body, body_display,
			created_at, created_on) VALUES ($1, $2, $3, $4, $5, $6, 'text', $7, $7, $8, $9)`, id, ch, seq, sender.tenant, sender.school,
			sender.id, body, at, at.UTC().Format("2006-01-02"))
		return id
	}
	s.exec(`INSERT INTO chat.messages (message_id, channel_id, seq, tenant_id, school_id, sender_id, kind, body, body_display, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'text', $7, $7, $8)`, id, ch, seq, sender.tenant, sender.school, sender.id, body, at)
	return id
}

// report posts createReport as student st (Accept-Language lang) with key.
func (s *safetyEnv) report(st student, key, lang string, body map[string]any) resp {
	s.t.Helper()
	h := map[string]string{"X-Tenant-Id": st.tenant.String(), "Idempotency-Key": key, "Accept-Language": lang}
	return s.do(http.MethodPost, "/chat/reports", st.token, h, body)
}

// caseOf returns the live case id of a target (uuid.Nil when none).
func (s *safetyEnv) caseOf(target uuid.UUID) uuid.UUID {
	s.t.Helper()
	var id uuid.UUID
	_ = s.su.QueryRowContext(context.Background(), `SELECT case_id FROM chat.moderation_cases WHERE target_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, target).Scan(&id)
	return id
}

// caseRow reads columns of a case (superuser, assertions).
func (s *safetyEnv) caseRow(id uuid.UUID) map[string]any {
	s.t.Helper()
	var tenant uuid.UUID
	var status, severity, source, contact string
	var category, hold *string
	var count int
	var version int64
	var schools, subjects []byte
	var assignee *string
	var sla, fr time.Time
	if err := s.su.QueryRowContext(context.Background(), `SELECT tenant_id, status, severity, source, category_code, report_count,
		school_ids::text, subject_user_ids::text, hold_sanction_id::text, sla_due_at, first_response_due_at, contact_path, version,
		assignee_moderator_id::text FROM chat.moderation_cases WHERE case_id = $1`, id).Scan(&tenant, &status, &severity, &source, &category,
		&count, &schools, &subjects, &hold, &sla, &fr, &contact, &version, &assignee); err != nil {
		s.t.Fatal(err)
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return map[string]any{"tenant_id": tenant, "status": status, "severity": severity, "source": source, "category": str(category),
		"report_count": count, "school_ids": string(schools), "subjects": string(subjects), "hold": str(hold), "sla": sla, "first_response": fr,
		"contact_path": contact, "version": version, "assignee": str(assignee)}
}

// envelopes returns the outbox envelopes of type typ, oldest first.
func (s *safetyEnv) envelopes(typ string) []outbox.Envelope {
	s.t.Helper()
	rows, err := s.relay.QueryContext(context.Background(), `SELECT envelope FROM infra.outbox_events WHERE event_type = $1
		ORDER BY created_at, event_id`, typ)
	if err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []outbox.Envelope
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			s.t.Fatal(err)
		}
		var e outbox.Envelope
		if err := json.Unmarshal(raw, &e); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// noContent fails when an outbox envelope (events, audit) carries one of the placeholder texts (R14, F121-AC-25).
func (s *safetyEnv) noContent(texts ...string) {
	s.t.Helper()
	rows, err := s.relay.QueryContext(context.Background(), `SELECT event_type, envelope::text FROM infra.outbox_events`)
	if err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var typ, env string
		if err := rows.Scan(&typ, &env); err != nil {
			s.t.Fatal(err)
		}
		for _, x := range texts {
			if strings.Contains(env, x) {
				s.t.Fatalf("%s carries content %q: %s", typ, x, env)
			}
		}
	}
}

func hasAudit(list []string, action string) bool {
	for _, a := range list {
		if a == action {
			return true
		}
	}
	return false
}

// captureLogs sends every log line of the test (debug level) to a buffer (R14 checks).
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return logs
}
