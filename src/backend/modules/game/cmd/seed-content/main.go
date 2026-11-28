// Command seed-content publishes the DEV-ONLY mini-game configs of module game (seed/minigames: two configs of Toán
// lớp 3 and the quest "Phiên chợ của bác Gấu Nâu" that plays them) through the Game Operations Console of the running
// stack (F078, F073 handoff): create → submit_review → approve → publish, so the question sources are checked by
// validateMinigameQuestionSource of module learning; then assigns them to the demo tenants through the System Admin API
// (ADR-0029). The learning ids of the files (subject, objectives) are placeholders resolved by code through the CMS API
// before anything is created (learning_refs.json), so the selection rules draw from the DEV pool of `make seed-content`.
// Accounts: author gd.thietke (game_designer), reviewer cm.duyet (content_reviewer), publisher lo.vanhanh
// (liveops), System Admin sa.admin for the assignments (demo users of the dev realm). Running it again
// publishes nothing new. LOCAL_DEVELOPMENT §5: `make seed-game-content` after `make seed-content`.
//
// Configuration (no default URL in code, AGENTS R3): EW_SEED_API_URL (public gateway, …/api/v1),
// EW_SEED_INTERNAL_API_URL (internal gateway, …/api/v1), EW_SEED_PASSWORD (demo password, never logged), EW_ENV=local.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func main() {
	dir := flag.String("dir", filepath.Join("modules", "game", "seed", "minigames"), "mini-game seed directory")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dir); err != nil {
		slog.Error("game content seed failed", "error", err)
		os.Exit(1)
	}
}

func env(k string) (string, error) {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return "", fmt.Errorf("set %s", k)
	}
	return v, nil
}

// seedItem is one file of the directory: its console type and its authored payload.
type seedItem struct {
	configType string
	idField    string
	payload    map[string]any
}

func load(dir string) ([]seedItem, map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var items []seedItem
	var quests []seedItem
	assign := map[string][]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // seed file of the repository
		if err != nil {
			return nil, nil, err
		}
		if e.Name() == learningRefsFile {
			continue // placeholders resolved by learningRefs
		}
		if e.Name() == "assignments.v1.json" {
			var f struct {
				Assignments      []map[string]any `json:"assignments"`
				QuestAssignments []map[string]any `json:"quest_assignments"`
			}
			if err := json.Unmarshal(b, &f); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
			}
			for _, a := range append(f.Assignments, f.QuestAssignments...) {
				id, _ := a["quest_id"].(string)
				if id == "" {
					id, _ = a["minigame_config_id"].(string)
				}
				for _, t := range a["tenants"].([]any) {
					assign[id] = append(assign[id], t.(string))
				}
			}
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, ok := m["minigame_config_id"]; ok {
			items = append(items, seedItem{configType: "minigame-configs", idField: "minigame_config_id", payload: m})
		} else {
			quests = append(quests, seedItem{configType: "quests", idField: "quest_id", payload: m})
		}
	}
	return append(items, quests...), assign, nil // the configs before the quest that plays them
}

