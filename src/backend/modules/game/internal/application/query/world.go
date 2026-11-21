package query

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// World answers the world read routes of a student (F071): listWorldAreas, getWorldManifest, getWorldExploration and
// getNpcDialogue. Read only (READ ONLY session): an area unlocked lazily by its rule is reported accessible, the unlock
// row itself is written by the consumer or enterWorldArea.
type World struct {
	Read      read.Session
	Content   port.WorldContent
	Manifests read.ManifestReadModel
}

// AreaView is WorldArea.
type AreaView struct {
	AreaID             string              `json:"area_id"`
	SceneRef           string              `json:"scene_ref"`
	BundleID           string              `json:"bundle_id"`
	Capacity           int                 `json:"capacity"`
	Accessible         bool                `json:"accessible"`
	LockReason         string              `json:"lock_reason,omitempty"`
	Name               world.Name          `json:"name"`
	RegionCode         string              `json:"region_code"`
	RegionName         *world.Name         `json:"region_name,omitempty"`
	AreaClass          string              `json:"area_class"`
	SubjectCode        string              `json:"subject_code,omitempty"`
	RoomType           string              `json:"room_type"`
	Version            int                 `json:"version"`
	Map                *world.MapPos       `json:"map,omitempty"`
	Portals            []world.Portal      `json:"portals"`
	UnlockRequirements []world.Requirement `json:"unlock_requirements,omitempty"`
}

// snapshot is everything the read routes compute from.
type snapshot struct {
	tenant   uuid.UUID
	student  *port.WorldStudent
	areas    []port.AreaRow
	chapters []port.ChapterRow
	progress *port.WorldProgress
	levels   []port.LevelRow
	manifest int64
}

func student(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return p.UserID, *p.TenantID, nil
}

func (h World) load(ctx context.Context, withLevels bool) (*snapshot, error) {
	id, tenant, err := student(ctx)
	if err != nil {
		return nil, err
	}
	s := &snapshot{tenant: tenant}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		if s.student, err = h.Content.Student(ctx, tenant, id); err != nil {
			return err
		}
		if s.student == nil || s.student.Deleted {
			return problem.New(problem.NotFound, "student not found")
		}
		if s.areas, err = h.Content.CurrentAreas(ctx); err != nil {
			return err
		}
		if s.chapters, err = h.Content.CurrentChapters(ctx); err != nil {
			return err
		}
		if s.progress, err = h.Content.Progress(ctx, tenant, id, s.chapters); err != nil {
			return err
		}
		if withLevels {
			if s.levels, err = h.Content.CurrentLevels(ctx); err != nil {
				return err
			}
			if h.Manifests != nil {
				m, err := h.Manifests.Latest(ctx, tenant)
				if err != nil {
					return err
				}
				if m != nil {
					s.manifest = m.Version
				}
			}
		}
		return nil
	})
	return s, err
}

func (s *snapshot) views() []AreaView {
	out := []AreaView{}
	visible := map[string]bool{}
	for _, a := range s.areas {
		_, unlocked := s.progress.Unlocks[a.Area.AreaID]
		visible[a.Area.AreaID] = world.Decide(a.Area, s.student.Domain(), unlocked, s.progress.Facts).Visible
	}
	for _, a := range s.areas {
		_, unlocked := s.progress.Unlocks[a.Area.AreaID]
		d := world.Decide(a.Area, s.student.Domain(), unlocked, s.progress.Facts)
		if !d.Visible {
			continue // F071-AC-03: a subject the school turned off is hidden
		}
		v := AreaView{AreaID: a.Area.AreaID, SceneRef: a.Area.SceneRef, BundleID: a.Area.BundleID, Capacity: a.Area.Capacity,
			Accessible: d.Accessible, LockReason: d.LockReason, Name: a.Area.Name, RegionCode: a.Area.RegionCode, RegionName: a.Area.RegionName,
			AreaClass: a.Area.Class, SubjectCode: a.Area.Subject(), RoomType: a.Area.Access.RoomType, Version: a.VersionNo, Map: a.Area.Map,
			Portals: []world.Portal{}}
		for _, p := range a.Area.Layout.Portals {
			if visible[p.TargetAreaID] { // a gate to a hidden subject is not shown either
				v.Portals = append(v.Portals, p)
			}
		}
		if !d.Accessible && len(d.Requirements) > 0 {
			v.UnlockRequirements = d.Requirements
		}
		out = append(out, v)
	}
	return out
}

