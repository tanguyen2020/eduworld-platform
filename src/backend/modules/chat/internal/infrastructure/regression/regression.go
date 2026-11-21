// Package regression runs the regression suite of the sensitive-term catalogue before a publish (F116 T022,
// F116-AC-14, MODERATION §11.4, filter_quality_gate): a labelled corpus is evaluated with the real engine against the
// CANDIDATE catalogue; recall is measured on the high/critical/PII samples, the false-positive rate on the clean ones.
//
// The corpus comes from a Source. The default, Embedded, is the placeholder corpus of the repository
// (seed/regression/corpus.v1.json): its positive catalogue samples are generated from the high/critical terms of the
// candidate itself (templates and written forms), plus invented PII/meeting samples and clean study sentences — no
// real sensitive word (R15). The operational corpus (≥ 2,000 labelled sentences, SC-001) is operational data kept
// outside the repository (object storage, chat.sensitive_terms.read; spec Assumptions); an adapter loading it
// implements Source and is passed to Suite instead of Embedded. No sample text is ever logged or returned (R14).
package regression

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	seed "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/seed/regression"
)

// Labels of a sample.
const (
	LabelPositive = "positive" // must be detected (high/critical term, PII, meeting)
	LabelClean    = "clean"    // must be delivered as is
)

// Sample is one labelled sentence.
type Sample struct {
	Text  string
	Label string
}

// Source provides the labelled corpus for a candidate catalogue (terms in force, used to generate catalogue samples).
type Source interface {
	Samples(ctx context.Context, terms []catalog.Term, cats catalog.Categories) ([]Sample, error)
}

type corpusFile struct {
	Version   int      `json:"version"`
	Templates []string `json:"term_templates"`
	Positives []struct {
		Text  string `json:"text"`
		Label string `json:"label"`
	} `json:"positives"`
	Clean []string `json:"clean"`
}

// Embedded is the placeholder corpus of the repository.
type Embedded struct {
	file corpusFile
}

// LoadEmbedded decodes seed/regression/corpus.v1.json.
func LoadEmbedded() (*Embedded, error) {
	var f corpusFile
	if err := json.Unmarshal(seed.V1, &f); err != nil {
		return nil, fmt.Errorf("chat: regression corpus: %w", err)
	}
	if f.Version < 1 || len(f.Templates) == 0 || len(f.Positives) == 0 || len(f.Clean) == 0 {
		return nil, errors.New("chat: regression corpus incomplete")
	}
	return &Embedded{file: f}, nil
}

// forms are the written forms of a term used in samples: as entered, upper case, its declared variants and, for a
// single short word, the letters separated by dots (n.g.u, MODERATION §3.1).
func forms(t catalog.Term) []string {
	out := []string{t.Term, strings.ToUpper(t.Term)}
	out = append(out, t.Variants...)
	if t.MatchType == catalog.MatchWord && !strings.ContainsAny(t.Term, " .") && utf8.RuneCountInString(t.Term) <= 12 {
		rs := []rune(t.Term)
		parts := make([]string, len(rs))
		for i, r := range rs {
			parts[i] = string(r)
		}
		out = append(out, strings.Join(parts, "."))
	}
	return out
}

// Samples implements Source: the fixed samples plus, for every high/critical term in force (not a pattern: a regular
// expression has no canonical sentence), each form in each template.
func (e *Embedded) Samples(_ context.Context, terms []catalog.Term, cats catalog.Categories) ([]Sample, error) {
	out := make([]Sample, 0, len(e.file.Positives)+len(e.file.Clean)+len(terms)*8)
	for _, p := range e.file.Positives {
		out = append(out, Sample{Text: p.Text, Label: LabelPositive})
	}
	for _, c := range e.file.Clean {
		out = append(out, Sample{Text: c, Label: LabelClean})
	}
	for _, t := range terms {
		if t.MatchType == catalog.MatchPattern {
			continue
		}
		cat, on := cats.Lookup(t.CategoryID)
		if !on {
			continue
		}
		sev := cat.Severity
		if t.SeverityOverride != "" {
			sev = t.SeverityOverride
		}
		if catalog.SeverityRank(sev) < catalog.SeverityRank(catalog.SeverityHigh) {
			continue
		}
		for _, f := range forms(t) {
			for _, tpl := range e.file.Templates {
				out = append(out, Sample{Text: strings.ReplaceAll(tpl, "{t}", f), Label: LabelPositive})
			}
		}
	}
	return out, nil
}

// Suite implements port.RegressionSuite with the real engine.
type Suite struct {
	Engine *moderation.Engine
	Source Source
}

var _ port.RegressionSuite = Suite{}

// Run implements port.RegressionSuite: the candidate is compiled in memory and every sample evaluated with every
// switchable step on (the gate measures the catalogue, not a switch state) and the standard strictness.
func (s Suite) Run(ctx context.Context, in port.RegressionInput) (port.RegressionResult, error) {
	if s.Engine == nil || s.Source == nil {
		return port.RegressionResult{}, errors.New("chat: regression suite not configured")
	}
	n := s.Engine.Normalizer()
	platform, _ := catalog.Compile(n, in.Platform)
	parts := []*catalog.Compiled{platform}
	terms := append([]catalog.Term(nil), in.Platform.Terms...)
	if in.Tenant != nil {
		tc, _ := catalog.Compile(n, *in.Tenant)
		parts = append(parts, tc)
		terms = append(terms, in.Tenant.Terms...)
	}
	cats := catalog.NewLookup(in.Platform.Categories, in.Settings)
	samples, err := s.Source.Samples(ctx, terms, cats)
	if err != nil {
		return port.RegressionResult{}, err
	}
	var res port.RegressionResult
	detected, flagged := 0, 0
	sw := moderation.Effective{Platform: moderation.AllOn()}
	for _, sm := range samples {
		if err := ctx.Err(); err != nil {
			return port.RegressionResult{}, err
		}
		o := s.Engine.Evaluate(moderation.Input{View: s.Engine.View(sm.Text), Strictness: moderation.StrictnessStandard, Switches: sw,
			Categories: cats, Catalog: parts})
		switch sm.Label {
		case LabelPositive:
			res.Positives++
			if moderation.Detected(o.Decision) {
				detected++
			}
		case LabelClean:
			res.Clean++
			if moderation.FalsePositive(o.Decision) {
				flagged++
			}
		}
	}
	res.Recall, res.FPRate = 1, 0
	if res.Positives > 0 {
		res.Recall = float64(detected) / float64(res.Positives)
	}
	if res.Clean > 0 {
		res.FPRate = float64(flagged) / float64(res.Clean)
	}
	return res, nil
}
