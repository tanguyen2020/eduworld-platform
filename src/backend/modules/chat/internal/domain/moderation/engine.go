package moderation

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"unicode"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/normalize"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/pii"
)

// Message decisions (message.Decision values).
const (
	DecisionAllow         = "allow"
	DecisionUnchecked     = "unchecked"
	DecisionMask          = "mask"
	DecisionBlock         = "block"
	DecisionHoldForReview = "hold_for_review"
	DecisionBlockAndAlert = "block_and_alert"
)

// Codes of the non-catalogue steps (reported instead of a category code, F116-AC-05).
const (
	CodeSpam     = "spam"
	CodeBullying = "bullying"
)

// GroomingCategory is the catalogue category matched even when the sensitive_terms step is off: grooming detection
// always runs (ADR-0078, F116-AC-30).
const GroomingCategory = "grooming_scam"

// Config is the configuration of the engine (seed/normalization/normalization.v1.json).
type Config struct {
	Normalize       normalize.Config
	PII             pii.Config
	BullyingTargets []string
}

// Engine is the pure part of the pipeline: views, catalogue, PII, content spam, bullying, context → outcome.
type Engine struct {
	n       *normalize.Normalizer
	pii     *pii.Detector
	targets []normalize.Phrase
}

// NewEngine compiles c.
func NewEngine(c Config) *Engine {
	n := normalize.New(c.Normalize)
	e := &Engine{n: n, pii: pii.New(n, c.PII)}
	for _, t := range c.BullyingTargets {
		if p := n.Phrase(t); len(p) > 0 {
			e.targets = append(e.targets, p)
		}
	}
	return e
}

// Normalizer returns the normaliser (catalogue compilation uses the same rules).
func (e *Engine) Normalizer() *normalize.Normalizer { return e.n }

// View normalises a text.
func (e *Engine) View(text string) normalize.View { return e.n.View(text) }

