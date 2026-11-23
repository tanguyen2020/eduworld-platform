package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SchoolEventInput is SchoolEventCreate / SchoolEventUpdate: Set names the fields present in the request (an update
// changes only them) and Raw keeps the request as given (school_params).
type SchoolEventInput struct {
	TemplateID   uuid.UUID
	Name         liveops.Localized
	Description  liveops.Localized
	StartsAt     *time.Time
	EndsAt       *time.Time
	SchoolIDs    []uuid.UUID
	GradeLevels  []int
	GoalTarget   *int64
	QuestIDs     []uuid.UUID
	ChallengeIDs []uuid.UUID
	RewardOption *string
	Set          map[string]bool
	Raw          map[string]any
}

// staffOf returns the staff principal of the session tenant (route guard portal_staff).
func staffOf(ctx context.Context) (authz.Principal, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return p, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStaff {
		return p, uuid.Nil, problem.New(problem.PortalAccessStaffOnly, "")
	}
	if p.TenantID != nil {
		return p, *p.TenantID, nil
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if id, single := s.SessionTenantID(); single {
			return p, id, nil
		}
	}
	return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
}

// SchoolGrants returns the schools the school-event grants of p are limited to (nil, true = the whole tenant).
func SchoolGrants(p authz.Principal) ([]uuid.UUID, bool) {
	var schools []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermLmsSchoolEventWrite) {
			continue
		}
		if g.SchoolID == nil {
			return nil, true
		}
		schools = append(schools, *g.SchoolID)
	}
	return schools, false
}

// SchoolEventVisible reports a school event p may see (F077-AC-25): every event for a tenant-wide grant, the events of
// its schools for a school-limited grant (other schools: 404).
func SchoolEventVisible(p authz.Principal, owner *uuid.UUID) bool {
	schools, all := SchoolGrants(p)
	return all || (owner != nil && slices.Contains(schools, *owner))
}

// requireSchoolEvent authorizes lms.school_event.write on the schools of an audience (403 + authz.denied without it).
func requireSchoolEvent(ctx context.Context, p authz.Principal, tenant uuid.UUID, schools []uuid.UUID) error {
	if len(schools) == 0 {
		return authz.Require(ctx, p, authz.PermLmsSchoolEventWrite, authz.Resource{TenantID: &tenant})
	}
	for _, s := range schools {
		sc := s
		if err := authz.Require(ctx, p, authz.PermLmsSchoolEventWrite, authz.Resource{TenantID: &tenant, SchoolID: &sc}); err != nil {
			return err
		}
	}
	return nil
}

func (in SchoolEventInput) params() liveops.SchoolParams {
	set := map[string]bool{}
	for _, k := range liveops.TemplateParams {
		if in.Set[k] {
			set[k] = true
		}
	}
	p := liveops.SchoolParams{Name: in.Name, Description: in.Description, SchoolIDs: in.SchoolIDs, GradeLevels: in.GradeLevels,
		GoalTarget: in.GoalTarget, QuestIDs: in.QuestIDs, ChallengeIDs: in.ChallengeIDs, RewardOption: in.RewardOption, Set: set}
	if in.StartsAt != nil {
		p.StartsAt = *in.StartsAt
	}
	if in.EndsAt != nil {
		p.EndsAt = *in.EndsAt
	}
	return p
}

// screen checks the texts of a school event against the effective sensitive-word catalogue of module chat (F077-AC-23):
// a flagged text is 422 VALIDATION_FAILED naming the field only (never the word); a failing or missing check is 503
// UNAVAILABLE and nothing is stored (never fail-open).
func (h *LiveOps) screen(ctx context.Context, tenant uuid.UUID, in SchoolEventInput) error {
	var fields, texts []string
	add := func(field, text string) {
		if strings.TrimSpace(text) != "" {
			fields, texts = append(fields, field), append(texts, text)
		}
	}
	add("name.vi", in.Name["vi"])
	add("name.en", in.Name["en"])
	add("description.vi", in.Description["vi"])
	add("description.en", in.Description["en"])
	if len(texts) == 0 {
		return nil
	}
	if h.Screener == nil {
		return problem.New(problem.Unavailable, "text screening is unavailable")
	}
	bad, err := h.Screener.Screen(ctx, tenant, "vi", texts)
	if err != nil {
		return problem.New(problem.Unavailable, "text screening is unavailable")
	}
	if len(bad) > 0 && bad[0] >= 0 && bad[0] < len(fields) {
		return problem.Invalid(fields[bad[0]], problem.SubInvalid, "contains words that are not allowed")
	}
	return nil
}

