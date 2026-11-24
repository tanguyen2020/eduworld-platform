package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/season"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Contract bounds of GameClientManifest (maxItems / maxLength of each list).
const (
	manifestAreasMax         = 1000
	manifestNPCsMax          = 5000
	manifestScenesMax        = 5000
	manifestInteractablesMax = 10000
	manifestIDMaxLen         = 64
	manifestSceneMaxLen      = 128
	localizedNameMaxLen      = 255
)

// ManifestPlatform is one registered platform build of GameClientManifest.
type ManifestPlatform struct {
	Platform      string    `json:"platform"`
	ClientVersion string    `json:"client_version"`
	PublishedAt   time.Time `json:"published_at"`
}

// ManifestArea is one area of GameClientManifest (name of its published world area when any).
type ManifestArea struct {
	AreaID string            `json:"area_id"`
	Name   map[string]string `json:"name,omitempty"`
}

// ManifestNPC is one NPC of GameClientManifest (npc_name of a published dialogue when any).
type ManifestNPC struct {
	NPCID string            `json:"npc_id"`
	Name  map[string]string `json:"name,omitempty"`
}

// ClientManifestView is GameClientManifest.
type ClientManifestView struct {
	Registered    bool               `json:"registered"`
	Platforms     []ManifestPlatform `json:"platforms"`
	Areas         []ManifestArea     `json:"areas"`
	NPCs          []ManifestNPC      `json:"npcs"`
	SceneRefs     []string           `json:"scene_refs"`
	Interactables []string           `json:"interactables"`
}

// ClientManifest is getGameClientManifest (F079-AC-07): the intersection of the newest manifest of every registered
// platform — the values submit_review accepts — with the names of the published areas and NPCs. Authorization: any console
// write permission, game.content.review or game.content.publish.
func (h GameAdmin) ClientManifest(ctx context.Context) (out ClientManifestView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameClientManifest")
	defer func() { done(err) }()
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireAnyRead(ctx, p); err != nil {
		return out, err
	}
	err = h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		rows, err := h.Data.ClientManifests(ctx)
		if err != nil {
			return err
		}
		out = ClientManifestView{Registered: len(rows) > 0, Platforms: []ManifestPlatform{}, Areas: []ManifestArea{}, NPCs: []ManifestNPC{},
			SceneRefs: []string{}, Interactables: []string{}}
		ms := make([]gameconfig.ClientManifest, 0, len(rows))
		for _, r := range rows {
			var m gameconfig.ClientManifest
			if err := json.Unmarshal(r.Manifest, &m); err != nil {
				return fmt.Errorf("client manifest %s: %w", r.Platform, err)
			}
			ms = append(ms, m)
			out.Platforms = append(out.Platforms, ManifestPlatform{Platform: r.Platform, ClientVersion: r.ClientVersion, PublishedAt: r.PublishedAt})
		}
		inter := gameconfig.IntersectManifests(ms)
		areas := bounded(ctx, "areas", inter.Areas, manifestIDMaxLen, manifestAreasMax)
		npcs := bounded(ctx, "npcs", inter.NPCs, manifestIDMaxLen, manifestNPCsMax)
		out.SceneRefs = bounded(ctx, "scene_refs", inter.Scenes, manifestSceneMaxLen, manifestScenesMax)
		out.Interactables = bounded(ctx, "interactables", inter.Interactables, manifestIDMaxLen, manifestInteractablesMax)
		areaNames, err := h.Data.AreaNames(ctx, areas)
		if err != nil {
			return err
		}
		npcNames, err := h.Data.NPCNames(ctx, npcs)
		if err != nil {
			return err
		}
		for _, a := range areas {
			out.Areas = append(out.Areas, ManifestArea{AreaID: a, Name: localizedName(areaNames[a])})
		}
		for _, n := range npcs {
			out.NPCs = append(out.NPCs, ManifestNPC{NPCID: n, Name: localizedName(npcNames[n])})
		}
		return nil
	})
	return out, err
}

// bounded keeps the values within the contract (maxLength, maxItems); a dropped value is logged (a build manifest
// outside the contract) — never silently served.
func bounded(ctx context.Context, list string, in []string, maxLen, maxItems int) []string {
	out := make([]string, 0, min(len(in), maxItems))
	dropped := 0
	for _, v := range in {
		if utf8.RuneCountInString(v) > maxLen || len(out) == maxItems {
			dropped++
			continue
		}
		out = append(out, v)
	}
	if dropped > 0 {
		slog.WarnContext(ctx, "client manifest values outside the contract dropped", "list", list, "dropped", dropped)
	}
	return out
}

