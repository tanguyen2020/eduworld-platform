package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReportInput is ReportCreate (createReport).
type ReportInput struct {
	TargetType string
	TargetID   uuid.UUID
	Reason     string
	Note       *string
	ChannelID  *uuid.UUID
	MatchID    *uuid.UUID
	PartyID    *uuid.UUID
	// ReportedField is context.reported_field ("nickname", F136-AC-18): only with target_type user.
	ReportedField string
	Lang          string // Accept-Language of the thank-you
}

// ReportedNickname is the only value of context.reported_field.
const ReportedNickname = "nickname"

// ReportResult is the result of createReport.
type ReportResult struct {
	ReportID uuid.UUID
	CaseID   uuid.UUID
}

// Signal is a pipeline or analysis signal about a stored message (chat.message_held, async analysis).
type Signal struct {
	EventID     uuid.UUID
	TenantID    uuid.UUID
	MessageID   uuid.UUID
	ChannelID   uuid.UUID
	ChannelType string
	SenderID    uuid.UUID
	Decision    string
	Reasons     []string
	Async       bool
	At          time.Time
}

// snapshot is the evidence copy of a report or signal (moderation_evidence.snapshot): the reported message id, the
// ±10 messages of its channel with their original content and revision state, and the capture time.
type snapshot struct {
	Kind      string             `json:"kind"`
	TargetID  uuid.UUID          `json:"target_id"`
	ChannelID *uuid.UUID         `json:"channel_id,omitempty"`
	Context   map[string]string  `json:"context,omitempty"`
	Messages  []read.CaseMessage `json:"messages"`
	// ReportedNickname is the nickname of the reported student read on the server at the time of a nickname report
	// (ref.player_profiles, F136-AC-18) — never a value sent by the client (R5).
	ReportedNickname *string   `json:"reported_nickname,omitempty"`
	TakenAt          time.Time `json:"taken_at"`
}

func (h Safety) evidence(ctx context.Context, c *read.CaseRow, id uuid.UUID, report, school *uuid.UUID, s snapshot) (*uuid.UUID, error) {
	if len(s.Messages) == 0 && s.ReportedNickname == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if err := h.Store.InsertEvidence(ctx, write.NewEvidence{ID: id, TenantID: c.TenantID, CaseID: c.ID, ReportID: report, SchoolID: school,
		ChannelID: s.ChannelID, Snapshot: b, SHA256: hex.EncodeToString(sum[:]), At: s.TakenAt}); err != nil {
		return nil, err
	}
	return &id, nil
}

func addID(list []uuid.UUID, id *uuid.UUID) []uuid.UUID {
	if id == nil || *id == uuid.Nil || slices.Contains(list, *id) || len(list) >= safety.MaxStudents {
		return list
	}
	return append(list, *id)
}

