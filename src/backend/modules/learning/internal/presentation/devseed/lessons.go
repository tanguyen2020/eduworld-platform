package devseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// LessonFile is a DEV-ONLY seed of extra platform content (seed/content/*.dev.json with "activities", LOCAL_DEVELOPMENT
// §5): media that goes through the real media pipeline (F027: presign → PUT → complete-upload → worker transcode),
// questions (optionally with corrected later versions, R11: a published version is never edited, a correction is a new
// version), and activities of any registered type whose config references the media and questions of the file through
// placeholders — "{{media:<key>}}" (asset id), "{{question:<key>}}" (version 1 id), "{{question:<key>@<n>}}" (version n
// id). Everything goes through the public CMS API with the same workflow as seed/content/math_g3.dev.json (author ≠
// reviewer) and is either platform content assigned to TenantCodes (ADR-0029) or, with OwnerTenantCode, tenant content
// of that tenant (the media lessons are tenant content; since 2026-11-28 platform media is assigned like the rest —
// resource_type media — so such a file may also be platform content with TenantCodes). Running it again creates
// nothing: media is found by file name, questions by tag, activities by type and title.
type LessonFile struct {
	Comment           json.RawMessage  `json:"$comment,omitempty"`
	CurriculumCode    string           `json:"curriculum_code"`
	SubjectCode       string           `json:"subject_code"`
	GradeLevel        int              `json:"grade_level"`
	AudioExemptReason string           `json:"audio_exempt_reason"`
	TenantCodes       []string         `json:"tenant_codes,omitempty"`
	OwnerTenantCode   string           `json:"owner_tenant_code,omitempty"`
	Media             []MediaSeed      `json:"media"`
	Questions         []LessonQuestion `json:"questions"`
	Activities        []LessonActivity `json:"activities"`
}

// LessonQuestion is a question of a topic; Versions are merge patches (QuestionAuthoringInput) of versions 2, 3, …
// created from the previous version and published after the activities (a corrected answer key, F058 US4).
type LessonQuestion struct {
	Key            string           `json:"key"`
	TopicCode      string           `json:"topic_code"`
	ObjectiveCodes []string         `json:"objective_codes"`
	Body           map[string]any   `json:"body"`
	Versions       []map[string]any `json:"versions,omitempty"`
}

