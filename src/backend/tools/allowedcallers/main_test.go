package main

import (
	"strings"
	"testing"
)

const spec = `paths:
  /internal/v1/principals:resolve:
    post:
      operationId: resolvePrincipal
      x-allowed-callers: [learning, game, learning]
  /internal/v1/tenants/{tenant_id}:
    parameters: []
    get:
      operationId: getTenant
      x-allowed-callers: [chat]
`

func TestParseAndRender(t *testing.T) {
	ops, err := Parse([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if got := ops["resolvePrincipal"]; len(got) != 2 || got[0] != "game" || got[1] != "learning" {
		t.Fatalf("callers = %v (sorted, deduplicated)", got)
	}
	src, err := Render("identity", ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, want := range []string{"DO NOT EDIT", "package api", `"getTenant":`, `"resolvePrincipal": {"game", "learning"}`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Index(s, "getTenant") > strings.Index(s, "resolvePrincipal") {
		t.Fatal("operations not sorted")
	}
}

func TestParseRejects(t *testing.T) {
	for name, bad := range map[string]string{
		"no callers":     "paths:\n  /x:\n    get:\n      operationId: x\n",
		"unknown caller": "paths:\n  /x:\n    get:\n      operationId: x\n      x-allowed-callers: [realtime]\n",
		"no operationId": "paths:\n  /x:\n    get:\n      x-allowed-callers: [chat]\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
