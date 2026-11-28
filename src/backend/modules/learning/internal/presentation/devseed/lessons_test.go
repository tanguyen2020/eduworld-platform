package devseed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func seedPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "..", "seed"}, parts...)...)
}

// The extra seed files of the repository are valid and consistent with each other: [DEV] titles, generated (not
// committed) video/audio with the Theora/Vorbis variants of the game client, subtitles from small text files, the
// F065/F066 shapes (cue at 01:30 within a ≥ 3:12 video, 2 chapters, vi subtitle, 2 audio plays, lever template), the
// faulty question of F058 with its correction, assignments that name activities of the files, and a student script
// over the golden questions and assignments.
func TestExtraSeedFilesValid(t *testing.T) {
	var media, golden LessonFile
	for path, f := range map[string]*LessonFile{seedPath("content", "media_g3.dev.json"): &media, seedPath("content", "golden_g3.dev.json"): &golden} {
		if err := LoadJSON(path, f); err != nil {
			t.Fatal(err)
		}
		if f.GradeLevel != 3 || f.SubjectCode != "math" || len(f.Comment) == 0 {
			t.Fatalf("%s: unexpected header", path)
		}
	}
	// Media lessons are tenant content of th-demo-a (platform media cannot be assigned to a tenant yet); the golden
	// content is platform content assigned to th-demo-a only.
	if media.OwnerTenantCode != "th-demo-a" || len(golden.TenantCodes) != 1 || golden.TenantCodes[0] != "th-demo-a" {
		t.Fatalf("owners: media %q, golden %v", media.OwnerTenantCode, golden.TenantCodes)
	}
	for _, m := range media.Media {
		switch m.Kind {
		case "video":
			if m.Generate == nil || m.Generate.DurationS < 193 || !slices.Contains(m.Variants, "ogv_480") || !slices.Contains(m.Variants, "ogv_720") {
				t.Fatalf("video %+v: generated, ≥ 3:13 (resume at 03:12), Theora variants", m)
			}
		case "audio":
			if m.Generate == nil || !slices.Contains(m.Variants, "ogg") {
				t.Fatalf("audio %+v", m)
			}
		case "subtitle":
			b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", filepath.FromSlash(m.File)))
			if err != nil || !strings.HasPrefix(string(b), "WEBVTT") || !strings.Contains(string(b), "[DEV]") || len(b) > 4096 {
				t.Fatalf("subtitle %s: small [DEV] WebVTT file required (%v)", m.File, err)
			}
		}
	}
	byType := map[string]LessonActivity{}
	for _, a := range media.Activities {
		byType[a.ActivityType] = a
	}
	video := byType["video_lesson"].Config["content"].(map[string]any)
	cues := video["embedded_questions"].([]any)
	if len(cues) != 1 || cues[0].(map[string]any)["at_ms"] != float64(90000) || len(video["chapters"].([]any)) != 2 ||
		video["subtitles"].([]any)[0].(map[string]any)["language"] != "vi" || video["duration_ms"] != float64(200000) {
		t.Fatalf("video lesson: %v", video)
	}
	if byType["audio_lesson"].Config["interaction"].(map[string]any)["max_plays"] != float64(2) ||
		byType["simulation"].Config["content"].(map[string]any)["template_code"] != "physics.lever" {
		t.Fatal("audio lesson plays or simulation template")
	}
	i := slices.IndexFunc(golden.Questions, func(q LessonQuestion) bool { return len(q.Versions) > 0 })
	if i < 0 || golden.Questions[i].Body["answer_key"].(map[string]any)["correct_option_id"] ==
		golden.Questions[i].Versions[0]["answer_key"].(map[string]any)["correct_option_id"] {
		t.Fatal("the regrade question needs a faulty version 1 and a corrected version 2")
	}
	keys := map[string]bool{}
	for _, f := range []LessonFile{media, golden} {
		for _, a := range f.Activities {
			keys[a.Key] = true
		}
	}
	assigned := map[string]bool{}
	for _, name := range []string{"demoa1_g3_media.dev.json", "demoa1_g3_golden.dev.json"} {
		var af AssignmentFile
		if err := LoadJSON(seedPath("assignments", name), &af); err != nil {
			t.Fatal(err)
		}
		for _, a := range af.Assignments {
			if !strings.HasPrefix(a.Title, "[DEV]") || !keys[strings.TrimPrefix(a.Content, lessonActivityPrefix)] || a.GradeLevel != 3 {
				t.Fatalf("%s: %+v", name, a)
			}
			if name == "demoa1_g3_media.dev.json" && !slices.Contains(a.ExcludeUsernames, "hs.chi.3b") {
				t.Fatalf("media assignment %q must exclude hs.chi.3b (F055 T015)", a.Title)
			}
			assigned[a.Title] = true
		}
	}
	var sf AttemptFile
	if err := LoadJSON(seedPath("attempts", "demoa1_g3_golden.dev.json"), &sf); err != nil {
		t.Fatal(err)
	}
	questions := map[string]bool{}
	for _, q := range golden.Questions {
		questions[q.Key] = true
	}
	for _, s := range sf.Students {
		for _, a := range s.Attempts {
			if !assigned[a.Assignment] {
				t.Fatalf("script %s: assignment %q not seeded", s.Username, a.Assignment)
			}
			for _, ans := range a.Answers {
				if !questions[ans.Question] {
					t.Fatalf("script %s: question %s not in the golden file", s.Username, ans.Question)
				}
			}
		}
	}
	if sf.Expected == nil || len(sf.Expected.Objectives) != 2 || len(sf.Expected.Assignments) != 2 || sf.Expected.ActiveStudents == nil {
		t.Fatal("the golden script needs its expected dashboard values")
	}
	if sf.AnswerWaitMS < 3000 {
		t.Fatalf("answer_wait_ms %d below min_response_ms 2000 × 1.5 (g1_3)", sf.AnswerWaitMS)
	}
}

