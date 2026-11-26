package configschema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

const specs = "../../../../../../../specs/schemas"

// TestEmbeddedCopiesMatchSpecs keeps the embedded schemas equal to specs/schemas (single source, R33).
func TestEmbeddedCopiesMatchSpecs(t *testing.T) {
	for _, file := range FileOf {
		want, err := os.ReadFile(filepath.Join(specs, file))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))) {
			t.Fatalf("%s differs from specs/schemas: copy it again", file)
		}
	}
	if len(FileOf) != len(gameconfig.Types()) {
		t.Fatal("a config type has no schema")
	}
}

// T007 — the seeds of F104/F106/F107 conform to the schemas; schema violations name their field (F108-AC-04).
func TestSeedsConformAndViolationsNameFields(t *testing.T) {
	read := func(f string) []byte {
		b, err := os.ReadFile("../../../seed/" + f)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for typ, f := range map[string]string{gameconfig.TypeMatchmaking: "matchmaking_config.json", gameconfig.TypeRanking: "ranking_rules.json",
		gameconfig.TypeSeasons: "seasons.json", gameconfig.TypeLeaderboards: "leaderboard_definitions.json"} {
		if vs, err := (Validator{}).Validate(typ, read(f)); err != nil || len(vs) != 0 {
			t.Fatalf("%s: %v %+v", typ, err, vs)
		}
	}
	var tpl struct {
		Templates []struct {
			ConfigType string          `json:"config_type"`
			Payload    json.RawMessage `json:"payload"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(read("tournament_templates.json"), &tpl); err != nil {
		t.Fatal(err)
	}
	for _, x := range tpl.Templates {
		if vs, err := (Validator{}).Validate(x.ConfigType, x.Payload); err != nil || len(vs) != 0 {
			t.Fatalf("%s: %v %+v", x.ConfigType, err, vs)
		}
	}
	var mm map[string]any
	_ = json.Unmarshal(read("matchmaking_config.json"), &mm)
	mm["cost_weights"] = map[string]any{"rating": 1, "rd": 0.1, "recent_opponent": 0.5, "same_class": 0.2}
	mm["grade_window_g1_5"] = map[string]any{"after_s": 30, "levels": 1}
	b, _ := json.Marshal(mm)
	vs, err := (Validator{}).Validate(gameconfig.TypeMatchmaking, b)
	if err != nil || len(vs) == 0 {
		t.Fatalf("class weight / grade window for grades 1–5 accepted: %v", err)
	}
	fields := map[string]bool{}
	for _, v := range vs {
		fields[v.Field] = true
	}
	if !fields["payload"] && !fields["payload/cost_weights"] {
		t.Fatalf("violations: %+v", vs)
	}
}
