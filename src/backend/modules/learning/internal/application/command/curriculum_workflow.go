package command

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	pobs "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// NewVersionInput is the request of createCurriculumVersion.
type NewVersionInput struct {
	CurriculumID  uuid.UUID
	FromVersion   int
	Label         string
	EffectiveFrom *time.Time
	EffectiveTo   *time.Time
	ChangeNote    *string
}

// CreateVersion deep-copies a version into a new draft (F021-AC-17): subject × grade pairs, topics, LOs and
// competencies get new ids and keep code and stable_key; supersedes_version_id points to the source; refused
// with 409 CONFLICT while the curriculum has a draft or in_review version.
func (h Curriculum) CreateVersion(ctx context.Context, in NewVersionInput) (out dto.CurriculumVersion, err error) {
	ctx, done := pobs.UseCase(ctx, "learning.command.CreateCurriculumVersion")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, in.CurriculumID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermCurriculumWrite, c.Owner); err != nil {
			return err
		}
		vs, err := h.Store.Versions(ctx, c.ID)
		if err != nil {
			return err
		}
		var src *dto.CurriculumVersion
		maxNo := 0
		for i, v := range vs {
			if curriculum.Open(v.Status) {
				return curriculumErr(curriculum.ErrOpenDraft)
			}
			if v.VersionNo == in.FromVersion {
				src = &vs[i]
			}
			maxNo = max(maxNo, v.VersionNo)
		}
		if src == nil {
			return problem.New(problem.NotFound, "the source version does not exist")
		}
		vin := VersionInput{Label: in.Label, EffectiveFrom: in.EffectiveFrom, EffectiveTo: in.EffectiveTo, ChangeNote: in.ChangeNote,
			ContentLanguage: src.ContentLanguage}
		if err := vin.validate(); err != nil {
			return err
		}
		srcID := src.ID
		v := dto.CurriculumVersion{ID: ids.New(), CurriculumID: c.ID, VersionNo: max(maxNo, c.LatestVersionNo) + 1, Label: in.Label,
			Status: contentmodel.StatusDraft, EffectiveFrom: in.EffectiveFrom, EffectiveTo: in.EffectiveTo, SupersedesID: &srcID, BasedOnID: &srcID,
			ChangeNote: in.ChangeNote, ContentLanguage: src.ContentLanguage, AuthorID: p.UserID, Owner: c.Owner}
		if err := h.Store.InsertVersion(ctx, v); err != nil {
			return err
		}
		if err := h.copyStructure(ctx, *src, v); err != nil {
			return err
		}
		c.LatestVersionNo = v.VersionNo
		if _, err := h.Store.UpdateCurriculum(ctx, c, c.RowVersion); err != nil {
			return err
		}
		if err := h.recordCurriculum(ctx, c.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: c.ID.String(),
			Metadata: map[string]any{"curriculum_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String()}}); err != nil {
			return err
		}
		if out, err = h.Store.VersionByID(ctx, v.ID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.VersionJSON(out))
	})
	return out, err
}

