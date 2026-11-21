package command

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// World events (specs/events/payloads/game/*.v1).
const (
	EventAreaEntered      = "world.area_entered"
	EventAreaUnlocked     = "world.area_unlocked"
	EventDiscoveryFound   = "world.discovery_found"
	EventAreaCompleted    = "world.area_completed"
	EventChapterCompleted = "story.chapter_completed"
)

// WorldConfig is the configuration of the world use cases (CONFIG_DEFAULTS through infrastructure/config).
type WorldConfig struct {
	PresenceTTL       time.Duration // game_session_heartbeat_timeout: the presence lives as long as the session heartbeat
	DiscoveryRate     int           // discovery_report_rate (30 per minute per student)
	DiscoveryWindow   time.Duration // 1 minute
	EnteredDedupe     time.Duration // world.area_entered at most once per area per minute
	InteractRadius    float64       // default interaction radius of an anchor without interact_radius_m (m)
	InteractTolerance float64       // tolerance added to the radius for the hub position sampling (m)
}

// WorldMetrics observes the world use cases; nil = not observed.
type WorldMetrics interface {
	AreaEnter(result string)
	AreaUnlocked()
	Discovery(result string)
}

// World holds the world use cases of a student (F071): enterWorldArea, startLevel, endLevelSession, reportDiscovery and
// the progress events (unlocks, story).
type World struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Content     port.WorldContent
	Store       write.WorldStore
	Sessions    write.SessionRepository
	Manifests   write.ManifestStore
	Events      write.EventPublisher
	Presence    port.PresenceStore
	Rate        port.RateLimiter
	Realtime    port.WorldRealtime // nil: no realtime push
	Hub         port.HubPositions  // nil: the interaction distance is not checked (presence only)
	Metrics     WorldMetrics
	Config      WorldConfig
	Now         func() time.Time
}

func (h World) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h World) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

