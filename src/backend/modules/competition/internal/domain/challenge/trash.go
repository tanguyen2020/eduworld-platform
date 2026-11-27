package challenge

import "errors"

// Trash of the competition templates (ADR-0028, ADMIN_PORTAL §3.8): module identity routes tournament_template and
// challenge_template to module competition; each one is a platform configuration of competition.game_configs (config
// type tournament-templates / challenge-templates). A tenant trash never has them.

// ErrNotInTrash: the template is not soft-deleted, or not of the type (404 NOT_FOUND).
var ErrNotInTrash = errors.New("challenge: template is not in the trash")

// ErrRestoreConflict: a live configuration holds the unique key (config_type, code) of the deleted one (409 CONFLICT).
var ErrRestoreConflict = errors.New("challenge: a live configuration already holds the code of this template")

// TrashTypes returns the resource types of the trash served by module competition.
func TrashTypes() []string { return []string{ResourceTournamentTemplate, ResourceChallengeTemplate} }