// copyStructure copies the content of version src into the new draft dst (ids remapped).
func (h Curriculum) copyStructure(ctx context.Context, src, dst dto.CurriculumVersion) error {
	sgs, err := h.Store.SubjectGrades(ctx, src.ID)
	if err != nil {
		return err
	}
	for _, sg := range sgs {
		sg.ID = ids.New()
		if err := h.Store.InsertSubjectGrade(ctx, dst, sg); err != nil {
			return err
		}
	}
	comps, err := h.Store.Competencies(ctx, src.ID)
	if err != nil {
		return err
	}
	compIDs := map[uuid.UUID]uuid.UUID{}
	for _, c := range comps {
		old := c.ID
		c.ID, c.VersionID, c.CurriculumID, c.Owner = ids.New(), dst.ID, dst.CurriculumID, dst.Owner
		compIDs[old] = c.ID
		if err := h.Store.InsertCompetency(ctx, c); err != nil {
			return err
		}
	}
	topics, err := h.Store.Topics(ctx, src.ID)
	if err != nil {
		return err
	}
	topicIDs := map[uuid.UUID]uuid.UUID{}
	for _, t := range topics {
		topicIDs[t.ID] = ids.New()
	}
	for _, t := range parentsFirst(topics) {
		t.ID, t.VersionID, t.CurriculumID, t.Owner = topicIDs[t.ID], dst.ID, dst.CurriculumID, dst.Owner
		if t.ParentID != nil {
			np := topicIDs[*t.ParentID]
			t.ParentID = &np
		}
		if err := h.Store.InsertTopic(ctx, t); err != nil {
			return err
		}
	}
	los, err := h.Store.Objectives(ctx, src.ID)
	if err != nil {
		return err
	}
	loIDs := map[uuid.UUID]uuid.UUID{}
	for _, o := range los {
		loIDs[o.ID] = ids.New()
	}
	remap := func(in []uuid.UUID, m map[uuid.UUID]uuid.UUID) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(in))
		for _, id := range in {
			if n, ok := m[id]; ok {
				out = append(out, n)
			}
		}
		return out
	}
	for _, o := range los {
		o.ID, o.VersionID, o.CurriculumID, o.Owner, o.TopicID = loIDs[o.ID], dst.ID, dst.CurriculumID, dst.Owner, topicIDs[o.TopicID]
		o.CompetencyIDs, o.PrerequisiteIDs = remap(o.CompetencyIDs, compIDs), remap(o.PrerequisiteIDs, loIDs)
		if err := h.Store.InsertObjective(ctx, o); err != nil {
			return err
		}
	}
	return nil
}

// parentsFirst orders topics so every parent precedes its children (FK parent_topic_id).
func parentsFirst(ts []dto.Topic) []dto.Topic {
	byID := map[uuid.UUID]dto.Topic{}
	for _, t := range ts {
		byID[t.ID] = t
	}
	depth := func(t dto.Topic) int {
		d := 0
		for cur := t.ParentID; cur != nil && d <= len(ts); d++ {
			p, ok := byID[*cur]
			if !ok {
				break
			}
			cur = p.ParentID
		}
		return d
	}
	out := slices.Clone(ts)
	slices.SortStableFunc(out, func(a, b dto.Topic) int { return depth(a) - depth(b) })
	return out
}

// TransitionInput is the request of transitionCurriculumVersion.
type TransitionInput struct {
	CurriculumID uuid.UUID
	VersionNo    int
	IfMatch      int64
	Action       string
	Note         string
}

var transitionAudit = map[contentmodel.Action]string{
	contentmodel.ActionSubmitReview: ActionContentSubmitted,
	contentmodel.ActionApprove:      ActionContentApproved,
	contentmodel.ActionReject:       ActionContentRejected,
	contentmodel.ActionRevise:       ActionContentRevised,
	contentmodel.ActionArchive:      ActionContentArchived,
}

