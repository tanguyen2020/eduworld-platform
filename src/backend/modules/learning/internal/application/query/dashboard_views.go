package query

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SchoolDashboardInput is getSchoolDashboard.
type SchoolDashboardInput struct {
	SchoolID   uuid.UUID
	GradeLevel *int
	ClassID    *uuid.UUID
	SubjectID  *uuid.UUID
	From, To   time.Time
	PageSize   int
}

// assignmentsDueSoon is the "sắp hạn" window of the assignment widget (SCHOOL_LMS §8.1: 48 hours).
const assignmentsDueSoon = 48 * time.Hour

// widgetAssignments bounds the assignments of the widget (contract maxItems).
const widgetAssignments = 20

// GetSchoolDashboard is getSchoolDashboard (F043-AC-07..AC-09, AC-11, AC-13..AC-15): every widget in one call.
func (q Dashboards) GetSchoolDashboard(ctx context.Context, in SchoolDashboardInput) (out dto.SchoolDashboardView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetSchoolDashboard")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(in.From, in.To, "")
	if err != nil {
		return out, err
	}
	sc, err := schoolScope(ctx, p, in.SchoolID, in.SubjectID, false)
	if err != nil {
		return out, err
	}
	sc.GradeLevel, sc.ClassID = in.GradeLevel, in.ClassID
	size := q.Config.PageSize(in.PageSize)
	now := q.now()
	return dashRun(ctx, q, "school_dashboard", p, sc.TenantID, &rg, []any{sc, rg.From, rg.To, size},
		func(ctx context.Context) (dto.SchoolDashboardView, error) {
			v := dto.SchoolDashboardView{SchoolID: in.SchoolID, ClassID: in.ClassID, SubjectID: in.SubjectID}
			ok, err := q.Model.SchoolExists(ctx, sc.TenantID, in.SchoolID)
			if err != nil || !ok {
				return v, notFoundOr(err)
			}
			if in.ClassID != nil {
				c, err := q.Model.Class(ctx, sc.TenantID, *in.ClassID)
				if err != nil || c == nil || c.SchoolID != in.SchoolID {
					return v, notFoundOr(err)
				}
			}
			var m read.RollupMeta
			totals, err := q.Model.SchoolTotals(ctx, sc, rg)
			if err != nil {
				return v, err
			}
			m.Merge(totals.RollupMeta)
			w := &v.Widgets
			w.Accuracy, w.RapidGuessRatio = analytics.Ratio(totals.AccNum, totals.AccDen), analytics.Ratio(totals.RapidNum, totals.RapidDen)
			series, sm, err := q.Model.SchoolSeries(ctx, sc, rg)
			if err != nil {
				return v, err
			}
			m.Merge(sm)
			w.ActiveMinutesSeries = q.points(rg, series, func(p read.SeriesPoint) *float64 {
				x := float64(p.ActiveSec / 60)
				return &x
			}, true)
			last := q.lastDay(rg)
			if w.ActiveStudents, err = q.Model.ActiveStudents(ctx, sc, last.AddDate(0, 0, -6), last); err != nil {
				return v, err
			}
			if err := q.classWidgets(ctx, sc, rg, w, &m); err != nil {
				return v, err
			}
			if err := q.assignmentWidgets(ctx, sc, now, w); err != nil {
				return v, err
			}
			att, total, err := q.Model.Attention(ctx, sc, size)
			if err != nil {
				return v, err
			}
			w.StudentsNeedingAttention, w.StudentsNeedingAttentionTotal = make([]dto.AttentionView, 0, len(att)), total
			for _, a := range att {
				w.StudentsNeedingAttention = append(w.StudentsNeedingAttention, dto.AttentionView{StudentID: a.StudentID, ClassID: a.ClassID,
					Reasons: a.Reasons})
				m.Merge(stamp(a.ComputedAt))
			}
			self, err := q.Model.SelfStudy(ctx, sc)
			if err != nil {
				return v, err
			}
			if self.Students > 0 {
				avg := float64(totals.ActiveSeconds) / 60 / float64(self.Students)
				avg = float64(int(avg*10+0.5)) / 10
				w.SelfStudy.AvgActiveMinutesPerStudent = &avg
			}
			w.SelfStudy.RecommendationStartedRate = analytics.Ratio(self.RecNum, self.RecDen)
			w.SelfStudy.ObjectivesImprovedAfterRecom = self.Improved
			if self.ComputedAtOldest != nil {
				m.Merge(read.RollupMeta{Oldest: self.ComputedAtOldest})
			}
			// Safety notifications come from module chat (F122, facade not available yet): null, never a mock (R17).
			w.SafetyNotificationsUnacknowledge = nil
			meta, err := q.metaOf(ctx, rg, m, sc)
			if err != nil {
				return v, err
			}
			v.DashboardMeta = meta
			return v, nil
		})
}

