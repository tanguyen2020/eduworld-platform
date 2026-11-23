// Package devseed loads the DEV-ONLY published learning content of LOCAL_DEVELOPMENT §5 (seed/content/*.dev.json)
// through the public CMS API of module learning — the same use cases as Content Studio (create, submit_review, approve,
// publish), never SQL — and assigns it with the curriculum to the demo tenants through the System Admin API of
// ADR-0029 (data assignment only; nothing of a student is ever copied, AGENTS R22). Running it again creates nothing:
// every item is found again by its seed tag or title and only the missing workflow steps run.
package devseed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// File is one seed file (seed/content/*.dev.json).
type File struct {
	CurriculumCode    string          `json:"curriculum_code"`
	SubjectCode       string          `json:"subject_code"`
	GradeLevel        int             `json:"grade_level"`
	TopicCode         string          `json:"topic_code"`
	ObjectiveCodes    []string        `json:"objective_codes"`
	AudioExemptReason string          `json:"audio_exempt_reason"`
	TenantCodes       []string        `json:"tenant_codes"`
	Questions         []QuestionSeed  `json:"questions"`
	QuestionSet       TitledSeed      `json:"question_set"`
	Activity          ActivitySeed    `json:"activity"`
	Lesson            LessonSeed      `json:"lesson"`
	Comment           json.RawMessage `json:"$comment,omitempty"`
}

// QuestionSeed is a question body (QuestionAuthoringInput without the curriculum fields) and its seed tag.
type QuestionSeed struct {
	Key  string         `json:"key"`
	Body map[string]any `json:"body"`
}

// TitledSeed is a question set.
type TitledSeed struct {
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
}

// ActivitySeed is a quiz activity over the seeded questions.
type ActivitySeed struct {
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Instructions string         `json:"instructions"`
	Config       map[string]any `json:"config"`
}

// LessonSeed is a lesson.
type LessonSeed struct {
	Title          string `json:"title"`
	CompletionRule string `json:"completion_rule"`
	Blocks         []any  `json:"blocks"`
}

// Validate checks the file before any call.
func (f File) Validate() error {
	switch {
	case f.CurriculumCode == "" || f.SubjectCode == "" || f.TopicCode == "" || len(f.ObjectiveCodes) == 0:
		return errors.New("devseed: curriculum_code, subject_code, topic_code and objective_codes are required")
	case f.GradeLevel < 1 || f.GradeLevel > 12:
		return errors.New("devseed: grade_level must be 1..12")
	case len(f.Questions) == 0 || f.QuestionSet.Title == "" || f.Activity.Title == "" || f.Lesson.Title == "":
		return errors.New("devseed: questions, question_set, activity and lesson are required")
	}
	seen := map[string]bool{}
	for _, q := range f.Questions {
		if q.Key == "" || seen[q.Key] || q.Body["type"] == nil {
			return fmt.Errorf("devseed: question %q: unique key and body.type required", q.Key)
		}
		seen[q.Key] = true
	}
	return nil
}

// Assigner assigns a platform resource to the demo tenants (System Admin API of ADR-0029).
type Assigner interface {
	Assign(ctx context.Context, resourceType string, id uuid.UUID) error
}

// Client calls the public CMS API (BaseURL ends with /api/v1) with the bearer tokens of a content author and of a
// different reviewer (reviewer ≠ author, CONTENT_MODEL §6).
type Client struct {
	BaseURL  string
	HTTP     *http.Client
	Author   string
	Reviewer string
	// ActivityReviewer reviews and publishes activities (learning_content.activity.review: content_admin or System Admin,
	// ADMIN_PORTAL §2.3; content_reviewer has no activity review); empty = Reviewer.
	ActivityReviewer string
}

// Result lists what the seed found or created.
type Result struct {
	CurriculumID uuid.UUID
	SubjectID    string // subject of the curriculum seed (assignment subject, RunAssignments)
	Questions    []uuid.UUID
	QuestionSet  uuid.UUID
	Activity     uuid.UUID
	Lesson       uuid.UUID
	Published    int // workflow publishes done by this run (0 on a second run)
}

