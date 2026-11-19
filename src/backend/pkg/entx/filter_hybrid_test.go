package entx

import (
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// The H filter selects assigned platform rows with EXISTS (SELECT 1 …): the literal 1 must not be quoted as a
// column (F020 found `SELECT "1"`), and the assignment is matched on the resource id column (ResourceIDColumn
// of version tables).
func TestHybridPredicateSQL(t *testing.T) {
	tenant := uuid.New()
	ts, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), ts)
	h := HybridContent{ResourceType: "question", PK: PK{Column: "question_version_id"}, ResourceIDColumn: "question_id"}
	ps, err := predicates(ctx, KindH, h.hybrid())
	if err != nil {
		t.Fatal(err)
	}
	s := sql.Dialect(dialect.Postgres).Select("*").From(sql.Table("question_versions"))
	for _, p := range ps {
		p(s)
	}
	q, _ := s.Query()
	for _, want := range []string{`EXISTS (SELECT 1 FROM "infra"."tenant_resource_assignments"`, `"question_versions"."question_id"`, `"deleted_at" IS NULL`} {
		if !strings.Contains(q, want) {
			t.Fatalf("query %s\nmissing %s", q, want)
		}
	}
	if strings.Contains(q, `"1"`) {
		t.Fatalf("quoted literal in %s", q)
	}
}