// CreateReport implements createReport (F121-AC-01, AC-02, AC-05, FR-MOD-002, MODERATION §5): a student reports a
// message of a channel they are (or were) a member of, or a student of their tenant; the report keeps a ±10 snapshot
// of the channel (original content), joins the open/in_review case of the same target created within 24 hours (one
// count per reporter) or opens a case of the tenant with its schools, priority and SLA; a p1 case holds the reported
// student at once (F121-AC-04). Unknown targets, channels the reporter never belonged to and other tenants are 404.
func (h Safety) CreateReport(ctx context.Context, in ReportInput) (res ReportResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateReport")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	if !slices.Contains(safety.Reasons, in.Reason) {
		return res, problem.Invalid("reason", problem.SubInvalid, "unknown reason")
	}
	if in.TargetType != safety.TargetMessage && in.TargetType != safety.TargetUser {
		return res, problem.Invalid("target_type", problem.SubInvalid, "message or user")
	}
	if in.ReportedField != "" && (in.ReportedField != ReportedNickname || in.TargetType != safety.TargetUser) {
		return res, problem.Invalid("context.reported_field", problem.SubInvalid, "nickname of a user report only")
	}
	now := h.now()
	var held []write.SafetySanction
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		reporter, err := h.Store.Student(ctx, tenant, me)
		if err != nil {
			return err
		}
		if reporter == nil || reporter.Deleted {
			return problem.New(problem.Unavailable, "student not projected yet").Retry()
		}
		var targetUser uuid.UUID
		var channel *uuid.UUID
		channelType := ""
		snap := snapshot{Kind: "report", TargetID: in.TargetID, TakenAt: now, Context: reportContext(in)}
		switch in.TargetType {
		case safety.TargetMessage:
			m, err := h.Store.Message(ctx, tenant, in.TargetID)
			if err != nil {
				return err
			}
			if m == nil {
				return notFound()
			}
			member, err := h.Store.Member(ctx, tenant, m.ChannelID, me, m.CreatedAt)
			if err != nil {
				return err
			}
			if !member {
				return notFound()
			}
			if m.SenderID == me {
				return self("target_id")
			}
			targetUser, channel, channelType = m.SenderID, &m.ChannelID, m.ChannelType
			if snap.Messages, err = h.Store.Window(ctx, tenant, m.ChannelID, m.Seq, safety.ContextBefore, safety.ContextAfter); err != nil {
				return err
			}
		default:
			if in.TargetID == me {
				return self("target_id")
			}
			st, err := h.Store.Student(ctx, tenant, in.TargetID)
			if err != nil {
				return err
			}
			if st == nil || st.Deleted {
				return notFound()
			}
			targetUser = in.TargetID
			if in.ReportedField == ReportedNickname {
				if snap.ReportedNickname, err = h.Store.PlayerNickname(ctx, tenant, in.TargetID); err != nil {
					return err
				}
			}
			if in.ChannelID != nil {
				member, err := h.Store.Member(ctx, tenant, *in.ChannelID, me, now)
				if err != nil {
					return err
				}
				if !member {
					return notFound()
				}
				if channelType, err = h.Store.ChannelType(ctx, tenant, *in.ChannelID); err != nil {
					return err
				}
				channel = in.ChannelID
				if snap.Messages, err = h.Store.Window(ctx, tenant, *in.ChannelID, 0,
					safety.ContextBefore+safety.ContextAfter+1, 0); err != nil {
					return err
				}
			}
		}
		snap.ChannelID = channel
		severity := safety.SeverityOfReport(in.Reason)
		weight := 1.0
		dismissed, err := h.Store.DismissedReports(ctx, tenant, me, now.Add(-safety.AbuseWindow))
		if err != nil {
			return err
		}
		if dismissed >= safety.AbuseThreshold {
			weight = safety.AbuseWeight // never a sanction of the reporter (MODERATION §5)
		}
		c, err := h.Store.OpenCase(ctx, tenant, in.TargetType, in.TargetID, now.Add(-safety.MergeWindow))
		if err != nil {
			return err
		}
		merged := c != nil
		school := reporter.SchoolID
		if !merged {
			c, err = h.newCase(ctx, tenant, severity, safety.SourceReport, in.Reason, in.TargetType, in.TargetID, channel, channelType, now)
			if err != nil {
				return err
			}
			if err := h.Store.InsertCase(ctx, *c); err != nil {
				return err
			}
		}
		reportID := ids.New()
		var evidence *uuid.UUID
		if len(snap.Messages) > 0 || snap.ReportedNickname != nil {
			eid := ids.New()
			evidence = &eid
		}
		id, inserted, err := h.Store.InsertReport(ctx, write.NewReport{ID: reportID, TenantID: tenant, SchoolID: &school, ReporterID: me,
			TargetType: in.TargetType, TargetID: in.TargetID, TargetUserID: &targetUser, Reason: in.Reason, Note: in.Note,
			Context: snap.Context, CaseID: c.ID, Weight: weight, EvidenceID: evidence, CreatedAt: now})
		if err != nil {
			return err
		}
		res = ReportResult{ReportID: id, CaseID: c.ID}
		if err := idempotency.SetResult(ctx, service.ReportBody(id, in.Lang)); err != nil {
			return err
		}
		if !inserted {
			return nil // the reporter already reported this target in this case: counted once (MODERATION §5)
		}
		if evidence != nil {
			if _, err := h.evidence(ctx, c, *evidence, &id, &school, snap); err != nil {
				return err
			}
		}
		c.ReportCount++
		c.SchoolIDs = addID(c.SchoolIDs, &school)
		c.Subjects = addID(c.Subjects, &targetUser)
		if st, err := h.Store.Student(ctx, tenant, targetUser); err == nil && st != nil {
			c.SchoolIDs = addID(c.SchoolIDs, &st.SchoolID)
		}
		if merged {
			if up := safety.Higher(c.Severity, severity); up != c.Severity {
				c.Severity = up
				fr, due := safety.Deadlines(up, now)
				c.FirstResponseDue, c.SLADue = minTime(c.FirstResponseDue, fr), minTime(c.SLADue, due)
			}
		}
		if err := h.rotate(ctx, c, now); err != nil { // F124-AC-13: on-call rotation of a p1 case
			return err
		}
		c.Version++
		if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
			return err
		}
		if err := h.action(ctx, c, safety.ActionReport, nil, "system", nil, nil, nil, now); err != nil {
			return err
		}
		if c.Severity == safety.P1 {
			if held, err = h.hold(ctx, c, now); err != nil {
				return err
			}
		}
		for i := range held {
			if err := h.later(ctx, held[i].UserID, &held[i]); err != nil {
				return err
			}
		}
		if err := h.publish(ctx, EventReportCreated, "chat_report", id, tenant, c.ID.String(), now, map[string]any{
			"tenant_id": tenant, "report_id": id, "case_id": c.ID, "target_type": in.TargetType, "target_id": in.TargetID,
			"reason": in.Reason, "severity": c.Severity, "merged": merged, "created_at": now}); err != nil {
			return err
		}
		return h.record(ctx, AuditReportCreated, "moderation_case", c.ID, &school, map[string]any{"report_id": id.String(),
			"reason": in.Reason, "severity": c.Severity, "merged": merged})
	})
	if err != nil {
		return res, err
	}
	if h.Metrics != nil {
		h.Metrics.Report(in.TargetType, in.Reason)
	}
	return res, nil
}