// Fingerprint identifies the content of a text for the spam windows (repeat, multi-channel): a hash of the folded
// canonical tokens (the normalised form, so trivial variations count as the same message). No content is kept.
func Fingerprint(v normalize.View) string {
	var b strings.Builder
	for _, t := range v.Merged {
		b.WriteString(t.Fold)
		b.WriteByte(' ')
	}
	if b.Len() == 0 {
		b.WriteString(strings.TrimSpace(strings.ToLower(v.Text)))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

// SpamCounts are the windows of the spam step for this message (current message included).
type SpamCounts struct {
	Repeats  int // identical messages of the sender within SpamRepeatWindow
	Channels int // distinct channels with the same content within SpamChannelWindow
}

// Input is one evaluation. There is no grade parameter (R15, F116-AC-11).
type Input struct {
	View       normalize.View
	Quick      bool // the text is the parameters of a quick message: a mask becomes a block (nothing partial to show)
	Mentions   int
	Strictness string
	Switches   Effective
	Categories catalog.Categories
	Catalog    []*catalog.Compiled
	Spam       *SpamCounts // nil when the step is off
}

// Outcome is the decision with its metadata (never a matched term or its position for a client).
type Outcome struct {
	Decision     string
	Display      string
	Codes        []string
	StepsRun     []string
	StepsSkipped []string
	Hits         []catalog.Hit
	PII          []pii.Finding
	Grooming     bool
	SlowPatterns int
}

// Evaluate runs, in order, normalize → format → sensitive_terms (grooming always) → pii → spam → bullying → context
// (F116-AC-01). The technical gate and the switch resolution happen before, in the application.
func (e *Engine) Evaluate(in Input) Outcome {
	out := Outcome{StepsRun: []string{StepNormalize, StepFormat}}
	action := ActionAllow
	var spans [][2]int
	code := func(c string) {
		if c != "" && !slices.Contains(out.Codes, c) && len(out.Codes) < 20 {
			out.Codes = append(out.Codes, c)
		}
	}
	display := stripInvisible(in.View.Text)
	content := false // a switchable content step ran

	// Catalogue: every category when the step is on; only grooming when it is off (ADR-0078).
	terms := in.Switches.On(StepSensitiveTerms)
	res := catalog.Match(in.View, in.Categories, PatternBudget, in.Catalog...)
	out.SlowPatterns = res.SlowPatterns
	if terms {
		out.StepsRun = append(out.StepsRun, StepSensitiveTerms)
		content = true
	} else {
		out.StepsSkipped = append(out.StepsSkipped, StepSensitiveTerms)
	}
	out.StepsRun = append(out.StepsRun, StepGrooming)
	for _, h := range res.Hits {
		if !terms && h.CategoryCode != GroomingCategory {
			continue
		}
		out.Hits = append(out.Hits, h)
		a := Bump(h.Action, in.Strictness)
		if h.CategoryCode == GroomingCategory {
			out.Grooming = true
		}
		if a == ActionAllow {
			continue // watch: allowed, risk score only (async analysis, F121)
		}
		if a == ActionMask {
			spans = append(spans, [2]int{h.Start, h.End})
		}
		action = Heavier(action, a)
		code(h.CategoryCode)
	}
	// PII: always (ADR-0078).
	out.StepsRun = append(out.StepsRun, StepPII)
	out.PII = e.pii.Detect(in.View)
	for _, f := range out.PII {
		action = Heavier(action, f.Action)
		code(f.Code)
		if f.Grooming {
			out.Grooming = true
		}
	}
	// Content spam.
	if in.Switches.On(StepSpam) && in.Spam != nil {
		out.StepsRun = append(out.StepsRun, StepSpam)
		content = true
		if in.Spam.Repeats >= SpamRepeatCount || in.Spam.Channels >= SpamChannelCount {
			action = Heavier(action, ActionBlock)
			code(CodeSpam)
		}
	} else {
		out.StepsSkipped = append(out.StepsSkipped, StepSpam)
	}
	// Bullying (rule): a targeted message (mention or second-person word) with a medium-or-worse term.
	if in.Switches.On(StepBullying) {
		out.StepsRun = append(out.StepsRun, StepBullying)
		content = true
		if e.targeted(in) {
			for _, h := range res.Hits {
				if catalog.SeverityRank(h.Severity) >= catalog.SeverityRank(catalog.SeverityMedium) && h.CategoryCode != GroomingCategory {
					action = Heavier(action, ActionHoldForReview)
					code(CodeBullying)
					break
				}
			}
		}
	} else {
		out.StepsSkipped = append(out.StepsSkipped, StepBullying)
	}
	if content {
		out.StepsRun = append(out.StepsRun, StepContext)
	}
	if !in.Switches.On(StepAsyncAnalysis) {
		out.StepsSkipped = append(out.StepsSkipped, StepAsyncAnalysis)
	}
	if action == ActionMask && len(spans) > 0 {
		display = stripInvisible(Mask(in.View.Text, spans))
	}
	if in.Switches.On(StepSpam) && in.Spam != nil && Rank(action) < Rank(ActionBlock) {
		if reduced, changed := Reduce(display); changed {
			display = reduced
			action = Heavier(action, ActionMask)
			code(CodeSpam)
		}
	}
	if in.Quick && action == ActionMask {
		action = ActionBlock
	}
	out.Display = display
	switch {
	case action == ActionAllow && !content:
		out.Decision = DecisionUnchecked
	case action == ActionAllow:
		out.Decision = DecisionAllow
	default:
		out.Decision = action
	}
	if out.Decision == DecisionAllow || out.Decision == DecisionUnchecked {
		out.Codes = nil
	}
	return out
}

func (e *Engine) targeted(in Input) bool {
	if in.Mentions > 0 {
		return true
	}
	for _, toks := range [][]normalize.Token{in.View.Tokens, in.View.Merged} {
		for i := range toks {
			for _, p := range e.targets {
				if p.MatchAt(toks, i) {
					return true
				}
			}
		}
	}
	return false
}

// stripInvisible removes zero-width and control characters from the displayed text (format step).
func stripInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || r == 0x200B || r == 0x200C || r == 0x2060 || r == 0xFEFF {
			return -1
		}
		return r
	}, s)
}
