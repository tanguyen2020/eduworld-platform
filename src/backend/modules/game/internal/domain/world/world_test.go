package world

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func area(t *testing.T, class, subject string, grades []int, rule *Rule) Area {
	t.Helper()
	var s *string
	if subject != "" {
		s = &subject
	}
	return Area{SchemaVersion: 1, AreaID: "a", Class: class, SubjectCode: s, Access: AccessPolicy{RoomType: RoomHub, GradeLevels: grades}, UnlockRule: rule,
		Layout: Layout{SpawnPoints: []SpawnPoint{{ID: "a.default"}}, Interactables: []Interactable{
			{ID: "d1", Kind: KindDiscovery}, {ID: "d2", Kind: KindDiscovery}, {ID: "c1", Kind: KindCollectible}, {ID: "n", Kind: KindNPC, NPCID: "npc.x"}}}}
}

// F071-AC-02, AC-03 (domain): a learning area of the grade and subject is open at level 1 with no coins; a disabled
// subject is hidden; another grade is locked; a gated area opens by unlock or by its rule.
func TestDecide(t *testing.T) {
	lvl := Rule{MinLevel: &LevelCond{Level: 10}}
	for g := 1; g <= 9; g++ {
		if d := Decide(area(t, ClassLearning, "math", []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, nil), Student{Grade: g}, false, Facts{Level: 1}); !d.Visible || !d.Accessible {
			t.Fatalf("grade %d: learning area must be open: %+v", g, d)
		}
	}
	if d := Decide(area(t, ClassLearning, "history", []int{1, 2}, nil), Student{Grade: 1, EnabledSubjects: []string{"math"}}, false, Facts{}); d.Visible {
		t.Fatal("disabled subject must be hidden")
	}
	if d := Decide(area(t, ClassLearning, "history", []int{4, 5}, nil), Student{Grade: 3}, false, Facts{}); !d.Visible || d.Accessible || d.LockReason != LockGradeNotAllowed {
		t.Fatalf("grade not allowed: %+v", d)
	}
	if d := Decide(area(t, ClassQuest, "", nil, &lvl), Student{Grade: 3}, false, Facts{Level: 2}); d.Accessible || d.LockReason != LockRequirementsNotMet || len(Missing(d.Requirements)) != 1 {
		t.Fatalf("locked by rule: %+v", d)
	}
	if d := Decide(area(t, ClassQuest, "", nil, &lvl), Student{Grade: 3}, true, Facts{}); !d.Accessible {
		t.Fatal("an unlock opens the area")
	}
	if d := Decide(area(t, ClassQuest, "", nil, &lvl), Student{Grade: 3}, false, Facts{Level: 10}); !d.Accessible || !d.RuleMet {
		t.Fatal("a met rule opens the area")
	}
}

// F071-AC-09 (domain): the partition never mixes two schools; same school and band share it.
func TestInstanceKey(t *testing.T) {
	tenant, a, b := uuid.New(), uuid.New(), uuid.New()
	k := func(s uuid.UUID, band string) uuid.UUID {
		return InstanceKey{TenantID: tenant, SchoolID: s, AreaID: "main_hub", GradeBand: band}.RoomID()
	}
	if k(a, "g1_3") == k(b, "g1_3") || k(a, "g1_3") == k(a, "g4_5") || k(a, "g1_3") != (InstanceKey{TenantID: tenant, SchoolID: a, AreaID: "main_hub", GradeBand: "g1_3"}).RoomID() {
		t.Fatal("partition room ids")
	}
}

// F071-AC-24 (domain) and the edge case "a new area version drops a found anchor": % against the current version, ≤ 100.
func TestExploration(t *testing.T) {
	a := area(t, ClassQuest, "", nil, nil)
	var e Exploration
	if !e.Record("d1", KindDiscovery) || e.Record("d1", KindDiscovery) {
		t.Fatal("record once")
	}
	if c := e.Completion(a); c.Pct != 33.33 || c.DiscoveriesFound != 1 || c.DiscoveriesTotal != 2 || c.CollectiblesTotal != 1 {
		t.Fatalf("completion: %+v", c)
	}
	e.Record("d2", KindDiscovery)
	e.Record("c1", KindCollectible)
	e.Record("gone", KindDiscovery)
	if c := e.Completion(a); c.Pct != 100 {
		t.Fatalf("completion capped: %+v", c)
	}
}

