package tenant

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	id := uuid.New()
	tn, err := New(id, " th-demo-x ", "TH Demo X", TypeSchool, "th-demo-x", "")
	if err != nil {
		t.Fatal(err)
	}
	if tn.Code != "th-demo-x" || tn.Status != StatusActive || tn.DefaultLocale != "vi-VN" || tn.Version != 1 {
		t.Fatalf("tenant = %+v", tn)
	}
	// F010-AC-01: alias pattern, type set; F010-AC-05: lengths.
	for name, c := range map[string]struct {
		code, name, alias string
		typ               Type
		field             string
	}{
		"alias upper":   {"ok", "n", "TH-X", TypeSchool, "keycloak_org_alias"},
		"alias short":   {"ok", "n", "ab", TypeSchool, "keycloak_org_alias"},
		"alias long":    {"ok", "n", strings.Repeat("a", 64), TypeSchool, "keycloak_org_alias"},
		"bad type":      {"ok", "n", "abc", Type("district"), "tenant_type"},
		"code short":    {"x", "n", "abc", TypeUnit, "code"},
		"code long":     {strings.Repeat("c", 65), "n", "abc", TypeUnit, "code"},
		"name empty":    {"ok", "  ", "abc", TypeUnit, "name"},
		"name too long": {"ok", strings.Repeat("ồ", 256), "abc", TypeUnit, "name"},
	} {
		_, err := New(id, c.code, c.name, c.typ, c.alias, "")
		var inv ErrInvalid
		if !errors.As(err, &inv) || inv.Field != c.field {
			t.Errorf("%s: err = %v, want field %s", name, err, c.field)
		}
	}
}

func TestApply(t *testing.T) {
	tn, _ := New(uuid.New(), "pgd-a", "PGD A", TypeUnit, "pgd-a", "vi-VN")
	disabled := StatusDisabled
	c, err := tn.Apply(Patch{Status: &disabled})
	if err != nil || !c.StatusChanged || c.NameChanged || tn.Status != StatusDisabled {
		t.Fatalf("disable: %+v %v", c, err)
	}
	if _, err := tn.Apply(Patch{Status: &disabled}); !errors.Is(err, ErrNoChange) {
		t.Fatalf("same status = %v", err)
	}
	name := " PGD A mới "
	if c, err := tn.Apply(Patch{Name: &name}); err != nil || !c.NameChanged || tn.Name != "PGD A mới" {
		t.Fatalf("rename: %+v %v %q", c, err, tn.Name)
	}
	bad := Status("archived")
	if _, err := tn.Apply(Patch{Status: &bad}); err == nil {
		t.Fatal("unknown status accepted")
	}
	attrs := tn.OrgAttributes()
	if attrs["tenant_id"][0] != tn.ID.String() || attrs["tenant_type"][0] != "unit" {
		t.Fatalf("attributes = %v", attrs)
	}
}

func TestCommonIDs(t *testing.T) {
	id, err := CommonClassID(4)
	if err != nil || id.String() != "0193a000-0000-7000-8000-000000000014" {
		t.Fatalf("class g4 = %v %v", id, err)
	}
	for _, g := range []int{0, 10} {
		if _, err := CommonClassID(g); err == nil {
			t.Fatalf("grade %d accepted", g)
		}
	}
	if CommonTenantID.String() != "0193a000-0000-7000-8000-000000000001" {
		t.Fatal("common tenant id drifted from the seed migration")
	}
}
