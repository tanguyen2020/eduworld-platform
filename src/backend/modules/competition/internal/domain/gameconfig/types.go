// Package gameconfig is the domain of the competition configurations of the Game Operations Console (F108, API_DESIGN
// §9.13b, COMPETITION_SYSTEM §13, ADR-0043): the seven config types served by module competition, the review workflow of a
// version (draft → in_review → approved/rejected → published → archived, reviewer ≠ author), the release rules (publish
// with effective_from ≥ now, rollback to a version published before, unpublish), the business invariants of each type
// (grade constraints, speed bonus, tiers, leaderboard scopes, template parameters, cosmetic-only rewards — F108-AC-04)
// and the diff of two versions. Pure Go, no I/O.
package gameconfig

import (
	"errors"
	"slices"
)

// ErrUnknownType: the config type is not served by module competition (404 NOT_FOUND).
var ErrUnknownType = errors.New("gameconfig: config type not served by module competition")

// Config types of module competition (GameConfigType, x-service-routing competition).
const (
	TypeMatchmaking  = "matchmaking-configs"
	TypeRanking      = "ranking-rules"
	TypeSeasons      = "seasons"
	TypeTournamentTp = "tournament-templates"
	TypeChallengeTp  = "challenge-templates"
	TypeLeaderboards = "leaderboard-definitions"
	TypeRewards      = "competition-rewards"
)

// DefaultCode is the code of a singleton configuration (one catalog per type, read by the runtime).
const DefaultCode = "default"

// Type describes one config type: its path name, whether it is a singleton catalog (code default, one config of the type),
// what the release API offers for it and the assignment resource type of a template.
type Type struct {
	Name string
	// Singleton: the type has exactly one configuration (code default) the runtime reads; a second create is 409 and a
	// change is a new version of it.
	Singleton bool
	// Unpublish: the configuration can be withdrawn. The singleton catalogs the runtime needs (matchmaking, ranking,
	// seasons, leaderboard definitions) cannot (a fix is a rollback or a new version — Clarifications 2026-11-25).
	Unpublish bool
	// Assignable is the resource_type of infra.tenant_resource_assignments ("" = not assignable).
	Assignable string
}

var types = []Type{
	{Name: TypeMatchmaking, Singleton: true},
	{Name: TypeRanking, Singleton: true},
	{Name: TypeSeasons, Singleton: true},
	{Name: TypeTournamentTp, Unpublish: true, Assignable: "tournament_template"},
	{Name: TypeChallengeTp, Unpublish: true, Assignable: "challenge_template"},
	{Name: TypeLeaderboards, Singleton: true},
	{Name: TypeRewards, Unpublish: true},
}

// Lookup returns the config type of a path name.
func Lookup(name string) (Type, error) {
	i := slices.IndexFunc(types, func(t Type) bool { return t.Name == name })
	if i < 0 {
		return Type{}, ErrUnknownType
	}
	return types[i], nil
}

// Types returns every config type served by module competition (routing, test matrices, metrics).
func Types() []Type { return slices.Clone(types) }

// Names returns the config_type values served by module competition.
func Names() []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, t.Name)
	}
	return out
}

// HasCode reports whether the payload of the type carries its config code (templates and reward sets: several
// configurations of the type, the code chosen at creation then kept by the server).
func (t Type) HasCode() bool { return !t.Singleton }

// Normalize fills the payload fields the server owns: the code of a multi-configuration type is the code of its
// configuration (a client value is ignored after creation).
func (t Type) Normalize(doc map[string]any, code string) {
	if t.HasCode() && code != "" {
		doc["code"] = code
	}
}
