package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MatchStaffPermission lets school staff read the matches of the students of their school (API_DESIGN §9.7 "SA
// (school)"). ADR-0095 §6 renames it lms.student.read; the catalog keeps lms.class.read until that rename lands.
const MatchStaffPermission = authz.PermLmsClassRead

// Battles are the read use cases of the battle engine (F105 US5): getMatch and listMyMatches, plus the review of a
// completed match read from module learning (revealPlanItem: question, correct answer, explanation — ADR-0016).
type Battles struct {
	Read   read.ReadSession
	Reader read.BattleReader
	Plans  port.QuestionPlans // nil: no review
}

type viewer struct {
	tenant  uuid.UUID
	user    uuid.UUID
	student bool
	p       authz.Principal
}

// matchViewer is the caller of getMatch: a student of the game client (game_player) or school staff of the portal;
// staff with a game token is GAME_ACCESS_STUDENT_ONLY (F105-AC-25).
func matchViewer(ctx context.Context) (viewer, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return viewer{}, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil {
		return viewer{}, problem.New(problem.NotFound, "")
	}
	switch {
	case p.UserType == authz.UserTypeStudent && p.Channel != authz.ChannelPortal:
		return viewer{tenant: *p.TenantID, user: p.UserID, student: true, p: p}, nil
	case p.UserType == authz.UserTypeStaff && p.Channel == authz.ChannelPortal:
		return viewer{tenant: *p.TenantID, user: p.UserID, p: p}, nil
	case p.Channel == authz.ChannelGame:
		return viewer{}, problem.New(problem.GameAccessStudentOnly, "")
	}
	return viewer{}, problem.New(problem.PortalAccessStaffOnly, "")
}

// staffSees: the staff grants holding MatchStaffPermission cover the school of a participant (no audit on a miss: the
// match is simply not found, no existence leak).
func staffSees(p authz.Principal, m *read.MatchRow) bool {
	for _, g := range p.Grants {
		held := false
		for _, perm := range g.Permissions {
			if perm == MatchStaffPermission {
				held = true
			}
		}
		if !held {
			continue
		}
		for _, pl := range m.Players {
			if g.SchoolID == nil || *g.SchoolID == pl.SchoolID {
				return true
			}
		}
	}
	return false
}

// GetMatch implements getMatch (F105-AC-23, AC-25): a participant, or staff with the permission in the school of a
// participant; anybody else 404. Opponents show nickname/avatar only (R16), never a rating. A completed match carries
// its review (questions, correct answers, explanations) read from module learning after the match.
func (q Battles) GetMatch(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetMatch")
	defer func() { done(err) }()
	v, err := matchViewer(ctx)
	if err != nil {
		return nil, err
	}
	if !v.student && !v.p.Has(MatchStaffPermission) {
		if err := authz.Require(ctx, v.p, MatchStaffPermission, authz.Resource{TenantID: &v.tenant}); err != nil {
			return nil, err
		}
	}
	var m *read.MatchRow
	var cards map[uuid.UUID]port.Card
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		if m, err = q.Reader.Match(ctx, id); err != nil || m == nil {
			return err
		}
		ids := make([]uuid.UUID, len(m.Players))
		for i, p := range m.Players {
			ids[i] = p.StudentID
		}
		cards, err = q.Reader.Cards(ctx, v.tenant, ids)
		return err
	})
	if err != nil {
		return nil, err
	}
	visible := m != nil
	if visible && v.student {
		visible = false
		for _, p := range m.Players {
			if p.StudentID == v.user {
				visible = true
			}
		}
	}
	if visible && !v.student {
		visible = staffSees(v.p, m)
	}
	if !visible {
		return nil, problem.New(problem.NotFound, "")
	}
	out = matchBody(*m, cards)
	if m.Status == "completed" && m.PlanID != nil && q.Plans != nil {
		out["review"] = q.review(ctx, v, *m)
	}
	return out, nil
}

