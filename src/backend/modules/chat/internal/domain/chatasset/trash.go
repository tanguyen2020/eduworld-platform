package chatasset

import "errors"

// Trash of the sticker packs (ADR-0028, ADMIN_PORTAL §3.8): module identity routes sticker_pack to module chat. A pack is
// platform content of chat.sticker_packs (owner_scope platform); a tenant trash never has one. GIFs are a shared library
// outside the trash (never assigned, Clarifications F082 2026-11-26).

// ResourceStickerPack is the trash resource type of module chat.
const ResourceStickerPack = "sticker_pack"

// ErrNotInTrash: the pack is not soft-deleted (404 NOT_FOUND).
var ErrNotInTrash = errors.New("chatasset: sticker pack is not in the trash")

// ErrRestoreConflict: a live pack holds the code of the deleted one (partial unique index, 409 CONFLICT).
var ErrRestoreConflict = errors.New("chatasset: a live sticker pack already holds the code of this one")
