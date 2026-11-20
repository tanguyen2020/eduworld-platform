package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MediaProgressRepo implements write.MediaProgressStore with explicit SQL in the transaction of ctx (TenantScope,
// infra.current_tenant_id(); RLS T) on assessment.video_progress, video_branch_choices and integrity_signals (F065).
type MediaProgressRepo struct{}

var _ write.MediaProgressStore = MediaProgressRepo{}

// ScanProgress reads one row of the progress columns (shared with the read model).
func ScanProgress(row interface{ Scan(...any) error }) (*videolesson.Progress, error) {
	var p videolesson.Progress
	var ranges []byte
	var cues string
	var segment sql.NullString
	var ratio float64
	err := row.Scan(&p.AttemptID, &p.TenantID, &p.SchoolID, &p.PositionMS, &ranges, &ratio, &p.LastHeartbeatAt, &p.ClientSeq,
		&p.Milestones, &cues, &segment, &p.ReachedEnding, &p.Version, &p.PlaysUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("video progress: %w", err)
	}
	p.Ratio = ratio
	p.Ranges = []videolesson.Range{}
	_ = json.Unmarshal(ranges, &p.Ranges)
	p.ProcessedCues = []string{}
	for _, c := range strings.Split(cues, ",") {
		if c != "" {
			p.ProcessedCues = append(p.ProcessedCues, c)
		}
	}
	p.SegmentID = segment.String
	p.LastHeartbeatAt = p.LastHeartbeatAt.UTC()
	return &p, nil
}

// ProgressColumns are the columns ScanProgress reads.
const ProgressColumns = `attempt_id, tenant_id, school_id, position_ms, watched_ranges, watched_ratio::float8, last_heartbeat_at, client_seq,
	milestones_emitted, array_to_string(processed_cue_ids, ','), segment_id, reached_ending, version, plays_used`

// Progress implements write.MediaProgressStore.
func (MediaProgressRepo) Progress(ctx context.Context, attemptID uuid.UUID) (*videolesson.Progress, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return ScanProgress(tx.QueryRowContext(ctx, `SELECT `+ProgressColumns+` FROM assessment.video_progress
		WHERE tenant_id = infra.current_tenant_id() AND attempt_id = $1 AND deleted_at IS NULL FOR UPDATE`, attemptID))
}

// SaveProgress implements write.MediaProgressStore (insert, or update of the live row of the attempt).
func (MediaProgressRepo) SaveProgress(ctx context.Context, p videolesson.Progress) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	ranges, err := json.Marshal(p.Ranges)
	if err != nil {
		return err
	}
	var segment any
	if p.SegmentID != "" {
		segment = p.SegmentID
	}
	cues := p.ProcessedCues
	if cues == nil {
		cues = []string{}
	}
	// tenancyguard:ok the row carries the tenant of the session (RLS WITH CHECK infra.current_tenant_id()); the update
	// targets the live row of the attempt only.
	_, err = tx.ExecContext(ctx, `INSERT INTO assessment.video_progress (attempt_id, tenant_id, school_id, position_ms, watched_ranges,
		watched_ratio, last_heartbeat_at, client_seq, milestones_emitted, processed_cue_ids, segment_id, reached_ending, plays_used)
		VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (attempt_id) DO UPDATE SET position_ms = EXCLUDED.position_ms, watched_ranges = EXCLUDED.watched_ranges,
		  watched_ratio = EXCLUDED.watched_ratio, last_heartbeat_at = EXCLUDED.last_heartbeat_at, client_seq = EXCLUDED.client_seq,
		  milestones_emitted = EXCLUDED.milestones_emitted, processed_cue_ids = EXCLUDED.processed_cue_ids,
		  segment_id = EXCLUDED.segment_id, reached_ending = EXCLUDED.reached_ending, plays_used = EXCLUDED.plays_used, version = assessment.video_progress.version + 1,
		  updated_at = now()
		WHERE assessment.video_progress.deleted_at IS NULL AND assessment.video_progress.tenant_id = infra.current_tenant_id()`,
		p.AttemptID, p.SchoolID, p.PositionMS, ranges, round4(p.Ratio), p.LastHeartbeatAt, p.ClientSeq, p.Milestones,
		"{"+strings.Join(cues, ",")+"}", segment, p.ReachedEnding, p.PlaysUsed)
	return mediaErr(err)
}

func round4(f float64) float64 { return float64(int64(f*10000+0.5)) / 10000 }

// BranchChoices implements write.MediaProgressStore.
func (MediaProgressRepo) BranchChoices(ctx context.Context, attemptID uuid.UUID) ([]write.BranchChoice, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT choice_id, tenant_id, school_id, attempt_id, branch_point_id, option_id, target_segment_id,
		chosen_at FROM assessment.video_branch_choices WHERE tenant_id = infra.current_tenant_id() AND attempt_id = $1
		AND deleted_at IS NULL ORDER BY chosen_at, choice_id`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("branch choices: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.BranchChoice
	for rows.Next() {
		var c write.BranchChoice
		if err := rows.Scan(&c.ID, &c.TenantID, &c.SchoolID, &c.AttemptID, &c.BranchPointID, &c.OptionID, &c.TargetSegmentID, &c.ChosenAt); err != nil {
			return nil, err
		}
		c.ChosenAt = c.ChosenAt.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// InsertBranchChoice implements write.MediaProgressStore.
func (MediaProgressRepo) InsertBranchChoice(ctx context.Context, c write.BranchChoice) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok the row carries the tenant of the session (RLS WITH CHECK infra.current_tenant_id())
	_, err = tx.ExecContext(ctx, `INSERT INTO assessment.video_branch_choices (choice_id, tenant_id, school_id, attempt_id, branch_point_id,
		option_id, target_segment_id, chosen_at) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7)`,
		c.ID, c.SchoolID, c.AttemptID, c.BranchPointID, c.OptionID, c.TargetSegmentID, c.ChosenAt)
	return mediaErr(err)
}

// InsertIntegritySignal implements write.MediaProgressStore.
func (MediaProgressRepo) InsertIntegritySignal(ctx context.Context, s write.IntegritySignal) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	ev := s.Evidence
	if len(ev) == 0 {
		ev = json.RawMessage("{}")
	}
	at := s.CreatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	// tenancyguard:ok append-only row with the tenant of the session (RLS WITH CHECK infra.current_tenant_id())
	_, err = tx.ExecContext(ctx, `INSERT INTO assessment.integrity_signals (signal_id, tenant_id, school_id, student_id, attempt_id,
		signal_type, score, evidence, created_at) VALUES ($1, infra.current_tenant_id(), $2, $3, $4, $5, $6, $7, $8)`,
		s.ID, s.SchoolID, s.StudentID, s.AttemptID, s.SignalType, s.Score, []byte(ev), at)
	return mediaErr(err)
}

func mediaErr(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return problem.New(problem.Conflict, "this branch point already has a choice").Wrap(err)
	}
	return fmt.Errorf("media progress: %w", err)
}
