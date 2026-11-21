// Package world is the domain of the world of EduWorld — "Quần đảo Tri Thức" (F071, WORLD_DESIGN §2–§9): areas and
// their access, unlock rules, story chapters, exploration and the instance partition. Pure: no I/O, no clock.
package world

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Area classes (world-area.schema.json area_class).
const (
	ClassHub       = "hub"
	ClassLearning  = "learning"
	ClassPractice  = "practice"
	ClassSocial    = "social"
	ClassQuest     = "quest"
	ClassExam      = "exam"
	ClassChallenge = "challenge"
	ClassArena     = "arena"
)

// Interactable kinds that count for the exploration of an area (WORLD_DESIGN §8).
const (
	KindDiscovery   = "discovery"
	KindCollectible = "collectible"
	KindNPC         = "npc"
)

// Room types of an area (access_policy.room_type).
const (
	RoomHub     = "hub_instance"
	RoomPrivate = "private_instance"
	RoomSingle  = "single"
)

// Lock reasons of listWorldAreas.
const (
	LockGradeNotAllowed     = "grade_not_allowed"
	LockRequirementsNotMet  = "requirements_not_met"
	LockAreaNotYetPublished = "not_published"
)

// Name is a bilingual display name {vi, en} (ADR-0056: both required before publish).
type Name struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Complete reports a name with both languages.
func (n Name) Complete() bool { return strings.TrimSpace(n.VI) != "" && strings.TrimSpace(n.EN) != "" }

// Vec3 is a position in metres.
type Vec3 [3]float64

// Bounds is the box an avatar may stand in (room host F091).
type Bounds struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

// AccessPolicy is access_policy of an area.
type AccessPolicy struct {
	RoomType    string `json:"room_type"`
	Public      bool   `json:"public"`
	GradeLevels []int  `json:"grade_levels,omitempty"`
}

// MapPos is the position on the World Map S04.
type MapPos struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	IconKey string  `json:"icon_key,omitempty"`
}

// SpawnPoint is a spawn anchor of the layout.
type SpawnPoint struct {
	ID       string `json:"spawn_point_id"`
	Position Vec3   `json:"position"`
}

// Portal is a wind gate of the layout.
type Portal struct {
	ID               string `json:"portal_id"`
	TargetAreaID     string `json:"target_area_id"`
	TargetSpawnPoint string `json:"target_spawn_point_id,omitempty"`
}

// Interactable is an interaction anchor of the layout.
type Interactable struct {
	ID           string   `json:"interactable_id"`
	Kind         string   `json:"kind"`
	NPCID        string   `json:"npc_id,omitempty"`
	CollectionID string   `json:"collection_id,omitempty"`
	Position     *Vec3    `json:"position,omitempty"`
	RadiusM      *float64 `json:"interact_radius_m,omitempty"`
}

// Layout is the set of anchors placed by the level designer (WORLD_RENDERING §5).
type Layout struct {
	SpawnPoints   []SpawnPoint   `json:"spawn_points"`
	Portals       []Portal       `json:"portals,omitempty"`
	Interactables []Interactable `json:"interactables,omitempty"`
}

// Area is one version of a world area (payload of specs/schemas/world-area.schema.json).
type Area struct {
	SchemaVersion int          `json:"schema_version"`
	AreaID        string       `json:"area_id"`
	RegionCode    string       `json:"region_code"`
	Name          Name         `json:"name"`
	RegionName    *Name        `json:"region_name,omitempty"`
	Class         string       `json:"area_class"`
	SubjectCode   *string      `json:"subject_code,omitempty"`
	SceneRef      string       `json:"scene_ref"`
	BundleID      string       `json:"bundle_id"`
	Capacity      int          `json:"capacity"`
	MaxSpeedMPS   float64      `json:"max_speed_mps"`
	Bounds        *Bounds      `json:"bounds,omitempty"`
	Access        AccessPolicy `json:"access_policy"`
	UnlockRule    *Rule        `json:"unlock_rule"`
	Map           *MapPos      `json:"map,omitempty"`
	Layout        Layout       `json:"layout"`
}