func notFoundOr(err error) error {
	if err != nil {
		return err
	}
	return problem.New(problem.NotFound, "")
}

// classWidgets fills the class rows (state read model + accuracy of the range), the completion rate and the weak objectives.
func (q Dashboards) classWidgets(ctx context.Context, sc read.DashScope, rg analytics.Range, w *dto.SchoolWidgets, m *read.RollupMeta) error {
	classes, err := q.Model.Classes(ctx, sc)
	if err != nil {
		return err
	}
	acc, am, err := q.Model.ClassTotals(ctx, sc, rg)
	if err != nil {
		return err
	}
	m.Merge(am)
	var compNum, compDen int64
	w.Classes = make([]dto.DashboardClassView, 0, len(classes))
	for _, c := range classes {
		active := c.Active7d
		row := dto.DashboardClassView{ClassID: c.ClassID, Name: c.Name, GradeLevel: c.GradeLevel, ActiveStudents: &active,
			AssignmentCompletionRate: analytics.Ratio(c.CompNum, c.CompDen)}
		if t, ok := acc[c.ClassID]; ok {
			row.Accuracy = analytics.Ratio(t.AccNum, t.AccDen)
		}
		compNum, compDen = compNum+c.CompNum, compDen+c.CompDen
		m.Merge(stamp(c.ComputedAt))
		w.Classes = append(w.Classes, row)
	}
	w.AssignmentCompletionRate = analytics.Ratio(compNum, compDen)
	stats, err := q.Model.ObjectiveStats(ctx, sc)
	if err != nil {
		return err
	}
	type agg struct {
		v       dto.WeakObjectiveView
		dist    []int
		classes []uuid.UUID
	}
	byObj := map[uuid.UUID]*agg{}
	order := []uuid.UUID{}
	for _, s := range stats {
		a := byObj[s.ObjectiveID]
		if a == nil {
			a = &agg{v: dto.WeakObjectiveView{ObjectiveID: s.ObjectiveID, Code: s.Code, YccdRef: s.YccdRef}, dist: make([]int, 5)}
			byObj[s.ObjectiveID] = a
			order = append(order, s.ObjectiveID)
		}
		for i := range a.dist {
			if i < len(s.LevelDist) {
				a.dist[i] += s.LevelDist[i]
			}
		}
		if r := analytics.BelowProficient(s.LevelDist); r != nil && *r >= 0.5 {
			a.classes = append(a.classes, s.ClassID)
		}
		m.Merge(stamp(s.ComputedAt))
	}
	weak := make([]dto.WeakObjectiveView, 0, len(order))
	for _, id := range order {
		a := byObj[id]
		r := analytics.BelowProficient(a.dist)
		if r == nil || *r <= 0 {
			continue
		}
		a.v.BelowProficientRatio, a.v.ClassIDs = *r, a.classes
		if a.v.ClassIDs == nil {
			a.v.ClassIDs = []uuid.UUID{}
		}
		weak = append(weak, a.v)
	}
	sort.SliceStable(weak, func(i, j int) bool {
		if weak[i].BelowProficientRatio != weak[j].BelowProficientRatio {
			return weak[i].BelowProficientRatio > weak[j].BelowProficientRatio
		}
		return weak[i].Code < weak[j].Code
	})
	if len(weak) > 20 {
		weak = weak[:20]
	}
	w.WeakObjectives = weak
	return nil
}

