package challenge

import (
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
)

// Limits of a request (ADR-0041, COMPETITION_SYSTEM §10.4).
const (
	MaxNameRunes    = 80
	MaxListed       = 500
	MaxExcluded     = 500
	MaxParams       = 20
	MaxParamString  = 255
	MaxTopics       = 50
	ParticipationOK = "individual"
)

// Target types of the audience (ADR-0095: no class, no group).
const (
	TargetGrade    = "grade"
	TargetStudents = "students"
)

// Target is one element of targets[].
type Target struct {
	Type       string
	Grade      *int
	StudentIDs []uuid.UUID
}

// Request is ChallengeCreate after decoding.
type Request struct {
	Name          string
	SchoolID      uuid.UUID
	Targets       []Target
	Exclude       []uuid.UUID
	StartsAt      time.Time
	EndsAt        *time.Time
	Participation string
	Params        map[string]any
}

// Values are the typed parameters filled by the tenant (only keys of allowed_params).
type Values struct {
	SubjectID     *uuid.UUID
	TopicIDs      []uuid.UUID
	MaxEntries    *int
	SeedingPolicy *string
	RegOpenAt     *time.Time
	RegCloseAt    *time.Time
	AutoConfirm   *bool
}

// FilterParams checks params against allowed_params of the template (F107-AC-02, AC-28): a key outside allowed_params
// (rules, reward, chat…) is 422 naming params.<key>; a value of the wrong type is 422 too.
func FilterParams(t Template, raw map[string]any) (Values, error) {
	var v Values
	if len(raw) > MaxParams {
		return v, invalid("params", "at most %d parameters", MaxParams)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		field := "params." + k
		if Forbidden(k) || !slices.Contains(t.AllowedParams, k) {
			return v, invalid(field, "is not a parameter this template allows")
		}
		val := raw[k]
		if s, ok := val.(string); ok && utf8.RuneCountInString(s) > MaxParamString {
			return v, invalid(field, "is at most %d characters", MaxParamString)
		}
		switch k {
		case ParamSubjectID:
			id, ok := uuidOf(val)
			if !ok {
				return v, invalid(field, "must be a UUID")
			}
			v.SubjectID = &id
		case ParamTopicIDs:
			list, ok := val.([]any)
			if !ok || len(list) > MaxTopics {
				return v, invalid(field, "must be at most %d UUIDs", MaxTopics)
			}
			for _, x := range list {
				id, ok := uuidOf(x)
				if !ok {
					return v, invalid(field, "must be UUIDs")
				}
				v.TopicIDs = append(v.TopicIDs, id)
			}
		case ParamMaxEntries:
			f, ok := val.(float64)
			if !ok || f != float64(int(f)) {
				return v, invalid(field, "must be an integer")
			}
			n := int(f)
			v.MaxEntries = &n
		case ParamSeedingPolicy:
			s, ok := val.(string)
			if !ok || !bracket.ValidPolicy(s) {
				return v, invalid(field, "must be rating, random or manual (separate_class is gone, ADR-0095)")
			}
			v.SeedingPolicy = &s
		case ParamRegistrationOpenAt, ParamRegistrationCloseAt:
			s, ok := val.(string)
			ts, err := time.Parse(time.RFC3339, s)
			if !ok || err != nil {
				return v, invalid(field, "must be an RFC 3339 timestamp")
			}
			ts = ts.UTC()
			if k == ParamRegistrationOpenAt {
				v.RegOpenAt = &ts
			} else {
				v.RegCloseAt = &ts
			}
		case ParamAutoConfirm:
			b, ok := val.(bool)
			if !ok {
				return v, invalid(field, "must be a boolean")
			}
			v.AutoConfirm = &b
		}
	}
	return v, nil
}

func uuidOf(v any) (uuid.UUID, bool) {
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil && id != uuid.Nil
}

// Plan is a validated request.
type Plan struct {
	Name        string
	Grade       *int        // the grade target
	Listed      []uuid.UUID // students listed by target students
	Exclude     []uuid.UUID
	StartsAt    time.Time
	EndsAt      time.Time
	JoinOpensAt time.Time
	SubjectID   uuid.UUID
	Values      Values
}

