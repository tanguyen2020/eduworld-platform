package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

type fakeAudit struct {
	calls int
	got   read.AuditFilter
}

func (m *fakeAudit) AuditLogs(_ context.Context, f read.AuditFilter) ([]dto.AuditLogEntry, error) {
	m.calls++
	m.got = f
	return nil, nil
}

// F059-3: action_prefix reaches the read model of both audit queries once validated; a malformed or too long list
// is 422 before any read.
func TestAuditActionPrefixes(t *testing.T) {
	m := &fakeAudit{}
	h := query.AuditLogs{Model: m, Cursors: codec(t), MaxRange: 93 * 24 * time.Hour}
	to := time.Now().UTC()
	in := query.AuditLogsInput{From: to.Add(-time.Hour), To: to}
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	ta := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "tenant_admin"))
	for _, bad := range [][]string{{"role.*"}, {"Role"}, {"role%"}, make([]string, 11)} {
		in.ActionPrefixes = bad
		if _, err := h.ListPlatform(sa, in); codeOf(err) != problem.ValidationFailed {
			t.Errorf("platform %v: %v", bad, err)
		}
		if _, err := h.ListTenant(ta, in); codeOf(err) != problem.ValidationFailed {
			t.Errorf("tenant %v: %v", bad, err)
		}
	}
	if m.calls != 0 {
		t.Fatal("read model reached for a rejected request")
	}
	in.ActionPrefixes = []string{"role", "user_role"}
	if _, err := h.ListPlatform(sa, in); err != nil || len(m.got.ActionPrefixes) != 2 || !m.got.PlatformOnly {
		t.Fatalf("platform: %+v %v", m.got, err)
	}
	if _, err := h.ListTenant(ta, in); err != nil || len(m.got.ActionPrefixes) != 2 || len(m.got.TenantIDs) != 1 || !m.got.NoModeration {
		t.Fatalf("tenant: %+v %v", m.got, err)
	}
}
