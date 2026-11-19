package command

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DocResource is the ResourceHandler of a composed content type (F024 T007, US6): trash, restore, owners and clone.
// A question set clone also clones the tenant questions it references (new ids, drafts) and keeps the platform ones
// (F024-AC-22); student data is never copied (R22).
type DocResource struct {
	Type      contentmodel.ContentType
	Store     write.DocStore
	Questions QuestionResource // question sets: the cloner of referenced tenant questions
}

var _ port.ResourceHandler = DocResource{}

// DocResources returns the handlers of the four types.
func DocResources(store write.DocStore, questions write.QuestionStore) []port.ResourceHandler {
	out := make([]port.ResourceHandler, len(DocTypes))
	for i, t := range DocTypes {
		out[i] = DocResource{Type: t, Store: store, Questions: QuestionResource{Store: questions}}
	}
	return out
}

// ResourceType implements port.ResourceHandler.
func (r DocResource) ResourceType() contentmodel.ResourceType {
	rt, _ := contentmodel.ResourceOf(r.Type)
	return rt
}

// WritePermission implements port.ResourceHandler.
func (DocResource) WritePermission() authz.Permission { return authz.PermLearningContentWrite }

// Owners implements port.ResourceHandler.
func (r DocResource) Owners(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error) {
	out := map[uuid.UUID]contentmodel.Owner{}
	for _, id := range idList {
		d, err := r.Store.Doc(ctx, r.Type, id)
		if err == nil {
			out[id] = d.Owner
			continue
		}
		if pe, ok := problemOf(err); !ok || pe.Code != problem.NotFound {
			return nil, err
		}
	}
	return out, nil
}

// SoftDelete implements port.ResourceHandler.
func (r DocResource) SoftDelete(ctx context.Context, id uuid.UUID, reason string) (port.Deleted, error) {
	d, err := r.Store.Doc(ctx, r.Type, id)
	if err != nil {
		return port.Deleted{}, err
	}
	if err := contentmodel.CanDeleteItem(d.CurrentVersionID); err != nil {
		return port.Deleted{}, err
	}
	if err := r.Store.SoftDeleteDoc(ctx, r.Type, id, reason); err != nil {
		return port.Deleted{}, err
	}
	return port.Deleted{Owner: d.Owner, Title: string(r.Type) + " " + id.String()}, nil
}

// ListDeleted implements port.ResourceHandler.
func (r DocResource) ListDeleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	return r.Store.DeletedDocs(ctx, r.Type, f)
}

// Restore implements port.ResourceHandler.
func (r DocResource) Restore(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	return r.Store.RestoreDoc(ctx, r.Type, id)
}

// Clone implements port.ResourceHandler (ctx carries tenancy.Bypass(tenant_clone)): the latest version becomes
// version 1 draft of the target tenant.
func (r DocResource) Clone(ctx context.Context, in port.CloneInput) (uuid.UUID, error) {
	src, err := r.Store.Doc(ctx, r.Type, in.SourceID)
	if err != nil || src.Owner.TenantID == nil || *src.Owner.TenantID != in.SourceTenant {
		return uuid.Nil, port.ErrNotInSourceTenant
	}
	vs, err := r.Store.DocVersions(ctx, r.Type, src.ID)
	if err != nil {
		return uuid.Nil, err
	}
	target, srcTenant, at, by := in.TargetTenant, in.SourceTenant, in.At, in.Actor
	o := contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &target}
	d := dto.Doc{Type: r.Type, ID: ids.New(), Owner: o, ClonedFromID: &src.ID, ClonedFromTenantID: &srcTenant, ClonedAt: &at, ClonedBy: &by}
	if len(vs) > 0 {
		d.LatestVersionNo = 1
	}
	if err := r.Store.InsertDoc(ctx, d); err != nil {
		return uuid.Nil, err
	}
	if len(vs) == 0 {
		return d.ID, nil
	}
	v := vs[0]
	latest := v.ID
	v.ID, v.ItemID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID, v.Owner =
		ids.New(), d.ID, 1, contentmodel.StatusDraft, in.Actor, nil, nil, &latest, o
	if r.Type == contentmodel.TypeQuestionSet {
		if err := r.cloneItems(ctx, in, &v); err != nil {
			return uuid.Nil, err
		}
	}
	return d.ID, r.Store.InsertDocVersion(ctx, v)
}

// cloneItems clones the source-tenant questions of a set version and points the copied items (and the definition)
// to the new question versions; platform questions are kept.
func (r DocResource) cloneItems(ctx context.Context, in port.CloneInput, v *dto.DocVersion) error {
	remap := map[uuid.UUID]uuid.UUID{}
	for i, it := range v.Items {
		qv, err := r.Questions.Store.QuestionVersionByID(ctx, it.QuestionVersionID)
		if err != nil {
			return err
		}
		if qv.Owner.Platform() {
			continue
		}
		newQ, ok := in.IDMap[it.QuestionID]
		if !ok {
			newQ, err = r.Questions.Clone(ctx, port.CloneInput{JobID: in.JobID, SourceID: it.QuestionID, SourceTenant: in.SourceTenant,
				TargetTenant: in.TargetTenant, Actor: in.Actor, At: in.At, IDMap: in.IDMap})
			if err != nil {
				return err
			}
			if in.IDMap != nil {
				in.IDMap[it.QuestionID] = newQ
			}
		}
		nv, err := r.Questions.Store.QuestionVersion(ctx, newQ, 1)
		if err != nil {
			return err
		}
		remap[it.QuestionVersionID] = nv.ID
		v.Items[i].QuestionID, v.Items[i].QuestionVersionID = newQ, nv.ID
	}
	if len(remap) == 0 {
		return nil
	}
	body := bodyOf(*v)
	var def map[string]any
	if err := json.Unmarshal(body["definition"], &def); err != nil {
		return err
	}
	if items, ok := def["items"].([]any); ok {
		for _, x := range items {
			if m, ok := x.(map[string]any); ok {
				if s, ok := m["question_version_id"].(string); ok {
					if id, err := uuid.Parse(s); err == nil {
						if n, ok := remap[id]; ok {
							m["question_version_id"] = n.String()
						}
					}
				}
			}
		}
	}
	b, err := json.Marshal(def)
	if err != nil {
		return err
	}
	body["definition"] = b
	v.Body, err = json.Marshal(body)
	return err
}