// assignments groups the class rows of the open/recent assignments.
func assignments(rows []read.AssignmentRow) []dto.AssignmentProgressView {
	out := []dto.AssignmentProgressView{}
	idx := map[uuid.UUID]int{}
	for _, r := range rows {
		i, ok := idx[r.AssignmentID]
		if !ok {
			i = len(out)
			idx[r.AssignmentID] = i
			out = append(out, dto.AssignmentProgressView{AssignmentID: r.AssignmentID, Title: r.Title, SubjectID: r.SubjectID, DueAt: r.DueAt.UTC(),
				Status: r.Status, ByClass: []dto.AssignmentClassProgress{}})
		}
		a := &out[i]
		a.Submitted, a.Total, a.NotStarted, a.Missed = a.Submitted+r.Submitted, a.Total+r.Total, a.NotStarted+r.NotStarted, a.Missed+r.Missed
		a.ByClass = append(a.ByClass, dto.AssignmentClassProgress{ClassID: r.ClassID, Submitted: r.Submitted, Total: r.Total,
			NotStarted: r.NotStarted, Missed: r.Missed})
	}
	return out
}

func (q Dashboards) assignmentWidgets(ctx context.Context, sc read.DashScope, now time.Time, w *dto.SchoolWidgets) error {
	rows, err := q.Model.Assignments(ctx, sc, now, widgetAssignments)
	if err != nil {
		return err
	}
	w.Assignments = assignments(rows)
	for _, a := range w.Assignments {
		if a.Status == "open" {
			w.AssignmentsOpen++
			if a.DueAt.After(now) && !a.DueAt.After(now.Add(assignmentsDueSoon)) {
				w.AssignmentsDueSoon++
			}
		}
	}
	return nil
}

// points maps the buckets of the range to points: a missing bucket is 0 for counters (zero = true), null for ratios; its
// is_final follows the day close.
func (q Dashboards) points(rg analytics.Range, rows []read.SeriesPoint, value func(read.SeriesPoint) *float64, zero bool) []dto.AnalyticsPointView {
	by := map[string]read.SeriesPoint{}
	for _, r := range rows {
		by[r.Bucket.Format(time.DateOnly)] = r
	}
	closable := q.Config.LastClosable(q.now())
	out := make([]dto.AnalyticsPointView, 0, len(rg.Buckets()))
	for _, b := range rg.Buckets() {
		k := b.Format(time.DateOnly)
		pt := dto.AnalyticsPointView{PeriodStart: k}
		if r, ok := by[k]; ok {
			pt.Value, pt.IsFinal = value(r), r.Final
		} else {
			pt.IsFinal = !analytics.BucketEnd(b, rg.Grain).After(closable)
			if zero {
				z := 0.0
				pt.Value = &z
			}
		}
		out = append(out, pt)
	}
	return out
}

// ClassDashboardInput is getClassDashboard.
type ClassDashboardInput struct {
	ClassID   uuid.UUID
	SubjectID *uuid.UUID
	From, To  time.Time
	Grain     string
	PageSize  int
	Cursor    string
}