// Validate checks a request against the template (F107-AC-05): name 1–80 characters, one grade target (grade in 1–9 and
// in grade_levels of the template) and/or one students target (1–500 students), at most 500 excluded, participation
// individual only (no study group, ADR-0095), the schedule inside the window of the template, a subject (template or
// parameter), the parameters of allowed_params. School membership of the students is checked by the use case.
func (r Request) Validate(t Template, now time.Time) (Plan, error) {
	var p Plan
	name := strings.TrimSpace(r.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameRunes {
		return p, invalid("name", "is 1 to %d characters", MaxNameRunes)
	}
	p.Name = name
	if r.Participation != "" && r.Participation != ParticipationOK {
		return p, invalid("participation", "only individual (no study group, ADR-0095)")
	}
	if len(r.Targets) == 0 || len(r.Targets) > 2 {
		return p, invalid("targets", "has 1 or 2 elements")
	}
	seenType := map[string]bool{}
	for i, tg := range r.Targets {
		field := "targets"
		if seenType[tg.Type] {
			return p, invalid(field, "has at most one grade target and one students target")
		}
		seenType[tg.Type] = true
		switch tg.Type {
		case TargetGrade:
			if tg.Grade == nil || *tg.Grade < 1 || *tg.Grade > 9 || !slices.Contains(t.GradeLevels, *tg.Grade) {
				return p, invalid(field, "element %d: grade_level is in 1–9 and in the grades of the template", i)
			}
			g := *tg.Grade
			p.Grade = &g
		case TargetStudents:
			if len(tg.StudentIDs) == 0 || len(tg.StudentIDs) > MaxListed {
				return p, invalid(field, "element %d: student_ids has 1 to %d students", i, MaxListed)
			}
			p.Listed = uniq(tg.StudentIDs)
		default:
			return p, invalid(field, "element %d: target_type is grade or students (no class, ADR-0095)", i)
		}
	}
	if len(r.Exclude) > MaxExcluded {
		return p, invalid("exclude_student_ids", "has at most %d students", MaxExcluded)
	}
	p.Exclude = uniq(r.Exclude)
	vals, err := FilterParams(t, r.Params)
	if err != nil {
		return p, err
	}
	p.Values = vals
	switch {
	case vals.SubjectID != nil:
		p.SubjectID = *vals.SubjectID
	case t.SubjectID != nil:
		p.SubjectID = *t.SubjectID
	default:
		return p, invalid("params.subject_id", "is required by this template")
	}
	start := r.StartsAt.UTC()
	if start.Before(now.Add(time.Duration(t.Schedule.MinLeadMinutes)*time.Minute)) ||
		start.After(now.Add(time.Duration(t.Schedule.MaxLeadDays)*24*time.Hour)) {
		return p, invalid("starts_at", "is outside the schedule window of the template")
	}
	end := start.Add(time.Duration(t.Schedule.MaxDurationMinutes) * time.Minute)
	join := start
	if t.Format == FormatTimedQuiz {
		end = start.Add(time.Duration(t.TimedQuiz.DurationMinutes) * time.Minute)
		join = start.Add(-time.Duration(t.TimedQuiz.JoinBeforeMinutes) * time.Minute)
	}
	if r.EndsAt != nil {
		end = r.EndsAt.UTC()
	}
	if !end.After(start) {
		return p, invalid("ends_at", "is after starts_at")
	}
	if end.Sub(start) > time.Duration(t.Schedule.MaxDurationMinutes)*time.Minute {
		return p, invalid("ends_at", "is outside the schedule window of the template")
	}
	p.StartsAt, p.EndsAt, p.JoinOpensAt = start, end, join
	return p, nil
}

func uniq(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Audience is the effective audience (F107-AC-05, ADR-0095 mục 3): (active students of the grade in the school ∪ listed
// students) − excluded students; exclude wins. The result is sorted by id (deterministic rooms).
func Audience(gradeStudents, listed, exclude []uuid.UUID) []uuid.UUID {
	ex := map[uuid.UUID]bool{}
	for _, id := range exclude {
		ex[id] = true
	}
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, list := range [][]uuid.UUID{gradeStudents, listed} {
		for _, id := range list {
			if !ex[id] && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// InAudience reports whether a student of school with grade belongs to the audience of a challenge (F107-AC-08, AC-27):
// same school, not excluded, in the grade target or listed.
func InAudience(student uuid.UUID, school uuid.UUID, grade int, chSchool uuid.UUID, chGrade *int, listed, exclude []uuid.UUID) bool {
	if school != chSchool || slices.Contains(exclude, student) {
		return false
	}
	return (chGrade != nil && *chGrade == grade) || slices.Contains(listed, student)
}

// TournamentPlan is the tournament of a challenge (template + parameters + schedule).
type TournamentPlan struct {
	Format          string
	ParticipantType string
	TeamSize        *int
	Mode            string
	MaxEntries      int
	MinEntries      int
	SeedingPolicy   string
	AutoConfirm     bool
	RoundGap        time.Duration
	RegOpenAt       time.Time
	RegCloseAt      time.Time
	CheckInOpensAt  time.Time
	StartsAt        time.Time
}

// TournamentOf builds the tournament of a validated plan (F107-AC-01): format, participants, mode, seeding and rewards
// from the template; max_entries, seeding_policy, auto_confirm and the registration window from the parameters when the
// template allows them; registration closes no later than the check-in opens (starts_at − check-in).
func TournamentOf(t Template, p Plan, timers tournament.Timers, now time.Time) (TournamentPlan, error) {
	s := t.Tournament
	tp := TournamentPlan{Format: s.Format, ParticipantType: s.ParticipantType, TeamSize: s.TeamSize, Mode: t.Mode, MaxEntries: s.MaxEntries,
		MinEntries: s.MinEntries, SeedingPolicy: s.SeedingPolicy, AutoConfirm: s.AutoConfirm,
		RoundGap: time.Duration(s.RoundGapMinutes) * time.Minute, StartsAt: p.StartsAt}
	tp.CheckInOpensAt = p.StartsAt.Add(-timers.CheckIn)
	v := p.Values
	if v.MaxEntries != nil {
		if *v.MaxEntries < s.MinEntries || *v.MaxEntries > s.MaxEntries {
			return tp, invalid("params.max_entries", "is between %d and %d", s.MinEntries, s.MaxEntries)
		}
		tp.MaxEntries = *v.MaxEntries
	}
	if v.SeedingPolicy != nil {
		tp.SeedingPolicy = *v.SeedingPolicy
	}
	if v.AutoConfirm != nil {
		tp.AutoConfirm = *v.AutoConfirm
	}
	tp.RegOpenAt, tp.RegCloseAt = now, tp.CheckInOpensAt
	if v.RegOpenAt != nil {
		tp.RegOpenAt = *v.RegOpenAt
	}
	if v.RegCloseAt != nil {
		tp.RegCloseAt = *v.RegCloseAt
	}
	if !tp.RegOpenAt.Before(tp.RegCloseAt) || tp.RegCloseAt.After(tp.CheckInOpensAt) {
		return tp, invalid("params.registration_close_at", "registration opens before it closes, and closes before the check-in opens")
	}
	return tp, nil
}
