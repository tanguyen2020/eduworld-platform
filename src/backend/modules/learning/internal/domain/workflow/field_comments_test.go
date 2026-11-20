package workflow

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// F025 T046 (OQ F054-1): field comments ride on a reject only, ≤ 50, JSON pointer ≤ 256, comment 1..1000.
func TestCheckFieldComments(t *testing.T) {
	ok := []FieldComment{{Path: "/stem/vi", Comment: "Sai chính tả"}, {Path: "", Comment: "Toàn bộ"}, {Path: "/a~1b/~0c", Comment: "x"}}
	if vs := CheckFieldComments(contentmodel.ActionReject, ok); len(vs) != 0 {
		t.Fatalf("valid comments refused: %+v", vs)
	}
	if vs := CheckFieldComments(contentmodel.ActionApprove, nil); len(vs) != 0 {
		t.Fatalf("no comments: %+v", vs)
	}
	if vs := CheckFieldComments(contentmodel.ActionApprove, ok[:1]); len(vs) != 1 || vs[0].Field != "/field_comments" {
		t.Fatalf("approve with comments: %+v", vs)
	}
	many := make([]FieldComment, 51)
	for i := range many {
		many[i] = FieldComment{Path: "/x", Comment: "y"}
	}
	if vs := CheckFieldComments(contentmodel.ActionReject, many); len(vs) != 1 {
		t.Fatalf("51 comments: %+v", vs)
	}
	bad := []FieldComment{{Path: "stem", Comment: "x"}, {Path: "/" + strings.Repeat("a", 256), Comment: "x"}, {Path: "/a~2", Comment: "x"},
		{Path: "/ok", Comment: "  "}, {Path: "/ok", Comment: strings.Repeat("ă", 1001)}}
	vs := CheckFieldComments(contentmodel.ActionReject, bad)
	want := []string{"/field_comments/0/path", "/field_comments/1/path", "/field_comments/2/path", "/field_comments/3/comment",
		"/field_comments/4/comment"}
	if len(vs) != len(want) {
		t.Fatalf("violations %+v", vs)
	}
	for i, w := range want {
		if vs[i].Field != w {
			t.Fatalf("violation %d = %s, want %s", i, vs[i].Field, w)
		}
	}
	if !vs[3].Required || vs[4].Required {
		t.Fatalf("required flags %+v", vs)
	}
}
