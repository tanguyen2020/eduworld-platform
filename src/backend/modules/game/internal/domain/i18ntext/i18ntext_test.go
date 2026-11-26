package i18ntext_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

var req = []string{"vi", "en"}

func doc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestTextsFindsBilingualNodesInArraysAndNesting(t *testing.T) {
	d := doc(t, `{"name":{"vi":"A","en":"B"},"code":"x","steps":[{"title":{"vi":"T"}},{"title":{"vi":"U","en":"V"}}],
		"meta":{"vi":1,"other":"no"},"a/b":{"vi":"s"}}`)
	got := i18ntext.Texts(d)
	var ptrs []string
	for _, x := range got {
		ptrs = append(ptrs, x.Pointer)
	}
	want := []string{"/a~1b", "/name", "/steps/0/title", "/steps/1/title"}
	if !reflect.DeepEqual(ptrs, want) {
		t.Fatalf("pointers %v, want %v", ptrs, want)
	}
}

func TestComputePartialCompleteOutdated(t *testing.T) {
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	// F082-AC-08: missing en → partial with the pointer.
	v1 := doc(t, `{"name":{"vi":"Rồng"},"desc":{"vi":"Mô tả","en":"Description"}}`)
	s := i18ntext.Compute(i18ntext.Input{Doc: v1, Required: req, Version: 1, VersionID: "v1", Now: now})
	if s.Status != i18ntext.Partial || !reflect.DeepEqual(s.Missing, []string{"/name/en"}) || len(s.Outdated) != 0 {
		t.Fatalf("partial: %+v", s)
	}
	// filled → complete; the translation of /name is recorded in this version.
	v1b := doc(t, `{"name":{"vi":"Rồng","en":"Dragon"},"desc":{"vi":"Mô tả","en":"Description"}}`)
	s2 := i18ntext.Compute(i18ntext.Input{Doc: v1b, Prev: v1, PrevMeta: s.Meta, Required: req, Version: 1, VersionID: "v1", Now: now})
	if s2.Status != i18ntext.Complete || len(s2.Missing) != 0 {
		t.Fatalf("complete: %+v", s2)
	}
	if s2.Meta["/name"].TranslatedInVersion != 1 {
		t.Fatalf("meta %+v", s2.Meta)
	}
	// the source vi of /name changes later (version 2, target untouched) → outdated, translated in version 1.
	v2 := doc(t, `{"name":{"vi":"Rồng lửa","en":"Dragon"},"desc":{"vi":"Mô tả","en":"Description"}}`)
	s3 := i18ntext.Compute(i18ntext.Input{Doc: v2, Prev: v1b, PrevMeta: s2.Meta, Required: req, Version: 2, VersionID: "v2", Now: now})
	if s3.Status != i18ntext.Outdated || len(s3.Outdated) != 1 || s3.Outdated[0].Pointer != "/name" || s3.Outdated[0].TranslatedInVersion != 1 {
		t.Fatalf("outdated: %+v", s3)
	}
	// translating again clears it.
	v2b := doc(t, `{"name":{"vi":"Rồng lửa","en":"Fire dragon"},"desc":{"vi":"Mô tả","en":"Description"}}`)
	s4 := i18ntext.Compute(i18ntext.Input{Doc: v2b, Prev: v2, PrevMeta: s3.Meta, Required: req, Version: 2, VersionID: "v2", Now: now})
	if s4.Status != i18ntext.Complete || s4.Meta["/name"].TranslatedInVersion != 2 {
		t.Fatalf("retranslated: %+v", s4)
	}
}

func TestComputeNFCAndNoTextIsComplete(t *testing.T) {
	now := time.Now()
	// NFD vs NFC of the same source: not outdated.
	nfc := doc(t, `{"n":{"vi":"Việt","en":"Viet"}}`)
	nfd := doc(t, "{\"n\":{\"vi\":\"Việt\",\"en\":\"Viet\"}}")
	s := i18ntext.Compute(i18ntext.Input{Doc: nfc, Required: req, Version: 1, VersionID: "a", Now: now})
	s2 := i18ntext.Compute(i18ntext.Input{Doc: nfd, Prev: nfc, PrevMeta: s.Meta, Required: req, Version: 1, VersionID: "a", Now: now})
	if s2.Status != i18ntext.Complete {
		t.Fatalf("nfd source flagged: %+v", s2)
	}
	// a type without game text (currencies: codes only) → complete.
	if s := i18ntext.Compute(i18ntext.Input{Doc: doc(t, `{"currency_code":"gem"}`), Required: req, Version: 1, Now: now}); s.Status != i18ntext.Complete {
		t.Fatalf("no text: %+v", s)
	}
}

