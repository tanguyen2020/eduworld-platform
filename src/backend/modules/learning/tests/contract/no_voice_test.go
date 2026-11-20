package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
)

// F066-AC-08 (T016, TC-AUD-004): no HTTP path or realtime channel uploads a recording, a voice sample or asks a
// pronunciation score, and the registry has no plugin for speaking (ADR-0059, ADR-0072).
func TestNoVoicePath(t *testing.T) {
	root := repoRoot(t)
	voice := regexp.MustCompile(`(?i)(record(ing)?s?|voice|speech|pronunciation|speaking|microphone)`)
	pathLine := regexp.MustCompile(`^  (/[^:]*):`)
	for _, f := range []string{"specs/openapi/eduworld-api.v1.yaml", "specs/asyncapi/eduworld-realtime.v1.yaml"} {
		raw, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if m := pathLine.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil && voice.MatchString(m[1]) {
				t.Fatalf("%s: voice path %s", f, m[1])
			}
		}
	}
	if _, err := plugins.Registry().Resolve("speaking"); err == nil {
		t.Fatal("speaking has a plugin")
	}
}
