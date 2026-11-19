package curriculum

import (
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Relations of an LO mapping (CURRICULUM_MODEL §5).
const (
	RelationSame     = "same"
	RelationSplit    = "split"
	RelationMerge    = "merge"
	RelationReplaces = "replaces"
	RelationRemoved  = "removed"
)

// MaxMappings bounds a mapping table (LoMappingList, event payload).
const MaxMappings = 5000

// Mapping maps an LO of the superseded version to an LO of the new version.
type Mapping struct {
	FromLO   uuid.UUID  `json:"from_lo_id"`
	ToLO     *uuid.UUID `json:"to_lo_id"`
	Relation string     `json:"relation"`
}

// MappingSet is the mapping table of a version.
type MappingSet []Mapping

// Validate checks every row: known relation, to_lo_id NULL exactly for removed, no duplicate row, from ∈ old LOs
// and to ∈ new LOs (data-model §1).
func (s MappingSet) Validate(oldLOs, newLOs map[uuid.UUID]bool) error {
	if len(s) > MaxMappings {
		return contentmodel.ErrInvalid{Field: "items", Reason: "has more than 5000 mappings"}
	}
	seen := map[string]bool{}
	for i, m := range s {
		field := "items[" + strconv.Itoa(i) + "]"
		switch m.Relation {
		case RelationSame, RelationSplit, RelationMerge, RelationReplaces, RelationRemoved:
		default:
			return contentmodel.ErrInvalid{Field: field + ".relation", Reason: "is not a mapping relation"}
		}
		if (m.Relation == RelationRemoved) != (m.ToLO == nil) {
			return contentmodel.ErrInvalid{Field: field + ".to_lo_id", Reason: "is empty exactly for relation removed"}
		}
		if !oldLOs[m.FromLO] {
			return contentmodel.ErrInvalid{Field: field + ".from_lo_id", Reason: "is not an objective of the superseded version"}
		}
		if m.ToLO != nil && !newLOs[*m.ToLO] {
			return contentmodel.ErrInvalid{Field: field + ".to_lo_id", Reason: "is not an objective of this version"}
		}
		k := m.FromLO.String() + "/"
		if m.ToLO != nil {
			k += m.ToLO.String()
		}
		if seen[k] {
			return contentmodel.ErrInvalid{Field: field, Reason: "duplicates another mapping"}
		}
		seen[k] = true
	}
	return nil
}

// Uncovered returns the LOs of the superseded version without any mapping (F021-AC-26), sorted.
func (s MappingSet) Uncovered(oldLOs []uuid.UUID) []uuid.UUID {
	covered := map[uuid.UUID]bool{}
	for _, m := range s {
		covered[m.FromLO] = true
	}
	var out []uuid.UUID
	for _, id := range oldLOs {
		if !covered[id] {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// SameByStableKey proposes `same` mappings for LOs whose stable_key is unchanged between the versions (used when a
// version is deep-copied: every copied LO keeps its code and stable_key).
func SameByStableKey(old, new map[string]uuid.UUID) MappingSet {
	keys := make([]string, 0, len(old))
	for k := range old {
		if _, ok := new[k]; ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	out := make(MappingSet, 0, len(keys))
	for _, k := range keys {
		to := new[k]
		out = append(out, Mapping{FromLO: old[k], ToLO: &to, Relation: RelationSame})
	}
	return out
}
