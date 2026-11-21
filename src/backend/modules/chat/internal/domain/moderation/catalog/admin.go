package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Scopes of a catalogue part.
const (
	ScopePlatform = "platform"
	ScopeTenant   = "tenant"
)

// Term status (MODERATION §11.4: a write stays a draft until the next publish).
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Locales of a term or allowlist entry (both catalogues run on every message, NFR-I18N-004).
const (
	LocaleVI = "vi"
	LocaleEN = "en"
)

// Limits of a stored term (SensitiveTermInput, MODERATION §11.1).
const (
	MaxTermLen  = 200
	MaxVariants = 50
)

// StoredCategory is a platform category row (chat.sensitive_term_categories, tenant_id always NULL).
type StoredCategory struct {
	ID                uuid.UUID
	Code              string
	Name              string
	Description       *string
	Severity          string
	DefaultAction     string
	Enabled           bool
	AllowTenantToggle bool
	Version           int64
	UpdatedAt         time.Time
}

// Snapshot returns the category as a snapshot entry.
func (c StoredCategory) Snapshot() Category {
	return Category{ID: c.ID, Code: c.Code, Severity: c.Severity, DefaultAction: c.DefaultAction, Enabled: c.Enabled,
		AllowTenantToggle: c.AllowTenantToggle}
}

// StoredTerm is a term row (chat.sensitive_terms): TenantID nil = platform term, otherwise an extra term of a tenant.
type StoredTerm struct {
	ID                 uuid.UUID
	TenantID           *uuid.UUID
	CategoryID         uuid.UUID
	Term               string
	MatchType          string
	Locale             string
	Variants           []string
	SeverityOverride   *string
	ActionOverride     *string
	Enabled            bool
	Status             string
	Note               *string
	PublishedInVersion *int64
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Scope returns platform or tenant.
func (t StoredTerm) Scope() string {
	if t.TenantID == nil {
		return ScopePlatform
	}
	return ScopeTenant
}

// Snapshot returns the term as a snapshot entry.
func (t StoredTerm) Snapshot() Term {
	s := Term{ID: t.ID, CategoryID: t.CategoryID, Term: t.Term, MatchType: t.MatchType, Locale: t.Locale, Variants: t.Variants}
	if t.SeverityOverride != nil {
		s.SeverityOverride = *t.SeverityOverride
	}
	if t.ActionOverride != nil {
		s.ActionOverride = *t.ActionOverride
	}
	return s
}

// StoredAllow is an allowlist row (chat.sensitive_allowlist; platform only, see AllowlistScope).
type StoredAllow struct {
	ID        uuid.UUID
	TenantID  *uuid.UUID
	Phrase    string
	Locale    string
	Reason    *string
	CreatedAt time.Time
}

// StoredVersion is a published catalogue version (chat.sensitive_catalog_versions, append-only).
type StoredVersion struct {
	ID            uuid.UUID
	TenantID      *uuid.UUID
	Scope         string
	Version       int64
	PublishedAt   time.Time
	PublishedBy   *uuid.UUID
	ChangeSummary string
	TermCount     int
	Checksum      string
	Snapshot      []byte
	Recall        *float64
	FPRate        *float64
}

// ErrInvalidTerm: a term does not satisfy the catalogue rules (SENSITIVE_TERM_INVALID). Field names the input field;
// the message never quotes the term (R14, R15).
type ErrInvalidTerm struct {
	Field  string
	Reason string
}

func (e ErrInvalidTerm) Error() string { return "catalog: invalid " + e.Field + ": " + e.Reason }

var validSeverity = map[string]bool{SeverityWatch: true, SeverityLow: true, SeverityMedium: true, SeverityHigh: true, SeverityCritical: true}

var validAction = map[string]bool{"allow": true, "mask": true, "block": true, "hold_for_review": true, "block_and_alert": true}

// ValidSeverity reports a known severity.
func ValidSeverity(s string) bool { return validSeverity[s] }

// ValidAction reports a known action.
func ValidAction(a string) bool { return validAction[a] }

// ValidateTerm checks a term before it is stored (F116-AC-12): known match type and locale, 1..200 characters, a
// pattern is an allowed RE2 expression (ValidatePattern) without variants, at most 50 non-empty variants, known
// overrides.
func ValidateTerm(t StoredTerm) error {
	switch t.MatchType {
	case MatchWord, MatchPhrase, MatchContains, MatchPattern:
	default:
		return ErrInvalidTerm{Field: "match_type", Reason: "unknown match type"}
	}
	if t.Locale != LocaleVI && t.Locale != LocaleEN {
		return ErrInvalidTerm{Field: "locale", Reason: "must be vi or en"}
	}
	n := utf8.RuneCountInString(t.Term)
	if strings.TrimSpace(t.Term) == "" || n > MaxTermLen {
		return ErrInvalidTerm{Field: "term", Reason: "must have 1 to 200 characters"}
	}
	if t.MatchType == MatchPattern {
		if err := ValidatePattern(t.Term); err != nil {
			return ErrInvalidTerm{Field: "term", Reason: "pattern must be RE2 without backreference or lookaround"}
		}
		if len(t.Variants) > 0 {
			return ErrInvalidTerm{Field: "variants", Reason: "a pattern has no variants"}
		}
	}
	if len(t.Variants) > MaxVariants {
		return ErrInvalidTerm{Field: "variants", Reason: "at most 50 variants"}
	}
	for _, v := range t.Variants {
		if strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > MaxTermLen {
			return ErrInvalidTerm{Field: "variants", Reason: "each variant must have 1 to 200 characters"}
		}
	}
	if t.SeverityOverride != nil && !ValidSeverity(*t.SeverityOverride) {
		return ErrInvalidTerm{Field: "severity_override", Reason: "unknown severity"}
	}
	if t.ActionOverride != nil && !ValidAction(*t.ActionOverride) {
		return ErrInvalidTerm{Field: "action_override", Reason: "unknown action"}
	}
	return nil
}

// Encode serialises a snapshot (the snapshot column) and returns its sha256 checksum (hex).
func Encode(s Snapshot) ([]byte, string, error) {
	if s.Terms == nil {
		s.Terms = []Term{}
	}
	if s.Allowlist == nil {
		s.Allowlist = []AllowEntry{}
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, "", fmt.Errorf("catalog: encode snapshot: %w", err)
	}
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), nil
}