var (
	areaIDRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)?$`)
	anchorRe   = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)
	codeRe     = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	subjectRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	sceneRe    = regexp.MustCompile(`^res://[A-Za-z0-9_./-]+\.tscn$`)
	npcIDRe    = regexp.MustCompile(`^npc\.[a-z0-9_]+$`)
	classes    = []string{ClassHub, ClassLearning, ClassPractice, ClassSocial, ClassQuest, ClassExam, ClassChallenge, ClassArena}
	roomTypes  = []string{RoomHub, RoomPrivate, RoomSingle}
	ikinds     = []string{"npc", "quest_board", "exam_board", "portal", "quest_object", "discovery", "collectible", "info_sign", "minigame_station", "seat"}
	errInvalid = errors.New("world: invalid content")
)

// ValidAreaID reports a well-formed area_id.
func ValidAreaID(id string) bool { return len(id) <= 64 && areaIDRe.MatchString(id) }

// ValidAnchorID reports a well-formed anchor id (interactable, spawn point, portal).
func ValidAnchorID(id string) bool { return anchorRe.MatchString(id) }

// ErrInvalid wraps every content validation error (the publish answers 422).
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errInvalid, fmt.Sprintf(format, args...))
}

// IsInvalid reports a content validation error.
func IsInvalid(err error) bool { return errors.Is(err, errInvalid) }

// ParseArea decodes and checks an area payload: the rules of world-area.schema.json the server relies on, plus the
// rule that a learning area is always open (F071-AC-16): its unlock_rule may not hold min_level or
// event_participation, and no condition kind outside the schema (no payment) is accepted.
func ParseArea(raw []byte) (Area, error) {
	var a Area
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, invalid("world area: %v", err)
	}
	return a, a.Validate()
}

// Validate checks an area.
func (a Area) Validate() error {
	switch {
	case a.SchemaVersion != 1:
		return invalid("world area: schema_version must be 1")
	case !ValidAreaID(a.AreaID):
		return invalid("world area: area_id %q", a.AreaID)
	case !codeRe.MatchString(a.RegionCode):
		return invalid("world area %s: region_code", a.AreaID)
	case !a.Name.Complete() || len(a.Name.VI) > 255 || len(a.Name.EN) > 255:
		return invalid("world area %s: name needs vi and en", a.AreaID)
	case a.RegionName != nil && !a.RegionName.Complete():
		return invalid("world area %s: region_name needs vi and en", a.AreaID)
	case !slices.Contains(classes, a.Class):
		return invalid("world area %s: area_class %q", a.AreaID, a.Class)
	case a.Class == ClassLearning && (a.SubjectCode == nil || !subjectRe.MatchString(*a.SubjectCode)):
		return invalid("world area %s: a learning area needs subject_code", a.AreaID)
	case a.Class == ClassLearning && len(a.Access.GradeLevels) == 0:
		return invalid("world area %s: a learning area needs access_policy.grade_levels", a.AreaID)
	case !sceneRe.MatchString(a.SceneRef) || len(a.SceneRef) > 255:
		return invalid("world area %s: scene_ref", a.AreaID)
	case !codeRe.MatchString(a.BundleID):
		return invalid("world area %s: bundle_id", a.AreaID)
	case a.MaxSpeedMPS <= 0 || a.MaxSpeedMPS > 50 || a.Bounds == nil:
		return invalid("world area %s: max_speed_mps (0, 50] and bounds are required", a.AreaID)
	case a.Capacity < 1 || a.Capacity > 40:
		return invalid("world area %s: capacity 1..40", a.AreaID)
	case !slices.Contains(roomTypes, a.Access.RoomType):
		return invalid("world area %s: access_policy.room_type", a.AreaID)
	case len(a.Layout.SpawnPoints) == 0:
		return invalid("world area %s: layout needs a spawn point", a.AreaID)
	}
	for _, g := range a.Access.GradeLevels {
		if g < 1 || g > 9 {
			return invalid("world area %s: grade level %d", a.AreaID, g)
		}
	}
	if a.UnlockRule != nil {
		if err := a.UnlockRule.validate(0); err != nil {
			return invalid("world area %s: unlock_rule: %v", a.AreaID, err)
		}
		if a.Class == ClassLearning {
			if k := a.UnlockRule.forbiddenForLearning(); k != "" {
				return invalid("world area %s: a learning area is always open — %s is not allowed (FR-GAM-009)", a.AreaID, k)
			}
		}
	}
	seen := map[string]bool{}
	for _, s := range a.Layout.SpawnPoints {
		if !ValidAnchorID(s.ID) || seen[s.ID] {
			return invalid("world area %s: spawn point %q duplicated or invalid", a.AreaID, s.ID)
		}
		seen[s.ID] = true
	}
	for _, p := range a.Layout.Portals {
		if !ValidAnchorID(p.ID) || seen[p.ID] || !ValidAreaID(p.TargetAreaID) {
			return invalid("world area %s: portal %q duplicated or invalid", a.AreaID, p.ID)
		}
		seen[p.ID] = true
	}
	for _, it := range a.Layout.Interactables {
		if !ValidAnchorID(it.ID) || seen[it.ID] || !slices.Contains(ikinds, it.Kind) {
			return invalid("world area %s: interactable %q duplicated or invalid", a.AreaID, it.ID)
		}
		if it.Kind == KindNPC && !npcIDRe.MatchString(it.NPCID) {
			return invalid("world area %s: interactable %q needs npc_id", a.AreaID, it.ID)
		}
		seen[it.ID] = true
	}
	return nil
}

