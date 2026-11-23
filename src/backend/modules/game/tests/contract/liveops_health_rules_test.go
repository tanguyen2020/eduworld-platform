package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
)

// gameRoot is the directory of module game (this package is three levels below it).
const gameRoot = "../.."

// pushWords are the producers of a notification outside the game (push, e-mail, SMS) — never sent for an event
// (EVENT_SEASON_SYSTEM §9, PRD FR-NTF-003).
var pushWords = regexp.MustCompile(`(?i)("notification\.|push_notification|fcm|apns|send_email|sms\.)`)

// streakWords are the sources of a reward for showing up (login, check-in, streak, online time — R19, NFR-SAFE-003).
var streakWords = regexp.MustCompile(`(?i)(login|check_?in|streak|online|session|minute|attendance)`)

// F077-AC-29 (T035, TC-EVT-009): the live-ops code produces no notification outside the game; the point sources hold no
// login, check-in, streak or online-time source; the ledgers of event and season points refuse a debit (a missed day never
// takes points) — CHECK delta > 0 in the migration.
func TestLiveOpsHealthRules(t *testing.T) {
	var files []string
	for _, dir := range []string{"internal/application/command", "internal/domain/liveops", "internal/domain/season",
		"internal/presentation/consumer", "internal/presentation/job", "internal/infrastructure/persistence/liveopssql"} {
		m, err := filepath.Glob(filepath.Join(gameRoot, dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range m {
			if strings.Contains(filepath.Base(f), "liveops") || strings.Contains(dir, "liveops") || strings.Contains(dir, "season") {
				files = append(files, f)
			}
		}
	}
	if len(files) < 6 {
		t.Fatalf("live-ops sources not found: %v", files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f) //nolint:gosec // repository source
		if err != nil {
			t.Fatal(err)
		}
		if m := pushWords.FindString(string(b)); m != "" {
			t.Errorf("%s produces a notification outside the game (%s)", f, m)
		}
	}
	for _, s := range liveops.SourceCodes {
		if streakWords.MatchString(s) {
			t.Errorf("source %s rewards showing up", s)
		}
	}
	mig, err := filepath.Glob(filepath.Join(gameRoot, "migrations", "*_f077_liveops.sql"))
	if err != nil || len(mig) != 1 {
		t.Fatalf("migration f077: %v %v", mig, err)
	}
	b, err := os.ReadFile(mig[0]) //nolint:gosec // repository migration
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"event_point_ledger_delta_chk CHECK (delta > 0)", "season_point_ledger_delta_chk CHECK (delta > 0)",
		"community_goal_contributions_amount_chk CHECK (amount > 0)", "REVOKE UPDATE, DELETE, TRUNCATE ON game.event_point_ledger",
		"REVOKE UPDATE, DELETE, TRUNCATE ON game.season_point_ledger", "(payload->>'paid_track') = 'false'"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("migration lacks %q", want)
		}
	}
}
