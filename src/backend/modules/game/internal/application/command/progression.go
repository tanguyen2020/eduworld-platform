package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Progression events (specs/events/payloads/game/*.v1).
const (
	EventXPAwarded     = "xp.awarded"
	EventLevelReached  = "level.reached"
	EventRewardGranted = "reward.granted"
)

// ErrStudentNotProjected: the event names a student the projection ref.students does not hold in the tenant of the
// envelope (not projected yet, or of another tenant). Retryable: the consumer retries, then dead-letters (F074-AC-11).
var ErrStudentNotProjected = errors.New("progression: student not in ref.students of the event tenant")

// capZone is the calendar of the daily XP cap (Asia/Ho_Chi_Minh, UTC+7, no DST — spec Assumptions [Confirmed]).
var capZone = time.FixedZone("ICT", 7*3600)

// CapDay is the calendar day (ICT) of the daily cap counters at t.
func CapDay(t time.Time) string { return t.In(capZone).Format(time.DateOnly) }

// ProgressionMetrics observes the Reward Engine; nil = not observed.
type ProgressionMetrics interface {
	XPAwarded(sourceCode string, capped bool, amount int)
	LevelReached(n int)
	TenantMismatch(consumer string)
	EngineDuration(seconds float64)
	LedgerMismatch()
}

// Progression is the write side of the game progression (F074): the Reward Engine (AwardFromEvent), the level-up
// with its rewards, the curve rollout, the grade transition, the XP reconciliation and the merge of two ledgers. XP
// only comes from verified server events (R5, ADR-0016); every posting, its cache update and its outbox events are
// one transaction (ADR-0049, F074-AC-03); realtime pushes happen after commit.
type Progression struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.ProgressionStore
	Scan        write.ProgressionScan
	Profiles    write.ProfileRepository
	Students    port.StudentDirectory
	Rules       port.RewardRules
	Curves      port.Curves
	Appliers    write.GrantApplierRegistry
	Events      write.EventPublisher
	Audit       write.Auditor
	Realtime    port.ProgressionRealtime // nil: no push
	Quests      GradeQuestExpirer        // nil: no quest expires on a grade transition
	Metrics     ProgressionMetrics
	Policy      reward.Policy
	Now         func() time.Time
}

func (h Progression) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Progression) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

func (h Progression) metric(fn func(m ProgressionMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

// Key is the hex SHA-256 idempotency key of parts (xp_ledger.idempotency_key, reward_grants.idempotency_key).
func Key(parts ...string) string {
	s := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(s[:])
}

// levelUpGrantID is the trigger_event_id of a level-up reward: UUIDv5 (curve_id, level, student_id).
func levelUpGrantID(curve uuid.UUID, level int, student uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(curve, []byte("level:"+itoa(level)+":"+student.String()))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// pushes collects the realtime messages of one transaction.
type pushes struct {
	xp     map[string]any
	levels []map[string]any
}

// AwardFromEvent is the Reward Engine on one verified server event (consumers game.reward_engine and
// game.reward_engine_game, F074-AC-01…AC-08, AC-11, AC-27). It runs in the consumer transaction (TenantScope of the
// envelope): it checks the student against ref.students of the tenant, locks the student, evaluates the rules in effect
// at the event time, appends the ledger rows (a replayed source key adds nothing, F074-AC-02), moves the cap counters,
// the xp_total/level cache, the level history and the level rewards, and writes xp.awarded, level.reached and
// reward.granted to the outbox — all or nothing (F074-AC-03).
func (h Progression) AwardFromEvent(ctx context.Context, tenant, studentID uuid.UUID, t reward.Trigger) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AwardFromEvent")
	defer func() { done(err) }()
	start := h.now()
	st, err := h.Students.Student(ctx, tenant, studentID)
	if err != nil {
		return err
	}
	if st == nil {
		h.metric(func(m ProgressionMetrics) { m.TenantMismatch("game.reward_engine") })
		return ErrStudentNotProjected
	}
	if st.Deleted || st.Status != "active" {
		return nil // an inactive or cleaned-up student earns nothing; the event is consumed
	}
	if len(t.SourceCodes) == 0 {
		return nil
	}
	rules, err := h.Rules.RulesFor(ctx, t.EventType, t.OccurredAt)
	if err != nil || len(rules) == 0 {
		return err
	}
	if err := h.Store.LockStudent(ctx, studentID); err != nil {
		return err
	}
	state, err := h.ensureProgress(ctx, tenant, studentID, st.SchoolID)
	if err != nil {
		return err
	}
	now := h.now()
	day := CapDay(now)
	buckets := []string{reward.TotalBucket}
	for _, r := range rules {
		if b := r.Bucket(t.SourceType); !contains(buckets, b) {
			buckets = append(buckets, b)
		}
	}
	granted, err := h.Store.LockCaps(ctx, tenant, studentID, day, buckets)
	if err != nil {
		return err
	}
	history := map[uuid.UUID]reward.History{}
	for _, r := range rules {
		if t.ContentVersionID == nil {
			continue
		}
		since := now.AddDate(0, 0, -h.Policy.RepeatWindowDays(r))
		in, ever, err := h.Store.Repeats(ctx, tenant, studentID, write.RepeatKey{ContentVersionID: *t.ContentVersionID, SourceCode: r.SourceCode,
			Since: since})
		if err != nil {
			return err
		}
		history[r.VersionID] = reward.History{Repeats: in, Ever: ever}
	}
	awards := h.Policy.Evaluate(t, rules, reward.Student{Grade: st.Grade}, func(r reward.Rule) reward.History { return history[r.VersionID] },
		granted)
	school := st.SchoolID
	var lastEntry *uuid.UUID
	delta, capped := 0, false
	var label string
	for _, a := range awards {
		ruleID, ruleVersion := a.Rule.RuleID, a.Rule.Version
		e := write.XpEntry{ID: ids.New(), TenantID: tenant, StudentID: studentID, SchoolID: &school, Amount: a.Amount, ReasonCode: write.ReasonReward,
			SourceType: t.SourceType, SourceCode: a.Rule.SourceCode, SourceID: t.SourceID, ContentVersionID: t.ContentVersionID, RuleID: &ruleID,
			RuleVersion: &ruleVersion, Capped: a.Capped, IdempotencyKey: Key(studentID.String(), t.SourceType, t.SourceID.String(), ruleID.String()),
			At: now}
		inserted, err := h.Store.InsertEntry(ctx, e)
		if err != nil {
			return err
		}
		if !inserted {
			continue // replay of an already rewarded source (F074-AC-02, AC-08)
		}
		if a.Amount > 0 {
			for _, b := range uniq(reward.TotalBucket, a.Bucket) {
				if err := h.Store.AddCap(ctx, tenant, studentID, day, b, a.Amount); err != nil {
					return err
				}
			}
		}
		state.XPTotal += int64(a.Amount)
		delta += a.Amount
		capped = capped || a.Capped
		label = a.Rule.SourceCode
		id := e.ID
		lastEntry = &id
		if err := h.Events.Publish(ctx, write.Event{Type: EventXPAwarded, SubjectType: "student", SubjectID: studentID.String(),
			PartitionKey: studentID.String(), TenantID: &tenant, OccurredAt: now, Payload: xpAwardedPayload(e, state.XPTotal, state.Level)}); err != nil {
			return err
		}
		if rewarded := scaled(a); len(rewarded) > 0 {
			vid := a.Rule.VersionID
			if err := h.grant(ctx, write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: studentID, SchoolID: &school, RuleVersionID: &vid,
				TriggerEventID: t.EventID, TriggerKind: write.GrantEventRule, Grants: grantMaps(rewarded),
				IdempotencyKey: Key("event_rule", t.EventID.String(), vid.String(), studentID.String()), At: now}, nil); err != nil {
				return err
			}
		}
		h.after(ctx, func(context.Context) {
			h.metric(func(m ProgressionMetrics) { m.XPAwarded(e.SourceCode, e.Capped, e.Amount) })
		})
	}
	if lastEntry == nil {
		return nil
	}
	p := pushes{}
	curve, err := h.Curves.Current(ctx)
	if err != nil {
		return err
	}
	if p.levels, err = h.levelUp(ctx, tenant, studentID, &school, state, curve, lastEntry, now); err != nil {
		return err
	}
	if err := h.Store.SaveProgress(ctx, tenant, studentID, *state); err != nil {
		return err
	}
	p.xp = h.xpPush(state, curve, delta, capped, label, granted)
	h.push(ctx, studentID, p, now)
	elapsed := now.Sub(start).Seconds()
	h.after(ctx, func(context.Context) { h.metric(func(m ProgressionMetrics) { m.EngineDuration(elapsed) }) })
	return nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func uniq(xs ...string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

// scaled are the non-XP grants of an award with the Xu diminished like the XP (F075-AC-02); a currency grant that
// scales to 0 is dropped.
func scaled(a reward.Award) []reward.Grant {
	out := make([]reward.Grant, 0, len(a.Rewarded))
	for _, g := range a.Rewarded {
		if s, ok := g.Scaled(a.Factor); ok {
			out = append(out, s)
		}
	}
	return out
}

func grantMaps(gs []reward.Grant) []map[string]any {
	out := make([]map[string]any, 0, len(gs))
	for _, g := range gs {
		m := map[string]any{"type": g.Type}
		if g.Amount > 0 && g.Type != "xp" {
			m["amount"] = g.Amount
		}
		for k, v := range map[string]string{"title_code": g.TitleCode, "item_id": g.ItemID, "badge_id": g.BadgeID, "currency_code": g.CurrencyCode,
			"collection_item_id": g.CollectionItemID} {
			if v != "" {
				m[k] = v
			}
		}
		out = append(out, m)
	}
	return out
}

func xpAwardedPayload(e write.XpEntry, total int64, level int) map[string]any {
	return map[string]any{"xp_entry_id": e.ID, "student_id": e.StudentID, "amount": e.Amount, "reason_code": e.ReasonCode,
		"source_type": e.SourceType, "source_code": e.SourceCode, "source_id": e.SourceID, "content_version_id": e.ContentVersionID,
		"rule_id": e.RuleID, "rule_version": e.RuleVersion, "capped": e.Capped, "xp_total": max(0, total), "level": level, "awarded_at": e.At}
}

// ensureProgress returns the progression cache of the student, creating the game profile when the student earns XP
// before the first game session (no nickname, no avatar: setup stays required, F070).
func (h Progression) ensureProgress(ctx context.Context, tenant, student, school uuid.UUID) (*write.ProgressState, error) {
	state, err := h.Store.Progress(ctx, tenant, student)
	if err != nil || state != nil {
		return state, err
	}
	if err := h.Profiles.Ensure(ctx, profile.New(tenant, student, school, h.now()), nil); err != nil {
		return nil, err
	}
	if state, err = h.Store.Progress(ctx, tenant, student); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, errors.New("progression: game profile missing after ensure")
	}
	return state, nil
}

// levelUp applies the current curve to the cache (floor protection, F074-AC-13, AC-16, AC-21): one level_history row,
// one level.reached and the level rewards (one RewardGrant per (student, level, curve), F074-AC-14) for every level
// gained in order; curve_version and title_code follow the curve. Returns the realtime payloads.
func (h Progression) levelUp(ctx context.Context, tenant, student uuid.UUID, school *uuid.UUID, state *write.ProgressState, curve *port.CurveRow,
	trigger *uuid.UUID, now time.Time) ([]map[string]any, error) {
	if curve == nil {
		return nil, nil
	}
	tb := curve.Table()
	level, reached := tb.Apply(max(0, state.XPTotal), state.Level)
	var out []map[string]any
	for _, l := range reached {
		lv := tb.Level(l)
		if err := h.Store.InsertLevel(ctx, write.LevelReached{ID: ids.New(), TenantID: tenant, StudentID: student, FromLevel: l - 1, ToLevel: l,
			CurveID: curve.CurveID, CurveVersion: curve.Version, TriggerEntry: trigger, At: now}); err != nil {
			return nil, err
		}
		title := tb.TitleAt(l)
		if err := h.Events.Publish(ctx, write.Event{Type: EventLevelReached, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"student_id": student, "from_level": l - 1,
				"to_level": l, "curve_id": curve.CurveID, "curve_version": curve.Version, "title_code": nullable(title.Code),
				"trigger_xp_entry_id": trigger, "reached_at": now}}); err != nil {
			return nil, err
		}
		push := map[string]any{"from_level": l - 1, "to_level": l, "title_code": title.Code, "title": title.Name}
		if len(lv.Rewards) > 0 {
			lvl, cid := l, curve.CurveID
			gid := levelUpGrantID(curve.CurveID, l, student)
			grants := make([]map[string]any, 0, len(lv.Rewards))
			for _, r := range lv.Rewards {
				grants = append(grants, map[string]any(r))
			}
			if err := h.grant(ctx, write.RewardGrant{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: school, TriggerEventID: gid,
				TriggerKind: write.GrantLevelUp, Level: &lvl, CurveID: &cid, Grants: grants,
				IdempotencyKey: Key("level_up", student.String(), itoa(l), curve.CurveID.String()), At: now}, &lvl); err != nil {
				return nil, err
			}
			push["rewards"] = grants
		}
		out = append(out, push)
	}
	if n := len(reached); n > 0 {
		h.after(ctx, func(context.Context) { h.metric(func(m ProgressionMetrics) { m.LevelReached(n) }) })
	}
	state.Level = level
	v := curve.Version
	state.CurveVersion = &v
	if code := tb.TitleAt(level).Code; code != "" {
		state.TitleCode = &code
	}
	return out, nil
}

// grant appends a RewardGrant once (its idempotency key), applies its grants through the registered appliers and
// writes reward.granted; an existing key gives nothing again (F074-AC-14).
func (h Progression) grant(ctx context.Context, g write.RewardGrant, level *int) error {
	_, err := applyGrant(ctx, h.Store, h.Appliers, h.Events, g, level)
	return err
}

// applyGrant is the grant step shared by the Reward Engine (rules, level-ups) and the economy (achievement tiers,
// collection completions, knowledge cards, F075): append the RewardGrant once, apply every grant through its applier in
// the same transaction, write reward.granted. Returns false when the idempotency key already exists (nothing given).
func applyGrant(ctx context.Context, store write.ProgressionStore, appliers write.GrantApplierRegistry, events write.EventPublisher,
	g write.RewardGrant, level *int) (bool, error) {
	inserted, err := store.InsertGrant(ctx, g)
	if err != nil || !inserted {
		return false, err
	}
	if appliers != nil {
		if err := appliers.Apply(ctx, write.GrantContext{TenantID: g.TenantID, StudentID: g.StudentID, GrantID: g.ID, At: g.At,
			SchoolID: g.SchoolID, TriggerKind: g.TriggerKind, RuleVersionID: g.RuleVersionID}, g.Grants); err != nil {
			return false, err
		}
	}
	xp := 0
	for _, it := range g.Grants {
		if it["type"] == "xp" {
			if n, ok := it["amount"].(int); ok {
				xp += n
			}
		}
	}
	return true, events.Publish(ctx, write.Event{Type: EventRewardGranted, SubjectType: "student", SubjectID: g.StudentID.String(),
		PartitionKey: g.StudentID.String(), TenantID: &g.TenantID, OccurredAt: g.At, Payload: map[string]any{"grant_id": g.ID,
			"student_id": g.StudentID, "trigger_kind": g.TriggerKind, "items": g.Grants, "xp": xp, "level": level, "curve_id": g.CurveID,
			"source_event_id": g.TriggerEventID, "granted_at": g.At}})
}

// xpPush is progression.xp.updated (AsyncAPI ProgressionXpUpdated).
func (h Progression) xpPush(state *write.ProgressState, curve *port.CurveRow, delta int, capped bool, sourceCode string,
	granted map[string]int) map[string]any {
	m := map[string]any{"delta": delta, "xp_total": max(0, state.XPTotal), "xp_in_level": int64(0), "xp_to_next": nil, "capped": capped,
		"daily_xp_remaining": max(0, h.Policy.DailyCap-granted[reward.TotalBucket])}
	if curve != nil {
		in, next := curve.Table().Status(state.Level, max(0, state.XPTotal))
		m["xp_in_level"] = in
		if next != nil {
			m["xp_to_next"] = *next
		}
	}
	if l, ok := reward.SourceLabels[sourceCode]; ok {
		m["source_label"] = l
	}
	return m
}

// push sends the realtime messages after commit (best effort).
func (h Progression) push(ctx context.Context, student uuid.UUID, p pushes, at time.Time) {
	if h.Realtime == nil {
		return
	}
	rt := h.Realtime
	h.after(ctx, func(ctx context.Context) {
		if p.xp != nil {
			_ = rt.XPUpdated(ctx, student, p.xp, at)
		}
		for _, l := range p.levels {
			_ = rt.LevelReached(ctx, student, l, at)
		}
	})
}
