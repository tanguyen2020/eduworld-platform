package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Installers implements port.InstallerReads over the identity projections of module game (ref.schools, ref.students),
// inside the read session of the tenant (RLS + explicit tenant condition, ADR-0089 §1).
type Installers struct{}

var _ port.InstallerReads = Installers{}

// SchoolExists implements port.InstallerReads.
func (Installers) SchoolExists(ctx context.Context, tenant, school uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok school projection of the tenant (ref.schools has no deleted_at; status kept)
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ref.schools WHERE tenant_id = $1 AND school_id = $2)`,
		tenant, school).Scan(&ok); err != nil {
		return false, fmt.Errorf("installers: school: %w", err)
	}
	return ok, nil
}

// SchoolGradeLevels implements port.InstallerReads: grade levels of the active, live students of the school.
func (Installers) SchoolGradeLevels(ctx context.Context, tenant, school uuid.UUID) ([]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT grade_level FROM ref.students
		WHERE tenant_id = $1 AND school_id = $2 AND deleted_at IS NULL AND status = 'active' AND grade_level IS NOT NULL
		ORDER BY grade_level`, tenant, school)
	if err != nil {
		return nil, fmt.Errorf("installers: grades: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var g int
		if err := rows.Scan(&g); err != nil {
			return nil, fmt.Errorf("installers: grades scan: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
