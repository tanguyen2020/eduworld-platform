package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
)

func TestProjectionCommand(t *testing.T) {
	var reg projection.Registry
	var out bytes.Buffer
	if err := runProjection(context.Background(), []string{"rebuild", "--module=learning"}, &reg, &out); err == nil {
		t.Fatal("missing --source accepted")
	}
	if err := runProjection(context.Background(), []string{"rebuild", "--module=learning", "--source=identity"}, &reg, &out); err == nil ||
		!strings.Contains(err.Error(), "nothing registered") {
		t.Fatalf("empty registry: %v", err)
	}
	_ = reg.Register(projection.Table{Module: "learning", Source: "identity", Name: "ref.schools",
		Fetch: func(context.Context, string) ([]projection.Row, string, error) {
			return nil, "", errors.New("facade down")
		},
		Upsert: func(context.Context, []projection.Row) error { return nil }})
	if err := runProjection(context.Background(), []string{"rebuild", "--module=learning", "--source=identity"}, &reg, &out); err == nil {
		t.Fatal("facade failure must give a non-zero exit")
	}
}
