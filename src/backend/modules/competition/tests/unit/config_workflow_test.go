package unit

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

// F108 T009 — F108-AC-05: the review workflow (reviewer ≠ author, reject needs a note), published versions never editable,
// effective_from never in the past, rollback only to another version published before, unpublish only where the runtime
// allows it, with a reason.

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestConfigWorkflowTransitions(t *testing.T) {
	author, other := uuid.New(), uuid.New()
	v := gameconfig.Version{Status: gameconfig.StatusDraft, Author: &author}
	to, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionSubmitReview, Actor: author})
	if err != nil || to != gameconfig.StatusInReview {
		t.Fatalf("submit: %v %v", to, err)
	}
	v.Status = gameconfig.StatusInReview
	if _, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionApprove, Actor: author}); !errors.Is(err, gameconfig.ErrSelfReview) {
		t.Fatalf("self review: %v", err)
	}
	if _, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionReject, Actor: other}); !errors.Is(err, gameconfig.ErrNoteRequired) {
		t.Fatalf("reject without note: %v", err)
	}
	if to, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionReject, Actor: other, Note: "tier"}); err != nil || to != gameconfig.StatusRejected {
		t.Fatalf("reject: %v %v", to, err)
	}
	if to, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionApprove, Actor: other}); err != nil || to != gameconfig.StatusApproved {
		t.Fatalf("approve: %v %v", to, err)
	}
	if _, err := gameconfig.Transition(gameconfig.Version{Status: gameconfig.StatusPublished, IsCurrent: true},
		gameconfig.Request{Action: gameconfig.ActionArchive, Actor: other}); !errors.Is(err, gameconfig.ErrArchiveCurrent) {
		t.Fatalf("archive current: %v", err)
	}
	if _, err := gameconfig.Transition(v, gameconfig.Request{Action: gameconfig.ActionApprove, Actor: other, Note: strings.Repeat("x", 2001)}); !errors.Is(err, gameconfig.ErrNoteTooLong) {
		t.Fatalf("long note: %v", err)
	}
	for _, s := range []gameconfig.Status{gameconfig.StatusPublished, gameconfig.StatusInReview, gameconfig.StatusApproved, gameconfig.StatusArchived} {
		if gameconfig.Editable(s) {
			t.Fatalf("%s editable", s)
		}
	}
	if !gameconfig.Editable(gameconfig.StatusDraft) || !gameconfig.Editable(gameconfig.StatusRejected) {
		t.Fatal("draft/rejected not editable")
	}
}

func TestConfigEffectiveFrom(t *testing.T) {
	now := mustTime(t, "2026-11-25T10:00:00Z")
	if at, err := gameconfig.EffectiveFrom(nil, now); err != nil || !at.Equal(now) {
		t.Fatalf("default: %v %v", at, err)
	}
	past := now.Add(-time.Minute)
	if _, err := gameconfig.EffectiveFrom(&past, now); !errors.Is(err, gameconfig.ErrRetroactive) {
		t.Fatalf("past: %v", err)
	}
	monday := mustTime(t, "2026-11-30T00:00:00+07:00")
	if at, err := gameconfig.EffectiveFrom(&monday, now); err != nil || !at.Equal(monday) {
		t.Fatalf("future: %v %v", at, err)
	}
	if err := gameconfig.CheckPublish(gameconfig.StatusInReview); !errors.Is(err, gameconfig.ErrNotApproved) {
		t.Fatalf("publish not approved: %v", err)
	}
}

func TestConfigRollbackAndUnpublish(t *testing.T) {
	cur, v1 := uuid.New(), uuid.New()
	ok := &gameconfig.Target{ID: v1, Status: gameconfig.StatusPublished, Published: true}
	if err := gameconfig.CheckRollback(&cur, ok, ""); !errors.Is(err, gameconfig.ErrReasonRequired) {
		t.Fatalf("no reason: %v", err)
	}
	if err := gameconfig.CheckRollback(&cur, ok, "v3 lỗi"); err != nil {
		t.Fatalf("valid rollback: %v", err)
	}
	for _, bad := range []*gameconfig.Target{nil, {ID: v1, Status: gameconfig.StatusApproved}, {ID: cur, Status: gameconfig.StatusPublished, Published: true},
		{ID: v1, Status: gameconfig.StatusArchived, Published: true}} {
		if err := gameconfig.CheckRollback(&cur, bad, "lý do"); !errors.Is(err, gameconfig.ErrRollbackTarget) {
			t.Fatalf("target %+v: %v", bad, err)
		}
	}
	mm, _ := gameconfig.Lookup(gameconfig.TypeMatchmaking)
	tpl, _ := gameconfig.Lookup(gameconfig.TypeChallengeTp)
	if err := gameconfig.CheckUnpublish(mm, &cur, "lý do"); !errors.Is(err, gameconfig.ErrUnpublishNotAllowed) {
		t.Fatalf("unpublish matchmaking: %v", err)
	}
	if err := gameconfig.CheckUnpublish(tpl, nil, "lý do"); !errors.Is(err, gameconfig.ErrNotPublished) {
		t.Fatalf("unpublish without release: %v", err)
	}
	if err := gameconfig.CheckUnpublish(tpl, &cur, "lý do"); err != nil {
		t.Fatalf("unpublish template: %v", err)
	}
	if _, err := gameconfig.Lookup("quests"); !errors.Is(err, gameconfig.ErrUnknownType) {
		t.Fatal("a game type is served by competition")
	}
	if len(gameconfig.Names()) != 7 {
		t.Fatalf("types: %v", gameconfig.Names())
	}
}

func TestConfigDiff(t *testing.T) {
	a := map[string]any{"code": "a", "x": 1.0, "list": []any{1.0, 2.0}, "gone": true}
	b := map[string]any{"code": "b", "x": 2.0, "list": []any{1.0}, "new": "v"}
	ch, tr := gameconfig.Diff(a, b)
	if tr || len(ch) != 4 {
		t.Fatalf("diff: %+v", ch)
	}
	for _, c := range ch {
		if c.Path == "/code" {
			t.Fatal("the server-owned code is diffed")
		}
	}
}