// eventDoc is the stored payload of a school event version (event.schema.json).
func eventDoc(d liveops.Definition, version uuid.UUID, versionNo int) ([]byte, string, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, "", err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(doc)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	doc["schema_version"], doc["event_version_id"], doc["version"], doc["status"] = 1, version.String(), versionNo, "published"
	stored, err := json.Marshal(doc)
	return stored, hex.EncodeToString(sum[:]), err
}

// CreateSchoolEvent is createSchoolEvent (F077-AC-21…AC-25): lms.school_event.write on the schools of the audience, a
// template assigned to the tenant, only its allowed_params and options, a duration within the template and
// school_event_max_duration_days, texts screened (sensitive words: 422 without the word; screening down: 503, nothing
// stored), at most school_events_concurrent_per_tenant active events (409 CONFLICT); the version is published and its run
// scheduled (event.scheduled), audit school_event.created.
func (h *LiveOps) CreateSchoolEvent(ctx context.Context, in SchoolEventInput) (out port.SchoolEventRow, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateSchoolEvent")
	defer func() { done(err) }()
	p, tenant, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	schools := in.SchoolIDs
	if limited, all := SchoolGrants(p); !all && len(schools) == 0 && len(limited) == 1 {
		schools = []uuid.UUID{limited[0]} // a school admin creates for its school
	}
	if err := requireSchoolEvent(ctx, p, tenant, schools); err != nil {
		return out, err
	}
	in.SchoolIDs = schools
	if in.StartsAt == nil || in.EndsAt == nil {
		return out, problem.Invalid("starts_at", problem.SubRequired, "starts_at and ends_at are required")
	}
	now := h.now()
	if in.StartsAt.Before(now.Add(-time.Minute)) {
		return out, problem.Invalid("starts_at", problem.SubInvalid, "must not be in the past")
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	// Template and parameters first (a refused request never reaches the screening), then the screening outside the
	// transaction: when module chat is down nothing is stored.
	var tpl *port.TemplateRow
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		if tpl, err = h.checkTemplate(ctx, tenant, in); err != nil {
			return err
		}
		return h.checkSchools(ctx, tenant, schools)
	}); err != nil {
		return out, err
	}
	if err := h.screen(ctx, tenant, in); err != nil {
		return out, err
	}
	eventID := ids.New()
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Store.LockTenant(ctx, tenant); err != nil {
			return err
		}
		active, err := h.Store.ActiveTenantEvents(ctx, tenant)
		if err != nil {
			return err
		}
		if active >= h.policy().SchoolEventsMax {
			return problem.New(problem.Conflict, "too many school events running at once")
		}
		code := "school.e" + strings.ReplaceAll(eventID.String(), "-", "")
		d := tpl.Template.Build(eventID, tenant, code, tpl.VersionNo, in.params(), allGrades())
		if err := d.Validate(h.policy().Rules, 0); err != nil {
			return liveopsProblem(err)
		}
		var owner *uuid.UUID
		if len(schools) == 1 {
			s := schools[0]
			owner = &s
		}
		if err := h.Store.InsertTenantEvent(ctx, tenant, eventID, code, d.Kind, tpl.TemplateID, owner, in.Raw, p.UserID, now); err != nil {
			return err
		}
		if err := h.publishTenantVersion(ctx, tenant, d, 1, in.Raw, p.UserID, now); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditSchoolEventCreated, TargetType: "school_event", TargetID: eventID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"template_id": tpl.TemplateID, "school_id": owner}}); err != nil {
			return err
		}
		row, err := h.Store.LockSchoolEvent(ctx, tenant, eventID)
		if err != nil || row == nil {
			return errors.Join(err, problem.New(problem.Internal, "school event not stored"))
		}
		out = *row
		return idempotency.SetResult(ctx, port.SchoolEventView(out))
	})
	return out, err
}

func allGrades() []int { return []int{1, 2, 3, 4, 5, 6, 7, 8, 9} }

func (h *LiveOps) checkTemplate(ctx context.Context, tenant uuid.UUID, in SchoolEventInput) (*port.TemplateRow, error) {
	tpl, err := h.Store.Template(ctx, tenant, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, problem.New(problem.EventTemplateParamNotAllowed, "template_id: no template assigned to the tenant")
	}
	if err := tpl.Template.Check(in.params(), h.policy().SchoolEventMaxDays); err != nil {
		return nil, liveopsProblem(err)
	}
	return tpl, nil
}

// checkSchools refuses an audience outside the tenant of the caller (F077-AC-22).
func (h *LiveOps) checkSchools(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID) error {
	if len(schools) == 0 {
		return nil
	}
	n, err := h.Store.KnownSchools(ctx, tenant, schools)
	if err != nil {
		return err
	}
	if n != len(schools) {
		return problem.New(problem.EventTemplateParamNotAllowed, "audience.school_ids: a school outside the tenant")
	}
	return nil
}

