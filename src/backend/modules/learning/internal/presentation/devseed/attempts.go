package devseed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// AttemptFile is the DEV-ONLY script of student attempts (seed/attempts/*.dev.json, LOCAL_DEVELOPMENT §5): which seeded
// student answers which option of which question in which DEV school assignment. Every attempt goes through the real
// attempt API of the game client (startActivityAttempt → submitActivityResponse → submitActivityAttempt): the server
// scores (AGENTS R5), the script only chooses the options like a student would. Before a student logs in, the staff
// read API (listAssignmentSubmissions) tells whether the attempts of that student are already submitted: then the
// student is skipped, so a second run logs no student in (repeated logins risk a lockout).
type AttemptFile struct {
	Comment    any    `json:"$comment,omitempty"`
	TenantCode string `json:"tenant_code"`
	SchoolCode string `json:"school_code"`
	GradeLevel int    `json:"grade_level"`
	// Staff is the school staff member that reads the submissions (school_admin of the school).
	Staff string `json:"staff"`
	// AnswerWaitMS is the wait before an answer that is not a rapid guess (above rapid_guess_min_response_ms of the
	// questions × 1.5 for g1_3, ACTIVITY_ENGINE §11.1).
	AnswerWaitMS int             `json:"answer_wait_ms"`
	Students     []StudentScript `json:"students"`
	// Expected are the golden values of the School Learning Dashboard after the script (checked by -verify).
	Expected *GoldenValues `json:"expected,omitempty"`
}

// StudentScript is what one student does (one login for all its attempts).
type StudentScript struct {
	Username string          `json:"username"`
	Attempts []AttemptScript `json:"attempts"`
}

// AttemptScript is one attempt of a DEV assignment (found by title in the school and grade of the file).
type AttemptScript struct {
	Assignment string         `json:"assignment"`
	Answers    []AnswerScript `json:"answers"`
}

// AnswerScript is one answer: the option of a single_choice question (seed key of a lesson file); Rapid answers at
// once (a rapid_guess on the server), any other answer after AnswerWaitMS.
type AnswerScript struct {
	Question string `json:"question"`
	Option   string `json:"option"`
	Rapid    bool   `json:"rapid,omitempty"`
}

// Validate checks the script before any call.
func (f AttemptFile) Validate() error {
	switch {
	case f.TenantCode == "" || f.SchoolCode == "" || f.Staff == "":
		return errors.New("devseed: tenant_code, school_code and staff are required")
	case f.GradeLevel < 1 || f.GradeLevel > 9:
		return errors.New("devseed: grade_level must be 1..9")
	case f.AnswerWaitMS < 0 || f.AnswerWaitMS > 60000:
		return errors.New("devseed: answer_wait_ms must be 0..60000")
	case len(f.Students) == 0:
		return errors.New("devseed: at least one student is required")
	}
	users := map[string]bool{}
	for _, s := range f.Students {
		if s.Username == "" || users[s.Username] || len(s.Attempts) == 0 {
			return fmt.Errorf("devseed: student %q: unique username with attempts required", s.Username)
		}
		users[s.Username] = true
		seen := map[string]bool{}
		for _, a := range s.Attempts {
			if a.Assignment == "" || seen[a.Assignment] || len(a.Answers) == 0 {
				return fmt.Errorf("devseed: student %s: one attempt per assignment, with answers", s.Username)
			}
			seen[a.Assignment] = true
			qs := map[string]bool{}
			for _, ans := range a.Answers {
				if !seedKey.MatchString(ans.Question) || ans.Option == "" || qs[ans.Question] {
					return fmt.Errorf("devseed: student %s, %q: one answer per question key with an option", s.Username, a.Assignment)
				}
				qs[ans.Question] = true
			}
		}
	}
	return nil
}

// StudentLogin signs a student in like the game client and returns the session (cmd/seed-attempts: gameLogin).
type StudentLogin func(ctx context.Context, username string) (StudentSession, error)

// StudentSession is the game session of a student: token, tenant and the client headers of the game.
type StudentSession struct {
	Token    string
	TenantID string
	Headers  map[string]string // X-Device-Id, X-Client-Version, X-Client-Platform
	Logout   func(ctx context.Context) error
}

// AttemptClient runs the script: Staff reads the school data, Login signs the students in.
type AttemptClient struct {
	BaseURL string // public gateway, ends with /api/v1
	HTTP    *http.Client
	Staff   string // token of AttemptFile.Staff
	Login   StudentLogin
	Sleep   func(ctx context.Context, d time.Duration) error // default: time.After
}

// AttemptResult tells what the run did.
type AttemptResult struct {
	TenantID, SchoolID string
	Students           map[string]string    // username → student id
	Assignments        map[string]uuid.UUID // title → assignment id
	Logins             int                  // students logged in by this run (0 on a second run)
	Submitted          int                  // attempts submitted by this run
	Skipped            int                  // attempts already submitted before this run
}

