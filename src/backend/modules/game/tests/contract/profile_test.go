package contract

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
}

func read(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// T002/T003: the seed catalog v1 follows specs/schemas/character-catalog.schema.json (with avatar-config.schema.json)
// and the server rules (CheckPayload); 6 presets (avatar_creation_presets), 6 default emotes of CHARACTER_SYSTEM §7.
func TestSeedCatalogMatchesSchema(t *testing.T) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, name := range []string{"avatar-config.schema.json", "character-catalog.schema.json"} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(read(t, "specs", "schemas", name)))
		if err != nil {
			t.Fatal(err)
		}
		id := doc.(map[string]any)["$id"].(string)
		if err := c.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	sch, err := c.Compile("https://schemas.eduworld.vn/character-catalog.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	raw := read(t, "src", "backend", "modules", "game", "seed", "character_catalog.v1.json")
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(doc); err != nil {
		t.Fatal(err)
	}
	p, err := catalog.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"emote_wave", "emote_clap", "emote_dance", "emote_celebrate", "emote_thank", "emote_apologize"}
	if len(p.Presets) != 6 || !slices.Equal(p.DefaultEmoteIDs, want) {
		t.Fatalf("presets %d, emotes %v", len(p.Presets), p.DefaultEmoteIDs)
	}
	var items []catalog.ItemDefinition
	if err := json.Unmarshal(read(t, "src", "backend", "modules", "game", "seed", "items.default.v1.json"), &items); err != nil {
		t.Fatal(err)
	}
	defined := map[string]catalog.ItemDefinition{}
	for _, it := range items {
		defined[it.ItemID] = it
	}
	for _, id := range append(append([]string{}, p.DefaultItemIDs...), p.DefaultEmoteIDs...) {
		if it, ok := defined[id]; !ok || it.Name.VI == "" || it.Name.EN == "" || !slices.Contains(it.Sources, "default") {
			t.Errorf("default item %s: %+v", id, it)
		}
	}
}

// realNames are the full and given names of the seeded students (identity SIS seed, every school).
func realNames(t *testing.T) (full, given []string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "src", "backend", "modules", "identity", "seed", "sis", "*", "students.csv"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no seeded students: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // repository seed file
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		col := slices.Index(rows[0], "full_name")
		for _, r := range rows[1:] {
			name := strings.ToLower(r[col])
			full = append(full, name)
			parts := strings.Fields(name)
			given = append(given, parts[len(parts)-1])
		}
	}
	return full, given
}

// SC-002 (F070-AC-07): 10,000 suggestions from the seed words (vi and en) are at most 24 characters and never contain
// the full name or the given name (as a word) of a seeded student.
func TestSuggestionsNeverContainRealNames(t *testing.T) {
	p, err := catalog.Parse(read(t, "src", "backend", "modules", "game", "seed", "character_catalog.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	full, given := realNames(t)
	g := profile.NicknameGenerator{Rand: rand.New(rand.NewPCG(7, 70))}
	n := 0
	for _, lang := range []string{"vi", "en"} {
		w := catalog.Catalog{Payload: p}.Words(lang)
		for i := 0; i < 5000; i++ {
			s := g.One(lang, w)
			if s == "" {
				continue
			}
			n++
			if utf8.RuneCountInString(s) > profile.MaxSuggestionRunes {
				t.Fatalf("%q longer than 24", s)
			}
			lower := strings.ToLower(s)
			for _, f := range full {
				if strings.Contains(lower, f) {
					t.Fatalf("%q contains the real name %q", s, f)
				}
			}
			words := strings.Fields(lower)
			for _, gn := range given {
				if slices.Contains(words, gn) {
					t.Fatalf("%q contains the given name %q", s, gn)
				}
			}
		}
	}
	if n < 9000 {
		t.Fatalf("only %d of 10000 draws fit 24 characters", n)
	}
}

// F070-AC-12 (T022): GameProfile of the contract and the payloads game_profile.updated, avatar.updated, avatar.created
// carry no real name, username, school or class — only student id, nickname, avatar, level.
func TestProfileContractsHaveNoPII(t *testing.T) {
	forbidden := []string{"full_name", "display_name", "first_name", "last_name", "username", "login_username", "school_id",
		"school_name", "class_id", "class_name", "grade_level", "email", "phone", "birth"}
	check := func(name string, props map[string]any) {
		for k := range props {
			for _, f := range forbidden {
				if strings.Contains(k, f) {
					t.Errorf("%s exposes %s", name, k)
				}
			}
		}
	}
	for _, ev := range []string{"game_profile.updated", "avatar.updated", "avatar.created"} {
		var s struct {
			Properties           map[string]any `json:"properties"`
			AdditionalProperties *bool          `json:"additionalProperties"`
		}
		if err := json.Unmarshal(read(t, "specs", "events", "payloads", "game", ev+".v1.schema.json"), &s); err != nil {
			t.Fatal(err)
		}
		if s.AdditionalProperties == nil || *s.AdditionalProperties {
			t.Errorf("%s must forbid additional properties", ev)
		}
		check(ev, s.Properties)
	}
	doc := spec(t)
	for _, name := range []string{"GameProfile", "AvatarConfig", "AvatarPreset", "NicknameSuggestion"} {
		ref := doc.Components.Schemas[name]
		if ref == nil || ref.Value == nil {
			t.Fatalf("schema %s missing", name)
		}
		props := map[string]any{}
		for k := range ref.Value.Properties {
			props[k] = true
		}
		check(name, props)
	}
	if gp := doc.Components.Schemas["GameProfileUpdate"].Value; gp.AdditionalProperties.Has == nil || *gp.AdditionalProperties.Has {
		t.Fatal("GameProfileUpdate must refuse a free nickname (additionalProperties: false, F070-AC-10)")
	}
}