func (h *LiveOps) publishTenantVersion(ctx context.Context, tenant uuid.UUID, d liveops.Definition, versionNo int, params map[string]any,
	by uuid.UUID, now time.Time) error {
	vid := ids.New()
	stored, sha, err := eventDoc(d, vid, versionNo)
	if err != nil {
		return err
	}
	if h.EventSchema != nil {
		if err := h.EventSchema.Validate(stored); err != nil {
			return problem.Invalid("payload", problem.SubInvalid, err.Error())
		}
	}
	if err := h.Store.InsertTenantVersion(ctx, write.EventVersionRow{VersionID: vid, EventID: d.EventID, TenantID: tenant, VersionNo: versionNo,
		Payload: stored, SHA256: sha, Definition: d, EffectiveFrom: now, By: by, At: now}, params); err != nil {
		return err
	}
	return h.EventPublished(ctx, d, vid, versionNo, now)
}

// UpdateSchoolEvent is updateSchoolEvent (F077-AC-22, AC-23): If-Match is the version of the event (412 when stale); only
// a scheduled event changes (409 once it runs); the merged parameters are checked against the template, the new texts
// screened, a new version published and its run re-pinned; audit school_event.updated.
func (h *LiveOps) UpdateSchoolEvent(ctx context.Context, event uuid.UUID, ifMatch int64, in SchoolEventInput) (out port.SchoolEventRow, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.UpdateSchoolEvent")
	defer func() { done(err) }()
	p, tenant, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	now := h.now()
	var merged SchoolEventInput
	var tpl *port.TemplateRow
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		row, err := h.visibleSchoolEvent(ctx, p, tenant, event)
		if err != nil {
			return err
		}
		if row.Version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if row.Run == nil || row.Run.Status != liveops.StatusScheduled {
			return problem.New(problem.Conflict, "a school event changes only before it starts")
		}
		if merged, err = mergeSchoolEvent(row, in); err != nil {
			return err
		}
		if len(merged.SchoolIDs) == 0 && row.SchoolID != nil {
			merged.SchoolIDs = []uuid.UUID{*row.SchoolID} // the school the event was created for (not a parameter of the request)
		}
		if err := requireSchoolEvent(ctx, p, tenant, merged.SchoolIDs); err != nil {
			return err
		}
		if tpl, err = h.checkTemplate(ctx, tenant, merged); err != nil {
			return err
		}
		return h.checkSchools(ctx, tenant, merged.SchoolIDs)
	}); err != nil {
		return out, err
	}
	if err := h.screen(ctx, tenant, merged); err != nil {
		return out, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		row, err := h.Store.LockSchoolEvent(ctx, tenant, event)
		if err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "")
		}
		if row.Version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if row.Run == nil || row.Run.Status != liveops.StatusScheduled {
			return problem.New(problem.Conflict, "a school event changes only before it starts")
		}
		d := tpl.Template.Build(event, tenant, row.Run.Definition.Code, tpl.VersionNo, merged.params(), allGrades())
		if err := d.Validate(h.policy().Rules, 0); err != nil {
			return liveopsProblem(err)
		}
		if err := h.publishTenantVersion(ctx, tenant, d, row.Run.VersionNo+1, merged.Raw, p.UserID, now); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditSchoolEventUpdated, TargetType: "school_event", TargetID: event.String(),
			Result: audit.ResultSuccess}); err != nil {
			return err
		}
		updated, err := h.Store.LockSchoolEvent(ctx, tenant, event)
		if err != nil || updated == nil {
			return errors.Join(err, problem.New(problem.Internal, "school event not stored"))
		}
		out = *updated
		return idempotency.SetResult(ctx, port.SchoolEventView(out))
	})
	return out, err
}

// mergeSchoolEvent applies an update on the stored parameters.
func mergeSchoolEvent(row *port.SchoolEventRow, in SchoolEventInput) (SchoolEventInput, error) {
	raw := map[string]any{}
	for k, v := range row.Params {
		raw[k] = v
	}
	for k, v := range in.Raw {
		raw[k] = v
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return in, err
	}
	merged, err := ParseSchoolEventInput(b)
	if err != nil {
		return in, err
	}
	merged.Raw = raw
	return merged, nil
}