// localizedName is a LocalizedName of the contract (vi and en, 1..255 characters) or nil.
func localizedName(n map[string]string) map[string]string {
	vi, en := n["vi"], n["en"]
	if vi == "" || en == "" || utf8.RuneCountInString(vi) > localizedNameMaxLen || utf8.RuneCountInString(en) > localizedNameMaxLen {
		return nil
	}
	return map[string]string{"vi": vi, "en": en}
}

// Simulable config types (GameSimulableConfigType).
const (
	simProgressionCurves = "progression-curves"
	simSeasonTracks      = "season-tracks"
)

// SimulationInput is the query of simulateGameConfig: MinutesPerDay is required for progression-curves and ignored for
// season-tracks.
type SimulationInput struct {
	MinutesPerDay *int
	DaysPerWeek   int
	Weeks         int
}

// SimulationAssumptions are GameConfigSimulation.assumptions.
type SimulationAssumptions struct {
	PerPlayDay    int64 `json:"per_play_day"`
	MinutesPerDay *int  `json:"minutes_per_day,omitempty"`
	DaysPerWeek   int   `json:"days_per_week"`
	Weeks         int   `json:"weeks"`
	MinuteRate    *int  `json:"minute_rate,omitempty"`
	DailyCap      int   `json:"daily_cap"`
}

// SimulationPoint is one entry of GameConfigSimulation.points.
type SimulationPoint struct {
	Week  int   `json:"week"`
	Level int   `json:"level"`
	Total int64 `json:"total"`
}

// SimulationThreshold is one entry of GameConfigSimulation.thresholds.
type SimulationThreshold struct {
	Level  int   `json:"level"`
	ToNext int64 `json:"to_next"`
	Total  int64 `json:"total"`
}

// SimulationView is GameConfigSimulation.
type SimulationView struct {
	ConfigType  string                `json:"config_type"`
	ConfigID    uuid.UUID             `json:"config_id"`
	Version     int                   `json:"version"`
	Assumptions SimulationAssumptions `json:"assumptions"`
	Points      []SimulationPoint     `json:"points"`
	Thresholds  []SimulationThreshold `json:"thresholds"`
}

// Simulate is simulateGameConfig (F079-AC-10, AC-15): the weekly progress of a player on the payload of one version —
// read only (no write, no event). progression-curves: XP per play day = min(minutes_per_day × xp_simulation_minute_rate,
// xp_daily_cap), level by the curve (formula + level_overrides, capped at max_level); season-tracks: points per play day
// = points.daily_cap, level = tier reached (at least 1, at most the last tier). A payload that cannot be simulated is 422
// naming its field. Authorization: the read permission of the type.
func (h GameAdmin) Simulate(ctx context.Context, configType string, configID uuid.UUID, no int, in SimulationInput) (out SimulationView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.SimulateGameConfig")
	defer func() { done(err) }()
	if configType != simProgressionCurves && configType != simSeasonTracks {
		return out, problem.New(problem.NotFound, "")
	}
	if err := in.validate(configType); err != nil {
		return out, err
	}
	if configType == simProgressionCurves && (h.Simulation.MinuteRate < 1 || h.Simulation.DailyCap < 1) {
		return out, problem.New(problem.Internal, "the progression simulation is not configured")
	}
	err = h.reader(ctx, configType, func(ctx context.Context, t gameconfig.Type) error {
		r, err := h.Data.Version(ctx, t.ItemType, configID, no)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "")
		}
		out = SimulationView{ConfigType: configType, ConfigID: configID, Version: no, Points: []SimulationPoint{}, Thresholds: []SimulationThreshold{},
			Assumptions: SimulationAssumptions{DaysPerWeek: in.DaysPerWeek, Weeks: in.Weeks}}
		if configType == simProgressionCurves {
			return h.simulateCurve(r.Payload, in, &out)
		}
		return simulateTrack(r.Payload, in, &out)
	})
	return out, err
}

func (in SimulationInput) validate(configType string) error {
	if in.DaysPerWeek < 1 || in.DaysPerWeek > 7 {
		return problem.Invalid("days_per_week", problem.SubInvalid, "must be 1..7")
	}
	if in.Weeks < 1 || in.Weeks > 52 {
		return problem.Invalid("weeks", problem.SubInvalid, "must be 1..52")
	}
	if configType != simProgressionCurves {
		return nil
	}
	if in.MinutesPerDay == nil {
		return problem.Invalid("minutes_per_day", problem.SubRequired, "required for progression-curves")
	}
	if *in.MinutesPerDay < 1 || *in.MinutesPerDay > 240 {
		return problem.Invalid("minutes_per_day", problem.SubInvalid, "must be 1..240")
	}
	return nil
}

