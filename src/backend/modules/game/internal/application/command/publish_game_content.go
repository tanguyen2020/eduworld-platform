package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// EventContentPublished is the release event of game content (specs/events/payloads/game/content.published.v1).
const EventContentPublished = "content.published"

// publishOrder publishes areas before their levels and chapters before the dialogues that reference their quests.
// Quests (F072) come before the dialogues that give them.
var publishOrder = []string{write.ItemWorldArea, write.ItemLevel, write.ItemStoryChapter, write.ItemQuest, write.ItemNPCDialogue,
	write.ItemProgressionCurve, write.ItemRewardRule}

// ContentItem is one authored game content payload to publish.
type ContentItem struct {
	Type    string
	Payload json.RawMessage
}

// GameRelease is one published item.
type GameRelease struct {
	ItemType  string
	ItemID    string
	VersionID uuid.UUID
	VersionNo int
	ReleaseID uuid.UUID
	Snapshot  []byte
	Key       string
}

// GameContentResult counts the items published and the items left unchanged (identical to the current version).
type GameContentResult struct {
	Published int
	Unchanged int
	Releases  []GameRelease
}

// PublishGameContent is the internal release core of game content (F071 T010, ADR-0043, DATABASE_DESIGN §3.13): it
// checks the payloads (JSON Schema rules the server relies on, F071-AC-16, AC-18), inserts the new published versions,
// moves the pointers, appends game.content_releases, writes content.published to the outbox (the manifest builder
// rebuilds every tenant manifest, F071-AC-05) and an audit record, then caches the immutable snapshots. Platform scope
// only: the seed (cmd/seed) now, the Game Operations Console (F078) later. A payload identical to the current version
// is not published again (the seed can run any number of times).
type PublishGameContent struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.GameContentStore
	Content     port.WorldContent
	Events      write.EventPublisher
	Audit       write.Auditor
	Cache       port.SnapshotCache // nil: no snapshot cache
	// Quests are the quest checks (F072: JSON Schema + QuestPublishValidator); QuestContent reads the published quests.
	Quests       *QuestPublishRules
	QuestContent port.QuestContent
	// Progression are the F074 checks (progression curves, reward rules) and the level table store.
	Progression *ProgressionPublishRules
	Now         func() time.Time
}

type parsedItem struct {
	ContentItem
	id      string
	parent  map[string]any
	columns map[string]any
	doc     map[string]any // canonical authored content
	area    *world.Area
	chapter *world.Chapter
	level   *world.Level
	dlg     *world.Dialogue
	quest   *quest.Version
	curve   *progression.Curve
	rule    *reward.Rule
}

