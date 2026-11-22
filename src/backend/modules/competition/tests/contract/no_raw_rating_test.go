package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// rawRating are the property names that would leak a Game Rating number (ADR-0018).
var rawRating = []string{"rating", "rd", "volatility", "rating_mu", "rating_rd", "mu"}

func walkSchema(t *testing.T, where string, s *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
	t.Helper()
	if s == nil || s.Value == nil || seen[s.Value] {
		return
	}
	v := s.Value
	seen[v] = true
	for name, p := range v.Properties {
		if slices.Contains(rawRating, name) {
			t.Errorf("%s: property %q exposes a raw rating (F106-AC-07)", where, name)
		}
		walkSchema(t, where+"."+name, p, seen)
	}
	walkSchema(t, where+"[]", v.Items, seen)
	for _, x := range append(append(append(openapi3.SchemaRefs{}, v.AllOf...), v.AnyOf...), v.OneOf...) {
		walkSchema(t, where, x, seen)
	}
}

// T016 (F106-AC-07, SC-003): no response of a route of module competition or of the leaderboards carries a raw rating
// (rating, RD, volatility) — students only ever see tiers.
func TestNoRawRatingInResponses(t *testing.T) {
	doc := spec(t)
	n := 0
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if svc, _ := op.Extensions["x-service"].(string); svc != "competition" || strings.HasPrefix(path, "/game-admin/") {
				continue
			}
			for code, r := range op.Responses.Map() {
				if !strings.HasPrefix(code, "2") || r.Value == nil {
					continue
				}
				for _, mt := range r.Value.Content {
					walkSchema(t, method+" "+path, mt.Schema, map[*openapi3.Schema]bool{})
					n++
				}
			}
		}
	}
	if n < 25 {
		t.Fatalf("only %d competition responses scanned", n)
	}
	rating := doc.Components.Schemas["Rating"].Value
	for _, want := range []string{"tier_code", "tier_name", "provisional", "provisional_games_required", "games_played"} {
		if rating.Properties[want] == nil {
			t.Errorf("Rating.%s missing", want)
		}
	}
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
}

func hasRawRating(v any) (string, bool) {
	switch x := v.(type) {
	case map[string]any:
		if props, ok := x["properties"].(map[string]any); ok {
			for name := range props {
				if slices.Contains(rawRating, name) {
					return name, true
				}
			}
		}
		for _, c := range x {
			if n, ok := hasRawRating(c); ok {
				return n, true
			}
		}
	case []any:
		for _, c := range x {
			if n, ok := hasRawRating(c); ok {
				return n, true
			}
		}
	}
	return "", false
}

// T016 (F106-AC-07): no realtime message to a client carries a raw rating; competition.tier.updated carries the tier only.
func TestNoRawRatingInRealtime(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(), "specs", "asyncapi", "eduworld-realtime.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Messages map[string]any `yaml:"messages"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Components.Messages["CompetitionTierUpdated"] == nil {
		t.Fatal("competition.tier.updated is not declared")
	}
	for name, m := range doc.Components.Messages {
		if p, ok := hasRawRating(m); ok {
			t.Errorf("realtime message %s carries %q (F106-AC-07)", name, p)
		}
	}
}

// T016 (F106-AC-09, FR-RNK-002, ADR-0018): module learning gets no Game Rating — the competition facade has no rating
// operation or field, and no learning event payload carries one.
func TestNoRatingForLearning(t *testing.T) {
	root := repoRoot()
	b, err := os.ReadFile(filepath.Join(root, "specs", "openapi", "modules", "competition.internal.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var facade map[string]any
	if err := yaml.Unmarshal(b, &facade); err != nil {
		t.Fatal(err)
	}
	if p, ok := hasRawRating(facade); ok {
		t.Errorf("the competition facade exposes %q to other modules", p)
	}
	if strings.Contains(strings.ToLower(string(b)), "operationid: getrating") {
		t.Error("a facade operation reads Game Ratings")
	}
	files, err := filepath.Glob(filepath.Join(root, "specs", "events", "payloads", "learning", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatal("no learning payloads", err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := yaml.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if p, ok := hasRawRating(v); ok {
			t.Errorf("%s carries %q", filepath.Base(f), p)
		}
	}
}