func run(ctx context.Context, dir string) error {
	if os.Getenv("EW_ENV") != "local" {
		return errors.New("seed-content is a local dev seed: EW_ENV must be local")
	}
	api, err := env("EW_SEED_API_URL")
	if err != nil {
		return err
	}
	internal, err := env("EW_SEED_INTERNAL_API_URL")
	if err != nil {
		return err
	}
	pw, err := env("EW_SEED_PASSWORD")
	if err != nil {
		return err
	}
	items, assignments, err := load(dir)
	if err != nil {
		return err
	}
	hc := &http.Client{Timeout: 30 * time.Second}
	admin, err := login(ctx, hc, api, "sa.admin", pw)
	if err != nil {
		return err
	}
	author, err := login(ctx, hc, api, "gd.thietke", pw)
	if err != nil {
		return err
	}
	publisher, err := login(ctx, hc, api, "lo.vanhanh", pw)
	if err != nil {
		return err
	}
	reviewer, err := login(ctx, hc, api, "cm.duyet", pw)
	if err != nil {
		return err
	}
	// The learning ids of the files are placeholders: resolve them by code on the running stack (learning_refs.json).
	resolve, err := learningRefs(ctx, hc, api, reviewer, dir)
	if err != nil {
		return err
	}
	published := 0
	for _, it := range items {
		it.payload = replaceIDs(it.payload, resolve).(map[string]any)
		n, err := publishOne(ctx, hc, internal, roles{author: author, reviewer: reviewer, publisher: publisher}, it)
		if err != nil {
			return err
		}
		published += n
	}
	tenants := map[string]string{}
	for id, codes := range assignments {
		ids := make([]string, 0, len(codes))
		for _, code := range codes {
			tid, ok := tenants[code]
			if !ok {
				if tid, err = tenantID(ctx, hc, internal, admin, code); err != nil {
					return err
				}
				tenants[code] = tid
			}
			ids = append(ids, tid)
		}
		rt := "quest"
		if slices.ContainsFunc(items, func(s seedItem) bool {
			return s.configType == "minigame-configs" && s.payload["minigame_config_id"] == id
		}) {
			rt = "minigame_config"
		}
		if _, err := do(ctx, hc, internal, admin, http.MethodPost, "/admin/tenant-assignments", nil,
			map[string]any{"resource_type": rt, "resource_id": id, "tenant_ids": ids}, nil); err != nil {
			return err
		}
	}
	slog.InfoContext(ctx, "game content seed done", "items", len(items), "learning_refs", len(resolve), "published_now", published,
		"assigned", len(assignments))
	return nil
}

// publishOne creates the item (409 = it exists: its current state is kept), then takes version 1 to published.
// roles are the tokens of the console workflow.
type roles struct{ author, reviewer, publisher string }

func publishOne(ctx context.Context, hc *http.Client, base string, r roles, it seedItem) (int, error) {
	admin, reviewer := r.author, r.reviewer
	id, _ := it.payload[it.idField].(string)
	path := "/game-admin/" + it.configType + "/" + id + "/versions/1"
	var v map[string]any
	etag, err := do(ctx, hc, base, admin, http.MethodPost, "/game-admin/"+it.configType, nil, map[string]any{"payload": it.payload}, &v)
	if err != nil && !strings.Contains(err.Error(), " 409 ") {
		return 0, err
	}
	if err != nil {
		if etag, err = do(ctx, hc, base, admin, http.MethodGet, path, nil, nil, &v); err != nil {
			return 0, err
		}
	}
	steps := []struct {
		from, action, token string
	}{{"draft", "submit_review", admin}, {"in_review", "approve", reviewer}}
	for _, s := range steps {
		if v["status"] != s.from {
			continue
		}
		if etag, err = do(ctx, hc, base, s.token, http.MethodPost, path+":transition", map[string]string{"If-Match": etag},
			map[string]any{"action": s.action}, &v); err != nil {
			return 0, err
		}
	}
	if v["status"] != "approved" {
		return 0, nil // already published (or archived): nothing to do
	}
	if _, err := do(ctx, hc, base, r.publisher, http.MethodPost, path+":publish", map[string]string{"If-Match": etag},
		map[string]any{"change_note": "DEV seed"}, nil); err != nil {
		return 0, err
	}
	return 1, nil
}

