// Package pipeline is the synchronous moderation pipeline of module chat (F116 US1): pipeline.Validator implements the
// ContentValidator port of the send path (F115). Order (F116-AC-01): technical gate (active sanctions; length, rate,
// burst, slow mode and channel rights are checked by command.Send before) → switches → normalize → format →
// sensitive terms → PII → spam → bullying → context → decision. No grade input (R15); no content in logs (R14); no
// matched term or position in what reaches the client (ADR-0030).
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CatalogProvider is the compiled catalogue store (service.Catalogs).
type CatalogProvider interface {
	Get(ctx context.Context, tenant uuid.UUID) (service.CatalogView, error)
}

// SwitchProvider resolves the effective switches (service.Switches); it never fails (all on without data).
type SwitchProvider interface {
	Effective(ctx context.Context, tenant uuid.UUID) (moderation.Effective, uuid.UUID)
}

// SanctionChecker returns the active sanctions of a sender (service.Sanctions).
type SanctionChecker interface {
	Active(ctx context.Context, tenant, user uuid.UUID, now time.Time) ([]port.Sanction, error)
}

// QuickCatalog returns an active quick message (service.QuickMessages).
type QuickCatalog interface {
	Get(ctx context.Context, id string) (*read.QuickMessageRow, error)
}

// Mute reasons of the system mutes (chat.sanctions.reason).
const (
	ReasonAutoSpam      = "auto_spam"
	ReasonBlockAndAlert = "block_and_alert_hold"
)

// SystemMuter applies a system mute (command.ApplySystemMute): duration nil = until the Safety Team reviews (F121).
type SystemMuter interface {
	Mute(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, user uuid.UUID, reason string) error
}

// Validator is the real ContentValidator (F116).
type Validator struct {
	Engine    *moderation.Engine
	Catalogs  CatalogProvider
	Switches  SwitchProvider
	Sanctions SanctionChecker
	Quick     QuickCatalog
	Spam      port.SpamWindow
	Muter     SystemMuter
	Decisions port.DecisionLog
	Metrics   port.ModerationMetrics // nil: none
	Now       func() time.Time
}

var _ port.ContentValidator = (*Validator)(nil)

func (v *Validator) now() time.Time {
	if v.Now != nil {
		return v.Now().UTC()
	}
	return time.Now().UTC()
}

var placeholder = regexp.MustCompile(`\{([a-z][a-z0-9_]{0,31})\}`)

// quickText checks a quick message and returns its parameters as the text to check (F116-AC-28).
func (v *Validator) quickText(ctx context.Context, in port.ValidationInput) (string, error) {
	q, err := v.Quick.Get(ctx, in.QuickMessageID)
	if err != nil {
		return "", err
	}
	if q == nil {
		return "", problem.Invalid("quick_message_id", problem.SubInvalid, "unknown or retired quick message")
	}
	want := map[string]bool{}
	for _, m := range placeholder.FindAllStringSubmatch(q.TextVI+" "+q.TextEN, -1) {
		want[m[1]] = true
	}
	keys := make([]string, 0, len(in.Params))
	for k := range in.Params {
		if !want[k] {
			return "", problem.Invalid("params", problem.SubInvalid, "parameter not used by the quick message")
		}
		keys = append(keys, k)
	}
	if len(keys) != len(want) {
		return "", problem.Invalid("params", problem.SubInvalid, "missing quick message parameter")
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = in.Params[k]
	}
	return strings.Join(parts, "\n"), nil
}

