// Command seed-attempts loads the DEV-ONLY golden dataset of LOCAL_DEVELOPMENT §5 (`make seed-attempts`, after
// `make seed-content`): the golden content of seed/content/golden_g3.dev.json (a quiz on MATH.G3.NUM.DIV_REMAINDER and a
// quiz whose MATH.G3.GEO.PERIMETER question version 1 has a wrong answer key, corrected by version 2 — F058 US4),
// published through the CMS workflow and assigned to th-demo-a; the DEV school assignments of
// seed/assignments/demoa1_g3_golden.dev.json (ad.truong.a, DEMOA1 khối 3); then the student script of
// seed/attempts/demoa1_g3_golden.dev.json through the real attempt API of the game client (server-scored, AGENTS R5).
// Each student logs in at most once per run, and only when the staff read API (listAssignmentSubmissions) shows that
// one of its scripted attempts is not submitted yet: a second run logs no student in and changes nothing. With -verify
// it then compares the School Learning Dashboard of the grade with the golden values of the script (polling up to
// -verify-wait: the heatmap is a read model refreshed every school_dashboard_refresh).
//
// Configuration (no default URL in code, AGENTS R3): EW_SEED_API_URL, EW_SEED_INTERNAL_API_URL, EW_SEED_PASSWORD (demo
// password of every demo account, never logged), EW_ENV=local, EW_SEED_CLIENT_VERSION (game client version sent at login,
// ≥ EW_GAME_MIN_CLIENT_VERSION).
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
	seed := filepath.Join("modules", "learning", "seed")
	lessons := flag.String("lessons", filepath.Join(seed, "content", "golden_g3.dev.json"), "DEV golden content file")
	assignments := flag.String("assignments", filepath.Join(seed, "assignments", "demoa1_g3_golden.dev.json"), "DEV golden assignment file")
	attempts := flag.String("attempts", filepath.Join(seed, "attempts", "demoa1_g3_golden.dev.json"), "DEV student attempt script")
	verify := flag.Bool("verify", false, "compare the dashboard with the golden values of the script")
	verifyWait := flag.Duration("verify-wait", 20*time.Minute, "how long -verify waits for the dashboard read models")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *lessons, *assignments, *attempts, *verify, *verifyWait); err != nil {
		slog.Error("learning attempt seed failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, lessonsPath, assignmentsPath, attemptsPath string, verify bool, verifyWait time.Duration) error {
	if os.Getenv("EW_ENV") != "local" {
		return errors.New("seed-attempts is a local dev seed: EW_ENV must be local")
	}
	vals := map[string]string{}
	for _, k := range []string{"EW_SEED_API_URL", "EW_SEED_INTERNAL_API_URL", "EW_SEED_PASSWORD", "EW_SEED_CLIENT_VERSION"} {
		v, err := devseed.Env(k)
		if err != nil {
			return err
		}
		vals[k] = v
	}
	api, internal, pw := vals["EW_SEED_API_URL"], vals["EW_SEED_INTERNAL_API_URL"], vals["EW_SEED_PASSWORD"]
	var lf devseed.LessonFile
	var af devseed.AssignmentFile
	var sf devseed.AttemptFile
	for path, v := range map[string]any{lessonsPath: &lf, assignmentsPath: &af, attemptsPath: &sf} {
		if err := devseed.LoadJSON(path, v); err != nil {
			return err
		}
	}
	hc := &http.Client{Timeout: time.Minute}
	login := func(u string) (string, error) { return devseed.PortalLogin(ctx, hc, api, u, pw) }
	tokens := map[string]string{}
	for _, u := range []string{"cm.noidung", "cm.duyet", "sa.admin", af.Author, sf.Staff} {
		if tokens[u] != "" {
			continue
		}
		t, err := login(u)
		if err != nil {
			return err
		}
		tokens[u] = t
	}
	admin := tokens["sa.admin"]
	res, err := devseed.RunLessons(ctx, devseed.Client{BaseURL: api, HTTP: hc, Author: tokens["cm.noidung"], Reviewer: tokens["cm.duyet"],
		ActivityReviewer: admin}, nil, &devseed.AdminAssigner{HTTP: hc, BaseURL: internal, Token: admin, Codes: lf.TenantCodes}, lf)
	if err != nil {
		return err
	}
	for k, id := range res.Activities {
		slog.InfoContext(ctx, "golden activity", "key", k, "activity_id", id)
	}
	for k, vs := range res.Versions {
		slog.InfoContext(ctx, "golden question", "key", k, "content_id", res.Questions[k], "version_ids", strings.Join(vs, ","))
	}
	ar, err := devseed.RunAssignments(ctx, devseed.AssignmentClient{BaseURL: api, HTTP: hc, Token: tokens[af.Author]},
		&devseed.AdminAssigner{HTTP: hc, BaseURL: internal, Token: admin, Codes: []string{af.TenantCode}}, af,
		devseed.Result{SubjectID: res.SubjectID, Activities: res.Activities})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "golden assignments done", "assignments", len(ar.Assignments), "created_now", ar.Created, "published_now", ar.Published)
	// The staff token is renewed before the script (portal access tokens are short-lived).
	staff, err := login(sf.Staff)
	if err != nil {
		return err
	}
	gc := devseed.GameClient{Version: vals["EW_SEED_CLIENT_VERSION"], Platform: "web"}
	tenants := &devseed.AdminAssigner{HTTP: hc, BaseURL: internal, Token: admin, Codes: []string{sf.TenantCode}}
	out, err := devseed.RunAttempts(ctx, devseed.AttemptClient{BaseURL: api, HTTP: hc, Staff: staff,
		Login: func(ctx context.Context, u string) (devseed.StudentSession, error) {
			return devseed.GameLogin(ctx, hc, api, u, pw, gc)
		}},
		tenants, sf, res)
	slog.InfoContext(ctx, "golden attempts", "student_logins_now", out.Logins, "submitted_now", out.Submitted, "already_submitted", out.Skipped)
	if err != nil {
		return err
	}
	if !verify {
		return nil
	}
	return verifyGolden(ctx, hc, api, func() (string, error) { return login(sf.Staff) }, out, sf, verifyWait)
}

// verifyGolden polls the dashboard of the grade until it shows the golden values of the script (or the wait ends).
func verifyGolden(ctx context.Context, hc *http.Client, api string, login func() (string, error), out devseed.AttemptResult, sf devseed.AttemptFile,
	wait time.Duration) error {
	if sf.Expected == nil {
		return errors.New("the attempt script has no expected golden values")
	}
	deadline := time.Now().Add(wait)
	staff := ""
	for {
		if staff == "" {
			t, err := login()
			if err != nil {
				return err
			}
			staff = t
		}
		from, to := devseed.DefaultRange(time.Now())
		got, err := devseed.Snapshot(ctx, hc, api, staff, out.TenantID, out.SchoolID, sf.GradeLevel, from, to, sf.Expected.Codes())
		if devseed.IsStatus(err, http.StatusUnauthorized) {
			staff = "" // the access token expired while waiting: sign in again
			continue
		}
		if err != nil {
			return err
		}
		b, _ := json.Marshal(got)
		diff := sf.Expected.Diff(got)
		if len(diff) == 0 {
			slog.InfoContext(ctx, "golden dashboard values match", "school_id", out.SchoolID, "grade", sf.GradeLevel, "from", from, "to", to,
				"values", string(b))
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("golden dashboard values differ (values %s): %s", b, strings.Join(diff, "; "))
		}
		slog.InfoContext(ctx, "golden dashboard values not there yet, waiting for the read models", "diff", strings.Join(diff, "; "))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}
