// Package static holds source checks of module chat (F115): the chat policy and send path never read a grade (R15,
// F115-AC-09) and only chat-persister writes chat messages to PostgreSQL; no client-facing component reaches NATS or
// Valkey of the chat path (R25, F115-AC-21).
package static

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func goFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[p] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// T021 / F115-AC-09: the packages of the chat policy, the send path and its facts never reference a grade.
func TestNoGradeCondition(t *testing.T) {
	root := moduleDir(t)
	grade := regexp.MustCompile(`(?i)grade_?(level|band)|GradeLevel|GradeBand`)
	dirs := []string{"internal/domain/policy", "internal/domain/message", "internal/domain/channel", "internal/application/service",
		"internal/application/query", "internal/presentation/natscmd", "internal/presentation/persister", "internal/infrastructure/moderation",
		"internal/infrastructure/natsstream"}
	files := []string{"internal/application/command/send_message.go", "internal/application/command/open_channel.go",
		"internal/application/command/persist.go", "internal/application/port/chat.go", "internal/application/port/read/chat.go",
		"internal/infrastructure/persistence/chat.go", "internal/infrastructure/persistence/messages.go",
		"internal/infrastructure/cache/chat.go", "internal/presentation/http/chat.go", "internal/infrastructure/realtime/chat.go"}
	n := 0
	for _, d := range dirs {
		for p, src := range goFiles(t, filepath.Join(root, d)) {
			n++
			if grade.MatchString(src) {
				t.Errorf("%s references a grade", p)
			}
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		n++
		if grade.MatchString(string(b)) {
			t.Errorf("%s references a grade", f)
		}
	}
	if n < 15 {
		t.Fatalf("only %d files scanned", n)
	}
	// The policy schema of the seed has no grade either.
	seed, err := os.ReadFile(filepath.Join(root, "seed", "chat_policies.platform.json"))
	if err != nil || grade.Match(seed) {
		t.Fatalf("seed: %v", err)
	}
}

// T032 / F115-AC-21: only chat-persister (persistence/messages.go behind command.Persist) inserts chat messages; the
// realtime gateway never imports module chat nor writes PostgreSQL; nothing in module chat opens a NATS or Valkey port to
// clients (the only entry points are /api/v1/chat/* and ew.cmd.chat.* from the gateway).
func TestOnlyPersisterWritesMessages(t *testing.T) {
	root := moduleDir(t)
	backend := filepath.Join(root, "..", "..")
	write := regexp.MustCompile(`(?is)(INSERT\s+INTO|UPDATE)\s+chat\.(messages|gameplay_messages)\b`)
	owner := filepath.Join("internal", "infrastructure", "persistence", "messages.go")
	// F120-AC-24: persistent-chat-retention (and the anonymisation of a user, F115 T042) only anonymises (UPDATE
	// body/body_display/quote/anonymized_at), never inserts.
	retention := filepath.Join("internal", "infrastructure", "persistence", "retention.go")
	insert := regexp.MustCompile(`(?is)INSERT\s+INTO\s+chat\.(messages|gameplay_messages)\b`)
	found := false
	for p, src := range goFiles(t, filepath.Join(backend, "modules")) {
		if !write.MatchString(src) {
			continue
		}
		if strings.HasSuffix(p, retention) && !insert.MatchString(src) {
			continue
		}
		if !strings.HasSuffix(p, owner) {
			t.Errorf("%s writes chat messages (only %s may)", p, owner)
		}
		found = true
	}
	if !found {
		t.Fatal("no message writer found")
	}
	// Only the persister use case calls the message store.
	for p, src := range goFiles(t, filepath.Join(root, "internal")) {
		if strings.Contains(src, "persistence.Messages{}") || strings.Contains(src, "write.MessageStore") {
			base := filepath.ToSlash(p)
			if !strings.HasSuffix(base, "application/command/persist.go") && !strings.HasSuffix(base, "port/write/chat.go") &&
				!strings.HasSuffix(base, "infrastructure/persistence/messages.go") {
				t.Errorf("%s uses the message store", p)
			}
		}
	}
	mod, err := os.ReadFile(filepath.Join(root, "module.go"))
	if err != nil || strings.Count(string(mod), "persistence.Messages{}") != 1 {
		t.Fatalf("module.go must wire the message store once, in chat-persister: %v", err)
	}
	// The gateway (binary realtime) does not depend on module chat: it only routes ew.cmd.chat.* (R25).
	for _, dir := range []string{filepath.Join(backend, "modules", "realtime"), filepath.Join(backend, "cmd", "realtime")} {
		for p, src := range goFiles(t, dir) {
			if strings.Contains(src, "/modules/chat") {
				t.Errorf("%s imports module chat", p)
			}
			if strings.Contains(src, "database/sql") || strings.Contains(src, "pgx") {
				t.Errorf("%s talks to PostgreSQL", p)
			}
		}
	}
	// No listener in module chat: NATS/Valkey are reached as clients by the backend only.
	listen := regexp.MustCompile(`net\.Listen|ListenAndServe|websocket\.Accept`)
	for p, src := range goFiles(t, root) {
		if listen.MatchString(src) {
			t.Errorf("%s opens a listener", p)
		}
	}
}
