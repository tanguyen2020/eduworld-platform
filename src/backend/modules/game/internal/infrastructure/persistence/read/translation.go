package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// TranslationJobs implements read.TranslationJobReader on game.translation_jobs (F082).
type TranslationJobs struct{}

var _ read.TranslationJobReader = TranslationJobs{}

// TranslationJob implements read.TranslationJobReader.
func (TranslationJobs) TranslationJob(ctx context.Context, id uuid.UUID) (*read.TranslationJobRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r read.TranslationJobRow
	var key, msg sql.NullString
	var report []byte
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT job_id, kind, status, format, target_language, dry_run, result_storage_key, report, error_message,
			requested_by, created_at, finished_at
		FROM game.translation_jobs WHERE job_id = $1 AND deleted_at IS NULL`, id).Scan(&r.ID, &r.Kind, &r.Status, &r.Format, &r.TargetLanguage,
		&r.DryRun, &key, &report, &msg, &r.RequestedBy, &r.CreatedAt, &r.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("translation job: %w", err)
	}
	r.ResultKey, r.ErrorMessage = ns(key), ns(msg)
	if len(report) > 0 {
		var rep i18ntext.Report
		if err := json.Unmarshal(report, &rep); err != nil {
			return nil, fmt.Errorf("translation job report: %w", err)
		}
		r.Report = &rep
	}
	return &r, nil
}
