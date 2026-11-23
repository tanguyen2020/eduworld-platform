package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/season"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LiveOpsPublishHook is the run side of a published platform event (command.LiveOps).
type LiveOpsPublishHook interface {
	EffectiveFrom(ctx context.Context, event uuid.UUID, d liveops.Definition, at time.Time) (time.Time, error)
	EventPublished(ctx context.Context, d liveops.Definition, version uuid.UUID, versionNo int, at time.Time) error
}

// LiveOpsPublishRules are the checks and the hook of the F077 content types on the release core (T017, T029, T033,
// F077-AC-20, AC-26…AC-28): the JSON Schemas of event, event-template and season-track, the bilingual rule, the healthy
// rules, the decoration budget and overlap, the rewards (published items, cosmetic tiers), then the run of a published
// event (scheduled run, re-pin at event_rule_change_effective).
type LiveOpsPublishRules struct {
	EventSchema    port.PayloadValidator
	TemplateSchema port.PayloadValidator
	TrackSchema    port.PayloadValidator
	Reader         port.LiveOpsReader
	Hook           LiveOpsPublishHook
	Policy         liveops.Policy
}

// liveOpsVersionField is the server id field of each F077 item type.
var liveOpsVersionField = map[string]string{write.ItemEvent: "event_version_id", write.ItemEventTemplate: "template_version_id",
	write.ItemSeasonTrack: "season_track_version_id"}

func liveOpsItem(t string) bool { _, ok := liveOpsVersionField[t]; return ok }

func liveOpsFieldProblem(err error) error {
	var sf *season.FieldError
	if errors.As(err, &sf) {
		return problem.Invalid(sf.Field, problem.SubInvalid, sf.Reason)
	}
	return liveopsProblem(err)
}

