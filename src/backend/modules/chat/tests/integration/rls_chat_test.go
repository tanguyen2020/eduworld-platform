//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T008 — TC-TEN-005, TC-TEN-016: the chat tables of F115 are tenant-isolated by RLS (tenant A never reads B; no
// app.tenant_id → 0 rows; the platform default policy is readable by every tenant) and soft-delete only: chat_app and
// chat_platform cannot DELETE nor TRUNCATE.
func TestChatRLSAndPrivileges(t *testing.T) {
	e := setup(t, envOpts{})
	ctx := context.Background()
	chA, chB := uuid.New(), uuid.New()
	for _, x := range []struct{ ch, tenant uuid.UUID }{{chA, e.tenantA}, {chB, e.tenantB}} {
		e.exec(`INSERT INTO chat.channels (channel_id, tenant_id, channel_type, is_gameplay, scope_ref, status, opened_at, created_at, updated_at)
			VALUES ($1, $2, 'party', true, '{"party_id":"p"}', 'open', now(), now(), now())`, x.ch, x.tenant)
		e.exec(`INSERT INTO chat.messages (message_id, channel_id, seq, tenant_id, sender_id, kind, body, body_display, created_at)
			VALUES ($1, $2, 1, $3, $4, 'text', 'x', 'x', now())`, uuid.New(), x.ch, x.tenant, uuid.New())
		e.exec(`INSERT INTO chat.gameplay_messages (message_id, channel_id, seq, tenant_id, sender_id, kind, body, body_display, created_at, created_on)
			VALUES ($1, $2, 1, $3, $4, 'text', 'x', 'x', now(), current_date)`, uuid.New(), x.ch, x.tenant, uuid.New())
	}
	count := func(tenant *uuid.UUID, q string) int {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != nil {
			if _, err := tx.Exec(`SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
				t.Fatal(err)
			}
		}
		var n int
		if err := tx.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range []string{"chat.channels", "chat.messages", "chat.gameplay_messages"} {
		if n := count(&e.tenantA, `SELECT count(*) FROM `+table); n != 1 {
			t.Errorf("%s as tenant A: %d", table, n)
		}
		if n := count(nil, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s without tenant: %d", table, n)
		}
	}
	if n := count(&e.tenantA, `SELECT count(*) FROM chat.chat_policies WHERE scope_type = 'platform'`); n != 1 {
		t.Errorf("platform policy visible to a tenant: %d", n)
	}
	for role, db := range map[string]*sql.DB{"app": e.app, "platform": e.platform} {
		for _, table := range []string{"chat.channels", "chat.channel_members", "chat.chat_policies", "chat.messages", "chat.gameplay_messages",
			"chat.gameplay_partition_channels", "chat.messages_default", "chat.gameplay_messages_default"} {
			for _, stmt := range []string{refused("DEL", "ETE FR", "OM "+table), refused("TRUN", "CATE "+table)} {
				if _, err := db.ExecContext(ctx, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
					t.Errorf("chat_%s %s: %v", role, stmt, err)
				}
			}
		}
	}
}

// T009: the platform default policy seeded by the migrations equals modules/chat/seed/chat_policies.platform.json, which
// is valid against specs/schemas/chat-policy.schema.json v5 and has the 6 channel types of CHAT_SYSTEM §3.1 (no
// class/study_group since ADR-0095).
func TestSeedPlatformPolicy(t *testing.T) {
	e := setup(t, envOpts{})
	root := testkit.RepoRoot(t)
	seed, err := os.ReadFile(filepath.Join(root, "src", "backend", "modules", "chat", "seed", "chat_policies.platform.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	schema, err := c.Compile(filepath.Join(root, "specs", "schemas", "chat-policy.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(seed)))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var want, got map[string]any
	_ = json.Unmarshal(seed, &want)
	var raw []byte
	if err := e.su.QueryRowContext(context.Background(), `SELECT rules FROM chat.chat_policies WHERE scope_type = 'platform'
		AND tenant_id IS NULL AND deleted_at IS NULL`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatal("platform policy in the database differs from the seed file")
	}
	types := map[string]bool{}
	for _, r := range want["rules"].([]any) {
		types[r.(map[string]any)["channel_type"].(string)] = true
	}
	if len(types) != 6 || types["class"] || types["study_group"] {
		t.Fatalf("channel types %v", types)
	}
	if regexp.MustCompile(`(?i)grade`).Match(seed) {
		t.Fatal("seed has a grade condition")
	}
}