func (h GameAdmin) simulateCurve(payload []byte, in SimulationInput, out *SimulationView) error {
	c, err := progression.ParseCurve(payload)
	if err != nil {
		return simulationProblem(err)
	}
	tb, err := c.SimulationTable()
	if err != nil {
		return simulationProblem(err)
	}
	minutes, rate := *in.MinutesPerDay, h.Simulation.MinuteRate
	perDay := progression.DailyXP(minutes, rate, h.Simulation.DailyCap)
	out.Assumptions.PerPlayDay, out.Assumptions.MinutesPerDay, out.Assumptions.MinuteRate = perDay, &minutes, &rate
	out.Assumptions.DailyCap = h.Simulation.DailyCap
	for _, p := range gameconfig.SimulateWeeks(perDay, in.DaysPerWeek, in.Weeks, tb.LevelFor) {
		out.Points = append(out.Points, SimulationPoint{Week: p.Week, Level: p.Level, Total: p.Total})
	}
	for _, lv := range tb.Levels {
		var next int64
		if lv.XPRequired != nil {
			next = *lv.XPRequired
		}
		out.Thresholds = append(out.Thresholds, SimulationThreshold{Level: lv.Level, ToNext: next, Total: lv.Cumulative})
	}
	return nil
}

func simulateTrack(payload []byte, in SimulationInput, out *SimulationView) error {
	tr, err := season.Parse(payload)
	if err != nil {
		return simulationProblem(err)
	}
	if err := tr.Simulable(); err != nil {
		return simulationProblem(err)
	}
	perDay := int64(tr.Points.DailyCap)
	out.Assumptions.PerPlayDay, out.Assumptions.DailyCap = perDay, tr.Points.DailyCap
	for _, p := range gameconfig.SimulateWeeks(perDay, in.DaysPerWeek, in.Weeks, tr.LevelAt) {
		out.Points = append(out.Points, SimulationPoint{Week: p.Week, Level: p.Level, Total: p.Total})
	}
	for _, th := range tr.Thresholds() {
		out.Thresholds = append(out.Thresholds, SimulationThreshold{Level: th.Tier, ToNext: int64(th.ToNext), Total: int64(th.Total)})
	}
	return nil
}

// simulationProblem maps a payload that cannot be simulated to 422 VALIDATION_FAILED naming its field.
func simulationProblem(err error) error {
	var cf *progression.FieldError
	if errors.As(err, &cf) {
		return problem.Invalid(cf.Field, problem.SubInvalid, cf.Reason)
	}
	var sf *season.FieldError
	if errors.As(err, &sf) {
		return problem.Invalid(sf.Field, problem.SubInvalid, sf.Reason)
	}
	return problem.Invalid("payload", problem.SubInvalid, "the payload cannot be simulated")
}

// ConfigCount is one entry of GameConfigCounts.data.
type ConfigCount struct {
	ConfigType string `json:"config_type"`
	Status     string `json:"status"`
	Count      int    `json:"count"`
}

// ConfigCountsView is GameConfigCounts.
type ConfigCountsView struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Data        []ConfigCount `json:"data"`
}

// ConfigCounts is getGameConfigCounts (F079 T045): the configs of module game counted by type and the status of their
// newest version (GROUP BY in the query; no play data — F080). Authorization: game.liveops.read, game.analytics.read,
// any console write permission, review or publish.
func (h GameAdmin) ConfigCounts(ctx context.Context) (out ConfigCountsView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameConfigCounts")
	defer func() { done(err) }()
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return out, err
	}
	if err := gameadmin.RequireDashboard(ctx, p); err != nil {
		return out, err
	}
	types := gameconfig.Types()
	itemTypes := make([]string, len(types))
	for i, t := range types {
		itemTypes[i] = t.ItemType
	}
	err = h.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		rows, err := h.Data.ConfigCounts(ctx, itemTypes)
		if err != nil {
			return err
		}
		out = ConfigCountsView{GeneratedAt: h.now(), Data: make([]ConfigCount, 0, len(rows))}
		for _, r := range rows {
			if t, ok := gameconfig.ByItemType(r.ItemType); ok {
				out.Data = append(out.Data, ConfigCount{ConfigType: t.Name, Status: r.Status, Count: r.Count})
			}
		}
		return nil
	})
	return out, err
}
