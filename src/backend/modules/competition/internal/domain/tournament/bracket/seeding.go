package bracket

import (
	"errors"
	"math/rand/v2"
	"sort"

	"github.com/google/uuid"
)

// Seeding policies (COMPETITION_SYSTEM §10.5). separate_class is gone with the classes (ADR-0095).
const (
	SeedRating = "rating"
	SeedRandom = "random"
	SeedManual = "manual"
)

// Policies are the accepted seeding policies.
var Policies = []string{SeedRating, SeedRandom, SeedManual}

// ErrUnknownPolicy: a seeding policy outside Policies (separate_class included, F107-AC-13).
var ErrUnknownPolicy = errors.New("bracket: unknown seeding policy")

// ErrManualOrder: the manual order is not a permutation of the entries.
var ErrManualOrder = errors.New("bracket: the manual order must list every entry once")

// Contender is an entry to seed: its rating (mean μ of the season, the average of the members for a team) and whether
// it is provisional (provisional entries go after the rated ones).
type Contender struct {
	ID          uuid.UUID
	Mu          float64
	Provisional bool
}

// ValidPolicy reports whether p is a seeding policy.
func ValidPolicy(p string) bool {
	for _, x := range Policies {
		if x == p {
			return true
		}
	}
	return false
}

// Order returns the entries in seed order (index 0 = seed 1, F107-AC-13): rating — rated before provisional, μ
// descending, id as the last tie-break; random — a shuffle driven by seed (stored for the audit: the same seed gives the
// same order); manual — exactly the given order.
func Order(policy string, cs []Contender, seed int64, manual []uuid.UUID) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(cs))
	switch policy {
	case SeedRating:
		sorted := append([]Contender(nil), cs...)
		sort.SliceStable(sorted, func(i, j int) bool {
			a, b := sorted[i], sorted[j]
			if a.Provisional != b.Provisional {
				return !a.Provisional
			}
			if a.Mu != b.Mu {
				return a.Mu > b.Mu
			}
			return a.ID.String() < b.ID.String()
		})
		for i, c := range sorted {
			out[i] = c.ID
		}
	case SeedRandom:
		sorted := append([]Contender(nil), cs...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID.String() < sorted[j].ID.String() })
		r := rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9e3779b97f4a7c15)) //nolint:gosec // reproducible audit shuffle, not a secret
		r.Shuffle(len(sorted), func(i, j int) { sorted[i], sorted[j] = sorted[j], sorted[i] })
		for i, c := range sorted {
			out[i] = c.ID
		}
	case SeedManual:
		if len(manual) != len(cs) {
			return nil, ErrManualOrder
		}
		known := map[uuid.UUID]bool{}
		for _, c := range cs {
			known[c.ID] = true
		}
		seen := map[uuid.UUID]bool{}
		for i, id := range manual {
			if !known[id] || seen[id] {
				return nil, ErrManualOrder
			}
			seen[id] = true
			out[i] = id
		}
	default:
		return nil, ErrUnknownPolicy
	}
	return out, nil
}
