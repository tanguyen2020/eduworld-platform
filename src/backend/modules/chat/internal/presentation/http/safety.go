package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SafetyCommands are the write use cases of the Safety Team (command.Safety, F121).
type SafetyCommands interface {
	CreateReport(ctx context.Context, in command.ReportInput) (command.ReportResult, error)
	AssignCase(ctx context.Context, id, assignee uuid.UUID) (command.CaseState, error)
	ActOnCase(ctx context.Context, id uuid.UUID, version int64, in command.ActInput) (command.CaseState, error)
	NotifySchool(ctx context.Context, id uuid.UUID, in command.NotifyInput) (read.NotificationRow, error)
	CreateSanction(ctx context.Context, in command.SanctionInput) (read.SanctionDetail, error)
	RevokeSanction(ctx context.Context, id uuid.UUID, reason string) (read.SanctionDetail, error)
	AcknowledgeSchoolNotification(ctx context.Context, id uuid.UUID, in command.AckInput) (read.NotificationRow, error)
}

// SafetyQueries are the read use cases of the Safety Team (query.Safety, F121).
type SafetyQueries interface {
	ListModerationCases(ctx context.Context, f query.CaseFilter) (query.CasePage, error)
	GetModerationCase(ctx context.Context, id uuid.UUID) (query.CaseDetail, error)
	ListModerationCaseMessages(ctx context.Context, id uuid.UUID, q string, limit int, cursor string) (query.CaseMessagePage, error)
	ListSanctions(ctx context.Context, tenants []uuid.UUID, user *uuid.UUID, active *bool, limit int, cursor string) (query.SanctionPage, error)
	ListSchoolNotificationsPlatform(ctx context.Context, tenants []uuid.UUID, acknowledged, overdue *bool, limit int, cursor string) (query.NotificationPage, error)
	ListSchoolNotifications(ctx context.Context, school *uuid.UUID, acknowledged *bool, createdFrom *time.Time, limit int, cursor string) (query.NotificationPage, error)
	ListModerationCaseTenants(ctx context.Context, limit int, cursor string) (query.CaseTenantPage, error)
}

// SafetyHandlers serve the operations of the Safety Team (core-api): createReport (player route, game_player guard),
// /admin/moderation/* and /admin/safety/* (internal gateway: httpx adds GuardPortalStaff + GuardPlatformRole; the use
// cases require moderation.* in Go, R7), /tenant/safety/* (school staff, portal).
type SafetyHandlers struct {
	Commands SafetyCommands
	Queries  SafetyQueries
}

// SafetyOperations are the F121 operations (checked against the spec by the contract test).
var SafetyOperations = []string{"createReport", "listModerationCases", "getModerationCase", "listModerationCaseMessages",
	"assignModerationCase", "actOnModerationCase", "notifySchoolForCase", "listSanctions", "createSanction", "revokeSanction",
	"listSchoolSafetyNotificationsPlatform", "listSchoolSafetyNotifications", "acknowledgeSchoolSafetyNotification",
	"listModerationCaseTenants"}

