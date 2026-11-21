package command

import (
	"context"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PseudonymKeys reads the secret key of the pseudonymous codes (Docker secret EW_EFFICACY_PSEUDONYM_KEY_FILE, F035-AC-33):
// read at each export, never logged nor returned; a missing key is efficacy.ErrNoKey.
type PseudonymKeys interface {
	Key(ctx context.Context) ([]byte, error)
}

// Dataset builds the pseudonymous dataset of a study (F035-AC-32, LEARNING_EFFICACY_PLAN §7): one row per active
// participant (withdrawn, excluded and anonymized students are left out) with participant_code, cohort_code (cluster school × grade, ADR-0095) and site_code
// = HMAC-SHA256(key, study_id ‖ id) truncated, pair_no, arm, form_order, grade, right/total by wave × objective, M_eff and
// level by objective, usage; never a student id, a name, a class, a school, a tenant nor a birth date. Read under the
// audited bypass efficacy_study.
func (h Efficacy) Dataset(ctx context.Context, studyID uuid.UUID) ([]string, [][]string, error) {
	if h.Keys == nil {
		return nil, nil, efficacy.ErrNoKey
	}
	key, err := h.Keys.Key(ctx)
	if err != nil {
		return nil, nil, efficacy.ErrNoKey
	}
	ps, err := efficacy.NewPseudonymizer(key)
	if err != nil {
		return nil, nil, err
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonEfficacyStudy)
	if err != nil {
		return nil, nil, err
	}
	in, err := h.reportData(bctx, studyID)
	if err != nil {
		return nil, nil, err
	}
	var grade int
	pairs := map[uuid.UUID]*int{}
	tenants := map[uuid.UUID]uuid.UUID{}
	orders := map[uuid.UUID]string{}
	if err := h.Read.Read(bctx, func(ctx context.Context) error {
		s, err := h.Data.StudyOf(ctx, studyID)
		if err != nil || s == nil {
			if err == nil {
				err = problem.New(problem.NotFound, "")
			}
			return err
		}
		grade = s.GradeLevel
		cohorts, err := h.Data.CohortsOf(ctx, studyID)
		if err != nil {
			return err
		}
		for _, c := range cohorts {
			pairs[c.SchoolID], tenants[c.SchoolID] = c.PairNo, c.TenantID
			if c.FormOrder != nil {
				orders[c.SchoolID] = *c.FormOrder
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	objectives := append([]uuid.UUID(nil), in.Objectives...)
	sort.Slice(objectives, func(i, j int) bool { return objectives[i].String() < objectives[j].String() })
	cols := []string{"participant_code", "cohort_code", "site_code", "pair_no", "arm", "form_order", "grade_level"}
	for _, kind := range []string{efficacy.WavePre, efficacy.WavePost} {
		for _, o := range objectives {
			cols = append(cols, kind+"_correct_lo_"+o.String(), kind+"_total_lo_"+o.String())
		}
	}
	for _, o := range objectives {
		cols = append(cols, "m_eff_lo_"+o.String(), "level_lo_"+o.String())
	}
	cols = append(cols, "active_minutes", "active_days", "practice_attempts", "rapid_guess_rate")
	type cnt struct{ right, total int }
	answers := map[[3]string]*cnt{}
	for _, a := range in.Answers {
		if containsID(in.Excluded, a.QuestionVersionID) {
			continue
		}
		for _, o := range a.ObjectiveIDs {
			k := [3]string{a.StudentID.String(), a.WaveKind, o.String()}
			if answers[k] == nil {
				answers[k] = &cnt{}
			}
			answers[k].total++
			if a.Correct {
				answers[k].right++
			}
		}
	}
	snaps := map[[2]uuid.UUID]efficacy.Snapshot{}
	for _, s := range in.Snapshots {
		snaps[[2]uuid.UUID{s.StudentID, s.ObjectiveID}] = s
	}
	usage := map[uuid.UUID]efficacy.Usage{}
	for _, u := range in.Usage {
		usage[u.StudentID] = u
	}
	var rows [][]string
	for _, p := range in.Participants {
		if p.Status != efficacy.ParticipantActive {
			continue // F035-AC-19: withdrawn and excluded participants are never exported
		}
		pair := ""
		if n := pairs[p.CohortID]; n != nil {
			pair = strconv.Itoa(*n)
		}
		row := []string{ps.Code(studyID, p.StudentID), ps.Code(studyID, p.CohortID), ps.Code(studyID, tenants[p.CohortID]), pair, p.Arm,
			orders[p.CohortID], strconv.Itoa(grade)}
		for _, kind := range []string{efficacy.WavePre, efficacy.WavePost} {
			for _, o := range objectives {
				c := answers[[3]string{p.StudentID.String(), kind, o.String()}]
				if c == nil {
					row = append(row, "", "")
					continue
				}
				row = append(row, strconv.Itoa(c.right), strconv.Itoa(c.total))
			}
		}
		for _, o := range objectives {
			if s, ok := snaps[[2]uuid.UUID{p.StudentID, o}]; ok {
				row = append(row, strconv.FormatFloat(s.MEff, 'f', 3, 64), s.Level)
			} else {
				row = append(row, "", "")
			}
		}
		u := usage[p.StudentID]
		rate := ""
		if u.Answers > 0 {
			rate = strconv.FormatFloat(float64(u.RapidGuesses)/float64(u.Answers), 'f', 3, 64)
		}
		row = append(row, strconv.FormatFloat(u.ActiveSeconds/60, 'f', 1, 64), strconv.Itoa(u.Days), strconv.Itoa(u.PracticeAttempts), rate)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return cols, rows, nil
}