func (h World) metric(fn func(m WorldMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

// player returns the student and tenant of the session (game_player routes: students only).
func player(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return p.UserID, *p.TenantID, nil
}

// activeSession checks that game_session_id is an active session of the student (F071-AC-10: 409 otherwise).
func (h World) activeSession(ctx context.Context, tenant, student, id uuid.UUID) error {
	s, err := h.Sessions.ByID(ctx, tenant, id)
	if err != nil {
		return err
	}
	if s == nil || !s.Active() || s.StudentID != student {
		return problem.New(problem.Conflict, "game_session_id is not an active session of the student")
	}
	return nil
}

// areaContext loads what the access decision of one area needs.
type areaContext struct {
	area     *port.AreaRow
	student  *port.WorldStudent
	progress *port.WorldProgress
	access   world.Access
}

func (h World) decide(ctx context.Context, tenant, student uuid.UUID, areaID string) (*areaContext, error) {
	area, err := h.Content.CurrentArea(ctx, areaID)
	if err != nil {
		return nil, err
	}
	if area == nil {
		return nil, problem.New(problem.NotFound, "unknown area")
	}
	st, err := h.Content.Student(ctx, tenant, student)
	if err != nil {
		return nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return nil, problem.New(problem.NotFound, "student not found")
	}
	chapters, err := h.Content.CurrentChapters(ctx)
	if err != nil {
		return nil, err
	}
	pr, err := h.Content.Progress(ctx, tenant, student, chapters)
	if err != nil {
		return nil, err
	}
	_, unlocked := pr.Unlocks[areaID]
	c := &areaContext{area: area, student: st, progress: pr, access: world.Decide(area.Area, st.Domain(), unlocked, pr.Facts)}
	if !c.access.Visible {
		return nil, problem.New(problem.NotFound, "unknown area")
	}
	if !c.access.Accessible {
		missing := world.Missing(c.access.Requirements)
		return nil, problem.New(problem.AreaLocked, c.access.LockReason).With("missing_requirements", missing)
	}
	return c, nil
}

// unlockIfMet records the unlock of an area whose rule is met but not yet recorded (the consumer may lag behind).
func (h World) unlockIfMet(ctx context.Context, tenant, student uuid.UUID, c *areaContext, at time.Time) error {
	a := c.area.Area
	if a.IsLearning() || a.UnlockRule == nil || !c.access.RuleMet {
		return nil
	}
	if _, ok := c.progress.Unlocks[a.AreaID]; ok {
		return nil
	}
	return h.unlock(ctx, tenant, student, *c.area, world.UnlockedBy(c.access.Requirements), at)
}

// unlock records one unlock (once) with its event and realtime push (F071-AC-15).
func (h World) unlock(ctx context.Context, tenant, student uuid.UUID, a port.AreaRow, by string, at time.Time) error {
	ok, err := h.Store.InsertUnlock(ctx, tenant, student, a.Area.AreaID, a.VersionNo, by, at)
	if err != nil || !ok {
		return err
	}
	t := tenant
	if err := h.Events.Publish(ctx, write.Event{Type: EventAreaUnlocked, SubjectType: "world_area", SubjectID: a.Area.AreaID,
		PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"student_id": student, "area_id": a.Area.AreaID,
			"rule_version": a.VersionNo, "unlocked_by": by, "unlocked_at": at}}); err != nil {
		return err
	}
	name := a.Area.Name
	h.after(ctx, func(ctx context.Context) {
		h.metric(func(m WorldMetrics) { m.AreaUnlocked() })
		if h.Realtime != nil {
			_ = h.Realtime.AreaUnlocked(ctx, student, a.Area.AreaID, name, by, at)
		}
	})
	return nil
}

// EnterInput is the request of enterWorldArea.
type EnterInput struct {
	AreaID        string
	GameSessionID uuid.UUID
	PortalID      string
}

// EnterResult is AreaEnterResult.
type EnterResult struct {
	AreaID       string     `json:"area_id"`
	RoomID       uuid.UUID  `json:"room_id"`
	RoomType     string     `json:"room_type"`
	SpawnPointID string     `json:"spawn_point_id"`
	LevelID      *uuid.UUID `json:"level_id,omitempty"`
	AreaVersion  int        `json:"area_version"`
}

// EnterArea implements enterWorldArea (F071-AC-07…AC-10): the area must be open for the student (403 AREA_LOCKED with the
// missing requirements otherwise), the game session active and the student's own (409); the partition room never mixes
// two schools (allocator v0 — ALLOCATOR TẠM: the hub instance itself is assigned by the room host of F091 on
// hub.instance.join with the same partition). The presence is written after commit; last_area_id only for public areas.
func (h World) EnterArea(ctx context.Context, in EnterInput) (out EnterResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.EnterWorldArea")
	defer func() {
		done(err)
		result := "ok"
		if err != nil {
			result = string(codeOf(err))
		}
		h.metric(func(m WorldMetrics) { m.AreaEnter(result) })
	}()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	if !world.ValidAreaID(in.AreaID) {
		return out, problem.New(problem.NotFound, "unknown area")
	}
	prev, _ := h.presence(ctx, student)
	at := h.now()
	var pres port.Presence
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.activeSession(ctx, tenant, student, in.GameSessionID); err != nil {
			return err
		}
		c, err := h.decide(ctx, tenant, student, in.AreaID)
		if err != nil {
			return err
		}
		if err := h.unlockIfMet(ctx, tenant, student, c, at); err != nil {
			return err
		}
		a := c.area.Area
		spawn := a.DefaultSpawn()
		if in.PortalID != "" && prev != nil && prev.TenantID == tenant {
			if src, err := h.Content.CurrentArea(ctx, prev.AreaID); err == nil && src != nil {
				if pt, ok := src.Area.Portal(in.PortalID); ok && pt.TargetAreaID == a.AreaID {
					spawn = a.SpawnFor(pt.TargetSpawnPoint)
				}
			}
		}
		band, _ := world.GradeBand(c.student.Grade)
		room := world.InstanceKey{TenantID: tenant, SchoolID: c.student.SchoolID, AreaID: a.AreaID, GradeBand: band}.RoomID()
		out = EnterResult{AreaID: a.AreaID, RoomID: room, RoomType: roomType(a), SpawnPointID: spawn, AreaVersion: c.area.VersionNo}
		lvl, err := h.Content.LevelOfArea(ctx, a.AreaID)
		if err != nil {
			return err
		}
		if lvl != nil {
			out.LevelID = &lvl.LevelID
		}
		if a.Access.Public {
			if err := h.Store.SetLastArea(ctx, tenant, student, a.AreaID, at); err != nil {
				return err
			}
		}
		pres = port.Presence{TenantID: tenant, GameSessionID: in.GameSessionID, AreaID: a.AreaID, RoomID: room, EnteredAt: at}
		if prev != nil && prev.AreaID == a.AreaID && prev.GameSessionID == in.GameSessionID {
			pres.LevelSessionID = prev.LevelSessionID
			if at.Sub(prev.EnteredAt) < h.Config.EnteredDedupe {
				pres.EnteredAt = prev.EnteredAt
				return nil // world.area_entered at most once per area per minute
			}
		}
		var from any
		if prev != nil && prev.TenantID == tenant {
			from = prev.AreaID
		}
		t := tenant
		return h.Events.Publish(ctx, write.Event{Type: EventAreaEntered, SubjectType: "world_area", SubjectID: a.AreaID,
			PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"student_id": student, "area_id": a.AreaID,
				"from_area_id": from, "instance_kind": out.RoomType, "entered_at": at}})
	})
	if err != nil {
		return out, err
	}
	if h.Presence != nil {
		if err := h.Presence.Set(ctx, student, pres, h.Config.PresenceTTL); err != nil {
			return out, problem.New(problem.Unavailable, "presence store unavailable").Wrap(err).Retry()
		}
	}
	return out, nil
}

