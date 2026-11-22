package unit

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// F118-AC-01 … AC-04, AC-08, AC-18: the message rules of the domain (server clock, sender only, text only, excerpt cut
// from the display content, mentions filtered and limited).
func TestMessageFeatureRules(t *testing.T) {
	sender, other := uuid.New(), uuid.New()
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	d := "xin chao ca nha"
	e := message.Envelope{SenderID: sender, MsgKind: message.KindText, BodyDisplay: &d, Revision: 1, CreatedAt: now,
		Moderation: message.Moderation{State: message.StateAllowed}}
	if err := e.CanEdit(sender, now.Add(14*time.Minute), 15*time.Minute); err != nil {
		t.Fatalf("edit within the window: %v", err)
	}
	for name, err := range map[string]error{
		"other":   e.CanEdit(other, now, 15*time.Minute),
		"late":    e.CanEdit(sender, now.Add(16*time.Minute), 15*time.Minute),
		"recall":  e.CanRecall(sender, now.Add(25*time.Hour), 24*time.Hour),
		"recall2": e.WithRecall(sender, now).CanRecall(sender, now, 24*time.Hour),
	} {
		if err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
	r := e.WithRecall(sender, now)
	if r.BodyDisplay != nil || r.Revision != 2 || !r.Recalled() || r.Forwardable() == nil || r.Quotable() {
		t.Fatalf("recalled copy: %+v", r)
	}
	ed := e.Edited("chao", nil, nil, message.Moderation{State: message.StateAllowed}, now)
	if ed.Revision != 2 || ed.EditCount != 1 || *ed.BodyDisplay != "chao" || ed.EditedAt == nil {
		t.Fatalf("edited: %+v", ed)
	}
	if ex := message.Excerpt("toi o *** nhe", 0, 100, 200); ex != "toi o *** nhe" {
		t.Fatalf("excerpt: %q", ex)
	}
	if ex := message.Excerpt(string(make([]rune, 300)), 0, 0, message.QuoteExcerptMax); len([]rune(ex)) > message.QuoteExcerptMax {
		t.Fatal("excerpt over 200")
	}
	member := func(u uuid.UUID) bool { return u != uuid.Nil }
	blocked := func(a, b uuid.UUID) bool { return a == other }
	if _, err := message.FilterMentions(make([]uuid.UUID, 6), sender, 5, member, blocked); !errors.Is(err, message.ErrMentionsLimit) {
		t.Fatalf("6 mentions: %v", err)
	}
	third := uuid.New()
	got, err := message.FilterMentions([]uuid.UUID{other, sender, third, third}, sender, 5, member, blocked)
	if err != nil || len(got) != 1 || got[0] != third {
		t.Fatalf("filtered: %v %v", got, err)
	}
	// Always-on features of a gameplay channel (F118-AC-27).
	f := policy.ApplyAlwaysOn(policy.Effective{ChannelType: "party"}).Features
	if !f.Edit || !f.Recall || !f.Reply || !f.Reaction || !f.Typing || f.ReceiptMode() != policy.ReceiptsPerMember || f.Pin {
		t.Fatalf("always-on features: %+v", f)
	}
	if (policy.Features{MaxPins: 9}).PinsMax() != policy.CeilingPins {
		t.Fatal("pins ceiling")
	}
}

// F118-AC-17: the workflow (GIF approved by someone else than its author) and the file rules of chat_asset_file_max.
func TestChatAssetRules(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if _, err := chatasset.Next(chatasset.TypeGifs, chatasset.StatusInReview, chatasset.ActionApprove, a, a); !errors.Is(err, chatasset.ErrSelfReview) {
		t.Fatalf("self review: %v", err)
	}
	if s, err := chatasset.Next(chatasset.TypeGifs, chatasset.StatusInReview, chatasset.ActionApprove, b, a); err != nil || s != chatasset.StatusApproved {
		t.Fatalf("approve: %v %v", s, err)
	}
	if _, err := chatasset.Next(chatasset.TypeStickerPacks, chatasset.StatusDraft, chatasset.ActionApprove, b, a); err == nil {
		t.Fatal("approve a draft")
	}
	l := chatasset.Limits{StickerMaxBytes: 512 * 1024, GifMaxBytes: 2 << 20, GifMaxPixels: 480}
	ok := chatasset.File{SizeBytes: 1000, ContentType: "image/gif", Width: 320, Height: 240, Animated: true, HashOK: true}
	prev := chatasset.File{SizeBytes: 100, ContentType: "image/png", Width: 320, Height: 240, HashOK: true}
	if err := chatasset.CheckGif(ok, prev, l); err != nil {
		t.Fatalf("valid gif: %v", err)
	}
	wide := ok
	wide.Width = 481
	animPrev := prev
	animPrev.ContentType, animPrev.Animated = "image/webp", true
	big := ok
	big.SizeBytes = 3 << 20
	for name, err := range map[string]error{"wide": chatasset.CheckGif(wide, prev, l), "animated preview": chatasset.CheckGif(ok, animPrev, l),
		"big": chatasset.CheckGif(big, prev, l), "sticker jpeg": chatasset.CheckSticker(chatasset.File{SizeBytes: 10, ContentType: "image/jpeg",
			HashOK: true}, l), "sticker hash": chatasset.CheckSticker(chatasset.File{SizeBytes: 10, ContentType: "image/png"}, l)} {
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if !chatasset.ValidKey("media/"+strings.Repeat("ab", 32)+"/a.png") || chatasset.ValidKey("media/x/a.png") || chatasset.ValidKey("https://giphy.example/a.gif") {
		t.Fatal("immutable library keys only (R31)")
	}
}