// Publish publishes items in one platform transaction; key makes the release rows idempotent per item.
func (h PublishGameContent) Publish(ctx context.Context, items []ContentItem, note, key string) (res GameContentResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PublishGameContent")
	defer func() { done(err) }()
	if s, ok := tenancy.ScopeFrom(ctx); !ok || !s.PlatformOnly() {
		return res, problem.New(problem.PermissionDenied, "game content is published under a platform scope")
	}
	if len(key) > 64 {
		return res, problem.Invalid("idempotency_key", problem.SubInvalid, "at most 64 characters")
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	parsed := make([]parsedItem, 0, len(items))
	for _, it := range items {
		var p parsedItem
		var err error
		switch {
		case questItem(it.Type):
			p, err = parseQuest(it, h.Quests)
		case progressionItem(it.Type):
			p, err = parseProgression(it, h.Progression, now)
		default:
			p, err = parse(it)
		}
		if err != nil {
			return res, err
		}
		parsed = append(parsed, p)
	}
	slices.SortStableFunc(parsed, func(a, b parsedItem) int {
		return slices.Index(publishOrder, a.Type) - slices.Index(publishOrder, b.Type)
	})
	by := uuid.Nil // system actor (seed); F078 passes the platform staff principal
	if p, ok := authz.PrincipalFrom(ctx); ok && p.UserID != uuid.Nil {
		by = p.UserID
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.checkRefs(ctx, parsed); err != nil {
			return err
		}
		res = GameContentResult{}
		for _, p := range parsed {
			rel, changed, err := h.publishOne(ctx, p, note, key, by, now)
			if err != nil {
				return err
			}
			if !changed {
				res.Unchanged++
				continue
			}
			res.Published++
			res.Releases = append(res.Releases, rel)
		}
		if res.Published == 0 {
			return nil
		}
		released := make([]map[string]any, 0, len(res.Releases))
		for _, r := range res.Releases {
			released = append(released, map[string]any{"item_type": r.ItemType, "item_id": r.ItemID, "version": r.VersionNo})
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "game_content.published", TargetType: "game_content", TargetID: key,
			Result: audit.ResultSuccess, Metadata: map[string]any{"published": res.Published, "unchanged": res.Unchanged, "items": released}}); err != nil {
			return err
		}
		if h.AfterCommit != nil && h.Cache != nil {
			releases := res.Releases
			_ = h.AfterCommit(ctx, func(ctx context.Context) {
				for _, r := range releases {
					_ = h.Cache.PutSnapshot(ctx, r.Key, r.Snapshot)
					_ = h.Cache.SetCurrent(ctx, r.ItemType, r.ItemID, r.VersionID)
				}
			})
		}
		return nil
	})
	return res, err
}

func parse(it ContentItem) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	var err error
	switch it.Type {
	case write.ItemWorldArea:
		var a world.Area
		if a, err = world.ParseArea(it.Payload); err == nil {
			p.area, p.id = &a, a.AreaID
			p.columns = map[string]any{"region_code": a.RegionCode, "area_class": a.Class, "subject_code": a.SubjectCode}
		}
	case write.ItemStoryChapter:
		var c world.Chapter
		if c, err = world.ParseChapter(it.Payload); err == nil {
			p.chapter, p.id = &c, c.Code
			p.columns = map[string]any{"region_code": c.RegionCode, "order_no": c.Order, "subject_code": c.SubjectCode}
		}
	case write.ItemLevel:
		var l world.Level
		if l, err = world.ParseLevel(it.Payload); err == nil {
			p.level, p.id = &l, l.LevelID
			p.parent = map[string]any{"area_id": l.AreaID}
			p.columns = map[string]any{"bundle_id": l.BundleID, "bundle_url": l.BundleURL, "bundle_sha256": l.BundleSHA256, "size_bytes": l.SizeBytes}
		}
	case write.ItemNPCDialogue:
		var d world.Dialogue
		if d, err = world.ParseDialogue(it.Payload); err == nil {
			p.dlg, p.id, p.doc = &d, d.DialogueID, d.Doc
			grades := d.GradeLevels
			if grades == nil {
				grades = []int{}
			}
			p.parent = map[string]any{"npc_id": d.NPCID}
			p.columns = map[string]any{"npc_id": d.NPCID, "grade_levels": grades}
		}
	default:
		return p, problem.Invalid("item_type", problem.SubInvalid, fmt.Sprintf("unknown game content type %q", it.Type))
	}
	switch {
	case errors.Is(err, world.ErrTranslationIncomplete):
		return p, problem.New(problem.TranslationIncomplete, err.Error())
	case err != nil:
		return p, problem.Invalid("payload", problem.SubInvalid, err.Error())
	}
	if p.doc == nil {
		if err := json.Unmarshal(it.Payload, &p.doc); err != nil {
			return p, problem.Invalid("payload", problem.SubInvalid, err.Error())
		}
	}
	return p, nil
}

