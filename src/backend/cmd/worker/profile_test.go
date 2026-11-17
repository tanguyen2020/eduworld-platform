package main

import "testing"

func TestResolveProfile(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	if p, err := resolveProfile("chat", env("")); err != nil || p != "chat" {
		t.Fatalf("flag: %q %v", p, err)
	}
	if p, err := resolveProfile("", env("jobs")); err != nil || p != "jobs" {
		t.Fatalf("env: %q %v", p, err)
	}
	if _, err := resolveProfile("", env("")); err == nil {
		t.Fatal("missing profile accepted")
	}
	if _, err := resolveProfile("bogus", env("")); err == nil {
		t.Fatal("unknown profile accepted")
	}
}
