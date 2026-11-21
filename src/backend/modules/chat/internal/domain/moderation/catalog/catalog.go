// Package catalog compiles a published sensitive-term catalogue (MODERATION §11) and matches it on the normalised views
// of a text (F116-AC-02, AC-03): word/phrase terms on the token sequences (word boundaries), contains terms on the
// joined tokens, RE2 patterns on the lowercase text with a time budget. Allowlisted phrases are excluded first. It is
// pure: the snapshot comes from chat.sensitive_catalog_versions; the result carries category, severity, action and the
// span, which never leave the server (ADR-0030, R15).
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/normalize"
)

// Severities of a category (MODERATION §11.2).
const (
	SeverityWatch    = "watch"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Match types of a term.
const (
	MatchWord     = "word"
	MatchPhrase   = "phrase"
	MatchContains = "contains"
	MatchPattern  = "pattern"
)

// MaxPatternLen is the longest RE2 pattern (MODERATION §11.1).
const MaxPatternLen = 200

// SlowRuns is the number of consecutive over-budget runs after which a pattern is skipped (metric, log without content).
const SlowRuns = 3

// Category is one category of a snapshot.
type Category struct {
	ID                uuid.UUID `json:"category_id"`
	Code              string    `json:"code"`
	Severity          string    `json:"severity"`
	DefaultAction     string    `json:"default_action"`
	Enabled           bool      `json:"enabled"`
	AllowTenantToggle bool      `json:"allow_tenant_toggle"`
}

// Term is one enabled term of a snapshot.
type Term struct {
	ID               uuid.UUID `json:"term_id"`
	CategoryID       uuid.UUID `json:"category_id"`
	Term             string    `json:"term"`
	MatchType        string    `json:"match_type"`
	Locale           string    `json:"locale"`
	Variants         []string  `json:"variants,omitempty"`
	SeverityOverride string    `json:"severity_override,omitempty"`
	ActionOverride   string    `json:"action_override,omitempty"`
}

// AllowEntry is one allowlisted phrase.
type AllowEntry struct {
	Phrase string `json:"phrase"`
	Locale string `json:"locale"`
}

// Snapshot is the immutable catalogue of one published version (platform: categories, terms, allowlist; tenant: its
// own terms and allowlist).
type Snapshot struct {
	Scope      string       `json:"scope"`
	Version    int64        `json:"version"`
	Categories []Category   `json:"categories,omitempty"`
	Terms      []Term       `json:"terms"`
	Allowlist  []AllowEntry `json:"allowlist"`
}

// ParseSnapshot decodes a stored snapshot.
func ParseSnapshot(raw []byte) (Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return Snapshot{}, fmt.Errorf("catalog: snapshot: %w", err)
	}
	return s, nil
}

// ErrInvalidPattern: a pattern term is not an allowed RE2 expression (SENSITIVE_TERM_INVALID).
var ErrInvalidPattern = errors.New("catalog: invalid pattern")

// ValidatePattern checks a pattern term: RE2 syntax, ≤ 200 characters, no backreference or lookaround (Go's RE2
// refuses both at parse time) and no empty match.
func ValidatePattern(p string) error {
	if p == "" || utf8.RuneCountInString(p) > MaxPatternLen {
		return ErrInvalidPattern
	}
	if strings.Contains(p, `\1`) || strings.Contains(p, "(?=") || strings.Contains(p, "(?!") || strings.Contains(p, "(?<") {
		return ErrInvalidPattern
	}
	re, err := syntax.Parse(p, syntax.Perl)
	if err != nil || re.MaxCap() > 20 {
		return ErrInvalidPattern
	}
	c, err := regexp.Compile(p)
	if err != nil || c.MatchString("") {
		return ErrInvalidPattern
	}
	return nil
}

// Hit is one match: category, severity, action of the term (override) or category, and the byte span in the text.
type Hit struct {
	CategoryID   uuid.UUID
	CategoryCode string
	Severity     string
	Action       string
	TermID       uuid.UUID
	Start, End   int
}

type phraseTerm struct {
	term   *Term
	phrase normalize.Phrase
}

