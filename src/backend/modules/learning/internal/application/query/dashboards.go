package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DashboardCache caches serialized dashboard answers per tenant data version (Valkey, DASHBOARD_DATA §9). It is never
// needed for a correct answer (ADR-0004): every error means "no cache".
type DashboardCache interface {
	Version(ctx context.Context, tenantID uuid.UUID) (int64, error)
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// DashboardLimiter bounds the dashboard queries running at the same time per user and per tenant (DASHBOARD_DATA §7):
// ok = false when a limit is reached; release must be called once the query ended.
type DashboardLimiter interface {
	Acquire(ctx context.Context, userID, tenantID uuid.UUID, perUser, perTenant int) (release func(), ok bool)
}

// DashboardMetrics observes the dashboard queries (DASHBOARD_DATA §13).
type DashboardMetrics interface {
	Query(queryID, grain, cache string, seconds float64)
	Rejected(reason string)
}

// BusyError is 429 DASHBOARD_BUSY with its Retry-After (F043-AC-19).
type BusyError struct {
	After time.Duration
	Err   *problem.Error
}

func (e *BusyError) Error() string { return e.Err.Error() }

// Unwrap exposes the problem error.
func (e *BusyError) Unwrap() error { return e.Err }

// Seconds is the Retry-After value (at least 1).
func (e *BusyError) Seconds() string {
	return strconv.Itoa(max(int((e.After+time.Second-1)/time.Second), 1))
}

// Dashboards are the dashboard reads of F043 (getSchoolDashboard, getClassDashboard, getSchoolAnalyticsSeries,
// getStudentLearningProfile, getTenantLearningSummary, getPlatformLearningSummary): rollups and read models only, through
// the dashboard read session (read replica, role learning_dashboard, never the primary — F043-AC-18, F043-AC-21), with range
// and grain rules, statement timeout, concurrency limits and a cache keyed by tenant and data version.
type Dashboards struct {
	Session read.Session // the dashboard session (nil = not configured → 503)
	Model   read.DashboardReader
	Cache   DashboardCache
	Limiter DashboardLimiter
	Metrics DashboardMetrics
	Config  analytics.Config
	Cursors *page.Codec
	Clock   func() time.Time
}

func (q Dashboards) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func (q Dashboards) rejected(reason string) {
	if q.Metrics != nil {
		q.Metrics.Rejected(reason)
	}
}

// dashRange validates from/to/grain (F043-AC-16, F043-AC-17).
func (q Dashboards) dashRange(from, to time.Time, grain string) (analytics.Range, error) {
	rg, err := analytics.NewRange(from, to, analytics.Grain(grain), q.Config.Range)
	if err != nil {
		q.rejected("range")
		return rg, problem.New(problem.DashboardRangeTooLarge, err.Error())
	}
	return rg, nil
}

// unavailable maps every error that is not a problem (replica down, connection refused) to 503 without falling back to the
// primary (F043-AC-21).
func unavailable(ctx context.Context, err error) error {
	var pe *problem.Error
	if err == nil || errors.As(err, &pe) {
		return err
	}
	slog.ErrorContext(ctx, "dashboard database unavailable", "error", err)
	return problem.New(problem.Unavailable, "dashboard database unavailable").Retry().Wrap(err)
}

func cacheKey(tenant uuid.UUID, ver int64, queryID string, parts ...any) string {
	b, _ := json.Marshal(append([]any{queryID}, parts...))
	h := sha256.Sum256(b)
	return fmt.Sprintf("ew:learning:dash:res:%s:%d:%s", tenant, ver, hex.EncodeToString(h[:16]))
}

// dashRun runs one catalog query: concurrency limit, cache lookup, read session with the statement timeout, cache fill.
func dashRun[T any](ctx context.Context, q Dashboards, queryID string, p authz.Principal, tenant uuid.UUID, rg *analytics.Range,
	key []any, fn func(ctx context.Context) (T, error)) (out T, err error) {
	if q.Session == nil || q.Model == nil {
		return out, problem.New(problem.Unavailable, "dashboard database not configured")
	}
	start := time.Now()
	grain := ""
	if rg != nil {
		grain = string(rg.Grain)
	}
	if q.Limiter != nil {
		release, ok := q.Limiter.Acquire(ctx, p.UserID, tenant, q.Config.MaxConcurrentPerUser, q.Config.MaxConcurrentTenant)
		if !ok {
			q.rejected("busy")
			return out, &BusyError{After: time.Second, Err: problem.New(problem.DashboardBusy, "")}
		}
		defer release()
	}
	ck := ""
	if q.Cache != nil && tenant != uuid.Nil {
		if ver, err := q.Cache.Version(ctx, tenant); err == nil {
			ck = cacheKey(tenant, ver, queryID, key...)
			if b, ok, err := q.Cache.Get(ctx, ck); err == nil && ok && json.Unmarshal(b, &out) == nil {
				if q.Metrics != nil {
					q.Metrics.Query(queryID, grain, "hit", time.Since(start).Seconds())
				}
				return out, nil
			}
		}
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if err := q.Model.Prepare(ctx, q.Config.StatementTimeout); err != nil {
			return err
		}
		var err error
		out, err = fn(ctx)
		return err
	})
	if err = unavailable(ctx, err); err != nil {
		if problemCode(err) == problem.DashboardQueryTimeout {
			q.rejected("timeout")
		}
		return out, err
	}
	if ck != "" {
		ttl := q.Config.CacheTTLClosed
		if rg == nil || !rg.To.Before(q.Config.Day(q.now())) {
			ttl = q.Config.CacheTTLToday
		}
		if b, err := json.Marshal(out); err == nil {
			_ = q.Cache.Set(ctx, ck, b, ttl)
		}
	}
	if q.Metrics != nil {
		q.Metrics.Query(queryID, grain, "miss", time.Since(start).Seconds())
	}
	return out, nil
}

