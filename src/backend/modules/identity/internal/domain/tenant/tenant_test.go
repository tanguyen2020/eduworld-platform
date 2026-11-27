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
	if CommonTenantID.String() != "0193a000-0000-7000-8000-000000000001" {
		t.Fatal("common tenant id drifted from the seed migration")
	}
}

// F134-AC-07: the QA flag is a patch field; the common tenant can never be flagged.
func TestTenantQAFlag(t *testing.T) {
	tn, err := New(uuid.New(), "qa-probe", "Tenant QA", TypeSchool, "qa-probe", "")
	if err != nil {
		t.Fatal(err)
	}
	on, off := true, false
	if c, err := tn.Apply(Patch{IsQA: &on}); err != nil || !c.QAChanged || !tn.IsQA {
		t.Fatalf("flag: %+v %v", c, err)
	}
	if _, err := tn.Apply(Patch{IsQA: &on}); err != ErrNoChange {
		t.Fatalf("same flag: %v", err)
	}
	if c, err := tn.Apply(Patch{IsQA: &off}); err != nil || !c.QAChanged || tn.IsQA {
		t.Fatalf("unflag: %+v %v", c, err)
	}
	common := &Tenant{ID: CommonTenantID, Type: TypeCommon, Name: "Common", Status: StatusActive}
	if _, err := common.Apply(Patch{IsQA: &on}); err == nil || common.IsQA {
		t.Fatal("the common tenant was flagged QA")
	}
	var inv ErrInvalid
	if err := common.SetQA(true); !errors.As(err, &inv) || inv.Field != "is_qa" {
		t.Fatalf("SetQA(common) = %v", err)
	}
}
