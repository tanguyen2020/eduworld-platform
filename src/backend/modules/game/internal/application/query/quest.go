package query

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Quests answers the quest read routes of a student (F072): listQuests and getQuest. Read only: what the student may
// see is computed from the published quests assigned to the tenant, the grade and school subjects of the student, its
// progress and the server facts; nothing is written.
type Quests struct {
	Read    read.Session
	Content port.QuestContent
	World   port.WorldContent
	Cursors *page.Codec
	Reset   quest.Reset
	Now     func() time.Time
}

func (h Quests) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// QuestSummary is QuestSummary.
type QuestSummary struct {
	QuestID             uuid.UUID                `json:"quest_id"`
	QuestVersionID      uuid.UUID                `json:"quest_version_id"`
	Code                string                   `json:"code"`
	Name                quest.Text               `json:"name"`
	QuestType           string                   `json:"quest_type"`
	GradeLevels         []int                    `json:"grade_levels"`
	SubjectID           string                   `json:"subject_id,omitempty"`
	SubjectCode         string                   `json:"subject_code,omitempty"`
	AreaID              string                   `json:"area_id,omitempty"`
	GiverNPCID          string                   `json:"giver_npc_id,omitempty"`
	StoryChapterCode    string                   `json:"story_chapter_code,omitempty"`
	StartTrigger        string                   `json:"start_trigger"`
	Status              string                   `json:"status"`
	CanAccept           bool                     `json:"can_accept"`
	LockedReason        string                   `json:"locked_reason,omitempty"`
	MissingRequirements []quest.Requirement      `json:"missing_requirements,omitempty"`
	AvailableUntil      *time.Time               `json:"available_until,omitempty"`
	RetryAfter          *time.Time               `json:"retry_after,omitempty"`
	Progress            *questrules.ProgressView `json:"progress,omitempty"`
}

// StepView is QuestStepView (delivery: no content_ref, no pass_rule — ADR-0016, R6).
type StepView struct {
	StepID          string      `json:"step_id"`
	Order           int         `json:"order"`
	StepType        string      `json:"step_type"`
	Title           quest.Text  `json:"title"`
	Hint            *quest.Text `json:"hint,omitempty"`
	Optional        bool        `json:"optional"`
	DependsOn       []string    `json:"depends_on,omitempty"`
	AreaID          string      `json:"area_id,omitempty"`
	NPCID           string      `json:"npc_id,omitempty"`
	InteractableIDs []string    `json:"interactable_ids,omitempty"`
	Count           int         `json:"count,omitempty"`
}

// RewardPreview is one reward intent.
type RewardPreview struct {
	Kind                string `json:"kind"`
	Amount              *int   `json:"amount,omitempty"`
	Ref                 string `json:"ref,omitempty"`
	FirstCompletionOnly bool   `json:"first_completion_only,omitempty"`
}

// QuestDetail is QuestDetail.
type QuestDetail struct {
	QuestSummary
	Description      quest.Text           `json:"description"`
	Narrative        *quest.Text          `json:"narrative,omitempty"`
	ObjectiveIDs     []string             `json:"objective_ids"`
	EstimatedMinutes int                  `json:"estimated_minutes,omitempty"`
	Repeatability    string               `json:"repeatability"`
	Steps            []StepView           `json:"steps"`
	CompletionRule   quest.CompletionRule `json:"completion_rule"`
	RewardsPreview   []RewardPreview      `json:"rewards_preview"`
}

// ListInput is the filter of listQuests.
type ListInput struct {
	QuestType string
	Status    string
	AreaID    string
	Limit     int
	Cursor    string
}

// QuestPage is the listQuests response.
type QuestPage struct {
	Data []QuestSummary `json:"data"`
	Page PageInfo       `json:"page"`
}

// PageInfo is PageInfo.
type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

// entry is one quest of the student with its view version.
type entry struct {
	row     port.QuestRow
	summary QuestSummary
}

type questSnapshot struct {
	entries map[uuid.UUID]*entry
}

