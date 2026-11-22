package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Sender is the student sending a message: tenant and user from the verified principal (REST) or the gateway headers
// Ew-Tenant-Id / Ew-User-Id (WS, trusted only from the NATS user of binary realtime); ConnID is the WebSocket connection
// (empty for REST) that receives chat.message.acked / held.
type Sender struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	ConnID   string
}

// SendConfig is the configuration of SendMessage (chat_rate_per_user cap).
type SendConfig struct {
	RatePerUser int
}

// AuditChannelAccessDenied is recorded when a student targets a channel it is not a member of (no content).
const AuditChannelAccessDenied = "chat.channel.access_denied"

// Send is the single send use case of module chat (F115 US1, NFR-SAFE-001): the WS command ew.cmd.chat.message.send
// and REST sendChannelMessage both call it, every kind goes through the same checks and the ContentValidator, the seq
// is allocated only after them, and "Sent" (chat.message.acked) and the fan-out happen only after the JetStream ack
// (R25, ADR-0035). It writes nothing to PostgreSQL (chat-persister does).
type Send struct {
	Facts     service.Facts
	Validator port.ContentValidator
	Seq       port.SeqAllocator
	Stream    port.ChatStream
	Fanout    port.Fanout
	Recent    port.RecentBuffer
	Rate      port.RateLimiter
	Dedupe    port.SendDedupe
	Metrics   port.ChatMetrics // nil: none
	UoW       write.UnitOfWork // REST idempotency record and audit
	Audit     write.Auditor    // nil: none
	Config    SendConfig
	Now       func() time.Time
}

// SendResult is the outcome of a send: the envelope acked by the stream and the card of the sender.
type SendResult struct {
	Envelope message.Envelope
	Sender   read.ChatCard
	Replayed bool
}

func (h Send) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Send) rejected(code problem.Code) {
	if h.Metrics != nil {
		h.Metrics.Rejected(string(code))
	}
}

// temporarily is CHAT_TEMPORARILY_UNAVAILABLE (503 + Retry-After, WS retryable): nothing was delivered.
func temporarily() error {
	return RetryAfter{Err: problem.New(problem.ChatTemporarilyUnavailable, "").Retry(), After: time.Second}
}

// ErrNoValidator: no real ContentValidator is wired outside EW_ENV=test (F115-AC-06, R17).
var ErrNoValidator = errors.New("chat: no real ContentValidator wired (F116 pipeline.Validator); refusing to start outside EW_ENV=test")

// RequireValidator is the start-up check of module chat (NFR-SAFE-001, F115-AC-06): outside EW_ENV=test the send path
// needs a real ContentValidator, never nil nor a labelled test double.
func RequireValidator(env string, v port.ContentValidator) error {
	if env == "test" {
		if v == nil {
			return ErrNoValidator
		}
		return nil
	}
	if v == nil {
		return ErrNoValidator
	}
	if td, ok := v.(port.TestDouble); ok && td.TestDouble() {
		return ErrNoValidator
	}
	return nil
}

// Send runs the send path of req for s.
func (h Send) Send(ctx context.Context, s Sender, req message.Request) (res SendResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.SendMessage")
	defer func() { done(err) }()
	start := time.Now()
	if s.TenantID == uuid.Nil || s.UserID == uuid.Nil {
		return SendResult{}, problem.New(problem.Unauthenticated, "")
	}
	if req.ClientMessageID == uuid.Nil || req.ChannelID == uuid.Nil {
		return SendResult{}, problem.Invalid("client_message_id", problem.SubInvalid, "must be a UUID")
	}
	prev, created, err := h.Dedupe.Reserve(ctx, s.UserID, req.ClientMessageID)
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if !created {
		return h.replay(ctx, s, req, prev)
	}
	res, err = h.send(ctx, s, req)
	if err != nil && !errors.Is(err, errKeepReservation) {
		if rerr := h.Dedupe.Release(context.WithoutCancel(ctx), s.UserID, req.ClientMessageID); rerr != nil {
			slog.WarnContext(ctx, "chat: release of client_message_id failed", "error", rerr)
		}
	}
	if errors.Is(err, errKeepReservation) {
		err = temporarily()
	}
	if err == nil && h.Metrics != nil {
		h.Metrics.SendLatency(time.Since(start))
	}
	return res, err
}

