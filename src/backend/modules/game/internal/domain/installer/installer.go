// Package installer holds the rules of the school installer page (F134 US3, DEPLOYMENT §9, PERFORMANCE_BUDGET §10.1):
// the signed school-kit index of the release CDN lists Android kits (APK + region packs per grade band, ADR-0101: no
// Windows installer), and a school gets the latest kit of each platform × grade-band set that matches its grades.
package installer

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Grade bands of GLOSSARY (g1_3, g4_5, g6_7, g8_9).
var GradeBands = []string{"g1_3", "g4_5", "g6_7", "g8_9"}

// GradeBandOf maps a grade level 1..9 to its band ("" outside 1..9).
func GradeBandOf(level int) string {
	switch {
	case level >= 1 && level <= 3:
		return "g1_3"
	case level == 4 || level == 5:
		return "g4_5"
	case level == 6 || level == 7:
		return "g6_7"
	case level == 8 || level == 9:
		return "g8_9"
	}
	return ""
}

// BandsOf returns the sorted distinct bands of grade levels.
func BandsOf(levels []int) []string {
	var out []string
	for _, l := range levels {
		if b := GradeBandOf(l); b != "" && !slices.Contains(out, b) {
			out = append(out, b)
		}
	}
	sort.Strings(out)
	return out
}

// Kit is one installer of the index (specs/schemas/school-kit-index.schema.json).
type Kit struct {
	KitID         string    `json:"kit_id"`
	Platform      string    `json:"platform"`
	ClientVersion string    `json:"client_version"`
	GradeBands    []string  `json:"grade_bands"`
	URL           string    `json:"url"`
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	MinOS         string    `json:"min_os"`
	ReleasedAt    time.Time `json:"released_at"`
}

// Index is the school-kit index of one channel.
type Index struct {
	SchemaVersion int       `json:"schema_version"`
	Channel       string    `json:"channel"`
	GeneratedAt   time.Time `json:"generated_at"`
	Kits          []Kit     `json:"kits"`
}

// ErrInvalidIndex: the index does not follow the schema (the page answers 503, never an unchecked link).
var ErrInvalidIndex = errors.New("installer: invalid school-kit index")

var (
	kitIDRe   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	semverRe  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	sha256Re  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	maxKits   = 50
	maxURLLen = 2048
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidIndex, fmt.Sprintf(format, a...))
}

// Validate checks the index against school-kit-index.schema.json (F134-AC-14): schema_version 1, channel, at most 50
// kits, every field bounded (R28), platform android only (ADR-0101), https URLs only.
func (ix Index) Validate(channel string) error {
	if ix.SchemaVersion != 1 {
		return invalid("schema_version %d", ix.SchemaVersion)
	}
	if ix.Channel != channel {
		return invalid("channel %q, want %q", ix.Channel, channel)
	}
	if ix.GeneratedAt.IsZero() {
		return invalid("generated_at missing")
	}
	if len(ix.Kits) > maxKits {
		return invalid("%d kits (max %d)", len(ix.Kits), maxKits)
	}
	for i, k := range ix.Kits {
		if err := k.validate(); err != nil {
			return invalid("kits/%d: %v", i, err)
		}
	}
	return nil
}

func (k Kit) validate() error {
	switch {
	case !kitIDRe.MatchString(k.KitID):
		return errors.New("kit_id")
	case k.Platform != "android":
		return errors.New("platform must be android")
	case len(k.ClientVersion) > 32 || !semverRe.MatchString(k.ClientVersion):
		return errors.New("client_version")
	case len(k.GradeBands) < 1 || len(k.GradeBands) > 4:
		return errors.New("grade_bands")
	case !sha256Re.MatchString(k.SHA256):
		return errors.New("sha256")
	case k.SizeBytes < 0:
		return errors.New("size_bytes")
	case k.MinOS == "" || len(k.MinOS) > 32:
		return errors.New("min_os")
	case k.ReleasedAt.IsZero():
		return errors.New("released_at")
	}
	seen := map[string]bool{}
	for _, b := range k.GradeBands {
		if !slices.Contains(GradeBands, b) || seen[b] {
			return errors.New("grade_bands")
		}
		seen[b] = true
	}
	if len(k.URL) > maxURLLen {
		return errors.New("url")
	}
	u, err := url.Parse(k.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("url must be an absolute https URL")
	}
	return nil
}

// ForSchool returns the kits whose bands meet the school's bands (every kit when the school has no active student —
// spec F134-AC-13 "trường chưa có lớp", ADR-0095 no class), keeping the latest released_at of each platform × band set,
// sorted by platform then bands.
func ForSchool(kits []Kit, schoolBands []string) []Kit {
	latest := map[string]Kit{}
	for _, k := range kits {
		if len(schoolBands) > 0 && !intersects(k.GradeBands, schoolBands) {
			continue
		}
		key := k.Platform + "|" + strings.Join(sortedCopy(k.GradeBands), ",")
		if cur, ok := latest[key]; !ok || k.ReleasedAt.After(cur.ReleasedAt) {
			latest[key] = k
		}
	}
	keys := make([]string, 0, len(latest))
	for k := range latest {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Kit, 0, len(keys))
	for _, k := range keys {
		out = append(out, latest[k])
	}
	return out
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}