type containsTerm struct {
	term  *Term
	canon string // joined canonical tokens
	fold  string // joined folded tokens
}

type patternTerm struct {
	term *Term
	re   *regexp.Regexp
	slow atomic.Bool
	over atomic.Int32
}

// Compiled is a compiled catalogue part (platform or one tenant): read-only, shared by every message.
type Compiled struct {
	Scope    string
	Version  int64
	Checksum string
	byFold   map[string][]phraseTerm // folded first token → candidate word/phrase terms
	contains []containsTerm
	patterns []*patternTerm
	allow    []normalize.Phrase
	terms    int
}

// Terms is the number of compiled terms (variants included).
func (c *Compiled) Terms() int {
	if c == nil {
		return 0
	}
	return c.terms
}

// Compile compiles the terms and allowlist of s with n (variants are compiled as terms of the same row; an invalid
// pattern is skipped and reported).
func Compile(n *normalize.Normalizer, s Snapshot) (*Compiled, []uuid.UUID) {
	c := &Compiled{Scope: s.Scope, Version: s.Version, byFold: map[string][]phraseTerm{}}
	var invalid []uuid.UUID
	for i := range s.Terms {
		t := &s.Terms[i]
		forms := append([]string{t.Term}, t.Variants...)
		for _, form := range forms {
			switch t.MatchType {
			case MatchPattern:
				if form != t.Term {
					continue
				}
				if ValidatePattern(form) != nil {
					invalid = append(invalid, t.ID)
					continue
				}
				c.patterns = append(c.patterns, &patternTerm{term: t, re: regexp.MustCompile("(?i)" + form)})
				c.terms++
			case MatchContains:
				p := n.Phrase(form)
				if len(p) == 0 {
					continue
				}
				var canon, fold strings.Builder
				for _, tk := range p {
					canon.WriteString(tk.Canon)
					fold.WriteString(tk.Fold)
				}
				c.contains = append(c.contains, containsTerm{term: t, canon: canon.String(), fold: fold.String()})
				c.terms++
			default: // word, phrase
				p := n.Phrase(form)
				if len(p) == 0 {
					continue
				}
				c.byFold[p[0].Fold] = append(c.byFold[p[0].Fold], phraseTerm{term: t, phrase: p})
				c.terms++
			}
		}
	}
	for _, a := range s.Allowlist {
		if p := n.Phrase(a.Phrase); len(p) > 0 {
			c.allow = append(c.allow, p)
		}
	}
	return c, invalid
}

// Categories resolves a category id to its definition and whether it is in force for the tenant.
type Categories interface {
	Lookup(id uuid.UUID) (Category, bool)
}

// Result is the outcome of one Match: the hits and the patterns that exceeded the time budget (skipped safely, metric).
type Result struct {
	Hits         []Hit
	SlowPatterns int
}

type span struct{ start, end int }

// allowed returns the byte spans of the text covered by an allowlisted phrase.
func allowed(v normalize.View, parts []*Compiled) []span {
	var out []span
	for _, c := range parts {
		if c == nil {
			continue
		}
		for _, toks := range [][]normalize.Token{v.Tokens, v.Merged} {
			for _, p := range c.allow {
				for i := range toks {
					if p.MatchAt(toks, i) {
						out = append(out, span{toks[i].Start, toks[i+len(p)-1].End})
					}
				}
			}
		}
	}
	return out
}

func inside(s span, spans []span) bool {
	for _, a := range spans {
		if s.start >= a.start && s.end <= a.end {
			return true
		}
	}
	return false
}

