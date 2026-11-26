package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TelemetryBatch is the decoded body of postTelemetry (TelemetryBatch of the contract); Events keep every attribute the
// client sent so the closed schema can be checked per type.
type TelemetryBatch struct {
	Platform      string
	ClientVersion string
	DeviceTier    string
	Preset        string
	Renderer      string
	Locale        string
	Events        []map[string]any
}

// AnalyticsIngest is the ingestion side of Game Analytics (F080 US1, US2): client telemetry of postTelemetry and the
// domain events of the durable consumer analytics, written to analytics.events_raw with pseudonymous keys (HMAC of the
// student / game session, FR-ANL-008), the standard dimensions and catalog props only; and the anonymisation of a student.
type AnalyticsIngest struct {
	UoW     *entx.UnitOfWork
	Store   port.AnalyticsStore
	Keys    analytics.KeyRing
	Rate    port.RateLimiter
	Metrics port.AnalyticsMetrics
	Config  analytics.Config
	Clock   func() time.Time
	Log     *slog.Logger
}

func (h AnalyticsIngest) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

func (h AnalyticsIngest) logger() *slog.Logger {
	if h.Log != nil {
		return h.Log
	}
	return slog.Default()
}

func (h AnalyticsIngest) metric(f func(m port.AnalyticsMetrics)) {
	if h.Metrics != nil {
		f(h.Metrics)
	}
}

// PostTelemetry is postTelemetry (F080-AC-01…04, AC-34): a student of the session tenant (game_player, R23) sends ≤ 100
// events; 30 batches per minute (429 RATE_LIMITED + Retry-After); every event is validated against the closed schema of
// its type (422 VALIDATION_FAILED, the whole batch); valid events are stored once per client_event_id, except QA tenants,
// product events of a tenant that switched product telemetry off, events from the future or older than the late window
// (dropped, metric only). client_error/crash also go to error tracking (a WARN log with crash_signature, no PII). The
// session_key comes from the game session of the student, never from the client; nothing here blocks play or grants
// anything (ADR-0016).
func (h AnalyticsIngest) PostTelemetry(ctx context.Context, b TelemetryBatch) (accepted int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PostTelemetry")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return 0, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return 0, problem.New(problem.GameAccessStudentOnly, "")
	}
	if len(h.Keys.All()) == 0 {
		return 0, problem.New(problem.Unavailable, "analytics key not configured").Retry()
	}
	if len(b.Events) == 0 || len(b.Events) > 100 {
		return 0, problem.Invalid("/events", problem.SubInvalid, "1 to 100 events")
	}
	if h.Rate != nil && h.Config.TelemetryRatePerMin > 0 {
		ok, wait, rerr := h.Rate.Allow(ctx, "telemetry:"+p.UserID.String(), h.Config.TelemetryRatePerMin, time.Minute)
		if rerr == nil && !ok {
			return 0, RetryAfter{Err: problem.New(problem.RateLimited, "telemetry: 30 batches per minute"), After: wait}
		}
	}
	type valid struct {
		typ   string
		at    time.Time
		ceid  *uuid.UUID
		attrs map[string]any
	}
	events := make([]valid, 0, len(b.Events))
	for i, e := range b.Events {
		typ, _ := e["type"].(string)
		if err := analytics.ValidateTelemetry(i, typ, e); err != nil {
			var fe analytics.FieldError
			if errors.As(err, &fe) {
				return 0, problem.Invalid(fe.Pointer, problem.SubInvalid, fe.Reason)
			}
			return 0, err
		}
		s, _ := e["occurred_at"].(string)
		at, perr := time.Parse(time.RFC3339Nano, s)
		if perr != nil {
			return 0, problem.Invalid("/events/"+itoa(i)+"/occurred_at", problem.SubInvalid, "RFC 3339 timestamp")
		}
		v := valid{typ: typ, at: at.UTC(), attrs: e}
		if c, ok := e["client_event_id"].(string); ok {
			id, perr := uuid.Parse(c)
			if perr != nil {
				return 0, problem.Invalid("/events/"+itoa(i)+"/client_event_id", problem.SubInvalid, "UUID")
			}
			v.ceid = &id
		}
		events = append(events, v)
	}
	tenant, student, key := *p.TenantID, p.UserID, h.Keys.Current()
	player := analytics.Pseudonym(key, student)
	now := h.now()
	for _, e := range events {
		if e.typ == "client_error" || e.typ == "crash" {
			// Error tracking (MONITORING, F080-AC-04): technical fields only — no token, student id, name or IP.
			h.logger().WarnContext(ctx, "game client error", "type", e.typ, "error_code", e.attrs["error_code"], "scene_code",
				e.attrs["scene_code"], "fatal", e.attrs["fatal"], "crash_signature", e.attrs["crash_signature"], "platform", b.Platform,
				"client_version", b.ClientVersion)
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		flags, err := h.Store.TenantFlags(ctx, tenant)
		if err != nil {
			return err
		}
		dims, err := h.Store.StudentDims(ctx, tenant, student)
		if err != nil {
			return err
		}
		product := h.Config.ProductTelemetry && (!flags.Known || flags.ProductTelemetry)
		rows := make([]port.RawEvent, 0, len(events))
		for _, e := range events {
			if reason := analytics.DropTelemetry(e.typ, e.at, now, product, flags.QA, h.Config); reason != analytics.Keep {
				h.metric(func(m port.AnalyticsMetrics) { m.Telemetry(e.typ, false, string(reason)) })
				if reason == analytics.DropLate {
					h.metric(func(m port.AnalyticsMetrics) { m.LateEvent() })
				}
				continue
			}
			props := analytics.TelemetryProps(e.typ, e.attrs)
			if e.typ != "screen_view" && e.typ != "tutorial_step" && e.typ != "break_reminder_shown" {
				if b.Preset != "" {
					if _, ok := props["preset"]; !ok {
						props["preset"] = b.Preset
					}
				}
				if b.Renderer != "" {
					if _, ok := props["renderer"]; !ok {
						props["renderer"] = b.Renderer
					}
				}
			}
			id := uuid.New()
			if e.ceid != nil {
				id = analytics.ClientEventID(player, *e.ceid)
			}
			r := port.RawEvent{EventID: id, OccurredAt: e.at, TenantID: tenant, SchoolID: dims.SchoolID, GradeLevel: dims.GradeLevel,
				Platform: trunc(b.Platform, 16), Locale: trunc(b.Locale, 8), DeviceTier: trunc(b.DeviceTier, 8),
				ClientVersion: trunc(b.ClientVersion, 32), PlayerKey: player, KeyVersion: key.Version, EventName: e.typ,
				Source: "client", Props: props}
			if dims.SessionID != nil {
				r.SessionKey = analytics.Pseudonym(key, *dims.SessionID)
			}
			if err := h.late(ctx, tenant, e.at, now); err != nil {
				if errors.Is(err, errLate) {
					continue
				}
				return err
			}
			rows = append(rows, r)
			h.metric(func(m port.AnalyticsMetrics) { m.Telemetry(e.typ, true, "") })
		}
		n, err := h.Store.InsertRaw(ctx, rows)
		accepted = n
		h.metric(func(m port.AnalyticsMetrics) { m.RawInserted(n) })
		return err
	})
	if err == nil {
		// Client metrics of MONITORING §3 from the valid events of a stored batch (whatever the analytics drop reason): a
		// failed batch is resent by the client and must not count twice. Values are statistics only (ADR-0016).
		for _, e := range events {
			if cm := analytics.MetricOf(e.typ, e.attrs); cm.Kind != "" {
				h.metric(func(m port.AnalyticsMetrics) { m.Client(cm, b.Platform, b.ClientVersion) })
			}
		}
	}
	return accepted, err
}

