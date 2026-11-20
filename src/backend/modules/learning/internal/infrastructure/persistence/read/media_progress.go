package read

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"database/sql"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
)

// MediaProgressModel implements read.MediaProgressReadModel with explicit SQL on the live view (TenantScope of the read
// session; RLS T).
type MediaProgressModel struct{}

var _ read.MediaProgressReadModel = MediaProgressModel{}

// Progress implements read.MediaProgressReadModel.
func (MediaProgressModel) Progress(ctx context.Context, attemptID uuid.UUID) (*videolesson.Progress, error) {
	q, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); the _live view has deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT attempt_id, tenant_id, school_id, position_ms, watched_ranges, watched_ratio::float8,
		last_heartbeat_at, client_seq, milestones_emitted, array_to_string(processed_cue_ids, ','), segment_id, reached_ending, version
		FROM assessment.video_progress_live WHERE tenant_id = infra.current_tenant_id() AND attempt_id = $1 AND deleted_at IS NULL`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("video progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var p videolesson.Progress
	var ranges []byte
	var cues string
	var segment sql.NullString
	if err := rows.Scan(&p.AttemptID, &p.TenantID, &p.SchoolID, &p.PositionMS, &ranges, &p.Ratio, &p.LastHeartbeatAt, &p.ClientSeq,
		&p.Milestones, &cues, &segment, &p.ReachedEnding, &p.Version); err != nil {
		return nil, fmt.Errorf("video progress: %w", err)
	}
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
	return &p, rows.Err()
}
