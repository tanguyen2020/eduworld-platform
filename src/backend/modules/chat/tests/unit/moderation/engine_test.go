// Package moderation holds the unit tests of the F116 moderation engine. Every term is a placeholder (prefix "zq" or
// "__…__", R15): no real sensitive word exists in this repository (static test no_real_terms_test.go).
package moderation

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderationcfg"
)

var (
	catProfanity = uuid.MustParse("01936a00-0000-7000-8000-000000000101")
	catInsult    = uuid.MustParse("01936a00-0000-7000-8000-000000000102")
	catSexual    = uuid.MustParse("01936a00-0000-7000-8000-000000000104")
	catDrugs     = uuid.MustParse("01936a00-0000-7000-8000-000000000105")
	catSelfHarm  = uuid.MustParse("01936a00-0000-7000-8000-000000000106")
	catGrooming  = uuid.MustParse("01936a00-0000-7000-8000-000000000107")
	catWatch     = uuid.MustParse("01936a00-0000-7000-8000-000000000110")
)

type cats map[uuid.UUID]catalog.Category

func (c cats) Lookup(id uuid.UUID) (catalog.Category, bool) {
	v, ok := c[id]
	return v, ok && v.Enabled
}

func categories() cats {
	return cats{
		catProfanity: {ID: catProfanity, Code: "profanity", Severity: "low", DefaultAction: "mask", Enabled: true},
		catInsult:    {ID: catInsult, Code: "insult", Severity: "medium", DefaultAction: "block", Enabled: true},
		catSexual:    {ID: catSexual, Code: "sexual", Severity: "high", DefaultAction: "hold_for_review", Enabled: true},
		catDrugs:     {ID: catDrugs, Code: "drugs", Severity: "medium", DefaultAction: "block", Enabled: true},
		catSelfHarm:  {ID: catSelfHarm, Code: "self_harm", Severity: "critical", DefaultAction: "block_and_alert", Enabled: true},
		catGrooming:  {ID: catGrooming, Code: "grooming_scam", Severity: "critical", DefaultAction: "block_and_alert", Enabled: true},
		catWatch:     {ID: catWatch, Code: "context_watch", Severity: "watch", DefaultAction: "allow", Enabled: true},
	}
}

func term(cat uuid.UUID, t, match string) catalog.Term {
	return catalog.Term{ID: uuid.New(), CategoryID: cat, Term: t, MatchType: match, Locale: "vi"}
}

// snapshot is the placeholder catalogue of the tests (built once: stable term ids).
func snapshot() catalog.Snapshot { return fixed }

var fixed = func() catalog.Snapshot {
	return catalog.Snapshot{Scope: "platform", Version: 7, Terms: []catalog.Term{
		term(catProfanity, "zqlow", catalog.MatchWord),
		term(catInsult, "zqơrốp", catalog.MatchWord),
		term(catInsult, "không zqtee", catalog.MatchPhrase),
		term(catSexual, "zqhigh one", catalog.MatchPhrase),
		term(catSelfHarm, "zqcrit", catalog.MatchWord),
		term(catGrooming, "zqgroom", catalog.MatchWord),
		term(catInsult, "zqcontain", catalog.MatchContains),
		term(catDrugs, `zq[0-9]{3}x`, catalog.MatchPattern),
		term(catWatch, "zqwatch", catalog.MatchWord),
		{ID: uuid.New(), CategoryID: catInsult, Term: "zqbase", MatchType: catalog.MatchWord, Locale: "en", Variants: []string{"zqslang"}},
	}, Allowlist: []catalog.AllowEntry{{Phrase: "zqơrốp town", Locale: "vi"}}}
}()

var engine = moderationcfg.MustEngine()

func compiled(t *testing.T) *catalog.Compiled {
	t.Helper()
	c, invalid := catalog.Compile(engine.Normalizer(), snapshot())
	if len(invalid) > 0 {
		t.Fatalf("invalid terms %v", invalid)
	}
	return c
}

func eval(t *testing.T, text string, mut ...func(*moderation.Input)) moderation.Outcome {
	t.Helper()
	in := moderation.Input{View: engine.View(text), Strictness: "strict", Categories: categories(), Catalog: []*catalog.Compiled{compiled(t)},
		Switches: moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}, Spam: &moderation.SpamCounts{Repeats: 1, Channels: 1}}
	for _, m := range mut {
		m(&in)
	}
	return engine.Evaluate(in)
}