// Match runs every compiled part on v; patternBudget bounds one pattern run (sensitive_terms_engine_limits: a slower
// pattern is skipped from then on, never blocking the whole message).
func Match(v normalize.View, cats Categories, patternBudget time.Duration, parts ...*Compiled) Result {
	var res Result
	allow := allowed(v, parts)
	seen := map[[3]any]bool{}
	add := func(t *Term, start, end int) {
		cat, ok := cats.Lookup(t.CategoryID)
		if !ok {
			return
		}
		s := span{start, end}
		if inside(s, allow) {
			return
		}
		k := [3]any{t.ID, start, end}
		if seen[k] {
			return
		}
		seen[k] = true
		sev := cat.Severity
		if t.SeverityOverride != "" {
			sev = t.SeverityOverride
		}
		action := t.ActionOverride
		if action == "" {
			if t.SeverityOverride != "" {
				action = ActionOf(sev)
			} else {
				action = cat.DefaultAction
			}
		}
		res.Hits = append(res.Hits, Hit{CategoryID: cat.ID, CategoryCode: cat.Code, Severity: sev, Action: action, TermID: t.ID,
			Start: start, End: end})
	}
	textDiacritic := normalize.HasDiacritics(v.Lower)
	for _, c := range parts {
		if c == nil {
			continue
		}
		for _, toks := range [][]normalize.Token{v.Tokens, v.Merged} {
			for i, tk := range toks {
				for _, pt := range c.byFold[tk.Fold] {
					if pt.phrase.MatchAt(toks, i) {
						add(pt.term, toks[i].Start, toks[i+len(pt.phrase)-1].End)
					}
				}
			}
			if len(c.contains) > 0 {
				canon, fold, owner := joined(toks)
				for _, ct := range c.contains {
					for _, m := range indexAll(canon, ct.canon) {
						add(ct.term, toks[owner.canon[m]].Start, toks[owner.canon[m+len(ct.canon)-1]].End)
					}
					if !textDiacritic {
						for _, m := range indexAll(fold, ct.fold) {
							add(ct.term, toks[owner.fold[m]].Start, toks[owner.fold[m+len(ct.fold)-1]].End)
						}
					}
				}
			}
		}
		for _, p := range c.patterns {
			if p.slow.Load() {
				res.SlowPatterns++
				continue
			}
			start := time.Now()
			locs := p.re.FindAllStringIndex(v.Lower, 8)
			if patternBudget > 0 && time.Since(start) > patternBudget {
				// The matches found are kept (never a wrong block); a pattern over the budget on SlowRuns consecutive
				// messages is skipped from then on (a single GC pause never disables a pattern).
				res.SlowPatterns++
				if p.over.Add(1) >= SlowRuns {
					p.slow.Store(true)
				}
			} else {
				p.over.Store(0)
			}
			for _, l := range locs {
				if l[1] > l[0] {
					add(p.term, v.LowerPos[l[0]], v.LowerPos[l[1]])
				}
			}
		}
	}
	sort.SliceStable(res.Hits, func(i, j int) bool { return res.Hits[i].Start < res.Hits[j].Start })
	return res
}

type owners struct{ canon, fold []int }

// joined concatenates the tokens (no separator) and records, per byte, the token it comes from.
func joined(toks []normalize.Token) (string, string, owners) {
	var c, f strings.Builder
	var o owners
	for i, t := range toks {
		c.WriteString(t.Canon)
		for range len(t.Canon) {
			o.canon = append(o.canon, i)
		}
		f.WriteString(t.Fold)
		for range len(t.Fold) {
			o.fold = append(o.fold, i)
		}
	}
	return c.String(), f.String(), o
}

func indexAll(s, sub string) []int {
	var out []int
	if sub == "" {
		return nil
	}
	for i := 0; i+len(sub) <= len(s); {
		k := strings.Index(s[i:], sub)
		if k < 0 {
			break
		}
		out = append(out, i+k)
		i += k + 1
	}
	return out
}

// ActionOf maps a severity to its default action (ADR-0030: watch→allow, low→mask, medium→block, high→hold_for_review,
// critical→block_and_alert).
func ActionOf(severity string) string {
	switch severity {
	case SeverityWatch:
		return "allow"
	case SeverityLow:
		return "mask"
	case SeverityMedium:
		return "block"
	case SeverityHigh:
		return "hold_for_review"
	case SeverityCritical:
		return "block_and_alert"
	}
	return "block" // unknown severity fails closed
}

// SeverityRank orders severities (watch < low < medium < high < critical).
func SeverityRank(s string) int {
	switch s {
	case SeverityWatch:
		return 0
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	case SeverityCritical:
		return 4
	}
	return 2
}
