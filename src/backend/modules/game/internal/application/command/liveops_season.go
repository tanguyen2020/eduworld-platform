package command

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/season"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// recordSeason credits the season points of a fact in the current season (F077-AC-18): the best source of the track,
// once per (season, source event, student), under the daily cap of the track (season_points_daily_cap); a new tier
// writes season_track.tier_reached and season.track.updated is pushed.
func (h *LiveOps) recordSeason(ctx context.Context, tenant uuid.UUID, st port.WorldStudent, f liveops.Fact, day time.Time,
	eventQuest func(uuid.UUID) bool, now time.Time) error {
	s, err := h.Store.CurrentSeason(ctx, f.At)
	if err != nil || s == nil {
		return err
	}
	tr, err := h.Store.Track(ctx, s.SeasonID)
	if err != nil || tr == nil {
		return err
	}
	code, pts := liveops.PointsFor(tr.Track.Points.Sources, f.SourceCodes(eventQuest))
	if pts == 0 {
		return nil
	}
	today, err := h.Store.SeasonPointsOn(ctx, tenant, s.SeasonID, f.StudentID, day)
	if err != nil {
		return err
	}
	credit, capped := liveops.Earn(pts, today, tr.Track.Points.DailyCap)
	h.metric(func(m LiveOpsMetrics) { m.SeasonPoints(capped) })
	if credit == 0 {
		return nil
	}
	ok, err := h.Store.InsertSeasonPoints(ctx, write.PointEntry{ID: ids.New(), TenantID: tenant, Ref: s.SeasonID, StudentID: f.StudentID,
		Delta: credit, SourceCode: code, SourceEventID: f.EventID, Capped: capped, Day: day, At: now})
	if err != nil || !ok {
		return err
	}
	p, err := h.Store.LockSeasonProgress(ctx, tenant, s.SeasonID, f.StudentID)
	if err != nil {
		return err
	}
	if p == nil {
		p = &port.SeasonProgressRow{ID: ids.New(), TenantID: tenant, SeasonID: s.SeasonID, StudentID: f.StudentID, SchoolID: st.SchoolID,
			Claimed: []int{}, Granted: []int{}}
	}
	before := p.Tier
	p.Points += credit
	p.Tier = max(p.Tier, tr.Track.TierFor(p.Points))
	if err := h.Store.SaveSeasonProgress(ctx, *p, now); err != nil {
		return err
	}
	for tier := before + 1; tier <= p.Tier; tier++ {
		if err := h.Events.Publish(ctx, write.Event{Type: EventSeasonTierReached, SubjectType: "student", SubjectID: f.StudentID.String(),
			PartitionKey: f.StudentID.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"season_id": s.SeasonID,
				"season_track_id": tr.TrackID, "student_id": f.StudentID, "tier": tier, "points": p.Points, "reached_at": now}}); err != nil {
			return err
		}
	}
	if h.Realtime != nil {
		payload := map[string]any{"season_id": s.SeasonID, "delta": credit, "points": p.Points, "tier": p.Tier, "tier_reached": p.Tier > before,
			"daily_points_remaining": max(0, tr.Track.Points.DailyCap-today-credit)}
		student, rt := f.StudentID, h.Realtime
		h.after(ctx, func(ctx context.Context) { _ = rt.Push(ctx, RTSeasonTrackUpdated, []uuid.UUID{student}, payload, now) })
	}
	return nil
}

// ClaimTier is claimSeasonTier (F077-AC-19): a reached tier of the current season is granted once through the Reward
// Engine (season_track.reward_claimed, audit); a tier not reached is 409 SEASON_TIER_NOT_REACHED; claimed already 409.
func (h *LiveOps) ClaimTier(ctx context.Context, tier int) (out ClaimResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ClaimSeasonTier")
	defer func() { done(err) }()
	student, tenant, err := h.playerOf(ctx)
	if err != nil {
		return out, err
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		s, err := h.Store.CurrentSeason(ctx, now)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		tr, err := h.Store.Track(ctx, s.SeasonID)
		if err != nil {
			return err
		}
		if tr == nil || tier < 1 || tier > tr.Track.MaxTier() {
			return problem.New(problem.NotFound, "")
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.LockSeasonProgress(ctx, tenant, s.SeasonID, student)
		if err != nil {
			return err
		}
		if p == nil || p.Tier < tier {
			return problem.New(problem.SeasonTierNotReached, "")
		}
		if slices.Contains(p.Claimed, tier) || slices.Contains(p.Granted, tier) {
			return problem.New(problem.Conflict, "the tier reward was given already")
		}
		school := p.SchoolID
		rewards := tr.Track.Rewards(tier)
		if _, err := h.grant(ctx, tenant, student, &school, write.GrantSeasonTier, "tier:"+s.SeasonID.String()+":"+itoa(tier)+":"+student.String(),
			rewards, now); err != nil {
			return err
		}
		p.Claimed = append(p.Claimed, tier)
		if err := h.Store.SaveSeasonProgress(ctx, *p, now); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventSeasonRewardClaimed, SubjectType: "student", SubjectID: student.String(),
			PartitionKey: student.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"season_id": s.SeasonID,
				"season_track_id": tr.TrackID, "student_id": student, "tier": tier, "auto_granted": false, "claimed_at": now}}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditSeasonTierClaimed, TargetType: "season", TargetID: s.SeasonID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"tier": tier}}); err != nil {
			return err
		}
		out = ClaimResult{Claimed: true, Rewards: copyGrants(rewards), ClaimedAt: now}
		h.after(ctx, func(context.Context) { h.metric(func(m LiveOpsMetrics) { m.Claim("season_tier") }) })
		return idempotency.SetResult(ctx, out.View())
	})
	return out, err
}

