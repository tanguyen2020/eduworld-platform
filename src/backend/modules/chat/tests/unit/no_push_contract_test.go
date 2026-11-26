package unit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
}

// pushWords are the push channels outside the game (mobile push, Web Push, e-mail, SMS) a contract or the chat module must
// not offer to students (FR-NTF-003).
var pushWords = regexp.MustCompile(`(?i)(push[_-]?token|device[_-]?token[_-]?register|fcm|apns|web[_-]?push|vapid|onesignal|firebase|` +
	`/push(/|:|$)|push[_-]?subscription|sms[_-]?(gateway|provider)|smtp)`)

// T022 / F123-AC-13: the public and realtime contracts have no operation, path, schema or message to register a push
// token or subscribe to Web Push; the notification inbox is the only way besides the realtime connection.
func TestNoPushContract(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"specs/openapi/eduworld-api.v1.yaml", "specs/openapi/modules/chat.v1.yaml",
		"specs/asyncapi/eduworld-realtime.v1.yaml", "specs/openapi/modules/chat.internal.v1.yaml"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			// Paths, operation ids, schema/property names and message names; descriptions may say "no push".
			if strings.HasPrefix(trimmed, "description:") || strings.HasPrefix(trimmed, "summary:") || strings.HasPrefix(trimmed, "#") ||
				strings.HasPrefix(trimmed, "- '") || strings.HasPrefix(trimmed, "title:") {
				continue
			}
			if m := pushWords.FindString(trimmed); m != "" {
				t.Errorf("%s:%d: push channel %q: %s", rel, i+1, m, trimmed)
			}
		}
	}
}

// T022 / F123-AC-13: module chat has no client that sends anything to a student outside the game (no SMTP, SMS or push
// package imported; no push token stored).
func TestNoPushClientInChat(t *testing.T) {
	root := filepath.Join(repoRoot(t), "src", "backend", "modules", "chat")
	imports := regexp.MustCompile(`"(net/smtp|[^"]*(firebase|onesignal|apns|webpush|twilio|sendgrid|vonage|nexmo)[^"]*)"`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := imports.FindString(string(raw)); m != "" {
			t.Errorf("%s imports %s", path, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migrations {
		raw, _ := os.ReadFile(m)
		if loc := regexp.MustCompile(`(?i)(push_token|device_token|fcm_token|apns_token|push_subscription)`).FindString(string(raw)); loc != "" {
			t.Errorf("%s stores %s", filepath.Base(m), loc)
		}
	}
}