func TestKeyRoundTrip(t *testing.T) {
	k := i18ntext.Key{ConfigType: "quests", ConfigID: "0193a5d2-0000-7000-8000-000000000001", Version: 3, Pointer: "/steps/0/title"}
	s := k.String()
	if s != "quests/0193a5d2-0000-7000-8000-000000000001/3/steps/0/title" {
		t.Fatalf("key %q", s)
	}
	got, err := i18ntext.ParseKey(s)
	if err != nil || got != k {
		t.Fatalf("parse %+v %v", got, err)
	}
	for _, bad := range []string{"", "quests/x", "quests/id/0/name", "quests/id/abc/name", "quests/id/2", "Quests!/id/2/name"} {
		if _, err := i18ntext.ParseKey(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSetAndGet(t *testing.T) {
	d := doc(t, `{"steps":[{"title":{"vi":"T"}}],"a/b":{"vi":"x"}}`)
	if err := i18ntext.Set(d, "/steps/0/title", "en", "Title"); err != nil {
		t.Fatal(err)
	}
	if v, ok := i18ntext.Get(d, "/steps/0/title"); !ok || v["en"] != "Title" || v["vi"] != "T" {
		t.Fatalf("get %v %v", v, ok)
	}
	if err := i18ntext.Set(d, "/a~1b", "en", "y"); err != nil {
		t.Fatal(err)
	}
	if err := i18ntext.Set(d, "/steps/3/title", "en", "x"); err == nil {
		t.Fatal("unknown pointer accepted")
	}
	if err := i18ntext.Set(d, "/steps", "en", "x"); err == nil {
		t.Fatal("non-text node accepted")
	}
}

func TestDecideImport(t *testing.T) {
	cases := []struct {
		name          string
		keyV, latest  int
		latestStatus  string
		want          i18ntext.Decision
		wantErrorCode string
	}{
		{"draft latest", 3, 3, "draft", i18ntext.WriteDraft, ""},
		{"rejected latest", 3, 3, "rejected", i18ntext.WriteDraft, ""},
		{"published latest", 3, 3, "published", i18ntext.NewDraft, ""},
		{"approved latest", 3, 3, "approved", i18ntext.NewDraft, ""},
		{"archived latest", 3, 3, "archived", i18ntext.NewDraft, ""},
		{"in review", 3, 3, "in_review", i18ntext.Skip, i18ntext.CodeVersionInReview},
		{"stale", 2, 3, "draft", i18ntext.Skip, i18ntext.CodeStaleVersion},
		{"future", 4, 3, "draft", i18ntext.Skip, i18ntext.CodeKeyNotFound},
	}
	for _, c := range cases {
		d, code := i18ntext.DecideImport(c.keyV, c.latest, c.latestStatus)
		if d != c.want || code != c.wantErrorCode {
			t.Errorf("%s: %v %q", c.name, d, code)
		}
	}
}

func TestValidationCopyFillsMissingLanguage(t *testing.T) {
	d := doc(t, `{"name":{"vi":"Rồng"},"x":[{"t":{"en":"Only en"}}],"both":{"vi":"","en":""}}`)
	c := i18ntext.ValidationCopy(d, req)
	if c["name"].(map[string]any)["en"] != "Rồng" {
		t.Fatalf("en not filled: %v", c["name"])
	}
	if c["x"].([]any)[0].(map[string]any)["t"].(map[string]any)["vi"] != "Only en" {
		t.Fatalf("vi not filled: %v", c["x"])
	}
	if d["name"].(map[string]any)["en"] != nil {
		t.Fatal("the original was changed")
	}
	if b := c["both"].(map[string]any); b["vi"] == "" || b["en"] == "" {
		t.Fatalf("empty pair not filled: %v", b)
	}
}
