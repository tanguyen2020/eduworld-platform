package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// TranslationRepo implements write.TranslationStore on game.config_translation_states and game.translation_jobs (F082).
// Platform content (G tables, no tenant_id): written under PlatformScope only (role game_platform); every query filters
// deleted_at; no physical delete (R21).
type TranslationRepo struct{}

var _ write.TranslationStore = TranslationRepo{}

func scanState(row interface{ Scan(...any) error }) (*write.TranslationState, error) {
	var s write.TranslationState
	var missing, outdated, meta []byte
	if err := row.Scan(&s.VersionID, &s.ConfigID, &s.ConfigType, &s.State.Status, &missing, &outdated, &meta, &s.ComputedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(missing, &s.State.Missing); err != nil {
		return nil, fmt.Errorf("missing_pointers: %w", err)
	}
	if err := json.Unmarshal(outdated, &s.State.Outdated); err != nil {
		return nil, fmt.Errorf("outdated_pointers: %w", err)
	}
	if err := json.Unmarshal(meta, &s.State.Meta); err != nil {
		return nil, fmt.Errorf("translation_meta: %w", err)
	}
	return &s, nil
}

const stateCols = `s.config_version_id, s.config_id, s.config_type, s.translation_status, s.missing_pointers, s.outdated_pointers,
	s.translation_meta, s.computed_at`

// State implements write.TranslationStore.
func (TranslationRepo) State(ctx context.Context, versionID uuid.UUID) (*write.TranslationState, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanState(t.QueryRowContext(ctx, `SELECT `+stateCols+` FROM game.config_translation_states s
		WHERE s.config_version_id = $1 AND s.deleted_at IS NULL`, versionID))
}

func jsonOr(v any, empty string) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if string(b) == "null" {
		return empty, nil
	}
	return string(b), nil
}

// SaveState implements write.TranslationStore.
func (TranslationRepo) SaveState(ctx context.Context, s write.TranslationState) error {
	missing, err := jsonOr(s.State.Missing, "[]")
	if err != nil {
		return err
	}
	outdated, err := jsonOr(s.State.Outdated, "[]")
	if err != nil {
		return err
	}
	meta, err := jsonOr(s.State.Meta, "{}")
	if err != nil {
		return err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); one live row per version
	_, err = exec(ctx, `INSERT INTO game.config_translation_states (config_version_id, created_at, updated_at, config_type, config_id,
			translation_status, missing_pointers, outdated_pointers, translation_meta, computed_at)
		VALUES ($1, $2, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $2)
		ON CONFLICT (config_version_id) DO UPDATE SET translation_status = EXCLUDED.translation_status,
			missing_pointers = EXCLUDED.missing_pointers, outdated_pointers = EXCLUDED.outdated_pointers,
			translation_meta = EXCLUDED.translation_meta, computed_at = EXCLUDED.computed_at, updated_at = EXCLUDED.updated_at,
			version = game.config_translation_states.version + 1`,
		s.VersionID, s.ComputedAt, s.ConfigType, s.ConfigID, s.State.Status, missing, outdated, meta)
	if err != nil {
		return fmt.Errorf("translation state: %w", err)
	}
	return nil
}