// parseLiveOps reads an authored event, template or season track: the bilingual rule first (TRANSLATION_INCOMPLETE), then
// the JSON Schema of the stored form, then the domain checks that need no database.
func parseLiveOps(it ContentItem, rules *LiveOpsPublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.EventSchema == nil || rules.TemplateSchema == nil || rules.TrackSchema == nil || rules.Reader == nil || rules.Hook == nil {
		return p, problem.New(problem.Internal, "live-ops publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubRequired, "a JSON object is required")
	}
	vf := liveOpsVersionField[it.Type]
	for _, f := range []string{vf, "version", "status"} {
		delete(p.doc, f)
	}
	stored := map[string]any{}
	for k, v := range p.doc {
		stored[k] = v
	}
	stored[vf], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	switch it.Type {
	case write.ItemEvent:
		d, err := liveops.Parse(raw)
		if err != nil {
			return p, liveOpsFieldProblem(err)
		}
		if d.OwnerScope != liveops.OwnerPlatform {
			return p, problem.Invalid("owner_scope", problem.SubInvalid, "the release core publishes platform events; school events come from a template")
		}
		if err := d.CheckTranslations(); err != nil {
			return p, liveOpsFieldProblem(err)
		}
		if err := rules.EventSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		p.event, p.id = &d, d.EventID.String()
		areas := []string{}
		if d.Components.Decoration != nil {
			areas = d.Components.Decoration.Areas
		}
		p.parent = map[string]any{"code": d.Code, "event_kind": d.Kind}
		p.columns = map[string]any{"event_kind": d.Kind, "start_at": d.Schedule.StartAt.UTC(), "end_at": d.Schedule.EndAt.UTC(),
			"decoration_areas": areas}
	case write.ItemEventTemplate:
		if err := rules.TemplateSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		t, err := liveops.ParseTemplate(raw)
		if err == nil {
			err = t.Validate()
		}
		if err != nil {
			return p, liveOpsFieldProblem(err)
		}
		p.template, p.id = &t, t.TemplateID.String()
		p.parent = map[string]any{"code": t.Code}
		p.columns = map[string]any{"code": t.Code, "event_kind": t.Kind, "allowed_params": nonNil(t.AllowedParams),
			"max_duration_days": t.MaxDurationDays}
	case write.ItemSeasonTrack:
		if err := rules.TrackSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		tr, err := season.Parse(raw)
		if err == nil {
			err = tr.Validate()
		}
		if err != nil {
			return p, liveOpsFieldProblem(err)
		}
		p.track, p.id = &tr, tr.SeasonTrackID.String()
		p.parent = map[string]any{"season_id": tr.SeasonID, "code": tr.Code}
		p.columns = map[string]any{"season_id": tr.SeasonID}
	}
	return p, nil
}

// checkLiveOps checks what needs the database (F077-AC-27, AC-28): the extra daily quests of an event, the overlap of its
// decoration with the current decoration of another event in one area, the items its rewards name (published; season
// tiers cosmetic only).
func (h PublishGameContent) checkLiveOps(ctx context.Context, items []parsedItem) error {
	var lookup func(string) *economy.Item
	itemsOf := func(codes []string) error {
		if len(codes) == 0 {
			return nil
		}
		if h.Economy == nil || h.Economy.Items == nil {
			return problem.New(problem.Internal, "economy publishing is not configured")
		}
		published, err := h.Economy.Items.Items(ctx, codes)
		if err != nil {
			return err
		}
		batch := map[string]economy.Item{}
		for _, p := range items {
			if p.item != nil {
				batch[p.item.ItemID] = *p.item
			}
		}
		lookup = func(code string) *economy.Item {
			if it, ok := batch[code]; ok {
				return &it
			}
			if d, ok := published[code]; ok {
				return &economy.Item{ItemID: d.ItemID, Kind: d.Kind, Rarity: d.Rarity}
			}
			return nil
		}
		return nil
	}
	for _, p := range items {
		switch {
		case p.event != nil:
			d := *p.event
			rec, err := h.LiveOps.Reader.QuestRecurrence(ctx, d.Components.QuestIDs)
			if err != nil {
				return err
			}
			daily := 0
			for i, q := range d.Components.QuestIDs {
				r, ok := rec[q]
				if !ok && !inBatchQuest(items, q) {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("components.quest_ids/%d is an unknown quest", i))
				}
				if r == "daily" {
					daily++
				}
			}
			if len(d.Components.ChallengeIDs) > 0 {
				// F076: the challenges of an event are published challenges (or in the batch).
				known, err := h.knownChallenges(ctx, items, d.Components.ChallengeIDs)
				if err != nil {
					return err
				}
				for i, c := range d.Components.ChallengeIDs {
					if !known[c] {
						return problem.New(problem.ItemRefInvalid, fmt.Sprintf("components.challenge_ids/%d is an unknown challenge", i))
					}
				}
			}
			if err := d.Validate(h.LiveOps.Policy, daily); err != nil {
				return liveOpsFieldProblem(err)
			}
			if dec := d.Components.Decoration; dec != nil && len(dec.Areas) > 0 {
				other, err := h.LiveOps.Reader.OverlappingDecorations(ctx, d.EventID, dec.Areas, d.Schedule.StartAt, d.Schedule.EndAt)
				if err != nil {
					return err
				}
				if len(other) > 0 {
					return problem.Invalid("components.decoration.areas", problem.SubInvalid, "an area already has an event decoration at that time")
				}
			}
			rewards := d.AllRewards()
			if err := itemsOf(liveops.RewardItemCodes(rewards)); err != nil {
				return err
			}
			if lookup != nil {
				if err := checkRewardItems(rewards, lookup); err != nil {
					return err
				}
			}
		case p.template != nil:
			var rewards []liveops.Reward
			for _, o := range p.template.RewardOptions {
				rewards = append(rewards, o.Rewards...)
			}
			for _, m := range p.template.CommunityGoal.Milestones {
				rewards = append(rewards, m.Rewards...)
			}
			if err := itemsOf(liveops.RewardItemCodes(rewards)); err != nil {
				return err
			}
			if lookup != nil {
				if err := checkRewardItems(rewards, lookup); err != nil {
					return err
				}
			}
		case p.track != nil:
			var rewards []liveops.Reward
			for _, t := range p.track.Tiers {
				rewards = append(rewards, t.Rewards...)
			}
			if err := itemsOf(liveops.RewardItemCodes(rewards)); err != nil {
				return err
			}
			if lookup == nil {
				continue
			}
			if err := checkRewardItems(rewards, lookup); err != nil {
				return err
			}
			for _, code := range liveops.RewardItemCodes(rewards) {
				if it := lookup(code); it != nil && !slices.Contains(season.CosmeticKinds, it.Kind) {
					return problem.Invalid("tiers", problem.SubInvalid, fmt.Sprintf("item %s is not cosmetic", code))
				}
			}
		}
	}
	return nil
}

func inBatchQuest(items []parsedItem, q uuid.UUID) bool {
	for _, p := range items {
		if p.quest != nil && p.quest.QuestID == q.String() {
			return true
		}
	}
	return false
}

// liveOpsStored adds the server fields to an authored F077 payload.
func liveOpsStored(p parsedItem, vid uuid.UUID, version int) ([]byte, error) {
	doc := map[string]any{}
	for k, v := range p.doc {
		doc[k] = v
	}
	doc[liveOpsVersionField[p.Type]], doc["version"], doc["status"] = vid.String(), version, "published"
	return json.Marshal(doc)
}
