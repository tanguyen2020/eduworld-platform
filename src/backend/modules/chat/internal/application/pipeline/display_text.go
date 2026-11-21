package pipeline

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// CheckDisplayText is moderateDisplayText of the chat facade (party and team names typed by students, MODERATION §1,
// R15): the same normalisation, catalogue (platform + tenant, category settings, effective switches; PII and grooming
// always) and context as chat messages, without the spam windows (a name is not a message). Only a clean result
// (allow, or unchecked while every content step is switched off) is allowed; mask and anything heavier is blocked —
// a name is never shown partly hidden. Nothing about the match leaves this method (R15); nothing is logged with the text
// (R14). An error (catalogue not loaded) fails closed: the caller keeps its generated default name.
func (v *Validator) CheckDisplayText(ctx context.Context, tenant uuid.UUID, text string) (allowed bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.pipeline.CheckDisplayText")
	defer func() { done(err) }()
	if strings.TrimSpace(text) == "" {
		return false, nil
	}
	sw, _ := v.Switches.Effective(ctx, tenant)
	cat, err := v.Catalogs.Get(ctx, tenant)
	if err != nil {
		return false, err
	}
	out := v.Engine.Evaluate(moderation.Input{View: v.Engine.View(text), Quick: true, Strictness: moderation.StrictnessStrict,
		Switches: sw, Categories: cat.Categories, Catalog: cat.Parts})
	if v.Metrics != nil {
		for _, h := range out.Hits {
			v.Metrics.Match(h.CategoryCode, h.Action)
		}
	}
	return out.Decision == moderation.DecisionAllow || out.Decision == moderation.DecisionUnchecked, nil
}
