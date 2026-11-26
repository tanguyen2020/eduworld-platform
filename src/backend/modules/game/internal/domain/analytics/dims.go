package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// Dimension keys of the rollups (GAME_ANALYTICS §6, DASHBOARD_DATA §3.3). The tenant is a column, never a dims key.
const (
	DimSchool     = "school_id"
	DimGradeBand  = "grade_band"
	DimPlatform   = "platform"
	DimLocale     = "locale"
	DimDeviceTier = "device_tier"
	DimReason     = "reason_code"
	DimEvent      = "event_id"
)

// Unknown is the dims value of a player whose dimension is not known (never confused with the whole-tenant set).
const Unknown = "unknown"

// DimSets are the fixed dimension sets of the rollups: (tenant), (tenant, school), (tenant, grade_band), (tenant, platform),
// (tenant, locale), (tenant, device_tier) plus (tenant, school, grade_band), (tenant, school, platform) and (tenant, school,
// device_tier) for the school report of a School Admin (spec Clarifications 2026-11-26). No other product is stored.
var DimSets = [][]string{{}, {DimSchool}, {DimGradeBand}, {DimPlatform}, {DimLocale}, {DimDeviceTier}, {DimSchool, DimGradeBand},
	{DimSchool, DimPlatform}, {DimSchool, DimDeviceTier}}

// Dims is the dimension values of one rollup row.
type Dims map[string]string

// Key is the canonical form of the dims (sorted key=value pairs), stable across processes.
func (d Dims) Key() string {
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(d[k])
	}
	return b.String()
}

// Hash is dims_hash: SHA-256 hex of Key (the whole-tenant set hashes the empty string).
func (d Dims) Hash() string {
	h := sha256.Sum256([]byte(d.Key()))
	return hex.EncodeToString(h[:])
}

// Set is the sorted list of keys of the dims (its dimension set).
func (d Dims) Set() string {
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// GradeBand maps a grade level to its band of the contract (GradeBand: g1_3, g4_5, g6_7, g8_9).
func GradeBand(grade int) string {
	switch {
	case grade >= 1 && grade <= 3:
		return "g1_3"
	case grade == 4 || grade == 5:
		return "g4_5"
	case grade == 6 || grade == 7:
		return "g6_7"
	case grade == 8 || grade == 9:
		return "g8_9"
	}
	return Unknown
}

// Filter is the dimension filter of a dashboard query: at most one standard dimension (plus the school of a school
// scope, combined with grade bands through the (school, grade_band) set).
type Filter struct {
	Key    string   // "" = whole scope
	Values []string // values of Key (nil = every value of the dimension)
}

// ErrTwoDimensions is returned when a query combines two dimension filters that no rollup set stores (422).
var ErrTwoDimensions = errors.New("only one dimension filter at a time (school, grade band, platform, locale or device tier)")

// NewFilter builds the filter of the optional value lists of a query; school + grade band is the only allowed pair and
// becomes the set (school, grade_band) whose values are "school|band".
func NewFilter(lists map[string][]string) (Filter, error) {
	var set []string
	for _, k := range []string{DimSchool, DimGradeBand, DimPlatform, DimLocale, DimDeviceTier} {
		if len(lists[k]) > 0 {
			set = append(set, k)
		}
	}
	switch {
	case len(set) == 0:
		return Filter{}, nil
	case len(set) == 1:
		return Filter{Key: set[0], Values: lists[set[0]]}, nil
	case len(set) == 2 && set[0] == DimSchool && set[1] == DimGradeBand:
		var vals []string
		for _, s := range lists[DimSchool] {
			for _, g := range lists[DimGradeBand] {
				vals = append(vals, s+"|"+g)
			}
		}
		return Filter{Key: DimSchool + "," + DimGradeBand, Values: vals}, nil
	}
	return Filter{}, ErrTwoDimensions
}

// ValueOf is the filter value of dims of the filter's set ("school|band" for the pair).
func (f Filter) ValueOf(d Dims) string {
	if f.Key == DimSchool+","+DimGradeBand {
		return d[DimSchool] + "|" + d[DimGradeBand]
	}
	return d[f.Key]
}

// Matches reports whether the dims value is selected (every value when Values is nil).
func (f Filter) Matches(d Dims) bool {
	if len(f.Values) == 0 {
		return true
	}
	v := f.ValueOf(d)
	for _, x := range f.Values {
		if x == v {
			return true
		}
	}
	return false
}