func roomType(a world.Area) string {
	if a.Access.RoomType == world.RoomPrivate {
		return world.RoomPrivate
	}
	return world.RoomHub
}

func (h World) presence(ctx context.Context, student uuid.UUID) (*port.Presence, error) {
	if h.Presence == nil {
		return nil, nil
	}
	return h.Presence.Get(ctx, student)
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return "error"
}

// LevelSessionView is the LevelSession response.
type LevelSessionView struct {
	LevelSessionID  uuid.UUID        `json:"level_session_id"`
	LevelID         uuid.UUID        `json:"level_id"`
	LevelVersion    int              `json:"level_version"`
	ManifestVersion int64            `json:"manifest_version"`
	Status          string           `json:"status"`
	ContentVersions []map[string]any `json:"content_versions"`
	Bundles         []map[string]any `json:"bundles"`
	StartedAt       time.Time        `json:"started_at"`
	EndedAt         *time.Time       `json:"ended_at,omitempty"`
}

func viewOf(s write.LevelSession, bundles []map[string]any) LevelSessionView {
	if bundles == nil {
		bundles = []map[string]any{}
	}
	return LevelSessionView{LevelSessionID: s.ID, LevelID: s.LevelID, LevelVersion: s.LevelVersionNo, ManifestVersion: s.ManifestVersion,
		Status: s.Status, ContentVersions: s.ContentVersions, Bundles: bundles, StartedAt: s.StartedAt, EndedAt: s.EndedAt}
}

func pin(itemType string, id uuid.UUID, code string, versionID uuid.UUID, versionNo int, sha string) map[string]any {
	m := map[string]any{"item_type": itemType, "item_id": id.String(), "version_id": versionID.String(), "version_no": versionNo, "sha256": sha}
	if code != "" {
		m["item_code"] = code
	}
	return m
}