// GetClassDashboard is getClassDashboard (F043-AC-10): assignment progress, class × objective level distribution and the
// students of the class by cursor pages (≤ dashboard_page_size_max).
func (q Dashboards) GetClassDashboard(ctx context.Context, in ClassDashboardInput) (out dto.ClassDashboardView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetClassDashboard")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(in.From, in.To, in.Grain)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermAnalyticsSchoolRead) {
		return out, authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: p.TenantID})
	}
	class, err := q.lookup(ctx, func(ctx context.Context) (*read.ClassRef, error) { return q.Model.Class(ctx, *p.TenantID, in.ClassID) })
	if err != nil {
		return out, err
	}
	if class == nil {
		return out, problem.New(problem.NotFound, "")
	}
	sc, err := schoolScope(ctx, p, class.SchoolID, in.SubjectID, false)
	if err != nil {
		return out, err
	}
	sc.ClassID = &in.ClassID
	size := q.Config.PageSize(in.PageSize)
	var after *uuid.UUID
	if in.Cursor != "" {
		cur, err := q.Cursors.Decode(in.Cursor)
		if err != nil || cur == nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &cur.ID
	}
	now := q.now()
	return dashRun(ctx, q, "class_dashboard", p, sc.TenantID, &rg, []any{sc, rg.From, rg.To, rg.Grain, size, in.Cursor},
		func(ctx context.Context) (dto.ClassDashboardView, error) {
			v := dto.ClassDashboardView{ClassID: in.ClassID, SchoolID: class.SchoolID, SubjectID: in.SubjectID, GeneratedAt: now,
				Assignments: []dto.AssignmentProgressView{}, ObjectiveDistribution: []dto.ObjectiveDistributionView{},
				StudentsNeedingAttention: []dto.ClassAttentionView{}}
			var m read.RollupMeta
			totals, tm, err := q.Model.ClassTotals(ctx, sc, rg)
			if err != nil {
				return v, err
			}
			m.Merge(tm)
			t := totals[in.ClassID]
			v.Accuracy, v.RapidGuessRatio = analytics.Ratio(t.AccNum, t.AccDen), analytics.Ratio(t.RapidNum, t.RapidDen)
			last := q.lastDay(rg)
			if v.ActiveStudents7d, err = q.Model.ActiveStudents(ctx, sc, last.AddDate(0, 0, -6), last); err != nil {
				return v, err
			}
			rows, err := q.Model.Assignments(ctx, sc, now, widgetAssignments)
			if err != nil {
				return v, err
			}
			v.Assignments = assignments(rows)
			for _, a := range v.Assignments {
				if a.Status == "open" {
					v.AssignmentsOpen++
				}
			}
			stats, err := q.Model.ObjectiveStats(ctx, sc)
			if err != nil {
				return v, err
			}
			for _, s := range stats {
				counts := map[string]int{}
				for i, l := range analytics.Levels {
					if i < len(s.LevelDist) {
						counts[l] = s.LevelDist[i]
					}
				}
				r := analytics.BelowProficient(s.LevelDist)
				v.ObjectiveDistribution = append(v.ObjectiveDistribution, dto.ObjectiveDistributionView{ObjectiveID: s.ObjectiveID, Code: s.Code,
					YccdRef: s.YccdRef, StudentsWithEvidence: s.WithEvidence, BelowProficientRatio: r, LevelCounts: counts})
				m.Merge(stamp(s.ComputedAt))
			}
			att, _, err := q.Model.Attention(ctx, sc, q.Config.PageSizeMax)
			if err != nil {
				return v, err
			}
			reasons := map[uuid.UUID][]string{}
			for _, a := range att {
				reasons[a.StudentID] = a.Reasons
				// Per-student weak objectives are not part of the read model yet: empty list (heatmap per class) [Proposed].
				v.StudentsNeedingAttention = append(v.StudentsNeedingAttention, dto.ClassAttentionView{StudentID: a.StudentID, Reasons: a.Reasons,
					WeakObjectiveIDs: []uuid.UUID{}})
				m.Merge(stamp(a.ComputedAt))
			}
			students, err := q.Model.ClassStudents(ctx, sc, rg, after, size+1)
			if err != nil {
				return v, err
			}
			v.Students = dto.ClassStudentsPage{Data: []dto.ClassStudentView{}, Page: dto.PageView{Limit: size}}
			if len(students) > size {
				students = students[:size]
				next := q.Cursors.Encode(page.Cursor{ID: students[size-1].StudentID})
				v.Students.Page.HasMore, v.Students.Page.NextCursor = true, &next
			}
			for _, s := range students {
				rs := reasons[s.StudentID]
				if rs == nil {
					rs = []string{}
				}
				v.Students.Data = append(v.Students.Data, dto.ClassStudentView{StudentID: s.StudentID, ActivitiesCompleted: int(s.Activities),
					LessonsCompleted: int(s.Lessons), Accuracy: analytics.Ratio(s.AccNum, s.AccDen), ActiveMinutes: minutes(s.ActS),
					RapidGuessRatio: analytics.Ratio(s.RapidNum, s.RapidDen), RapidGuessFlag: rapidFlag(s.RapidNum, s.RapidDen, q.Config.Attention),
					Reasons: rs})
			}
			meta, err := q.metaOf(ctx, rg, m, sc)
			if err != nil {
				return v, err
			}
			v.DashboardMeta = meta
			return v, nil
		})
}

// lookup runs a small scope lookup (class, student) in its own dashboard read before the authorization of the scope, so that
// a cached answer is never served before the scope check.
func lookup[T any](ctx context.Context, q Dashboards, fn func(ctx context.Context) (T, error)) (out T, err error) {
	if q.Session == nil || q.Model == nil {
		return out, problem.New(problem.Unavailable, "dashboard database not configured")
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := q.Model.Prepare(ctx, q.Config.StatementTimeout); err != nil {
			return err
		}
		var err error
		out, err = fn(ctx)
		return err
	})
	return out, unavailable(ctx, err)
}

