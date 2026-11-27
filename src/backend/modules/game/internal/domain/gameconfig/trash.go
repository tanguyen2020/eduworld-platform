package gameconfig

import (
	"errors"
	"slices"
)

// Trash of the game resources (ADR-0028, ADMIN_PORTAL §3.8): module identity routes these resource types of the
// ResourceType enum to module game (platformadmin owners table); each one is a console item of game.config_items whose
// item_type equals the resource type. They are platform content (owner_scope platform): a tenant trash never has them.
var trashTypes = []string{"challenge", "quest", "minigame_config", "item", "shop_item", "event_template"}

// ErrNotInTrash: the item is not soft-deleted, or not an item of the type (404 NOT_FOUND).
var ErrNotInTrash = errors.New("gameconfig: item is not in the trash")

// ErrRestoreConflict: a live item holds the unique key (item_type, item_key) of the deleted one (409 CONFLICT).
var ErrRestoreConflict = errors.New("gameconfig: a live item already holds the key of this one")

// TrashTypes returns the resource types of the trash served by module game.
func TrashTypes() []string { return slices.Clone(trashTypes) }

// IsTrashType reports whether rt is a trash resource type of module game.
func IsTrashType(rt string) bool { return slices.Contains(trashTypes, rt) }
