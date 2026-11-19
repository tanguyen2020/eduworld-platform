package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Audit actions of the game session.
const (
	AuditAccessDenied    = "game_access_denied_non_student"
	AuditSessionReplaced = "game_session.replaced"
)

// StartInput is the body of createGameSession plus the headers and the hashed client address.
type StartInput struct {
	DeviceID      uuid.UUID
	Platform      string
	ClientVersion string
	DeviceModel   *string
	Takeover      bool
	IPHash        []byte
}

// StartResult is the GameSession of the response.
type StartResult struct {
	GameSessionID     uuid.UUID
	StartedAt         time.Time
	LoginAt           time.Time
	ReplacedSessionID *uuid.UUID
}

// Body is the GameSession of the 201 response (also stored for Idempotency-Key replays).
func (r StartResult) Body() map[string]any {
	return map[string]any{"game_session_id": r.GameSessionID, "started_at": r.StartedAt, "login_at": r.LoginAt,
		"replaced_session_id": r.ReplacedSessionID}
}

// startOutcome is the metric result of a start.
func startOutcome(err error, takeover bool) string {
	var pe *problem.Error
	switch {
	case err == nil && takeover:
		return port.StartTakeover
	case err == nil:
		return port.StartCreated
	case !errors.As(err, &pe):
		return port.StartError
	}
	switch pe.Code {
	case problem.ActiveSessionOnOtherDevice:
		return port.StartConflict
	case problem.GameAccessStudentOnly:
		return port.StartDenied
	case problem.CurfewActive:
		return port.StartCurfew
	case problem.ClientVersionUnsupported:
		return port.StartUnsupported
	case problem.Maintenance:
		return port.StartMaintenance
	case problem.Unavailable:
		return port.StartNotProjected
	}
	return port.StartError
}

// Start implements createGameSession (F014 US1, DATABASE_DESIGN §3.12): one transaction serialized per student
// by an advisory lock; the one-device rule decides 409 / takeover / timeout / same device; the session, the login
// event and the outbox events are written together. module identity is never called (the Principal is resolved by
// the httpx chain with its cache, R30).
func (h Sessions) Start(ctx context.Context, in StartInput) (res StartResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartGameSession")
	started := time.Now()
	replaced := false
	defer func() {
		done(err)
		if h.Metrics != nil {
			h.Metrics.Started(startOutcome(err, replaced), time.Since(started))
		}
	}()
	p, err := principal(ctx)
	if err != nil {
		return res, err
	}
	if p.UserType != authz.UserTypeStudent || p.Channel != authz.ChannelGame || p.TenantID == nil {
		return res, h.deny(ctx, p, "not_student")
	}
	if !gamesession.ValidPlatform(in.Platform) || in.DeviceID == uuid.Nil {
		return res, problem.Invalid("client_platform", problem.SubInvalid, "unknown platform or device")
	}
	if m := h.Config.Maintenance; m.Active {
		e := problem.New(problem.Maintenance, m.Message).Retry()
		if m.Until != nil {
			e = e.With("until", m.Until.UTC())
		}
		return res, e
	}
	if !gamesession.Supported(in.ClientVersion, h.Config.MinClientVersion) {
		return res, problem.New(problem.ClientVersionUnsupported, "").With("min_client_version", h.Config.MinClientVersion)
	}
	tenant, student := *p.TenantID, p.UserID
	var denied string
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		st, err := h.Directory.Student(ctx, tenant, student)
		if err != nil {
			return err
		}
		if st == nil {
			return problem.New(problem.Unavailable, "student not projected yet").Retry()
		}
		if !st.Playable() {
			denied = "student_not_active"
			return problem.New(problem.GameAccessStudentOnly, "")
		}
		now := h.now()
		cf, err := h.curfew(ctx, tenant, st.SchoolID)
		if err != nil {
			return err
		}
		if active, reopen := cf.Active(now); active {
			return problem.New(problem.CurfewActive, "").With("reopen_at", reopen.UTC())
		}
		if err := h.Sessions.LockStudent(ctx, student); err != nil {
			return err
		}
		old, err := h.Sessions.Active(ctx, tenant, student)
		if err != nil {
			return err
		}
		decision := gamesession.Decide(old, in.DeviceID, in.Takeover, now, h.Config.HeartbeatTimeout)
		if decision == gamesession.Conflict {
			return conflict(old)
		}
		school := st.SchoolID
		s := &gamesession.Session{ID: ids.New(), TenantID: tenant, StudentID: student, SchoolID: &school, DeviceID: in.DeviceID,
			DeviceModel: in.DeviceModel, Platform: in.Platform, ClientVersion: in.ClientVersion, KeycloakSID: p.SessionID,
			StartedAt: now, LastHeartbeatAt: now}
		if decision != gamesession.NoActive {
			var by *uuid.UUID
			if decision == gamesession.TakeOver {
				by = &s.ID
			}
			if _, err := h.endSession(ctx, old, decision.EndReason(), now, by); err != nil {
				return err
			}
		}
		if err := h.Sessions.Create(ctx, s); err != nil {
			return err
		}
		login := write.LoginEvent{ID: ids.New(), TenantID: tenant, StudentID: student, GameSessionID: s.ID, DeviceID: in.DeviceID,
			Platform: in.Platform, DeviceModel: in.DeviceModel, ClientVersion: in.ClientVersion, IPHash: in.IPHash, LoginAt: now}
		if err := h.Logins.Create(ctx, login); err != nil {
			return err
		}
		if err := h.started(ctx, s, login, old, decision); err != nil {
			return err
		}
		res = StartResult{GameSessionID: s.ID, StartedAt: now, LoginAt: now}
		if decision == gamesession.TakeOver {
			replaced = true
			oldID := old.ID
			res.ReplacedSessionID = &oldID
		}
		h.afterCommit(ctx, func(context.Context) {
			if h.Metrics != nil {
				h.Metrics.Login(tenant.String(), in.Platform)
				if replaced {
					h.Metrics.Takeover(in.Platform)
				}
			}
		})
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		if decision == gamesession.TakeOver {
			oldID := old.ID
			h.afterCommit(ctx, func(ctx context.Context) {
				if h.Realtime != nil {
					_ = h.Realtime.SessionReplaced(ctx, student, oldID, in.Platform, now)
				}
			})
		}
		return nil
	})
	if denied != "" {
		return res, h.deny(ctx, p, denied)
	}
	return res, err
}

