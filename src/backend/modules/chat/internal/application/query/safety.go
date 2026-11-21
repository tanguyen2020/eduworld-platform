package query

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the reads of case content (MODERATION §6.1, §9): ids and the target tenant, never content (R14).
const (
	AuditCaseViewed         = "moderation.case.viewed"
	AuditCaseMessagesViewed = "moderation.case.messages_viewed"
	AuditCaseSearched       = "moderation.case.searched"
)

// maxTenantFilter bounds tenant_ids (as the httpx cross-tenant parameter).
const maxTenantFilter = 100

// Safety are the read use cases of the Platform Safety Team (F121 US2–US4, ADR-0072): the cross-tenant queue, case
// details and messages (assignee or team lead only), sanctions and school notifications. Cross-tenant reads run ONLY
// here, through tenancy.Bypass(ctx, "moderation_case") — audited (cross_tenant.accessed) and counted
// (eduworld_tenant_filter_bypass_total{reason="moderation_case"}); each view of case content adds its own audit with the
// target tenant. No endpoint reads or searches chat outside a case (F121-AC-25).
type Safety struct {
	Read    ReadSession
	Reader  read.SafetyReader
	Audit   audit.Recorder
	Cursors *page.Codec
	Now     func() time.Time
}

func (q Safety) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

// CaseDetail is ModerationCaseDetail.
type CaseDetail struct {
	Case          read.CaseRow
	Actions       []read.ActionRow
	Reports       []read.ReportRow
	Evidence      []read.EvidenceRow
	Notifications []read.NotificationRow
	SupportGuide  string
}

// CasePage is a page of listModerationCases.
type CasePage struct {
	Data []read.CaseRow
	Page page.Info
}

// CaseMessagePage is a page of listModerationCaseMessages.
type CaseMessagePage struct {
	Data []CaseMessage
	Page page.Info
}

// CaseMessage is ModerationCaseMessage.
type CaseMessage struct {
	read.CaseMessage
	InSnapshot bool
}

// SanctionPage is a page of listSanctions.
type SanctionPage struct {
	Data []read.SanctionDetail
	Page page.Info
}

// NotificationPage is a page of the school notifications.
type NotificationPage struct {
	Data []read.NotificationRow
	Page page.Info
}

// CaseFilter is the query of listModerationCases.
type CaseFilter struct {
	TenantIDs []uuid.UUID
	Status    string
	Severity  string
	Assignee  string
	Category  string
	Limit     int
	Cursor    string
}

// moderator authorizes a read of the Safety Team: platform role staff with perm (the internal gateway and the platform role guard are the
// httpx guards of /admin/).
func moderator(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(perm), "why": "platform_role"})
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func isLead(p authz.Principal) bool { return p.IsSystemAdmin || p.Has(authz.PermModerationCaseLead) }

// bypass lifts the tenant filter for one moderation read (reason moderation_case, audited, counted).
func bypass(ctx context.Context, tenants []uuid.UUID) (context.Context, error) {
	if len(tenants) > maxTenantFilter {
		return ctx, problem.Invalid("tenant_ids", problem.SubInvalid, "too many tenant ids")
	}
	return tenancy.Bypass(tenancy.WithModule(ctx, "chat"), tenancy.ReasonModerationCase, tenancy.WithTenantIDs(tenants...))
}

func (q Safety) audit(ctx context.Context, action string, c read.CaseRow, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["tenant_id"], meta["case_id"], meta["bypass_reason"] = c.TenantID.String(), c.ID.String(), string(tenancy.ReasonModerationCase)
	return q.Audit.Record(ctx, audit.Event{Action: action, TargetType: "moderation_case", TargetID: c.ID.String(),
		Result: audit.ResultSuccess, Metadata: meta})
}