func login(ctx context.Context, hc *http.Client, base, user, pw string) (string, error) {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	body := map[string]any{"username": user, "password": pw, "device_id": uuid.NewString(), "locale": "vi-VN"}
	if _, err := do(ctx, hc, base, "", http.MethodPost, "/auth/portal/login", nil, body, &out); err != nil {
		return "", fmt.Errorf("portal login %s: %w", user, err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("portal login %s: no access token", user)
	}
	return out.AccessToken, nil
}

func tenantID(ctx context.Context, hc *http.Client, base, token, code string) (string, error) {
	var p struct {
		Data []map[string]any `json:"data"`
	}
	if _, err := do(ctx, hc, base, token, http.MethodGet, "/admin/tenants?limit=100&q="+url.QueryEscape(code), nil, nil, &p); err != nil {
		return "", err
	}
	for _, t := range p.Data {
		if t["code"] == code && t["status"] == "active" {
			s, _ := t["tenant_id"].(string)
			return s, nil
		}
	}
	return "", fmt.Errorf("tenant %s not found or not active: run make seed-identity first", code)
}

// do sends one request (a fresh Idempotency-Key on every write) and decodes the JSON answer; it returns the ETag.
func do(ctx context.Context, hc *http.Client, base, token, method, path string, h map[string]string, body, out any) (string, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		rd = bytes.NewReader(b)
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
		req.Header.Set("Idempotency-Key", uuid.Must(uuid.NewV7()).String())
	}
	for k, v := range h {
		req.Header.Set(k, v)
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

// learningRefsFile maps the placeholder learning ids of the seed files to learning codes (subjects, objectives of one
// curriculum).
const learningRefsFile = "learning_refs.json"

// learningRefs resolves learningRefsFile of dir through the public CMS API of module learning (every page): placeholder id
// → id of the running stack. No file = nothing to resolve; a code that is not seeded is an error (run make seed-learning and
// make seed-content first).
func learningRefs(ctx context.Context, hc *http.Client, base, token, dir string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, learningRefsFile)) //nolint:gosec // seed file of the repository
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f struct {
		CurriculumCode string            `json:"curriculum_code"`
		Subjects       map[string]string `json:"subjects"`
		Objectives     map[string]string `json:"objectives"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", learningRefsFile, err)
	}
	byCode := func(path, idField string) (map[string]string, error) {
		items, err := listAll(ctx, hc, base, token, path)
		if err != nil {
			return nil, err
		}
		out := map[string]string{}
		for _, it := range items {
			code, _ := it["code"].(string)
			id, _ := it[idField].(string)
			out[code] = id
		}
		return out, nil
	}
	curricula, err := listAll(ctx, hc, base, token, "/cms/learning/curricula")
	if err != nil {
		return nil, err
	}
	version := ""
	for _, c := range curricula {
		if cur, ok := c["current_version"].(map[string]any); ok && c["code"] == f.CurriculumCode {
			version, _ = cur["curriculum_version_id"].(string)
		}
	}
	if version == "" {
		return nil, fmt.Errorf("curriculum %s has no published version: run make seed-learning first", f.CurriculumCode)
	}
	subjects, err := byCode("/cms/learning/subjects", "subject_id")
	if err != nil {
		return nil, err
	}
	objectives, err := byCode("/cms/learning/objectives?curriculum_version_id="+url.QueryEscape(version), "objective_id")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for placeholder, code := range f.Subjects {
		if out[placeholder] = subjects[code]; out[placeholder] == "" {
			return nil, fmt.Errorf("subject %s not found: run make seed-learning first", code)
		}
	}
	for placeholder, code := range f.Objectives {
		if out[placeholder] = objectives[code]; out[placeholder] == "" {
			return nil, fmt.Errorf("objective %s not found in %s: run make seed-learning first", code, f.CurriculumCode)
		}
	}
	return out, nil
}

// listAll reads every page of a list endpoint (page.next_cursor).
func listAll(ctx context.Context, hc *http.Client, base, token, path string) ([]map[string]any, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var out []map[string]any
	cursor := ""
	for range 100 {
		q := path + sep + "limit=100"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		var p struct {
			Data []map[string]any `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		if _, err := do(ctx, hc, base, token, http.MethodGet, q, nil, nil, &p); err != nil {
			return nil, err
		}
		out = append(out, p.Data...)
		if !p.Page.HasMore || p.Page.NextCursor == nil || *p.Page.NextCursor == "" {
			return out, nil
		}
		cursor = *p.Page.NextCursor
	}
	return nil, fmt.Errorf("%s: too many pages", path)
}

// replaceIDs returns v with every string equal to a key of ids replaced by its value (JSON values of a seed payload).
func replaceIDs(v any, ids map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = replaceIDs(e, ids)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = replaceIDs(e, ids)
		}
		return out
	case string:
		if r, ok := ids[x]; ok {
			return r
		}
	}
	return v
}