// checkRefs checks the references between items against the batch and the published content (F071-AC-18): a level of
// a known area, a portal to a known area, a dialogue of an NPC placed in an area and of known quests, one published
// dialogue per NPC and grade.
func (h PublishGameContent) checkRefs(ctx context.Context, items []parsedItem) error {
	areas, err := h.Content.CurrentAreas(ctx)
	if err != nil {
		return err
	}
	chapters, err := h.Content.CurrentChapters(ctx)
	if err != nil {
		return err
	}
	known, npcs, quests := map[string]bool{}, map[string]bool{}, map[string]bool{}
	addArea := func(a world.Area) {
		known[a.AreaID] = true
		for _, n := range a.NPCs() {
			npcs[n] = true
		}
	}
	for _, r := range areas {
		addArea(r.Area)
	}
	for _, r := range chapters {
		for _, q := range r.Chapter.MainQuestIDs {
			quests[q] = true
		}
	}
	for _, p := range items {
		if p.area != nil {
			addArea(*p.area)
		}
		if p.chapter != nil {
			for _, q := range p.chapter.MainQuestIDs {
				quests[q] = true
			}
		}
		if p.quest != nil {
			quests[p.quest.QuestID] = true
		}
	}
	if h.QuestContent != nil {
		current, err := h.QuestContent.CurrentQuests(ctx)
		if err != nil {
			return err
		}
		for _, r := range current {
			quests[r.QuestID.String()] = true
		}
	}
	if err := h.checkQuests(ctx, items, areas, chapters); err != nil {
		return err
	}
	if err := h.checkCurve(ctx, items); err != nil {
		return err
	}
	grades := map[string][]struct {
		id     string
		grades []int
	}{}
	for _, p := range items {
		switch {
		case p.area != nil:
			for _, pt := range p.area.Layout.Portals {
				if !known[pt.TargetAreaID] {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("portal %s targets unknown area %s", pt.ID, pt.TargetAreaID))
				}
			}
		case p.level != nil:
			if !known[p.level.AreaID] {
				return problem.New(problem.ItemRefInvalid, fmt.Sprintf("level %s of unknown area %s", p.level.LevelID, p.level.AreaID))
			}
		case p.dlg != nil:
			if !npcs[p.dlg.NPCID] {
				return problem.New(problem.ItemRefInvalid, fmt.Sprintf("dialogue %s: npc %s is placed in no area", p.dlg.DialogueID, p.dlg.NPCID))
			}
			for _, q := range p.dlg.QuestRefs {
				if !quests[q] {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("dialogue %s: unknown quest %s", p.dlg.DialogueID, q))
				}
			}
			grades[p.dlg.NPCID] = append(grades[p.dlg.NPCID], struct {
				id     string
				grades []int
			}{p.dlg.DialogueID, p.dlg.GradeLevels})
		}
	}
	for npc, batch := range grades {
		current, err := h.Content.CurrentDialogues(ctx, npc)
		if err != nil {
			return err
		}
		for i, d := range batch {
			for _, o := range batch[:i] {
				if world.Overlaps(d.grades, o.grades) {
					return problem.Invalid("grade_levels", problem.SubInvalid, fmt.Sprintf("npc %s: two dialogues serve one grade", npc))
				}
			}
			for _, c := range current {
				if c.DialogueID.String() != d.id && world.Overlaps(d.grades, c.Grades) && !inBatch(batch, c.DialogueID.String()) {
					return problem.Invalid("grade_levels", problem.SubInvalid, fmt.Sprintf("npc %s: dialogue %s already serves these grades", npc, c.DialogueID))
				}
			}
		}
	}
	return nil
}

func inBatch(batch []struct {
	id     string
	grades []int
}, id string) bool {
	for _, b := range batch {
		if b.id == id {
			return true
		}
	}
	return false
}