// Transition applies a workflow action to a version (F021-AC-13, AC-18): submit_review and revise need
// curriculum.write, approve, reject and archive curriculum.publish; the reviewer differs from the author, reject
// needs a note; a wrong transition is 422 INVALID_STATE_TRANSITION. Every transition is written to
// content.content_transitions and audited.
func (h Curriculum) Transition(ctx context.Context, in TransitionInput) (out dto.CurriculumVersion, err error) {
	ctx, done := pobs.UseCase(ctx, "learning.command.TransitionCurriculumVersion")
	defer func() { done(err) }()
	action := contentmodel.Action(in.Action)
	perm := authz.PermCurriculumPublish
	switch action {
	case contentmodel.ActionSubmitReview, contentmodel.ActionRevise:
		perm = authz.PermCurriculumWrite
	case contentmodel.ActionApprove, contentmodel.ActionReject, contentmodel.ActionArchive:
	default:
		return out, problem.Invalid("action", problem.SubInvalid, "must be submit_review, approve, reject, revise or archive")
	}
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, in.CurriculumID)
		if err != nil {
			return err
		}
		v, err := h.Store.Version(ctx, c.ID, in.VersionNo)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, perm, v.Owner); err != nil {
			return err
		}
		if err := checkRowVersion(v.RowVersion, in.IfMatch); err != nil {
			return err
		}
		from := v.Status
		to, err := contentmodel.Transition(from, action, contentmodel.TransitionInput{Actor: p.UserID, Author: v.AuthorID, Note: in.Note,
			IsCurrent: c.CurrentVersionID != nil && *c.CurrentVersionID == v.ID})
		if err != nil {
			return curriculumErr(err)
		}
		var warnings []curriculum.Warning
		if action == contentmodel.ActionSubmitReview {
			if warnings, err = h.overlapWarnings(ctx, v); err != nil {
				return err
			}
		}
		if action == contentmodel.ActionApprove {
			v.ReviewerID = &p.UserID
		}
		v.Status = to
		if out, err = h.Store.UpdateVersion(ctx, v, in.IfMatch); err != nil {
			return err
		}
		if err := h.Store.RecordTransition(ctx, out, from, to, p.UserID, in.Note); err != nil {
			return err
		}
		if err := h.recordCurriculum(ctx, v.Owner, audit.Event{Action: transitionAudit[action], TargetID: c.ID.String(), Reason: truncate(in.Note, 500),
			Metadata: map[string]any{"curriculum_version_id": v.ID.String(), "from_status": string(from), "to_status": string(to)}}); err != nil {
			return err
		}
		out.IsCurrent = c.CurrentVersionID != nil && *c.CurrentVersionID == out.ID
		out.Warnings = warnings
		return idempotency.SetResult(ctx, dto.VersionJSON(out))
	})
	return out, err
}

// overlapWarnings warns when the effective range of v overlaps a published or open version of the same curriculum
// that shares a subject × grade (spec edge case; never blocks).
func (h Curriculum) overlapWarnings(ctx context.Context, v dto.CurriculumVersion) ([]curriculum.Warning, error) {
	vs, err := h.Store.Versions(ctx, v.CurriculumID)
	if err != nil {
		return nil, err
	}
	pairs := map[string]bool{}
	for _, sg := range v.SubjectGrades {
		pairs[sg.SubjectID.String()+"/"+strconv.Itoa(sg.GradeLevel)] = true
	}
	var others []curriculum.Range
	for _, o := range vs {
		if o.ID == v.ID || (o.Status != contentmodel.StatusPublished && !curriculum.Open(o.Status) && o.Status != contentmodel.StatusApproved) {
			continue
		}
		sgs, err := h.Store.SubjectGrades(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(sgs, func(sg dto.SubjectGrade) bool { return pairs[sg.SubjectID.String()+"/"+strconv.Itoa(sg.GradeLevel)] }) {
			others = append(others, curriculum.Range{VersionID: o.ID, From: o.EffectiveFrom, To: o.EffectiveTo})
		}
	}
	return curriculum.OverlapWarnings(curriculum.Range{VersionID: v.ID, From: v.EffectiveFrom, To: v.EffectiveTo}, others), nil
}

// PublishInput is the request of publishCurriculumVersion.
type PublishInput struct {
	CurriculumID uuid.UUID
	VersionNo    int
	IfMatch      int64
}

// VersionPublishedPayload is the payload of curriculum.version_published (v1).
type VersionPublishedPayload struct {
	CurriculumID        uuid.UUID            `json:"curriculum_id"`
	CurriculumCode      string               `json:"curriculum_code"`
	CurriculumVersionID uuid.UUID            `json:"curriculum_version_id"`
	Version             int                  `json:"version"`
	VersionLabel        string               `json:"version_label"`
	SupersedesVersionID *uuid.UUID           `json:"supersedes_version_id"`
	FromVersionID       *uuid.UUID           `json:"from_version_id"`
	OwnerScope          string               `json:"owner_scope"`
	TenantID            *uuid.UUID           `json:"tenant_id"`
	ReleaseID           uuid.UUID            `json:"release_id"`
	Mappings            []curriculum.Mapping `json:"mappings"`
	PublishedBy         uuid.UUID            `json:"published_by"`
}

// Publish publishes an approved version in one transaction (F021-AC-15): status published, release pointer
// current_version_id, content.content_releases (item_type curriculum), transition, audit content.published and
// outbox curriculum.version_published with the LO mapping table (AC-27). Refused with 422 when GDPT2018 has LOs
// without yccd_ref (AC-14) or an LO of the superseded version has no mapping (AC-26). The Idempotency-Key of the
// request replays the stored response.
func (h Curriculum) Publish(ctx context.Context, in PublishInput) (out dto.CurriculumVersion, err error) {
	ctx, done := pobs.UseCase(ctx, "learning.command.PublishCurriculumVersion")
	defer func() {
		h.countPublish(err)
		done(err)
	}()
	p, err := authorize(ctx, authz.PermCurriculumPublish)
	if err != nil {
		return out, err
	}
	key := ids.New().String()
	if pend, ok := idempotency.PendingFrom(ctx); ok {
		key = pend.Key.Key.String()
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Curriculum(ctx, in.CurriculumID)
		if err != nil {
			return err
		}
		v, err := h.Store.Version(ctx, c.ID, in.VersionNo)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermCurriculumPublish, v.Owner); err != nil {
			return err
		}
		if err := checkRowVersion(v.RowVersion, in.IfMatch); err != nil {
			return err
		}
		out, err = h.publish(ctx, p.UserID, c, v, key)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.VersionJSON(out))
	})
	return out, err
}