// Run seeds the content of f and assigns it (with the curriculum) through a.
func Run(ctx context.Context, c Client, a Assigner, f File) (Result, error) {
	var out Result
	if err := f.Validate(); err != nil {
		return out, err
	}
	refs, err := c.curriculum(ctx, f)
	if err != nil {
		return out, err
	}
	out.CurriculumID = refs.curriculum
	out.SubjectID = refs.subject
	var versions []string
	for _, q := range f.Questions {
		id, vid, n, err := c.question(ctx, f, refs, q)
		if err != nil {
			return out, fmt.Errorf("question %s: %w", q.Key, err)
		}
		out.Questions = append(out.Questions, id)
		versions = append(versions, vid)
		out.Published += n
	}
	items := make([]any, len(versions))
	for i, v := range versions {
		items[i] = map[string]any{"question_version_id": v}
	}
	set := map[string]any{"title": map[string]any{"vi": f.QuestionSet.Title}, "subject_id": refs.subject, "grade_levels": []int{f.GradeLevel},
		"curriculum_version_id": refs.version, "topic_ids": []string{refs.topic}, "objective_ids": refs.objectives, "owner_scope": "platform",
		"content_language": "vi", "definition": map[string]any{"kind": "static", "purpose": f.QuestionSet.Purpose, "items": items}}
	if out.QuestionSet, err = c.doc(ctx, "question_set", "/cms/learning/question-sets", f.QuestionSet.Title, set, &out.Published); err != nil {
		return out, fmt.Errorf("question set: %w", err)
	}
	// A quiz takes only the question types of quiz.schema.json (allowed_question_types default).
	var qs []any
	for i, v := range versions {
		if slices.Contains([]string{"single_choice", "multiple_choice", "true_false", "image_based"}, fmt.Sprint(f.Questions[i].Body["type"])) {
			qs = append(qs, map[string]any{"question_version_id": v})
		}
	}
	cfg := map[string]any{"content": map[string]any{"question_source": map[string]any{"kind": "questions", "questions": qs}}}
	for k, v := range f.Activity.Config {
		cfg[k] = v
	}
	act := map[string]any{"activity_type": "quiz", "title": map[string]any{"vi": f.Activity.Title}, "grade_levels": []int{f.GradeLevel},
		"subject_id": refs.subject, "topic_id": refs.topic, "objective_ids": refs.objectives, "difficulty": 2, "content_language": "vi",
		"owner_scope": "platform", "description": map[string]any{"vi": f.Activity.Description},
		"instructions": map[string]any{"vi": f.Activity.Instructions}, "config": cfg}
	if f.AudioExemptReason != "" {
		act["audio_exempt_reason"] = f.AudioExemptReason
	}
	if out.Activity, err = c.doc(ctx, "activity", "/cms/learning/activities", f.Activity.Title, act, &out.Published); err != nil {
		return out, fmt.Errorf("activity: %w", err)
	}
	lesson := map[string]any{"title": map[string]any{"vi": f.Lesson.Title}, "subject_id": refs.subject, "grade_level": f.GradeLevel,
		"curriculum_version_id": refs.version, "topic_id": refs.topic, "objective_ids": refs.objectives, "owner_scope": "platform",
		"content_language": "vi", "completion_rule": f.Lesson.CompletionRule, "blocks": f.Lesson.Blocks}
	if out.Lesson, err = c.doc(ctx, "lesson", "/cms/learning/lessons", f.Lesson.Title, lesson, &out.Published); err != nil {
		return out, fmt.Errorf("lesson: %w", err)
	}
	if a == nil {
		return out, nil
	}
	assign := []struct {
		t  string
		id uuid.UUID
	}{{"curriculum", out.CurriculumID}, {"question_set", out.QuestionSet}, {"activity", out.Activity}, {"lesson", out.Lesson}}
	for _, q := range out.Questions {
		assign = append(assign, struct {
			t  string
			id uuid.UUID
		}{"question", q})
	}
	for _, x := range assign {
		if err := a.Assign(ctx, x.t, x.id); err != nil {
			return out, fmt.Errorf("assign %s %s: %w", x.t, x.id, err)
		}
	}
	return out, nil
}

type curriculumRefs struct {
	curriculum              uuid.UUID
	version, subject, topic string
	objectives              []string
}