// LessonActivity is an activity of a topic; Config is the activity config with placeholders.
type LessonActivity struct {
	Key            string         `json:"key"`
	ActivityType   string         `json:"activity_type"`
	TopicCode      string         `json:"topic_code"`
	ObjectiveCodes []string       `json:"objective_codes"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Instructions   string         `json:"instructions"`
	Difficulty     int            `json:"difficulty"`
	Config         map[string]any `json:"config"`
}

var (
	seedKey     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
	placeholder = regexp.MustCompile(`^\{\{(media|question):([a-z0-9][a-z0-9_-]{0,62})(?:@([1-9][0-9]?))?\}\}$`)
	// activityTypes are the registered activity types a lesson file may create (listActivityTypes, F030).
	activityTypes = []string{"quiz", "interactive_exercise", "video_lesson", "audio_lesson", "simulation"}
)

// Validate checks the file before any call: unique keys, known activity types, [DEV] titles, placeholders that name a
// media or question of the file (and an existing version).
func (f LessonFile) Validate() error {
	switch {
	case f.CurriculumCode == "" || f.SubjectCode == "":
		return errors.New("devseed: curriculum_code and subject_code are required")
	case f.GradeLevel < 1 || f.GradeLevel > 12:
		return errors.New("devseed: grade_level must be 1..12")
	case len(f.Activities) == 0:
		return errors.New("devseed: at least one activity is required")
	case (len(f.TenantCodes) == 0) == (f.OwnerTenantCode == ""):
		return errors.New("devseed: exactly one of tenant_codes (platform content) and owner_tenant_code (tenant content)")
	}
	media := map[string]bool{}
	for _, m := range f.Media {
		if !seedKey.MatchString(m.Key) || media[m.Key] {
			return fmt.Errorf("devseed: media %q: unique key [a-z0-9_-] required", m.Key)
		}
		if err := m.Validate(); err != nil {
			return err
		}
		media[m.Key] = true
	}
	versions := map[string]int{}
	for _, q := range f.Questions {
		if !seedKey.MatchString(q.Key) || versions[q.Key] > 0 || q.Body["type"] == nil || q.TopicCode == "" || len(q.ObjectiveCodes) == 0 {
			return fmt.Errorf("devseed: question %q: unique key, body.type, topic_code and objective_codes required", q.Key)
		}
		versions[q.Key] = 1 + len(q.Versions)
	}
	seen := map[string]bool{}
	for _, a := range f.Activities {
		switch {
		case !seedKey.MatchString(a.Key) || seen[a.Key]:
			return fmt.Errorf("devseed: activity %q: unique key required", a.Key)
		case !slices.Contains(activityTypes, a.ActivityType):
			return fmt.Errorf("devseed: activity %q: activity_type must be one of %v", a.Key, activityTypes)
		case !strings.HasPrefix(a.Title, "[DEV] ") || a.TopicCode == "" || len(a.ObjectiveCodes) == 0 || a.Config == nil:
			return fmt.Errorf("devseed: activity %q: [DEV] title, topic_code, objective_codes and config required", a.Key)
		case a.Difficulty < 0 || a.Difficulty > 5:
			return fmt.Errorf("devseed: activity %q: difficulty must be 0..5", a.Key)
		}
		seen[a.Key] = true
		if err := walkPlaceholders(a.Config, func(kind, key string, n int) error {
			if (kind == "media" && !media[key]) || (kind == "question" && n > versions[key]) {
				return fmt.Errorf("devseed: activity %q: placeholder %s:%s@%d names nothing of the file", a.Key, kind, key, n)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// walkPlaceholders calls fn for every placeholder string of v (n = version, 1 when absent; 0 for media).
func walkPlaceholders(v any, fn func(kind, key string, n int) error) error {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(x) {
			if err := walkPlaceholders(x[k], fn); err != nil {
				return err
			}
		}
	case []any:
		for _, e := range x {
			if err := walkPlaceholders(e, fn); err != nil {
				return err
			}
		}
	case string:
		if m := placeholder.FindStringSubmatch(x); m != nil {
			n := 1
			if m[1] == "media" {
				n = 0
			} else if m[3] != "" {
				n, _ = strconv.Atoi(m[3])
			}
			return fn(m[1], m[2], n)
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// resolve returns a deep copy of v with every placeholder replaced by its id.
func resolve(v any, media map[string]uuid.UUID, versions map[string][]string) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			r, err := resolve(e, media, versions)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			r, err := resolve(e, media, versions)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case string:
		m := placeholder.FindStringSubmatch(x)
		if m == nil {
			return x, nil
		}
		if m[1] == "media" {
			id, ok := media[m[2]]
			if !ok {
				return nil, fmt.Errorf("devseed: media %s not seeded", m[2])
			}
			return id.String(), nil
		}
		n := 1
		if m[3] != "" {
			n, _ = strconv.Atoi(m[3])
		}
		vs := versions[m[2]]
		if n > len(vs) || vs[n-1] == "" {
			return nil, fmt.Errorf("devseed: question %s has no published version %d yet", m[2], n)
		}
		return vs[n-1], nil
	}
	return v, nil
}

// LessonResult lists what RunLessons found or created.
type LessonResult struct {
	SubjectID string
	Media     map[string]uuid.UUID // media key → asset id
	Questions map[string]uuid.UUID // question key → content id
	// Versions are the published version ids of each question key (index 0 = version 1).
	Versions   map[string][]string
	Activities map[string]uuid.UUID // activity key → activity id
	Published  int                  // workflow publishes done by this run (0 on a second run)
	Uploaded   int                  // media uploaded by this run (0 on a second run)
}

// QuestionKeyOf returns the seed key of a published question version id ("" when it is not of the file).
func (r LessonResult) QuestionKeyOf(versionID string) string {
	for k, vs := range r.Versions {
		if slices.Contains(vs, versionID) {
			return k
		}
	}
	return ""
}

// RunLessons seeds f: media through m (nil when the file has no media), questions version 1, activities (published),
// then the corrected question versions, and assigns everything to the tenants of a (nil = no assignment).
func RunLessons(ctx context.Context, c Client, m *MediaClient, a Assigner, f LessonFile) (LessonResult, error) {
	out := LessonResult{Media: map[string]uuid.UUID{}, Questions: map[string]uuid.UUID{}, Versions: map[string][]string{},
		Activities: map[string]uuid.UUID{}}
	if err := f.Validate(); err != nil {
		return out, err
	}
	if len(f.Media) > 0 && m == nil {
		return out, errors.New("devseed: the lesson file has media but no media client")
	}
	for _, ms := range f.Media {
		id, uploaded, err := m.Ensure(ctx, ms)
		if err != nil {
			return out, fmt.Errorf("media %s: %w", ms.Key, err)
		}
		out.Media[ms.Key] = id
		if uploaded {
			out.Uploaded++
		}
	}
	refs := map[string]curriculumRefs{}
	refsOf := func(topic string, objectives []string) (curriculumRefs, error) {
		k := topic + "|" + strings.Join(objectives, ",")
		if r, ok := refs[k]; ok {
			return r, nil
		}
		r, err := c.curriculum(ctx, File{CurriculumCode: f.CurriculumCode, SubjectCode: f.SubjectCode, TopicCode: topic, ObjectiveCodes: objectives})
		if err == nil {
			refs[k] = r
		}
		return r, err
	}
	for _, q := range f.Questions {
		r, err := refsOf(q.TopicCode, q.ObjectiveCodes)
		if err != nil {
			return out, err
		}
		out.SubjectID = r.subject
		id, vid, n, err := c.question(ctx, f.GradeLevel, f.AudioExemptReason, r, QuestionSeed{Key: q.Key, Body: q.Body})
		if err != nil {
			return out, fmt.Errorf("question %s: %w", q.Key, err)
		}
		out.Questions[q.Key] = id
		out.Versions[q.Key] = []string{vid}
		out.Published += n
	}
	// Later versions that already exist (second run) are needed by the activities that pin them.
	for _, q := range f.Questions {
		for i := range q.Versions {
			v, err := c.version(ctx, "question", out.Questions[q.Key].String(), i+2)
			if err != nil {
				return out, fmt.Errorf("question %s v%d: %w", q.Key, i+2, err)
			}
			vid := ""
			if v != nil && str(v, "status") == "published" {
				vid = str(v, "content_version_id")
			}
			out.Versions[q.Key] = append(out.Versions[q.Key], vid)
		}
	}
	for _, act := range f.Activities {
		r, err := refsOf(act.TopicCode, act.ObjectiveCodes)
		if err != nil {
			return out, err
		}
		out.SubjectID = r.subject
		cfg, err := resolve(act.Config, out.Media, out.Versions)
		if err != nil {
			return out, fmt.Errorf("activity %s: %w", act.Key, err)
		}
		body := map[string]any{"activity_type": act.ActivityType, "title": map[string]any{"vi": act.Title}, "grade_levels": []int{f.GradeLevel},
			"subject_id": r.subject, "topic_id": r.topic, "objective_ids": r.objectives, "difficulty": act.Difficulty, "content_language": "vi",
			"owner_scope": c.owner(), "description": map[string]any{"vi": act.Description},
			"instructions": map[string]any{"vi": act.Instructions}, "config": cfg}
		if f.AudioExemptReason != "" {
			body["audio_exempt_reason"] = f.AudioExemptReason
		}
		id, err := c.doc(ctx, "activity", "/cms/learning/activities", act.Title, body, &out.Published)
		if err != nil {
			return out, fmt.Errorf("activity %s: %w", act.Key, err)
		}
		out.Activities[act.Key] = id
	}
	for _, q := range f.Questions {
		for i, patch := range q.Versions {
			vid, n, err := c.correction(ctx, out.Questions[q.Key].String(), i+2, patch)
			if err != nil {
				return out, fmt.Errorf("question %s v%d: %w", q.Key, i+2, err)
			}
			out.Versions[q.Key][i+1] = vid
			out.Published += n
		}
	}
	if a == nil {
		return out, nil
	}
	type res struct {
		t  string
		id uuid.UUID
	}
	var all []res
	for _, ms := range f.Media {
		all = append(all, res{"media", out.Media[ms.Key]})
	}
	for _, q := range f.Questions {
		all = append(all, res{"question", out.Questions[q.Key]})
	}
	for _, act := range f.Activities {
		all = append(all, res{"activity", out.Activities[act.Key]})
	}
	for _, x := range all {
		if err := a.Assign(ctx, x.t, x.id); err != nil {
			return out, fmt.Errorf("assign %s %s: %w", x.t, x.id, err)
		}
	}
	return out, nil
}

// version returns version n of an item (nil when it does not exist).
func (c Client) version(ctx context.Context, ctype, id string, n int) (map[string]any, error) {
	var v map[string]any
	err := c.get(ctx, c.Author, fmt.Sprintf("/cms/learning/content/%s/%s/versions/%d", ctype, id, n), &v)
	if IsStatus(err, http.StatusNotFound) {
		return nil, nil
	}
	return v, err
}

// correction makes version n of a question from version n-1 (createQuestionVersion), applies the merge patch while it
// is a draft (updateQuestionDraft) and runs the workflow to published; it returns the version id and 1 when it
// published.
func (c Client) correction(ctx context.Context, id string, n int, patch map[string]any) (string, int, error) {
	v, err := c.version(ctx, "question", id, n)
	if err != nil {
		return "", 0, err
	}
	if v == nil {
		if err := c.send(ctx, c.Author, http.MethodPost, "/cms/learning/questions/"+id+"/versions", nil,
			map[string]any{"from_version": n - 1, "note": "dev seed: corrected answer key"}, nil); err != nil {
			return "", 0, err
		}
		if v, err = c.version(ctx, "question", id, n); err != nil || v == nil {
			return "", 0, fmt.Errorf("version %d not created: %w", n, err)
		}
	}
	if s := str(v, "status"); s == "draft" || s == "rejected" {
		path := fmt.Sprintf("/cms/learning/questions/%s/versions/%d", id, n)
		etag, err := c.getETag(ctx, c.Author, path, nil)
		if err != nil {
			return "", 0, err
		}
		if err := c.send(ctx, c.Author, http.MethodPatch, path, map[string]string{"If-Match": etag, "Content-Type": "application/merge-patch+json"},
			patch, nil); err != nil {
			return "", 0, err
		}
	}
	pv, published, err := c.publish(ctx, "question", id, n)
	if err != nil {
		return "", published, err
	}
	return str(pv, "content_version_id"), published, nil
}
