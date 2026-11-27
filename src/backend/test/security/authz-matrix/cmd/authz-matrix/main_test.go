package main

import (
	"context"
	"errors"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

const (
	specRel   = "../../../../../../../specs/openapi/eduworld-api.v1.yaml"
	matrixRel = "../../../../../test/authz/matrix_gen.json"
	modsRel   = "../../../../../../../specs/openapi/modules"
	ovRel     = "../../expectations.yaml"
)

func args(mode string) []string {
	return []string{"-mode", mode, "-spec", specRel, "-matrix", matrixRel, "-modules", modsRel, "-overrides", ovRel}
}

// F132-AC-07: the runner refuses production (and an unset EW_ENV) before sending anything.
func TestRefusesProduction(t *testing.T) {
	for _, env := range []string{"production", ""} {
		t.Setenv(envguard.Var, env)
		get := func(k string) string {
			if k == "EW_BASE_URL" {
				return "http://127.0.0.1:1/api/v1"
			}
			return ""
		}
		if err := run(context.Background(), args("matrix"), get); !errors.Is(err, envguard.ErrRefused) {
			t.Fatalf("EW_ENV=%q: %v", env, err)
		}
	}
}

// The offline generation needs no environment and loads the committed overrides.
func TestGenOffline(t *testing.T) {
	if err := run(context.Background(), args("gen"), func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
}

func TestHostNeverReportsUserinfo(t *testing.T) {
	if h := host("https://user:secret@api.staging.example/api/v1"); h != "api.staging.example" {
		t.Fatal(h)
	}
}