// IsLearning reports a learning area (B. Learning Zone): always open for the grade and subject of the student.
func (a Area) IsLearning() bool { return a.Class == ClassLearning }

// Subject returns the subject code of a learning area ("" otherwise).
func (a Area) Subject() string {
	if a.SubjectCode == nil {
		return ""
	}
	return *a.SubjectCode
}

// AdmitsGrade reports whether a student of grade may enter (no grade list = every grade).
func (a Area) AdmitsGrade(grade int) bool {
	return len(a.Access.GradeLevels) == 0 || slices.Contains(a.Access.GradeLevels, grade)
}

// GradeBands returns the grade bands of the admitted grades (all four when every grade is admitted).
func (a Area) GradeBands() []string {
	grades := a.Access.GradeLevels
	if len(grades) == 0 {
		grades = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	}
	var out []string
	for _, g := range grades {
		if b, ok := GradeBand(g); ok && !slices.Contains(out, b) {
			out = append(out, b)
		}
	}
	slices.Sort(out)
	return out
}

// DefaultSpawn is the first spawn point of the layout (by convention "<area_id>.default").
func (a Area) DefaultSpawn() string {
	for _, s := range a.Layout.SpawnPoints {
		if s.ID == a.AreaID+".default" {
			return s.ID
		}
	}
	return a.Layout.SpawnPoints[0].ID
}

// SpawnFor returns the spawn point of a portal arrival, else the default spawn.
func (a Area) SpawnFor(id string) string {
	for _, s := range a.Layout.SpawnPoints {
		if id != "" && s.ID == id {
			return s.ID
		}
	}
	return a.DefaultSpawn()
}

// Portal returns the portal with id.
func (a Area) Portal(id string) (Portal, bool) {
	for _, p := range a.Layout.Portals {
		if p.ID == id {
			return p, true
		}
	}
	return Portal{}, false
}

// Interactable returns the interactable with id.
func (a Area) Interactable(id string) (Interactable, bool) {
	for _, it := range a.Layout.Interactables {
		if it.ID == id {
			return it, true
		}
	}
	return Interactable{}, false
}

// Totals counts the discoveries and collectibles of the layout.
func (a Area) Totals() (discoveries, collectibles int) {
	for _, it := range a.Layout.Interactables {
		switch it.Kind {
		case KindDiscovery:
			discoveries++
		case KindCollectible:
			collectibles++
		}
	}
	return discoveries, collectibles
}

// NPCs returns the npc_id of every NPC interactable of the layout.
func (a Area) NPCs() []string {
	var out []string
	for _, it := range a.Layout.Interactables {
		if it.Kind == KindNPC && !slices.Contains(out, it.NPCID) {
			out = append(out, it.NPCID)
		}
	}
	return out
}

// GradeBand maps a grade level 1–9 to its grade band (GLOSSARY "Grade band").
func GradeBand(grade int) (string, bool) {
	switch {
	case grade >= 1 && grade <= 3:
		return "g1_3", true
	case grade == 4 || grade == 5:
		return "g4_5", true
	case grade == 6 || grade == 7:
		return "g6_7", true
	case grade == 8 || grade == 9:
		return "g8_9", true
	}
	return "", false
}