func (q Dashboards) lookup(ctx context.Context, fn func(ctx context.Context) (*read.ClassRef, error)) (*read.ClassRef, error) {
	return lookup(ctx, q, fn)
}

// SeriesInput is getSchoolAnalyticsSeries.
type SeriesInput struct {
	SchoolID   uuid.UUID
	Metric     string
	GradeLevel *int
	SubjectID  *uuid.UUID
	From, To   time.Time
	Grain      string
}

var seriesUnits = map[string]string{"active_students": "students", "active_minutes": "minutes", "attempts_completed": "attempts",
	"accuracy": "ratio", "assignment_completion_rate": "ratio", "lessons_completed": "lessons"}

// GetSchoolSeries is getSchoolAnalyticsSeries (F043-AC-12, F043-AC-17): at most 120 points of the grain chosen by the server.
func (q Dashboards) GetSchoolSeries(ctx context.Context, in SeriesInput) (out dto.AnalyticsSeriesView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetSchoolAnalyticsSeries")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	unit, ok := seriesUnits[in.Metric]
	if !ok {
		return out, problem.Invalid("metric", problem.SubInvalid, "unknown metric")
	}
	rg, err := q.dashRange(in.From, in.To, in.Grain)
	if err != nil {
		return out, err
	}
	sc, err := schoolScope(ctx, p, in.SchoolID, in.SubjectID, false)
	if err != nil {
		return out, err
	}
	sc.GradeLevel = in.GradeLevel
	return dashRun(ctx, q, "school_series", p, sc.TenantID, &rg, []any{sc, in.Metric, rg.From, rg.To, rg.Grain},
		func(ctx context.Context) (dto.AnalyticsSeriesView, error) {
			v := dto.AnalyticsSeriesView{Metric: in.Metric, Unit: unit}
			ok, err := q.Model.SchoolExists(ctx, sc.TenantID, in.SchoolID)
			if err != nil || !ok {
				return v, notFoundOr(err)
			}
			rows, m, err := q.Model.SchoolSeries(ctx, sc, rg)
			if err != nil {
				return v, err
			}
			value := func(p read.SeriesPoint) *float64 {
				var x float64
				switch in.Metric {
				case "active_students":
					x = float64(p.ActiveStudents)
				case "active_minutes":
					x = float64(p.ActiveSec / 60)
				case "attempts_completed":
					x = float64(p.Activities)
				case "lessons_completed":
					x = float64(p.Lessons)
				case "accuracy":
					return analytics.Ratio(p.AccNum, p.AccDen)
				case "assignment_completion_rate":
					return analytics.Ratio(p.AssignCompleted, p.AssignDue)
				}
				return &x
			}
			v.Points = q.points(rg, rows, value, unit != "ratio")
			meta, err := q.metaOf(ctx, rg, m, sc)
			if err != nil {
				return v, err
			}
			v.DashboardMeta = meta
			return v, nil
		})
}

// ProfileInput is getStudentLearningProfile.
type ProfileInput struct {
	StudentID uuid.UUID
	SubjectID *uuid.UUID
	From, To  time.Time
}

// trendWeeks is the length of the mastery trend of the profile (SCHOOL_LMS §8.5: 8 weeks).
const trendWeeks = 8