func problemCode(err error) problem.Code {
	var pe *problem.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// staff returns the principal of a tenant staff member (students, platform staff without tenant such as safety_moderator
// → 403 PERMISSION_DENIED, F043-AC-14).
func staff(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// grantSubjects are the subjects of the grants holding perm that cover school: nil when one of them is not subject-limited.
func grantSubjects(p authz.Principal, perm authz.Permission, school uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) || (g.SchoolID != nil && *g.SchoolID != school) {
			continue
		}
		if g.SubjectID == nil {
			return nil
		}
		out = append(out, *g.SubjectID)
	}
	return out
}

// schoolScope applies analytics.school.read to a school (F043-AC-13, F043-AC-14, F043-AC-15): no permission → 403; school
// outside every grant → 404 (no existence leak, authz.denied audited); subject outside a subject-limited grant → 403
// PERMISSION_DENIED (404 when subjectNotFound, student profile F043-AC-24); without subject, a subject-limited grant reads
// its subjects only [Proposed].
func schoolScope(ctx context.Context, p authz.Principal, school uuid.UUID, subject *uuid.UUID, subjectNotFound bool) (read.DashScope, error) {
	perm := authz.PermAnalyticsSchoolRead
	if !holds(p, perm) {
		return read.DashScope{}, authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID})
	}
	if !coversSchool(p, perm, school) {
		_ = authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return read.DashScope{}, problem.New(problem.NotFound, "")
	}
	sc := read.DashScope{TenantID: *p.TenantID, SchoolIDs: []uuid.UUID{school}}
	if subject != nil {
		if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: p.TenantID, SchoolID: &school, SubjectID: subject}); err != nil {
			if subjectNotFound {
				return sc, problem.New(problem.NotFound, "")
			}
			return sc, err
		}
		sc.SubjectIDs = []uuid.UUID{*subject}
		return sc, nil
	}
	sc.SubjectIDs = grantSubjects(p, perm, school)
	return sc, nil
}

// meta builds the DashboardMeta of an answer: data_as_of = oldest computed_at read (now when nothing was read), is_partial
// when a provisional row was read or the range contains today (spec edge case).
func (q Dashboards) meta(rg analytics.Range, m read.RollupMeta) dto.DashboardMeta {
	now := q.now()
	asOf := now
	if m.Oldest != nil {
		asOf = m.Oldest.UTC()
	}
	return dto.DashboardMeta{Grain: string(rg.Grain), DataAsOf: asOf, IsPartial: m.Partial || !rg.To.Before(q.Config.Day(now)),
		From: rg.From.Format(time.DateOnly), To: rg.To.Format(time.DateOnly)}
}

// lastDay is the last calendar day of the range that is not in the future.
func (q Dashboards) lastDay(rg analytics.Range) time.Time {
	today := q.Config.Day(q.now())
	if rg.To.After(today) {
		return today
	}
	return rg.To
}

func minutes(seconds int64) int { return int(seconds / 60) }

func rapidFlag(num, den int64, th analytics.Thresholds) bool {
	return den > 0 && float64(num) >= th.RapidGuessRatio*float64(den)
}

func stamp(t time.Time) read.RollupMeta {
	if t.IsZero() {
		return read.RollupMeta{}
	}
	return read.RollupMeta{Oldest: &t}
}

// crossTenantIDs returns the tenants of the cross-tenant scope of a System Admin request (tenant_ids), if any.
func crossTenantIDs(ctx context.Context) ([]uuid.UUID, bool) {
	sc, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return nil, false
	}
	if cs, cross := sc.(tenancy.CrossTenantScope); cross {
		return cs.TenantIDs(), true
	}
	return nil, false
}
