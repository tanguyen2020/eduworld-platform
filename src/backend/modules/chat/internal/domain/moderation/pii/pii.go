// Package pii is the PII detector of the moderation pipeline (MODERATION §3.3, F116-AC-04): phone numbers (digits or
// spelled out, split by separators), identity/bank numbers, e-mails (also "a còng … chấm com"), URLs and domains
// outside the allowlist, social networks/apps and @handles → block; addresses → hold_for_review; meeting in person →
// hold_for_review with the grooming risk (ADR-0078: this step and grooming detection always run). Pure: the phrase
// lists come from the versioned configuration (seed/normalization), no sensitive word.
package pii

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/normalize"
)

// Config is the PII part of seed/normalization/normalization.v1.json.
type Config struct {
	NumberWords     map[string]string `json:"number_words"`
	PhoneMinDigits  int               `json:"phone_min_digits"`
	SocialApps      []string          `json:"social_apps"`
	AtWords         []string          `json:"at_words"`
	DotWords        []string          `json:"dot_words"`
	LinkTLDs        []string          `json:"link_tlds"`
	LinkAllowlist   []string          `json:"link_allowlist"`
	AddressKeywords []string          `json:"address_keywords"`
	MeetupPhrases   []string          `json:"meetup_phrases"`
}

// Kinds of findings.
const (
	KindPhone   = "phone"
	KindEmail   = "email"
	KindURL     = "url"
	KindSocial  = "social"
	KindHandle  = "handle"
	KindAddress = "address"
	KindMeetup  = "meetup"
)

// Codes reported to the client and in the decision log (never the content).
const (
	CodePII      = "pii"
	CodeGrooming = "grooming_scam"
)

// Finding is one detected item; Start/End are byte offsets of the NFC text.
type Finding struct {
	Kind       string
	Code       string
	Action     string // block or hold_for_review
	Start, End int
	// Grooming marks a grooming risk signal (meeting in person).
	Grooming bool
}

// Detector is compiled once from Config.
type Detector struct {
	numbers   map[string]string
	minDigits int
	social    []normalize.Phrase
	at        []normalize.Phrase
	dot       []normalize.Phrase
	address   []normalize.Phrase
	meetup    []normalize.Phrase
	allow     []string
	email     *regexp.Regexp
	url       *regexp.Regexp
	domain    *regexp.Regexp
	handle    *regexp.Regexp
	symDomain *regexp.Regexp
	symEmail  *regexp.Regexp
}

