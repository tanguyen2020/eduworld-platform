// Package seedfiles reads the seed files of module game (src/backend/modules/game/seed): the world seed v1 of F071 —
// areas, story chapters, level packages and NPC dialogues — as authored payloads for command.PublishGameContent.
package seedfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
)

// World reads seed/world/{areas,levels,story_chapters}.v1.json (arrays) and seed/world/npc_dialogues/*.json (one dialogue
// per file) under dir (the seed directory of module game).
func World(dir string) ([]command.ContentItem, error) {
	root := filepath.Join(dir, "world")
	var out []command.ContentItem
	for _, f := range []struct{ file, typ string }{{"areas.v1.json", write.ItemWorldArea}, {"levels.v1.json", write.ItemLevel},
		{"story_chapters.v1.json", write.ItemStoryChapter}} {
		raw, err := os.ReadFile(filepath.Join(root, f.file)) //nolint:gosec // repository seed path
		if err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("%s: %w", f.file, err)
		}
		for _, it := range items {
			out = append(out, command.ContentItem{Type: f.typ, Payload: it})
		}
	}
	files, err := filepath.Glob(filepath.Join(root, "npc_dialogues", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f) //nolint:gosec // repository seed path
		if err != nil {
			return nil, err
		}
		out = append(out, command.ContentItem{Type: write.ItemNPCDialogue, Payload: raw})
	}
	return out, nil
}
