package devseed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssignmentFile is the DEV-ONLY school assignment seed (seed/assignments/*.dev.json, LOCAL_DEVELOPMENT §5): assignments
// of the seeded content to one grade of one school, created and published by a school admin of that school through the
// public LMS API (createAssignment → publishAssignment, F041) — never SQL. Running it again changes nothing: an
// assignment is found again by its natural key (school, grade, title) and only a missing publish runs.
type AssignmentFile struct {
	TenantCode  string           `json:"tenant_code"`
	SchoolCode  string           `json:"school_code"`
	Author      string           `json:"author"`
	WindowDays  int              `json:"window_days"`
	Assignments []AssignmentSeed `json:"assignments"`
}

// AssignmentSeed is one assignment of the seeded content: `content` = activity | question_set of the content seed, or
// activity:<key> for an activity of a lesson file (LessonFile). ExcludeUsernames are students of the grade left out of
// the grade target (exclude_student_ids, ADR-0095 §3), resolved through listTenantGradeStudents.
type AssignmentSeed struct {
	Title            string   `json:"title"`
	Instructions     string   `json:"instructions"`
	Content          string   `json:"content"`
	Mode             string   `json:"mode"`
	GradeLevel       int      `json:"grade_level"`
	MaxAttempts      int      `json:"max_attempts"`
	ExcludeUsernames []string `json:"exclude_usernames,omitempty"`
}

// seedContentKinds are the content seed items an assignment can reference (auto-graded only, F041).
var seedContentKinds = []string{"activity", "question_set"}

// lessonActivityPrefix names an activity of a lesson file in AssignmentSeed.Content.
const lessonActivityPrefix = "activity:"

// assignable reports a content of the content seed or an activity:<key> of a lesson file.
func assignable(content string) bool {
	if slices.Contains(seedContentKinds, content) {
		return true
	}
	key, ok := strings.CutPrefix(content, lessonActivityPrefix)
	return ok && seedKey.MatchString(key)
}

// Validate checks the file before any call (lengths of AssignmentCreate, specs/openapi).
func (f AssignmentFile) Validate() error {
	switch {
	case f.TenantCode == "" || f.SchoolCode == "" || f.Author == "":
		return errors.New("devseed: tenant_code, school_code and author are required")
	case f.WindowDays < 1 || f.WindowDays > 3650:
		return errors.New("devseed: window_days must be 1..3650")
	case len(f.Assignments) == 0:
		return errors.New("devseed: at least one assignment is required")
	}
	seen := map[string]bool{}
	for _, a := range f.Assignments {
		key := strconv.Itoa(a.GradeLevel) + "|" + a.Title
		switch {
		case a.Title == "" || len([]rune(a.Title)) > 120 || seen[key]:
			return fmt.Errorf("devseed: assignment %q: unique title of 1..120 characters per grade required", a.Title)
		case !assignable(a.Content):
			return fmt.Errorf("devseed: assignment %q: content must be one of %v or activity:<key>", a.Title, seedContentKinds)
		case len(a.ExcludeUsernames) > 200:
			return fmt.Errorf("devseed: assignment %q: at most 200 exclude_usernames", a.Title)
		case a.Mode != "practice" && a.Mode != "graded":
			return fmt.Errorf("devseed: assignment %q: mode must be practice or graded", a.Title)
		case a.GradeLevel < 1 || a.GradeLevel > 9 || a.MaxAttempts < 1 || a.MaxAttempts > 10 || len([]rune(a.Instructions)) > 2000:
			return fmt.Errorf("devseed: assignment %q: grade_level 1..9, max_attempts 1..10, instructions ≤ 2000", a.Title)
		}
		seen[key] = true
	}
	return nil
}

// TenantResolver resolves a tenant code to its id (System Admin listTenants, AdminAssigner).
type TenantResolver interface {
	Tenants(ctx context.Context) ([]string, error)
}

// AssignmentClient calls the public LMS API (BaseURL ends with /api/v1) as the school admin of the seeded school.
type AssignmentClient struct {
	BaseURL string
	HTTP    *http.Client
	Token   string
	Now     func() time.Time
}

// AssignmentResult lists the assignments the seed found or created and how many it published in this run.
type AssignmentResult struct {
	TenantID    string
	SchoolID    string
	Assignments []uuid.UUID
	Created     int
	Published   int
}

// RunAssignments creates and publishes the assignments of f over the content of res (output of Run) in the tenant of
// tenants: open from one hour before the first run, due WindowDays later, so they are inside their window right away.
func RunAssignments(ctx context.Context, c AssignmentClient, tenants TenantResolver, f AssignmentFile, res Result) (AssignmentResult, error) {
	var out AssignmentResult
	if err := f.Validate(); err != nil {
		return out, err
	}
	if res.SubjectID == "" {
		return out, errors.New("devseed: the content seed result has no subject")
	}
	for _, a := range f.Assignments {
		if _, err := res.contentOf(a.Content); err != nil {
			return out, fmt.Errorf("assignment %q: %w", a.Title, err)
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
	var schools struct {
		Items []map[string]any `json:"items"`
	}
	if _, err := Do(ctx, c.HTTP, c.BaseURL, c.Token, http.MethodGet, "/catalog/schools?limit=100&q="+url.QueryEscape(f.SchoolCode), h, nil, &schools); err != nil {
		return out, err
	}
	i := slices.IndexFunc(schools.Items, func(s map[string]any) bool { return str(s, "code") == f.SchoolCode })
	if i < 0 {
		return out, fmt.Errorf("school %s not found in tenant %s: run make seed-identity first", f.SchoolCode, f.TenantCode)
	}
	out.SchoolID = str(schools.Items[i], "school_id")
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	openAt := now().UTC().Add(-time.Hour).Truncate(time.Minute)
	for _, a := range f.Assignments {
		var exclude []string
		if len(a.ExcludeUsernames) > 0 {
			students, err := GradeStudents(ctx, c.HTTP, c.BaseURL, c.Token, out.TenantID, out.SchoolID, a.GradeLevel)
			if err != nil {
				return out, fmt.Errorf("assignment %q: %w", a.Title, err)
			}
			for _, u := range a.ExcludeUsernames {
				id, ok := students[u]
				if !ok {
					return out, fmt.Errorf("assignment %q: student %s not in grade %d of %s: run make seed-identity first", a.Title, u, a.GradeLevel, f.SchoolCode)
				}
				exclude = append(exclude, id)
			}
		}
		id, created, published, err := c.assignment(ctx, h, out.SchoolID, res, a, exclude, openAt, openAt.AddDate(0, 0, f.WindowDays))
		if err != nil {
			return out, fmt.Errorf("assignment %q: %w", a.Title, err)
		}
		out.Assignments = append(out.Assignments, id)
		out.Created += created
		out.Published += published
	}
	return out, nil
}

// contentOf resolves the content of an assignment seed in the result of the content seed (or of a lesson file).
func (r Result) contentOf(content string) (uuid.UUID, error) {
	var id uuid.UUID
	switch {
	case content == "activity":
		id = r.Activity
	case content == "question_set":
		id = r.QuestionSet
	case strings.HasPrefix(content, lessonActivityPrefix):
		id = r.Activities[strings.TrimPrefix(content, lessonActivityPrefix)]
	}
	if id == uuid.Nil {
		return id, fmt.Errorf("devseed: content %q is not in the content seed result", content)
	}
	return id, nil
}

// GradeStudents returns username → student id of the students of one grade of a school (listTenantGradeStudents of
// module identity, every page) as a school staff member of the tenant.
func GradeStudents(ctx context.Context, hc *http.Client, base, token, tenantID, schoolID string, grade int) (map[string]string, error) {
	out := map[string]string{}
	h := map[string]string{"X-Tenant-Id": tenantID}
	cursor := ""
	for range 100 {
		q := url.Values{"limit": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p struct {
			Items []map[string]any `json:"items"` // GradeStudentPage
			Page  struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		path := fmt.Sprintf("/tenant/schools/%s/grades/%d/students?%s", schoolID, grade, q.Encode())
		if _, err := Do(ctx, hc, base, token, http.MethodGet, path, h, nil, &p); err != nil {
			return nil, err
		}
		for _, s := range p.Items {
			out[str(s, "username")] = str(s, "student_id")
		}
		if !p.Page.HasMore || p.Page.NextCursor == nil || *p.Page.NextCursor == "" {
			return out, nil
		}
		cursor = *p.Page.NextCursor
	}
	return nil, fmt.Errorf("students of grade %d: too many pages", grade)
}

// assignment finds the assignment titled a.Title for (school, grade) or creates it as a draft, then publishes a draft.
func (c AssignmentClient) assignment(ctx context.Context, h map[string]string, schoolID string, res Result, a AssignmentSeed,
	exclude []string, openAt, dueAt time.Time) (uuid.UUID, int, int, error) {
	q := url.Values{"school_id": {schoolID}, "grade_level": {strconv.Itoa(a.GradeLevel)}, "limit": {"100"}}
	var found page
	if _, err := Do(ctx, c.HTTP, c.BaseURL, c.Token, http.MethodGet, "/lms/assignments?"+q.Encode(), h, nil, &found); err != nil {
		return uuid.Nil, 0, 0, err
	}
	var item map[string]any
	for _, it := range found.Data {
		if str(it, "title") == a.Title && str(it, "status") != "cancelled" && str(it, "status") != "archived" {
			item = it
		}
	}
	created := 0
	etag := ""
	if item == nil {
		contentID, err := res.contentOf(a.Content)
		if err != nil {
			return uuid.Nil, 0, 0, err
		}
		contentType := a.Content
		if strings.HasPrefix(contentType, lessonActivityPrefix) {
			contentType = "activity"
		}
		target := map[string]any{"target_type": "grade", "grade_level": a.GradeLevel}
		if len(exclude) > 0 {
			target["exclude_student_ids"] = exclude
		}
		body := map[string]any{"title": a.Title, "school_id": schoolID, "subject_id": res.SubjectID, "mode": a.Mode,
			"content_ref": map[string]any{"content_type": contentType, "content_id": contentID.String()},
			"targets":     []any{target},
			"open_at":     openAt.Format(time.RFC3339), "due_at": dueAt.Format(time.RFC3339), "max_attempts": a.MaxAttempts}
		if a.Instructions != "" {
			body["instructions"] = a.Instructions
		}
		tag, err := Do(ctx, c.HTTP, c.BaseURL, c.Token, http.MethodPost, "/lms/assignments", h, body, &item)
		if err != nil {
			return uuid.Nil, 0, 0, err
		}
		etag, created = tag, 1
	}
	id, err := uuid.Parse(str(item, "assignment_id"))
	if err != nil {
		return uuid.Nil, created, 0, fmt.Errorf("assignment without assignment_id: %w", err)
	}
	if str(item, "status") != "draft" {
		return id, created, 0, nil
	}
	path := "/lms/assignments/" + id.String()
	if etag == "" {
		if etag, err = Do(ctx, c.HTTP, c.BaseURL, c.Token, http.MethodGet, path, h, nil, nil); err != nil {
			return id, created, 0, err
		}
	}
	ph := map[string]string{"X-Tenant-Id": h["X-Tenant-Id"], "If-Match": etag}
	if _, err := Do(ctx, c.HTTP, c.BaseURL, c.Token, http.MethodPost, path+":publish", ph, nil, nil); err != nil {
		return id, created, 0, err
	}
	return id, created, 1, nil
}
