package query

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
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
	// List reads the quest list of a student page by page in SQL (API_DESIGN §5).
	Pages   read.QuestListReadModel
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

// questPage is one page of the quests of a student and whether the list goes on after it.
type questPage struct {
	entries []*entry
	last    *read.QuestListRow // the last row read (the keyset of the next page)
	more    bool
}

// page reads one page of the quests of the student (F072-AC-01, AC-02, AC-24 and the edge cases) in SQL — every active
// progress (on its pinned version, even when the quest was unassigned or the grade changed since) and every assigned
// quest visible for the current grade, subjects and availability (an auto daily/weekly quest only once granted),
// filtered, ordered by (status rank, code, quest_id) and cut by keyset + LIMIT (API_DESIGN §5) — then builds the
// summaries of that page only with the domain rules (quest.Decide, quest.CanRepeat), which stay the source of truth.
func (h Quests) page(ctx context.Context, id, tenant uuid.UUID, in read.QuestListQuery) (*questPage, error) {
	out := &questPage{}
	now := h.now()
	err := h.Read.Read(ctx, func(ctx context.Context) error {
		st, err := h.World.Student(ctx, tenant, id)
		if err != nil {
			return err
		}
		if st == nil || st.Deleted || st.Status != "active" {
			return nil
		}
		disc, err := h.Content.Discovered(ctx, tenant, id)
		if err != nil {
			return err
		}
		q := in
		q.Tenant, q.Student, q.Grade, q.EnabledSubjects, q.Now = tenant, id, st.Grade, st.EnabledSubjects, now
		q.DailyKey, q.WeeklyKey = h.Reset.PeriodKey(quest.RecurrenceDaily, now), h.Reset.PeriodKey(quest.RecurrenceWeekly, now)
		q.Discovered = make([]string, 0, len(disc))
		for d := range disc {
			q.Discovered = append(q.Discovered, d)
		}
		slices.Sort(q.Discovered)
		q.Limit = in.Limit + 1
		rows, err := h.Pages.Page(ctx, q)
		if err != nil {
			return err
		}
		if len(rows) > in.Limit {
			rows, out.more = rows[:in.Limit], true
		}
		if len(rows) == 0 {
			return nil
		}
		out.last = &rows[len(rows)-1]
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.QuestID
		}
		hist, err := h.Pages.Histories(ctx, tenant, id, ids, []string{q.DailyKey, q.WeeklyKey})
		if err != nil {
			return err
		}
		var f *quest.Facts
		for _, r := range rows {
			hs := hist[r.QuestID]
			if hs != nil && hs.Latest != nil && hs.Latest.Active() && hs.Latest.VersionID == r.VersionID {
				sum := summaryOf(r.QuestRow)
				sum.Status = quest.StatusActive
				v := questrules.ViewOf(hs.Latest, r.Version)
				sum.Progress = &v
				out.entries = append(out.entries, &entry{row: r.QuestRow, summary: sum})
				continue
			}
			if f == nil {
				completed, err := h.Pages.CompletedQuests(ctx, tenant, id)
				if err != nil {
					return err
				}
				ff, err := questrules.Facts(ctx, h.World, h.Content, tenant, id, st, completed)
				if err != nil {
					return err
				}
				f = &ff
			}
			sum, ok := h.summarize(r.QuestRow, hs, st, *f, now)
			if !ok || sum.Status != r.Status {
				// the SQL form of quest.Decide/CanRepeat and the domain disagree: the domain answers, the page keeps its keyset
				slog.WarnContext(ctx, "game.query.ListQuests: SQL status differs from the domain rules", "quest_id", r.QuestID,
					"sql_status", r.Status, "domain_status", sum.Status, "listed", ok)
				if !ok {
					continue
				}
			}
			out.entries = append(out.entries, &entry{row: r.QuestRow, summary: sum})
		}
		return nil
	})
	return out, err
}

// summarize is the summary of an assigned quest the student has no active progress on (F072-AC-01, AC-25): visible by
// quest.Decide, status by quest.CanRepeat and the start trigger; ok is false for a quest the student does not see.
func (h Quests) summarize(row port.QuestRow, hs *port.QuestHistory, st *port.WorldStudent, f quest.Facts, now time.Time) (QuestSummary, bool) {
	v := row.Version
	d := quest.Decide(v, questrules.Student(st), f, now)
	if !d.Visible {
		return QuestSummary{}, false
	}
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
			return QuestSummary{}, false // granted or assigned by the server only (auto_daily/auto_weekly; assignment, event: F041/F077)
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
	return sum, true
}

func summaryOf(row port.QuestRow) QuestSummary {
	v := row.Version
	_, until := v.Window()
	return QuestSummary{QuestID: row.QuestID, QuestVersionID: row.VersionID, Code: v.Code, Name: v.Name, QuestType: v.QuestType,
		GradeLevels: v.GradeLevels, SubjectID: v.SubjectID, SubjectCode: v.SubjectCode, AreaID: v.AreaID, GiverNPCID: v.GiverNPCID,
		StoryChapterCode: v.StoryChapterCode, StartTrigger: v.StartTrigger.Kind, AvailableUntil: until}
}

// cursorKey is the keyset of a row as the opaque cursor stores it: "<status rank>:<code>".
func cursorKey(r read.QuestListRow) string { return strconv.Itoa(r.Rank) + ":" + r.Version.Code }

// List implements listQuests (F072-AC-01): filters status, quest_type and area_id; keyset pages by (status rank, code,
// quest_id) in SQL.
func (h Quests) List(ctx context.Context, in ListInput) (out QuestPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListQuests")
	defer func() { done(err) }()
	id, tenant, err := student(ctx)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	q := read.QuestListQuery{QuestType: in.QuestType, Status: in.Status, AreaID: in.AreaID, Limit: limit}
	if h.Cursors != nil {
		cur, err := h.Cursors.Decode(in.Cursor)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		if cur != nil {
			r, code, ok := strings.Cut(cur.Key, ":")
			n, err := strconv.Atoi(r)
			if !ok || err != nil {
				return out, problem.New(problem.BadRequest, "invalid cursor")
			}
			q.After = &read.QuestKey{Rank: n, Code: code, QuestID: cur.ID}
		}
	}
	p, err := h.page(ctx, id, tenant, q)
	if err != nil {
		return out, err
	}
	out = QuestPage{Data: make([]QuestSummary, 0, len(p.entries)), Page: PageInfo{Limit: limit}}
	for _, e := range p.entries {
		out.Data = append(out.Data, e.summary)
	}
	if p.more {
		out.Page.HasMore = true
		if h.Cursors != nil {
			next := h.Cursors.Encode(page.Cursor{Key: cursorKey(*p.last), ID: p.last.QuestID})
			out.Page.NextCursor = &next
		}
	}
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
	p, err := h.page(ctx, id, tenant, read.QuestListQuery{QuestID: &questID, Limit: 1})
	if err != nil {
		return out, err
	}
	if len(p.entries) == 0 {
		return out, problem.New(problem.NotFound, "quest not found")
	}
	return detailOf(p.entries[0]), nil
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
