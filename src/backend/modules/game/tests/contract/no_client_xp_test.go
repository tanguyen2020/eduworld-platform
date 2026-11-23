package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
)

// banned are the request fields a student could use to claim XP or a score (ADR-0016, FR-RWD-003).
var banned = regexp.MustCompile(`^(xp|xp_.*|.*_xp|amount|score|.*_score)$`)

func requestFields(s *openapi3.SchemaRef, seen map[*openapi3.Schema]bool, out map[string]bool) {
	if s == nil || s.Value == nil || seen[s.Value] {
		return
	}
	seen[s.Value] = true
	for name, p := range s.Value.Properties {
		out[name] = true
		requestFields(p, seen, out)
	}
	requestFields(s.Value.Items, seen, out)
	for _, x := range append(append(s.Value.AllOf, s.Value.OneOf...), s.Value.AnyOf...) {
		requestFields(x, seen, out)
	}
}

// F074-AC-09 (SC-005, NFR-SAFE-003): no request of a student route of module game carries xp, amount or score (body or
// query); no reward rule of the seed — nor any trigger of the Reward Engine — reacts to a login, online time, speed or a
// streak.
func TestNoClientXP(t *testing.T) {
	doc := spec(t)
	checked := 0
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(gamehttp.PlayerOperations, op.OperationID) {
				continue
			}
			checked++
			fields := map[string]bool{}
			for _, p := range op.Parameters {
				if p.Value != nil {
					fields[p.Value.Name] = true
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for _, mt := range op.RequestBody.Value.Content {
					requestFields(mt.Schema, map[*openapi3.Schema]bool{}, fields)
				}
			}
			for f := range fields {
				if banned.MatchString(f) {
					t.Errorf("%s: request field %q lets the client claim XP or a score", op.OperationID, f)
				}
			}
		}
	}
	if checked < len(gamehttp.PlayerOperations) {
		t.Fatalf("checked %d of %d player operations", checked, len(gamehttp.PlayerOperations))
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "modules", "game", "seed", "progression", "reward-rules.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rules []struct {
		Trigger struct {
			EventType string `json:"event_type"`
		} `json:"trigger"`
		SourceCode string `json:"source_code"`
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatal(err)
	}
	for _, r := range append(rules, struct {
		Trigger struct {
			EventType string `json:"event_type"`
		} `json:"trigger"`
		SourceCode string `json:"source_code"`
	}{}) {
		if r.Trigger.EventType == "" {
			continue
		}
		if !slices.Contains(reward.TriggerEvents, r.Trigger.EventType) {
			t.Errorf("seed rule triggered by %s", r.Trigger.EventType)
		}
	}
	for _, e := range reward.TriggerEvents {
		for _, w := range []string{"login", "session", "online", "streak", "speed"} {
			if strings.Contains(e, w) {
				t.Errorf("trigger %s rewards %s", e, w)
			}
		}
	}
}

// F074-AC-10 (TC-GAM-003, FR-GAM-005/006): module game never calls module learning for progression and publishes no
// event that changes a score or a mastery; module learning has no consumer of xp.awarded or level.reached.
func TestProgressionDoesNotTouchLearning(t *testing.T) {
	root := filepath.Join(repoRoot(t), "src", "backend", "modules")
	consumes := regexp.MustCompile(`"(xp\.awarded|level\.reached|reward\.granted)"`)
	err := filepath.WalkDir(filepath.Join(root, "learning"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path) //nolint:gosec // repository file
		if err != nil {
			return err
		}
		if consumes.Match(b) {
			t.Errorf("module learning refers to a progression event: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	learningImport := regexp.MustCompile(`modules/learning/api|internal/infrastructure/modules"`)
	for _, dir := range []string{"internal/domain/reward", "internal/domain/progression", "internal/infrastructure/persistence/progressionsql",
		"internal/infrastructure/reward", "internal/infrastructure/progressionschema"} {
		files, _ := filepath.Glob(filepath.Join(root, "game", filepath.FromSlash(dir), "*.go"))
		for _, f := range files {
			b, err := os.ReadFile(f) //nolint:gosec // repository file
			if err != nil {
				t.Fatal(err)
			}
			if learningImport.Match(b) {
				t.Errorf("%s calls module learning", f)
			}
		}
	}
	for _, f := range []string{"progression.go", "progression_ops.go"} {
		b, err := os.ReadFile(filepath.Join(root, "game", "internal", "application", "command", f)) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "Learning") || regexp.MustCompile(`Event[A-Za-z]* *= *"(learning|mastery|score|attempt)`).Match(b) {
			t.Errorf("%s reaches module learning or publishes a learning event", f)
		}
	}
	gameEvents := regexp.MustCompile(`Event[A-Za-z]+ += "([a-z_.]+)"`)
	b, _ := os.ReadFile(filepath.Join(root, "game", "internal", "application", "command", "progression.go")) //nolint:gosec // repository file
	for _, m := range gameEvents.FindAllStringSubmatch(string(b), -1) {
		if !slices.Contains([]string{"xp.awarded", "level.reached", "reward.granted"}, m[1]) {
			t.Errorf("progression publishes %s", m[1])
		}
	}
}
