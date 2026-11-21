package world

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Chapter statuses (StoryChapterProgress.status): no story_progress row means locked or available by the rules.
const (
	ChapterLocked     = "locked"
	ChapterAvailable  = "available"
	ChapterInProgress = "in_progress"
	ChapterCompleted  = "completed"
)

// ChapterRule is StoryChapter.unlock_rule.
type ChapterRule struct {
	FirstEntry         bool   `json:"first_entry,omitempty"`
	AfterChapter       string `json:"after_chapter,omitempty"`
	MinSubjectChapters int    `json:"min_subject_chapters,omitempty"`
}

// Chapter is one version of a story chapter (payload of specs/schemas/story-chapter.schema.json).
type Chapter struct {
	SchemaVersion int         `json:"schema_version"`
	Code          string      `json:"chapter_code"`
	RegionCode    string      `json:"region_code"`
	Title         Name        `json:"title"`
	BossName      *Name       `json:"boss_name,omitempty"`
	Order         int         `json:"order"`
	SubjectCode   *string     `json:"subject_code,omitempty"`
	MainQuestIDs  []string    `json:"main_quest_ids"`
	BossID        *string     `json:"boss_id,omitempty"`
	Unlock        ChapterRule `json:"unlock_rule"`
}

// ParseChapter decodes and checks a chapter payload.
func ParseChapter(raw []byte) (Chapter, error) {
	var c Chapter
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, invalid("story chapter: %v", err)
	}
	switch {
	case c.SchemaVersion != 1:
		return c, invalid("story chapter: schema_version must be 1")
	case !codeRe.MatchString(c.Code) || !codeRe.MatchString(c.RegionCode):
		return c, invalid("story chapter %q: chapter_code/region_code", c.Code)
	case !c.Title.Complete() || (c.BossName != nil && !c.BossName.Complete()):
		return c, invalid("story chapter %s: title and boss_name need vi and en", c.Code)
	case c.Order < 0 || c.Order > 50 || len(c.MainQuestIDs) > 50:
		return c, invalid("story chapter %s: order 0..50, ≤ 50 main quests", c.Code)
	case c.SubjectCode != nil && !subjectRe.MatchString(*c.SubjectCode):
		return c, invalid("story chapter %s: subject_code", c.Code)
	case c.Unlock.AfterChapter != "" && !codeRe.MatchString(c.Unlock.AfterChapter):
		return c, invalid("story chapter %s: after_chapter", c.Code)
	case c.Unlock.MinSubjectChapters < 0 || c.Unlock.MinSubjectChapters > 10:
		return c, invalid("story chapter %s: min_subject_chapters", c.Code)
	case !c.Unlock.FirstEntry && c.Unlock.AfterChapter == "" && c.Unlock.MinSubjectChapters == 0:
		return c, invalid("story chapter %s: unlock_rule needs first_entry, after_chapter or min_subject_chapters", c.Code)
	}
	for i, q := range c.MainQuestIDs {
		if !uuidRe.MatchString(q) || slices.Contains(c.MainQuestIDs[:i], q) {
			return c, invalid("story chapter %s: main quest %q", c.Code, q)
		}
	}
	return c, nil
}

// Subject returns the subject of a subject chapter ("" for chapters 0, 5, 6).
func (c Chapter) Subject() string {
	if c.SubjectCode == nil {
		return ""
	}
	return *c.SubjectCode
}

// IsSubjectChapter reports a subject chapter (1–4): it counts for min_subject_chapters.
func (c Chapter) IsSubjectChapter() bool { return c.Subject() != "" }

// HasMainQuest reports whether quest is a main quest of the chapter.
func (c Chapter) HasMainQuest(quest string) bool { return slices.Contains(c.MainQuestIDs, quest) }

// Progress is the story_progress of a student in a chapter (nil = no row yet).
type Progress struct {
	Status          string
	CompletedQuests []string
}

// StoryState is what the chapter rules read: the chapters a student completed.
type StoryState struct {
	Completed       map[string]bool // chapter_code → completed
	SubjectChapters int             // subject chapters completed
}

// Status returns the status of chapter c for a student (F071-AC-21…AC-23): the stored progress when there is one;
// otherwise available when its rule is met (chapter 0 on first entry, subject chapters in parallel after chapter 0,
// chapters 5/6 after enough subject chapters), else locked.
func (c Chapter) Status(p *Progress, s StoryState) string {
	if p != nil && p.Status != "" && p.Status != ChapterAvailable {
		return p.Status
	}
	if c.Unlock.FirstEntry {
		return ChapterAvailable
	}
	if c.Unlock.AfterChapter != "" && !s.Completed[c.Unlock.AfterChapter] {
		return ChapterLocked
	}
	if c.Unlock.MinSubjectChapters > 0 && s.SubjectChapters < c.Unlock.MinSubjectChapters {
		return ChapterLocked
	}
	return ChapterAvailable
}

// RecordQuest applies a completed main quest to the progress of the chapter: the first main quest moves it to
// in_progress, the last one to completed (F071-AC-21). changed is false for a quest already counted or not a main
// quest of the chapter (a replayed event); completedNow is true only the first time the chapter completes.
func (c Chapter) RecordQuest(p *Progress, quest string) (next Progress, changed, completedNow bool) {
	if p != nil {
		next = Progress{Status: p.Status, CompletedQuests: append([]string{}, p.CompletedQuests...)}
	}
	if !c.HasMainQuest(quest) || slices.Contains(next.CompletedQuests, quest) || next.Status == ChapterCompleted {
		return next, false, false
	}
	next.CompletedQuests = append(next.CompletedQuests, quest)
	done := 0
	for _, q := range c.MainQuestIDs {
		if slices.Contains(next.CompletedQuests, q) {
			done++
		}
	}
	if done == len(c.MainQuestIDs) {
		next.Status = ChapterCompleted
		return next, true, true
	}
	next.Status = ChapterInProgress
	return next, true, false
}

// MainQuestsDone counts the main quests of the chapter in the progress.
func (c Chapter) MainQuestsDone(p *Progress) int {
	if p == nil {
		return 0
	}
	n := 0
	for _, q := range c.MainQuestIDs {
		if slices.Contains(p.CompletedQuests, q) {
			n++
		}
	}
	return n
}