// VersionsWithoutState implements write.TranslationStore.
func (TranslationRepo) VersionsWithoutState(ctx context.Context, limit int) ([]write.ConfigVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+prefixed("v.", versionCols)+` FROM game.config_versions v
		JOIN game.config_items c ON c.config_id = v.config_id AND c.deleted_at IS NULL
		WHERE v.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM game.config_translation_states s WHERE s.config_version_id = v.config_version_id)
		ORDER BY v.created_at, v.config_version_id LIMIT $1 FOR UPDATE OF v SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("versions without state: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ConfigVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// LatestVersions implements write.TranslationStore.
func (TranslationRepo) LatestVersions(ctx context.Context, itemTypes []string, ids []uuid.UUID, after *uuid.UUID, limit int) ([]write.LatestVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var idFilter any
	if ids != nil {
		idFilter = uuidStrings(ids)
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); only the console items of module game (no tenant rows exist
	// in game.config_items: school events stay out of the console, F082-AC-17); deleted_at filtered; keyset on config_id
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (c.config_id) c.config_id, c.item_type, c.item_key, c.latest_version_no,
			`+prefixed("v.", versionCols)+`,
			s.config_version_id, s.config_type, s.translation_status, s.missing_pointers, s.outdated_pointers, s.translation_meta, s.computed_at
		FROM game.config_items c
		JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
		LEFT JOIN game.config_translation_states s ON s.config_version_id = v.config_version_id AND s.deleted_at IS NULL
		WHERE c.deleted_at IS NULL AND c.item_type = ANY($1::varchar[]) AND ($2::uuid[] IS NULL OR c.config_id = ANY($2::uuid[]))
			AND ($3::uuid IS NULL OR c.config_id > $3)
		ORDER BY c.config_id, v.version_no DESC LIMIT $4`, itemTypes, idFilter, after, limit)
	if err != nil {
		return nil, fmt.Errorf("latest versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.LatestVersion
	for rows.Next() {
		var lv write.LatestVersion
		var v write.ConfigVersion
		var title, note, change sql.NullString
		var payload []byte
		var sid uuid.NullUUID
		var stype, sstatus sql.NullString
		var missing, outdated, meta []byte
		var computed sql.NullTime
		if err := rows.Scan(&lv.Item.ConfigID, &lv.Item.ItemType, &lv.Item.ItemKey, &lv.Item.LatestVersionNo,
			&v.ID, &v.ConfigID, &v.ItemType, &v.VersionNo, &v.Status, &title, &payload, &v.SHA256, &v.AuthorID, &v.ReviewerID,
			&note, &change, &v.BasedOn, &v.SubmittedAt, &v.ReviewedAt, &v.PublishedAt, &v.PublishedBy, &v.UpdatedAt, &v.Revision,
			&sid, &stype, &sstatus, &missing, &outdated, &meta, &computed); err != nil {
			return nil, err
		}
		v.Payload = payload
		v.Title, v.ReviewNote, v.ChangeNote = strPtr(title), strPtr(note), strPtr(change)
		lv.Version = v
		if sid.Valid {
			st := write.TranslationState{VersionID: sid.UUID, ConfigID: lv.Item.ConfigID, ConfigType: stype.String, ComputedAt: computed.Time}
			st.State.Status = sstatus.String
			if err := errors.Join(json.Unmarshal(missing, &st.State.Missing), json.Unmarshal(outdated, &st.State.Outdated),
				json.Unmarshal(meta, &st.State.Meta)); err != nil {
				return nil, fmt.Errorf("translation state: %w", err)
			}
			lv.State = &st
		}
		out = append(out, lv)
	}
	return out, rows.Err()
}

func prefixed(p, cols string) string {
	out := ""
	for i, c := range splitCols(cols) {
		if i > 0 {
			out += ", "
		}
		out += p + c
	}
	return out
}

func splitCols(cols string) []string {
	var out []string
	cur := ""
	for _, r := range cols {
		switch r {
		case ',':
			out = append(out, cur)
			cur = ""
		case ' ', '\n', '\t':
		default:
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

const jobCols = `job_id, kind, status, format, source_language, target_language, only_missing, dry_run, request, input_asset_id,
	result_storage_key, report, error_message, requested_by, idempotency_key, attempts, started_at, finished_at, created_at`

type jobRequest struct {
	ContentTypes []string    `json:"content_types"`
	ContentIDs   []uuid.UUID `json:"content_ids"`
}

func scanJob(row interface{ Scan(...any) error }) (*write.TranslationJob, error) {
	var j write.TranslationJob
	var request, report []byte
	var key, errMsg, idem sql.NullString
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Format, &j.SourceLanguage, &j.TargetLanguage, &j.OnlyMissing, &j.DryRun, &request,
		&j.InputAssetID, &key, &report, &errMsg, &j.RequestedBy, &idem, &j.Attempts, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var req jobRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("job request: %w", err)
	}
	j.ContentTypes, j.ContentIDs = req.ContentTypes, req.ContentIDs
	if len(report) > 0 {
		var r i18ntext.Report
		if err := json.Unmarshal(report, &r); err != nil {
			return nil, fmt.Errorf("job report: %w", err)
		}
		j.Report = &r
	}
	j.ResultKey, j.ErrorMessage, j.IdempotencyKey = strPtr(key), strPtr(errMsg), strPtr(idem)
	return &j, nil
}

// InsertJob implements write.TranslationStore.
func (TranslationRepo) InsertJob(ctx context.Context, j write.TranslationJob) error {
	req, err := json.Marshal(jobRequest{ContentTypes: j.ContentTypes, ContentIDs: j.ContentIDs})
	if err != nil {
		return err
	}
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err = exec(ctx, `INSERT INTO game.translation_jobs (job_id, created_at, created_by, updated_at, updated_by, kind, status, format,
			source_language, target_language, only_missing, dry_run, request, input_asset_id, requested_by, idempotency_key)
		VALUES ($1, $2, $3, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $3, $13)`,
		j.ID, j.CreatedAt, j.RequestedBy, j.Kind, j.Status, j.Format, j.SourceLanguage, j.TargetLanguage, j.OnlyMissing, j.DryRun, string(req),
		j.InputAssetID, j.IdempotencyKey)
	if uniqueViolation(err) {
		return write.ErrConflict
	}
	return err
}

// JobByIdempotency implements write.TranslationStore.
func (TranslationRepo) JobByIdempotency(ctx context.Context, requestedBy uuid.UUID, key string) (*write.TranslationJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanJob(t.QueryRowContext(ctx, `SELECT `+jobCols+` FROM game.translation_jobs
		WHERE requested_by = $1 AND idempotency_key = $2 AND deleted_at IS NULL`, requestedBy, key))
}

// ClaimJob implements write.TranslationStore.
func (TranslationRepo) ClaimJob(ctx context.Context, now time.Time, lease time.Duration) (*write.TranslationJob, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; one worker per job (SKIP LOCKED)
	return scanJob(t.QueryRowContext(ctx, `SELECT `+jobCols+` FROM game.translation_jobs
		WHERE deleted_at IS NULL AND (status = 'queued' OR (status = 'running' AND started_at < $1))
		ORDER BY created_at, job_id LIMIT 1 FOR UPDATE SKIP LOCKED`, now.Add(-lease)))
}

// SaveJob implements write.TranslationStore.
func (TranslationRepo) SaveJob(ctx context.Context, j write.TranslationJob, at time.Time) error {
	var report any
	if j.Report != nil {
		b, err := json.Marshal(j.Report)
		if err != nil {
			return err
		}
		report = string(b)
	}
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err := exec(ctx, `UPDATE game.translation_jobs SET status = $2, result_storage_key = $3, report = $4::jsonb, error_message = $5,
			attempts = $6, started_at = $7, finished_at = $8, updated_at = $9, version = version + 1
		WHERE job_id = $1 AND deleted_at IS NULL`, j.ID, j.Status, j.ResultKey, report, j.ErrorMessage, j.Attempts, j.StartedAt, j.FinishedAt, at)
	if err != nil {
		return fmt.Errorf("translation job: %w", err)
	}
	return nil
}
