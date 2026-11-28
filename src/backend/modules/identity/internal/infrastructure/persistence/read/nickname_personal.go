package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// NicknamePersonalModel implements read.NicknamePersonalReader in a read-only session under the TenantScope of the
// caller (screenNicknamePersonalData, F136): the names never leave module identity and are never logged (R14, R16).
type NicknamePersonalModel struct{ Session *entx.ReadSession }

var _ read.NicknamePersonalReader = NicknamePersonalModel{}

// NicknamePersonal implements read.NicknamePersonalReader.
func (m NicknamePersonalModel) NicknamePersonal(ctx context.Context, tenantID, studentID uuid.UUID) (names, schools []string, found bool, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		var full, display, school sql.NullString
		var i18n []byte
		// tenancyguard:ok org.students/org.schools under the TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
		err := tx.QueryRowContext(ctx, `SELECT s.full_name, u.display_name, sc.name, sc.name_i18n
			FROM org.students s
			JOIN org.schools sc ON sc.school_id = s.school_id AND sc.deleted_at IS NULL
			LEFT JOIN identity.users u ON u.user_id = s.student_id AND u.deleted_at IS NULL
			WHERE s.tenant_id = $1 AND s.student_id = $2 AND s.deleted_at IS NULL`, tenantID, studentID).
			Scan(&full, &display, &school, &i18n)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("nickname personal data: %w", err)
		}
		found = true
		for _, n := range []sql.NullString{full, display} {
			if n.Valid && n.String != "" {
				names = append(names, n.String)
			}
		}
		if school.Valid && school.String != "" {
			schools = append(schools, school.String)
		}
		var tr map[string]string
		if len(i18n) > 0 && json.Unmarshal(i18n, &tr) == nil {
			for _, v := range tr {
				if v != "" {
					schools = append(schools, v)
				}
			}
		}
		return nil
	})
	return names, schools, found, err
}
