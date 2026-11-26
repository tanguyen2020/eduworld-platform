//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Delta F076-3 (additive): getGameChallenge returns description_i18n (vi + en of the published version) next to the
// unchanged description string chosen by Accept-Language; a version without description has neither; rules_summary_i18n
// stays absent (challenge.schema.json has no such text). Tenant isolation: tenant B never reads the challenge (404) and
// an X-Tenant-Id of another tenant is TENANT_MISMATCH.
func TestChallengeDescriptionI18n(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	described := "0193a5f4-6c1e-7b2a-9d1f-000000076191"
	e.publishChallenges(ctx, t, []command.ContentItem{
		challengeItem(described, "chl.math.g3_described", "time", []int{3}, func(m map[string]any) {
			m["description"] = map[string]any{"vi": "Trả lời nhanh 5 câu đúng", "en": "Answer 5 questions correctly, fast"}
		}),
	}, e.tenantA)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	bao := e.studentOfGrade(ctx, t, e.tenantB, e.schoolB, 3)
	get := func(s student, id, lang string) resp {
		h := th(s)
		if lang != "" {
			h["Accept-Language"] = lang
		}
		return e.do(t, http.MethodGet, "/game/challenges/"+id, e.gtoken(s), h, nil)
	}
	vi := get(an, described, "")
	i18n, _ := vi.body["description_i18n"].(map[string]any)
	if vi.status != http.StatusOK || vi.body["description"] != "Trả lời nhanh 5 câu đúng" || i18n["vi"] != "Trả lời nhanh 5 câu đúng" ||
		i18n["en"] != "Answer 5 questions correctly, fast" {
		t.Fatalf("description_i18n: %d %s", vi.status, vi.raw)
	}
	if _, ok := vi.body["rules_summary_i18n"]; ok {
		t.Fatalf("rules_summary_i18n without a source: %s", vi.raw)
	}
	en := get(an, described, "en-US")
	if en.body["description"] != "Answer 5 questions correctly, fast" || en.body["description_i18n"].(map[string]any)["vi"] != "Trả lời nhanh 5 câu đúng" {
		t.Fatalf("en: %s", en.raw)
	}
	plain := get(an, chlTime3, "")
	if _, ok := plain.body["description_i18n"]; plain.status != http.StatusOK || ok {
		t.Fatalf("no description: %d %s", plain.status, plain.raw)
	}
	if r := get(bao, described, ""); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
	// X-Tenant-Id of tenant B with the token of A → TENANT_MISMATCH (staff are refused by TestPlayerRoutesStaffDenied).
	wrong := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	if r := e.do(t, http.MethodGet, "/game/challenges/"+described, e.gtoken(an), wrong, nil); r.status != http.StatusForbidden ||
		r.code() != string(problem.TenantMismatch) {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
}