// load computes the quests of the student (F072-AC-01, AC-02, AC-24 and the edge cases): every active progress (on its
// pinned version, even when the quest was unassigned or the grade changed since) and every assigned quest visible for
// the current grade, subjects and availability (an auto daily/weekly quest only once granted).
func (h Quests) load(ctx context.Context, id, tenant uuid.UUID) (*questSnapshot, error) {
	s := &questSnapshot{entries: map[uuid.UUID]*entry{}}
	now := h.now()
	err := h.Read.Read(ctx, func(ctx context.Context) error {
		st, err := h.World.Student(ctx, tenant, id)
		if err != nil {
			return err
		}
		if st == nil || st.Deleted || st.Status != "active" {
			return nil
		}
		hist, err := h.Content.Histories(ctx, tenant, id)
		if err != nil {
			return err
		}
		for qid, hs := range hist {
			if hs.Latest == nil || !hs.Latest.Active() {
				continue
			}
			row, err := h.Content.QuestVersion(ctx, hs.Latest.VersionID)
			if err != nil {
				return err
			}
			if row == nil {
				continue
			}
			sum := summaryOf(*row)
			sum.Status = quest.StatusActive
			v := questrules.ViewOf(hs.Latest, row.Version)
			sum.Progress = &v
			s.entries[qid] = &entry{row: *row, summary: sum}
		}
		rows, err := h.Content.AssignedQuests(ctx, tenant, st.Grade)
		if err != nil {
			return err
		}
		f, err := questrules.Facts(ctx, h.World, h.Content, tenant, id, st, hist)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if s.entries[row.QuestID] != nil {
				continue
			}
			v := row.Version
			d := quest.Decide(v, questrules.Student(st), f, now)
			if !d.Visible {
				continue
			}
			hs := hist[row.QuestID]
			sum := summaryOf(row)
			if hs != nil && hs.Latest != nil {
				pv := questrules.ViewOf(hs.Latest, v)
				sum.Progress = &pv
			}
			rep, retry := quest.CanRepeat(v, questrules.History(hs, v, h.Reset, now), now)
			switch {
			case rep == quest.RepeatAlreadyCompleted:
				sum.Status, sum.LockedReason = quest.StatusCompleted, "already_completed"
			case v.AutoGranted() || !v.Acceptable():
				if hs == nil || hs.Latest == nil {
					continue // granted or assigned by the server only (auto_daily/auto_weekly; assignment, event: F041/F077)
				}
				sum.Status = hs.Latest.Status
			case rep == quest.RepeatCooldown:
				sum.Status, sum.LockedReason, sum.RetryAfter = quest.StatusCompleted, "cooldown", retry
			default:
				sum.Status, sum.CanAccept = "available", d.Met
				if !d.Met {
					sum.LockedReason, sum.MissingRequirements = "prerequisite_not_met", d.Missing()
				}
			}
			s.entries[row.QuestID] = &entry{row: row, summary: sum}
		}
		return nil
	})
	return s, err
}

func summaryOf(row port.QuestRow) QuestSummary {
	v := row.Version
	_, until := v.Window()
	return QuestSummary{QuestID: row.QuestID, QuestVersionID: row.VersionID, Code: v.Code, Name: v.Name, QuestType: v.QuestType,
		GradeLevels: v.GradeLevels, SubjectID: v.SubjectID, SubjectCode: v.SubjectCode, AreaID: v.AreaID, GiverNPCID: v.GiverNPCID,
		StoryChapterCode: v.StoryChapterCode, StartTrigger: v.StartTrigger.Kind, AvailableUntil: until}
}

// inArea reports a quest of area: its area, the area of its trigger or of one of its steps.
func inArea(v quest.Version, area string) bool {
	if v.AreaID == area || v.StartTrigger.AreaID == area {
		return true
	}
	for _, s := range v.Steps {
		if s.AreaID == area {
			return true
		}
	}
	return false
}

var statusRank = map[string]int{quest.StatusActive: 0, "available": 1, quest.StatusCompleted: 2}

func sortKey(s QuestSummary) string {
	r, ok := statusRank[s.Status]
	if !ok {
		r = 3
	}
	return fmt.Sprintf("%d:%s", r, s.Code)
}

