package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func code(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// T014 / F115-AC-07: a body over max_text_length of the effective policy or a send over the rate is refused before
// the seq (and before the pipeline); the limits run whatever the moderation switches say (they are not pipeline steps).
func TestTechnicalLimits(t *testing.T) {
	h := newHarness(channel.Party)
	ctx := context.Background()
	s := command.Sender{TenantID: h.tenant, UserID: h.sender}
	long := h.req(message.KindText)
	long.Body = strings.Repeat("a", 201) // party max_text_length 200
	if _, err := h.send.Send(ctx, s, long); code(err) != problem.ValidationFailed {
		t.Fatalf("too long: %v", err)
	}
	ok := h.req(message.KindText)
	ok.Body = strings.Repeat("a", 200)
	if _, err := h.send.Send(ctx, s, ok); err != nil {
		t.Fatalf("exactly max: %v", err)
	}
	for i := 1; i < 12; i++ { // party rate 12 per minute
		if _, err := h.send.Send(ctx, s, h.req(message.KindEmoji)); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	calls, published := len(h.validator.calls), len(h.stream.sent)
	_, err := h.send.Send(ctx, s, h.req(message.KindText))
	var ra command.RetryAfter
	if code(err) != problem.ChatRateLimited || !errors.As(err, &ra) || ra.After <= 0 {
		t.Fatalf("13th message: %v", err)
	}
	if len(h.validator.calls) != calls || len(h.stream.sent) != published || h.seq.n[h.channel] != int64(published) {
		t.Fatalf("a refused send reached the pipeline or got a seq: calls %d/%d published %d/%d seq %d",
			len(h.validator.calls), calls, len(h.stream.sent), published, h.seq.n[h.channel])
	}
	// Content not matching the kind is VALIDATION_FAILED, before any seq.
	bad := h.req(message.KindSticker)
	bad.StickerID = nil
	h2 := newHarness(channel.Class)
	if _, err := h2.send.Send(ctx, command.Sender{TenantID: h2.tenant, UserID: h2.sender}, func() message.Request {
		r := h2.req(message.KindSticker)
		r.StickerID = nil
		return r
	}()); code(err) != problem.ValidationFailed || h2.seq.allocated() != 0 {
		t.Fatalf("sticker without id: %v", err)
	}
	// NATS down: CHAT_TEMPORARILY_UNAVAILABLE (retryable), nothing fanned out (F115-AC-03).
	h3 := newHarness(channel.Party)
	h3.stream.fail = errBoom
	_, err = h3.send.Send(ctx, command.Sender{TenantID: h3.tenant, UserID: h3.sender}, h3.req(message.KindText))
	if code(err) != problem.ChatTemporarilyUnavailable || len(h3.fanout.created)+len(h3.fanout.acked) != 0 {
		t.Fatalf("stream down: %v created %d", err, len(h3.fanout.created))
	}
}
