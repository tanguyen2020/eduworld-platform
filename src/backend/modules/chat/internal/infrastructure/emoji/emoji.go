// Package emoji implements port.EmojiSet from the embedded platform set (seed/emoji/emoji_sets.json).
package emoji

import (
	"encoding/json"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	seed "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/seed/emoji"
)

// Set implements port.EmojiSet.
type Set struct {
	emoji, reactions map[string]bool
}

var _ port.EmojiSet = Set{}

// Load parses the embedded set; every code must have the allowed format and every reaction must be an allowed emoji.
func Load() (Set, error) {
	var doc struct {
		Emoji     []string `json:"emoji"`
		Reactions []string `json:"reactions"`
	}
	if err := json.Unmarshal(seed.JSON, &doc); err != nil {
		return Set{}, err
	}
	s := Set{emoji: map[string]bool{}, reactions: map[string]bool{}}
	for _, c := range doc.Emoji {
		if !message.ValidEmojiCode(c) {
			return Set{}, errors.New("emoji set: invalid code")
		}
		s.emoji[c] = true
	}
	for _, c := range doc.Reactions {
		if !s.emoji[c] {
			return Set{}, errors.New("emoji set: a reaction is not an allowed emoji")
		}
		s.reactions[c] = true
	}
	if len(s.emoji) == 0 || len(s.reactions) == 0 {
		return Set{}, errors.New("emoji set: empty")
	}
	return s, nil
}

// Emoji implements port.EmojiSet.
func (s Set) Emoji(code string) bool { return s.emoji[code] }

// Reaction implements port.EmojiSet.
func (s Set) Reaction(code string) bool { return s.reactions[code] }
