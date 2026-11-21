package moderation

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderationcfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/regression"
)

func regressionPlatform(terms []catalog.Term, allow []catalog.AllowEntry) catalog.Snapshot {
	cs := make([]catalog.Category, 0, len(categories()))
	for _, c := range categories() {
		cs = append(cs, c)
	}
	return catalog.Snapshot{Scope: "platform", Version: 2, Categories: cs, Terms: terms, Allowlist: allow}
}

func runSuite(t *testing.T, in port.RegressionInput) port.RegressionResult {
	t.Helper()
	src, err := regression.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	res, err := regression.Suite{Engine: moderationcfg.MustEngine(), Source: src}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// T022 / F116-AC-14: the placeholder corpus passes filter_quality_gate on a sound catalogue (PII samples all detected,
// no clean sentence flagged, every high/critical placeholder detected in every template and written form); an allowlist
// entry that hides a critical term, or a pattern that matches ordinary words, fails the gate.
func TestRegressionSuite(t *testing.T) {
	critical := term(catSelfHarm, "zqcrit", catalog.MatchWord)
	high := term(catSexual, "zqhigh", catalog.MatchWord)
	terms := []catalog.Term{critical, high, term(catInsult, "zqơrốp", catalog.MatchWord), term(catProfanity, "zqlow", catalog.MatchWord)}

	empty := runSuite(t, port.RegressionInput{Platform: regressionPlatform(nil, nil)})
	if empty.Recall != 1 || empty.FPRate != 0 || empty.Positives < 15 || empty.Clean < 40 {
		t.Fatalf("empty catalogue: %+v", empty)
	}
	sound := runSuite(t, port.RegressionInput{Platform: regressionPlatform(terms, nil)})
	if sound.Recall != 1 || sound.FPRate != 0 || sound.Positives <= empty.Positives {
		t.Fatalf("sound catalogue: %+v", sound)
	}
	if !moderation.PassesQualityGate(sound.Recall, sound.FPRate) {
		t.Fatal("sound catalogue must pass the gate")
	}

	// An allowlist entry equal to the critical term hides it: recall falls under 0.95.
	hidden := runSuite(t, port.RegressionInput{Platform: regressionPlatform(terms, []catalog.AllowEntry{{Phrase: "zqcrit", Locale: "vi"},
		{Phrase: "zqhigh", Locale: "vi"}})})
	if moderation.PassesQualityGate(hidden.Recall, hidden.FPRate) || hidden.Recall >= moderation.RegressionMinRecall {
		t.Fatalf("hidden critical term: %+v", hidden)
	}
	// A pattern matching every word flags the clean sentences: false positives over 3%.
	broad := term(catInsult, "zq|[a-z]{3}", catalog.MatchPattern)
	noisy := runSuite(t, port.RegressionInput{Platform: regressionPlatform(append(terms, broad), nil)})
	if moderation.PassesQualityGate(noisy.Recall, noisy.FPRate) || noisy.FPRate <= moderation.RegressionMaxFPRate {
		t.Fatalf("broad pattern: %+v", noisy)
	}

	// Tenant publish: the tenant candidate joins the platform in force; a high tenant term generates samples too.
	tenant := catalog.Snapshot{Scope: "tenant", Version: 1, Terms: []catalog.Term{term(catSexual, "zqtenanthigh", catalog.MatchWord)},
		Allowlist: []catalog.AllowEntry{}}
	tr := runSuite(t, port.RegressionInput{Platform: regressionPlatform(terms, nil), Tenant: &tenant, Settings: map[uuid.UUID]bool{}})
	if tr.Recall != 1 || tr.Positives <= sound.Positives {
		t.Fatalf("tenant candidate: %+v", tr)
	}
}