// attemptItem is an item of the Attempt answer (position and pinned question version).
type attemptItem struct {
	Position          int    `json:"position"`
	QuestionVersionID string `json:"question_version_id"`
}

type seededAssignment struct {
	id, activityID string
}

// RunAttempts runs the script f over the questions of lessons (version id → key) in the tenant of tenants.
func RunAttempts(ctx context.Context, c AttemptClient, tenants TenantResolver, f AttemptFile, lessons LessonResult) (AttemptResult, error) {
	out := AttemptResult{Students: map[string]string{}, Assignments: map[string]uuid.UUID{}}
	if err := f.Validate(); err != nil {
		return out, err
	}
	if c.Sleep == nil {
		c.Sleep = func(ctx context.Context, d time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
				return nil
			}
		}
	}
	ids, err := tenants.Tenants(ctx)
	if err != nil {
		return out, err
	}
	if len(ids) != 1 {
		return out, fmt.Errorf("devseed: tenant %s resolved to %d tenants", f.TenantCode, len(ids))
	}
	out.TenantID = ids[0]
	h := map[string]string{"X-Tenant-Id": out.TenantID}
	if out.SchoolID, err = schoolID(ctx, c.HTTP, c.BaseURL, c.Staff, h, f.SchoolCode, f.TenantCode); err != nil {
		return out, err
	}
	if out.Students, err = GradeStudents(ctx, c.HTTP, c.BaseURL, c.Staff, out.TenantID, out.SchoolID, f.GradeLevel); err != nil {
		return out, err
	}
	asg := map[string]seededAssignment{}
	for _, s := range f.Students {
		if _, ok := out.Students[s.Username]; !ok {
			return out, fmt.Errorf("student %s not in grade %d of %s: run make seed-identity first", s.Username, f.GradeLevel, f.SchoolCode)
		}
		for _, a := range s.Attempts {
			if _, ok := asg[a.Assignment]; ok {
				continue
			}
			sa, err := c.findAssignment(ctx, h, out.SchoolID, f.GradeLevel, a.Assignment)
			if err != nil {
				return out, err
			}
			asg[a.Assignment] = sa
			out.Assignments[a.Assignment] = uuid.MustParse(sa.id)
		}
	}
	done := map[string]map[string]bool{} // assignment → student id → submitted
	for title, sa := range asg {
		if done[title], err = c.submitted(ctx, h, sa.id); err != nil {
			return out, err
		}
	}
	for _, s := range f.Students {
		sid := out.Students[s.Username]
		var pending []AttemptScript
		for _, a := range s.Attempts {
			if done[a.Assignment][sid] {
				out.Skipped++
				continue
			}
			pending = append(pending, a)
		}
		if len(pending) == 0 {
			slog.InfoContext(ctx, "student attempts already submitted, no login", "student", s.Username)
			continue
		}
		n, err := c.student(ctx, f, s.Username, pending, asg, lessons)
		out.Submitted += n
		out.Logins++
		if err != nil {
			return out, fmt.Errorf("student %s: %w", s.Username, err)
		}
	}
	return out, nil
}

func schoolID(ctx context.Context, hc *http.Client, base, token string, h map[string]string, code, tenantCode string) (string, error) {
	var schools struct {
		Items []map[string]any `json:"items"`
	}
	if _, err := Do(ctx, hc, base, token, http.MethodGet, "/catalog/schools?limit=100&q="+url.QueryEscape(code), h, nil, &schools); err != nil {
		return "", err
	}
	i := slices.IndexFunc(schools.Items, func(s map[string]any) bool { return str(s, "code") == code })
	if i < 0 {
		return "", fmt.Errorf("school %s not found in tenant %s: run make seed-identity first", code, tenantCode)
	}
	return str(schools.Items[i], "school_id"), nil
}

// findAssignment returns the published DEV assignment titled title of (school, grade) with its activity.
func (c AttemptClient) findAssignment(ctx context.Context, h map[string]string, school string, grade int, title string) (seededAssignment, error) {
	q := url.Values{"school_id": {school}, "grade_level": {strconv.Itoa(grade)}, "limit": {"100"}}
	var found page
	if _, err := Do(ctx, c.HTTP, c.BaseURL, c.Staff, http.MethodGet, "/lms/assignments?"+q.Encode(), h, nil, &found); err != nil {
		return seededAssignment{}, err
	}
	for _, it := range found.Data {
		if str(it, "title") != title || str(it, "status") == "cancelled" || str(it, "status") == "archived" || str(it, "status") == "draft" {
			continue
		}
		ref, _ := it["content_ref"].(map[string]any)
		if str(ref, "content_type") != "activity" {
			return seededAssignment{}, fmt.Errorf("assignment %q: only activity assignments are scripted", title)
		}
		return seededAssignment{id: str(it, "assignment_id"), activityID: str(ref, "content_id")}, nil
	}
	return seededAssignment{}, fmt.Errorf("assignment %q not published in grade %d: run the assignment seed first", title, grade)
}