// StartLevel implements startLevel (F071-AC-11, ADR-0043): a level session pins the current versions of the level, its
// area, the story chapters and the dialogues of the NPCs of the area, and the current manifest_version; one active level
// session per student (an older active one expires). Idempotency-Key is the httpx middleware.
func (h World) StartLevel(ctx context.Context, levelID, gameSessionID uuid.UUID) (out LevelSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartLevel")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	var created write.LevelSession
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.activeSession(ctx, tenant, student, gameSessionID); err != nil {
			return err
		}
		lvl, err := h.Content.CurrentLevel(ctx, levelID)
		if err != nil {
			return err
		}
		if lvl == nil {
			return problem.New(problem.NotFound, "unknown level")
		}
		c, err := h.decide(ctx, tenant, student, lvl.Level.AreaID)
		if err != nil {
			return err
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		if old, err := h.Store.ActiveLevelSession(ctx, tenant, student); err != nil {
			return err
		} else if old != nil {
			if err := h.Store.EndLevelSession(ctx, tenant, old.ID, write.LevelExpired, at); err != nil {
				return err
			}
		}
		pins := []map[string]any{pin(write.ItemLevel, lvl.LevelID, lvl.Level.AreaID, lvl.VersionID, lvl.VersionNo, lvl.SHA256),
			pin(write.ItemWorldArea, world.ContentItemID(write.ItemWorldArea, c.area.Area.AreaID), c.area.Area.AreaID, c.area.VersionID,
				c.area.VersionNo, c.area.SHA256)}
		chapters, err := h.Content.CurrentChapters(ctx)
		if err != nil {
			return err
		}
		for _, ch := range chapters {
			pins = append(pins, pin(write.ItemStoryChapter, world.ContentItemID(write.ItemStoryChapter, ch.Chapter.Code), ch.Chapter.Code,
				ch.VersionID, ch.VersionNo, ch.SHA256))
		}
		npcs := c.area.Area.NPCs()
		dialogues, err := h.Content.CurrentDialogues(ctx, "")
		if err != nil {
			return err
		}
		for _, d := range dialogues {
			if slices.Contains(npcs, d.NPCID) {
				pins = append(pins, pin(write.ItemNPCDialogue, d.DialogueID, d.NPCID, d.VersionID, d.VersionNo, d.SHA256))
			}
		}
		manifest, _, err := h.Manifests.Latest(ctx, tenant)
		if err != nil {
			return err
		}
		created = write.LevelSession{ID: ids.New(), TenantID: tenant, StudentID: student, GameSessionID: gameSessionID, LevelID: lvl.LevelID,
			LevelVersionID: lvl.VersionID, LevelVersionNo: lvl.VersionNo, AreaID: lvl.Level.AreaID, ManifestVersion: manifest,
			ContentVersions: pins, Status: write.LevelActive, StartedAt: at}
		if err := h.Store.CreateLevelSession(ctx, created); err != nil {
			return err
		}
		b := map[string]any{"bundle_id": lvl.Level.BundleID, "version": lvl.VersionNo, "url": lvl.Level.BundleURL, "sha256": lvl.Level.BundleSHA256,
			"size_bytes": lvl.Level.SizeBytes, "platforms": lvl.Level.Platforms}
		if lvl.Level.RequiresClient != "" {
			b["requires_client"] = lvl.Level.RequiresClient
		}
		out = viewOf(created, []map[string]any{b})
		return idempotency.SetResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	h.setLevelPresence(ctx, student, created.AreaID, &created.ID)
	return out, nil
}

func (h World) setLevelPresence(ctx context.Context, student uuid.UUID, area string, level *uuid.UUID) {
	if h.Presence == nil {
		return
	}
	p, err := h.Presence.Get(ctx, student)
	if err != nil || p == nil || p.AreaID != area {
		return
	}
	p.LevelSessionID = level
	_ = h.Presence.Set(ctx, student, *p, h.Config.PresenceTTL)
}

// EndLevel implements endLevelSession (F071-AC-11): the level session of the student ends (completed or abandoned) and
// its pins no longer apply; an unknown or foreign session is 404, an ended one 409.
func (h World) EndLevel(ctx context.Context, id uuid.UUID, outcome string) (out LevelSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.EndLevelSession")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	if outcome != write.LevelCompleted && outcome != write.LevelAbandoned {
		return out, problem.Invalid("outcome", problem.SubInvalid, "completed or abandoned")
	}
	at := h.now()
	var area string
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.LevelSession(ctx, tenant, id)
		if err != nil {
			return err
		}
		if s == nil || s.StudentID != student {
			return problem.New(problem.NotFound, "level session not found")
		}
		if s.Status != write.LevelActive {
			return problem.New(problem.Conflict, "level session already ended")
		}
		if err := h.Store.EndLevelSession(ctx, tenant, id, outcome, at); err != nil {
			return err
		}
		s.Status, s.EndedAt, area = outcome, &at, s.AreaID
		out = viewOf(*s, nil)
		return idempotency.SetResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	h.setLevelPresence(ctx, student, area, nil)
	return out, nil
}