// SeasonEvent is season.started or season.ended of module competition (owner F106).
type SeasonEvent struct {
	Type          string
	SeasonID      uuid.UUID
	Code          string
	Name          string
	ThemeCode     *string
	StartsAt      *time.Time
	EndsAt        *time.Time
	EndedAt       *time.Time
	SourceVersion int64
}

// ProjectSeason is the consumer game.season_projection (F077-AC-17, AC-20): ref.seasons follows season.started and
// season.ended by source_version (an older event is ignored: ended before started stays ended); when a season ends,
// every reached tier neither claimed nor granted is granted (auto_granted), tenant by tenant; the history stays and the
// next season starts from 0 (a new progress row). Platform transaction of the envelope.
func (h *LiveOps) ProjectSeason(ctx context.Context, e SeasonEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ProjectSeason")
	defer func() { done(err) }()
	cur, err := h.Store.Season(ctx, e.SeasonID)
	if err != nil {
		return err
	}
	row := port.SeasonRow{SeasonID: e.SeasonID, Code: e.Code, Name: e.Name, ThemeCode: e.ThemeCode, SourceVersion: e.SourceVersion}
	if cur != nil {
		row.Name, row.ThemeCode, row.StartsAt, row.EndsAt = cur.Name, cur.ThemeCode, cur.StartsAt, cur.EndsAt
		if e.Name != "" {
			row.Name = e.Name
		}
		if e.ThemeCode != nil {
			row.ThemeCode = e.ThemeCode
		}
	}
	if e.StartsAt != nil {
		row.StartsAt = e.StartsAt.UTC()
	}
	if e.EndsAt != nil {
		row.EndsAt = e.EndsAt.UTC()
	}
	switch e.Type {
	case "season.started":
		row.Status = "started"
	case "season.ended":
		row.Status, row.EndedAt = "ended", e.EndedAt
		if row.StartsAt.IsZero() && e.EndedAt != nil {
			row.StartsAt, row.EndsAt = *e.EndedAt, *e.EndedAt
		}
	default:
		return nil
	}
	applied, err := h.Store.UpsertSeason(ctx, row)
	if err != nil || !applied || row.Status != "ended" {
		return err
	}
	return h.closeSeason(ctx, e.SeasonID)
}

// closeSeason grants the unclaimed reached tiers of a season to every student (F077-AC-20).
func (h *LiveOps) closeSeason(ctx context.Context, seasonID uuid.UUID) error {
	tr, err := h.Store.Track(ctx, seasonID)
	if err != nil || tr == nil {
		return err
	}
	tenants, err := h.Store.SeasonTenants(ctx, seasonID)
	if err != nil {
		return err
	}
	now := h.now()
	for _, t := range tenants {
		sc, err := authz.EventScope(t)
		if err != nil {
			return err
		}
		after := uuid.Nil
		for {
			var page []port.SeasonProgressRow
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				var err error
				if page, err = h.Store.SeasonProgressPage(ctx, t, seasonID, after, liveOpsPage); err != nil {
					return err
				}
				granted := 0
				for _, p := range page {
					n, err := h.autoGrantTiers(ctx, *tr, p, now)
					if err != nil {
						return err
					}
					granted += n
				}
				if granted > 0 {
					return h.Audit.Record(ctx, audit.Event{Action: auditSeasonTiersAutoGranted, TargetType: "season", TargetID: seasonID.String(),
						Result: audit.ResultSuccess, Metadata: map[string]any{"granted": granted}})
				}
				return nil
			}); err != nil {
				return err
			}
			if len(page) < liveOpsPage {
				break
			}
			after = page[len(page)-1].ID
		}
	}
	return nil
}

func (h *LiveOps) autoGrantTiers(ctx context.Context, tr port.TrackRow, p port.SeasonProgressRow, now time.Time) (int, error) {
	if err := h.Store.LockStudent(ctx, p.StudentID); err != nil {
		return 0, err
	}
	locked, err := h.Store.LockSeasonProgress(ctx, p.TenantID, p.SeasonID, p.StudentID)
	if err != nil || locked == nil {
		return 0, err
	}
	pending := season.Unclaimed(locked.Tier, locked.Claimed, locked.Granted)
	school := locked.SchoolID
	for _, tier := range pending {
		if _, err := h.grant(ctx, locked.TenantID, locked.StudentID, &school, write.GrantSeasonTier, "tier:"+locked.SeasonID.String()+":"+itoa(tier)+":"+
			locked.StudentID.String(), tr.Track.Rewards(tier), now); err != nil {
			return 0, err
		}
		locked.Granted = append(locked.Granted, tier)
		if err := h.Events.Publish(ctx, write.Event{Type: EventSeasonRewardClaimed, SubjectType: "student", SubjectID: locked.StudentID.String(),
			PartitionKey: locked.StudentID.String(), TenantID: &locked.TenantID, OccurredAt: now, Payload: map[string]any{"season_id": locked.SeasonID,
				"season_track_id": tr.TrackID, "student_id": locked.StudentID, "tier": tier, "auto_granted": true, "claimed_at": now}}); err != nil {
			return 0, err
		}
	}
	at := now
	locked.AutoGrantedAt = &at
	if err := h.Store.SaveSeasonProgress(ctx, *locked, now); err != nil {
		return 0, err
	}
	if len(pending) > 0 {
		h.metric(func(m LiveOpsMetrics) { m.Claim("season_tier_auto") })
	}
	return len(pending), nil
}
