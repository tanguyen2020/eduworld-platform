// Package config reads binary configuration from the environment with koanf (F002 T043, ADR-0025).
// Variable names follow LOCAL_DEVELOPMENT §4 (the single table of environment variables). Every value
// X may instead be given as a Docker secret file through X_FILE (staging/production). Missing required
// variables are reported by NAME only — values are never printed (F002-AC-22, AGENTS R3/R14).
package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config is a read-only view of the environment taken at Load time.
type Config struct {
	k   *koanf.Koanf
	get func(string) string // FromGetenv: every key is read through this function
}

// Load snapshots EW_* and OTEL_* environment variables.
func Load() *Config {
	k := koanf.New("::") // keys contain dots/underscores; never split them
	for _, prefix := range []string{"EW_", "OTEL_"} {
		_ = k.Load(env.Provider(prefix, "::", func(s string) string { return s }), nil)
	}
	return &Config{k: k}
}

// FromGetenv reads every key through getenv (os.Getenv in a binary, a map lookup in its tests), so the
// composition root and its tests read the configuration from the same source as bootstrap.Options.Getenv.
func FromGetenv(getenv func(string) string) *Config {
	if getenv == nil {
		return Load()
	}
	return &Config{get: getenv}
}

func (c *Config) raw(key string) string {
	if c.get != nil {
		return c.get(key)
	}
	return c.k.String(key)
}

// lookup returns the value of key, else the content of the file named by key_FILE.
func (c *Config) lookup(key string) (string, bool, error) {
	if v := strings.TrimSpace(c.raw(key)); v != "" {
		return v, true, nil
	}
	if f := strings.TrimSpace(c.raw(key + "_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", false, fmt.Errorf("%s_FILE: cannot read secret file", key)
		}
		return strings.TrimSpace(string(b)), true, nil
	}
	return "", false, nil
}

// String returns the value or "" when absent/unreadable.
func (c *Config) String(key string) string {
	v, _, _ := c.lookup(key)
	return v
}

// StringOr returns the value or def.
func (c *Config) StringOr(key, def string) string {
	if v, ok, _ := c.lookup(key); ok && v != "" {
		return v
	}
	return def
}

// List splits a comma-separated value, trimming blanks.
func (c *Config) List(key string) []string {
	var out []string
	for _, p := range strings.Split(c.String(key), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Duration parses a Go duration ("45s"), returning def when absent or invalid.
func (c *Config) Duration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(c.String(key)); err == nil {
		return d
	}
	return def
}

// Int parses an integer, returning def when absent or invalid.
func (c *Config) Int(key string, def int) int {
	if n, err := strconv.Atoi(c.String(key)); err == nil {
		return n
	}
	return def
}

// Bool parses a boolean, returning def when absent or invalid.
func (c *Config) Bool(key string, def bool) bool {
	if b, err := strconv.ParseBool(c.String(key)); err == nil {
		return b
	}
	return def
}

// MissingError lists required variables that are absent. It never contains values.
type MissingError struct {
	Names    []string
	Problems []string
}

func (e *MissingError) Error() string {
	parts := []string{}
	if len(e.Names) > 0 {
		parts = append(parts, "missing required environment variables: "+strings.Join(e.Names, ", "))
	}
	parts = append(parts, e.Problems...)
	return strings.Join(parts, "; ")
}

// Require checks that every key has a value (directly or through key_FILE).
func (c *Config) Require(keys ...string) error {
	var e MissingError
	for _, k := range keys {
		_, ok, err := c.lookup(k)
		switch {
		case err != nil:
			e.Problems = append(e.Problems, err.Error())
		case !ok:
			e.Names = append(e.Names, k)
		}
	}
	if len(e.Names) == 0 && len(e.Problems) == 0 {
		return nil
	}
	sort.Strings(e.Names)
	return &e
}