// DiscoveryResult is the response of reportDiscovery.
type DiscoveryResult struct {
	AreaID            string      `json:"area_id"`
	InteractableID    string      `json:"interactable_id"`
	Kind              string      `json:"kind"`
	AreaCompletionPct float64     `json:"area_completion_pct"`
	Rewards           []any       `json:"rewards"`
	UnlockedQuestIDs  []uuid.UUID `json:"unlocked_quest_ids"`
}

// ReportDiscovery implements reportDiscovery (F071-AC-24, AC-25): discovery_report_rate per student (429), the
// presence must be in the area (409 STEP_PRECONDITION_FAILED — also when the presence was lost), the interactable must
// be a discovery or collectible of the area version pinned by the level session (else the current version) and, when
// the hub knows the student's position, within reach; each anchor counts once (409 DISCOVERY_ALREADY_FOUND); the
// percentage is against the current area version. XP and rewards are F074/F075 (rewards empty), secret quests F072.
func (h World) ReportDiscovery(ctx context.Context, areaID, interactableID string) (out DiscoveryResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ReportDiscovery")
	defer func() {
		done(err)
		result := "ok"
		if err != nil {
			result = string(codeOf(err))
		}
		h.metric(func(m WorldMetrics) { m.Discovery(result) })
	}()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	if h.Rate != nil {
		ok, wait, err := h.Rate.Allow(ctx, "discovery:"+student.String(), h.Config.DiscoveryRate, h.Config.DiscoveryWindow)
		if err != nil {
			return out, problem.New(problem.Unavailable, "rate limiter unavailable").Wrap(err).Retry()
		}
		if !ok {
			return out, RetryAfter{Err: problem.New(problem.RateLimited, "discovery_report_rate"), After: wait}
		}
	}
	pres, err := h.presence(ctx, student)
	if err != nil {
		return out, problem.New(problem.Unavailable, "presence store unavailable").Wrap(err).Retry()
	}
	if pres == nil || pres.TenantID != tenant || pres.AreaID != areaID {
		return out, problem.New(problem.StepPreconditionFailed, "not in this area")
	}
	if err := h.withinReach(ctx, tenant, student, areaID, interactableID, pres); err != nil {
		return out, err
	}
	at := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		pinned, err := h.pinnedArea(ctx, tenant, student, areaID, pres)
		if err != nil {
			return err
		}
		it, ok := pinned.Area.Interactable(interactableID)
		if !ok || (it.Kind != world.KindDiscovery && it.Kind != world.KindCollectible) {
			return problem.New(problem.StepPreconditionFailed, "not a discovery of this area")
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		current, err := h.Content.CurrentArea(ctx, areaID)
		if err != nil {
			return err
		}
		if current == nil {
			return problem.New(problem.StepPreconditionFailed, "area no longer published")
		}
		pr, err := h.Content.Progress(ctx, tenant, student, nil)
		if err != nil {
			return err
		}
		var e world.Exploration
		var completedAt *time.Time
		if row := pr.Exploration[areaID]; row != nil {
			e, completedAt = row.Exploration, row.CompletedAt
		}
		if !e.Record(it.ID, it.Kind) {
			return problem.New(problem.DiscoveryAlreadyFound, "")
		}
		c := e.Completion(current.Area)
		completedNow := c.Pct >= 100 && completedAt == nil
		if completedNow {
			completedAt = &at
		}
		if err := h.Store.SaveExploration(ctx, tenant, student, areaID, e, c.Pct, completedAt, at); err != nil {
			return err
		}
		t := tenant
		var collection any
		if it.CollectionID != "" {
			collection = it.CollectionID
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventDiscoveryFound, SubjectType: "world_area", SubjectID: areaID,
			PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"student_id": student, "area_id": areaID,
				"interactable_id": it.ID, "kind": it.Kind, "collection_id": collection, "level_session_id": pres.LevelSessionID,
				"completion_pct": c.Pct, "found_at": at}}); err != nil {
			return err
		}
		if completedNow {
			if err := h.Events.Publish(ctx, write.Event{Type: EventAreaCompleted, SubjectType: "world_area", SubjectID: areaID,
				PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"student_id": student, "area_id": areaID,
					"completion_pct": c.Pct, "completed_at": at}}); err != nil {
				return err
			}
		}
		out = DiscoveryResult{AreaID: areaID, InteractableID: it.ID, Kind: it.Kind, AreaCompletionPct: c.Pct, Rewards: []any{},
			UnlockedQuestIDs: []uuid.UUID{}}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// pinnedArea is the area version pinned by the active level session of the presence, else the current version.
