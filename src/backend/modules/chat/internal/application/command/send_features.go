package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// prepare applies the F118 rules of a new message before the pipeline: the features of the channel type
// (CHAT_FEATURE_DISABLED, F118-AC-27), the mentions (CHAT_SYSTEM §7.7: limit → CHAT_MENTION_LIMIT, invalid ids dropped),
// the allowed emoji set (F118-AC-15) and the replied/quoted original (same channel, seen by the sender; the quote
// excerpt cut by the server from the display content — F118-AC-07, AC-08).
func (h Send) prepare(ctx context.Context, s Sender, facts *read.ChannelFacts, eff policy.Effective, req *message.Request) error {
	f := eff.Features
	switch req.Kind {
	case message.KindSticker:
		if !f.Sticker {
			return disabled(policy.FeatureSticker)
		}
	case message.KindGif:
		if !f.Gif {
			return disabled(policy.FeatureGif)
		}
	case message.KindEmoji:
		if h.Emoji == nil || !message.ValidEmojiCode(req.Emoji) || !h.Emoji.Emoji(req.Emoji) {
			return problem.Invalid("emoji", problem.SubInvalid, "not in the allowed emoji set")
		}
	}
	mentions, err := filterMentions(facts, eff, s.UserID, req.Mentions)
	if err != nil {
		return err
	}
	req.Mentions = mentions
	if req.ReplyToMessageID != nil {
		if !f.Reply {
			return disabled(policy.FeatureReply)
		}
		orig, err := h.original(ctx, s, facts, *req.ReplyToMessageID)
		if err != nil {
			return err
		}
		if orig == nil {
			return problem.Invalid("reply_to_message_id", problem.SubInvalid, "no such message in this channel")
		}
	}
	if req.Quote != nil {
		if !f.Quote {
			return disabled(policy.FeatureQuote)
		}
		orig, err := h.original(ctx, s, facts, req.Quote.MessageID)
		if err != nil {
			return err
		}
		if orig == nil || !orig.Quotable() {
			return problem.Invalid("quote", problem.SubInvalid, "no quotable message in this channel")
		}
		ex := message.Excerpt(*orig.BodyDisplay, req.Quote.Start, req.Quote.End, message.QuoteExcerptMax)
		if ex == "" {
			return problem.Invalid("quote", problem.SubInvalid, "empty excerpt")
		}
		req.Quote = &message.Quote{MessageID: orig.MessageID, Start: req.Quote.Start, End: req.Quote.End, Excerpt: ex}
	}
	return nil
}

// original returns the newest state of a message of the channel the sender sees, nil when none.
func (h Send) original(ctx context.Context, s Sender, facts *read.ChannelFacts, id uuid.UUID) (*message.Envelope, error) {
	if h.Lookup == nil || id == uuid.Nil {
		return nil, nil
	}
	found, err := h.Lookup.Message(ctx, s.TenantID, id)
	if err != nil {
		return nil, temporarily()
	}
	if found == nil {
		return nil, nil
	}
	e := found.Envelope
	if e.TenantID != s.TenantID || e.ChannelID != facts.Channel.ID || facts.Hidden(s.UserID, e.SenderID) ||
		(!e.Visible(s.UserID) && !e.Recalled()) {
		return nil, nil
	}
	return &e, nil
}

// asset checks the sticker/GIF of a message against the internal library (F118-AC-12, AC-13): only a published asset
// (a sticker of a pack assigned to the tenant and owned when locked) is sent; anything else is CHAT_ASSET_NOT_AVAILABLE.
func (h Send) asset(ctx context.Context, s Sender, req message.Request) error {
	var ok bool
	var err error
	switch req.Kind {
	case message.KindSticker:
		if h.Assets == nil {
			return problem.New(problem.ChatAssetNotAvailable, "")
		}
		ok, err = h.Assets.Sticker(ctx, s.TenantID, s.UserID, *req.StickerID)
	case message.KindGif:
		if h.Assets == nil {
			return problem.New(problem.ChatAssetNotAvailable, "")
		}
		ok, err = h.Assets.Gif(ctx, s.TenantID, *req.GifID)
	default:
		return nil
	}
	if err != nil {
		slog.WarnContext(ctx, "chat: asset library unavailable", "error", err)
		return temporarily()
	}
	if !ok {
		return problem.New(problem.ChatAssetNotAvailable, "")
	}
	return nil
}

// quoteVerdict runs the pipeline on the quote excerpt (derived content: the spam window of the sender is not counted
// again); a mask replaces the excerpt by its display, anything not delivered refuses the message.
func (h Send) quoteVerdict(ctx context.Context, s Sender, school *uuid.UUID, ch channel.Channel, eff policy.Effective, req *message.Request) error {
	v, err := h.Validator.Validate(ctx, port.ValidationInput{TenantID: s.TenantID, SchoolID: school, ChannelID: ch.ID,
		ChannelType: string(ch.Type), SenderID: s.UserID, Kind: message.KindText, Text: req.Quote.Excerpt, Strictness: eff.Strictness,
		Derived: true})
	var rejected *problem.Error
	if errors.As(err, &rejected) {
		h.rejected(rejected.Code)
		return rejected
	}
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return temporarily()
	}
	switch v.Decision {
	case message.Allow, message.Unchecked:
	case message.Mask:
		if v.Display != "" {
			q := *req.Quote
			q.Excerpt = v.Display
			req.Quote = &q
		}
	default:
		h.rejected(problem.ChatContentBlocked)
		return problem.New(problem.ChatContentBlocked, "")
	}
	return nil
}