// Validate implements port.ContentValidator. A *problem.Error (CHAT_MUTED, VALIDATION_FAILED) refuses the message; any
// other error fails closed (CHAT_TEMPORARILY_UNAVAILABLE).
func (v *Validator) Validate(ctx context.Context, in port.ValidationInput) (verdict port.Verdict, err error) {
	ctx, done := observability.UseCase(ctx, "chat.pipeline.Validate")
	defer func() {
		var pe *problem.Error
		if errors.As(err, &pe) {
			done(nil)
			return
		}
		done(err)
	}()
	start := time.Now()
	now := v.now()
	var stages stageTimes
	mark := start
	// Technical gate: sanctions always apply, whatever the switches (F116-AC-08, F116-AC-24).
	sanctions, err := v.Sanctions.Active(ctx, in.TenantID, in.SenderID, now)
	if err != nil {
		return port.Verdict{}, err
	}
	for _, s := range sanctions {
		if code := s.Refusal(in.ChannelID, string(in.Kind), in.ChannelType, now); code != "" {
			return port.Verdict{}, problem.New(problem.Code(code), "")
		}
	}
	mark = stages.lap(StageSanctions, mark)
	text := in.Text
	if in.Kind == message.KindQuick {
		if text, err = v.quickText(ctx, in); err != nil {
			return port.Verdict{}, err
		}
	}
	sw, snapshot := v.Switches.Effective(ctx, in.TenantID)
	cat, err := v.Catalogs.Get(ctx, in.TenantID)
	if err != nil {
		return port.Verdict{}, err
	}
	mark = stages.lap(StageCatalog, mark)
	view := v.Engine.View(text)
	var spam *moderation.SpamCounts
	if sw.On(moderation.StepSpam) && !in.Derived && strings.TrimSpace(text) != "" {
		c, err := v.Spam.Record(ctx, in.SenderID, in.ChannelID, moderation.Fingerprint(view), now)
		if err != nil {
			return port.Verdict{}, fmt.Errorf("%w: spam window: %w", port.ErrUnavailable, err)
		}
		spam = &c
		mark = stages.lap(StageSpam, mark)
	}
	out := v.Engine.Evaluate(moderation.Input{View: view, Quick: in.Kind == message.KindQuick, Mentions: in.Mentions,
		Strictness: in.Strictness, Switches: sw, Categories: cat.Categories, Catalog: cat.Parts, Spam: spam})
	stages.lap(StageEvaluate, mark)
	latency := time.Since(start)
	pv, tv := cat.PlatformVersion, cat.TenantVersion
	verdict = port.Verdict{Decision: message.Decision(out.Decision), CategoryCodes: out.Codes, StepsRun: out.StepsRun,
		StepsSkipped: out.StepsSkipped, CatalogVersion: &pv, SwitchSnapshotID: &snapshot, LatencyMS: int(latency.Milliseconds())}
	if tv > 0 {
		verdict.CatalogVersionTenant = &tv
	}
	if in.Kind == message.KindText || in.Kind == message.KindEmoji {
		verdict.Display = out.Display
	}
	v.observe(in, out, latency, stages)
	switch verdict.Decision {
	case message.Block:
		v.blocked(ctx, in, verdict, now)
	case message.BlockAndAlert:
		// Temporary restriction of the sender until the Safety Team handles it (MODERATION §6.3, F116-AC-06).
		if err := v.Muter.Mute(context.WithoutCancel(ctx), in.TenantID, in.SchoolID, in.SenderID, ReasonBlockAndAlert); err != nil {
			slog.ErrorContext(ctx, "chat: system mute after block_and_alert failed", "error", err)
		}
	}
	return verdict, nil
}

// blocked writes the decision of a blocked message (it has no envelope) and applies the auto-mute after 3 blocks in
// 10 minutes (auto_mute_spam, F116-AC-07): the counter runs whatever the switches (sanction mechanism).
func (v *Validator) blocked(ctx context.Context, in port.ValidationInput, verdict port.Verdict, now time.Time) {
	bg := context.WithoutCancel(ctx)
	if v.Decisions != nil {
		v.Decisions.Record(bg, port.DecisionRecord{DecisionID: ids.New(), TenantID: in.TenantID, ChannelID: in.ChannelID,
			SenderID: in.SenderID, Decision: string(verdict.Decision), CategoryCodes: verdict.CategoryCodes, StepsRun: verdict.StepsRun,
			StepsSkipped: verdict.StepsSkipped, CatalogVersionPlatform: verdict.CatalogVersion, CatalogVersionTenant: verdict.CatalogVersionTenant,
			SwitchSnapshotID: verdict.SwitchSnapshotID, LatencyMS: verdict.LatencyMS, CreatedAt: now})
	}
	n, err := v.Spam.Block(bg, in.SenderID, now)
	if err != nil {
		slog.WarnContext(ctx, "chat: block counter unavailable", "error", err)
		return
	}
	if n >= moderation.SpamViolationCount {
		if err := v.Muter.Mute(bg, in.TenantID, in.SchoolID, in.SenderID, ReasonAutoSpam); err != nil {
			slog.ErrorContext(ctx, "chat: auto-mute failed", "error", err)
		}
	}
}

func (v *Validator) observe(in port.ValidationInput, out moderation.Outcome, latency time.Duration, stages stageTimes) {
	if v.Metrics == nil {
		return
	}
	for _, s := range stages {
		v.Metrics.FilterDuration(s.stage, s.d)
	}
	v.Metrics.FilterDuration(StageTotal, latency)
	for _, h := range out.Hits {
		v.Metrics.Match(h.CategoryCode, moderation.Bump(h.Action, in.Strictness))
	}
	if out.Decision == moderation.DecisionUnchecked {
		v.Metrics.Unchecked(in.TenantID)
	}
	if out.SlowPatterns > 0 {
		v.Metrics.RE2Timeouts(out.SlowPatterns)
	}
}

// Stages of eduworld_chat_filter_duration_seconds{stage} (MONITORING §3), in pipeline order: the technical gate (sanctions),
// the quick message, effective switches and catalogue (catalog), the spam window when the step runs (spam, with the text
// normalisation), the content steps (evaluate) and the whole synchronous pipeline (total, moderation_pipeline_latency).
const (
	StageSanctions = "sanctions"
	StageCatalog   = "catalog"
	StageSpam      = "spam"
	StageEvaluate  = "evaluate"
	StageTotal     = "total"
)

type stageTime struct {
	stage string
	d     time.Duration
}

type stageTimes []stageTime

// lap records the time since mark for stage and returns the new mark.
func (s *stageTimes) lap(stage string, mark time.Time) time.Time {
	now := time.Now()
	*s = append(*s, stageTime{stage: stage, d: now.Sub(mark)})
	return now
}
