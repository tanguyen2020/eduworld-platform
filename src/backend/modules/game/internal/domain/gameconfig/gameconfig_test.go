package gameconfig

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// F078 T014: every transition of the workflow, reviewer ≠ author, archive of the current release (F078-AC-08).
func TestWorkflowTransitions(t *testing.T) {
	author, other := uuid.New(), uuid.New()
	cases := []struct {
		name    string
		from    Status
		current bool
		req     Request
		want    Status
		err     error
	}{
		{"submit draft", StatusDraft, false, Request{Action: ActionSubmitReview, Actor: author}, StatusInReview, nil},
		{"submit rejected", StatusRejected, false, Request{Action: ActionSubmitReview, Actor: author}, StatusInReview, nil},
		{"submit approved", StatusApproved, false, Request{Action: ActionSubmitReview, Actor: author}, StatusApproved, ErrInvalidTransition},
		{"approve by other", StatusInReview, false, Request{Action: ActionApprove, Actor: other}, StatusApproved, nil},
		{"approve by author", StatusInReview, false, Request{Action: ActionApprove, Actor: author}, StatusInReview, ErrSelfReview},
		{"reject by author", StatusInReview, false, Request{Action: ActionReject, Actor: author, Note: "x"}, StatusInReview, ErrSelfReview},
		{"reject without note", StatusInReview, false, Request{Action: ActionReject, Actor: other, Note: "  "}, StatusInReview, ErrNoteRequired},
		{"reject", StatusInReview, false, Request{Action: ActionReject, Actor: other, Note: "thiếu bản dịch"}, StatusRejected, nil},
		{"approve draft", StatusDraft, false, Request{Action: ActionApprove, Actor: other}, StatusDraft, ErrInvalidTransition},
		{"archive published", StatusPublished, false, Request{Action: ActionArchive, Actor: author}, StatusArchived, nil},
		{"archive current", StatusPublished, true, Request{Action: ActionArchive, Actor: author}, StatusPublished, ErrArchiveCurrent},
		{"archive in review", StatusInReview, false, Request{Action: ActionArchive, Actor: author}, StatusInReview, ErrInvalidTransition},
		{"archive archived", StatusArchived, false, Request{Action: ActionArchive, Actor: author}, StatusArchived, ErrInvalidTransition},
		{"publish is not a transition", StatusApproved, false, Request{Action: "publish", Actor: author}, StatusApproved, ErrUnknownAction},
		{"note too long", StatusInReview, false, Request{Action: ActionReject, Actor: other, Note: strings.Repeat("ă", 2001)}, StatusInReview, ErrNoteTooLong},
	}
	for _, c := range cases {
		got, err := Transition(Version{Status: c.from, Author: &author, IsCurrent: c.current}, c.req)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%s: got %s %v, want %s %v", c.name, got, err, c.want, c.err)
		}
	}
	// A seed version (system actor, no author) can be reviewed by anyone.
	if got, err := Transition(Version{Status: StatusInReview}, Request{Action: ActionApprove, Actor: author}); got != StatusApproved || err != nil {
		t.Fatalf("approve seed version: %s %v", got, err)
	}
	if !Editable(StatusDraft) || !Editable(StatusRejected) || Editable(StatusInReview) || Editable(StatusPublished) || Editable(StatusApproved) {
		t.Fatal("editable statuses")
	}
}

