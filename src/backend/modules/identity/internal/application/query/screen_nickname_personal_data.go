package query

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/nicknamescreen"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ScreenNicknamePersonalData is screenNicknamePersonalData (x-allowed-callers: game, F136-AC-12, ADR-0105 §5): read
// context only (no write, no event). The nickname is compared with the real names and the school names of the student
// of the caller's TenantScope; only the decision leaves the module — never a name (R16). Neither the text nor the names
// are logged (R14).
type ScreenNicknamePersonalData struct {
	Model read.NicknamePersonalReader
}

// Handle reports whether text holds personal data of student. A student outside the scope tenant is 404 NOT_FOUND.
func (q ScreenNicknamePersonalData) Handle(ctx context.Context, student uuid.UUID, text string) (blocked bool, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ScreenNicknamePersonalData")
	defer func() { done(err) }()
	if n := utf8.RuneCountInString(text); n == 0 || n > 32 {
		return false, problem.Invalid("text", problem.SubInvalid, "1 to 32 characters")
	}
	sc, ok := tenancy.ScopeFrom(ctx)
	tenant, single := uuid.Nil, false
	if ok {
		tenant, single = sc.SessionTenantID()
	}
	if !single {
		return false, problem.New(problem.PermissionDenied, "screenNicknamePersonalData needs the TenantScope of the caller")
	}
	names, schools, found, err := q.Model.NicknamePersonal(ctx, tenant, student)
	if err != nil {
		return false, err
	}
	if !found {
		return false, problem.New(problem.NotFound, "")
	}
	return nicknamescreen.Blocked(text, nicknamescreen.Personal{Names: names, Schools: schools}), nil
}
