package devseed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
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

// AssignmentSeed is one assignment of the seeded content (`content` = activity | question_set of the content seed).
type AssignmentSeed struct {
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	Content      string `json:"content"`
	Mode         string `json:"mode"`
	GradeLevel   int    `json:"grade_level"`
	MaxAttempts  int    `json:"max_attempts"`
}

// seedContentKinds are the content seed items an assignment can reference (auto-graded only, F041).
var seedContentKinds = []string{"activity", "question_set"}

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
		case !slices.Contains(seedContentKinds, a.Content):
			return fmt.Errorf("devseed: assignment %q: content must be one of %v", a.Title, seedContentKinds)
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
	if res.SubjectID == "" || res.Activity == uuid.Nil || res.QuestionSet == uuid.Nil {
		return out, errors.New("devseed: the content seed result has no subject, activity or question set")
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
		id, created, published, err := c.assignment(ctx, h, out.SchoolID, res, a, openAt, openAt.AddDate(0, 0, f.WindowDays))
		if err != nil {
			return out, fmt.Errorf("assignment %q: %w", a.Title, err)
		}
		out.Assignments = append(out.Assignments, id)
		out.Created += created
		out.Published += published
	}
	return out, nil
}

// assignment finds the assignment titled a.Title for (school, grade) or creates it as a draft, then publishes a draft.
func (c AssignmentClient) assignment(ctx context.Context, h map[string]string, schoolID string, res Result, a AssignmentSeed,
	openAt, dueAt time.Time) (uuid.UUID, int, int, error) {
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
		contentID := res.Activity
		if a.Content == "question_set" {
			contentID = res.QuestionSet
		}
		body := map[string]any{"title": a.Title, "school_id": schoolID, "subject_id": res.SubjectID, "mode": a.Mode,
			"content_ref": map[string]any{"content_type": a.Content, "content_id": contentID.String()},
			"targets":     []any{map[string]any{"target_type": "grade", "grade_level": a.GradeLevel}},
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