// T009 / F116-AC-01, F116-AC-02, F116-AC-03 (TC-CHAT-003, TC-CHAT-011): variants, allowlist, severity → action,
// strictness bump, mask only the matched span, pipeline order.
func TestEngineVariantsAndActions(t *testing.T) {
	cases := []struct {
		name, text, decision, code string
	}{
		{"clean", "Chào bạn, mình làm câu 3 rồi nè", "allow", ""},
		{"accented", "đồ zqơrốp", "block", "insult"},
		{"unaccented text matches the fold", "do zqorop", "block", "insult"},
		{"another diacritic never matches (ngủ ≠ ngu)", "zqơrộp ngon", "allow", ""},
		{"leetspeak", "zq0r0p", "block", "insult"},
		{"repeated letters", "zqơơơơrốppp", "block", "insult"},
		{"separators", "z.q.ơ.r.ố.p", "block", "insult"},
		{"spaces", "z q o r o p", "block", "insult"},
		{"homoglyph (Cyrillic o)", "zqоrоp", "block", "insult"},
		{"uppercase", "ZQƠRỐP", "block", "insult"},
		{"teencode expanded", "ko zqtee", "block", "insult"},
		{"allowlist excluded first", "mình ở zqơrốp town", "allow", ""},
		{"word boundary", "zqơrốpxyz", "allow", ""},
		{"contains inside a word", "abczqcontaindef", "block", "insult"},
		{"pattern RE2", "mua zq123x đi", "block", "drugs"},
		{"declared variant", "zqslang", "block", "insult"},
		{"high → hold_for_review", "zqhigh one", "hold_for_review", "sexual"},
		{"critical → block_and_alert", "zqcrit", "block_and_alert", "self_harm"},
		{"watch → allow", "zqwatch", "allow", ""},
		{"low → mask", "xin chào zqlow nhé", "mask", "profanity"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := eval(t, c.text)
			if o.Decision != c.decision {
				t.Fatalf("decision %s, want %s (codes %v)", o.Decision, c.decision, o.Codes)
			}
			if c.code != "" && !slices.Contains(o.Codes, c.code) {
				t.Fatalf("codes %v, want %s", o.Codes, c.code)
			}
		})
	}
	// mask hides only the matched span.
	if o := eval(t, "xin chào zqlow nhé"); o.Display != "xin chào *** nhé" {
		t.Fatalf("masked display %q", o.Display)
	}
	// strictness very_strict raises one level: mask → block.
	if o := eval(t, "zqlow", func(in *moderation.Input) { in.Strictness = "very_strict" }); o.Decision != "block" {
		t.Fatalf("very_strict: %s", o.Decision)
	}
	// the heaviest action wins.
	if o := eval(t, "zqlow zqcrit"); o.Decision != "block_and_alert" || !slices.Contains(o.Codes, "self_harm") {
		t.Fatalf("heaviest: %+v", o)
	}
	// pipeline order (F116-AC-01).
	o := eval(t, "hello")
	want := []string{"normalize", "format", "sensitive_terms", "grooming", "pii", "spam", "bullying", "context"}
	if !reflect.DeepEqual(o.StepsRun, want) {
		t.Fatalf("steps %v, want %v", o.StepsRun, want)
	}
}

// F116-AC-07 (rule part): repeats and multi-channel content are blocked, a shouting text is reduced.
func TestEngineSpamRules(t *testing.T) {
	if o := eval(t, "hello", func(in *moderation.Input) { in.Spam = &moderation.SpamCounts{Repeats: 3, Channels: 1} }); o.Decision != "block" ||
		!slices.Contains(o.Codes, "spam") {
		t.Fatalf("repeat: %+v", o)
	}
	if o := eval(t, "hello", func(in *moderation.Input) { in.Spam = &moderation.SpamCounts{Repeats: 1, Channels: 4} }); o.Decision != "block" {
		t.Fatalf("channels: %+v", o)
	}
	if o := eval(t, "MÌNH THẮNG RỒI NHA CẢ NHÀ"); o.Decision != "mask" || o.Display != "mình thắng rồi nha cả nhà" {
		t.Fatalf("caps: %+v", o)
	}
	if o := eval(t, strings.Repeat("😀", 14)); o.Decision != "mask" || o.Display != strings.Repeat("😀", 10) {
		t.Fatalf("emoji: %+v", o)
	}
}

// Bullying rule: a targeted message with a medium-or-worse term is held.
func TestEngineBullying(t *testing.T) {
	o := eval(t, "zqơrốp", func(in *moderation.Input) { in.Mentions = 1 })
	if o.Decision != "hold_for_review" || !slices.Contains(o.Codes, "bullying") {
		t.Fatalf("mention: %+v", o)
	}
	if o := eval(t, "mày zqơrốp"); o.Decision != "hold_for_review" {
		t.Fatalf("second person: %+v", o)
	}
	if o := eval(t, "zqơrốp", func(in *moderation.Input) {
		in.Mentions = 1
		in.Switches.Platform.Steps["bullying"] = false
	}); o.Decision != "block" || !slices.Contains(o.StepsSkipped, "bullying") {
		t.Fatalf("bullying off: %+v", o)
	}
}