// errKeepReservation: the envelope may be in the stream (no ack): the pending record keeps it so that a retry with the
// same client_message_id republishes the same message_id and seq (JetStream dedupes, F115-AC-04).
var errKeepReservation = errors.New("chat: publish not acknowledged")

// replay answers a client_message_id already seen: the same message_id and seq when it was acked, a republish of the
// same envelope when the previous attempt was not acked, CHAT_TEMPORARILY_UNAVAILABLE while it is in flight.
func (h Send) replay(ctx context.Context, s Sender, req message.Request, prev port.SendRecord) (SendResult, error) {
	if prev.Envelope == nil || prev.Envelope.ChannelID != req.ChannelID || prev.Envelope.TenantID != s.TenantID {
		return SendResult{}, temporarily()
	}
	e := *prev.Envelope
	card := h.card(ctx, s.TenantID, e)
	if prev.Done {
		return SendResult{Envelope: e, Sender: card, Replayed: true}, nil
	}
	if err := h.Stream.Publish(ctx, e); err != nil {
		if h.Metrics != nil {
			h.Metrics.PublishError()
		}
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	return h.acked(ctx, s, e, card, true)
}

func (h Send) card(ctx context.Context, tenant uuid.UUID, e message.Envelope) read.ChatCard {
	card := read.ChatCard{UserID: e.SenderID, Level: 1}
	if facts, err := h.Facts.Channel(ctx, tenant, e.ChannelID); err == nil && facts != nil {
		if c, ok := facts.Cards[e.SenderID]; ok {
			card = c
		}
	}
	return card
}

func (h Send) denied(ctx context.Context, s Sender, channel uuid.UUID) error {
	h.rejected(problem.NotFound)
	if h.Audit != nil && h.UoW != nil {
		// Best effort: the denial is answered even when the audit cannot be written (PostgreSQL down).
		err := h.UoW.InTenantTx(ctx, mustScope(s.TenantID), func(ctx context.Context) error {
			return h.Audit.Record(ctx, audit.Event{Action: AuditChannelAccessDenied, TargetType: "chat_channel",
				TargetID: channel.String(), Result: audit.ResultDenied})
		})
		if err != nil {
			slog.WarnContext(ctx, "chat: access denial not audited", "error", err)
		}
	}
	return problem.New(problem.NotFound, "")
}

func (h Send) send(ctx context.Context, s Sender, req message.Request) (SendResult, error) {
	now := h.now()
	facts, err := h.Facts.Channel(ctx, s.TenantID, req.ChannelID)
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if facts == nil {
		return SendResult{}, h.denied(ctx, s, req.ChannelID)
	}
	member, ok := facts.Members.Active(s.UserID)
	if !ok {
		return SendResult{}, h.denied(ctx, s, req.ChannelID)
	}
	ch := facts.Channel
	if !ch.Open() {
		h.rejected(problem.ChatChannelClosed)
		return SendResult{}, problem.New(problem.ChatChannelClosed, "")
	}
	school := member.SchoolID
	if school == nil {
		school = ch.SchoolID
	}
	// Curfew first: every channel, gameplay included, before any seq (ADR-0078, F115-AC-22).
	curfew, err := h.Facts.Curfew(ctx, s.TenantID, school)
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if active, reopen := curfew.Active(now); active {
		h.rejected(problem.CurfewActive)
		return SendResult{}, problem.New(problem.CurfewActive, "").With("reopen_at", reopen.UTC())
	}
	eff, err := h.Facts.Effective(ctx, s.TenantID, school, ch)
	switch {
	case errors.Is(err, policy.ErrDenied):
		h.rejected(problem.ChatNotAllowed)
		return SendResult{}, problem.New(problem.ChatNotAllowed, "")
	case err != nil:
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if !eff.Allows(policy.Kind(req.Kind)) {
		h.rejected(problem.ChatKindNotAllowed)
		return SendResult{}, problem.New(problem.ChatKindNotAllowed, "")
	}
	if err := req.Validate(eff.MaxTextLength); err != nil {
		h.rejected(problem.ValidationFailed)
		field := "body"
		if errors.Is(err, message.ErrMentions) {
			field = "mentions"
		}
		return SendResult{}, problem.Invalid(field, problem.SubInvalid, err.Error())
	}
	msgID := ids.New()
	perUser := h.Config.RatePerUser
	if eff.GlobalPerMinute > 0 && (perUser <= 0 || eff.GlobalPerMinute < perUser) {
		perUser = eff.GlobalPerMinute
	}
	ok, retry, err := h.Rate.Allow(ctx, s.UserID, ch.ID, msgID, port.RateLimits{PerChannel: eff.Rate.MessagesPerMinute, PerUser: perUser,
		Burst: eff.Rate.Burst, BurstWindow: port.BurstWindow, SlowMode: time.Duration(eff.SlowModeSeconds) * time.Second}, now)
	switch {
	case err != nil:
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	case !ok:
		h.rejected(problem.ChatRateLimited)
		return SendResult{}, RetryAfter{Err: problem.New(problem.ChatRateLimited, "").Retry(), After: retry}
	}
	// The pipeline: every kind, every channel (R15). An error fails closed.
	verdict, err := h.Validator.Validate(ctx, port.ValidationInput{TenantID: s.TenantID, SchoolID: school, ChannelID: ch.ID,
		ChannelType: string(ch.Type), SenderID: s.UserID, Kind: req.Kind, Text: req.Text(), QuickMessageID: req.QuickMessageID,
		Params: req.Params, Mentions: len(req.Mentions), Strictness: eff.Strictness})
	var rejected *problem.Error
	if errors.As(err, &rejected) { // CHAT_MUTED (technical gate), VALIDATION_FAILED (quick message)
		h.rejected(rejected.Code)
		return SendResult{}, rejected
	}
	if err != nil {
		slog.WarnContext(ctx, "chat: content validator failed", "error", err)
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	state, _, stored, err := verdict.Decision.Outcome()
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if !stored { // block: no seq, no publish, no content kept
		if h.Metrics != nil {
			h.Metrics.Sent(string(ch.Type), string(req.Kind), string(verdict.Decision))
		}
		h.rejected(problem.ChatContentBlocked)
		pe := problem.New(problem.ChatContentBlocked, "")
		if len(verdict.CategoryCodes) > 0 {
			pe = pe.With("category_code", verdict.CategoryCodes[0])
		}
		return SendResult{}, pe
	}
	seq, err := h.Seq.Next(ctx, ch.ID, func(ctx context.Context) (int64, error) { return h.Facts.SeqFloor(ctx, s.TenantID, ch.ID) })
	if err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	e := envelope(ch.TenantID, school, facts, s.UserID, req, msgID, seq, verdict, state, now)
	e.TraceID = traceID(ctx)
	if err := h.Dedupe.Save(ctx, s.UserID, req.ClientMessageID, port.SendRecord{Envelope: &e}); err != nil {
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, temporarily()
	}
	if err := h.Stream.Publish(ctx, e); err != nil {
		if h.Metrics != nil {
			h.Metrics.PublishError()
		}
		h.rejected(problem.ChatTemporarilyUnavailable)
		return SendResult{}, fmt.Errorf("%w: %w", errKeepReservation, err)
	}
	if h.Metrics != nil {
		h.Metrics.Sent(string(ch.Type), string(req.Kind), string(verdict.Decision))
	}
	card := facts.Cards[s.UserID]
	if card.UserID == uuid.Nil {
		card = read.ChatCard{UserID: s.UserID, Level: 1}
	}
	return h.acked(ctx, s, e, card, false)
}

// acked runs everything after the JetStream ack: the record becomes done, "Sent" goes to the sending connection, the
// envelope enters the recent buffer and the members get chat.message.created (allow/unchecked/mask) — or only the
// sender gets chat.message.held (hold_for_review/block_and_alert). Pub/Sub is best effort: a lost packet is recovered by
// chat.channel.sync (F115-AC-19).
func (h Send) acked(ctx context.Context, s Sender, e message.Envelope, card read.ChatCard, replayed bool) (SendResult, error) {
	bg := context.WithoutCancel(ctx)
	if err := h.Dedupe.Save(bg, s.UserID, *e.ClientMessageID, port.SendRecord{Done: true, Envelope: &e}); err != nil {
		slog.WarnContext(ctx, "chat: idempotency record not completed", "error", err)
	}
	if s.ConnID != "" {
		if err := h.Fanout.Acked(bg, s.UserID, s.ConnID, e); err != nil {
			slog.WarnContext(ctx, "chat: acked not pushed", "error", err)
		}
	}
	if err := h.Recent.Push(bg, e); err != nil {
		slog.WarnContext(ctx, "chat: recent buffer not updated", "error", err)
	}
	_, deliver, _, _ := e.Moderation.Decision.Outcome()
	if deliver {
		facts, err := h.Facts.Channel(bg, s.TenantID, e.ChannelID)
		if err == nil && facts != nil {
			recipients := make([]uuid.UUID, 0, len(facts.Members))
			for _, id := range facts.Members.ActiveIDs() {
				if !facts.Hidden(id, e.SenderID) { // block both ways, mute one way (F117-AC-12)
					recipients = append(recipients, id)
				}
			}
			if err := h.Fanout.Created(bg, e, card, recipients); err != nil {
				slog.WarnContext(ctx, "chat: fan-out failed", "error", err)
			}
		}
	} else if s.ConnID != "" {
		if err := h.Fanout.Held(bg, s.UserID, s.ConnID, e); err != nil {
			slog.WarnContext(ctx, "chat: held not pushed", "error", err)
		}
	}
	return SendResult{Envelope: e, Sender: card, Replayed: replayed}, nil
}

func envelope(tenant uuid.UUID, school *uuid.UUID, facts *read.ChannelFacts, sender uuid.UUID, req message.Request, id uuid.UUID,
	seq int64, v port.Verdict, state message.State, now time.Time) message.Envelope {
	ch := facts.Channel
	cmid := req.ClientMessageID
	e := message.Envelope{V: 1, Kind: message.EnvelopeKind, MessageID: id, TenantID: tenant, SchoolID: school, ChannelID: ch.ID,
		ChannelType: string(ch.Type), IsGameplay: ch.IsGameplay, Seq: seq, Revision: 1, SenderID: sender, ClientMessageID: &cmid,
		MsgKind: req.Kind, StickerID: req.StickerID, GifID: req.GifID, ReplyToMessageID: req.ReplyToMessageID,
		Mentions: append([]uuid.UUID{}, req.Mentions...), CreatedAt: now,
		Moderation: message.Moderation{Decision: v.Decision, State: state, CategoryCodes: v.CategoryCodes, StepsRun: v.StepsRun,
			StepsSkipped: v.StepsSkipped, CatalogVersion: v.CatalogVersion, CatalogVersionTenant: v.CatalogVersionTenant,
			SwitchSnapshotID: v.SwitchSnapshotID, LatencyMS: v.LatencyMS}}
	if req.Kind == message.KindQuick {
		q := req.QuickMessageID
		e.QuickMessageID = &q
		if len(req.Params) > 0 {
			e.Params = req.Params
		}
	}
	if text := req.Text(); text != "" {
		display := v.Display
		if display == "" {
			display = text
		}
		e.BodyDisplay = &display
		if display != text {
			orig := text
			e.BodyOriginal = &orig
		}
	}
	return e
}

// RecordResult writes the REST idempotency record (infra.idempotency_records) of sendChannelMessage with its response
// body; best effort: when PostgreSQL is down the client_message_id record still dedupes a retry (F115-AC-04).
func (h Send) RecordResult(ctx context.Context, tenant uuid.UUID, body any) {
	if _, ok := idempotency.PendingFrom(ctx); !ok || h.UoW == nil {
		return
	}
	err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error { return idempotency.SetResult(ctx, body) })
	if err != nil {
		slog.WarnContext(ctx, "chat: idempotency record not written", "error", err)
	}
}

// mustScope is the TenantScope of a verified tenant (principal or gateway header).
func mustScope(tenant uuid.UUID) tenancy.Scope {
	s, _ := authz.EventScope(tenant)
	return s
}

func traceID(ctx context.Context) *string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	id := sc.TraceID().String()
	return &id
}