func matchBody(m read.MatchRow, cards map[uuid.UUID]port.Card) map[string]any {
	parts := make([]map[string]any, 0, len(m.Players))
	for _, p := range m.Players {
		c, ok := cards[p.StudentID]
		if !ok {
			c = port.Card{UserID: p.StudentID, Level: 1}
		}
		e := map[string]any{"player": view.CardBody(c), "team_no": p.TeamNo, "outcome": p.Outcome, "score": p.Score,
			"correct_count": p.Correct, "competition_points": nil}
		parts = append(parts, e)
	}
	b := map[string]any{"match_id": m.ID, "mode": m.Mode, "subject_id": m.SubjectID, "grade_level": m.Grade, "status": m.Status,
		"matchmaking_type": m.Type, "ranked": m.Ranked, "is_bot": m.IsBot, "end_reason": m.EndReason, "rated": m.Rated != nil && *m.Rated,
		"question_count": m.QuestionCount, "participants": parts}
	if m.RoomID != nil {
		b["room_id"] = *m.RoomID
	}
	if m.StartedAt != nil {
		b["started_at"] = *m.StartedAt
	}
	if m.EndedAt != nil {
		b["ended_at"] = *m.EndedAt
	}
	return b
}

// review reads each question of a completed match from module learning (the answer key never left it); a question it
// cannot serve is listed without content. my_* are the viewer's own results (students only).
func (q Battles) review(ctx context.Context, v viewer, m read.MatchRow) []map[string]any {
	var summary struct {
		Questions []struct {
			Index   int `json:"index"`
			Answers []struct {
				Student  uuid.UUID `json:"student_id"`
				Answered bool      `json:"answered"`
				Correct  bool      `json:"is_correct"`
				Points   int       `json:"points"`
			} `json:"per_student"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(m.Summary, &summary)
	closed := time.Now().UTC()
	if m.EndedAt != nil {
		closed = *m.EndedAt
	}
	sctx := ctx
	if sc, err := authz.EventScope(v.tenant); err == nil {
		sctx = tenancy.WithScope(ctx, sc)
	}
	out := []map[string]any{}
	for i := 0; i < min(m.QuestionCount, 30); i++ {
		e := map[string]any{"index": i, "correct_response": nil, "explanation": nil, "my_answer_correct": nil, "my_points": nil}
		if rv, err := q.Plans.Reveal(sctx, *m.PlanID, i, closed); err == nil {
			if len(rv.Question) > 0 {
				e["question"] = rv.Question
			}
			if len(rv.CorrectResponse) > 0 {
				e["correct_response"] = rv.CorrectResponse
			}
			if rv.Explanation != "" {
				e["explanation"] = rv.Explanation
			}
		} else {
			slog.WarnContext(ctx, "competition: match review unavailable", "question_index", i, "error", err)
		}
		if v.student {
			for _, qr := range summary.Questions {
				if qr.Index != i {
					continue
				}
				for _, a := range qr.Answers {
					if a.Student == v.user && a.Answered {
						c, pts := a.Correct, a.Points
						e["my_answer_correct"], e["my_points"] = c, pts
					}
				}
			}
		}
		out = append(out, e)
	}
	return out
}

// MatchPage is a page of listMyMatches.
type MatchPage struct {
	Data []map[string]any
	Next string
}

// ListMyMatches implements listMyMatches (F105-AC-23): the matches of the student (game_player), newest first.
func (q Battles) ListMyMatches(ctx context.Context, mode string, limit int, cursor string) (out MatchPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListMyMatches")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return out, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var before *time.Time
	beforeID := uuid.Nil
	if cursor != "" {
		t, id, ok := decodeCursor(cursor)
		if !ok {
			return out, problem.Invalid("/cursor", problem.SubInvalid, "is not a cursor of this list")
		}
		before, beforeID = &t, id
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Reader.MyMatches(ctx, me, mode, before, beforeID, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			last := rows[limit-1]
			out.Next = encodeCursor(last.CreatedAt, last.ID)
			rows = rows[:limit]
		}
		ids := []uuid.UUID{}
		for _, r := range rows {
			for _, p := range r.Players {
				ids = append(ids, p.StudentID)
			}
		}
		cards, err := q.Reader.Cards(ctx, tenant, ids)
		if err != nil {
			return err
		}
		out.Data = make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			out.Data = append(out.Data, matchBody(r, cards))
		}
		return nil
	})
	return out, err
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(t.UnixNano(), 10) + "|" + id.String()))
}

func decodeCursor(s string) (time.Time, uuid.UUID, bool) {
	if len(s) > 128 {
		return time.Time{}, uuid.Nil, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	ns, id, ok := strings.Cut(string(b), "|")
	n, err1 := strconv.ParseInt(ns, 10, 64)
	u, err2 := uuid.Parse(id)
	if !ok || err1 != nil || err2 != nil {
		return time.Time{}, uuid.Nil, false
	}
	return time.Unix(0, n).UTC(), u, true
}
