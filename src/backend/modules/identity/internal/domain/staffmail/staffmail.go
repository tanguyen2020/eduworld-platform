// Package staffmail holds the rules of the emails module identity sends to staff for the Platform Safety Team (F124 US2,
// US3, US4; MODERATION §6.1, §6.3, §6.4): templates and the parameters each may carry (ids and instants only — never a
// student name, a case summary, a category or chat content: R14, F124-AC-09), recipient rules and the retry schedule.
// Standard library only.
package staffmail

import (
	"slices"
	"time"
)

// Templates (identity.email_deliveries.template_code).
const (
	TemplateSchoolNotification = "school_safety_notification"
	TemplateSchoolReminder     = "school_safety_reminder"
	TemplateOncallPage         = "moderation_oncall_page"
	TemplateBreakGlassRequest  = "break_glass_request"
)

// Statuses of a delivery.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Roles of the recipients of a school notification: the School Admins of the school, otherwise the Tenant Admins of the
// tenant (F124 Assumptions, [Confirmed]).
const (
	RoleSchoolAdmin = "school_admin"
	RoleTenantAdmin = "tenant_admin"
	RoleSystemAdmin = "system_admin"
)

// allowed lists the parameters each template may store and render (F124-AC-09): no case id in the school emails.
var allowed = map[string][]string{
	TemplateSchoolNotification: {"school_id", "notification_id", "ack_due_at"},
	TemplateSchoolReminder:     {"school_id", "notification_id", "ack_due_at", "reminder_no"},
	TemplateOncallPage:         {"case_id", "severity", "level"},
	TemplateBreakGlassRequest:  {"grant_id"},
}

// Known reports a template.
func Known(template string) bool { _, ok := allowed[template]; return ok }

// Params keeps the allowed parameters of template only (anything else is dropped before it is stored).
func Params(template string, in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		if slices.Contains(allowed[template], k) && v != "" && len(v) <= 64 {
			out[k] = v
		}
	}
	return out
}

// School reports a template addressed to the staff of a school (tenant data) rather than to platform staff.
func School(template string) bool {
	return template == TemplateSchoolNotification || template == TemplateSchoolReminder
}

// Backoff returns the wait before attempt n+1 after n failed attempts: 1, 2, 4 … minutes, at most one hour
// (email_delivery_max_attempts, CONFIG_DEFAULTS).
func Backoff(attempts int) time.Duration {
	d := time.Minute
	for i := 1; i < attempts && d < time.Hour; i++ {
		d *= 2
	}
	return min(d, time.Hour)
}

// Exhausted reports a delivery that reached the maximum number of attempts (→ failed, alert EmailDeliveryFailing).
func Exhausted(attempts, max int) bool { return attempts >= max }
