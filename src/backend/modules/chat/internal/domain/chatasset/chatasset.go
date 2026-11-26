// Package chatasset is the platform sticker/GIF library of module chat (F118 US3, CHAT_SYSTEM §7.5, ADR-0027, ADR-0034):
// versions of a sticker pack or GIF composed by Game Studio, their workflow (draft → in_review → approved → published →
// archived; GIF approved only by someone else than its author) and the file rules (sticker WebP/PNG ≤ 512 KB; GIF
// ≤ 2 MB, ≤ 480 px, with a static preview). Pure Go: no I/O.
package chatasset

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Type is config_type of /game-admin/{config_type}.
type Type string

// Config types served by module chat.
const (
	TypeStickerPacks Type = "sticker-packs"
	TypeGifs         Type = "gifs"
)

// Valid reports a config type of module chat.
func (t Type) Valid() bool { return t == TypeStickerPacks || t == TypeGifs }

// ItemType is content.published item_type of the type.
func (t Type) ItemType() string {
	if t == TypeGifs {
		return "gif"
	}
	return "sticker_pack"
}

// Status of a version (ContentStatus).
type Status string

// Statuses.
const (
	StatusDraft     Status = "draft"
	StatusInReview  Status = "in_review"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

// Transition actions of transitionGameConfigVersion.
const (
	ActionSubmit  = "submit_review"
	ActionApprove = "approve"
	ActionReject  = "reject"
	ActionArchive = "archive"
)

// Errors of the workflow and of the payloads.
var (
	ErrTransition   = errors.New("chatasset: transition not allowed from this status")
	ErrSelfReview   = errors.New("chatasset: a GIF is approved by someone else than its author")
	ErrNotPublished = errors.New("chatasset: only an approved version is published")
	ErrImmutable    = errors.New("chatasset: only a draft is changed")
)

// FieldError is an invalid payload field (VALIDATION_FAILED with the field path, never the content).
type FieldError struct {
	Field  string
	Reason string
}

func (e FieldError) Error() string { return "chatasset: " + e.Field + ": " + e.Reason }

// Next returns the status after action on a version of status s by actor (author is the author of the version).
func Next(t Type, s Status, action string, actor, author uuid.UUID) (Status, error) {
	switch {
	case action == ActionSubmit && (s == StatusDraft || s == StatusRejected):
		return StatusInReview, nil
	case action == ActionApprove && s == StatusInReview:
		if t == TypeGifs && actor == author {
			return "", ErrSelfReview
		}
		return StatusApproved, nil
	case action == ActionReject && (s == StatusInReview || s == StatusApproved):
		return StatusRejected, nil
	case action == ActionArchive && s != StatusArchived:
		return StatusArchived, nil
	}
	return "", ErrTransition
}

var (
	keyRe  = regexp.MustCompile(`^media/[0-9a-f]{64}/[A-Za-z0-9._-]{1,200}$`)
	codeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	tagRe  = regexp.MustCompile(`^[a-z0-9 _-]{1,64}$`)
	// itemCodeRe is the item code of item.granted (RB-308, ADR-0100).
	itemCodeRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
)

// ValidTag reports a GIF tag (lowercase without diacritics, ≤ 64).
func ValidTag(t string) bool { return tagRe.MatchString(t) }

// ValidKey reports an immutable library key media/<sha256>/<file> (R31).
func ValidKey(k string) bool { return keyRe.MatchString(k) }

// KeyHash returns the sha256 part of a library key.
func KeyHash(k string) string {
	parts := strings.SplitN(k, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

// UnlockRule locks a pack until the student owns an item or got a reward (module game, F075); nil = for everyone. ItemID
// is the item code carried by item.granted (Clarifications F082 2026-11-26), RewardRuleID the published reward rule.
type UnlockRule struct {
	ItemID       *string    `json:"item_id,omitempty"`
	RewardRuleID *uuid.UUID `json:"reward_rule_id,omitempty"`
}

// Sticker is one sticker of a pack version.
type Sticker struct {
	StickerID   uuid.UUID `json:"sticker_id"`
	AssetKey    string    `json:"asset_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	AltVI       string    `json:"alt_text_vi"`
	AltEN       string    `json:"alt_text_en,omitempty"`
	SortOrder   int       `json:"sort_order"`
}

// Pack is the payload of a sticker pack version.
type Pack struct {
	Code       string      `json:"code"`
	NameVI     string      `json:"name_vi"`
	NameEN     string      `json:"name_en,omitempty"`
	UnlockRule *UnlockRule `json:"unlock_rule,omitempty"`
	HintVI     string      `json:"unlock_hint_vi,omitempty"`
	HintEN     string      `json:"unlock_hint_en,omitempty"`
	Stickers   []Sticker   `json:"stickers"`
}

// Gif is the payload of a GIF version.
type Gif struct {
	AssetKey   string   `json:"asset_key,omitempty"`
	PreviewKey string   `json:"preview_key,omitempty"`
	SizeBytes  int      `json:"size_bytes,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	TagsVI     []string `json:"tags_vi,omitempty"`
	TagsEN     []string `json:"tags_en,omitempty"`
}

// Limits are chat_asset_file_max (CONFIG_DEFAULTS).
type Limits struct {
	StickerMaxBytes int
	GifMaxBytes     int
	GifMaxPixels    int
}

// MaxStickers per pack (contract payload bound).
const MaxStickers = 64

func text(field, v string, maxLen int, required bool) error {
	if required && strings.TrimSpace(v) == "" {
		return FieldError{field, "required"}
	}
	if utf8.RuneCountInString(v) > maxLen {
		return FieldError{field, "too long"}
	}
	return nil
}

// Check validates a pack payload (complete: every sticker has its file key).
func (p Pack) Check() error {
	if !codeRe.MatchString(p.Code) {
		return FieldError{"payload.code", "^[a-z0-9][a-z0-9_-]{0,63}$"}
	}
	for _, c := range []struct {
		f, v string
		n    int
		req  bool
	}{{"payload.name_vi", p.NameVI, 128, true}, {"payload.name_en", p.NameEN, 128, false}, {"payload.unlock_hint_vi", p.HintVI, 1000, false},
		{"payload.unlock_hint_en", p.HintEN, 1000, false}} {
		if err := text(c.f, c.v, c.n, c.req); err != nil {
			return err
		}
	}
	if p.UnlockRule != nil {
		switch r := p.UnlockRule; {
		case (r.ItemID == nil) == (r.RewardRuleID == nil):
			return FieldError{"payload.unlock_rule", "exactly one of item_id or reward_rule_id"}
		case r.ItemID != nil && !itemCodeRe.MatchString(*r.ItemID):
			return FieldError{"payload.unlock_rule.item_id", "^[a-z0-9_]{1,64}$"}
		}
	}
	if len(p.Stickers) > MaxStickers {
		return FieldError{"payload.stickers", "too many"}
	}
	for _, s := range p.Stickers {
		if s.AssetKey != "" && !ValidKey(s.AssetKey) {
			return FieldError{"payload.stickers.asset_key", "media/<sha256>/<file>"}
		}
		if err := text("payload.stickers.alt_text_vi", s.AltVI, 1000, true); err != nil {
			return err
		}
		if err := text("payload.stickers.alt_text_en", s.AltEN, 1000, false); err != nil {
			return err
		}
	}
	return nil
}

// Check validates a GIF payload (tags lowercase without diacritics, ≤ 20 per locale).
func (g Gif) Check() error {
	for _, k := range []struct{ f, v string }{{"payload.asset_key", g.AssetKey}, {"payload.preview_key", g.PreviewKey}} {
		if k.v != "" && !ValidKey(k.v) {
			return FieldError{k.f, "media/<sha256>/<file>"}
		}
	}
	for _, tags := range [][]string{g.TagsVI, g.TagsEN} {
		if len(tags) > 20 {
			return FieldError{"payload.tags", "at most 20 per locale"}
		}
		for _, t := range tags {
			if !tagRe.MatchString(t) {
				return FieldError{"payload.tags", "^[a-z0-9 _-]{1,64}$"}
			}
		}
	}
	return nil
}

// File is what the inspector read of an uploaded object (never trusted from the client).
type File struct {
	Key         string
	SizeBytes   int
	ContentType string // sniffed from the bytes
	Width       int
	Height      int
	Animated    bool
	HashOK      bool // sha256 of the bytes equals the key
}

// Rejection reasons of a file (metric label, no content).
const (
	RejectFormat     = "format"
	RejectSize       = "size"
	RejectDimensions = "dimensions"
	RejectPreview    = "preview"
	RejectHash       = "hash"
	RejectMissing    = "missing"
)

// FileError is a file refused by the rules of chat_asset_file_max.
type FileError struct {
	Field  string
	Reason string
}

func (e FileError) Error() string { return "chatasset: file " + e.Field + ": " + e.Reason }

// CheckSticker applies the sticker rules (WebP/PNG ≤ 512 KB).
func CheckSticker(f File, l Limits) error {
	switch { // size first: an oversized object is read only up to the limit, its hash cannot match
	case f.SizeBytes <= 0 || f.SizeBytes > l.StickerMaxBytes:
		return FileError{"asset_key", RejectSize}
	case !f.HashOK:
		return FileError{"asset_key", RejectHash}
	case f.ContentType != "image/webp" && f.ContentType != "image/png":
		return FileError{"asset_key", RejectFormat}
	}
	return nil
}

// CheckGif applies the GIF rules (GIF ≤ 2 MB, ≤ 480 px; a static WebP/PNG preview ≤ 480 px).
func CheckGif(asset, preview File, l Limits) error {
	switch { // size first (see CheckSticker)
	case asset.SizeBytes <= 0 || asset.SizeBytes > l.GifMaxBytes:
		return FileError{"asset_key", RejectSize}
	case !asset.HashOK:
		return FileError{"asset_key", RejectHash}
	case asset.ContentType != "image/gif":
		return FileError{"asset_key", RejectFormat}
	case asset.Width <= 0 || asset.Height <= 0 || asset.Width > l.GifMaxPixels || asset.Height > l.GifMaxPixels:
		return FileError{"asset_key", RejectDimensions}
	case !preview.HashOK:
		return FileError{"preview_key", RejectHash}
	case preview.ContentType != "image/png" && preview.ContentType != "image/webp" || preview.Animated:
		return FileError{"preview_key", RejectPreview}
	case preview.Width > l.GifMaxPixels || preview.Height > l.GifMaxPixels:
		return FileError{"preview_key", RejectDimensions}
	}
	return nil
}