// publish runs the publish of an approved version (shared with the seed).
func (h Curriculum) publish(ctx context.Context, actor uuid.UUID, c dto.Curriculum, v dto.CurriculumVersion, key string) (dto.CurriculumVersion, error) {
	to, err := contentmodel.Transition(v.Status, contentmodel.ActionPublish, contentmodel.TransitionInput{})
	if err != nil {
		return v, curriculumErr(err)
	}
	los, err := h.Store.Objectives(ctx, v.ID)
	if err != nil {
		return v, err
	}
	refs := make([]curriculum.ObjectiveRef, len(los))
	for i, lo := range los {
		refs[i] = curriculum.ObjectiveRef{ID: lo.ID, Code: lo.Code, YccdRef: deref(lo.YccdRef)}
	}
	if missing := curriculum.MissingYccd(c.Code, refs); len(missing) > 0 {
		return v, problem.Invalid("yccd_ref", problem.SubRequired, "objectives without yccd_ref: "+strings.Join(missing, ", ")).
			With("objective_codes", missing)
	}
	var mappings curriculum.MappingSet
	if v.SupersedesID != nil {
		old, err := h.Store.Objectives(ctx, *v.SupersedesID)
		if err != nil {
			return v, err
		}
		if mappings, err = h.Store.Mappings(ctx, v.ID); err != nil {
			return v, err
		}
		oldIDs := make([]uuid.UUID, len(old))
		codes := map[uuid.UUID]string{}
		for i, o := range old {
			oldIDs[i], codes[o.ID] = o.ID, o.Code
		}
		if un := mappings.Uncovered(oldIDs); len(un) > 0 {
			names := make([]string, len(un))
			for i, id := range un {
				names[i] = codes[id]
			}
			slices.Sort(names)
			return v, problem.Invalid("lo_mappings", problem.SubRequired, "objectives of the superseded version without mapping: "+strings.Join(names, ", ")).
				With("unmapped_lo_ids", un)
		}
	}
	at := h.now()
	from := v.Status
	v.Status, v.PublishedAt = to, &at
	out, err := h.Store.UpdateVersion(ctx, v, v.RowVersion)
	if err != nil {
		return v, err
	}
	prev := c.CurrentVersionID
	c.CurrentVersionID = &out.ID
	if _, err := h.Store.UpdateCurriculum(ctx, c, c.RowVersion); err != nil {
		return out, err
	}
	release, err := h.Store.RecordRelease(ctx, write.Release{Owner: c.Owner, ItemType: string(contentmodel.TypeCurriculum), ItemID: c.ID,
		FromVersionID: prev, ToVersionID: out.ID, ReleasedBy: actor, ReleasedAt: at, IdempotencyKey: key})
	if err != nil {
		return out, err
	}
	if err := h.Store.RecordTransition(ctx, out, from, to, actor, ""); err != nil {
		return out, err
	}
	if err := h.recordCurriculum(ctx, c.Owner, audit.Event{Action: ActionContentPublished, TargetID: c.ID.String(),
		Metadata: map[string]any{"curriculum_version_id": out.ID.String(), "version": out.VersionNo, "release_id": release.String()}}); err != nil {
		return out, err
	}
	if mappings == nil {
		mappings = curriculum.MappingSet{}
	}
	payload := VersionPublishedPayload{CurriculumID: c.ID, CurriculumCode: c.Code, CurriculumVersionID: out.ID, Version: out.VersionNo,
		VersionLabel: out.Label, SupersedesVersionID: out.SupersedesID, FromVersionID: prev, OwnerScope: string(c.Owner.Scope), TenantID: c.Owner.TenantID,
		ReleaseID: release, Mappings: mappings, PublishedBy: actor}
	e := write.Event{Type: EventCurriculumVersionPublished, SubjectType: string(contentmodel.ResourceCurriculum), SubjectID: out.ID.String(), Payload: payload}
	if c.Owner.TenantID == nil {
		e.Platform = true
	} else {
		e.TenantID = ptrClone(c.Owner.TenantID)
	}
	if err := h.Events.Publish(ctx, e); err != nil {
		return out, err
	}
	out.IsCurrent = true
	return out, nil
}