// curriculum resolves the published version, subject, topic and objectives of the file (platform curriculum).
func (c Client) curriculum(ctx context.Context, f File) (curriculumRefs, error) {
	var r curriculumRefs
	var cs page
	if err := c.get(ctx, c.Author, "/cms/learning/curricula?limit=100", &cs); err != nil {
		return r, err
	}
	for _, it := range cs.Data {
		if str(it, "code") != f.CurriculumCode {
			continue
		}
		r.curriculum, _ = uuid.Parse(str(it, "curriculum_id"))
		if cur, ok := it["current_version"].(map[string]any); ok {
			r.version = str(cur, "curriculum_version_id")
		}
	}
	if r.version == "" {
		return r, fmt.Errorf("curriculum %s has no published version: run make seed-learning first", f.CurriculumCode)
	}
	var subjects page
	if err := c.get(ctx, c.Author, "/cms/learning/subjects?limit=100", &subjects); err != nil {
		return r, err
	}
	for _, s := range subjects.Data {
		if str(s, "code") == f.SubjectCode {
			r.subject = str(s, "subject_id")
		}
	}
	var topics page
	if err := c.get(ctx, c.Author, "/cms/learning/topics?limit=100&curriculum_version_id="+r.version, &topics); err != nil {
		return r, err
	}
	for _, t := range topics.Data {
		if str(t, "code") == f.TopicCode {
			r.topic = str(t, "topic_id")
		}
	}
	if r.subject == "" || r.topic == "" {
		return r, fmt.Errorf("subject %s or topic %s not found in %s", f.SubjectCode, f.TopicCode, f.CurriculumCode)
	}
	var objs page
	if err := c.get(ctx, c.Author, "/cms/learning/objectives?limit=100&curriculum_version_id="+r.version+"&topic_id="+r.topic, &objs); err != nil {
		return r, err
	}
	for _, code := range f.ObjectiveCodes {
		i := slices.IndexFunc(objs.Data, func(o map[string]any) bool { return str(o, "code") == code })
		if i < 0 {
			return r, fmt.Errorf("objective %s not found in topic %s", code, f.TopicCode)
		}
		r.objectives = append(r.objectives, str(objs.Data[i], "objective_id"))
	}
	return r, nil
}

// question finds the question tagged q.Key (or creates it) and publishes version 1; it returns the content id, the
// published version id and the number of publishes done.
func (c Client) question(ctx context.Context, f File, r curriculumRefs, q QuestionSeed) (uuid.UUID, string, int, error) {
	stem := ""
	if b, ok := q.Body["stem"].(map[string]any)["blocks"].([]any); ok && len(b) > 0 {
		stem, _ = b[0].(map[string]any)["text"].(string)
	}
	var found page
	qs := url.Values{"q": {stem}, "grade_level": {strconv.Itoa(f.GradeLevel)}, "limit": {"100"}}
	if err := c.get(ctx, c.Author, "/cms/learning/questions?"+qs.Encode(), &found); err != nil {
		return uuid.Nil, "", 0, err
	}
	var item map[string]any
	for _, it := range found.Data {
		if tags, ok := it["tags"].([]any); ok && slices.Contains(tags, any(q.Key)) {
			item = it
		}
	}
	if item == nil {
		body := map[string]any{"owner_scope": "platform", "grade_levels": []int{f.GradeLevel}, "subject_id": r.subject, "topic_id": r.topic,
			"curriculum_version_id": r.version, "objective_ids": r.objectives, "content_language": "vi", "tags": []string{q.Key}}
		if f.AudioExemptReason != "" {
			body["audio_exempt_reason"] = f.AudioExemptReason
		}
		for k, v := range q.Body {
			body[k] = v
		}
		if err := c.send(ctx, c.Author, http.MethodPost, "/cms/learning/questions", nil, body, &item); err != nil {
			return uuid.Nil, "", 0, err
		}
	}
	id, err := uuid.Parse(str(item, "content_id"))
	if err != nil {
		return uuid.Nil, "", 0, fmt.Errorf("question without content_id: %w", err)
	}
	v, n, err := c.publish(ctx, "question", id.String(), 1)
	if err != nil {
		return id, "", n, err
	}
	return id, str(v, "content_version_id"), n, nil
}

