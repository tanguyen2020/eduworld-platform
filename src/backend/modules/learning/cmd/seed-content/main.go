// Command seed-content loads the DEV-ONLY published learning content (Toán khối 3 over GDPT2018_SAMPLE,
// seed/content/*.dev.json) through the public CMS API of the running stack and assigns it, with the curriculum, to the
// demo tenants through the System Admin API (ADR-0029; sa.admin also reviews the activity, which needs
// learning_content.activity.review) — LOCAL_DEVELOPMENT §5, `make seed-content` after
// `make up-app && make seed-check`. Accounts: author cm.noidung, reviewer cm.duyet, System Admin sa.admin (demo users of
// the dev realm). Running it again creates nothing.
//
// Configuration (no default URL in code, AGENTS R3): EW_SEED_API_URL (public gateway, …/api/v1),
// EW_SEED_INTERNAL_API_URL (internal gateway, …/api/v1), EW_SEED_PASSWORD (demo password, never logged), EW_ENV=local.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/devseed"
)

func main() {
	file := flag.String("file", filepath.Join("modules", "learning", "seed", "content", "math_g3.dev.json"), "seed content file")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *file); err != nil {
		slog.Error("learning content seed failed", "error", err)
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

func run(ctx context.Context, path string) error {
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
	b, err := os.ReadFile(path) //nolint:gosec // seed file of the repository
	if err != nil {
		return err
	}
	var f devseed.File
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	hc := &http.Client{Timeout: 30 * time.Second}
	login := func(u string) (string, error) { return devseed.PortalLogin(ctx, hc, api, u, pw) }
	author, err := login("cm.noidung")
	if err != nil {
		return err
	}
	reviewer, err := login("cm.duyet")
	if err != nil {
		return err
	}
	admin, err := login("sa.admin")
	if err != nil {
		return err
	}
	res, err := devseed.Run(ctx, devseed.Client{BaseURL: api, HTTP: hc, Author: author, Reviewer: reviewer, ActivityReviewer: admin},
		&devseed.AdminAssigner{HTTP: hc, BaseURL: internal, Token: admin, Codes: f.TenantCodes}, f)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "learning content seed done", "questions", len(res.Questions), "question_set", res.QuestionSet,
		"activity", res.Activity, "lesson", res.Lesson, "published_now", res.Published, "tenants", strings.Join(f.TenantCodes, ","))
	return nil
}