// LoMappingsInput is the request of putCurriculumVersionLoMappings.
type LoMappingsInput struct {
	CurriculumID uuid.UUID
	VersionNo    int
	IfMatch      int64
	Items        curriculum.MappingSet
}

// LoMappings is the mapping table of a version and the LOs of the superseded version still without mapping.
type LoMappings struct {
	Version  dto.CurriculumVersion
	Items    curriculum.MappingSet
	Unmapped []uuid.UUID
}

// View returns the JSON view.
func (m LoMappings) View() dto.LoMappingListView {
	items, un := m.Items, m.Unmapped
	if items == nil {
		items = curriculum.MappingSet{}
	}
	if un == nil {
		un = []uuid.UUID{}
	}
	return dto.LoMappingListView{CurriculumVersionID: m.Version.ID, SupersedesVersionID: m.Version.SupersedesID, Items: items, UnmappedLoIDs: un}
}

// PutLoMappings replaces the LO mapping table of a draft version that supersedes another (F021-AC-26).
func (h Curriculum) PutLoMappings(ctx context.Context, in LoMappingsInput) (out LoMappings, err error) {
	ctx, done := pobs.UseCase(ctx, "learning.command.PutLoMappings")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		v, err := h.Store.Version(ctx, in.CurriculumID, in.VersionNo)
		if err != nil {
			return err
		}
		if v, err = h.draftVersion(ctx, p, v.ID); err != nil {
			return err
		}
		if err := checkRowVersion(v.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if v.SupersedesID == nil {
			return problem.Invalid("curriculum_version_id", problem.SubInvalid, "the version supersedes no version")
		}
		old, err := h.Store.Objectives(ctx, *v.SupersedesID)
		if err != nil {
			return err
		}
		cur, err := h.Store.Objectives(ctx, v.ID)
		if err != nil {
			return err
		}
		oldSet, curSet := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
		oldIDs := make([]uuid.UUID, len(old))
		for i, o := range old {
			oldSet[o.ID], oldIDs[i] = true, o.ID
		}
		for _, o := range cur {
			curSet[o.ID] = true
		}
		if err := in.Items.Validate(oldSet, curSet); err != nil {
			return invalid(err)
		}
		if err := h.Store.ReplaceMappings(ctx, v, in.Items); err != nil {
			return err
		}
		out = LoMappings{Version: v, Items: in.Items, Unmapped: in.Items.Uncovered(oldIDs)}
		return nil
	})
	return out, err
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (h Curriculum) countPublish(err error) {
	if h.Metrics == nil {
		return
	}
	var pe *problem.Error
	switch {
	case err == nil:
		h.Metrics.Published("success")
	case errors.As(err, &pe) && (pe.Code == problem.ValidationFailed || pe.Code == problem.InvalidStateTransition):
		h.Metrics.Published("rejected")
	default:
		h.Metrics.Published("error")
	}
}
