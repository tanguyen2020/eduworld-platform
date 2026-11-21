package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrTranslationIncomplete is a dialogue text without both vi and en (422 TRANSLATION_INCOMPLETE, ADR-0056).
var ErrTranslationIncomplete = errors.New("world: translation incomplete")

// Server-assigned fields of a dialogue version: the authored payload never carries them and the content hash ignores them.
var dialogueServerFields = []string{"dialogue_version_id", "version", "status"}

// Dialogue is an NPC dialogue graph (specs/schemas/npc-dialogue.schema.json) as authored, decoded generically: the
// server checks the rules it relies on and serves the payload as published.
type Dialogue struct {
	DialogueID  string
	NPCID       string
	GradeLevels []int
	QuestRefs   []string // quest ids of actions and conditions (checked against the known quests)
	Doc         map[string]any
}

// ParseDialogue decodes an authored dialogue and checks it (F071-AC-18): ids, nodes, links, and vi + en on every node
// text and choice text.
func ParseDialogue(raw []byte) (Dialogue, error) {
	var d Dialogue
	if err := json.Unmarshal(raw, &d.Doc); err != nil || d.Doc == nil {
		return d, invalid("npc dialogue: %v", err)
	}
	for _, f := range dialogueServerFields {
		delete(d.Doc, f)
	}
	if v, _ := d.Doc["schema_version"].(float64); v != 1 {
		return d, invalid("npc dialogue: schema_version must be 1")
	}
	d.DialogueID, _ = d.Doc["dialogue_id"].(string)
	d.NPCID, _ = d.Doc["npc_id"].(string)
	if !uuidRe.MatchString(d.DialogueID) || !npcIDRe.MatchString(d.NPCID) {
		return d, invalid("npc dialogue: dialogue_id must be a UUID and npc_id npc.<code>")
	}
	if s, ok := d.Doc["owner_scope"]; ok && s != "platform" {
		return d, invalid("npc dialogue %s: owner_scope is platform", d.NPCID)
	}
	if gl, ok := d.Doc["grade_levels"].([]any); ok {
		for _, g := range gl {
			n, ok := g.(float64)
			if !ok || n < 1 || n > 9 || slices.Contains(d.GradeLevels, int(n)) {
				return d, invalid("npc dialogue %s: grade_levels", d.NPCID)
			}
			d.GradeLevels = append(d.GradeLevels, int(n))
		}
	}
	nodes, _ := d.Doc["nodes"].([]any)
	if len(nodes) == 0 || len(nodes) > 200 {
		return d, invalid("npc dialogue %s: 1..200 nodes", d.NPCID)
	}
	ids := map[string]bool{}
	for _, n := range nodes {
		node, _ := n.(map[string]any)
		id, _ := node["node_id"].(string)
		if id == "" || ids[id] {
			return d, invalid("npc dialogue %s: node_id %q missing or duplicated", d.NPCID, id)
		}
		ids[id] = true
	}
	start, _ := d.Doc["start_node_id"].(string)
	if !ids[start] {
		return d, invalid("npc dialogue %s: start_node_id %q is not a node", d.NPCID, start)
	}
	for _, n := range nodes {
		node := n.(map[string]any)
		id := node["node_id"].(string)
		if err := localized(node["text"]); err != nil {
			return d, fmt.Errorf("%w: npc dialogue %s node %s: %w", ErrTranslationIncomplete, d.NPCID, id, err)
		}
		if next, ok := node["next"].(string); ok && !ids[next] {
			return d, invalid("npc dialogue %s node %s: next %q is not a node", d.NPCID, id, next)
		}
		choices, _ := node["choices"].([]any)
		if len(choices) > 4 {
			return d, invalid("npc dialogue %s node %s: at most 4 choices", d.NPCID, id)
		}
		for i, c := range choices {
			choice, _ := c.(map[string]any)
			if err := localized(choice["text"]); err != nil {
				return d, fmt.Errorf("%w: npc dialogue %s node %s choice %d: %w", ErrTranslationIncomplete, d.NPCID, id, i, err)
			}
			if next, _ := choice["next"].(string); !ids[next] {
				return d, invalid("npc dialogue %s node %s choice %d: next %q is not a node", d.NPCID, id, i, next)
			}
			d.collect(choice)
		}
		d.collect(node)
	}
	if eps, ok := d.Doc["entry_points"].([]any); ok {
		for _, e := range eps {
			ep, _ := e.(map[string]any)
			if nid, _ := ep["node_id"].(string); !ids[nid] {
				return d, invalid("npc dialogue %s: entry point node %q is not a node", d.NPCID, nid)
			}
			d.collect(ep)
		}
	}
	return d, nil
}

func localized(v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return errors.New("text {vi, en} required")
	}
	for _, lang := range []string{"vi", "en"} {
		s, _ := m[lang].(string)
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("text.%s missing", lang)
		}
		if len([]rune(s)) > 500 {
			return fmt.Errorf("text.%s longer than 500", lang)
		}
	}
	return nil
}

// collect gathers the quest ids of the actions and quest_status conditions under v.
func (d *Dialogue) collect(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if k == "quest_id" {
				if s, ok := x.(string); ok && !slices.Contains(d.QuestRefs, s) {
					d.QuestRefs = append(d.QuestRefs, s)
				}
				continue
			}
			d.collect(x)
		}
	case []any:
		for _, x := range t {
			d.collect(x)
		}
	}
}

// AdmitsGrade reports whether the dialogue serves grade (no grade list = every grade).
func (d Dialogue) AdmitsGrade(grade int) bool {
	return len(d.GradeLevels) == 0 || slices.Contains(d.GradeLevels, grade)
}

// Overlaps reports two dialogues of the same NPC serving a common grade (one published dialogue per NPC and grade).
func Overlaps(a, b []int) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	for _, g := range a {
		if slices.Contains(b, g) {
			return true
		}
	}
	return false
}
