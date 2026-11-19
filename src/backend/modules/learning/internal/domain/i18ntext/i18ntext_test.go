package i18ntext

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

const stemVI = `{"stem":{"blocks":[{"type":"paragraph","text":"7 × 8 = ?"}]},"title":{"vi":"Phép nhân"}}`

// F025-AC-18: a text without its content_language version is reported by JSON pointer.
func TestMissingSource(t *testing.T) {
	if got := MissingSource(json.RawMessage(stemVI), VI); len(got) != 0 {
		t.Fatalf("complete source reported %v", got)
	}
	doc := `{"stem":{"blocks":[{"type":"paragraph","text":{"en":"7 x 8 = ?"}}]},"title":{"vi":"Phép nhân","en":"Multiplication"}}`
	if got := MissingSource(json.RawMessage(doc), VI); !slices.Equal(got, []string{"/stem/blocks/0/text"}) {
		t.Fatalf("missing %v", got)
	}
	// An English subject needs the en texts.
	if got := MissingSource(json.RawMessage(`{"title":{"vi":"Bài 1"}}`), EN); !slices.Equal(got, []string{"/title"}) {
		t.Fatalf("en subject %v", got)
	}
	// Non-text strings (ids, types) are not texts.
	if got := Texts(json.RawMessage(`{"options":[{"id":"o_1","type":"x"}]}`), VI); len(got) != 0 {
		t.Fatalf("ids as texts %v", got)
	}
}

// F025-AC-19: complete / partial / outdated.
func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	if s := ComputeStatus(json.RawMessage(stemVI), VI, nil); s != Partial {
		t.Fatalf("vi only %s", s)
	}
	v1 := json.RawMessage(`{"title":{"vi":"Phép nhân","en":"Multiplication"}}`)
	meta := Track(nil, v1, VI, nil, now)
	if s := ComputeStatus(v1, VI, meta); s != Complete || meta["/title"].SourceSHA256 == "" {
		t.Fatalf("both %s %v", s, meta)
	}
	// The source changes, the translation does not: outdated.
	v2 := json.RawMessage(`{"title":{"vi":"Phép nhân hai số","en":"Multiplication"}}`)
	meta2 := Track(v1, v2, VI, meta, now.Add(time.Hour))
	if s := ComputeStatus(v2, VI, meta2); s != Outdated || !slices.Equal(OutdatedFields(v2, VI, meta2), []string{"/title"}) {
		t.Fatalf("source changed %s", s)
	}
	// The translation is updated: complete again.
	v3 := json.RawMessage(`{"title":{"vi":"Phép nhân hai số","en":"Multiplying two numbers"}}`)
	meta3 := Track(v2, v3, VI, meta2, now.Add(2*time.Hour))
	if s := ComputeStatus(v3, VI, meta3); s != Complete {
		t.Fatalf("translation updated %s", s)
	}
	if s := ComputeStatus(json.RawMessage(`{}`), VI, nil); s != Complete {
		t.Fatalf("no text %s", s)
	}
	if m := ParseMeta(meta3.JSON()); m["/title"] != meta3["/title"] {
		t.Fatalf("meta round trip %v", m)
	}
}

// F025-AC-20: the requested language, else the content_language with the language actually served.
func TestResolve(t *testing.T) {
	doc := json.RawMessage(`{"title":{"vi":"Phép nhân","en":"Multiplication"},"stem":{"blocks":[{"type":"paragraph","text":{"vi":"Tính"}}]}}`)
	out, lang := Resolve(doc, EN, VI)
	if lang != VI || string(out) != `{"stem":{"blocks":[{"text":"Tính","type":"paragraph"}]},"title":"Multiplication"}` {
		t.Fatalf("fallback %s %s", lang, out)
	}
	full := json.RawMessage(`{"title":{"vi":"Phép nhân","en":"Multiplication"}}`)
	if out, lang := Resolve(full, EN, VI); lang != EN || string(out) != `{"title":"Multiplication"}` {
		t.Fatalf("en %s %s", lang, out)
	}
	if _, lang := Resolve(json.RawMessage(`{"blocks":[{"text":"Chào"}]}`), EN, VI); lang != VI {
		t.Fatalf("plain vi served as %s", lang)
	}
	if _, lang := Resolve(full, VI, VI); lang != VI {
		t.Fatalf("vi %s", lang)
	}
	for in, want := range map[string]string{"en-US,en;q=0.9": EN, "vi-VN": VI, "": VI, "fr-FR, en;q=0.5": EN} {
		if got := Locale(in); got != want {
			t.Fatalf("Locale(%q) = %s", in, got)
		}
	}
}

// F025-AC-23: an imported translation is written at its pointer.
func TestSetTranslation(t *testing.T) {
	doc := json.RawMessage(`{"stem":{"blocks":[{"type":"paragraph","text":"Tính"}]},"title":{"vi":"Bài 1"}}`)
	out, err := SetTranslation(doc, "/stem/blocks/0/text", VI, EN, "Compute")
	if err != nil {
		t.Fatal(err)
	}
	out, err = SetTranslation(out, "/title", VI, EN, "Lesson 1")
	if err != nil || string(out) != `{"stem":{"blocks":[{"text":{"en":"Compute","vi":"Tính"},"type":"paragraph"}]},"title":{"en":"Lesson 1","vi":"Bài 1"}}` {
		t.Fatalf("%v %s", err, out)
	}
	for _, p := range []string{"/stem/blocks/0/type", "/stem/blocks/9/text", "stem", "/nope/x"} {
		if _, err := SetTranslation(doc, p, VI, EN, "x"); err == nil {
			t.Fatalf("%s accepted", p)
		}
	}
}
