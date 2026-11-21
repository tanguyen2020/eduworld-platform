// Package hub is the domain of the hub instance (F091, MULTIPLAYER_GAMEPLAY §4–§6, §9.2, REALTIME_ARCHITECTURE §6):
// partitions, the instance assignment policy, the movement validator, the interest grid and the in-memory instance
// that the room host ticks. Pure: no I/O, the clock is passed in.
package hub

import (
	"errors"
	"math"

	"github.com/google/uuid"
)

// Grade bands (GLOSSARY "Grade band"): the default partition never mixes two bands (ADR-0054).
const (
	BandG1to3 = "g1_3"
	BandG4to5 = "g4_5"
	BandG6to7 = "g6_7"
	BandG8to9 = "g8_9"
)

// RoomTypeHub is the room_type of a hub instance.
const RoomTypeHub = "hub_instance"

// RoomTypePrivate is the room_type of the private_instance of a party session (F101): only the students of its session
// memberships may take a seat.
const RoomTypePrivate = "private_instance"

// BandOf maps a grade level (1–9) to its grade band.
func BandOf(grade int) (string, bool) {
	switch {
	case grade >= 1 && grade <= 3:
		return BandG1to3, true
	case grade == 4 || grade == 5:
		return BandG4to5, true
	case grade == 6 || grade == 7:
		return BandG6to7, true
	case grade == 8 || grade == 9:
		return BandG8to9, true
	}
	return "", false
}

// Partition is the default hub partition (tenant, school, grade band) — MULTIPLAYER_GAMEPLAY §6 [Confirmed].
type Partition struct {
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	GradeBand string
}

// Valid reports a complete partition.
func (p Partition) Valid() bool {
	return p.TenantID != uuid.Nil && p.SchoolID != uuid.Nil && p.GradeBand != ""
}

// Vec3 is a position in metres [x, y, z].
type Vec3 [3]float64

// Quantize rounds to 1 cm (AsyncAPI Vec3).
func (v Vec3) Quantize() Vec3 {
	for i := range v {
		v[i] = math.Round(v[i]*100) / 100
	}
	return v
}

// Dist is the Euclidean distance.
func (v Vec3) Dist(o Vec3) float64 {
	dx, dy, dz := v[0]-o[0], v[1]-o[1], v[2]-o[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// Finite reports a position without NaN/Inf.
func (v Vec3) Finite() bool {
	for _, c := range v {
		if math.IsNaN(c) || math.IsInf(c, 0) {
			return false
		}
	}
	return true
}

// Area is the runtime data of a world area (projection ref.world_areas of module game, F071).
type Area struct {
	ID             string
	Capacity       int
	MaxSpeedMPS    float64
	Min, Max       Vec3
	Spawns         map[string]Vec3
	Bands          []string
	RequiresUnlock bool
	Active         bool
}

// Admits reports whether the area is open to grade band.
func (a Area) Admits(band string) bool {
	for _, b := range a.Bands {
		if b == band {
			return true
		}
	}
	return false
}

// Contains reports a position inside the bounds of the area.
func (a Area) Contains(p Vec3) bool {
	for i := range p {
		if p[i] < a.Min[i] || p[i] > a.Max[i] {
			return false
		}
	}
	return true
}

// Spawn returns the named spawn point, else "default", else the centre of the bounds on the floor.
func (a Area) Spawn(name string) Vec3 {
	if p, ok := a.Spawns[name]; ok && name != "" {
		return p
	}
	if p, ok := a.Spawns["default"]; ok {
		return p
	}
	return Vec3{(a.Min[0] + a.Max[0]) / 2, a.Min[1], (a.Min[2] + a.Max[2]) / 2}
}

// Capacity returns min(area capacity, hub_instance_capacity).
func Capacity(area, platform int) int {
	if area <= 0 || (platform > 0 && platform < area) {
		return platform
	}
	return area
}

// Errors of the hub domain.
var (
	ErrFull           = errors.New("hub: instance full")
	ErrNotMember      = errors.New("hub: not a member of the instance")
	ErrWrongPartition = errors.New("hub: member outside the partition of the instance")
)
