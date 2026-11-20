package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

type realValidator struct{ spyValidator }

// T013 / F115-AC-06: every kind goes through the ContentValidator (spy, TEST DOUBLE) before a seq is allocated; a block
// gets no seq and no publish; outside EW_ENV=test a missing or test-double validator refuses the start.
func TestSendPipelineRequired(t *testing.T) {
	h := newHarness(channel.Party)
	ctx := context.Background()
	s := command.Sender{TenantID: h.tenant, UserID: h.sender, ConnID: "c1"}
	kinds := []message.Kind{message.KindText, message.KindEmoji, message.KindQuick, message.KindSticker, message.KindGif}
	for _, k := range kinds {
		if _, err := h.send.Send(ctx, s, h.req(k)); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
	}
	if len(h.validator.calls) != len(kinds) || len(h.stream.sent) != len(kinds) {
		t.Fatalf("validator calls %d, published %d, want %d each", len(h.validator.calls), len(h.stream.sent), len(kinds))
	}
	for i, c := range h.validator.calls {
		if c.Kind != kinds[i] || c.ChannelType != "party" || c.SenderID != h.sender || c.TenantID != h.tenant {
			t.Fatalf("validator input %d: %+v", i, c)
		}
	}
	// block: no seq allocated (counter unchanged), no publish, CHAT_CONTENT_BLOCKED.
	before := h.seq.n[h.channel]
	h.validator.decision = message.Block
	_, err := h.send.Send(ctx, s, h.req(message.KindText))
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.ChatContentBlocked || h.seq.n[h.channel] != before || len(h.stream.sent) != len(kinds) {
		t.Fatalf("block: %v seq %d→%d published %d", err, before, h.seq.n[h.channel], len(h.stream.sent))
	}
	// mask: the display text is delivered, the original kept as evidence only in the envelope (body_original).
	h.validator.decision, h.validator.display = message.Mask, "x***"
	res, err := h.send.Send(ctx, s, h.req(message.KindText))
	if err != nil || res.Envelope.Moderation.State != message.StateMasked || *res.Envelope.BodyDisplay != "x***" ||
		res.Envelope.BodyOriginal == nil || len(h.fanout.created) != len(kinds)+1 {
		t.Fatalf("mask: %+v %v", res.Envelope, err)
	}
	// hold_for_review: acked, stored as held, never fanned out — only the sender gets chat.message.held.
	h.validator.decision = message.HoldForReview
	res, err = h.send.Send(ctx, s, h.req(message.KindText))
	if err != nil || res.Envelope.Moderation.State != message.StateHeld || len(h.fanout.created) != len(kinds)+1 || len(h.fanout.held) != 1 {
		t.Fatalf("held: %+v %v created %d held %d", res.Envelope, err, len(h.fanout.created), len(h.fanout.held))
	}
	// Start-up check (R17).
	if err := command.RequireValidator("production", nil); !errors.Is(err, command.ErrNoValidator) {
		t.Fatalf("production without validator: %v", err)
	}
	if err := command.RequireValidator("local", moderation.TestDoubleValidator{}); !errors.Is(err, command.ErrNoValidator) {
		t.Fatalf("local with the test double: %v", err)
	}
	if err := command.RequireValidator("test", moderation.For("test")); err != nil {
		t.Fatalf("test env: %v", err)
	}
	if moderation.For("staging") != nil {
		t.Fatal("a validator is wired outside test before F116")
	}
	var real port.ContentValidator = &realValidator{}
	if err := command.RequireValidator("production", real); err != nil {
		t.Fatalf("real validator refused: %v", err)
	}
}
