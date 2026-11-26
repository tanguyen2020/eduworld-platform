package static

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// T011 — F120-AC-01, AC-02, AC-03 (Removed — ADR-0095, negative checks): there is no class/study_group channel type (the
// domain refuses it; the OpenAPI ChannelType, the AsyncAPI ChatChannelType and the putChatPolicy scope have no class
// value) and no durable consumer of module chat subscribes class.*, enrollment.changed or study_group.*.
func TestNoClassChannelNorConsumer(t *testing.T) {
	for _, ty := range []string{"class", "study_group"} {
		if channel.Type(ty).Valid() {
			t.Fatalf("%s is a channel type", ty)
		}
	}
	root := moduleDir(t)
	forbidden := regexp.MustCompile(`"(class\.[a-z_]+|enrollment\.changed|study_group\.[a-z_]+)"`)
	for _, dir := range []string{"internal/infrastructure/messaging", "internal/presentation/consumer"} {
		for p, src := range goFiles(t, filepath.Join(root, dir)) {
			if forbidden.MatchString(src) {
				t.Errorf("%s subscribes or handles a class/enrollment/study_group event", p)
			}
		}
	}
	repo := filepath.Join(root, "..", "..", "..", "..")
	api, err := os.ReadFile(filepath.Join(repo, "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"enum: [world, area, match, party, team, direct]", "schema: { type: string, enum: [tenant, school] }"} {
		if !strings.Contains(string(api), want) {
			t.Errorf("OpenAPI lacks %q", want)
		}
	}
	rt, err := os.ReadFile(filepath.Join(repo, "specs", "asyncapi", "eduworld-realtime.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rt), "enum: [world, area, match, party, team, direct]") {
		t.Error("AsyncAPI ChatChannelType still has class/study_group")
	}
}

// T032 — F120-AC-16 (TC-AUD-004): no voice anywhere: no operation, path or realtime message names voice, the channel
// capabilities CHECK refuses voice true and every Channel the API renders says voice false.
func TestNoVoice(t *testing.T) {
	root := moduleDir(t)
	repo := filepath.Join(root, "..", "..", "..", "..")
	voice := regexp.MustCompile(`(?im)^\s*(operationId:\s*\w*voice\w*|/[^\s:]*voice[^\s:]*:|name:\s*\S*voice\S*)`)
	for _, f := range []string{filepath.Join(repo, "specs", "openapi", "eduworld-api.v1.yaml"),
		filepath.Join(repo, "specs", "asyncapi", "eduworld-realtime.v1.yaml")} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := voice.Find(b); m != nil {
			t.Errorf("%s has a voice entry: %s", filepath.Base(f), m)
		}
	}
	mig, err := os.ReadFile(filepath.Join(root, "migrations", "20261231000006_f120_persistent_clubs.sql"))
	if err != nil || !strings.Contains(string(mig), "(capabilities->>'voice') IS DISTINCT FROM 'true'") {
		t.Fatalf("capabilities CHECK missing: %v", err)
	}
	// ChannelBody (persistent.go) renders every Channel: listMyChannels and openDirectChannel (delta F119).
	for _, f := range []string{"internal/presentation/http/persistent.go"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil || !strings.Contains(string(b), `"voice": false`) {
			t.Errorf("%s does not render capabilities.voice false", f)
		}
	}
	if b, err := os.ReadFile(filepath.Join(root, "internal/presentation/http/chat.go")); err != nil || !strings.Contains(string(b), "ChannelBody(") {
		t.Error("listMyChannels does not render its channels with ChannelBody")
	}
}
