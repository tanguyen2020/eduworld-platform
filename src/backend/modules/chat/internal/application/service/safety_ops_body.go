package service

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// ShiftBody renders OncallShift (F124).
func ShiftBody(s read.OncallShiftRow) map[string]any {
	m := map[string]any{"shift_id": s.ID, "moderator_user_id": s.Moderator, "oncall_role": s.Role, "starts_at": s.Start, "ends_at": s.End,
		"created_at": s.CreatedAt, "version": s.Version}
	setOpt(m, "note", s.Note)
	setOpt(m, "created_by", s.CreatedBy)
	return m
}

// GrantBody renders BreakGlassGrant (F124): never message content.
func GrantBody(g read.GrantRow) map[string]any {
	m := map[string]any{"grant_id": g.ID, "tenant_id": g.TenantID, "scope_type": g.ScopeType, "scope_id": g.ScopeID,
		"window_from": g.WindowFrom, "window_to": g.WindowTo, "duration_minutes": g.DurationMinutes, "reason": g.Reason,
		"status": g.Status, "requested_by": g.RequestedBy, "requested_at": g.RequestedAt, "messages_viewed_count": g.MessagesViewedCount,
		"version": g.Version}
	setOpt(m, "decided_by", g.DecidedBy)
	setOpt(m, "decided_at", g.DecidedAt)
	setOpt(m, "decision_note", g.DecisionNote)
	setOpt(m, "expires_at", g.ExpiresAt)
	setOpt(m, "revoked_by", g.RevokedBy)
	setOpt(m, "revoked_at", g.RevokedAt)
	setOpt(m, "revoke_reason", g.RevokeReason)
	return m
}

// BreakGlassMessageBody renders BreakGlassMessage (original content: the requester of an active grant only).
func BreakGlassMessageBody(m read.BreakGlassMessage) map[string]any {
	out := map[string]any{"message_id": m.MessageID, "channel_id": m.ChannelID, "sender_id": m.SenderID, "kind": m.Kind,
		"revision": max(m.Revision, 1), "moderation_state": m.ModerationState, "created_at": m.CreatedAt}
	if m.Seq > 0 {
		out["seq"] = m.Seq
	}
	setOpt(out, "sender_school_id", m.SenderSchoolID)
	setOpt(out, "body", m.Body)
	setOpt(out, "body_display", m.BodyDisplay)
	setOpt(out, "quick_message_id", m.QuickMessageID)
	setOpt(out, "recalled_at", m.RecalledAt)
	return out
}