// GetStudentProfile is getStudentLearningProfile (F043-AC-23, F043-AC-24): a student outside the school/subject scope of the
// caller (or of another tenant) answers 404 without leaking its existence. The caller records learning.profile_viewed after
// a successful read (command.ProfileViews).
func (q Dashboards) GetStudentProfile(ctx context.Context, in ProfileInput) (out dto.StudentLearningProfileView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetStudentLearningProfile")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(in.From, in.To, "")
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermAnalyticsSchoolRead) {
		return out, authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: p.TenantID})
	}
	st, err := lookup(ctx, q, func(ctx context.Context) (*read.DashStudent, error) {
		return q.Model.Student(ctx, *p.TenantID, in.StudentID)
	})
	if err != nil {
		return out, err
	}
	if st == nil {
		return out, problem.New(problem.NotFound, "")
	}
	sc, err := schoolScope(ctx, p, st.SchoolID, in.SubjectID, true)
	if err != nil {
		return out, err
	}
	last := q.lastDay(rg)
	weekTo := analytics.BucketStart(last, analytics.Week)
	weekFrom := weekTo.AddDate(0, 0, -7*(trendWeeks-1))
	return dashRun(ctx, q, "student_profile", p, sc.TenantID, &rg, []any{sc, in.StudentID, rg.From, rg.To},
		func(ctx context.Context) (dto.StudentLearningProfileView, error) {
			v := dto.StudentLearningProfileView{StudentID: in.StudentID, SchoolID: st.SchoolID, Subjects: []dto.ProfileSubjectView{},
				TrendWeeks: []dto.TrendWeekView{}, RecentAttempts: []dto.ProfileAssignmentView{}, ActiveRecommendations: []dto.ProfileRecommendationView{}}
			subjects, m, err := q.Model.ProfileSubjects(ctx, sc, in.StudentID, rg)
			if err != nil {
				return v, err
			}
			topics, err := q.Model.ProfileTopics(ctx, sc, in.StudentID)
			if err != nil {
				return v, err
			}
			idx := map[uuid.UUID]int{}
			for _, s := range subjects {
				idx[s.SubjectID] = len(v.Subjects)
				v.Subjects = append(v.Subjects, dto.ProfileSubjectView{SubjectID: s.SubjectID, Accuracy: analytics.Ratio(s.AccNum, s.AccDen),
					ActiveMinutes: minutes(s.ActS), ActivitiesCompleted: int(s.Activities), LessonsCompleted: int(s.Lessons),
					MasteryByTopic: []dto.TopicMasteryView{}})
			}
			for _, t := range topics {
				i, ok := idx[t.SubjectID]
				if !ok {
					i = len(v.Subjects)
					idx[t.SubjectID] = i
					v.Subjects = append(v.Subjects, dto.ProfileSubjectView{SubjectID: t.SubjectID, MasteryByTopic: []dto.TopicMasteryView{}})
				}
				v.Subjects[i].MasteryByTopic = append(v.Subjects[i].MasteryByTopic, dto.TopicMasteryView{TopicID: t.TopicID, TopicName: t.TopicName,
					LevelDist: t.LevelDist})
			}
			trend, err := q.Model.ProfileTrend(ctx, sc, in.StudentID, weekFrom, weekTo)
			if err != nil {
				return v, err
			}
			for _, t := range trend {
				v.TrendWeeks = append(v.TrendWeeks, dto.TrendWeekView{WeekStart: t.Week.Format(time.DateOnly), SubjectID: t.SubjectID,
					MeanMastery: t.MeanMastery, MasteredCount: t.MasteredCount})
			}
			hist, err := q.Model.ProfileAssignments(ctx, sc, in.StudentID, 20)
			if err != nil {
				return v, err
			}
			for _, h := range hist {
				v.RecentAttempts = append(v.RecentAttempts, dto.ProfileAssignmentView{AssignmentID: h.AssignmentID, Title: h.Title,
					SubjectID: h.SubjectID, Status: h.Status, Score: h.Score, MaxScore: h.MaxScore, DueAt: h.DueAt.UTC(), SubmittedAt: h.SubmittedAt})
			}
			recs, err := q.Model.ProfileRecommendations(ctx, sc, in.StudentID, 3)
			if err != nil {
				return v, err
			}
			for _, r := range recs {
				v.ActiveRecommendations = append(v.ActiveRecommendations, dto.ProfileRecommendationView{RecommendationID: r.RecommendationID,
					RuleCode: r.RuleCode, ReasonCode: r.ReasonCode, SubjectID: r.SubjectID})
			}
			meta, err := q.metaOf(ctx, rg, m, sc)
			if err != nil {
				return v, err
			}
			v.DashboardMeta = meta
			return v, nil
		})
}

// TenantSummaryInput is getTenantLearningSummary.
type TenantSummaryInput struct {
	SchoolIDs  []uuid.UUID
	GradeLevel *int
	From, To   time.Time
	Grain      string
}

