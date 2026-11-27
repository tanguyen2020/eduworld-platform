// Package envguard refuses to run the F132 test tooling (datagen, loadbot, authz-matrix, wsfuzz, anti-cheat runner)
// outside a test environment (F132-AC-07, TEST_STRATEGY §4): synthetic load and generated accounts never reach
// production, and real student data is never touched. The environment is named explicitly by EW_ENV; nothing is
// guessed from a URL or a host name (AGENTS R3).
package envguard

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Var is the environment variable naming the target environment.
const Var = "EW_ENV"

// Allowed are the environments the tooling may target: local (a developer machine or CI ephemeral stack), dev and
// staging. Everything else, production included, is refused.
var Allowed = []string{"local", "dev", "staging"}

// ErrRefused is returned when the target environment is not a test environment.
var ErrRefused = errors.New("envguard: refused")

// Check returns nil when env is one of Allowed (case-insensitive, surrounding blanks ignored).
func Check(env string) error {
	e := strings.ToLower(strings.TrimSpace(env))
	if e == "" {
		return fmt.Errorf("%w: %s is not set (one of %s)", ErrRefused, Var, strings.Join(Allowed, ", "))
	}
	for _, a := range Allowed {
		if e == a {
			return nil
		}
	}
	return fmt.Errorf("%w: %s=%q is not a test environment (one of %s)", ErrRefused, Var, env, strings.Join(Allowed, ", "))
}

// FromEnv checks EW_ENV of the process and returns its normalised value.
func FromEnv() (string, error) {
	v := os.Getenv(Var)
	if err := Check(v); err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(v)), nil
}
