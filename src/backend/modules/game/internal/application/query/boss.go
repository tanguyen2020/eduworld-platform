package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Bosses answers the boss read routes of a student (F103): listBosses and getBossEncounter. Read only.
type Bosses struct {
	Read    read.Session
	Content port.BossContent
	Reads   port.BossReads
	World   port.WorldContent
	Cursors *page.Codec
	Now     func() time.Time
}

func (h Bosses) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// BossSummary is BossSummary of game.v1.yaml.
type BossSummary struct {
	BossID         uuid.UUID `json:"boss_id"`
	Code           string    `json:"code"`
	Title          boss.Text `json:"title"`
	AreaID         string    `json:"area_id,omitempty"`
	SubjectID      string    `json:"subject_id,omitempty"`
	PhaseCount     int       `json:"phase_count"`
	Modes          []string  `json:"modes"`
	PartySizeMin   *int      `json:"party_size_min,omitempty"`
	PartySizeMax   *int      `json:"party_size_max,omitempty"`
	DefeatedBefore bool      `json:"defeated_before"`
}

// BossPage is the page of listBosses.
type BossPage struct {
	Data []BossSummary `json:"data"`
	Page PageInfo      `json:"page"`
}

// BossListInput are the parameters of listBosses.
type BossListInput struct {
	AreaID string
	Limit  int
	Cursor string
}

func (h Bosses) student(ctx context.Context) (uuid.UUID, uuid.UUID, *port.WorldStudent, error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	st, err := h.World.Student(ctx, *p.TenantID, p.UserID)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return uuid.Nil, uuid.Nil, nil, problem.New(problem.NotFound, "student not found")
	}
	return p.UserID, *p.TenantID, st, nil
}

// ListBosses implements listBosses (F103-AC-01): the bosses assigned to the tenant whose current version serves the grade
// of the student (of the area given), keyset on boss_id with LIMIT n+1 in the database.
func (h Bosses) ListBosses(ctx context.Context, in BossListInput) (out BossPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListBosses")
	defer func() { done(err) }()
	if len(in.AreaID) > 64 {
		return out, problem.Invalid("area_id", problem.SubInvalid, "at most 64 characters")
	}
	limit, err := limitOf(in.Limit)
	if err != nil {
		return out, err
	}
	var after *uuid.UUID
	if in.Cursor != "" {
		if len(in.Cursor) > 512 || h.Cursors == nil {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		c, err := h.Cursors.Decode(in.Cursor)
		if err != nil || c == nil || c.Key != "boss" {
			return out, problem.Invalid("cursor", problem.SubInvalid, "malformed cursor")
		}
		after = &c.ID
	}
	out = BossPage{Data: []BossSummary{}, Page: PageInfo{Limit: limit}}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		id, tenant, st, err := h.student(ctx)
		if err != nil {
			return err
		}
		rows, err := h.Content.AvailableBosses(ctx, port.BossFilter{Tenant: tenant, Grade: st.Grade, AreaID: in.AreaID, After: after, Limit: limit + 1})
		if err != nil {
			return err
		}
		for i, row := range rows {
			if i == limit {
				out.Page.HasMore = true
				if h.Cursors != nil {
					c := h.Cursors.Encode(page.Cursor{Key: "boss", ID: out.Data[len(out.Data)-1].BossID})
					out.Page.NextCursor = &c
				}
				break
			}
			use, err := h.Reads.Usage(ctx, tenant, id, row.BossID, "")
			if err != nil {
				return err
			}
			out.Data = append(out.Data, bossSummaryOf(row, use.Defeated))
		}
		return nil
	})
	return out, err
}

func bossSummaryOf(row port.BossRow, defeated bool) BossSummary {
	v := row.Version
	s := BossSummary{BossID: row.BossID, Code: v.Code, Title: v.Name, AreaID: v.AreaID, SubjectID: v.SubjectID, PhaseCount: len(v.Phases),
		Modes: []string{}, DefeatedBefore: defeated}
	if v.Solo() {
		s.Modes = append(s.Modes, "solo")
	}
	if v.Party() {
		s.Modes = append(s.Modes, "party")
		lo, hi := v.SizeMin(), v.SizeMax()
		s.PartySizeMin, s.PartySizeMax = &lo, &hi
	}
	return s
}

// GetBossEncounter implements getBossEncounter (F103-AC-11, AC-27): the full state for a member (present or not),
// including its own questions of the current round (my_questions, render only) — a member back from another device
// gets everything again; another student or tenant gets 404.
func (h Bosses) GetBossEncounter(ctx context.Context, id uuid.UUID) (out partyrules.EncounterView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetBossEncounter")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	tenant, me := *p.TenantID, p.UserID
	now := h.now()
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		e, err := h.Reads.Encounter(ctx, tenant, id)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "encounter not found")
		}
		if _, ok := e.Member(me); !ok {
			return problem.New(problem.NotFound, "encounter not found")
		}
		row, err := h.Content.BossVersion(ctx, e.VersionID)
		if err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "boss version not found")
		}
		in := partyrules.ViewInput{E: e, Row: *row, Me: me, Now: now, Questions: true}
		if e.Status == boss.StatusPhaseActive {
			if in.Plans, err = h.Reads.Plans(ctx, tenant, e.ID, e.PhaseID, e.Retries); err != nil {
				return err
			}
			correct, err := h.Reads.Round(ctx, tenant, e.ID, e.PhaseID, e.Retries)
			if err != nil {
				return err
			}
			items := map[uuid.UUID]map[string]int{}
			for _, pl := range in.Plans {
				if items[pl.StudentID] == nil {
					items[pl.StudentID] = map[string]int{}
				}
				items[pl.StudentID][pl.RequirementID] = len(pl.Items)
			}
			in.Round = boss.Round{Items: items, Correct: correct}
			if in.Answered, err = h.Reads.Answered(ctx, tenant, e.ID, me); err != nil {
				return err
			}
		}
		ids := make([]uuid.UUID, len(e.Members))
		for i, m := range e.Members {
			ids[i] = m.StudentID
		}
		if in.Names, err = h.Reads.Nicknames(ctx, tenant, ids); err != nil {
			return err
		}
		out = partyrules.View(in)
		return nil
	})
	return out, err
}
