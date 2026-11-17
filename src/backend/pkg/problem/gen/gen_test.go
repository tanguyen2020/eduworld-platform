package main

import (
	"strings"
	"testing"
)

const tableMD = `## 4. Lỗi
### 4.1. Catalog mã lỗi

| HTTP | ` + "`code`" + ` | Khi nào |
|---|---|---|
| 400 | ` + "`BAD_REQUEST`" + ` | x |
| 403 | ` + "`TENANT_MISMATCH`" + ` / ` + "`PERMISSION_DENIED`" + ` | y |
| 503 | ` + "`UNAVAILABLE`" + ` | z |

## 5. Khác
| 418 | ` + "`NOT_IN_SECTION`" + ` | ignored |
`

func TestParseStatuses(t *testing.T) {
	st, err := parseStatuses(tableMD)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"BAD_REQUEST": 400, "TENANT_MISMATCH": 403, "PERMISSION_DENIED": 403, "UNAVAILABLE": 503}
	if len(st) != len(want) {
		t.Fatalf("got %v", st)
	}
	for k, v := range want {
		if st[k] != v {
			t.Fatalf("%s = %d, want %d", k, st[k], v)
		}
	}
}

// F002-AC-21: a code in ProblemCode without a status row (or the reverse) is an error, so gen-check fails.
func TestRenderRejectsMismatch(t *testing.T) {
	if _, err := render([]string{"BAD_REQUEST", "NEW_CODE"}, map[string]int{"BAD_REQUEST": 400}); err == nil {
		t.Fatal("enum code without status accepted")
	}
	if _, err := render([]string{"BAD_REQUEST"}, map[string]int{"BAD_REQUEST": 400, "EXTRA": 500}); err == nil {
		t.Fatal("status row without enum code accepted")
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	codes := []string{"UNAVAILABLE", "BAD_REQUEST", "TENANT_HEADER_REQUIRED"}
	st := map[string]int{"UNAVAILABLE": 503, "BAD_REQUEST": 400, "TENANT_HEADER_REQUIRED": 400}
	a, err := render(codes, st)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := render([]string{"TENANT_HEADER_REQUIRED", "UNAVAILABLE", "BAD_REQUEST"}, st)
	if string(a) != string(b) {
		t.Fatal("output depends on input order")
	}
	if !strings.Contains(string(a), `TenantHeaderRequired Code = "TENANT_HEADER_REQUIRED"`) || !strings.Contains(string(a), "DO NOT EDIT") {
		t.Fatalf("unexpected output:\n%s", a)
	}
}
