package health

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

// Stable error codes returned in the check output instead of raw errors (no DSN/host leaks).
const (
	CodeTimeout       = "timeout"
	CodeUnreachable   = "unreachable"
	CodeAuth          = "auth_failed"
	CodeNotMigrated   = "not_migrated"
	CodeRevisionLow   = "revision_behind"
	CodeNotConnected  = "not_connected"
	CodeNotConfigured = "not_configured"
	CodeError         = "error"
)

// CodedError carries a stable code for the readiness body.
type CodedError struct {
	Code string
	Err  error
}

func (e *CodedError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *CodedError) Unwrap() error { return e.Err }

func coded(code string, err error) error { return &CodedError{Code: code, Err: err} }

// ErrorCode maps an error to its stable code.
func ErrorCode(err error) string {
	var ce *CodedError
	switch {
	case errors.As(err, &ce):
		return ce.Code
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return CodeTimeout
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "28P01", "28000", "42501":
			return CodeAuth
		case "3D000":
			return CodeNotConfigured
		}
		return CodeError
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return CodeTimeout
		}
		return CodeUnreachable
	}
	if strings.Contains(err.Error(), "connect") || strings.Contains(err.Error(), "dial") {
		return CodeUnreachable
	}
	return CodeError
}

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// PostgresCheck checks the pool of one module database (check name `db:<module>`).
func PostgresCheck(pool Pinger) CheckFunc {
	return func(ctx context.Context) error {
		if pool == nil {
			return coded(CodeNotConfigured, errors.New("pool is nil"))
		}
		return pool.Ping(ctx)
	}
}

// AtlasRevisionCheck verifies the module database has applied at least minVersion
// (check name `atlas:<module>`). Binaries never migrate (F001-AC-03); an unmigrated
// database keeps the binary not ready instead of creating tables.
func AtlasRevisionCheck(pool *pgxpool.Pool, minVersion string) CheckFunc {
	return func(ctx context.Context) error {
		if pool == nil {
			return coded(CodeNotConfigured, errors.New("pool is nil"))
		}
		var current string
		err := pool.QueryRow(ctx,
			`SELECT version FROM atlas_schema_revisions.atlas_schema_revisions
			 WHERE type <> 1 ORDER BY version DESC LIMIT 1`).Scan(&current)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "3F000")) {
				return coded(CodeNotMigrated, err)
			}
			return err
		}
		if current < minVersion {
			return coded(CodeRevisionLow, fmt.Errorf("revision %s < required %s", current, minVersion))
		}
		return nil
	}
}

// ValkeyPinger is satisfied by a small adapter over the Valkey client (see ValkeyCheck callers).
type ValkeyPinger interface {
	Ping(ctx context.Context) error
}

// ValkeyCheck checks Valkey (check name `valkey`).
func ValkeyCheck(p ValkeyPinger) CheckFunc {
	return func(ctx context.Context) error {
		if p == nil {
			return coded(CodeNotConfigured, errors.New("valkey client is nil"))
		}
		return p.Ping(ctx)
	}
}

// NATSProbeTimeout is nats_readiness_probe_timeout (CONFIG_DEFAULTS §18, owner DEPLOYMENT §14): the longest wait for
// the PONG of the readiness PING, below the 2 s budget of one /readyz evaluation (DefaultTimeout).
const NATSProbeTimeout = time.Second

// ProbeNATS reports whether the NATS server answers on conn: the client state must be CONNECTED and a PING must get
// its PONG within timeout (one round trip, a few bytes). The client state alone stays CONNECTED for minutes when the
// server is paused or the network silently drops packets, until the client's own ping timer notices (F132 chaos
// finding), so readiness and eduworld_nats_connected probe the server instead.
func ProbeNATS(ctx context.Context, conn *nats.Conn, timeout time.Duration) error {
	if conn == nil {
		return coded(CodeNotConfigured, errors.New("nats connection is nil"))
	}
	if !conn.IsConnected() {
		return coded(CodeNotConnected, fmt.Errorf("nats status %s", conn.Status()))
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := conn.FlushWithContext(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
			return coded(CodeTimeout, fmt.Errorf("nats ping: %w", err))
		}
		return coded(CodeUnreachable, fmt.Errorf("nats ping: %w", err))
	}
	return nil
}

// NATSCheck checks that the NATS server answers (check name `nats`, ProbeNATS with NATSProbeTimeout).
func NATSCheck(conn *nats.Conn) CheckFunc {
	return func(ctx context.Context) error { return ProbeNATS(ctx, conn, NATSProbeTimeout) }
}

// NATSConnectedGauge is eduworld_nats_connected (MONITORING §3 "Metric bổ sung"): 1 when ProbeNATS succeeds at scrape
// time, 0 otherwise. A binary connects to any node of the cluster (EW_NATS_URLS) and reconnects to another one, so
// every binary at 0 means the whole cluster is unreachable (alert NatsClusterDown, MONITORING §5).
func NATSConnectedGauge(conn *nats.Conn) prometheus.Collector {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "eduworld_nats_connected",
		Help: "1 when the NATS server answers a PING of this binary within nats_readiness_probe_timeout, 0 otherwise.",
	}, func() float64 {
		if ProbeNATS(context.Background(), conn, NATSProbeTimeout) != nil {
			return 0
		}
		return 1
	})
}
