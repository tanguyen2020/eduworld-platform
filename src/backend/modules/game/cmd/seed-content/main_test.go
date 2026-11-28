package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MOCK (TEST FAKE) of the CMS list endpoints of module learning: learningRefs resolves the placeholder ids of
// learning_refs.json by code over every page, then replaceIDs rewrites them anywhere in a seed payload.
func TestLearningRefsResolveSeedPlaceholders(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/cms/learning/curricula":
			_, _ = w.Write([]byte(`{"data":[{"code":"GDPT2018_SAMPLE","current_version":{"curriculum_version_id":"v-1"}}],"page":{"has_more":false,"limit":100}}`))
		case "/api/v1/cms/learning/subjects":
			pages++
			if r.URL.Query().Get("cursor") == "" { // two pages: math is on the second one
				_, _ = w.Write([]byte(`{"data":[{"code":"english","subject_id":"s-en"}],"page":{"has_more":true,"next_cursor":"c2","limit":100}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"code":"math","subject_id":"s-math"}],"page":{"has_more":false,"limit":100}}`))
		case "/api/v1/cms/learning/objectives":
			if r.URL.Query().Get("curriculum_version_id") != "v-1" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"code":"MATH.G3.NUM.MUL_TABLE.2","objective_id":"o-mul"}],"page":{"has_more":false,"limit":100}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	base := srv.URL + "/api/v1"
	dir := filepath.Join("..", "..", "seed", "minigames")
	ids, err := learningRefs(ctx, srv.Client(), base, "tok", dir)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || ids["01936b2e-0000-7000-8000-00000000a001"] != "s-math" || ids["0193a5f4-6c1e-7b2a-9d1f-0000000720f1"] != "s-math" ||
		ids["01936b2e-0000-7000-8000-00000000c311"] != "o-mul" {
		t.Fatalf("pages %d, ids %v", pages, ids)
	}
	b, err := os.ReadFile(filepath.Join(dir, "math.g3.true_false_race.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(replaceIDs(m, ids))
	if strings.Contains(string(out), "01936b2e-0000-7000-8000-00000000") || !strings.Contains(string(out), `"subject_id":"s-math"`) ||
		!strings.Contains(string(out), `"objective_ids":["o-mul"]`) {
		t.Fatalf("placeholders left: %s", out)
	}
	if _, err := learningRefs(ctx, srv.Client(), base, "tok", t.TempDir()); err != nil {
		t.Fatalf("no learning_refs.json: %v", err)
	}
	if _, err := learningRefs(ctx, srv.Client(), base, "bad", dir); err == nil {
		t.Fatal("unauthorized list accepted")
	}
	items, _, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("load read %d seed items (learning_refs.json is not one)", len(items))
	}
}
