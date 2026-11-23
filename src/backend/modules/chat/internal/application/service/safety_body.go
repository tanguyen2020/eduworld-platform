package service

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Rendering of the Safety Team resources (F121): the body written by the presentation and stored as the idempotent
// response (idempotency.SetResult) of the writes. Ids, codes and times only; message content appears only in
// CaseMessageBody (assignee or team lead, audited).

func setOpt[T any](m map[string]any, key string, v *T) {
	if v != nil {
		m[key] = *v
	}
}

// CaseBody renders ModerationCase (metadata, never content).
func CaseBody(c read.CaseRow) map[string]any {
	m := map[string]any{"case_id": c.ID, "tenant_id": c.TenantID, "status": c.Status, "severity": c.Severity, "source": c.Source,
		"target_type": c.TargetType, "target_id": c.TargetID, "school_ids": c.SchoolIDs, "report_count": c.ReportCount,
		"first_response_due_at": c.FirstResponseDue, "sla_due_at": c.SLADue, "contact_path": c.ContactPath, "version": c.Version,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt}
	setOpt(m, "tenant_name", c.TenantName)
	setOpt(m, "tenant_type", c.TenantType)
	setOpt(m, "category_code", c.Category)
	setOpt(m, "channel_id", c.ChannelID)
	setOpt(m, "channel_type", c.ChannelType)
	setOpt(m, "assignee_moderator_id", c.Assignee)
	setOpt(m, "school_notified_at", c.SchoolNotifiedAt)
	setOpt(m, "school_ack_at", c.SchoolAckAt)
	setOpt(m, "contact_outcome", c.ContactOutcome)
	setOpt(m, "resolved_at", c.ResolvedAt)
	return m
}

// ActionsBody renders the history of a case.
func ActionsBody(list []read.ActionRow) []map[string]any {
	out := make([]map[string]any, len(list))
	for i, a := range list {
		m := map[string]any{"action": a.Action, "actor_kind": a.ActorKind, "at": a.At}
		setOpt(m, "actor_id", a.ActorID)
		setOpt(m, "note", a.Note)
		setOpt(m, "sanction_id", a.SanctionID)
		setOpt(m, "message_id", a.MessageID)
		out[i] = m
	}
	return out
}

// CaseMessageBody renders ModerationCaseMessage (original content: Safety Team only).
func CaseMessageBody(cm read.CaseMessage, inSnapshot bool) map[string]any {
	m := map[string]any{"message_id": cm.MessageID, "channel_id": cm.ChannelID, "sender_id": cm.SenderID, "kind": cm.Kind,
		"revision": max(cm.Revision, 1), "moderation_state": cm.ModerationState, "created_at": cm.CreatedAt, "in_snapshot": inSnapshot}
	if cm.Seq > 0 {
		m["seq"] = cm.Seq
	}
	if cm.ChannelType != "" {
		m["channel_type"] = cm.ChannelType
	}
	setOpt(m, "sender_school_id", cm.SenderSchoolID)
	setOpt(m, "body", cm.Body)
	setOpt(m, "body_display", cm.BodyDisplay)
	setOpt(m, "quick_message_id", cm.QuickMessageID)
	setOpt(m, "recalled_at", cm.RecalledAt)
	revisions := []map[string]any{}
	if cm.Revision > 1 || cm.EditedAt != nil {
		r := map[string]any{"revision": max(cm.Revision, 1)}
		setOpt(r, "edited_at", cm.EditedAt)
		revisions = append(revisions, r)
	}
	m["revisions"] = revisions
	return m
}

// SanctionBody renders Sanction.
func SanctionBody(s read.SanctionDetail) map[string]any {
	m := map[string]any{"sanction_id": s.ID, "tenant_id": s.TenantID, "user_id": s.UserID, "sanction_type": s.Type, "reason": s.Reason,
		"reason_code": s.ReasonCode, "issued_by_kind": s.IssuedByKind, "starts_at": s.StartsAt, "scope": map[string]any{}}
	if s.ChannelID != nil {
		m["scope"] = map[string]any{"channel_id": *s.ChannelID}
	}
	setOpt(m, "case_id", s.CaseID)
	setOpt(m, "ends_at", s.EndsAt)
	setOpt(m, "revoked_at", s.RevokedAt)
	setOpt(m, "revoked_by", s.RevokedBy)
	setOpt(m, "revoke_reason", s.RevokeReason)
	setOpt(m, "created_by", s.IssuedBy)
	return m
}

// NotificationBody renders SchoolSafetyNotification: ids, codes, the template summary — never chat content.
func NotificationBody(n read.NotificationRow) map[string]any {
	students := make([]map[string]any, len(n.Students))
	for i, s := range n.Students {
		students[i] = map[string]any{"student_id": s.StudentID, "role": s.Role}
	}
	m := map[string]any{"notification_id": n.ID, "case_id": n.CaseID, "tenant_id": n.TenantID, "school_id": n.SchoolID,
		"severity": n.Severity, "students": students, "summary": n.Summary, "school_notified_at": n.SchoolNotifiedAt,
		"ack_due_at": n.AckDueAt, "reminder_count": n.ReminderCount}
	setOpt(m, "category_code", n.Category)
	setOpt(m, "measures_taken", n.MeasuresTaken)
	setOpt(m, "last_reminded_at", n.LastRemindedAt)
	setOpt(m, "school_ack_at", n.SchoolAckAt)
	setOpt(m, "acknowledged_by", n.AcknowledgedBy)
	setOpt(m, "contact_outcome", n.ContactOutcome)
	setOpt(m, "ack_note", n.AckNote)
	return m
}

// reportThanks is the thank-you of createReport per language (MODERATION §5): never the identity of anyone.
var reportThanks = map[string]string{
	"vi": "Cảm ơn em đã báo cáo. Đội an toàn EduWorld sẽ xem xét. Em có thể chặn bạn ấy nếu muốn.",
	"en": "Thank you for reporting. The EduWorld safety team will review it. You can block this player if you want.",
}

// ReportLang returns the supported language of an Accept-Language value (vi by default).
func ReportLang(acceptLanguage string) string {
	if len(acceptLanguage) >= 2 && (acceptLanguage[:2] == "en" || acceptLanguage[:2] == "EN" || acceptLanguage[:2] == "En") {
		return "en"
	}
	return "vi"
}

// ReportBody renders the 201 of createReport.
func ReportBody(id any, lang string) map[string]any {
	return map[string]any{"report_id": id, "status": "open", "message": reportThanks[ReportLang(lang)]}
}

// CaseStateBody renders ModerationCaseDetail after a write (the case, its students and history; no content).
func CaseStateBody(c read.CaseRow, actions []read.ActionRow) map[string]any {
	m := CaseBody(c)
	m["subject_user_ids"] = c.Subjects
	m["actions"] = ActionsBody(actions)
	return m
}