// F116-AC-23, F116-AC-30 (engine part): a disabled step is skipped; enabled_all off → unchecked, PII and grooming still
// run.
func TestEngineSwitches(t *testing.T) {
	off := func(steps ...string) func(*moderation.Input) {
		return func(in *moderation.Input) {
			for _, s := range steps {
				in.Switches.Platform.Steps[s] = false
			}
		}
	}
	o := eval(t, "đồ zqơrốp", off("sensitive_terms"))
	if o.Decision != "allow" || !slices.Contains(o.StepsSkipped, "sensitive_terms") {
		t.Fatalf("terms off: %+v", o)
	}
	allOff := func(in *moderation.Input) {
		exp := time.Now().Add(time.Hour)
		in.Switches.Tenant = &moderation.Switch{EnabledAll: false, Steps: map[string]bool{}, ExpiresAt: &exp}
	}
	if o := eval(t, "đồ zqơrốp", allOff); o.Decision != "unchecked" ||
		!reflect.DeepEqual(o.StepsSkipped, []string{"sensitive_terms", "spam", "bullying", "async_analysis"}) {
		t.Fatalf("all off: %+v", o)
	}
	if o := eval(t, "số mình 0912345678", allOff); o.Decision != "block" || !slices.Contains(o.Codes, "pii") {
		t.Fatalf("pii with all off: %+v", o)
	}
	if o := eval(t, "zqgroom", allOff); o.Decision != "block_and_alert" || !o.Grooming {
		t.Fatalf("grooming with all off: %+v", o)
	}
	// a disabled tenant switch past expires_at counts as enabled (never default off).
	past := time.Now().Add(-time.Minute)
	if o := eval(t, "đồ zqơrốp", func(in *moderation.Input) {
		in.Switches.Tenant = &moderation.Switch{EnabledAll: false, Steps: map[string]bool{}, ExpiresAt: &past}
	}); o.Decision != "block" {
		t.Fatalf("expired switch: %+v", o)
	}
	// a legacy steps.pii = false is ignored (ADR-0078).
	e := moderation.Effective{Platform: moderation.Switch{EnabledAll: true, Steps: map[string]bool{"pii": false}}, At: time.Now()}
	if !e.On("pii") || !e.On("grooming") {
		t.Fatal("pii/grooming must always run")
	}
}

// Quick message parameters: a mask becomes a block (nothing partial to show).
func TestEngineQuickParams(t *testing.T) {
	if o := eval(t, "zqlow", func(in *moderation.Input) { in.Quick = true }); o.Decision != "block" {
		t.Fatalf("quick mask: %+v", o)
	}
}

// T015 / F116-AC-11 (TC-CHAT-010): no grade input anywhere; the same text gives the same decision.
func TestGradeParity(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(moderation.Input{}), reflect.TypeOf(port.ValidationInput{})} {
		for i := range typ.NumField() {
			n := strings.ToLower(typ.Field(i).Name)
			if strings.Contains(n, "grade") || strings.Contains(n, "class") || n == "age" || strings.HasSuffix(n, "band") {
				t.Fatalf("%s has a grade-like field %s (R15)", typ.Name(), typ.Field(i).Name)
			}
		}
	}
	a, b := eval(t, "đồ zqơrốp"), eval(t, "đồ zqơrốp")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same text, different decisions: %+v %+v", a, b)
	}
}

// F116-AC-12 (rule part): pattern validation.
func TestValidatePattern(t *testing.T) {
	for _, p := range []string{`zq[0-9]{3}`, `zq(a|b)+x`} {
		if err := catalog.ValidatePattern(p); err != nil {
			t.Fatalf("%s refused: %v", p, err)
		}
	}
	for _, p := range []string{`(a)\1`, `zq(?=x)`, `zq(?!x)`, `(?<=a)b`, `[`, `a*`, strings.Repeat("a", 201)} {
		if catalog.ValidatePattern(p) == nil {
			t.Fatalf("%q accepted", p)
		}
	}
}

func BenchmarkEvaluate(b *testing.B) {
	c, _ := catalog.Compile(engine.Normalizer(), snapshot())
	text := strings.Repeat("mình làm câu ba rồi nha ", 9)[:200]
	in := moderation.Input{View: engine.View(text), Categories: categories(), Catalog: []*catalog.Compiled{c},
		Switches: moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}, Spam: &moderation.SpamCounts{}}
	b.ReportAllocs()
	for b.Loop() {
		in.View = engine.View(text)
		_ = engine.Evaluate(in)
	}
}
