package query

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GetQuestionStats is getQuestionStats (F044-AC-17, AC-18): learning_content.read; the statistics of every version × grade
// of a question of the tenant, or of an assigned platform question (read-only); another tenant's question → 404.
func (q Reports) GetQuestionStats(ctx context.Context, questionID uuid.UUID) (out dto.QuestionStatsView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetQuestionStats")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermLearningContentRead, authz.Resource{TenantID: p.TenantID}); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		owner, err := q.Quality.Question(ctx, questionID)
		if err != nil {
			return err
		}
		if owner == nil {
			return problem.New(problem.NotFound, "")
		}
		stats, err := q.Quality.QuestionStats(ctx, questionID)
		if err != nil {
			return err
		}
		out = dto.QuestionStatsView{QuestionID: questionID, ReadOnly: p.TenantID != nil && owner.TenantID == nil,
			Items: make([]dto.QuestionStatsItem, 0, len(stats))}
		for _, s := range stats {
			dr := s.DistractorRates
			if dr == nil {
				dr = map[string]float64{}
			}
			out.Items = append(out.Items, dto.QuestionStatsItem{QuestionVersionID: s.QuestionVersionID, VersionNo: s.VersionNo,
				GradeLevel: s.GradeLevel, FirstAttemptCount: s.FirstAttempts, ExcludedRapidGuessCount: s.ExcludedRapidGuess,
				WithHintCount: s.WithHint, Sufficient: s.PValue != nil, PValue: s.PValue, PValueWithHint: s.PValueWithHint,
				Discrimination: s.Discrimination, DistractorRates: dr, AvgTimeMS: s.AvgTimeMS, RapidGuessRate: s.RapidGuessRate,
				HintUsageRate: s.HintUsageRate, Flags: s.Flags, ComputedAt: s.ComputedAt})
		}
		return nil
	})
	return out, err
}

// ListCalibrationProposals is listCalibrationProposals (F044 US5): learning_content.review; platform reviewers see the
// proposals of platform questions, tenant reviewers those of their tenant.
func (q Reports) ListCalibrationProposals(ctx context.Context, status string, in PageInput) (out Page[dto.CalibrationProposalView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCalibrationProposals")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermLearningContentReview, authz.Resource{TenantID: p.TenantID}); err != nil {
		return out, err
	}
	switch status {
	case "", "pending", "accepted", "dismissed":
	default:
		return out, problem.Invalid("status", problem.SubInvalid, "must be pending, accepted or dismissed")
	}
	limit := page.Limit(in.Limit)
	after := uuid.Nil
	if cur, err := q.Cursors.Decode(in.Cursor); err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	} else if cur != nil {
		after = cur.ID
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Quality.Proposals(ctx, p.TenantID == nil, status, after, limit)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows, out.HasMore = rows[:limit], true
			s := q.Cursors.Encode(page.Cursor{ID: rows[limit-1].ID})
			out.NextCursor = &s
		}
		out.Data = make([]dto.CalibrationProposalView, 0, len(rows))
		for _, r := range rows {
			g := r.Grades
			if g == nil {
				g = map[string]any{}
			}
			out.Data = append(out.Data, dto.CalibrationProposalView{ProposalID: r.ID, QuestionID: r.QuestionID, BasedOnVersionID: r.BasedOnVersionID,
				CurrentDifficulty: r.CurrentDifficulty, ProposedDifficulty: r.ProposedDifficulty, PValue: r.PValue, SampleSize: r.SampleSize,
				GradeLevel: r.GradeLevel, GradeDistribution: g, Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt,
				CreatedVersionID: r.CreatedVersionID, CreatedAt: r.CreatedAt})
		}
		return nil
	})
	out.Limit = limit
	return out, err
}

// GetExamSessionReport is getExamSessionReport (F044-AC-23…AC-25): lms.report.read in the school and subject of the session
// (another school or tenant → 404, subject outside the scope or student → 403); the score distribution, median, pass rate,
// p-value per item without rapid guesses, frequent wrong choices by misconception tag, average time, reliability (KR-20 or
// alpha, SEM) with its warning. Aggregates only: no answer of a named student.
func (q Reports) GetExamSessionReport(ctx context.Context, id uuid.UUID) (out dto.ExamSessionReportView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetExamSessionReport")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType == authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		s, err := q.Exams.ExamSession(ctx, id)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := lmsScope(ctx, p, authz.PermLmsReportRead, s.SchoolID, s.SubjectID); err != nil {
			return err
		}
		out = dto.ExamSessionReportView{ExamSessionID: s.ID, IsOfficial: s.IsOfficial, Status: s.Status, ScoreDistribution: []int{},
			Variants: json.RawMessage("[]"), Items: json.RawMessage("[]"), ProblemItems: json.RawMessage("[]")}
		r, err := q.Exams.ExamQuality(ctx, id)
		if err != nil || r == nil {
			return err
		}
		at := r.ComputedAt
		out.Computed, out.ComputedAt, out.StudentCount, out.Mean, out.Median, out.PassRate = true, &at, r.StudentCount, r.Mean, r.Median, r.PassRate
		out.ReliabilityWarning, out.VariantGap = r.ReliabilityWarning, r.VariantGap
		out.Variants, out.Items, out.ProblemItems = r.Variants, r.Items, r.ProblemItems
		if r.Distribution != nil {
			out.ScoreDistribution = r.Distribution
		}
		return nil
	})
	return out, err
}