func (h World) pinnedArea(ctx context.Context, tenant, student uuid.UUID, areaID string, pres *port.Presence) (*port.AreaRow, error) {
	if pres.LevelSessionID != nil {
		s, err := h.Store.LevelSession(ctx, tenant, *pres.LevelSessionID)
		if err != nil {
			return nil, err
		}
		if s != nil && s.StudentID == student && s.Status == write.LevelActive && s.AreaID == areaID {
			for _, p := range s.ContentVersions {
				if p["item_type"] == write.ItemWorldArea {
					vid, err := uuid.Parse(fmt.Sprint(p["version_id"]))
					if err != nil {
						break
					}
					a, err := h.Content.AreaVersion(ctx, vid)
					if err != nil || a != nil {
						return a, err
					}
				}
			}
		}
	}
	a, err := h.Content.CurrentArea(ctx, areaID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, problem.New(problem.StepPreconditionFailed, "area no longer published")
	}
	return a, nil
}

// withinReach checks the interaction distance when the hub (F091, getHubPosition) knows the position of the student in
// this area; an unavailable facade or a student outside any hub instance leaves the presence check alone.
func (h World) withinReach(ctx context.Context, tenant, student uuid.UUID, areaID, interactableID string, pres *port.Presence) error {
	if h.Hub == nil {
		return nil
	}
	var anchor world.Interactable
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.pinnedArea(ctx, tenant, student, areaID, pres)
		if err != nil {
			return err
		}
		anchor, _ = a.Area.Interactable(interactableID)
		return nil
	})
	if err != nil || anchor.Position == nil {
		return err
	}
	pos, err := h.Hub.Position(ctx, tenant, student)
	if err != nil || pos == nil || pos.AreaID != areaID {
		return nil //nolint:nilerr // facade unavailable or not in a hub instance: presence only (F071 T049)
	}
	radius := h.Config.InteractRadius
	if anchor.RadiusM != nil {
		radius = *anchor.RadiusM
	}
	p := *anchor.Position
	dx, dy, dz := pos.Position[0]-p[0], pos.Position[1]-p[1], pos.Position[2]-p[2]
	if dx*dx+dy*dy+dz*dz > (radius+h.Config.InteractTolerance)*(radius+h.Config.InteractTolerance) {
		return problem.New(problem.StepPreconditionFailed, "too far from the discovery")
	}
	return nil
}