// F078-AC-03: only the types of module game are served; competition and chat types are unknown here.
func TestLookupTypes(t *testing.T) {
	for _, n := range []string{"quests", "minigame-configs", "npc-dialogues", "world-areas", "story-chapters", "items", "shop-items", "collections",
		"achievements", "reward-rules", "progression-curves", "events", "event-templates", "season-tracks", "challenges"} {
		if _, err := Lookup(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	for _, n := range []string{"sticker-packs", "gifs", "matchmaking-configs", "seasons", "progression-levels", "currencies", "nope"} {
		if _, err := Lookup(n); !errors.Is(err, ErrUnknownType) {
			t.Errorf("%s served", n)
		}
	}
	q, _ := Lookup("quests")
	doc := map[string]any{"quest_id": "client", "version": 9, "status": "published"}
	q.Normalize(doc, "k", "v", 3, StatusDraft)
	if doc["quest_id"] != "k" || doc["quest_version_id"] != "v" || doc["version"] != 3 || doc["status"] != "draft" || doc["owner_scope"] != "platform" {
		t.Fatalf("normalize %+v", doc)
	}
	a, _ := Lookup("world-areas")
	area := map[string]any{"area_id": "main_hub"}
	a.Normalize(area, "ignored", "v", 1, StatusDraft)
	if area["area_id"] != "main_hub" || a.Key(area) != "main_hub" || len(area) != 1 {
		t.Fatalf("code keyed type %+v", area)
	}
	if it, ok := ByItemType("minigame_config"); !ok || it.Name != "minigame-configs" {
		t.Fatal("by item type")
	}
	if it, ok := ByAssignable("event_template"); !ok || it.Name != "event-templates" {
		t.Fatal("by assignable")
	}
	if _, ok := ByAssignable("collection"); ok {
		t.Fatal("collections are not assignable")
	}
}

// F078-AC-12, AC-13, AC-14: release rules.
func TestReleaseRules(t *testing.T) {
	now := time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)
	if err := CheckPublish(StatusApproved); err != nil {
		t.Fatal(err)
	}
	if err := CheckPublish(StatusInReview); !errors.Is(err, ErrNotApproved) {
		t.Fatal("publish in review")
	}
	if err := CheckSchedule(now.Add(4*time.Minute), now, 5*time.Minute); !errors.Is(err, ErrScheduleTooSoon) {
		t.Fatal("lead too short")
	}
	if err := CheckSchedule(now.Add(5*time.Minute), now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	q, _ := Lookup("quests")
	v1, v2 := uuid.New(), uuid.New()
	if err := CheckRollback(q, &v2, Target{ID: v1, Status: StatusPublished}, "lỗi bước 3"); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		target Target
		reason string
		err    error
	}{
		"current":   {Target{ID: v2, Status: StatusPublished}, "x", ErrRollbackTarget},
		"draft":     {Target{ID: v1, Status: StatusDraft}, "x", ErrRollbackTarget},
		"archived":  {Target{ID: v1, Status: StatusArchived}, "x", ErrRollbackTarget},
		"no reason": {Target{ID: v1, Status: StatusPublished}, " ", ErrReasonRequired},
		"long":      {Target{ID: v1, Status: StatusPublished}, strings.Repeat("x", 501), ErrReasonTooLong},
	} {
		if err := CheckRollback(q, &v2, c.target, c.reason); !errors.Is(err, c.err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	ev, _ := Lookup("events")
	if err := CheckRollback(ev, &v2, Target{ID: v1, Status: StatusPublished}, "x"); !errors.Is(err, ErrRollbackNotAllowed) {
		t.Fatal("event rollback")
	}
	if err := CheckUnpublish(q, nil, "x"); !errors.Is(err, ErrNotPublished) {
		t.Fatal("unpublish without pointer")
	}
	if err := CheckUnpublish(q, &v2, "x"); err != nil {
		t.Fatal(err)
	}
	if s, err := Cancel(ScheduleScheduled); s != ScheduleCancelled || err != nil {
		t.Fatal("cancel")
	}
	if _, err := Cancel(ScheduleApplied); !errors.Is(err, ErrNotScheduled) {
		t.Fatal("cancel applied")
	}
	if !Due(ScheduleScheduled, now, now) || Due(ScheduleScheduled, now.Add(time.Second), now) || Due(ScheduleCancelled, now, now) {
		t.Fatal("due")
	}
}

// F078-AC-09: the diff lists the changes by JSON pointer and skips the server fields.
func TestDiff(t *testing.T) {
	q, _ := Lookup("quests")
	a := map[string]any{"version": 1.0, "quest_version_id": "a", "name": map[string]any{"vi": "Chợ", "en": "Market"},
		"steps": []any{map[string]any{"id": "s1"}, map[string]any{"id": "s2"}}, "a/b": 1.0, "gone": true}
	b := map[string]any{"version": 2.0, "quest_version_id": "b", "name": map[string]any{"vi": "Chợ phiên", "en": "Market"},
		"steps": []any{map[string]any{"id": "s1"}}, "a/b": 2.0, "new": "x"}
	got, trunc := Diff(q, a, b)
	if trunc {
		t.Fatal("truncated")
	}
	want := map[string]string{"/a~1b": "changed", "/gone": "removed", "/name/vi": "changed", "/new": "added", "/steps/1": "removed"}
	if len(got) != len(want) {
		t.Fatalf("changes %+v", got)
	}
	for _, c := range got {
		if want[c.Path] != c.Op {
			t.Errorf("unexpected %+v", c)
		}
	}
	big := map[string]any{}
	for i := 0; i < MaxChanges+5; i++ {
		big["k"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+uuid.NewString()] = i
	}
	if got, trunc := Diff(q, map[string]any{}, big); !trunc || len(got) != MaxChanges {
		t.Fatalf("cap %d %v", len(got), trunc)
	}
}

// F078-AC-07: references against the client manifest, media refs and missing translations.
func TestRefsAndTranslations(t *testing.T) {
	asset := uuid.NewString()
	other := uuid.NewString()
	doc := map[string]any{"area_id": "learning_zone.math", "giver_npc_id": "npc_owl", "scene_ref": "res://scenes/zone.tscn",
		"steps":      []any{map[string]any{"target_area_id": "main_hub", "interactable_id": "lantern_1"}},
		"asset_refs": map[string]any{"model": "asset:" + asset}, "banner_asset_id": other, "name": map[string]any{"vi": "Chợ", "en": ""},
		"title": map[string]any{"vi": "A", "en": "B"}, "desc": map[string]any{"vi": "C"}}
	r := CollectRefs(doc)
	if strings.Join(r.Areas, ",") != "learning_zone.math,main_hub" || strings.Join(r.NPCs, ",") != "npc_owl" ||
		len(r.Scenes) != 1 || strings.Join(r.Interactables, ",") != "lantern_1" || len(r.Assets) != 2 {
		t.Fatalf("refs %+v", r)
	}
	m := ClientManifest{Areas: []string{"main_hub"}, NPCs: []string{"npc_owl"}, Scenes: []string{"res://scenes/zone.tscn"}}
	missing := m.Missing(r)
	if strings.Join(missing, ",") != "area:learning_zone.math,interactable:lantern_1" {
		t.Fatalf("missing %v", missing)
	}
	tr := MissingTranslations(doc, []string{"vi", "en"})
	if strings.Join(tr, ",") != "/desc,/name" {
		t.Fatalf("translations %v", tr)
	}
	if len(MissingTranslations(doc, []string{"vi"})) != 0 {
		t.Fatal("vi only")
	}
}

// F078-AC-17: the preview grant reads one version, by its creator, before it expires.
func TestPreviewSession(t *testing.T) {
	now := time.Now()
	p, cfg := uuid.New(), uuid.New()
	s := PreviewSession{Type: "quests", ConfigID: cfg, Version: 3, CreatedBy: p, ExpiresAt: now.Add(PreviewTTL)}
	if !s.Allows(p, "quests", cfg, 3, now) {
		t.Fatal("own version")
	}
	if s.Allows(p, "quests", cfg, 2, now) || s.Allows(uuid.New(), "quests", cfg, 3, now) || s.Allows(p, "items", cfg, 3, now) ||
		s.Allows(p, "quests", uuid.New(), 3, now) || s.Allows(p, "quests", cfg, 3, now.Add(PreviewTTL)) {
		t.Fatal("other grants")
	}
	if h := HashToken("abc"); len(h) != 64 || h == "abc" {
		t.Fatal("hash")
	}
}

// F078-AC-18 (415/413), AC-20 (backoff, dead letter).
func TestMediaAndRender(t *testing.T) {
	sha := strings.Repeat("a", 64)
	ok := Upload{Kind: MediaModel3D, MIME: "model/gltf-binary", Size: 50 << 20, SHA256: sha, Filename: "bear.glb", License: "CC0"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		u   Upload
		err error
	}{
		"too large":  {Upload{Kind: MediaModel3D, MIME: "model/gltf-binary", Size: 60 << 20, SHA256: sha, Filename: "x", License: "y"}, ErrMediaTooLarge},
		"wrong type": {Upload{Kind: MediaImage, MIME: "image/gif", Size: 1, SHA256: sha, Filename: "x", License: "y"}, ErrMediaType},
		"video":      {Upload{Kind: "video", MIME: "video/mp4", Size: 1, SHA256: sha, Filename: "x", License: "y"}, ErrMediaKind},
		"render":     {Upload{Kind: MediaRenderThumbnail, MIME: "image/png", Size: 1, SHA256: sha, Filename: "x", License: "y"}, ErrMediaKind},
		"bad sha":    {Upload{Kind: MediaImage, MIME: "image/png", Size: 1, SHA256: "x", Filename: "x", License: "y"}, ErrMediaInput},
	} {
		if err := c.u.Validate(); !errors.Is(err, c.err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if Extension(MediaModel3D, "model/gltf-binary") != "glb" || Extension(MediaImage, "x/y") != "bin" {
		t.Fatal("extension")
	}
	now := time.Now()
	st, at := RenderFailure(1, 3, time.Minute, now)
	if st != RenderFailed || at == nil || at.Sub(now) != time.Minute {
		t.Fatal("first retry")
	}
	if st, at = RenderFailure(2, 3, time.Minute, now); st != RenderFailed || at.Sub(now) != 2*time.Minute {
		t.Fatal("second retry")
	}
	if st, at = RenderFailure(3, 3, time.Minute, now); st != RenderDead || at != nil {
		t.Fatal("dead")
	}
}
