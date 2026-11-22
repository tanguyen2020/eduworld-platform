//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/purge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// purgeJob is the job gameplay-chat-purge on the pool of the real role chat_purger (DELETE through its grants and RLS only).
func (c *chatEnv) purgeJob(store port.PurgeStore) command.GameplayPurge {
	c.t.Helper()
	w, err := outbox.NewWriter("chat", "purge", nil)
	if err != nil {
		c.t.Fatal(err)
	}
	rec := outbox.NewAuditRecorder(w, c.purger)
	uow, err := entx.NewUnitOfWork(entx.Options{Module: "chat", DBs: entx.DBs{App: c.purger, Platform: c.purger}, Events: w, Audit: rec})
	if err != nil {
		c.t.Fatal(err)
	}
	return command.GameplayPurge{UoW: uow, Store: store, Events: persistence.Publisher{W: w}, Audit: persistence.Auditor{R: rec},
		Metrics: observability.Purge{}, Facts: c.facts, Now: c.clock.Now,
		Config: command.PurgeConfig{EvidenceContext: 20, Batch: 2, BacklogAlert: 48 * time.Hour, EphemeralRetention: 24 * time.Hour}}
}

func (c *chatEnv) caseOf(tenant uuid.UUID, targetType string, target uuid.UUID, ch *uuid.UUID, status string) uuid.UUID {
	c.t.Helper()
	id := uuid.New()
	c.exec(`INSERT INTO chat.moderation_cases (case_id, tenant_id, school_ids, status, severity, source, target_type, target_id, channel_id,
		channel_type, subject_user_ids, first_response_due_at, sla_due_at)
		VALUES ($1, $2, '[]', $3, 'p2', 'report', $4, $5, $6, 'party', '[]', now(), now())`, id, tenant, status, targetType, target, ch)
	return id
}

func (c *chatEnv) report(tenant, reporter uuid.UUID, message, caseID uuid.UUID, status string) {
	c.t.Helper()
	c.exec(`INSERT INTO chat.reports (report_id, tenant_id, reporter_id, target_type, target_id, reason, case_id, status)
		VALUES ($1, $2, $3, 'message', $4, 'bullying', $5, $6)`, uuid.New(), tenant, reporter, message, caseID, status)
}