// ListModerationCases implements listModerationCases (F121-AC-06): the queue of every tenant (or of tenant_ids) for the
// Safety Team, filtered by status, severity, assignee and category, keyset paginated; metadata only.
func (q Safety) ListModerationCases(ctx context.Context, f CaseFilter) (out CasePage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListModerationCases")
	defer func() { done(err) }()
	p, err := moderator(ctx, authz.PermModerationCaseRead)
	if err != nil {
		return out, err
	}
	limit := page.Limit(f.Limit)
	rf := read.CaseFilter{TenantIDs: f.TenantIDs, Status: f.Status, Severity: f.Severity, Assignee: f.Assignee, Me: p.UserID,
		Category: f.Category, Limit: limit + 1}
	if f.Cursor != "" && q.Cursors != nil {
		cur, err := q.Cursors.Decode(f.Cursor)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		sev, due, _ := strings.Cut(cur.Key, "|")
		at, err := time.Parse(time.RFC3339Nano, due)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		rf.After = &read.CaseCursor{Severity: sev, Due: at, ID: cur.ID}
	}
	bctx, err := bypass(ctx, f.TenantIDs)
	if err != nil {
		return out, err
	}
	var rows []read.CaseRow
	err = q.Read.Read(bctx, func(ctx context.Context) error {
		rows, err = q.Reader.Cases(ctx, rf)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		if q.Cursors != nil {
			last := rows[len(rows)-1]
			s := q.Cursors.Encode(page.Cursor{Key: last.Severity + "|" + last.SLADue.UTC().Format(time.RFC3339Nano), ID: last.ID})
			out.Page.NextCursor, out.Page.HasMore = &s, true
		}
	}
	out.Data = rows
	return out, nil
}

// readable loads a case through the bypass and checks that the caller may read its content: the assignee or a team lead
// (F121-AC-07); anyone else is 403 PERMISSION_DENIED (audited).
func (q Safety) readable(ctx context.Context, p authz.Principal, id uuid.UUID, fn func(ctx context.Context, c read.CaseRow) error) error {
	bctx, err := bypass(ctx, nil)
	if err != nil {
		return err
	}
	return q.Read.Read(bctx, func(rctx context.Context) error {
		c, err := q.Reader.Case(rctx, id)
		if err != nil {
			return err
		}
		if c == nil {
			return problem.New(problem.NotFound, "")
		}
		if !safety.CanRead(c.Assignee, p.UserID, isLead(p)) {
			authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "not_assignee", "case_id": id.String(),
				"user_id": p.UserID.String()})
			return problem.New(problem.PermissionDenied, "")
		}
		return fn(rctx, *c)
	})
}

