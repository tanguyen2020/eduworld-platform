//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// stepCtx is the context of an identity saga step: the System Admin who approved it (platform.user.merge), the
// TenantScope of the step and its Idempotency-Key.
func stepCtx(t *testing.T, tenant uuid.UUID) context.Context {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return modcall.WithIdempotencyKey(tenancy.WithModule(tenancy.WithScope(authztest.Ctx(context.Background(),
		authztest.Platform("system_admin")), sc), "chat"), uuid.NewString())
}

func lifecycleStatusOf(t *testing.T, r any, err error) string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	switch v := r.(type) {
	case api.CleanupStudentData200JSONResponse:
		if v.BlockedReason != nil {
			return string(v.Status) + ":" + string(*v.BlockedReason)
		}
		return string(v.Status)
	case api.RestoreStudentData200JSONResponse:
		return string(v.Status)
	case api.AnonymizeUserData200JSONResponse:
		return string(v.Status)
	case api.MergeUserData200JSONResponse:
		return string(v.Status)
	}
	t.Fatalf("response %T", r)
	return ""
}

// F115 T042 — OQ F016-2, F045-3 (receiving side, chat database): cleanupStudentData is blocked while a moderation case of
// the student is open and otherwise soft-deletes its memberships (never a DELETE); restoreStudentData brings back the
// persistent ones; anonymizeUserData copies the evidence of the reported messages, then anonymises the messages of the
// user, the quotes of the replies and the projected card; mergeUserData moves the persistent memberships to the target
// and carries the blocks, mutes and active sanctions of the source. Every step is idempotent, audited without content,
// and confined to the tenant of the step.
func TestUserLifecycleFacade(t *testing.T) {
	s := setupSafety(t)
	h := command.UserLifecycle{UoW: s.uow, AfterCommit: entx.AfterCommit, Store: persistence.Lifecycle{}, Audit: persistence.Auditor{R: s.rec},
		Facts: s.facts, Social: s.social, Safety: s.cmd, EvidenceContext: 2, Batch: 2, Now: s.clock.Now}
	f := facade.Facade{Lifecycle: h}
	status := func(r any, err error) string { t.Helper(); return lifecycleStatusOf(t, r, err) }
	ctx := stepCtx(t, s.tenantA)
	an, binh, chi := s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA), s.student(s.tenantA, s.schoolA)
	openDirect := func(a, b student) uuid.UUID {
		t.Helper()
		id := uuid.Must(uuid.NewV7())
		school := a.school
		if _, err := s.channels.OpenChannel(context.Background(), command.OpenChannelInput{ID: id, TenantID: a.tenant, SchoolID: &school,
			Type: channel.Direct, ScopeRef: map[string]string{"pair": id.String()}, Members: []uuid.UUID{a.id, b.id}}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	direct := openDirect(an, binh)
	direct2 := openDirect(chi, binh)
	party := s.open(channel.Party, an, binh, chi)
	m1 := s.msg(direct, false, an, 1, "zqan mot")
	m2 := s.msg(direct, false, an, 2, "zqan hai")
	m3 := s.msg(direct, false, an, 3, "zqan ba")
	reply := s.msg(direct, false, binh, 4, "zqbinh tra loi")
	s.exec(`UPDATE chat.messages SET reply_to_message_id = $1, quote = '{"text":"zqan ba"}' WHERE message_id = $2`, m3, reply)
	members := func(ch, user uuid.UUID) (live int, reason string) {
		t.Helper()
		live = s.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2 AND deleted_at IS NULL`, ch, user)
		_ = s.su.QueryRowContext(context.Background(), `SELECT coalesce(delete_reason, '') FROM chat.channel_members WHERE channel_id = $1
			AND user_id = $2 ORDER BY updated_at DESC LIMIT 1`, ch, user).Scan(&reason)
		return live, reason
	}

	// Open case about An (report of Bình): cleanup blocked, nothing changed.
	r := s.report(binh, newKey(), "vi", map[string]any{"target_type": "message", "target_id": m2, "reason": "asked_personal_info",
		"context": map[string]any{"channel_id": direct}})
	if r.status != http.StatusCreated {
		t.Fatalf("report: %d %s", r.status, r.raw)
	}
	cleanup := func(id uuid.UUID) api.CleanupStudentDataRequestObject {
		return api.CleanupStudentDataRequestObject{StudentId: id, Body: &api.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(),
			Reason: "inactive_cleanup"}}
	}
	if st := status(f.CleanupStudentData(ctx, cleanup(an.id))); st != "blocked:open_moderation_case" {
		t.Fatalf("cleanup with an open case: %s", st)
	}
	if n, _ := members(direct, an.id); n != 1 {
		t.Fatal("a blocked cleanup changed the memberships")
	}

	// Cleanup of Chi: every active membership soft-deleted (inactive_cleanup), no row deleted; idempotent.
	rows := s.count(`SELECT count(*) FROM chat.channel_members`)
	if st := status(f.CleanupStudentData(ctx, cleanup(chi.id))); st != "done" {
		t.Fatalf("cleanup: %s", st)
	}
	for _, ch := range []uuid.UUID{direct2, party} {
		if n, reason := members(ch, chi.id); n != 0 || reason != "inactive_cleanup" {
			t.Fatalf("after cleanup %s: %d %q", ch, n, reason)
		}
	}
	if st := status(f.CleanupStudentData(ctx, cleanup(chi.id))); st != "nothing_to_do" {
		t.Fatalf("cleanup twice: %s", st)
	}
	// Tenant isolation: the same student id in the scope of tenant B finds nothing.
	ctxB := stepCtx(t, s.tenantB)
	restore := api.RestoreStudentDataRequestObject{StudentId: chi.id, Body: &api.RestoreStudentDataJSONRequestBody{Reason: "student_logged_in_again"}}
	if st := status(f.RestoreStudentData(ctxB, restore)); st != "nothing_to_do" {
		t.Fatalf("restore in tenant B: %s", st)
	}
	// Restore: the persistent membership comes back, the gameplay one (temporary) does not; idempotent.
	if st := status(f.RestoreStudentData(ctx, restore)); st != "done" {
		t.Fatalf("restore: %s", st)
	}
	if n, _ := members(direct2, chi.id); n != 1 {
		t.Fatal("direct membership not restored")
	}
	if n, _ := members(party, chi.id); n != 0 {
		t.Fatal("a gameplay membership was restored")
	}
	if st := status(f.RestoreStudentData(ctx, restore)); st != "nothing_to_do" {
		t.Fatalf("restore twice: %s", st)
	}
	if got := s.count(`SELECT count(*) FROM chat.channel_members`); got != rows {
		t.Fatalf("rows %d → %d: cleanup and restore never insert or delete", rows, got)
	}

	// Merge Bình → Bình mới: persistent memberships move, the gameplay one stays with its owner module; blocks, mutes
	// and the active sanction of the source are carried to the target; messages keep their original sender.
	newBinh := s.student(s.tenantA, s.schoolA)
	s.exec(`INSERT INTO social.blocks (block_id, tenant_id, created_at, updated_at, blocker_id, blocked_id) VALUES ($1, $2, now(), now(), $3, $4)`,
		uuid.New(), s.tenantA, chi.id, binh.id)
	s.exec(`INSERT INTO social.mutes (mute_id, tenant_id, created_at, updated_at, muter_id, muted_id) VALUES ($1, $2, now(), now(), $3, $4)`,
		uuid.New(), s.tenantA, binh.id, an.id)
	sanction := uuid.New()
	s.exec(`INSERT INTO chat.sanctions (sanction_id, tenant_id, created_at, updated_at, school_id, user_id, type, scope, reason, reason_code,
		issued_by_kind, starts_at, ends_at) VALUES ($1, $2, $3, $3, $4, $5, 'restrict_text', 'all', 'zqly do noi bo', 'moderation', 'moderator', $3, $6)`,
		sanction, s.tenantA, s.clock.Now().Add(-time.Hour), s.schoolA, binh.id, s.clock.Now().Add(24*time.Hour))
	merge := api.MergeUserDataRequestObject{UserId: binh.id, Body: &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: newBinh.id,
		Reason: "cùng một học sinh"}}
	before := map[string]int{}
	for _, typ := range []string{"user.blocked", "user.muted", "sanction.applied"} {
		before[typ] = len(s.envelopes(typ)) // the report may already have held An (case_hold)
	}
	if st := status(f.MergeUserData(ctxB, merge)); st != "nothing_to_do" {
		t.Fatalf("merge in tenant B: %s", st)
	}
	if st := status(f.MergeUserData(ctx, merge)); st != "done" {
		t.Fatalf("merge: %s", st)
	}
	for _, ch := range []uuid.UUID{direct, direct2} {
		if n, _ := members(ch, newBinh.id); n != 1 {
			t.Fatalf("target not a member of %s", ch)
		}
		if n, reason := members(ch, binh.id); n != 0 || reason != "user_merge" {
			t.Fatalf("source still a member of %s: %d %q", ch, n, reason)
		}
	}
	if n, _ := members(party, binh.id); n != 1 {
		t.Fatal("the gameplay membership follows the party events, not the merge")
	}
	if n := s.count(`SELECT count(*) FROM social.blocks WHERE blocker_id = $1 AND blocked_id = $2 AND deleted_at IS NULL`, chi.id, newBinh.id); n != 1 {
		t.Fatal("block not carried")
	}
	if n := s.count(`SELECT count(*) FROM social.mutes WHERE muter_id = $1 AND muted_id = $2 AND deleted_at IS NULL`, newBinh.id, an.id); n != 1 {
		t.Fatal("mute not carried")
	}
	if n := s.count(`SELECT count(*) FROM chat.sanctions WHERE user_id = $1 AND type = 'restrict_text' AND issued_by_kind = 'system'
		AND revoked_at IS NULL AND deleted_at IS NULL`, newBinh.id); n != 1 {
		t.Fatal("sanction not copied")
	}
	if n := s.count(`SELECT count(*) FROM chat.messages WHERE sender_id = $1`, binh.id); n != 1 {
		t.Fatal("messages must keep their original sender")
	}
	events := map[string]int{}
	for _, typ := range []string{"user.blocked", "user.muted", "sanction.applied"} {
		events[typ] = len(s.envelopes(typ))
	}
	for typ, n := range events {
		if n != before[typ]+1 {
			t.Fatalf("merge events %v (before %v)", events, before)
		}
	}
	if e := s.envelopes("sanction.applied"); !strings.Contains(string(e[len(e)-1].Payload), newBinh.id.String()) {
		t.Fatalf("sanction.applied of the copy: %s", e[len(e)-1].Payload)
	}
	if st := status(f.MergeUserData(ctx, merge)); st != "nothing_to_do" {
		t.Fatalf("merge twice: %s", st)
	}
	for typ, n := range events {
		if got := len(s.envelopes(typ)); got != n {
			t.Fatalf("replay published %s again", typ)
		}
	}

	// Anonymize An: the reported message is copied to the evidence first, the messages and the quote of the reply are
	// anonymised (in batches of 2), the card is cleared, the memberships end; idempotent.
	evidence := s.count(`SELECT count(*) FROM chat.moderation_evidence WHERE tenant_id = $1`, s.tenantA)
	anon := api.AnonymizeUserDataRequestObject{UserId: an.id, Body: &api.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(),
		Reason: "data_subject_request"}}
	if st := status(f.AnonymizeUserData(ctxB, anon)); st != "nothing_to_do" {
		t.Fatalf("anonymize in tenant B: %s", st)
	}
	if st := status(f.AnonymizeUserData(ctx, anon)); st != "done" {
		t.Fatalf("anonymize: %s", st)
	}
	if n := s.count(`SELECT count(*) FROM chat.messages WHERE sender_id = $1 AND (body IS NOT NULL OR body_display IS NOT NULL
		OR anonymized_at IS NULL)`, an.id); n != 0 {
		t.Fatalf("%d messages of the user not anonymised", n)
	}
	if n := s.count(`SELECT count(*) FROM chat.messages WHERE message_id = ANY($1::uuid[])`, []string{m1.String(), m2.String(), m3.String()}); n != 3 {
		t.Fatal("anonymisation must keep the rows")
	}
	if n := s.count(`SELECT count(*) FROM chat.messages WHERE message_id = $1 AND quote IS NULL AND body = 'zqbinh tra loi'`, reply); n != 1 {
		t.Fatal("the quote of the reply must be cleared, its own body kept")
	}
	if got := s.count(`SELECT count(*) FROM chat.moderation_evidence WHERE tenant_id = $1 AND snapshot::text LIKE '%zqan hai%'
		AND snapshot->'context'->>'source' = 'user-anonymize'`, s.tenantA); got != 1 || s.count(`SELECT count(*) FROM chat.moderation_evidence
		WHERE tenant_id = $1`, s.tenantA) != evidence+1 {
		t.Fatalf("evidence copied before anonymisation: %d (before %d)", got, evidence)
	}
	if n := s.count(`SELECT count(*) FROM ref.player_profiles WHERE student_id = $1 AND nickname IS NULL AND avatar_config IS NULL`, an.id); n != 1 {
		t.Fatal("card not cleared")
	}
	if n, reason := members(direct, an.id); n != 0 || reason != "anonymized" {
		t.Fatalf("memberships of the anonymised user: %d %q", n, reason)
	}
	if st := status(f.AnonymizeUserData(ctx, anon)); st != "nothing_to_do" {
		t.Fatalf("anonymize twice: %s", st)
	}

	// Audits: one per step that changed something, ids and counts only; no message text in any event or audit (R14).
	audits := s.audits()
	for action, want := range map[string]int{command.AuditStudentCleanedUp: 1, command.AuditStudentRestored: 1, command.AuditUserMerged: 1,
		command.AuditUserAnonymized: 1} {
		n := 0
		for _, a := range audits {
			if a == action {
				n++
			}
		}
		if n != want {
			t.Fatalf("audit %s: %d, want %d (%v)", action, n, want, audits)
		}
	}
	s.noContent("zqan mot", "zqan hai", "zqan ba", "zqbinh tra loi", "zqly do noi bo")
	s.checkEventSchemas()
	if n := s.count(`SELECT count(*) FROM chat.channel_members WHERE tenant_id = $1`, s.tenantB); n != 0 {
		t.Fatal("tenant B touched")
	}
}