// ErrProtectedCategory: the grooming category cannot be removed, switched off or opened to tenant toggles (grooming
// detection always runs, ADR-0078).
var ErrProtectedCategory = errors.New("catalog: protected category")

// ProtectedCategory is the category code of the always-on grooming detection.
const ProtectedCategory = "grooming_scam"

// CheckCategoryChange refuses a change that would weaken the grooming category.
func CheckCategoryChange(code string, enabled, allowToggle, deleting bool) error {
	if code != ProtectedCategory {
		return nil
	}
	if deleting || !enabled || allowToggle {
		return ErrProtectedCategory
	}
	return nil
}

// Lookup implements Categories for the catalogue in force for a tenant: a category is in force when enabled on the
// platform, unless the tenant setting (allowed only when allow_tenant_toggle) says otherwise.
type Lookup struct {
	Cats     map[uuid.UUID]Category
	Settings map[uuid.UUID]bool
}

// NewLookup indexes the categories of a platform snapshot with the settings of a tenant (nil: none).
func NewLookup(cats []Category, settings map[uuid.UUID]bool) Lookup {
	l := Lookup{Cats: make(map[uuid.UUID]Category, len(cats)), Settings: settings}
	for _, c := range cats {
		l.Cats[c.ID] = c
	}
	return l
}

// Lookup implements Categories.
func (l Lookup) Lookup(id uuid.UUID) (Category, bool) {
	cat, ok := l.Cats[id]
	if !ok {
		return Category{}, false
	}
	on := cat.Enabled
	if v, set := l.Settings[id]; set && cat.AllowTenantToggle {
		on = v
	}
	return cat, on
}
