package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ProgressionPublishRules are the checks and the side table of the F074 content types on the release core (T032,
// F074-AC-21, AC-23): the JSON Schemas of progression-curve.schema.json and reward-rule.schema.json, the grant appliers
// (a reward type without applier is refused), the level table written when a curve version is published, and the
// current curve (Release 1.0: one curve, another curve_id is 409).
type ProgressionPublishRules struct {
	CurveSchema port.PayloadValidator
	RuleSchema  port.PayloadValidator
	Appliers    write.GrantApplierRegistry
	Levels      write.LevelTableStore
	Curves      port.Curves
}

// serverFields of the F074 payloads are set by the release core, never authored.
var (
	curveServerFields = []string{"curve_version_id", "version", "status"}
	ruleServerFields  = []string{"version", "status"}
)

func progressionItem(t string) bool {
	return t == write.ItemProgressionCurve || t == write.ItemRewardRule
}

// schemaProblem maps a JSON Schema failure to 422 VALIDATION_FAILED naming the first failing field (F074-AC-23).
func schemaProblem(err error) error {
	msg := err.Error()
	field := "payload"
	if i := strings.LastIndex(msg, ": /"); i >= 0 {
		if f := strings.SplitN(msg[i+3:], ",", 2)[0]; f != "" {
			field = f
		}
	}
	return problem.Invalid(field, problem.SubInvalid, msg)
}

func fieldProblem(err error) error {
	var cf *progression.FieldError
	if errors.As(err, &cf) {
		return problem.Invalid(cf.Field, problem.SubInvalid, cf.Reason)
	}
	var rf *reward.FieldError
	if errors.As(err, &rf) {
		return problem.Invalid(rf.Field, problem.SubInvalid, rf.Reason)
	}
	return problem.Invalid("payload", problem.SubInvalid, err.Error())
}

// parseProgression reads an authored progression curve or reward rule: JSON Schema of the stored form (placeholders for
// the server fields), then the domain checks.
func parseProgression(it ContentItem, rules *ProgressionPublishRules, now time.Time) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.CurveSchema == nil || rules.RuleSchema == nil || rules.Appliers == nil {
		return p, problem.New(problem.Internal, "progression publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubRequired, "a JSON object is required")
	}
	stored := map[string]any{}
	if it.Type == write.ItemProgressionCurve {
		for _, f := range curveServerFields {
			delete(p.doc, f)
		}
		for k, v := range p.doc {
			stored[k] = v
		}
		stored["curve_version_id"], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	} else {
		for _, f := range ruleServerFields {
			delete(p.doc, f)
		}
		for k, v := range p.doc {
			stored[k] = v
		}
		stored["version"], stored["status"] = 1, "published"
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	if it.Type == write.ItemProgressionCurve {
		if err := rules.CurveSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		c, err := progression.ParseCurve(raw)
		if err != nil {
			return p, fieldProblem(err)
		}
		if err := c.Validate(rules.Appliers.Has); err != nil {
			return p, fieldProblem(err)
		}
		p.curve, p.id = &c, c.CurveID
		p.parent = map[string]any{"code": c.Code}
		p.columns = map[string]any{"code": c.Code, "max_level": c.MaxLevel, "level_floor_protection": c.Protected()}
		return p, nil
	}
	if err := rules.RuleSchema.Validate(raw); err != nil {
		return p, schemaProblem(err)
	}
	r, err := reward.ParseRule(raw)
	if err != nil {
		return p, fieldProblem(err)
	}
	if err := r.Validate(rules.Appliers.Has); err != nil {
		return p, fieldProblem(err)
	}
	var window struct {
		ActiveFrom *time.Time `json:"active_from"`
		ActiveTo   *time.Time `json:"active_to"`
	}
	_ = json.Unmarshal(raw, &window)
	from := now
	if window.ActiveFrom != nil {
		from = window.ActiveFrom.UTC()
	}
	if window.ActiveTo != nil && !window.ActiveTo.After(from) {
		return p, problem.Invalid("active_to", problem.SubInvalid, "must be after active_from")
	}
	p.rule, p.id = &r, r.RuleID.String()
	p.parent = map[string]any{"code": r.Code}
	p.columns = map[string]any{"code": r.Code, "trigger_event_type": r.EventType, "source_code": r.SourceCode, "effective_from": from,
		"active_to": window.ActiveTo}
	return p, nil
}

// checkCurve refuses a second progression curve (Release 1.0: one platform curve, spec Assumptions [Confirmed]).
func (h PublishGameContent) checkCurve(ctx context.Context, items []parsedItem) error {
	var ids []string
	for _, p := range items {
		if p.curve != nil {
			ids = append(ids, p.curve.CurveID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids[1:] {
		if id != ids[0] {
			return problem.New(problem.Conflict, "one progression curve in Release 1.0")
		}
	}
	if h.Progression == nil || h.Progression.Curves == nil {
		return nil
	}
	cur, err := h.Progression.Curves.Current(ctx)
	if err != nil {
		return err
	}
	if cur != nil && cur.CurveID.String() != ids[0] {
		return problem.New(problem.Conflict, fmt.Sprintf("progression curve %s is the platform curve; another curve is not allowed", cur.CurveID))
	}
	return nil
}

// progressionStored adds the server fields to an authored F074 payload.
func (h PublishGameContent) progressionStored(ctx context.Context, p parsedItem, vid uuid.UUID, version int) (json.RawMessage, error) {
	doc := map[string]any{}
	for k, v := range p.doc {
		doc[k] = v
	}
	doc["version"], doc["status"] = version, "published"
	if p.curve == nil {
		return json.Marshal(doc)
	}
	doc["curve_version_id"] = vid.String()
	return json.Marshal(doc)
}

// insertLevels writes the level table of a published curve version (after its version row, same transaction).
func (h PublishGameContent) insertLevels(ctx context.Context, p parsedItem, vid uuid.UUID, version int) error {
	if h.Progression == nil || h.Progression.Levels == nil {
		return problem.New(problem.Internal, "progression level table store is not configured")
	}
	tb := p.curve.Table(version)
	rows := make([]write.LevelRow, 0, len(tb.Levels))
	for _, l := range tb.Levels {
		rewards := make([]map[string]any, 0, len(l.Rewards))
		for _, r := range l.Rewards {
			rewards = append(rewards, map[string]any(r))
		}
		rows = append(rows, write.LevelRow{Level: l.Level, XPRequired: l.XPRequired, Cumulative: l.Cumulative, TitleCode: l.TitleCode, Rewards: rewards})
	}
	return h.Progression.Levels.InsertLevels(ctx, vid, rows)
}
