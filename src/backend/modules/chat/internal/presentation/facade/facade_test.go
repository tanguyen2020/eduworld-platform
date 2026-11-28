package facade_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// checker is a TEST DOUBLE (R17) of the pipeline display-text check: it records the call and answers the configured
// result (no word list: R15).
type checker struct {
	allowed bool
	err     error
	tenant  uuid.UUID
	text    string
}

func (c *checker) CheckDisplayText(_ context.Context, tenant uuid.UUID, text string) (bool, error) {
	c.tenant, c.text = tenant, text
	return c.allowed, c.err
}

// ScreenTexts answers the configured violations (TEST DOUBLE: position "1" of the texts is flagged when !allowed).
func (c *checker) ScreenTexts(_ context.Context, tenant uuid.UUID, texts []string) ([]int, error) {
	c.tenant = tenant
	if c.err != nil {
		return nil, c.err
	}
	if !c.allowed && len(texts) > 1 {
		return []int{1}, nil
	}
	return []int{}, nil
}

// Callee-side facade test of moderateDisplayText (x-allowed-callers: competition, F116): the example of the internal
// spec is allowed, the decision only is returned, the body is validated and its tenant must be the caller's scope.
func TestModerateDisplayTextFacade(t *testing.T) {
	tenant := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000001")
	author := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000005")
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	req := func(text string, c api.DisplayTextCheckRequestContext, tn uuid.UUID) api.ModerateDisplayTextRequestObject {
		return api.ModerateDisplayTextRequestObject{Body: &api.DisplayTextCheckRequest{Text: text, Context: c, TenantId: tn, AuthorId: author}}
	}
	ck := &checker{allowed: true}
	f := facade.Facade{Moderation: ck}
	// Example of specs/openapi/modules/chat.internal.v1.yaml (DisplayTextCheckRequest → DisplayTextCheckResult).
	res, err := f.ModerateDisplayText(ctx, req("Nhóm Sao Băng", api.PartyName, tenant))
	if err != nil || res != api.ModerateDisplayText200JSONResponse(api.DisplayTextCheckResult{Decision: api.DisplayTextCheckResultDecisionAllowed}) ||
		ck.tenant != tenant || ck.text != "Nhóm Sao Băng" {
		t.Fatalf("allowed: %+v %v %+v", res, err, ck)
	}
	ck.allowed = false
	if res, err := f.ModerateDisplayText(ctx, req("Đội zqơrốp", api.TeamName, tenant)); err != nil ||
		res.(api.ModerateDisplayText200JSONResponse).Decision != api.DisplayTextCheckResultDecisionBlocked {
		t.Fatalf("blocked: %+v %v", res, err)
	}
	var pe *problem.Error
	for name, r := range map[string]api.ModerateDisplayTextRequestObject{
		"no body":      {},
		"empty text":   req("", api.PartyName, tenant),
		"long text":    req(string(make([]rune, 256)), api.PartyName, tenant),
		"bad context":  req("Nhóm", "class_name", tenant),
		"other tenant": req("Nhóm", api.PartyName, uuid.New()),
	} {
		if _, err := f.ModerateDisplayText(ctx, r); !errors.As(err, &pe) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := f.ModerateDisplayText(context.Background(), req("Nhóm", api.PartyName, tenant)); !errors.As(err, &pe) || pe.Code != problem.Unauthenticated {
		t.Fatalf("no scope: %v", err)
	}
	ck.err = errors.New("TEST DOUBLE: catalogue not loaded")
	if _, err := f.ModerateDisplayText(ctx, req("Nhóm", api.PartyName, tenant)); err == nil {
		t.Fatal("pipeline failure must fail closed")
	}
}

// Callee-side facade test of screenText (x-allowed-callers: game, F077-AC-23): the example of the internal spec is
// allowed; a flagged text is returned by position only (never the matched term — R15); the body is validated and its
// tenant must be the caller's scope; a pipeline failure is an error (the caller answers 503, never fail-open).
func TestScreenTextFacade(t *testing.T) {
	tenant := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000001")
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	req := func(tn uuid.UUID, texts ...string) api.ScreenTextRequestObject {
		return api.ScreenTextRequestObject{Body: &api.TextScreenRequest{TenantId: tn, Locale: api.Vi, Texts: texts}}
	}
	ck := &checker{allowed: true}
	f := facade.Facade{Moderation: ck}
	res, err := f.ScreenText(ctx, req(tenant, "Tuần lễ Toán học", "Cả trường cùng giải toán"))
	if v, ok := res.(api.ScreenText200JSONResponse); err != nil || !ok || !v.Allowed || len(v.Violations) != 0 || ck.tenant != tenant {
		t.Fatalf("allowed: %+v %v", res, err)
	}
	ck.allowed = false
	res, err = f.ScreenText(ctx, req(tenant, "Tuần lễ Toán học", "placeholder-flagged"))
	if v, ok := res.(api.ScreenText200JSONResponse); err != nil || !ok || v.Allowed || len(v.Violations) != 1 || v.Violations[0] != 1 {
		t.Fatalf("flagged: %+v %v", res, err)
	}
	var pe *problem.Error
	for name, r := range map[string]api.ScreenTextRequestObject{
		"no body":      {},
		"no text":      req(tenant),
		"empty text":   req(tenant, ""),
		"long text":    req(tenant, string(make([]rune, 4001))),
		"other tenant": req(uuid.New(), "Tuần lễ"),
	} {
		if _, err := f.ScreenText(ctx, r); !errors.As(err, &pe) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := f.ScreenText(context.Background(), req(tenant, "Tuần lễ")); !errors.As(err, &pe) || pe.Code != problem.Unauthenticated {
		t.Fatalf("no scope: %v", err)
	}
	ck.err = errors.New("TEST DOUBLE: catalogue not loaded")
	if _, err := f.ScreenText(ctx, req(tenant, "Tuần lễ")); err == nil {
		t.Fatal("pipeline failure must be an error")
	}
}