// ListAreas implements listWorldAreas (F071-AC-01…AC-03).
func (h World) ListAreas(ctx context.Context) (out []AreaView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListWorldAreas")
	defer func() { done(err) }()
	s, err := h.load(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.views(), nil
}

// ManifestView is WorldManifest.
type ManifestView struct {
	ManifestVersion string           `json:"manifest_version"`
	Areas           []AreaView       `json:"areas"`
	Bundles         []map[string]any `json:"bundles"`
}

// Manifest implements getWorldManifest (F071-AC-04): the visible areas with their scene and the bundle of their level
// (url, sha256, size_bytes) by the current versions — renaming or moving an area is data, no client build.
func (h World) Manifest(ctx context.Context) (out ManifestView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetWorldManifest")
	defer func() { done(err) }()
	s, err := h.load(ctx, true)
	if err != nil {
		return out, err
	}
	out = ManifestView{ManifestVersion: strconv.FormatInt(s.manifest, 10), Areas: s.views(), Bundles: []map[string]any{}}
	visible := map[string]bool{}
	for _, a := range out.Areas {
		visible[a.AreaID] = true
	}
	for _, l := range s.levels {
		if !visible[l.Level.AreaID] {
			continue
		}
		out.Bundles = append(out.Bundles, map[string]any{"bundle_id": l.Level.BundleID, "version": strconv.Itoa(l.VersionNo),
			"url": l.Level.BundleURL, "sha256": l.Level.BundleSHA256, "size_bytes": l.Level.SizeBytes})
	}
	return out, nil
}

// ExplorationView is one AreaExploration.
type ExplorationView struct {
	AreaID             string              `json:"area_id"`
	Name               world.Name          `json:"name"`
	RegionCode         string              `json:"region_code"`
	Unlocked           bool                `json:"unlocked"`
	UnlockedAt         *time.Time          `json:"unlocked_at,omitempty"`
	CompletionPct      float64             `json:"completion_pct"`
	DiscoveriesFound   int                 `json:"discoveries_found"`
	DiscoveriesTotal   int                 `json:"discoveries_total"`
	CollectiblesFound  int                 `json:"collectibles_found"`
	CollectiblesTotal  int                 `json:"collectibles_total"`
	CompletedAt        *time.Time          `json:"completed_at,omitempty"`
	UnlockRequirements []world.Requirement `json:"unlock_requirements,omitempty"`
}

// ChapterView is StoryChapterProgress.
type ChapterView struct {
	ChapterCode         string     `json:"chapter_code"`
	RegionCode          string     `json:"region_code"`
	Title               world.Name `json:"title"`
	Order               int        `json:"order"`
	SubjectCode         string     `json:"subject_code,omitempty"`
	Status              string     `json:"status"`
	MainQuestsCompleted int        `json:"main_quests_completed"`
	MainQuestsTotal     int        `json:"main_quests_total"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// Exploration is the response of getWorldExploration.
type Exploration struct {
	Areas    []ExplorationView `json:"areas"`
	Chapters []ChapterView     `json:"chapters"`
}

// Exploration implements getWorldExploration (F071-AC-23, AC-26): per visible area the exploration against the
// current area version (never above 100 %) and the missing requirements; the chapters with their status (a chapter of
// a subject the school turned off is hidden, F071-AC-22).
func (h World) Exploration(ctx context.Context) (out Exploration, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetWorldExploration")
	defer func() { done(err) }()
	s, err := h.load(ctx, false)
	if err != nil {
		return out, err
	}
	out = Exploration{Areas: []ExplorationView{}, Chapters: []ChapterView{}}
	for _, a := range s.areas {
		_, unlocked := s.progress.Unlocks[a.Area.AreaID]
		d := world.Decide(a.Area, s.student.Domain(), unlocked, s.progress.Facts)
		if !d.Visible {
			continue
		}
		v := ExplorationView{AreaID: a.Area.AreaID, Name: a.Area.Name, RegionCode: a.Area.RegionCode, Unlocked: d.Accessible}
		if at, ok := s.progress.Unlocks[a.Area.AreaID]; ok {
			v.UnlockedAt = &at
		}
		var e world.Exploration
		if row := s.progress.Exploration[a.Area.AreaID]; row != nil {
			e, v.CompletedAt = row.Exploration, row.CompletedAt
		}
		c := e.Completion(a.Area)
		v.CompletionPct, v.DiscoveriesFound, v.DiscoveriesTotal, v.CollectiblesFound, v.CollectiblesTotal =
			c.Pct, c.DiscoveriesFound, c.DiscoveriesTotal, c.CollectiblesFound, c.CollectiblesTotal
		if !d.Accessible {
			v.UnlockRequirements = world.Missing(d.Requirements)
		}
		out.Areas = append(out.Areas, v)
	}
	state := s.progress.StoryState()
	dom := s.student.Domain()
	for _, c := range s.chapters {
		if !dom.SubjectEnabled(c.Chapter.Subject()) {
			continue
		}
		p := s.progress.Story[c.Chapter.Code]
		v := ChapterView{ChapterCode: c.Chapter.Code, RegionCode: c.Chapter.RegionCode, Title: c.Chapter.Title, Order: c.Chapter.Order,
			SubjectCode: c.Chapter.Subject(), Status: c.Chapter.Status(p, state), MainQuestsCompleted: c.Chapter.MainQuestsDone(p),
			MainQuestsTotal: len(c.Chapter.MainQuestIDs)}
		if at, ok := s.progress.StoryDone[c.Chapter.Code]; ok {
			v.CompletedAt = &at
		}
		out.Chapters = append(out.Chapters, v)
	}
	return out, nil
}

// Dialogue is the response of getNpcDialogue with its ETag (the dialogue version id).
type Dialogue struct {
	ETag    string
	Payload map[string]any
}

// NpcDialogue implements getNpcDialogue (F071-AC-17): the published dialogue of the NPC for the grade of the student;
// an unknown NPC, or none for the grade, is 404.
func (h World) NpcDialogue(ctx context.Context, npcID string) (out Dialogue, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetNpcDialogue")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		st, err := h.Content.Student(ctx, tenant, id)
		if err != nil {
			return err
		}
		if st == nil || st.Deleted {
			return problem.New(problem.NotFound, "student not found")
		}
		rows, err := h.Content.CurrentDialogues(ctx, npcID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if len(r.Grades) > 0 && !contains(r.Grades, st.Grade) {
				continue
			}
			if err := json.Unmarshal(r.Payload, &out.Payload); err != nil {
				return err
			}
			out.ETag = r.VersionID.String()
			return nil
		}
		return problem.New(problem.NotFound, "no dialogue for this NPC and grade")
	})
	return out, err
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
