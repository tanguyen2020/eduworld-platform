package read

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// AvailableFrom implements read.DashboardReader (query available_from, F043 T042): the earliest daily bucket of
// rollup_school_daily of the schools of the scope (the whole tenant when SchoolIDs is nil). Grade and subject filters are
// not applied: the date is the start of the data of the school (ADR-0095: no class level).
func (DashboardModel) AvailableFrom(ctx context.Context, sc read.DashScope) (*time.Time, error) {
	q, err := dtx(ctx, "available_from")
	if err != nil {
		return nil, err
	}
	var d sql.NullTime
	// tenancyguard:ok derived rollup (no deleted_at): explicit tenant_id and schools of the scope (index path of the PK)
	err = q.row(ctx, `SELECT min(r.bucket_date) FROM learning.rollup_school_daily r
			WHERE r.tenant_id = $1 AND ($2::uuid[] IS NULL OR r.school_id = ANY($2::uuid[])) AND r.metric_version = 1`,
		sc.TenantID, strIDs(sc.SchoolIDs)).Scan(&d)
	if err != nil || !d.Valid {
		return nil, dashErr("available_from", err)
	}
	t := d.Time.UTC()
	return &t, nil
}

// ReplicaLag implements read.DashboardReader: the replay lag of the replica serving the dashboard session — 0 on a primary
// (local, tests) and on a replica that replayed everything it received (an idle primary is not a lag). It runs in a
// savepoint so that a failure never aborts the read transaction of the answer.
func (DashboardModel) ReplicaLag(ctx context.Context) (time.Duration, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `SAVEPOINT replica_lag`); err != nil {
		return 0, fmt.Errorf("replica lag: %w", err)
	}
	var seconds float64
	err = tx.QueryRowContext(ctx, `SELECT CASE WHEN NOT pg_is_in_recovery() OR pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 0
		ELSE COALESCE(EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp()), 0) END`).Scan(&seconds)
	if err != nil {
		if _, rbErr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT replica_lag`); rbErr != nil {
			slog.WarnContext(ctx, "replica lag: rollback to savepoint", "error", rbErr)
		}
		return 0, fmt.Errorf("replica lag: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT replica_lag`); err != nil {
		return 0, fmt.Errorf("replica lag: %w", err)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
