package moderation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/pipeline"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Callee-side facade test of moderateDisplayText with context nickname (caller game, F136-AC-11, AC-29, TC-CHAT-010):
// the real pipeline of F116 behind the facade — the placeholder catalogue term is blocked, every PII form (phone number
// spelled or split, spelled e-mail, social account, @handle) is blocked even with every switch off (ADR-0078), a clean
// nickname is allowed, and the response is the decision only (no term, position, category — R15).
func TestModerateDisplayTextNicknameFacade(t *testing.T) {
	tenant := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000001")
	author := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000005")
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	view := service.CatalogView{Parts: []*catalog.Compiled{compiled(t)}, Categories: categories(), PlatformVersion: 7}
	allOn := moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}
	v := &pipeline.Validator{Engine: engine, Catalogs: fakeCatalogs{view: view}, Switches: fakeSwitches{e: allOn}}
	f := facade.Facade{Moderation: v}
	check := func(text string) (api.DisplayTextCheckResultDecision, []byte) {
		t.Helper()
		res, err := f.ModerateDisplayText(ctx, api.ModerateDisplayTextRequestObject{Body: &api.DisplayTextCheckRequest{
			Text: text, Context: api.Nickname, TenantId: tenant, AuthorId: author}})
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		ok, isOK := res.(api.ModerateDisplayText200JSONResponse)
		if !isOK {
			t.Fatalf("%q: response %T", text, res)
		}
		b, _ := json.Marshal(ok)
		return ok.Decision, b
	}
	// Example of specs/openapi/modules/chat.internal.v1.yaml (context nickname).
	if d, _ := check("Siêu Nhân Sao Băng"); d != api.DisplayTextCheckResultDecisionAllowed {
		t.Fatalf("clean nickname: %s", d)
	}
	blocked := []string{
		"Minh zqơrốp",       // placeholder catalogue term (never a real word in tests — R15)
		"Minh 0912 345 678", // phone number split by spaces
		"Minh khong chin mot hai ba bon nam sau bay tam", // phone number spelled out
		"mail a còng gmail chấm com",                     // spelled e-mail
		"zalo minhxinh",                                  // social account
		"Minh minh123 tiktok",                            // social account
		"@minh_123",                                      // @handle
	}
	for _, text := range blocked {
		d, body := check(text)
		if d != api.DisplayTextCheckResultDecisionBlocked {
			t.Errorf("%q: %s, want blocked", text, d)
		}
		if string(body) != `{"decision":"blocked"}` {
			t.Errorf("%q: body %s — the decision only (R15)", text, body)
		}
		for _, w := range strings.Fields(text) {
			if len(w) > 3 && strings.Contains(string(body), w) {
				t.Errorf("%q: response leaks %q", text, w)
			}
		}
	}
	// Every switch off: the catalogue step is skipped, PII still runs (ADR-0078).
	exp := time.Now().Add(time.Hour)
	v.Switches = fakeSwitches{e: moderation.Effective{Platform: moderation.AllOn(),
		Tenant: &moderation.Switch{EnabledAll: false, Steps: map[string]bool{}, ExpiresAt: &exp}, At: time.Now()}}
	for _, text := range []string{"Minh 0912 345 678", "zalo minhxinh", "@minh_123"} {
		if d, _ := check(text); d != api.DisplayTextCheckResultDecisionBlocked {
			t.Errorf("switches off, %q: %s — PII must always run", text, d)
		}
	}
}