// List implements listQuests (F072-AC-01): filters status, quest_type and area_id; keyset pages by (status, code).
func (h Quests) List(ctx context.Context, in ListInput) (out QuestPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListQuests")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	var cur *page.Cursor
	if h.Cursors != nil {
		if cur, err = h.Cursors.Decode(in.Cursor); err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
	}
	s, err := h.load(ctx, id, tenant)
	if err != nil {
		return out, err
	}
	var list []QuestSummary
	for _, e := range s.entries {
		sum := e.summary
		switch {
		case in.Status != "" && sum.Status != in.Status:
			continue
		case in.QuestType != "" && sum.QuestType != in.QuestType:
			continue
		case in.AreaID != "" && !inArea(e.row.Version, in.AreaID):
			continue
		}
		list = append(list, sum)
	}
	slices.SortFunc(list, func(a, b QuestSummary) int {
		if c := strings.Compare(sortKey(a), sortKey(b)); c != 0 {
			return c
		}
		return strings.Compare(a.QuestID.String(), b.QuestID.String())
	})
	if cur != nil {
		i := slices.IndexFunc(list, func(q QuestSummary) bool {
			k := sortKey(q)
			return k > cur.Key || (k == cur.Key && q.QuestID.String() > cur.ID.String())
		})
		if i < 0 {
			i = len(list)
		}
		list = list[i:]
	}
	out = QuestPage{Data: []QuestSummary{}, Page: PageInfo{Limit: limit}}
	if len(list) > limit {
		list, out.Page.HasMore = list[:limit], true
		if h.Cursors != nil {
			last := list[len(list)-1]
			next := h.Cursors.Encode(page.Cursor{Key: sortKey(last), ID: last.QuestID})
			out.Page.NextCursor = &next
		}
	}
	out.Data = append(out.Data, list...)
	return out, nil
}

// Get implements getQuest (F072-AC-02, AC-03, AC-24): the delivery of the pinned version while the student works on
// the quest, else of the current version; a quest not assigned to the tenant or not of the grade of the student is
// 404 (never revealed).
func (h Quests) Get(ctx context.Context, questID uuid.UUID) (out QuestDetail, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetQuest")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	s, err := h.load(ctx, id, tenant)
	if err != nil {
		return out, err
	}
	e := s.entries[questID]
	if e == nil {
		return out, problem.New(problem.NotFound, "quest not found")
	}
	return detailOf(e), nil
}

func detailOf(e *entry) QuestDetail {
	v := e.row.Version
	d := QuestDetail{QuestSummary: e.summary, Description: v.Description, Narrative: v.Narrative, ObjectiveIDs: v.ObjectiveIDs,
		EstimatedMinutes: v.EstimatedMinutes, Repeatability: v.RepeatMode(), CompletionRule: v.CompletionRule, Steps: []StepView{},
		RewardsPreview: []RewardPreview{}}
	if d.ObjectiveIDs == nil {
		d.ObjectiveIDs = []string{}
	}
	for _, s := range v.SortedSteps() {
		sv := StepView{StepID: s.ID, Order: s.Order, StepType: s.Type, Title: s.Title, Hint: s.Hint, Optional: s.Optional, DependsOn: v.Deps(s),
			AreaID: s.AreaID, NPCID: s.NPCID, InteractableIDs: s.Targets()}
		if s.Type == quest.StepCollect || s.Type == quest.StepDiscover {
			sv.Count = s.Target()
		}
		d.Steps = append(d.Steps, sv)
	}
	for _, r := range v.Rewards {
		p := RewardPreview{}
		p.Kind, _ = r["type"].(string)
		if a, ok := r["amount"].(float64); ok {
			n := int(a)
			p.Amount = &n
		}
		for _, k := range []string{"item_id", "badge_id", "currency_code", "title_code", "collection_item_id"} {
			if ref, ok := r[k].(string); ok && ref != "" {
				p.Ref = ref
			}
		}
		p.FirstCompletionOnly, _ = r["first_completion_only"].(bool)
		d.RewardsPreview = append(d.RewardsPreview, p)
	}
	return d
}