// GetModerationCase implements getModerationCase (F121-AC-07, AC-24): the case with its history, reports, evidence
// references and school notifications, plus the support guide of a self-harm case; audited moderation.case.viewed with
// the target tenant.
func (q Safety) GetModerationCase(ctx context.Context, id uuid.UUID) (out CaseDetail, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetModerationCase")
	defer func() { done(err) }()
	p, err := moderator(ctx, authz.PermModerationCaseRead)
	if err != nil {
		return out, err
	}
	err = q.readable(ctx, p, id, func(rctx context.Context, c read.CaseRow) error {
		out.Case = c
		if out.Actions, err = q.Reader.ActionsOf(rctx, c.TenantID, c.ID, 100); err != nil {
			return err
		}
		if out.Reports, err = q.Reader.ReportsOf(rctx, c.TenantID, c.ID, 100); err != nil {
			return err
		}
		if out.Evidence, err = q.Reader.EvidenceOf(rctx, c.TenantID, c.ID); err != nil {
			return err
		}
		out.Notifications, err = q.Reader.CaseNotifications(rctx, c.TenantID, c.ID)
		return err
	})
	if err != nil {
		return out, err
	}
	out.SupportGuide = safety.SupportGuide(deref(out.Case.Category))
	return out, q.audit(ctx, AuditCaseViewed, out.Case, nil)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ListModerationCaseMessages implements listModerationCaseMessages (F121-AC-07, AC-25, AC-26): the evidence snapshots of
// the case and, while its channel exists, ±200 messages of context around the reported message — original content,
// revision state; q searches only within these messages (audit moderation.case.searched). After the purge of a gameplay
// channel only the stored evidence remains (ADR-0033). Every call is audited with the target tenant.
func (q Safety) ListModerationCaseMessages(ctx context.Context, id uuid.UUID, text string, limit int, cursor string) (out CaseMessagePage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListModerationCaseMessages")
	defer func() { done(err) }()
	p, err := moderator(ctx, authz.PermModerationCaseMessagesRead)
	if err != nil {
		return out, err
	}
	var c read.CaseRow
	var all []CaseMessage
	err = q.readable(ctx, p, id, func(rctx context.Context, row read.CaseRow) error {
		c = row
		evidence, err := q.Reader.EvidenceOf(rctx, c.TenantID, c.ID)
		if err != nil {
			return err
		}
		byID := map[uuid.UUID]int{}
		anchor := int64(0)
		for _, e := range evidence {
			var snap struct {
				TargetID uuid.UUID          `json:"target_id"`
				Messages []read.CaseMessage `json:"messages"`
			}
			if err := json.Unmarshal(e.Snapshot, &snap); err != nil {
				return err
			}
			for _, m := range snap.Messages {
				if m.MessageID == snap.TargetID && anchor == 0 {
					anchor = m.Seq
				}
				if _, ok := byID[m.MessageID]; !ok {
					byID[m.MessageID] = len(all)
					all = append(all, CaseMessage{CaseMessage: m, InSnapshot: true})
				}
			}
		}
		if c.ChannelID == nil {
			return nil
		}
		typ, live, err := q.Reader.ChannelExists(rctx, c.TenantID, *c.ChannelID)
		if err != nil || !live {
			return err // purged channel: the stored evidence only (F121-AC-26)
		}
		window, err := q.Reader.ChannelWindow(rctx, c.TenantID, *c.ChannelID, anchor, safety.ModeratorContext, safety.ModeratorContext)
		if err != nil {
			return err
		}
		for _, m := range window {
			m.ChannelType = typ
			if i, ok := byID[m.MessageID]; ok {
				all[i].CaseMessage = m // current state (revision, removed) of a snapshot message
				continue
			}
			byID[m.MessageID] = len(all)
			all = append(all, CaseMessage{CaseMessage: m})
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Seq != all[j].Seq {
			return all[i].Seq < all[j].Seq
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	action := AuditCaseMessagesViewed
	meta := map[string]any{"messages": len(all)}
	if text = strings.TrimSpace(text); text != "" {
		action = AuditCaseSearched
		meta["query_length"] = len([]rune(text)) // never the query itself (R14)
		needle := strings.ToLower(text)
		all = slices.DeleteFunc(all, func(m CaseMessage) bool {
			return !strings.Contains(strings.ToLower(deref(m.Body)), needle) && !strings.Contains(strings.ToLower(deref(m.BodyDisplay)), needle)
		})
	}
	limit = page.Limit(limit)
	start := 0
	if cursor != "" && q.Cursors != nil {
		cur, err := q.Cursors.Decode(cursor)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		if start, err = strconv.Atoi(cur.Key); err != nil || start < 0 || cur.ID != c.ID {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
	}
	if start > len(all) {
		start = len(all)
	}
	end := min(start+limit, len(all))
	out.Data = all[start:end]
	out.Page = page.Info{Limit: limit}
	if end < len(all) && q.Cursors != nil {
		s := q.Cursors.Encode(page.Cursor{Key: strconv.Itoa(end), ID: c.ID})
		out.Page.NextCursor, out.Page.HasMore = &s, true
	}
	return out, q.audit(ctx, action, c, meta)
}

// ListSanctions implements listSanctions (moderation.sanction.read): sanctions of every tenant (or tenant_ids), of one
// student, active or not; newest first.
func (q Safety) ListSanctions(ctx context.Context, tenants []uuid.UUID, user *uuid.UUID, active *bool, limit int, cursor string) (out SanctionPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSanctions")
	defer func() { done(err) }()
	if _, err := moderator(ctx, authz.PermModerationSanctionRead); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	f := read.SanctionFilter{TenantIDs: tenants, UserID: user, Active: active, Now: q.now(), Limit: limit + 1}
	if f.After, err = q.timeCursor(cursor); err != nil {
		return out, err
	}
	bctx, err := bypass(ctx, tenants)
	if err != nil {
		return out, err
	}
	var rows []read.SanctionDetail
	if err = q.Read.Read(bctx, func(ctx context.Context) error {
		rows, err = q.Reader.Sanctions(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		out.Page.NextCursor, out.Page.HasMore = q.next(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID), true
	}
	out.Data = rows
	return out, nil
}

func (q Safety) timeCursor(cursor string) (*read.Cursor, error) {
	if cursor == "" || q.Cursors == nil {
		return nil, nil
	}
	cur, err := q.Cursors.Decode(cursor)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return timeCursor(cur)
}

func (q Safety) next(at time.Time, id uuid.UUID) *string {
	if q.Cursors == nil {
		return nil
	}
	s := q.Cursors.Encode(page.Cursor{Key: at.UTC().Format(time.RFC3339Nano), ID: id})
	return &s
}

// ListSchoolNotificationsPlatform implements listSchoolSafetyNotificationsPlatform (F121-AC-20): the notifications of
// every tenant (or tenant_ids) for the Safety Team, filtered by acknowledged and overdue.
func (q Safety) ListSchoolNotificationsPlatform(ctx context.Context, tenants []uuid.UUID, acknowledged, overdue *bool, limit int,
	cursor string) (out NotificationPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSchoolNotificationsPlatform")
	defer func() { done(err) }()
	if _, err := moderator(ctx, authz.PermModerationSchoolNotificationSend); err != nil {
		return out, err
	}
	bctx, err := bypass(ctx, tenants)
	if err != nil {
		return out, err
	}
	return q.notifications(bctx, read.NotificationFilter{TenantIDs: tenants, Acknowledged: acknowledged, Overdue: overdue}, limit, cursor)
}

func (q Safety) notifications(ctx context.Context, f read.NotificationFilter, limit int, cursor string) (out NotificationPage, err error) {
	limit = page.Limit(limit)
	f.Now, f.Limit = q.now(), limit+1
	if f.After, err = q.timeCursor(cursor); err != nil {
		return out, err
	}
	var rows []read.NotificationRow
	if err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.Notifications(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		out.Page.NextCursor, out.Page.HasMore = q.next(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID), true
	}
	out.Data = rows
	return out, nil
}

// ListSchoolNotifications implements listSchoolSafetyNotifications (F121-AC-18): a school admin sees the notifications of
// the schools of their grants, a tenant admin those of every school of the tenant; a school outside the grants is 403.
// Summary, students (ids, role), severity, measures and deadlines only — never message content.
func (q Safety) ListSchoolNotifications(ctx context.Context, school *uuid.UUID, acknowledged *bool, limit int, cursor string) (out NotificationPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSchoolNotifications")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if p.TenantID == nil || p.UserType != authz.UserTypeStaff {
		return out, problem.New(problem.PermissionDenied, "")
	}
	tenant := *p.TenantID
	perm := authz.PermModerationSchoolNotificationRead
	if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: &tenant, SchoolID: school}); err != nil {
		if school != nil || !p.Has(perm) {
			return out, err
		}
	}
	f := read.NotificationFilter{TenantIDs: []uuid.UUID{tenant}, Acknowledged: acknowledged}
	if school != nil {
		f.SchoolIDs = []uuid.UUID{*school}
	} else {
		var schools []uuid.UUID
		for _, g := range p.Grants {
			if !slices.Contains(g.Permissions, perm) {
				continue
			}
			if g.SchoolID == nil {
				schools = nil
				break
			}
			schools = append(schools, *g.SchoolID)
		}
		f.SchoolIDs = schools
	}
	return q.notifications(ctx, f, limit, cursor)
}

// QueueCounts returns the open cases per severity and assignment and the overdue school notifications of every tenant
// (gauges of the job moderation-sla-escalator; tenancy.Bypass(platform_job), counts only).
func (q Safety) QueueCounts(ctx context.Context) ([]read.OpenCount, int, error) {
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "chat"), tenancy.ReasonPlatformJob)
	if err != nil {
		return nil, 0, err
	}
	var counts []read.OpenCount
	var overdue int
	err = q.Read.Read(bctx, func(ctx context.Context) error {
		var e error
		if counts, e = q.Reader.OpenCounts(ctx); e != nil {
			return e
		}
		overdue, e = q.Reader.OverdueNotifications(ctx, q.now())
		return e
	})
	return counts, overdue, err
}