func conflict(old *gamesession.Session) error {
	other := map[string]any{"platform": old.Platform, "last_seen_at": old.LastHeartbeatAt.UTC()}
	if l := gamesession.DeviceLabel(old.DeviceModel); l != nil {
		other["device_label"] = *l
	}
	return problem.New(problem.ActiveSessionOnOtherDevice, "").With("other_device", other)
}

// started writes the outbox events of a new session: game_session.started and student.game_logged_in, plus
// game_session.replaced and its audit on takeover (F014-AC-04, AC-09).
func (h Sessions) started(ctx context.Context, s *gamesession.Session, login write.LoginEvent, old *gamesession.Session, d gamesession.Decision) error {
	tenant, id, student := s.TenantID, s.ID.String(), s.StudentID.String()
	payload := map[string]any{"game_session_id": id, "student_id": student, "platform": s.Platform, "client_version": s.ClientVersion,
		"started_at": s.StartedAt}
	if d == gamesession.TakeOver {
		payload["replaced_session_id"] = old.ID.String()
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "game_session.started", SubjectType: "game_session", SubjectID: id,
		PartitionKey: student, TenantID: &tenant, OccurredAt: s.StartedAt, Payload: payload}); err != nil {
		return err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "student.game_logged_in", SubjectType: "student", SubjectID: student,
		PartitionKey: student, TenantID: &tenant, OccurredAt: login.LoginAt, Payload: map[string]any{
			"student_id": student, "tenant_id": tenant.String(), "game_session_id": id, "login_event_id": login.ID.String(),
			"platform": s.Platform, "login_at": login.LoginAt}}); err != nil {
		return err
	}
	if d != gamesession.TakeOver {
		return nil
	}
	sid := old.KeycloakSID
	if sid == "" {
		sid = "-" // the old token had no sid: identity revokes by device only
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "game_session.replaced", SubjectType: "game_session", SubjectID: old.ID.String(),
		PartitionKey: student, TenantID: &tenant, OccurredAt: s.StartedAt, Payload: map[string]any{
			"old_game_session_id": old.ID.String(), "new_game_session_id": id, "student_id": student, "new_platform": s.Platform,
			"old_keycloak_sid": sid, "old_device_id": old.DeviceID.String()}}); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: AuditSessionReplaced, TargetType: "game_session", TargetID: old.ID.String(),
		Result: audit.ResultSuccess, Reason: gamesession.ReasonReplaced, Metadata: map[string]any{"new_game_session_id": id,
			"new_platform": s.Platform}})
}

// deny records the refusal of a non-student or inactive student (F014-AC-02, R23) and returns 403
// GAME_ACCESS_STUDENT_ONLY. No session and no login event is written.
func (h Sessions) deny(ctx context.Context, p authz.Principal, why string) error {
	if h.Metrics != nil {
		h.Metrics.Denied("not_student")
	}
	_ = h.Audit.Record(ctx, audit.Event{Action: AuditAccessDenied, TargetType: "user", TargetID: p.UserID.String(),
		Result: audit.ResultDenied, Reason: string(problem.GameAccessStudentOnly), Metadata: map[string]any{"why": why,
			"user_type": string(p.UserType)}})
	return problem.New(problem.GameAccessStudentOnly, "")
}
