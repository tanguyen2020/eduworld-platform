// Package i18n holds the translation files of module learning served by the API (ADR-0056): the reasons of the
// automatic recommendations (F032 T003), keyed by reason_code, in Vietnamese and English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

//go:embed recommendation_reasons.*.json
var files embed.FS

// MaxReasonText is the maxLength of RecommendationList.items[].reason_text.
const MaxReasonText = 300

// Reasons implements read.ReasonCatalog from the embedded files.
type Reasons struct {
	byLang map[string]map[string]string
}

var _ read.ReasonCatalog = Reasons{}

// LoadReasons reads the embedded translation files.
func LoadReasons() (Reasons, error) {
	r := Reasons{byLang: map[string]map[string]string{}}
	for _, lang := range []string{"vi", "en"} {
		b, err := files.ReadFile("recommendation_reasons." + lang + ".json")
		if err != nil {
			return r, fmt.Errorf("recommendation reasons %s: %w", lang, err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			return r, fmt.Errorf("recommendation reasons %s: %w", lang, err)
		}
		r.byLang[lang] = m
	}
	return r, nil
}

// MustReasons is LoadReasons for the composition root (the files are embedded: an error is a build defect).
func MustReasons() Reasons {
	r, err := LoadReasons()
	if err != nil {
		panic(err)
	}
	return r
}

// Text implements read.ReasonCatalog: the template of code in lang (Vietnamese when missing) with {name} replaced by the
// parameters, cut to MaxReasonText characters; an unknown code renders as the code.
func (r Reasons) Text(code, lang string, params map[string]string) string {
	if lang != "en" {
		lang = "vi"
	}
	t, ok := r.byLang[lang][code]
	if !ok {
		if t, ok = r.byLang["vi"][code]; !ok {
			return code
		}
	}
	for k, v := range params {
		t = strings.ReplaceAll(t, "{"+k+"}", v)
	}
	if rs := []rune(t); len(rs) > MaxReasonText {
		t = string(rs[:MaxReasonText-1]) + "…"
	}
	return t
}