func phrases(n *normalize.Normalizer, ss []string) []normalize.Phrase {
	out := make([]normalize.Phrase, 0, len(ss))
	for _, s := range ss {
		if p := n.Phrase(s); len(p) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// New compiles c with the normaliser of the pipeline.
func New(n *normalize.Normalizer, c Config) *Detector {
	d := &Detector{numbers: map[string]string{}, minDigits: c.PhoneMinDigits, social: phrases(n, c.SocialApps),
		at: phrases(n, c.AtWords), dot: phrases(n, c.DotWords), address: phrases(n, c.AddressKeywords),
		meetup: phrases(n, c.MeetupPhrases)}
	if d.minDigits <= 0 {
		d.minDigits = 9
	}
	for k, v := range c.NumberWords {
		for _, t := range n.Tokens(k) {
			d.numbers[t.Canon] = v
		}
	}
	for _, a := range c.LinkAllowlist {
		d.allow = append(d.allow, strings.ToLower(a))
	}
	tlds := make([]string, 0, len(c.LinkTLDs))
	for _, t := range c.LinkTLDs {
		tlds = append(tlds, regexp.QuoteMeta(strings.ToLower(t)))
	}
	tld := "(?:" + strings.Join(tlds, "|") + ")"
	if len(tlds) == 0 {
		tld = "(?:[a-z]{2,6})"
	}
	d.email = regexp.MustCompile(`[\p{L}\p{N}._%+\-]+\s*@\s*[\p{L}\p{N}\-]+(?:\s*\.\s*[\p{L}\p{N}\-]+)+`)
	d.url = regexp.MustCompile(`(?:https?\s*:\s*/\s*/|www\s*\.)\S*`)
	// No space around the dot (a sentence end followed by a word is not a link), except before the common TLDs.
	strongTLD := "(?:com|vn|net|org)"
	d.domain = regexp.MustCompile(`[\p{L}\p{N}\-]{2,}(?:\.[\p{L}\p{N}\-]+)*\.` + tld + `(?:[^\p{L}\p{N}]|$)|` +
		`[\p{L}\p{N}\-]{2,}\s*\.\s*` + strongTLD + `(?:[^\p{L}\p{N}]|$)`)
	d.handle = regexp.MustCompile(`(?:^|[\s(,;:])@[\p{L}\p{N}_.]{3,}`)
	d.symDomain = regexp.MustCompile(`[a-z0-9\-]{2,}(?:\.[a-z0-9\-]+)*\.` + strongTLD + `(?:[^a-z0-9]|$)`)
	d.symEmail = regexp.MustCompile(`[a-z0-9._%+\-]+@[a-z0-9\-]+(?:\.[a-z0-9\-]+)+`)
	return d
}

func (d *Detector) allowed(s string) bool {
	s = strings.ToLower(strings.Join(strings.Fields(s), ""))
	for _, a := range d.allow {
		if strings.Contains(s, a) {
			return true
		}
	}
	return false
}

// Detect returns every PII finding of v (empty: none).
func (d *Detector) Detect(v normalize.View) []Finding {
	var out []Finding
	add := func(kind string, action string, start, end int) {
		code := CodePII
		if kind == KindMeetup {
			code = CodeGrooming
		}
		out = append(out, Finding{Kind: kind, Code: code, Action: action, Start: start, End: end, Grooming: kind == KindMeetup})
	}
	lowerSpan := func(loc []int) (int, int) { return v.LowerPos[loc[0]], v.LowerPos[loc[1]] }
	// E-mail and URLs on the lowercase text.
	for _, loc := range d.email.FindAllStringIndex(v.Lower, -1) {
		s, e := lowerSpan(loc)
		add(KindEmail, "block", s, e)
	}
	for _, loc := range d.url.FindAllStringIndex(v.Lower, -1) {
		if !d.allowed(v.Lower[loc[0]:loc[1]]) {
			s, e := lowerSpan(loc)
			add(KindURL, "block", s, e)
		}
	}
	for _, loc := range d.domain.FindAllStringIndex(v.Lower, -1) {
		if !d.allowed(v.Lower[loc[0]:loc[1]]) && !numeric(v.Lower[loc[0]:loc[1]]) {
			s, e := lowerSpan(loc)
			add(KindURL, "block", s, e)
		}
	}
	for _, loc := range d.handle.FindAllStringIndex(v.Lower, -1) {
		s, e := lowerSpan(loc)
		add(KindHandle, "block", s, e)
	}
	for _, toks := range [][]normalize.Token{v.Tokens, v.Merged} {
		d.phones(toks, add)
		d.spoken(toks, add)
		for i := range toks {
			for _, p := range d.social {
				if p.MatchAt(toks, i) {
					add(KindSocial, "block", toks[i].Start, toks[i+len(p)-1].End)
				}
			}
			for _, p := range d.meetup {
				if p.MatchAt(toks, i) {
					add(KindMeetup, "hold_for_review", toks[i].Start, toks[i+len(p)-1].End)
				}
			}
			for _, p := range d.address {
				if p.MatchAt(toks, i) && i+len(p) < len(toks) && hasDigit(toks[i+len(p)].Raw) {
					add(KindAddress, "hold_for_review", toks[i].Start, toks[i+len(p)].End)
				}
			}
		}
	}
	return out
}

// numeric reports a "domain" made only of digits and dots (a decimal number like 3.14 is not a link).
func numeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func hasDigit(s string) bool { return strings.IndexFunc(s, unicode.IsDigit) >= 0 }

func digitsOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// phones finds runs of consecutive digit tokens or spelled digits ("không chín một …") with ≥ minDigits digits:
// phone numbers, identity card and bank account numbers.
func (d *Detector) phones(toks []normalize.Token, add func(string, string, int, int)) {
	for i := 0; i < len(toks); {
		j, digits := i, 0
		for j < len(toks) {
			t := toks[j]
			switch {
			case digitsOnly(t.Raw):
				digits += len(t.Raw)
			case d.numbers[t.Canon] != "":
				digits++
			default:
				goto end
			}
			j++
		}
	end:
		if j > i && digits >= d.minDigits {
			add(KindPhone, "block", toks[i].Start, toks[j-1].End)
		}
		if j == i {
			i++
		} else {
			i = j
		}
	}
}

// spoken rebuilds addresses written with words ("ten a còng gmail chấm com", "abc chấm vn") into symbols and looks
// for e-mails and domains in it.
func (d *Detector) spoken(toks []normalize.Token, add func(string, string, int, int)) {
	var b strings.Builder
	owner := []int{}
	write := func(s string, tok int) {
		b.WriteString(s)
		for range len(s) {
			owner = append(owner, tok)
		}
	}
	symbols := false
	for i := 0; i < len(toks); {
		matched := false
		for _, group := range []struct {
			ps  []normalize.Phrase
			sym string
		}{{d.at, "@"}, {d.dot, "."}} {
			for _, p := range group.ps {
				if p.MatchAt(toks, i) {
					write(group.sym, i)
					i += len(p)
					matched, symbols = true, true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			continue
		}
		if b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != '@' && last != '.' {
				write(" ", i)
			}
		}
		write(toks[i].Fold, i)
		i++
	}
	if !symbols {
		return
	}
	s := strings.ReplaceAll(strings.ReplaceAll(b.String(), " @", "@"), " .", ".")
	// owner indexes shift when spaces are removed: rebuild a mapping on the compacted string.
	compact := make([]int, 0, len(s))
	src := b.String()
	for k := 0; k < len(src); k++ {
		if src[k] == ' ' && k+1 < len(src) && (src[k+1] == '@' || src[k+1] == '.') {
			continue
		}
		compact = append(compact, owner[k])
	}
	for _, re := range []*regexp.Regexp{d.symEmail, d.symDomain} {
		kind := KindEmail
		if re == d.symDomain {
			kind = KindURL
		}
		for _, loc := range re.FindAllStringIndex(s, -1) {
			if kind == KindURL && (d.allowed(s[loc[0]:loc[1]]) || numeric(s[loc[0]:loc[1]])) {
				continue
			}
			end := loc[1] - 1
			if end < loc[0] {
				end = loc[0]
			}
			add(kind, "block", toks[compact[loc[0]]].Start, toks[compact[end]].End)
		}
	}
}