// F071-AC-21, AC-22 (domain): chapter 0 on first entry, subject chapters in parallel after chapter 0, chapter 5/6 by
// the number of subject chapters; first main quest → in_progress, last → completed once.
func TestChapterRules(t *testing.T) {
	math, q1, q2 := "math", uuid.NewString(), uuid.NewString()
	c0 := Chapter{Code: "c0", Unlock: ChapterRule{FirstEntry: true}}
	c1 := Chapter{Code: "c1", SubjectCode: &math, MainQuestIDs: []string{q1, q2}, Unlock: ChapterRule{AfterChapter: "c0"}}
	c5 := Chapter{Code: "c5", Unlock: ChapterRule{MinSubjectChapters: 1}}
	none := StoryState{Completed: map[string]bool{}}
	if c0.Status(nil, none) != ChapterAvailable || c1.Status(nil, none) != ChapterLocked || c5.Status(nil, none) != ChapterLocked {
		t.Fatal("initial statuses")
	}
	after0 := StoryState{Completed: map[string]bool{"c0": true}}
	if c1.Status(nil, after0) != ChapterAvailable {
		t.Fatal("subject chapter after chapter 0")
	}
	p, changed, done := c1.RecordQuest(nil, q1)
	if !changed || done || p.Status != ChapterInProgress {
		t.Fatalf("first quest: %+v", p)
	}
	if _, changed, _ := c1.RecordQuest(&p, q1); changed {
		t.Fatal("replayed quest")
	}
	p2, changed, done := c1.RecordQuest(&p, q2)
	if !changed || !done || p2.Status != ChapterCompleted || c1.MainQuestsDone(&p2) != 2 {
		t.Fatalf("last quest: %+v", p2)
	}
	if _, changed, done := c1.RecordQuest(&p2, uuid.NewString()); changed || done {
		t.Fatal("completed chapter")
	}
	if c5.Status(nil, StoryState{SubjectChapters: 1}) != ChapterAvailable {
		t.Fatal("chapter 5 after one subject chapter")
	}
}

// F071-AC-18 (domain): every node and choice text needs vi and en; links must exist.
func TestParseDialogue(t *testing.T) {
	ok := `{"schema_version":1,"dialogue_id":"0193a5f4-6c1e-7b2a-9d1f-000000071301","npc_id":"npc.mr_numbers","grade_levels":[1,2,3],"start_node_id":"hello",
		"nodes":[{"node_id":"hello","speaker":"npc","text":{"vi":"Chào","en":"Hi"},"choices":[{"text":{"vi":"Nhận","en":"Take"},"next":"bye",
		"actions":[{"type":"give_quest","quest_id":"0193a5f4-6c1e-7b2a-9d1f-000000071101"}]}]},{"node_id":"bye","speaker":"npc","text":{"vi":"Tạm biệt","en":"Bye"}}]}`
	d, err := ParseDialogue([]byte(ok))
	if err != nil || len(d.QuestRefs) != 1 || !d.AdmitsGrade(3) || d.AdmitsGrade(7) {
		t.Fatalf("valid dialogue: %v %+v", err, d)
	}
	missingEN := `{"schema_version":1,"dialogue_id":"0193a5f4-6c1e-7b2a-9d1f-000000071301","npc_id":"npc.mr_numbers","start_node_id":"hello",
		"nodes":[{"node_id":"hello","speaker":"npc","text":{"vi":"Chào"}}]}`
	if _, err := ParseDialogue([]byte(missingEN)); !errors.Is(err, ErrTranslationIncomplete) {
		t.Fatalf("missing en: %v", err)
	}
	badLink := `{"schema_version":1,"dialogue_id":"0193a5f4-6c1e-7b2a-9d1f-000000071301","npc_id":"npc.mr_numbers","start_node_id":"hello",
		"nodes":[{"node_id":"hello","speaker":"npc","text":{"vi":"Chào","en":"Hi"},"next":"nowhere"}]}`
	if _, err := ParseDialogue([]byte(badLink)); !IsInvalid(err) {
		t.Fatalf("bad link: %v", err)
	}
	if !Overlaps([]int{1, 2}, []int{2, 3}) || Overlaps([]int{1, 2}, []int{6, 7}) || !Overlaps(nil, []int{1}) {
		t.Fatal("overlaps")
	}
}