// tenantSchools is the school scope of the tenant summary: every school with an unlimited analytics.tenant.read grant (nil);
// otherwise the schools of the analytics.tenant.read / analytics.school.read grants; none → 403.
func tenantSchools(ctx context.Context, p authz.Principal) ([]uuid.UUID, error) {
	var schools []uuid.UUID
	found := false
	for _, g := range p.Grants {
		for _, perm := range g.Permissions {
			switch perm {
			case authz.PermAnalyticsTenantRead:
				found = true
				if g.SchoolID == nil {
					return nil, nil
				}
				schools = append(schools, *g.SchoolID)
			case authz.PermAnalyticsSchoolRead:
				if g.SchoolID != nil {
					found = true
					schools = append(schools, *g.SchoolID)
				}
			}
		}
	}
	if !found {
		return nil, authz.Require(ctx, p, authz.PermAnalyticsTenantRead, authz.Resource{TenantID: p.TenantID})
	}
	return schools, nil
}

// GetTenantSummary is getTenantLearningSummary (F043-AC-25, F043-AC-27): school × grade × subject cells of the session
// tenant (tenant_ids of a tenant user are ignored), cells under analytics_min_cell_size hidden with complementary
// suppression, totals over the visible cells only.
func (q Dashboards) GetTenantSummary(ctx context.Context, in TenantSummaryInput) (out dto.LearningSummaryView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetTenantLearningSummary")
	defer func() { done(err) }()
	p, err := staff(ctx)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(in.From, in.To, in.Grain)
	if err != nil {
		return out, err
	}
	schools, err := tenantSchools(ctx, p)
	if err != nil {
		return out, err
	}
	if len(in.SchoolIDs) > 0 {
		if schools == nil {
			schools = in.SchoolIDs
		} else {
			keep := []uuid.UUID{}
			for _, s := range schools {
				for _, x := range in.SchoolIDs {
					if s == x {
						keep = append(keep, s)
					}
				}
			}
			schools = keep
		}
		if len(schools) == 0 {
			schools = []uuid.UUID{uuid.Nil} // no school of the scope: empty answer
		}
	}
	sc := read.DashScope{TenantID: *p.TenantID, SchoolIDs: schools, GradeLevel: in.GradeLevel}
	return dashRun(ctx, q, "tenant_summary", p, sc.TenantID, &rg, []any{sc, rg.From, rg.To, rg.Grain},
		func(ctx context.Context) (dto.LearningSummaryView, error) {
			cells, m, err := q.Model.SchoolCells(ctx, sc, rg)
			if err != nil {
				return dto.LearningSummaryView{}, err
			}
			v := q.summary(rg, cells, func(c read.SummaryCell) string { return c.SchoolID.String() }, false)
			meta, err := q.metaOf(ctx, rg, m, sc)
			if err != nil {
				return v, err
			}
			v.DashboardMeta = meta
			return v, nil
		})
}

// PlatformSummaryInput is getPlatformLearningSummary.
type PlatformSummaryInput struct {
	GradeLevel *int
	From, To   time.Time
	Grain      string
}

// GetPlatformSummary is getPlatformLearningSummary (F043-AC-26): System Admin only, under the CrossTenantScope of tenant_ids
// (every active tenant when absent, reason platform_analytics), audited cross_tenant.accessed by the read session; QA tenants
// are excluded (DASHBOARD_DATA §3.4); every row carries its tenant_id; cells under analytics_min_cell_size are hidden.
func (q Dashboards) GetPlatformSummary(ctx context.Context, in PlatformSummaryInput) (out dto.LearningSummaryView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetPlatformLearningSummary")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformCrossTenantRead, authz.Resource{}); err != nil {
		return out, err
	}
	if p.TenantID != nil || !p.IsSystemAdmin {
		return out, problem.New(problem.PermissionDenied, "")
	}
	rg, err := q.dashRange(in.From, in.To, in.Grain)
	if err != nil {
		return out, err
	}
	var requested []uuid.UUID
	if ids, ok := crossTenantIDs(ctx); ok {
		requested = ids
	}
	type tenants struct{ active, qa []uuid.UUID }
	ts, err := lookup(ctx, q, func(ctx context.Context) (tenants, error) {
		a, qa, err := q.Model.Tenants(ctx, requested)
		return tenants{a, qa}, err
	})
	if err != nil {
		return out, err
	}
	ids := []uuid.UUID{}
	for _, id := range ts.active {
		if !containsUUID(ts.qa, id) {
			ids = append(ids, id)
		}
	}
	excluded := []uuid.UUID{}
	for _, id := range requested {
		if containsUUID(ts.qa, id) {
			excluded = append(excluded, id)
		}
	}
	if len(ids) == 0 {
		v := q.summary(rg, nil, nil, true)
		v.DashboardMeta, v.ExcludedTenantIDs = q.meta(rg, read.RollupMeta{}), excluded
		return v, nil
	}
	cs, err := authz.CrossTenantScope(p, ids, tenancy.ReasonPlatformAnalytics)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	ctx = tenancy.WithScope(ctx, cs)
	return dashRun(ctx, q, "platform_summary", p, uuid.Nil, &rg, nil, func(ctx context.Context) (dto.LearningSummaryView, error) {
		cells, m, err := q.Model.TenantCells(ctx, ids, in.GradeLevel, rg)
		if err != nil {
			return dto.LearningSummaryView{}, err
		}
		v := q.summary(rg, cells, func(c read.SummaryCell) string { return c.TenantID.String() }, true)
		v.DashboardMeta, v.ExcludedTenantIDs = q.meta(rg, m), excluded
		return v, nil
	})
}

