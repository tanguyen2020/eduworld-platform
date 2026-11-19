//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F020-AC-14 (TC-TEN-024, ADR-0041): every string is bounded — varchar(n) in the database, MaxLen in ent, and
// the domain validation names the field.
func TestStringsAreBounded(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)

	// Database: no unbounded text column in the F020 tables, and an overlong value is refused (22001).
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema IN ('ref','content','infra') AND data_type = 'text'
		AND table_name IN ('tenants','provinces','schools','classes','students','enrollments','content_releases',
			'content_transitions','tenant_resource_assignments')`); n != 0 {
		t.Fatalf("%d text columns", n)
	}
	_, err := e.platform.ExecContext(ctx, `INSERT INTO content.content_transitions (transition_id, created_at, content_type,
		content_version_id, to_status, actor_id, at, note) VALUES ($1, now(), 'question', $1, 'draft', $1, now(), $2)`,
		ids.New(), strings.Repeat("x", 2001))
	if pgCode(err) != "22001" {
		t.Fatalf("note of 2001 characters: %v (want 22001)", err)
	}

	// ent: MaxLen validators refuse before the database.
	staffA := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "school_admin"))
	err = e.uow.Do(staffA, func(ctx context.Context) error {
		return learningClient(ctx).ContentTransition.Create().SetContentType("question").SetContentVersionID(uuid.New()).
			SetToStatus("draft").SetActorID(uuid.New()).SetNote(strings.Repeat("é", 2001)).Exec(ctx)
	})
	var ve *ent.ValidationError
	if !errors.As(err, &ve) || ve.Name != "note" {
		t.Fatalf("ent MaxLen on note: %v", err)
	}

	// Domain: an overlong change note is reported with its field (422 VALIDATION_FAILED via the use case).
	m := contentmodel.ContentMeta{ChangeNote: strings.Repeat("x", 501)}
	var v contentmodel.Violations
	if err := m.Validate(contentmodel.TypeLesson); !errors.As(err, &v) || v[len(v)-1].Field != "change_note" {
		t.Fatalf("domain change_note: %v", err)
	}
}