var errLate = errors.New("analytics: event later than analytics_late_event_window")

// late applies DASHBOARD_DATA §5 to an event of a tenant day (F080-AC-09): a closed day within the window is marked for
// reclose; later events are dropped (metric eduworld_rollup_late_events_total).
func (h AnalyticsIngest) late(ctx context.Context, tenant uuid.UUID, at, now time.Time) error {
	day := analytics.DayOf(at)
	final, err := h.Store.DayFinal(ctx, tenant, day)
	if err != nil {
		return err
	}
	switch analytics.Late(at, now, h.Config.LateWindow, final) {
	case analytics.Drop:
		h.metric(func(m port.AnalyticsMetrics) { m.LateEvent() })
		return errLate
	case analytics.ApplyReclose:
		return h.Store.MarkReclose(ctx, tenant, day)
	}
	return nil
}

// DomainEvent is one verified domain event for the analytics consumer (envelope fields used).
type DomainEvent struct {
	EventID    uuid.UUID
	EventType  string
	OccurredAt time.Time
	TenantID   uuid.UUID
	Payload    map[string]any
}

// IngestDomainEvent maps a domain event of the catalog to analytics events (F080-AC-05): player_key/session_key are
// HMAC-SHA256 with the secret key, dimensions come from the projections (ref.students, game session), props are the
// catalog keys only; rows are idempotent per event id (one row per student for multi-student events). QA tenants and
// late events are dropped with a metric. Unknown events and students are ignored (metric), never an error.
func (h AnalyticsIngest) IngestDomainEvent(ctx context.Context, e DomainEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.IngestAnalyticsEvent")
	defer func() { done(err) }()
	if len(h.Keys.All()) == 0 {
		return errors.New("analytics: no player key configured")
	}
	mapped := analytics.Map(e.EventType, e.Payload, h.Config.LateNight)
	if len(mapped) == 0 {
		h.metric(func(m port.AnalyticsMetrics) { m.Dropped("not_mapped") })
		return nil
	}
	at := e.OccurredAt.UTC()
	if at.IsZero() {
		at = h.now()
	}
	key := h.Keys.Current()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		flags, err := h.Store.TenantFlags(ctx, e.TenantID)
		if err != nil {
			return err
		}
		if flags.QA {
			h.metric(func(m port.AnalyticsMetrics) { m.Dropped("qa_tenant") })
			return nil
		}
		if err := h.late(ctx, e.TenantID, at, h.now()); err != nil {
			if errors.Is(err, errLate) {
				return nil
			}
			return err
		}
		rows := make([]port.RawEvent, 0, len(mapped))
		for _, m := range mapped {
			if m.Tenant != nil && *m.Tenant != e.TenantID {
				h.metric(func(mm port.AnalyticsMetrics) { mm.Dropped("other_tenant") })
				continue
			}
			r := port.RawEvent{OccurredAt: at, TenantID: e.TenantID, KeyVersion: key.Version, EventName: m.EventName, Source: "server",
				Props: m.Props, Platform: trunc(m.Platform, 16), ClientVersion: trunc(m.ClientVersion, 32)}
			r.EventID = e.EventID
			if m.Student != nil {
				if len(mapped) > 1 {
					r.EventID = analytics.StudentEventID(e.EventID, *m.Student)
				}
				dims, err := h.Store.StudentDims(ctx, e.TenantID, *m.Student)
				if err != nil {
					return err
				}
				if !dims.Known {
					h.metric(func(mm port.AnalyticsMetrics) { mm.Dropped("unknown_student") })
					continue
				}
				r.PlayerKey = analytics.Pseudonym(key, *m.Student)
				r.SchoolID, r.GradeLevel = dims.SchoolID, dims.GradeLevel
				if r.Platform == "" {
					r.Platform = trunc(dims.Platform, 16)
				}
				if r.ClientVersion == "" {
					r.ClientVersion = trunc(dims.ClientVersion, 32)
				}
				sess := m.SessionID
				if sess == nil {
					sess = dims.SessionID
				}
				if sess != nil {
					r.SessionKey = analytics.Pseudonym(key, *sess)
				}
			}
			if m.EventName == "match_result" {
				if s, ok := e.Payload["duration_s"].(float64); ok {
					h.metric(func(mm port.AnalyticsMetrics) { mm.MatchDuration(e.Payload["mode"].(string), s) })
				}
			}
			rows = append(rows, r)
		}
		if e.EventType == "match.completed" {
			h.matchDuration(ctx, e)
		}
		n, err := h.Store.InsertRaw(ctx, rows)
		h.metric(func(m port.AnalyticsMetrics) { m.RawInserted(n) })
		return err
	})
}