func containsUUID(xs []uuid.UUID, id uuid.UUID) bool {
	for _, x := range xs {
		if x == id {
			return true
		}
	}
	return false
}

// summary applies k-anonymity to the cells (groups: school or tenant) and sums the visible ones (F043-AC-25).
func (q Dashboards) summary(rg analytics.Range, cells []read.SummaryCell, group func(read.SummaryCell) string, platform bool) dto.LearningSummaryView {
	v := dto.LearningSummaryView{Cells: []dto.SummaryCellView{}}
	buckets := int64(max(len(rg.Buckets()), 1))
	kc := make([]analytics.Cell, len(cells))
	for i, c := range cells {
		kc[i] = analytics.Cell{Group: group(c), Students: c.Students}
	}
	hidden := analytics.Suppress(kc, q.Config.MinCellSize)
	type tot struct{ active, acts, accNum, accDen, actS, due, done int64 }
	var all tot
	groups := map[string]*tot{}
	order := []string{}
	for i, c := range cells {
		cv := dto.SummaryCellView{GradeLevel: c.GradeLevel, SubjectID: c.SubjectID, Suppressed: hidden[i]}
		if platform {
			id := c.TenantID
			cv.TenantID = &id
		} else {
			cv.SchoolID = c.SchoolID
		}
		if hidden[i] {
			v.SuppressedCells++
			v.Cells = append(v.Cells, cv)
			continue
		}
		students, active, acts := c.Students, int(c.ActiveSum/buckets), int(c.Activities)
		cv.Students, cv.ActiveStudents, cv.AttemptsComplete = &students, &active, &acts
		cv.Accuracy = analytics.Ratio(c.AccNum, c.AccDen)
		if active > 0 {
			avg := float64(c.ActS) / 60 / float64(c.ActiveSum)
			avg = float64(int(avg*10+0.5)) / 10
			cv.ActiveMinutesAvg = &avg
		}
		v.Cells = append(v.Cells, cv)
		g := groups[group(c)]
		if g == nil {
			g = &tot{}
			groups[group(c)] = g
			order = append(order, group(c))
		}
		for _, t := range []*tot{g, &all} {
			t.active += c.ActiveSum
			t.acts += c.Activities
			t.accNum += c.AccNum
			t.accDen += c.AccDen
			t.actS += c.ActS
			t.due += c.AssignDue
			t.done += c.AssignCompleted
		}
	}
	if len(cells) > v.SuppressedCells {
		active, acts := int(all.active/buckets), int(all.acts)
		v.ActiveStudents, v.AttemptsCompleted = &active, &acts
		v.Accuracy = analytics.Ratio(all.accNum, all.accDen)
		v.AssignmentCompletionRate = analytics.Ratio(all.done, all.due)
		if all.active > 0 {
			avg := float64(all.actS) / 60 / float64(all.active)
			avg = float64(int(avg*10+0.5)) / 10
			v.ActiveMinutesAvg = &avg
		}
	}
	for _, k := range order {
		g := groups[k]
		id, _ := uuid.Parse(k)
		active := int(g.active / buckets)
		row := dto.SummaryGroupView{ActiveStudents: &active, Accuracy: analytics.Ratio(g.accNum, g.accDen)}
		if platform {
			row.TenantID = &id
			v.ByTenant = append(v.ByTenant, row)
		} else {
			row.SchoolID = &id
			v.BySchool = append(v.BySchool, row)
		}
	}
	return v
}
