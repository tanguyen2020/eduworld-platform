// Package platformadmin holds the rules of Platform Administration (F045, ADR-0027, ADR-0028, ADR-0029,
// ADR-0074): which module owns a resource type, what can be assigned, cloned or restored, the clone plan and the
// invariants of a clone job and of an account merge. Pure Go, no infrastructure.
package platformadmin

import (
	"errors"
	"fmt"
	"slices"
)

// Module names of the owners (ADR-0074).
const (
	ModuleIdentity    = "identity"
	ModuleLearning    = "learning"
	ModuleGame        = "game"
	ModuleCompetition = "competition"
	ModuleChat        = "chat"
)

// ParticipantModules are the modules a merge saga calls (mergeUserData of ACCOUNT_PROVISIONING §7b).
var ParticipantModules = []string{ModuleLearning, ModuleGame, ModuleCompetition, ModuleChat}

// Identity-owned trash types: merged accounts (read-only in the trash, F045-AC-33) and students cleaned up by
// the inactive-student cleanup (restored by the F016 saga student_restore, F016-AC-19).
const (
	ResourceUser    = "user"
	ResourceStudent = "student"
)

// ErrUnknownResourceType is a resource_type outside the ResourceType enum.
var ErrUnknownResourceType = errors.New("platformadmin: unknown resource_type")

// owners is the routing table resource_type → owning module (DATABASE_DESIGN §2.17, the ResourceType enum of
// specs/openapi/eduworld-api.v1.yaml); one place in the code (constitution VII).
var owners = map[string]string{
	"curriculum": ModuleLearning, "lesson": ModuleLearning, "learning_material": ModuleLearning, "question": ModuleLearning,
	"question_set": ModuleLearning, "assessment_blueprint": ModuleLearning, "media": ModuleLearning, "assignment": ModuleLearning,
	"course": ModuleLearning, "learning_path": ModuleLearning,
	"challenge": ModuleGame, "challenge_template": ModuleGame, "quest": ModuleGame, "minigame_config": ModuleGame, "item": ModuleGame,
	"tournament_template": ModuleCompetition,
	"class_template":      ModuleIdentity, "school_settings": ModuleIdentity, "tenant_settings": ModuleIdentity,
	ResourceUser: ModuleIdentity, ResourceStudent: ModuleIdentity,
}

// cloneAllowlist are the types a clone job may copy (data-model §3): configuration and content only, never
// student data (AGENTS R22).
var cloneAllowlist = []string{"curriculum", "lesson", "learning_material", "question", "question_set", "assessment_blueprint", "media",
	"learning_path", "assignment", "challenge_template", "class_template", "school_settings", "tenant_settings"}

// Owner returns the module owning rt.
func Owner(rt string) (string, error) {
	m, ok := owners[rt]
	if !ok {
		return "", fmt.Errorf("%w %q", ErrUnknownResourceType, rt)
	}
	return m, nil
}

// Assignable reports whether rt can be assigned to tenants (platform data of another module; the user type
// and the identity configuration types are not platform resources).
func Assignable(rt string) bool {
	m, ok := owners[rt]
	return ok && m != ModuleIdentity
}

// Clonable reports whether rt is in the clone allowlist.
func Clonable(rt string) bool { return slices.Contains(cloneAllowlist, rt) }

// RestorableThroughTrash reports whether a trash item of rt can be restored from the trash: a merged account
// never is (its identities moved to the target account); a cleaned-up student is, through the F016 saga.
func RestorableThroughTrash(rt string) bool { return rt != ResourceUser }

// DeleteReasonOf is the users.delete_reason of an identity-owned account trash type.
func DeleteReasonOf(rt string) string {
	if rt == ResourceStudent {
		return "inactive_cleanup"
	}
	return "merged"
}
