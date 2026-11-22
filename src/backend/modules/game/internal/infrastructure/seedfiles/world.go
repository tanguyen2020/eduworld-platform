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

// Quests reads seed/quests/*.v1.json (one quest per file, quest.schema.json v2) except assignments.v1.json.
func Quests(dir string) ([]command.ContentItem, error) {
	files, err := filepath.Glob(filepath.Join(dir, "quests", "*.v1.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []command.ContentItem
	for _, f := range files {
		if filepath.Base(f) == "assignments.v1.json" {
			continue
		}
		raw, err := os.ReadFile(f) //nolint:gosec // repository seed path
		if err != nil {
			return nil, err
		}
		out = append(out, command.ContentItem{Type: write.ItemQuest, Payload: raw})
	}
	return out, nil
}

// QuestAssignment is one entry of seed/quests/assignments.v1.json: the tenant codes a seed quest is assigned to.
type QuestAssignment struct {
	QuestID string   `json:"quest_id"`
	Tenants []string `json:"tenants"`
}

// QuestAssignments reads seed/quests/assignments.v1.json.
func QuestAssignments(dir string) ([]QuestAssignment, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "quests", "assignments.v1.json")) //nolint:gosec // repository seed path
	if err != nil {
		return nil, err
	}
	var doc struct {
		Assignments []QuestAssignment `json:"assignments"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("assignments.v1.json: %w", err)
	}
	return doc.Assignments, nil
}
