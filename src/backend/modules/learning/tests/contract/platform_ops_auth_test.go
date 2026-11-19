// Package contract holds the facade contract tests of module learning (make test-contract): the platform-ops
// operations of specs/openapi/modules/learning.internal.v1.yaml are callable only by module identity, in
// process, and authorize the original Principal.
package contract

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// platformOps are the facade operations F020 implements.
var platformOps = []string{"listTrash", "restoreTrashItem", "executeTenantCloneStep", "applyTenantAssignments",
	"revokeTenantAssignments", "listResourceAssignments"}

func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			b, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

type specOp struct {
	OperationID    string   `yaml:"operationId"`
	Tags           []string `yaml:"tags"`
	AllowedCallers []string `yaml:"x-allowed-callers"`
	Service        string   `yaml:"x-service"`
}

func operations(t *testing.T, rel string) map[string]specOp {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(repoFile(t, rel), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]specOp{}
	for _, item := range doc.Paths {
		for _, node := range item {
			var op specOp
			if node.Kind != yaml.MappingNode || node.Decode(&op) != nil || op.OperationID == "" {
				continue
			}
			out[op.OperationID] = op
		}
	}
	return out
}

// F020-AC-19 (TC-SVC-005, TC-SVC-006): every platform-ops operation of learning is callable only by module
// identity (spec, generated map and pkg/modcall at run time), has no public HTTP route and can never be
// mounted under /internal/; a module that calls no learning operation cannot import the facade (archgen).
func TestPlatformOpsOnlyForIdentity(t *testing.T) {
	internal := operations(t, "specs/openapi/modules/learning.internal.v1.yaml")
	public := operations(t, "specs/openapi/eduworld-api.v1.yaml")
	for _, id := range platformOps {
		op, ok := internal[id]
		if !ok {
			t.Fatalf("%s missing from learning.internal.v1.yaml", id)
		}
		if !slices.Equal(op.AllowedCallers, []string{"identity"}) || !slices.Contains(op.Tags, "platform-ops") {
			t.Errorf("%s: x-allowed-callers %v tags %v", id, op.AllowedCallers, op.Tags)
		}
		if !slices.Equal(learningapi.AllowedCallers[id], []string{"identity"}) {
			t.Errorf("%s: generated AllowedCallers %v", id, learningapi.AllowedCallers[id])
		}
		// The public operations of the same name (/api/v1/admin/trash…) belong to module identity (F045).
		if pub, exposed := public[id]; exposed && pub.Service != "identity" {
			t.Errorf("%s has a public route served by %q", id, pub.Service)
		}
		for _, caller := range []string{"game", "competition", "chat", "realtime"} {
			_, err := modcall.Call(context.Background(), modcall.Op{Caller: caller, Callee: "learning", Operation: id,
				AllowedCallers: learningapi.AllowedCallers[id]}, func(context.Context) (int, error) {
				t.Errorf("%s reached %s", caller, id)
				return 0, nil
			})
			var pe *problem.Error
			if !errors.As(err, &pe) || pe.Code != problem.CallerNotAllowed {
				t.Errorf("%s → %s: %v (want CALLER_NOT_ALLOWED)", caller, id, err)
			}
		}
	}
	// No HTTP route: httpx refuses to serve /internal/ (ADR-0074, F002-AC-05).
	idp := httpxtest.NewIDP(t)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: noResolver{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Handle(httpx.Operation{ID: "applyTenantAssignments", Method: http.MethodPost,
		Pattern: "/internal/v1/platform/tenant-assignments:apply"}, http.NotFoundHandler()); err == nil {
		t.Fatal("httpx mounted an /internal/ route")
	}
	// archgen: chat calls no learning operation, so .golangci.arch.yml denies it modules/learning/api.
	rules := string(repoFile(t, "src/backend/.golangci.arch.yml"))
	if !strings.Contains(rules, `pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"`) ||
		!strings.Contains(rules, "chat is not in x-allowed-callers of learning") {
		t.Fatal("depguard rule denying modules/learning/api to chat is missing (run tools/archgen)")
	}
}

type noResolver struct{}

func (noResolver) Resolve(context.Context, authz.ResolveRequest) (authz.Principal, error) {
	return authz.Principal{}, authz.ErrNotMember
}

func codeOf(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// Authorization of every facade use case of F020: the original Principal must be platform staff with the
// permission of the operation; a tenant user (of any tenant), a student, platform staff without the
// permission and a missing Principal are refused before any data access (the handlers below have no
// repository: a refused call never reaches one).
func TestPlatformOpsAuthorizePrincipal(t *testing.T) {
	ctx := context.Background()
	in := command.AssignmentsInput{ResourceType: "question", ResourceID: uuid.New(), TenantIDs: []uuid.UUID{authztest.TenantA}, Reason: "valid reason"}
	ops := map[string]func(ctx context.Context) error{
		"applyTenantAssignments": func(ctx context.Context) error {
			_, err := command.ApplyTenantAssignments{}.Handle(ctx, in)
			return err
		},
		"revokeTenantAssignments": func(ctx context.Context) error {
			_, err := command.RevokeTenantAssignments{}.Handle(ctx, in)
			return err
		},
		"listResourceAssignments": func(ctx context.Context) error {
			_, err := query.ListResourceAssignments{}.Handle(ctx, query.ListResourceAssignmentsInput{ResourceType: "question", ResourceID: uuid.New()})
			return err
		},
		"executeTenantCloneStep": func(ctx context.Context) error {
			_, err := command.ExecuteCloneStep{}.Handle(ctx, command.CloneStepInput{JobID: uuid.New(), SourceTenant: authztest.TenantA,
				TargetTenant: authztest.TenantB, ResourceType: "question", ResourceIDs: []uuid.UUID{uuid.New()}})
			return err
		},
		// Trash: tenant_admin holds tenant.trash.* [Proposed] and is allowed within its tenant (integration test).
		"listTrash": func(ctx context.Context) error {
			_, err := query.ListTrash{}.Handle(ctx, query.ListTrashInput{})
			return err
		},
		"restoreTrashItem": func(ctx context.Context) error {
			_, err := command.RestoreResource{}.Handle(ctx, command.RestoreInput{ResourceType: "question", ID: uuid.New(), Reason: "valid reason"})
			return err
		},
	}
	denied := map[string]authz.Principal{
		"tenant admin A":  authztest.Staff(authztest.TenantA, "tenant_admin"),
		"school admin B":  authztest.Staff(authztest.TenantB, "school_admin"),
		"student":         authztest.Student(authztest.TenantA),
		"content admin":   authztest.Platform("content_admin"),
		"no role (staff)": authztest.NoRole(authztest.TenantA),
	}
	trashOp := map[string]bool{"listTrash": true, "restoreTrashItem": true}
	for op, call := range ops {
		// Facade operations are not in the public route matrix (tools/routematrix): this loop is their role-denied test.
		for who, p := range denied {
			if trashOp[op] && who == "tenant admin A" {
				continue
			}
			if got := codeOf(call(authztest.Ctx(ctx, p))); got != problem.PermissionDenied {
				t.Errorf("%s by %s: %q, want PERMISSION_DENIED", op, who, got)
			}
		}
		if got := codeOf(call(ctx)); got != problem.Unauthenticated {
			t.Errorf("%s without Principal: %q", op, got)
		}
	}
}