// ParseSchoolEventInput reads a SchoolEventCreate/Update body (Set: the fields present).
func ParseSchoolEventInput(body []byte) (SchoolEventInput, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return SchoolEventInput{}, problem.New(problem.BadRequest, "invalid JSON body")
	}
	var b struct {
		TemplateID  uuid.UUID         `json:"template_id"`
		Name        liveops.Localized `json:"name"`
		Description liveops.Localized `json:"description"`
		StartsAt    *time.Time        `json:"starts_at"`
		EndsAt      *time.Time        `json:"ends_at"`
		Audience    *struct {
			SchoolIDs   []uuid.UUID `json:"school_ids"`
			GradeLevels []int       `json:"grade_levels"`
		} `json:"audience"`
		GoalTarget   *int64      `json:"community_goal_target"`
		QuestIDs     []uuid.UUID `json:"quest_ids"`
		ChallengeIDs []uuid.UUID `json:"challenge_ids"`
		RewardOption *string     `json:"reward_option_id"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return SchoolEventInput{}, problem.New(problem.BadRequest, "invalid JSON body")
	}
	in := SchoolEventInput{TemplateID: b.TemplateID, Name: b.Name, Description: b.Description, StartsAt: b.StartsAt, EndsAt: b.EndsAt,
		GoalTarget: b.GoalTarget, QuestIDs: b.QuestIDs, ChallengeIDs: b.ChallengeIDs, RewardOption: b.RewardOption, Set: map[string]bool{}, Raw: raw}
	for _, k := range []string{"description", "community_goal_target", "quest_ids", "challenge_ids", "reward_option_id"} {
		if v, ok := raw[k]; ok && v != nil {
			in.Set[k] = true
		}
	}
	if b.Audience != nil {
		in.SchoolIDs, in.GradeLevels = b.Audience.SchoolIDs, b.Audience.GradeLevels
		if len(b.Audience.SchoolIDs) > 0 {
			in.Set[liveops.ParamSchools] = true
		}
		if len(b.Audience.GradeLevels) > 0 {
			in.Set[liveops.ParamGrades] = true
		}
	}
	return in, nil
}

func (h *LiveOps) visibleSchoolEvent(ctx context.Context, p authz.Principal, tenant, event uuid.UUID) (*port.SchoolEventRow, error) {
	if _, all := SchoolGrants(p); !all {
		if schools, _ := SchoolGrants(p); len(schools) == 0 {
			return nil, authz.Require(ctx, p, authz.PermLmsSchoolEventWrite, authz.Resource{TenantID: &tenant})
		}
	}
	row, err := h.Store.LockSchoolEvent(ctx, tenant, event)
	if err != nil {
		return nil, err
	}
	if row == nil || !SchoolEventVisible(p, row.SchoolID) {
		return nil, problem.New(problem.NotFound, "")
	}
	return row, nil
}

// CancelSchoolEvent is cancelSchoolEvent (F077-AC-24): a reason of 3..1000 characters, audit school_event.cancelled,
// event.cancelled; the run is archived by the scheduler after granting what students reached (kept, never taken back).
func (h *LiveOps) CancelSchoolEvent(ctx context.Context, event uuid.UUID, reason string) (out port.SchoolEventRow, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CancelSchoolEvent")
	defer func() { done(err) }()
	p, tenant, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	sc, err := tenantScope(ctx, tenant)
	if err != nil {
		return out, err
	}
	now := h.now()
	var run port.EventRunRow
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		row, err := h.visibleSchoolEvent(ctx, p, tenant, event)
		if err != nil {
			return err
		}
		if row.Run == nil {
			return problem.New(problem.Conflict, "the school event has no run")
		}
		r, err := h.Store.LockRun(ctx, row.Run.RunID)
		if err != nil || r == nil {
			return errors.Join(err, problem.New(problem.Conflict, "the run is busy"))
		}
		to, err := liveops.Operate(r.Status, liveops.ActionCancel, reason)
		if errors.Is(err, liveops.ErrInvalidTransition) {
			return problem.New(problem.Conflict, "the school event can no longer be cancelled")
		}
		if err != nil {
			return liveopsProblem(err)
		}
		by := p.UserID
		ok, err := h.Store.SetRunStatus(ctx, r.RunID, r.StatusVersion, to, &reason, &by, now)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.Conflict, "the run changed meanwhile")
		}
		if err := h.runEvent(ctx, *r, liveops.EventFor(r.Status, to), to, &reason, now); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: auditSchoolEventCancelled, TargetType: "school_event", TargetID: event.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"from": r.Status}}); err != nil {
			return err
		}
		run = *r
		run.Status = to
		updated, err := h.Store.LockSchoolEvent(ctx, tenant, event)
		if err != nil || updated == nil {
			return errors.Join(err, problem.New(problem.Internal, "school event not found"))
		}
		out = *updated
		return idempotency.SetResult(ctx, port.SchoolEventView(out))
	})
	if err != nil {
		return out, err
	}
	h.metric(func(m LiveOpsMetrics) { m.RunTransition(liveops.StatusCancelled) })
	h.afterRunChange(ctx, run, liveops.StatusCancelled, now)
	return out, nil
}