func init() {
	Operations["createReport"] = op("createReport", http.MethodPost, "/chat/reports", httpx.IdempotencyRequired, http.StatusCreated)
	for _, o := range []httpx.Operation{
		{ID: "listModerationCases", Method: http.MethodGet, Pattern: "/admin/moderation/cases", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "listModerationCaseTenants", Method: http.MethodGet, Pattern: "/admin/moderation/case-tenants", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "getModerationCase", Method: http.MethodGet, Pattern: "/admin/moderation/cases/{case_id}", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "listModerationCaseMessages", Method: http.MethodGet, Pattern: "/admin/moderation/cases/{case_id}/messages",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "assignModerationCase", Method: http.MethodPost, Pattern: "/admin/moderation/cases/{case_id}:assign", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "actOnModerationCase", Method: http.MethodPost, Pattern: "/admin/moderation/cases/{case_id}:act", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "notifySchoolForCase", Method: http.MethodPost, Pattern: "/admin/moderation/cases/{case_id}:notify-school",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "listSanctions", Method: http.MethodGet, Pattern: "/admin/moderation/sanctions", Tenant: httpx.TenantNone,
			SuccessStatus: http.StatusOK},
		{ID: "createSanction", Method: http.MethodPost, Pattern: "/admin/moderation/sanctions", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "revokeSanction", Method: http.MethodPost, Pattern: "/admin/moderation/sanctions/{sanction_id}:revoke",
			Tenant: httpx.TenantRequired, Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "listSchoolSafetyNotificationsPlatform", Method: http.MethodGet, Pattern: "/admin/safety/school-notifications",
			Tenant: httpx.TenantNone, SuccessStatus: http.StatusOK},
		{ID: "listSchoolSafetyNotifications", Method: http.MethodGet, Pattern: "/tenant/safety/school-notifications",
			Tenant: httpx.TenantRequired, SuccessStatus: http.StatusOK},
		{ID: "acknowledgeSchoolSafetyNotification", Method: http.MethodPost,
			Pattern: "/tenant/safety/school-notifications/{notification_id}:acknowledge", Tenant: httpx.TenantRequired,
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
	} {
		o.Module = "chat"
		Operations[o.ID] = o
	}
}

// registerSafety mounts the F121 operations.
func registerSafety(s *httpx.Server, h *SafetyHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the safety handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"createReport":                          h.createReport,
		"listModerationCases":                   h.listCases,
		"listModerationCaseTenants":             h.listCaseTenants,
		"getModerationCase":                     h.getCase,
		"listModerationCaseMessages":            h.caseMessages,
		"assignModerationCase":                  h.assign,
		"actOnModerationCase":                   h.act,
		"notifySchoolForCase":                   h.notifySchool,
		"listSanctions":                         h.listSanctions,
		"createSanction":                        h.createSanction,
		"revokeSanction":                        h.revokeSanction,
		"listSchoolSafetyNotificationsPlatform": h.listNotificationsPlatform,
		"listSchoolSafetyNotifications":         h.listNotifications,
		"acknowledgeSchoolSafetyNotification":   h.acknowledge,
	}
	for _, id := range SafetyOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func (h SafetyHandlers) createReport(w http.ResponseWriter, r *http.Request) {
	var b struct {
		TargetType string    `json:"target_type"`
		TargetID   uuid.UUID `json:"target_id"`
		Reason     string    `json:"reason"`
		Note       *string   `json:"note"`
		Context    *struct {
			ChannelID     *uuid.UUID `json:"channel_id"`
			MatchID       *uuid.UUID `json:"match_id"`
			PartyID       *uuid.UUID `json:"party_id"`
			ReportedField string     `json:"reported_field"`
		} `json:"context"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := command.ReportInput{TargetType: b.TargetType, TargetID: b.TargetID, Reason: b.Reason, Note: b.Note}
	if b.Context != nil {
		in.ChannelID, in.MatchID, in.PartyID = b.Context.ChannelID, b.Context.MatchID, b.Context.PartyID
		in.ReportedField = b.Context.ReportedField
	}
	in.Lang = service.ReportLang(r.Header.Get("Accept-Language"))
	res, err := h.Commands.CreateReport(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Language", in.Lang)
	writeJSON(w, http.StatusCreated, service.ReportBody(res.ReportID, in.Lang))
}

// tenantIDs reads tenant_ids (repeated or comma-separated).
func tenantIDs(r *http.Request) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, v := range r.URL.Query()["tenant_ids"] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part == "" {
				continue
			}
			id, err := uuid.Parse(part)
			if err != nil {
				return nil, problem.Invalid("tenant_ids", problem.SubInvalid, "tenant_ids must be UUIDs")
			}
			out = append(out, id)
		}
	}
	return out, nil
}

func optBool(r *http.Request, name string) (*bool, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil, problem.Invalid(name, problem.SubInvalid, "must be a boolean")
	}
	return &b, nil
}

func (h SafetyHandlers) listCases(w http.ResponseWriter, r *http.Request) {
	tenants, err := tenantIDs(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	qs := r.URL.Query()
	p, err := h.Queries.ListModerationCases(r.Context(), query.CaseFilter{TenantIDs: tenants, Status: qs.Get("status"),
		Severity: qs.Get("severity"), Assignee: qs.Get("assignee"), Category: qs.Get("category_code"), Limit: limitOf(r),
		Cursor: qs.Get("cursor")})
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, c := range p.Data {
		data[i] = service.CaseBody(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyHandlers) listCaseTenants(w http.ResponseWriter, r *http.Request) {
	p, err := h.Queries.ListModerationCaseTenants(r.Context(), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, t := range p.Data {
		m := map[string]any{"tenant_id": t.TenantID}
		if t.Name != nil {
			m["tenant_name"] = *t.Name
		}
		if t.Type != nil {
			m["tenant_type"] = *t.Type
		}
		data[i] = m
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyHandlers) getCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "case_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	d, err := h.Queries.GetModerationCase(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	m := service.CaseBody(d.Case)
	m["subject_user_ids"] = d.Case.Subjects
	m["actions"] = service.ActionsBody(d.Actions)
	reports := make([]map[string]any, len(d.Reports))
	for i, rp := range d.Reports {
		reports[i] = map[string]any{"report_id": rp.ID, "reporter_id": rp.ReporterID, "reason": rp.Reason, "status": rp.Status,
			"created_at": rp.CreatedAt}
		if rp.Note != nil {
			reports[i]["note"] = *rp.Note
		}
	}
	m["reports"] = reports
	evidence := make([]map[string]any, len(d.Evidence))
	var messageIDs []uuid.UUID
	for i, e := range d.Evidence {
		evidence[i] = map[string]any{"evidence_id": e.ID, "created_at": e.CreatedAt}
		var snap struct {
			Messages []struct {
				MessageID uuid.UUID `json:"message_id"`
			} `json:"messages"`
			ReportedNickname *string `json:"reported_nickname"`
		}
		if json.Unmarshal(e.Snapshot, &snap) == nil {
			if snap.ReportedNickname != nil { // server snapshot of a nickname report (F136-AC-18, AC-22)
				evidence[i]["reported_nickname"] = *snap.ReportedNickname
			}
			for _, msg := range snap.Messages {
				if len(messageIDs) < 200 && !containsID(messageIDs, msg.MessageID) {
					messageIDs = append(messageIDs, msg.MessageID)
				}
			}
		}
	}
	m["evidence"] = evidence
	if messageIDs == nil {
		messageIDs = []uuid.UUID{}
	}
	m["evidence_message_ids"] = messageIDs
	notes := make([]map[string]any, len(d.Notifications))
	for i, n := range d.Notifications {
		notes[i] = service.NotificationBody(n)
	}
	m["school_notifications"] = notes
	if d.SupportGuide != "" {
		m["support_guide"] = d.SupportGuide
	}
	w.Header().Set("Cache-Control", "no-store")
	etag(w, d.Case.Version)
	writeJSON(w, http.StatusOK, m)
}

func containsID(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func (h SafetyHandlers) caseMessages(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "case_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListModerationCaseMessages(r.Context(), id, r.URL.Query().Get("q"), limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, m := range p.Data {
		data[i] = service.CaseMessageBody(m.CaseMessage, m.InSnapshot)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyHandlers) assign(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "case_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Assignee uuid.UUID `json:"assignee_moderator_id"`
	}
	if err := decode(r, &b); err != nil || b.Assignee == uuid.Nil {
		fail(w, r, problem.Invalid("assignee_moderator_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	s, err := h.Commands.AssignCase(r.Context(), id, b.Assignee)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Case.Version)
	writeJSON(w, http.StatusOK, service.CaseBody(s.Case))
}

func (h SafetyHandlers) act(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "case_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	version, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Action          string     `json:"action"`
		TargetUserID    *uuid.UUID `json:"target_user_id"`
		MessageID       *uuid.UUID `json:"message_id"`
		SanctionType    *string    `json:"sanction_type"`
		DurationMinutes *int       `json:"duration_minutes"`
		Note            string     `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.ActOnCase(r.Context(), id, version, command.ActInput{Action: b.Action, TargetUserID: b.TargetUserID,
		MessageID: b.MessageID, SanctionType: b.SanctionType, DurationMinutes: b.DurationMinutes, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Case.Version)
	writeJSON(w, http.StatusOK, service.CaseStateBody(s.Case, s.Actions))
}

func (h SafetyHandlers) notifySchool(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "case_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		SchoolID   uuid.UUID   `json:"school_id"`
		StudentIDs []uuid.UUID `json:"student_ids"`
		Summary    string      `json:"summary"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	n, err := h.Commands.NotifySchool(r.Context(), id, command.NotifyInput{SchoolID: b.SchoolID, StudentIDs: b.StudentIDs, Summary: b.Summary})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, service.NotificationBody(n))
}

func (h SafetyHandlers) listSanctions(w http.ResponseWriter, r *http.Request) {
	tenants, err := tenantIDs(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	user, err := optUUID(r, "user_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	active, err := optBool(r, "active")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListSanctions(r.Context(), tenants, user, active, limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, s := range p.Data {
		data[i] = service.SanctionBody(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyHandlers) createSanction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		UserID          uuid.UUID  `json:"user_id"`
		CaseID          *uuid.UUID `json:"case_id"`
		SanctionType    string     `json:"sanction_type"`
		DurationMinutes *int       `json:"duration_minutes"`
		Reason          string     `json:"reason"`
		Scope           *struct {
			ChannelID *uuid.UUID `json:"channel_id"`
		} `json:"scope"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	in := command.SanctionInput{UserID: b.UserID, CaseID: b.CaseID, Type: b.SanctionType, DurationMinutes: b.DurationMinutes, Reason: b.Reason}
	if b.Scope != nil {
		in.ChannelID = b.Scope.ChannelID
	}
	s, err := h.Commands.CreateSanction(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, service.SanctionBody(s))
}

func (h SafetyHandlers) revokeSanction(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "sanction_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.Commands.RevokeSanction(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, service.SanctionBody(s))
}

func (h SafetyHandlers) listNotificationsPlatform(w http.ResponseWriter, r *http.Request) {
	tenants, err := tenantIDs(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	ack, err := optBool(r, "acknowledged")
	if err != nil {
		fail(w, r, err)
		return
	}
	overdue, err := optBool(r, "overdue")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListSchoolNotificationsPlatform(r.Context(), tenants, ack, overdue, limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeNotifications(w, p)
}

func (h SafetyHandlers) listNotifications(w http.ResponseWriter, r *http.Request) {
	school, err := optUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	ack, err := optBool(r, "acknowledged")
	if err != nil {
		fail(w, r, err)
		return
	}
	var from *time.Time
	if v := r.URL.Query().Get("created_from"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			fail(w, r, problem.Invalid("created_from", problem.SubInvalid, "must be a date-time"))
			return
		}
		from = &t
	}
	p, err := h.Queries.ListSchoolNotifications(r.Context(), school, ack, from, limitOf(r), r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeNotifications(w, p)
}

func writeNotifications(w http.ResponseWriter, p query.NotificationPage) {
	data := make([]map[string]any, len(p.Data))
	for i, n := range p.Data {
		data[i] = service.NotificationBody(n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h SafetyHandlers) acknowledge(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "notification_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Outcome string  `json:"contact_outcome"`
		Note    *string `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	n, err := h.Commands.AcknowledgeSchoolNotification(r.Context(), id, command.AckInput{Outcome: b.Outcome, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, service.NotificationBody(n))
}