func TestLessonFileValidateRejects(t *testing.T) {
	ok := LessonFile{CurriculumCode: "C", SubjectCode: "math", GradeLevel: 3, TenantCodes: []string{"t"},
		Media:     []MediaSeed{{Key: "sub", Kind: "subtitle", Filename: "devseed-a.vtt", MIME: "text/vtt", License: "CC0", File: "a.vtt"}},
		Questions: []LessonQuestion{{Key: "q1", TopicCode: "T", ObjectiveCodes: []string{"O"}, Body: map[string]any{"type": "single_choice"}}},
		Activities: []LessonActivity{{Key: "a", ActivityType: "quiz", TopicCode: "T", ObjectiveCodes: []string{"O"}, Title: "[DEV] A",
			Config: map[string]any{"x": []any{"{{question:q1}}", "{{media:sub}}"}}}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(f *LessonFile){
		"unknown media":   func(f *LessonFile) { f.Activities[0].Config = map[string]any{"x": "{{media:nope}}"} },
		"missing version": func(f *LessonFile) { f.Activities[0].Config = map[string]any{"x": "{{question:q1@2}}"} },
		"binary from a file": func(f *LessonFile) {
			f.Media[0] = MediaSeed{Key: "v", Kind: "video", Filename: "devseed-v.mp4", MIME: "video/mp4", License: "x", File: "v.mp4"}
		},
		"not a dev title": func(f *LessonFile) { f.Activities[0].Title = "Quiz" },
		"unknown type":    func(f *LessonFile) { f.Activities[0].ActivityType = "written_response" },
		"two owners":      func(f *LessonFile) { f.OwnerTenantCode = "t" },
		"odd video size": func(f *LessonFile) {
			f.Media[0] = MediaSeed{Key: "v", Kind: "video", Filename: "devseed-v.mp4", MIME: "video/mp4", License: "x",
				Generate: &MediaGenerate{DurationS: 10, Width: 641, Height: 360, FPS: 15, ToneHz: 440}}
		},
	}
	for name, mutate := range cases {
		f := ok
		f.Media = slices.Clone(ok.Media)
		f.Activities = slices.Clone(ok.Activities)
		mutate(&f)
		if f.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	got, err := resolve(map[string]any{"a": []any{"{{question:q1@2}}", "{{media:sub}}", "keep"}}, map[string]uuid.UUID{"sub": uuid.Nil},
		map[string][]string{"q1": {"v1", "v2"}})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(got); string(b) != `{"a":["v2","00000000-0000-0000-0000-000000000000","keep"]}` {
		t.Fatalf("resolve: %s", b)
	}
	if _, err := resolve("{{question:q1@2}}", nil, map[string][]string{"q1": {"v1", ""}}); err == nil {
		t.Fatal("an unpublished version resolved")
	}
}

type fakeGen struct{ calls int }

func (g *fakeGen) Generate(context.Context, string, MediaGenerate) ([]byte, error) {
	g.calls++
	return []byte("\x00\x00\x00\x18ftypmp42 synthetic"), nil
}

// MOCK (TEST FAKE) of the media API and of the object storage: the first Ensure generates the clip, presigns, PUTs it
// to S3Addr with the signed Host of the URL and no bearer token, completes and waits for ready; the second Ensure finds
// the asset by file name and neither generates nor uploads.
func TestMediaClientUploadsOnceAndWaits(t *testing.T) {
	var mu sync.Mutex
	assets := map[string]map[string]any{}
	polls := 0
	var put *http.Request
	var putBody []byte
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		put = r.Clone(context.Background())
		putBody, _ = io.ReadAll(r.Body)
	}))
	defer storage.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer author" {
			http.Error(w, "token", http.StatusUnauthorized)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/api/v1")
		switch {
		case r.Method == http.MethodGet && p == "/cms/media":
			var data []any
			for _, a := range assets {
				if strings.Contains(a["original_filename"].(string), r.URL.Query().Get("q")) {
					data = append(data, a)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "page": map[string]any{"has_more": false}})
		case r.Method == http.MethodPost && p == "/cms/media:presign-upload":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["owner_scope"] != "platform" || in["kind"] != "video" || len(in["sha256"].(string)) != 64 || r.Header.Get("Idempotency-Key") == "" {
				http.Error(w, "presign", http.StatusUnprocessableEntity)
				return
			}
			id := uuid.NewString()
			assets[id] = map[string]any{"asset_id": id, "status": "quarantined", "owner_scope": "platform", "original_filename": in["filename"]}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"asset_id": id, "status": "quarantined",
				"upload_url": "http://floci:4566/ew-media-quarantine/q/" + id + "?X-Amz-Signature=x", "upload_headers": map[string]string{"Content-Type": "video/mp4"}})
		case r.Method == http.MethodPost && strings.HasSuffix(p, ":complete-upload"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "/cms/media/"), ":complete-upload")
			assets[id]["status"] = "processing"
			_ = json.NewEncoder(w).Encode(assets[id])
		case r.Method == http.MethodGet && strings.HasPrefix(p, "/cms/media/"):
			a := assets[strings.TrimPrefix(p, "/cms/media/")]
			if polls++; polls >= 2 {
				a["status"] = "ready"
				a["variants"] = []any{map[string]any{"variant": "ogv_480"}, map[string]any{"variant": "ogv_720"}}
			}
			_ = json.NewEncoder(w).Encode(a)
		default:
			http.Error(w, p, http.StatusNotFound)
		}
	}))
	defer api.Close()
	su, _ := url.Parse(storage.URL)
	gen := &fakeGen{}
	mc := &MediaClient{BaseURL: api.URL + "/api/v1", HTTP: api.Client(), Token: "author", Generator: gen, S3Addr: su.Host, Poll: time.Millisecond, Wait: time.Second}
	m := MediaSeed{Key: "v", Kind: "video", Filename: "devseed-v.mp4", MIME: "video/mp4", License: "CC0",
		Generate: &MediaGenerate{DurationS: 10, Width: 64, Height: 36, FPS: 5, ToneHz: 440}, Variants: []string{"ogv_480", "ogv_720"}}
	ctx := context.Background()
	id, uploaded, err := mc.Ensure(ctx, m)
	if err != nil || !uploaded || id == uuid.Nil {
		t.Fatalf("first ensure: %v %v %v", id, uploaded, err)
	}
	if put == nil || put.Host != "floci:4566" || put.Header.Get("Authorization") != "" || put.Header.Get("Content-Type") != "video/mp4" ||
		string(putBody) != "\x00\x00\x00\x18ftypmp42 synthetic" || !strings.Contains(put.URL.RawQuery, "X-Amz-Signature") {
		t.Fatalf("presigned PUT: %+v", put)
	}
	again, uploaded, err := mc.Ensure(ctx, m)
	if err != nil || uploaded || again != id || gen.calls != 1 || len(assets) != 1 {
		t.Fatalf("second ensure: %v %v %v calls=%d assets=%d", again, uploaded, err, gen.calls, len(assets))
	}
	m.Variants = []string{"ogv_480", "ogv_1080"}
	if _, _, err := mc.Ensure(ctx, m); err == nil || !strings.Contains(err.Error(), "ogv_1080") {
		t.Fatalf("missing variant not reported: %v", err)
	}
}
