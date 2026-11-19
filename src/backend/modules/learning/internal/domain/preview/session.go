// Package preview is the domain of the preview sessions of learning content (F027 US4, ADMIN_PORTAL §5): a short
// lived view of one version as a student would see it, never an attempt (ADR-0031). Pure Go, no I/O.
package preview

import (
	"time"

	"github.com/google/uuid"
)

// Item is one question of a previewed content at its position, with the shuffle seed of its render.
type Item struct {
	Position          int       `json:"position"`
	QuestionVersionID uuid.UUID `json:"question_version_id"`
	Seed              uint64    `json:"seed"`
}

// Session is a preview session: who opened it, for which tenant scope and version, until when.
type Session struct {
	ID              uuid.UUID  `json:"preview_session_id"`
	TenantID        *uuid.UUID `json:"tenant_id,omitempty"` // session tenant of the principal; nil for platform staff
	PrincipalID     uuid.UUID  `json:"principal_id"`
	ContentType     string     `json:"content_type"`
	ContentID       uuid.UUID  `json:"content_id"`
	Version         int        `json:"version"`
	DisplayLanguage string     `json:"display_language"`
	Items           []Item     `json:"items"`
	ExpiresAt       time.Time  `json:"expires_at"`
}

// Item returns the item at position.
func (s Session) Item(position int) (Item, bool) {
	for _, it := range s.Items {
		if it.Position == position {
			return it, true
		}
	}
	return Item{}, false
}

// OwnedBy reports whether the principal and tenant of a request own the session: another principal, or the same
// one in another tenant session, sees no session (404, F027-AC-19).
func (s Session) OwnedBy(principal uuid.UUID, tenant *uuid.UUID) bool {
	if s.PrincipalID != principal {
		return false
	}
	if (s.TenantID == nil) != (tenant == nil) {
		return false
	}
	return s.TenantID == nil || *s.TenantID == *tenant
}

// Expired reports whether the session is over at now.
func (s Session) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }

// Language maps a display Locale (vi-VN, en-US) to the content language key (vi, en); empty → content language.
func Language(locale, contentLanguage string) string {
	switch locale {
	case "en-US":
		return "en"
	case "vi-VN":
		return "vi"
	}
	if contentLanguage != "" {
		return contentLanguage
	}
	return "vi"
}

// Localize resolves the bilingual texts of a decoded render ({"vi": …, "en": …}, i18nText of ADR-0056) to lang,
// falling back to the other language when the translation is missing (F027-AC-16 display_language).
func Localize(v any, lang string) any {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := i18n(t, lang); ok {
			return s
		}
		for k, val := range t {
			t[k] = Localize(val, lang)
		}
		return t
	case []any:
		for i := range t {
			t[i] = Localize(t[i], lang)
		}
		return t
	}
	return v
}

// i18n returns the text of an object whose keys are only vi/en with string values.
func i18n(m map[string]any, lang string) (string, bool) {
	if len(m) == 0 || len(m) > 2 {
		return "", false
	}
	for k, val := range m {
		if _, ok := val.(string); !ok || (k != "vi" && k != "en") {
			return "", false
		}
	}
	if s, ok := m[lang].(string); ok && s != "" {
		return s, true
	}
	for _, k := range []string{"vi", "en"} {
		if s, ok := m[k].(string); ok && s != "" {
			return s, true
		}
	}
	return "", true
}