func (h PublishGameContent) publishOne(ctx context.Context, p parsedItem, note, key string, by uuid.UUID, now time.Time) (GameRelease, bool, error) {
	canonical, err := json.Marshal(p.doc) // map keys sorted: stable hash of the authored content
	if err != nil {
		return GameRelease{}, false, err
	}
	sum := sha256.Sum256(canonical)
	sha := hex.EncodeToString(sum[:])
	head, err := h.Store.Head(ctx, p.Type, p.id, p.parent, now)
	if err != nil {
		return GameRelease{}, false, err
	}
	if head.CurrentSHA256 == sha {
		return GameRelease{}, false, nil
	}
	vid := ids.New()
	stored := canonical
	if p.quest != nil {
		if stored, err = questStored(p.doc, vid, head.NextVersionNo); err != nil {
			return GameRelease{}, false, err
		}
	}
	if p.curve != nil || p.rule != nil {
		if stored, err = h.progressionStored(ctx, p, vid, head.NextVersionNo); err != nil {
			return GameRelease{}, false, err
		}
	}
	if p.Type == write.ItemNPCDialogue {
		doc := map[string]any{}
		for k, v := range p.doc {
			doc[k] = v
		}
		doc["dialogue_version_id"], doc["version"], doc["status"] = vid.String(), head.NextVersionNo, "published"
		if stored, err = json.Marshal(doc); err != nil {
			return GameRelease{}, false, err
		}
	}
	snap := sha256.Sum256(stored)
	snapSHA := hex.EncodeToString(snap[:])
	snapKey := fmt.Sprintf("ew:game:cv:%s:%s:%d", p.Type, p.id, head.NextVersionNo)
	if err := h.Store.Publish(ctx, write.ContentVersion{ItemType: p.Type, ItemID: p.id, VersionID: vid, VersionNo: head.NextVersionNo,
		Payload: stored, SHA256: sha, Note: note, SnapshotKey: snapKey, SnapshotSHA256: snapSHA, At: now, By: by, Columns: p.columns}); err != nil {
		return GameRelease{}, false, err
	}
	if p.curve != nil {
		if err := h.insertLevels(ctx, p, vid, head.NextVersionNo); err != nil {
			return GameRelease{}, false, err
		}
	}
	rid := ids.New()
	idem := key
	if idem == "" {
		idem = rid.String()
	}
	if err := h.Store.Release(ctx, write.ContentRelease{ReleaseID: rid, ItemType: p.Type, ItemID: p.id, Action: "publish", From: head.CurrentVersionID,
		To: &vid, Reason: note, By: by, At: now, IdempotencyKey: idem}); err != nil {
		return GameRelease{}, false, err
	}
	payload := map[string]any{"item_type": p.Type, "item_id": p.id, "from_version_id": head.CurrentVersionID, "to_version_id": vid,
		"to_version": head.NextVersionNo, "release_id": rid, "owner_scope": "platform", "tenant_id": nil, "scheduled": false, "published_by": by,
		"snapshot_sha256": snapSHA, "released_at": now}
	if p.area != nil {
		payload["world_area"] = runtimeArea(*p.area)
	}
	if err := h.Events.Publish(ctx, write.Event{Type: EventContentPublished, SubjectType: p.Type, SubjectID: p.id, Platform: true, OccurredAt: now,
		Payload: payload}); err != nil {
		return GameRelease{}, false, err
	}
	return GameRelease{ItemType: p.Type, ItemID: p.id, VersionID: vid, VersionNo: head.NextVersionNo, ReleaseID: rid, Snapshot: stored,
		Key: snapKey}, true, nil
}

// runtimeArea is the part of an area the room host of the hub needs (projection ref.world_areas of module competition).
func runtimeArea(a world.Area) map[string]any {
	spawns := map[string]world.Vec3{}
	for _, s := range a.Layout.SpawnPoints {
		spawns[s.ID] = s.Position
	}
	return map[string]any{"area_id": a.AreaID, "capacity": a.Capacity, "max_speed_mps": a.MaxSpeedMPS, "bounds": a.Bounds, "spawn_points": spawns,
		"allowed_grade_bands": a.GradeBands(), "requires_unlock": !a.IsLearning() && a.UnlockRule != nil, "status": "active"}
}
