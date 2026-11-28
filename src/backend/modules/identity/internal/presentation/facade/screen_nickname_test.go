package facade_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// personalReader is a TEST DOUBLE (R17) of read.NicknamePersonalReader with synthetic names.
type personalReader struct {
	tenant, student uuid.UUID
	names, schools  []string
	err             error
	asked           uuid.UUID
}

func (r *personalReader) NicknamePersonal(_ context.Context, tenant, student uuid.UUID) ([]string, []string, bool, error) {
	r.asked = tenant
	if r.err != nil {
		return nil, nil, false, r.err
	}
	if tenant != r.tenant || student != r.student {
		return nil, nil, false, nil
	}
	return r.names, r.schools, true, nil
}

// Callee-side facade test of screenNicknamePersonalData (x-allowed-callers: game, F136-AC-12, AC-29, TC-SVC-005): the
// example of the internal spec is allowed; the real name, two of its words or the school name is blocked; the response
// is the decision only (never the name or the school — R16); a student outside the scope tenant is 404; the text is
// validated (1..32) and a TenantScope is required; a read failure is an error (the caller answers 503).
func TestScreenNicknamePersonalDataFacade(t *testing.T) {
	tenant, student := uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000001"), uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000005")
	r := &personalReader{tenant: tenant, student: student, names: []string{"Nguyễn Văn Minh"}, schools: []string{"Trường Tiểu học Hòa Bình"}}
	f := facade.Nicknames{Screen: query.ScreenNicknamePersonalData{Model: r}}
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	call := func(ctx context.Context, id uuid.UUID, text string) (identityapi.ScreenNicknamePersonalDataResponseObject, error) {
		return f.ScreenNicknamePersonalData(ctx, identityapi.ScreenNicknamePersonalDataRequestObject{StudentId: id,
			Body: &identityapi.NicknameScreenRequest{Text: text}})
	}
	// Example of specs/openapi/modules/identity.internal.v1.yaml (NicknameScreenRequest → NicknameScreenResult).
	res, err := call(ctx, student, "Siêu Nhân Sao Băng")
	if err != nil || res != identityapi.ScreenNicknamePersonalData200JSONResponse(identityapi.NicknameScreenResult{Decision: identityapi.Allowed}) {
		t.Fatalf("allowed: %+v %v", res, err)
	}
	if r.asked != tenant {
		t.Fatalf("read in tenant %v, want the scope tenant", r.asked)
	}
	for _, text := range []string{"Nguyen Van Minh", "Minh Nguyễn Siêu", "Hòa Bình Vui"} {
		res, err := call(ctx, student, text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		b, _ := json.Marshal(res)
		if string(b) != `{"decision":"blocked"}` {
			t.Fatalf("%q: %s, want the decision only", text, b)
		}
		if strings.Contains(strings.ToLower(string(b)), "nguy") || strings.Contains(strings.ToLower(string(b)), "binh") {
			t.Fatalf("%q: response leaks personal data: %s", text, b)
		}
	}
	if res, err := call(ctx, uuid.New(), "Siêu Nhân"); err != nil {
		t.Fatalf("unknown student: %v", err)
	} else if _, ok := res.(identityapi.ScreenNicknamePersonalData404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatalf("unknown student: %T, want 404", res)
	}
	var pe *problem.Error
	for name, text := range map[string]string{"empty": "", "too long": strings.Repeat("a", 33)} {
		if _, err := call(ctx, student, text); !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := f.ScreenNicknamePersonalData(ctx, identityapi.ScreenNicknamePersonalDataRequestObject{StudentId: student}); err == nil {
		t.Fatal("no body")
	}
	if _, err := call(context.Background(), student, "Siêu Nhân"); !errors.As(err, &pe) || pe.Code != problem.PermissionDenied {
		t.Fatalf("no scope: %v", err)
	}
	r.err = errors.New("TEST DOUBLE: database down")
	if _, err := call(ctx, student, "Siêu Nhân"); err == nil {
		t.Fatal("read failure must be an error (caller fails closed)")
	}
}
