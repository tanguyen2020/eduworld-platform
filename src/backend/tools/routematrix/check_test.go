package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const spec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /auth/game/login:
    post:
      operationId: gameLogin
      x-service: identity
      security: []
  /lms/classes/{class_id}:
    parameters:
      - $ref: '#/components/parameters/TenantIdHeader'
    get:
      operationId: getClass
      x-service: learning
  /me:
    get:
      operationId: getMe
      x-service: identity
components:
  parameters:
    TenantIdHeader: { name: X-Tenant-Id, in: header, required: true, schema: { type: string } }
`

func TestGen(t *testing.T) {
	m, err := Gen([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Entry{}
	for _, e := range m {
		byID[e.OperationID] = e
	}
	if e := byID["getClass"]; !e.TenantScoped || !e.HasIDParam || e.Module != "learning" || strings.Join(e.Required, ",") != "cross_tenant,role_denied" {
		t.Fatalf("getClass = %+v", e)
	}
	if e := byID["getMe"]; e.TenantScoped || strings.Join(e.Required, ",") != "role_denied" {
		t.Fatalf("getMe = %+v", e)
	}
	if e := byID["gameLogin"]; !e.Public || len(e.Required) != 0 {
		t.Fatalf("gameLogin = %+v", e)
	}
	if m[0].OperationID != "gameLogin" || m[len(m)-1].OperationID != "getMe" {
		t.Fatal("matrix not sorted")
	}
}

func write(t *testing.T, dir, rel, src string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

const routes = `package presentation
import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
var Ops = []httpx.Operation{{ID: "getClass", Method: "GET", Pattern: "/lms/classes/{class_id}"}, {ID: "getMe"}}
`

// F002-AC-28: an implemented operation without tenant A/B and role-denied tests fails the check.
func TestCheck(t *testing.T) {
	m, _ := Gen([]byte(spec))
	dir := t.TempDir()
	write(t, dir, "modules/learning/internal/presentation/routes.go", routes)
	write(t, dir, "modules/learning/internal/presentation/routes_test.go", `package presentation
import ("testing"; "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest")
func TestGetClass(t *testing.T) { authztest.Cover(t, "getClass", authztest.CrossTenant) }
func TestGetMe(t *testing.T) { authztest.Cover(t, "getMe", authztest.RoleDenied) }
`)
	problems, err := Check(m, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "getClass") || !strings.Contains(problems[0], "role_denied") {
		t.Fatalf("problems = %v", problems)
	}

	write(t, dir, "modules/learning/internal/presentation/more_test.go", `package presentation
import ("testing"; at "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest")
func TestGetClassRole(t *testing.T) { at.Cover(t, "getClass", at.RoleDenied) }
`)
	if problems, _ := Check(m, dir); len(problems) != 0 {
		t.Fatalf("covered operation still reported: %v", problems)
	}

	// An implemented operation that is not in the contract is reported too.
	write(t, dir, "modules/learning/internal/presentation/extra.go", `package presentation
import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
var X = httpx.Operation{ID: "notInSpec"}
`)
	if problems, _ := Check(m, dir); len(problems) != 1 || !strings.Contains(problems[0], "notInSpec") {
		t.Fatalf("problems = %v", problems)
	}
}

// Non-literal Cover arguments cannot be verified and are reported.
func TestCoverMustUseLiterals(t *testing.T) {
	m, _ := Gen([]byte(spec))
	dir := t.TempDir()
	write(t, dir, "modules/learning/x_test.go", `package x
import ("testing"; "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest")
func TestX(t *testing.T) { op := "getClass"; authztest.Cover(t, op, authztest.CrossTenant) }
`)
	problems, err := Check(m, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "string literal") {
		t.Fatalf("problems = %v", problems)
	}
}