func (c *chatEnv) messageIDs(ch uuid.UUID) []uuid.UUID {
	c.t.Helper()
	rows, err := c.su.Query(`SELECT message_id FROM chat.gameplay_messages WHERE channel_id = $1 ORDER BY seq`, ch)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			c.t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

func (c *chatEnv) exists(ch uuid.UUID) bool {
	return c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1`, ch) == 1
}

// purgeFault is a TEST DOUBLE (R17) around the real purge store: ready=false simulates the moderation module missing;
// failAfter makes PurgeChannel fail after the real work, so the transaction must roll every step back.
type purgeFault struct {
	purge.Store
	notReady  bool
	failAfter bool
}

func (s purgeFault) ModerationReady(ctx context.Context) (bool, error) {
	if s.notReady {
		return false, nil
	}
	return s.Store.ModerationReady(ctx)
}

func (s purgeFault) PurgeChannel(ctx context.Context, c port.ExpiredChannel, n, batch int, at time.Time) (port.ChannelPurge, error) {
	out, err := s.Store.PurgeChannel(ctx, c, n, batch, at)
	if err == nil && s.failAfter {
		return out, errors.New("TEST DOUBLE: evidence store failed")
	}
	return out, err
}

// T026–T030 — F117-AC-15 … AC-23 (TC-CHAT-021 … TC-CHAT-024, TC-SOC-009): the job, connected as chat_purger, keeps a channel
// until purge_after, defers it while a case is open, copies every referenced message with its context into the evidence
// (readable after the purge) before deleting messages, members and channel in batches, rolls everything back when a step
// fails, deletes nothing while the moderation sources are missing, drops a daily partition once every channel of that day
// is purged, purges world/area messages past their retention after the evidence copy, records runs, audits and events
// without content — and never touches a club team channel, a direct channel or chat.messages.
func TestGameplayPurge(t *testing.T) {
	c := setupChat(t, chatOpts{})
	ctx := context.Background()
	day1 := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	c.clock.now = day1
	job := c.purgeJob(purge.Store{})
	if err := job.UoW.Do(ctx, func(ctx context.Context) error {
		_, _, err := purge.Store{}.EnsurePartitions(ctx, day1.AddDate(0, 0, -2), 10)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	other := c.student(c.tenantB, c.schoolB)
	// Durable channels: a club team channel (no match_id) and a direct channel, each with a message (chat.messages).
	club := uuid.New()
	if _, err := c.channels.OpenChannel(ctx, command.OpenChannelInput{ID: club, TenantID: c.tenantA, Type: channel.Team,
		ScopeRef: map[string]string{"club_id": uuid.NewString()}, Members: []uuid.UUID{an.id, binh.id}}); err != nil {
		t.Fatal(err)
	}
	direct := c.open(channel.Direct, an, binh)
	for _, ch := range []uuid.UUID{club, direct} {
		if r := c.sendWS(an, "conn-an", ch, "ben vung"); !r.OK {
			t.Fatalf("durable send: %+v", r.Problem)
		}
	}
	// Party P1 of day 1: three messages, the second reported (case resolved, report linked).
	p1 := c.openParty(an, binh)
	for _, body := range []string{"mot", "hai", "ba"} {
		if r := c.sendWS(binh, "conn-binh", p1, body); !r.OK {
			t.Fatalf("party send: %+v", r.Problem)
		}
	}
	// Party P2 without messages; party P3 of tenant B (isolation of the purge transactions).
	p2 := c.openParty(binh, an)
	p3 := c.openParty(other)
	if r := c.sendWS(other, "conn-o", p3, "tenant b"); !r.OK {
		t.Fatalf("tenant B send: %+v", r.Problem)
	}
	c.waitCount(2, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id IN ($1, $2)`, club, direct)
	c.waitCount(3, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, p1)
	c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, p3)
	ms := c.messageIDs(p1)
	resolved := c.caseOf(c.tenantA, "message", ms[1], &p1, "resolved")
	c.report(c.tenantA, an.id, ms[1], resolved, "linked")
	// F118 side rows (edit evidence, reaction, pin) of the party P1 and of the durable club channel.
	c.exec(`INSERT INTO chat.message_revisions (tenant_id, channel_id, message_id, revision, state, body, body_display, decision, edited_at)
		VALUES ($1, $2, $3, 1, 'previous', 'hai cu', 'hai cu', 'allow', now())`, c.tenantA, p1, ms[1])
	for _, ch := range []uuid.UUID{p1, club} {
		msg := uuid.New()
		if ch == p1 {
			msg = ms[0]
		}
		c.exec(`INSERT INTO chat.message_reactions (tenant_id, channel_id, message_id, user_id, emoji_code) VALUES ($1, $2, $3, $4, 'heart')`,
			c.tenantA, ch, msg, an.id)
		c.exec(`INSERT INTO chat.pinned_messages (tenant_id, channel_id, message_id, pinned_by) VALUES ($1, $2, $3, $4)`, c.tenantA, ch, msg, an.id)
	}
	// Match M1 on day 2 with an open case on its channel (deferred).
	c.clock.now = day1.AddDate(0, 0, 1)
	m1 := uuid.New()
	c.mustCompetition("match.started", c.tenantA, matchStarted(m1, "battle_1v1", participants(map[int][]student{0: {an}, 1: {binh}})))
	m1ch := chatid.Match(m1)
	if r := c.sendWS(an, "conn-an", m1ch, "tran dau"); !r.OK {
		t.Fatalf("match send: %+v", r.Problem)
	}
	c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, m1ch)
	openCase := c.caseOf(c.tenantA, "user", an.id, &m1ch, "in_review")
	// Close everything on day 2 (retention: default 7 days).
	closedAt := day1.AddDate(0, 0, 1).Add(time.Hour)
	for _, p := range []struct {
		ch     uuid.UUID
		tenant uuid.UUID
	}{{p1, c.tenantA}, {p2, c.tenantA}, {p3, c.tenantB}} {
		var party uuid.UUID
		if err := c.su.QueryRow(`SELECT (scope_ref->>'party_id')::uuid FROM chat.channels WHERE channel_id = $1`, p.ch).Scan(&party); err != nil {
			t.Fatal(err)
		}
		c.mustCompetition("party.disbanded", p.tenant, map[string]any{"party_id": party, "tenant_id": p.tenant, "reason": "all_left",
			"disbanded_at": closedAt, "state_version": 9})
	}
	c.mustCompetition("match.completed", c.tenantA, map[string]any{"match_id": m1, "results": participants(map[int][]student{0: {an}, 1: {binh}}),
		"ended_at": closedAt})
	// Ephemeral world messages: day 1 (one reported), a day without partition (default partition), and a recent one.
	world := uuid.New()
	ephemeral := []struct {
		at   time.Time
		body string
	}{{day1, "w1"}, {day1.Add(time.Minute), "w2"}, {day1.Add(2 * time.Minute), "w3"}, {time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC), "old"},
		{closedAt.AddDate(0, 0, 7), "recent"}}
	var eph []uuid.UUID
	for _, m := range ephemeral {
		id := uuid.New()
		eph = append(eph, id)
		c.exec(`INSERT INTO chat.ephemeral_messages (message_id, channel_id, tenant_id, school_id, sender_id, kind, body, created_at, created_on)
			VALUES ($1, $2, $3, $4, $5, 'text', $6, $7, ($7 AT TIME ZONE 'UTC')::date)`, id, world, c.tenantA, c.schoolA, an.id, m.body, m.at)
	}
	worldCase := c.caseOf(c.tenantA, "message", eph[1], &world, "resolved")
	c.report(c.tenantA, binh.id, eph[1], worldCase, "linked")
	durable := c.count(`SELECT count(*) FROM chat.messages`) + c.count(`SELECT count(*) FROM chat.channels WHERE NOT is_gameplay`)
	purgeAfter := closedAt.AddDate(0, 0, 7)

	// Before purge_after: nothing goes (F117-AC-16, D+N−1).
	c.clock.now = purgeAfter.Add(-time.Hour)
	if res, err := job.RunOnce(ctx); err != nil || res.ChannelsPurged != 0 || !c.exists(p1) {
		t.Fatalf("purged before purge_after: %+v %v", res, err)
	}
	// Moderation sources missing: nothing deleted, run blocked (F117-AC-19).
	c.clock.now = purgeAfter.Add(time.Hour)
	blocked := c.purgeJob(purgeFault{notReady: true})
	if res, err := blocked.RunOnce(ctx); err != nil || !res.Blocked || !c.exists(p1) || !c.exists(p2) {
		t.Fatalf("blocked run deleted: %+v %v", res, err)
	}
	if n := c.count(`SELECT count(*) FROM chat.gameplay_purge_runs WHERE status = 'blocked' AND tenant_id IS NULL`); n != 2 {
		t.Fatalf("blocked run rows: %d", n)
	}
	// A failure after the work of a channel rolls the whole channel back (fail-closed).
	failing := c.purgeJob(purgeFault{failAfter: true})
	if res, err := failing.RunOnce(ctx); err != nil || res.Failed == 0 || res.ChannelsPurged != 0 {
		t.Fatalf("failing run: %+v %v", res, err)
	}
	if !c.exists(p1) || c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, p1) != 3 ||
		c.count(`SELECT count(*) FROM chat.moderation_evidence WHERE snapshot->>'kind' = 'gameplay_purge'`) != 0 ||
		c.count(`SELECT count(*) FROM chat.gameplay_chat_daily_stats`) != 0 {
		t.Fatal("a failed channel purge left partial effects")
	}

	// D+N: P1, P2 (tenant A) and P3 (tenant B) go; M1 is deferred by its open case (F117-AC-18).
	res, err := job.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ChannelsPurged != 3 || res.Deferred != 1 || res.Failed != 0 {
		t.Fatalf("run: %+v", res)
	}
	for _, ch := range []uuid.UUID{p1, p2, p3} {
		if c.exists(ch) || c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1`, ch) != 0 ||
			c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch) != 0 {
			t.Fatalf("channel %s not fully purged", ch)
		}
	}
	if !c.exists(m1ch) || c.count(`SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, m1ch) != 1 {
		t.Fatal("the channel with an open case was purged")
	}
	// F118 (left by F117): revisions, reactions and pins of the purged channel are gone, the durable club keeps its rows,
	// and the evidence of the reported message carries its earlier version (F118-AC-06).
	for _, tb := range []string{"message_revisions", "message_reactions", "pinned_messages"} {
		if n := c.count(`SELECT count(*) FROM chat.`+tb+` WHERE channel_id = $1`, p1); n != 0 {
			t.Fatalf("%s of the purged channel left: %d", tb, n)
		}
	}
	if c.count(`SELECT count(*) FROM chat.message_reactions WHERE channel_id = $1`, club) != 1 ||
		c.count(`SELECT count(*) FROM chat.pinned_messages WHERE channel_id = $1`, club) != 1 {
		t.Fatal("the purge touched the side rows of a durable channel")
	}
	if n := c.count(`SELECT count(*) FROM chat.moderation_evidence e, jsonb_array_elements(e.snapshot->'messages') m,
		jsonb_array_elements(m->'revisions') r WHERE e.case_id = $1 AND r->>'body' = 'hai cu'`, resolved); n != 1 {
		t.Fatalf("evidence without the revision of the reported message: %d", n)
	}
	// Evidence of the reported message: original content, sender, time, channel type, party id, context (F117-AC-17).
	var target, kind, channelType, partyID, body string
	var n int
	if err := c.su.QueryRow(`SELECT snapshot->>'target_id', snapshot->>'kind', snapshot->'context'->>'channel_type',
		coalesce(snapshot->'context'->>'party_id', ''), jsonb_array_length(snapshot->'messages'),
		(SELECT m->>'body' FROM jsonb_array_elements(snapshot->'messages') m WHERE m->>'message_id' = snapshot->>'target_id')
		FROM chat.moderation_evidence WHERE case_id = $1 AND snapshot->>'kind' = 'gameplay_purge'`, resolved).
		Scan(&target, &kind, &channelType, &partyID, &n, &body); err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if target != ms[1].String() || channelType != "party" || partyID == "" || n != 3 || body != "hai" {
		t.Fatalf("evidence snapshot: target=%s type=%s party=%s n=%d body=%q", target, channelType, partyID, n, body)
	}
	scope, err := authz.EventScope(c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	var ev []read.EvidenceRow
	err = c.read.InTenantRead(ctx, scope, func(ctx context.Context) error {
		var err error
		ev, err = persistence.Safety{}.CaseEvidence(ctx, c.tenantA, resolved)
		return err
	})
	if err != nil || len(ev) == 0 {
		t.Fatalf("evidence unreadable by the Safety Team after the purge: %v %d", err, len(ev))
	}
	// Rollup kept (F117-AC-16): day 1, party, 3 messages, 1 reported.
	if n := c.count(`SELECT count(*) FROM chat.gameplay_chat_daily_stats WHERE tenant_id = $1 AND day = '2026-11-01'
		AND channel_type = 'party' AND channels = 1 AND messages = 3 AND reported = 1`, c.tenantA); n != 1 {
		t.Fatal("daily stats of day 1 missing")
	}
	// The daily partition of day 1 is dropped (every channel of the day purged); day 2 (M1 deferred) stays.
	if n := c.count(`SELECT count(*) FROM pg_class WHERE relname = 'gameplay_messages_p20261101'`); n != 0 {
		t.Fatal("partition of day 1 not dropped")
	}
	if n := c.count(`SELECT count(*) FROM pg_class WHERE relname = 'gameplay_messages_p20261102'`); n != 1 {
		t.Fatal("partition of day 2 dropped although M1 is not purged")
	}
	// Ephemeral (F117-AC-21): evidence of the reported world message, day partition dropped, old default rows deleted,
	// the recent message kept.
	if n := c.count(`SELECT count(*) FROM chat.moderation_evidence WHERE case_id = $1 AND snapshot->>'kind' = 'ephemeral_purge'
		AND snapshot->>'target_id' = $2`, worldCase, eph[1].String()); n != 1 {
		t.Fatal("ephemeral evidence missing")
	}
	if n := c.count(`SELECT count(*) FROM pg_class WHERE relname = 'ephemeral_messages_p20261101'`); n != 0 {
		t.Fatal("ephemeral partition of day 1 not dropped")
	}
	if n := c.count(`SELECT count(*) FROM chat.ephemeral_messages`); n != 1 ||
		c.count(`SELECT count(*) FROM chat.ephemeral_messages WHERE body = 'recent'`) != 1 {
		t.Fatalf("ephemeral rows left: %d", n)
	}
	// After the purge every chat API answers 404 for the channel (F117-AC-23).
	if r := c.call(an, http.MethodGet, "/chat/channels/"+p1.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("history of a purged channel: %d", r.status)
	}
	if r := c.sendWS(an, "conn-an", p1, "con day khong"); r.OK || replyCode(r) != "NOT_FOUND" {
		t.Fatalf("send to a purged channel: %+v", r)
	}
	if lr := c.call(an, http.MethodGet, "/chat/channels", "", nil); strings.Contains(lr.raw, p1.String()) {
		t.Fatal("listMyChannels still lists the purged channel")
	}
	// Durable data untouched (F117-AC-20, SC-002).
	if got := c.count(`SELECT count(*) FROM chat.messages`) + c.count(`SELECT count(*) FROM chat.channels WHERE NOT is_gameplay`); got != durable {
		t.Fatalf("durable rows %d → %d", durable, got)
	}
	// Run records, audit and events without content (F117-AC-22).
	if n := c.count(`SELECT count(*) FROM chat.gameplay_purge_runs WHERE tenant_id = $1 AND kind = 'gameplay' AND channels_purged = 2
		AND deferred_open_case = 1 AND evidence_copied = 1 AND messages_purged = 3 AND status = 'succeeded'`, c.tenantA); n != 1 {
		t.Fatal("run record of tenant A")
	}
	if n := c.count(`SELECT count(*) FROM chat.gameplay_purge_runs WHERE run_id = $1 AND tenant_id IS NULL AND kind = 'gameplay'
		AND channels_purged = 3 AND deferred_open_case = 1 AND partitions_dropped = 1`, res.RunID); n != 1 {
		t.Fatalf("run-level record of the run (res %+v)", res)
	}
	audits := 0
	for _, a := range c.audits() {
		if a == command.AuditGameplayPurgeRun {
			audits++
		}
	}
	if audits < 4 {
		t.Fatalf("purge audits: %d", audits)
	}
	for _, p := range c.events(command.EventGameplayPurged) {
		for _, k := range []string{"body", "message_id", "user_id", "sender_id"} {
			if _, ok := p[k]; ok {
				t.Fatalf("chat.gameplay_purged carries %s", k)
			}
		}
	}
	seen := c.checkEvents(command.EventGameplayPurged, command.EventDailyStats, command.EventChannelClosed)
	if seen[command.EventGameplayPurged] < 4 || seen[command.EventDailyStats] < 1 {
		t.Fatalf("events: %v", seen)
	}
	// The case closes → purged at the next run.
	c.exec(`UPDATE chat.moderation_cases SET status = 'resolved' WHERE case_id = $1`, openCase)
	if res, err := job.RunOnce(ctx); err != nil || res.ChannelsPurged != 1 || c.exists(m1ch) {
		t.Fatalf("after the case closed: %+v %v", res, err)
	}
}