// matchDuration observes eduworld_match_duration_seconds{mode} (FR-OPS-003, F080-AC-18) from the match_start row of the
// same match (match.completed carries no start time); best effort.
func (h AnalyticsIngest) matchDuration(ctx context.Context, e DomainEvent) {
	id, _ := e.Payload["match_id"].(string)
	mode, _ := e.Payload["mode"].(string)
	end, perr := time.Parse(time.RFC3339Nano, func() string { s, _ := e.Payload["ended_at"].(string); return s }())
	if id == "" || perr != nil {
		return
	}
	start, ok, err := h.Store.MatchStart(ctx, e.TenantID, id, end)
	if err != nil || !ok || end.Before(start) {
		return
	}
	h.metric(func(m port.AnalyticsMetrics) { m.MatchDuration(mode, end.Sub(start).Seconds()) })
}

// AnonymizeStudent pseudonymises nothing new: it computes the player_key of the student under every key version and sets
// player_key/session_key to NULL in events_raw and player_days (F080-AC-07, GAME_ANALYTICS §9); aggregate tables carry no
// identity and stay unchanged; nothing is deleted (ADR-0070, R21). Runs in the transaction of ctx when there is one.
func (h AnalyticsIngest) AnonymizeStudent(ctx context.Context, student uuid.UUID) (raw, days int64, err error) {
	keys := make([]string, 0, len(h.Keys.All()))
	for _, k := range h.Keys.All() {
		keys = append(keys, analytics.Pseudonym(k, student))
	}
	if len(keys) == 0 {
		return 0, 0, nil
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		raw, days, err = h.Store.Anonymize(ctx, keys)
		return err
	})
	return raw, days, err
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