// submitted returns the students whose submission of the assignment is submitted or graded (listAssignmentSubmissions,
// every page).
func (c AttemptClient) submitted(ctx context.Context, h map[string]string, assignment string) (map[string]bool, error) {
	out := map[string]bool{}
	cursor := ""
	for range 100 {
		q := url.Values{"limit": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p struct {
			Data []map[string]any `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		if _, err := Do(ctx, c.HTTP, c.BaseURL, c.Staff, http.MethodGet, "/lms/assignments/"+assignment+"/submissions?"+q.Encode(), h, nil, &p); err != nil {
			return nil, err
		}
		for _, s := range p.Data {
			if st := str(s, "status"); st == "submitted" || st == "graded" {
				out[str(s, "student_id")] = true
			}
		}
		if !p.Page.HasMore || p.Page.NextCursor == nil || *p.Page.NextCursor == "" {
			return out, nil
		}
		cursor = *p.Page.NextCursor
	}
	return nil, fmt.Errorf("submissions of %s: too many pages", assignment)
}

// student logs one student in once and runs its pending attempts; it returns the attempts submitted.
func (c AttemptClient) student(ctx context.Context, f AttemptFile, username string, pending []AttemptScript,
	asg map[string]seededAssignment, lessons LessonResult) (int, error) {
	sess, err := c.Login(ctx, username)
	if err != nil {
		return 0, err
	}
	defer func() {
		if sess.Logout != nil {
			if err := sess.Logout(ctx); err != nil {
				slog.WarnContext(ctx, "student logout failed", "student", username, "error", err)
			}
		}
	}()
	h := map[string]string{"X-Tenant-Id": sess.TenantID}
	for k, v := range sess.Headers {
		h[k] = v
	}
	n := 0
	for _, a := range pending {
		sa := asg[a.Assignment]
		if err := c.attempt(ctx, f, sess.Token, h, sa, a, lessons); err != nil {
			return n, fmt.Errorf("%q: %w", a.Assignment, err)
		}
		n++
	}
	return n, nil
}

// attempt starts (or resumes) the attempt of an assignment, answers the items not answered yet and submits.
func (c AttemptClient) attempt(ctx context.Context, f AttemptFile, token string, h map[string]string, sa seededAssignment,
	a AttemptScript, lessons LessonResult) error {
	var at struct {
		AttemptID      string        `json:"attempt_id"`
		Status         string        `json:"status"`
		Items          []attemptItem `json:"items"`
		SavedResponses []struct {
			Position int `json:"position"`
		} `json:"saved_responses"`
	}
	body := map[string]any{"mode": "assignment", "context_type": "assignment", "context_id": sa.id}
	if _, err := Do(ctx, c.HTTP, c.BaseURL, token, http.MethodPost, "/learning/activities/"+sa.activityID+"/attempts", h, body, &at); err != nil {
		return err
	}
	if at.Status != "in_progress" {
		return nil
	}
	answers := map[string]AnswerScript{}
	for _, ans := range a.Answers {
		answers[ans.Question] = ans
	}
	saved := map[int]bool{}
	for _, s := range at.SavedResponses {
		saved[s.Position] = true
	}
	items := at.Items
	slices.SortFunc(items, func(x, y attemptItem) int { return x.Position - y.Position })
	last := time.Now()
	for _, it := range items {
		if saved[it.Position] {
			continue
		}
		key := lessons.QuestionKeyOf(it.QuestionVersionID)
		ans, ok := answers[key]
		if !ok {
			return fmt.Errorf("item %d (question %q) has no scripted answer", it.Position, key)
		}
		if !ans.Rapid {
			if wait := time.Duration(f.AnswerWaitMS)*time.Millisecond - time.Since(last); wait > 0 {
				if err := c.Sleep(ctx, wait); err != nil {
					return err
				}
			}
		}
		r := map[string]any{"kind": "question", "position": it.Position, "response": map[string]any{"option_id": ans.Option},
			"client_elapsed_ms": time.Since(last).Milliseconds()}
		if _, err := Do(ctx, c.HTTP, c.BaseURL, token, http.MethodPost, "/learning/activity-attempts/"+at.AttemptID+"/responses", h, r, nil); err != nil {
			return fmt.Errorf("item %d: %w", it.Position, err)
		}
		last = time.Now()
	}
	if _, err := Do(ctx, c.HTTP, c.BaseURL, token, http.MethodPost, "/learning/activity-attempts/"+at.AttemptID+":submit", h, nil, nil); err != nil &&
		!IsStatus(err, http.StatusConflict) {
		return err
	}
	slog.InfoContext(ctx, "attempt submitted", "attempt_id", at.AttemptID, "assignment_id", sa.id)
	return nil
}