// T009 / F117-AC-20 (TC-TEN-016, TC-CHAT-024): with the real roles, chat_purger deletes only the gameplay tables of the
// allow-list, only closed gameplay channels (never a club team or direct channel, never chat.messages) and cannot TRUNCATE;
// chat_app and chat_platform cannot DELETE or TRUNCATE anything; scripts/ci/db-privileges.sql finds no violation.
func TestPurgerPrivileges(t *testing.T) {
	c := setupChat(t, chatOpts{})
	ctx := context.Background()
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	club := uuid.New()
	if _, err := c.channels.OpenChannel(ctx, command.OpenChannelInput{ID: club, TenantID: c.tenantA, Type: channel.Team,
		ScopeRef: map[string]string{"club_id": uuid.NewString()}, Members: []uuid.UUID{an.id, binh.id}}); err != nil {
		t.Fatal(err)
	}
	direct := c.open(channel.Direct, an, binh)
	open := c.openParty(an, binh)
	purger := func(q string, args ...any) (int64, error) {
		tx, err := c.purger.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, c.tenantA.String()); err != nil {
			return 0, err
		}
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	// tenancyguard:ok privilege probes of the test (expected to delete nothing)
	for _, ch := range []uuid.UUID{club, direct, open} {
		if n, err := purger(`DELETE FROM chat.channel_members WHERE channel_id = $1`, ch); err != nil || n != 0 {
			t.Fatalf("purger deleted members of a non-purgeable channel: %d %v", n, err)
		}
		if n, err := purger(`DELETE FROM chat.channels WHERE channel_id = $1`, ch); err == nil && n != 0 {
			t.Fatalf("purger deleted a non-purgeable channel: %d", n)
		}
	}
	for _, q := range []string{`DELETE FROM chat.messages`, `DELETE FROM chat.moderation_evidence`, `DELETE FROM chat.reports`,
		`DELETE FROM social.blocks`, `TRUNCATE chat.gameplay_messages`, `DELETE FROM chat.chat_policies`} {
		if _, err := purger(q); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("purger %q: %v", q, err)
		}
	}
	for _, db := range []struct {
		name string
		dsn  string
	}{{"chat_app", "app"}, {"chat_platform", "platform"}} {
		pool := c.app
		if db.dsn == "platform" {
			pool = c.platform
		}
		for _, q := range []string{`DELETE FROM chat.gameplay_messages`, `DELETE FROM chat.channels`, `DELETE FROM chat.ephemeral_messages`,
			`TRUNCATE chat.gameplay_purge_runs`} {
			if _, err := pool.ExecContext(ctx, q); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("%s %q: %v", db.name, q, err)
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "scripts", "ci", "db-privileges.sql"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.su.QueryContext(ctx, string(b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var v, role, obj, priv string
		_ = rows.Scan(&v, &role, &obj, &priv)
		t.Errorf("db-privileges violation: %s %s %s %s", v, role, obj, priv)
	}
	if n := c.count(`SELECT count(*) FROM information_schema.role_table_grants WHERE grantee = 'chat_purger' AND privilege_type = 'DELETE'`); n != 8 {
		t.Fatalf("chat_purger DELETE grants: %d (want the 5 gameplay tables + revisions, reactions, pins of F118)", n)
	}
}