func reportContext(in ReportInput) map[string]string {
	m := map[string]string{}
	for k, v := range map[string]*uuid.UUID{"channel_id": in.ChannelID, "match_id": in.MatchID, "party_id": in.PartyID} {
		if v != nil {
			m[k] = v.String()
		}
	}
	if in.ReportedField != "" {
		m["reported_field"] = in.ReportedField
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// newCase builds a case of tenant: priority, report_sla deadlines and the contact path (tenant common: platform_direct,
// MODERATION §6.4).
func (h Safety) newCase(ctx context.Context, tenant uuid.UUID, severity, source, category, targetType string, target uuid.UUID,
	channel *uuid.UUID, channelType string, now time.Time) (*read.CaseRow, error) {
	fr, res := safety.Deadlines(severity, now)
	c := &read.CaseRow{ID: ids.New(), TenantID: tenant, Status: safety.StatusOpen, Severity: severity, Source: source,
		TargetType: targetType, TargetID: target, ChannelID: channel, SchoolIDs: []uuid.UUID{}, Subjects: []uuid.UUID{},
		FirstResponseDue: fr, SLADue: res, ContactPath: safety.PathSchool, CreatedAt: now, UpdatedAt: now, Version: 1}
	if category != "" {
		cat := category
		c.Category = &cat
	}
	if channelType != "" {
		ct := channelType
		c.ChannelType = &ct
	}
	typ, err := h.Store.TenantType(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if typ == "common" {
		c.ContactPath = safety.PathPlatformDirect
	}
	return c, nil
}

// hold restricts the reported students of a p1 case at once (F121-AC-04, MODERATION §6.3): a system mute of every channel
// (reason_code case_hold, no end) that also blocks new friend requests; an active hold without case (block_and_alert of
// the pipeline, F116) is linked to the case instead of a second one. Released when the case resolves.
func (h Safety) hold(ctx context.Context, c *read.CaseRow, now time.Time) ([]write.SafetySanction, error) {
	var out []write.SafetySanction
	for _, user := range c.Subjects {
		active, err := h.Store.ActiveHold(ctx, c.TenantID, user)
		if err != nil {
			return nil, err
		}
		if active != nil {
			if active.CaseID == nil {
				if err := h.Store.LinkSanction(ctx, c.TenantID, active.ID, c.ID); err != nil {
					return nil, err
				}
			}
			if c.HoldSanctionID == nil && (active.CaseID == nil || *active.CaseID == c.ID) {
				id := active.ID
				c.HoldSanctionID = &id
			}
			continue
		}
		st, err := h.Store.Student(ctx, c.TenantID, user)
		if err != nil {
			return nil, err
		}
		var school *uuid.UUID
		if st != nil {
			school = &st.SchoolID
		}
		caseID := c.ID
		s := write.SafetySanction{ID: ids.New(), TenantID: c.TenantID, SchoolID: school, UserID: user, Type: safety.SanctionMute,
			Scope: "all", Reason: reasonCaseHold, ReasonCode: safety.CodeCaseHold, CaseID: &caseID, IssuedByKind: "system", StartsAt: now}
		if err := h.applySanction(ctx, s); err != nil {
			return nil, err
		}
		if c.HoldSanctionID == nil {
			id := s.ID
			c.HoldSanctionID = &id
		}
		sid := s.ID
		if err := h.action(ctx, c, safety.ActionHold, nil, "system", nil, &sid, nil, now); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if c.HoldSanctionID != nil {
		c.Version++
		if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// OpenCaseFromSignal opens (or joins) the case of a pipeline signal (F121-AC-03, MODERATION §3–§4): chat.message_held
// (hold_for_review, block_and_alert) and the async analysis/self-harm signals; idempotent by event id. Source and
// priority follow the reasons (self-harm → self_harm_signal p1); a p1 case holds the sender (linking the block_and_alert
// mute of F116). Never a long sanction (CHILD_SAFETY §9).
func (h Safety) OpenCaseFromSignal(ctx context.Context, s Signal) (caseID uuid.UUID, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.OpenCaseFromSignal")
	defer func() { done(err) }()
	sc, err := authz.EventScope(s.TenantID)
	if err != nil {
		return uuid.Nil, err
	}
	now := h.now()
	if s.At.IsZero() {
		s.At = now
	}
	var held []write.SafetySanction
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if s.EventID != uuid.Nil {
			existing, err := h.Store.CaseByEvent(ctx, s.TenantID, s.EventID)
			if err != nil {
				return err
			}
			if existing != nil {
				caseID = existing.ID
				return nil
			}
		}
		severity, source, category := safety.Classify(safety.Signal{Decision: s.Decision, Reasons: s.Reasons, Async: s.Async})
		snap := snapshot{Kind: "signal", TargetID: s.MessageID, TakenAt: now}
		m, err := h.Store.Message(ctx, s.TenantID, s.MessageID)
		if err != nil {
			return err
		}
		channel, channelType, sender := s.ChannelID, s.ChannelType, s.SenderID
		var school *uuid.UUID
		if m != nil {
			channel, channelType, sender, school = m.ChannelID, m.ChannelType, m.SenderID, m.SchoolID
			if snap.Messages, err = h.Store.Window(ctx, s.TenantID, m.ChannelID, m.Seq, safety.ContextBefore, safety.ContextAfter); err != nil {
				return err
			}
		}
		snap.ChannelID = &channel
		c, err := h.Store.OpenCase(ctx, s.TenantID, safety.TargetMessage, s.MessageID, now.Add(-safety.MergeWindow))
		if err != nil {
			return err
		}
		merged := c != nil
		if !merged {
			if c, err = h.newCase(ctx, s.TenantID, severity, source, category, safety.TargetMessage, s.MessageID, &channel, channelType, now); err != nil {
				return err
			}
			ev := s.EventID
			if ev != uuid.Nil {
				c.SourceEventID = &ev
			}
		} else if up := safety.Higher(c.Severity, severity); up != c.Severity {
			c.Severity = up
			fr, res := safety.Deadlines(up, now)
			c.FirstResponseDue, c.SLADue = minTime(c.FirstResponseDue, fr), minTime(c.SLADue, res)
		}
		c.Subjects = addID(c.Subjects, &sender)
		if school == nil {
			if st, err := h.Store.Student(ctx, s.TenantID, sender); err == nil && st != nil {
				school = &st.SchoolID
			}
		}
		c.SchoolIDs = addID(c.SchoolIDs, school)
		if merged {
			c.Version++
			if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
				return err
			}
		} else if err := h.Store.InsertCase(ctx, *c); err != nil {
			return err
		}
		// F124-AC-13: on-call rotation of a p1 case (after the row exists: the history references it).
		if paged, err := h.rotated(ctx, c, now); err != nil {
			return err
		} else if paged {
			c.Version++
			if err := h.Store.UpdateCase(ctx, *c, nil); err != nil {
				return err
			}
		}
		if _, err := h.evidence(ctx, c, ids.New(), nil, school, snap); err != nil {
			return err
		}
		if err := h.action(ctx, c, safety.ActionSignal, nil, "system", nil, nil, &s.MessageID, now); err != nil {
			return err
		}
		switch {
		case c.Source == safety.SourceSelfHarm:
			// A child at risk is supported, never sanctioned for a self-harm signal (CHILD_SAFETY §8, F121-AC-24): the
			// block_and_alert mute of the pipeline is lifted; the message itself stays held for the Safety Team.
			if err := h.noPenalty(ctx, c, sender, now); err != nil {
				return err
			}
		case c.Severity == safety.P1:
			if held, err = h.hold(ctx, c, now); err != nil {
				return err
			}
		}
		for i := range held {
			if err := h.later(ctx, held[i].UserID, &held[i]); err != nil {
				return err
			}
		}
		caseID = c.ID
		return h.record(ctx, AuditCaseOpened, "moderation_case", c.ID, school, map[string]any{"source": c.Source,
			"severity": c.Severity, "message_id": s.MessageID.String(), "merged": merged})
	})
	if err != nil {
		return uuid.Nil, err
	}
	return caseID, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// noPenalty lifts the temporary hold of the sender of a self-harm signal (block_and_alert of the pipeline) and records
// it in the case history.
func (h Safety) noPenalty(ctx context.Context, c *read.CaseRow, user uuid.UUID, now time.Time) error {
	active, err := h.Store.ActiveHold(ctx, c.TenantID, user)
	if err != nil || active == nil {
		return err
	}
	if active.CaseID != nil && *active.CaseID != c.ID {
		return nil // the hold of another case stays
	}
	if err := h.liftSanction(ctx, *active, nil, reasonSelfHarmSupport, liftRevoked, now); err != nil {
		return err
	}
	sid := active.ID
	if err := h.action(ctx, c, safety.ActionReleaseHold, nil, "system", nil, &sid, nil, now); err != nil {
		return err
	}
	return h.later(ctx, user, nil)
}