// doc finds the item of type ctype titled title in list (or creates it from body) and publishes version 1.
func (c Client) doc(ctx context.Context, ctype, list, title string, body map[string]any, published *int) (uuid.UUID, error) {
	idKey := "content_id"
	query := "?limit=100&q=" + url.QueryEscape(title)
	if ctype == "activity" {
		idKey, query = "activity_id", "?limit=100&activity_type=quiz"
	}
	var found page
	if err := c.get(ctx, c.Author, list+query, &found); err != nil {
		return uuid.Nil, err
	}
	var item map[string]any
	for _, it := range found.Data {
		if t, ok := it["title"].(map[string]any); ok && t["vi"] == title && str(it, "owner_scope") != "tenant" {
			item = it
		}
	}
	if item == nil {
		if err := c.send(ctx, c.Author, http.MethodPost, list, nil, body, &item); err != nil {
			return uuid.Nil, err
		}
	}
	id, err := uuid.Parse(str(item, idKey))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s without %s: %w", ctype, idKey, err)
	}
	_, n, err := c.publish(ctx, ctype, id.String(), 1)
	*published += n
	return id, err
}

// publish runs the missing steps draft → in_review → approved → published of a version (author submits, reviewer
// approves with the audio exemption acknowledged, reviewer publishes); it returns the version and 1 when it published.
func (c Client) publish(ctx context.Context, ctype, id string, version int) (map[string]any, int, error) {
	path := fmt.Sprintf("/cms/learning/content/%s/%s/versions/%d", ctype, id, version)
	reviewer := c.Reviewer
	if ctype == "activity" && c.ActivityReviewer != "" {
		reviewer = c.ActivityReviewer
	}
	for range 4 {
		var v map[string]any
		etag, err := c.getETag(ctx, c.Author, path, &v)
		if err != nil {
			return nil, 0, err
		}
		h := map[string]string{"If-Match": etag}
		switch str(v, "status") {
		case "published":
			return v, 0, nil
		case "draft", "rejected":
			err = c.send(ctx, c.Author, http.MethodPost, path+":transition", h, map[string]any{"action": "submit_review"}, nil)
		case "in_review":
			err = c.send(ctx, reviewer, http.MethodPost, path+":transition", h,
				map[string]any{"action": "approve", "audio_exemption_acknowledged": true, "note": "dev seed"}, nil)
		case "approved":
			if err = c.send(ctx, reviewer, http.MethodPost, path+":publish", h, nil, nil); err == nil {
				_, _ = c.getETag(ctx, c.Author, path, &v)
				return v, 1, nil
			}
		default:
			return nil, 0, fmt.Errorf("%s %s v%d: unexpected status %q", ctype, id, version, str(v, "status"))
		}
		if err != nil {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("%s %s v%d: not published after the workflow", ctype, id, version)
}

type page struct {
	Data []map[string]any `json:"data"`
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func (c Client) get(ctx context.Context, token, path string, out any) error {
	_, err := c.call(ctx, token, http.MethodGet, path, nil, nil, out)
	return err
}

func (c Client) getETag(ctx context.Context, token, path string, out any) (string, error) {
	return c.call(ctx, token, http.MethodGet, path, nil, nil, out)
}

func (c Client) send(ctx context.Context, token, method, path string, h map[string]string, body, out any) error {
	_, err := c.call(ctx, token, method, path, h, body, out)
	return err
}

// call sends one request (a fresh Idempotency-Key on every write) and decodes the JSON answer; it returns the ETag.
func (c Client) call(ctx context.Context, token, method, path string, h map[string]string, body, out any) (string, error) {
	return Do(ctx, c.HTTP, c.BaseURL, token, method, path, h, body, out)
}

// Do is the HTTP helper of the seed (also used by the System Admin assigner of cmd/seed-content).
func Do(ctx context.Context, hc *http.Client, base, token, method, path string, h map[string]string, body, out any) (string, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		rd = bytes.NewReader(b)
	} else if method != http.MethodGet {
		rd = bytes.NewReader([]byte("{}"))
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(base, "/")+path, rd)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.Must(uuid.NewV7()).String()) // UUIDv7 (API_DESIGN)
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		var p struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &p)
		return "", fmt.Errorf("%s %s: %d %s %s", method, path, res.StatusCode, p.Code, p.Detail)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return "", fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	return res.Header.Get("ETag"), nil
}
