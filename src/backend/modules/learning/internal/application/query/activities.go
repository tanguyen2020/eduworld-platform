package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ActivityTypes lists the types of the Activity Registry (listActivityTypes, F030-AC-22).
type ActivityTypes struct {
	Registry *activity.Registry
}

// List returns every registered type with its schema, strategies, min_client_version and status.
func (q ActivityTypes) List(ctx context.Context) (out []activity.Info, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListActivityTypes")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := authz.Require(ctx, p, authz.PermLearningContentRead, authz.Resource{}); err != nil {
		return nil, err
	}
	return q.Registry.Infos(), nil
}

// CmsActivityView is an activity with its newest version and the number of its published release.
type CmsActivityView struct {
	DocView
	CurrentPublishedVersion *int
}

// CmsActivities are the CMS reads of activities (getCmsActivity, listCmsActivities): the composed-content reads of
// F024 for the type activity plus the version number of the release pointer.
type CmsActivities struct{ Docs Docs }

// Get returns an activity with its newest version.
func (q CmsActivities) Get(ctx context.Context, id uuid.UUID) (CmsActivityView, error) {
	v, err := q.Docs.Get(ctx, contentmodel.TypeActivity, id)
	if err != nil {
		return CmsActivityView{}, err
	}
	out := CmsActivityView{DocView: v}
	err = q.Docs.Session.Read(ctx, func(ctx context.Context) error {
		out.CurrentPublishedVersion, err = currentNo(ctx, q.Docs.Model, v)
		return err
	})
	return out, err
}

// List lists activities whose newest version matches f.
func (q CmsActivities) List(ctx context.Context, f read.DocFilter, in PageInput) (Page[CmsActivityView], error) {
	p, err := q.Docs.List(ctx, contentmodel.TypeActivity, f, in)
	if err != nil {
		return Page[CmsActivityView]{}, err
	}
	out := Page[CmsActivityView]{NextCursor: p.NextCursor, HasMore: p.HasMore, Limit: p.Limit, Data: make([]CmsActivityView, len(p.Data))}
	err = q.Docs.Session.Read(ctx, func(ctx context.Context) error {
		for i, v := range p.Data {
			no, err := currentNo(ctx, q.Docs.Model, v)
			if err != nil {
				return err
			}
			out.Data[i] = CmsActivityView{DocView: v, CurrentPublishedVersion: no}
		}
		return nil
	})
	return out, err
}

// currentNo finds the number of the version the release pointer of an item designates (nil when unpublished).
func currentNo(ctx context.Context, m read.DocReadModel, v DocView) (*int, error) {
	cur := v.Doc.CurrentVersionID
	if cur == nil {
		return nil, nil
	}
	if *cur == v.Version.ID {
		no := v.Version.VersionNo
		return &no, nil
	}
	for no := v.Version.VersionNo - 1; no >= 1; no-- {
		pv, err := m.Version(ctx, contentmodel.TypeActivity, v.Doc.ID, no)
		if err != nil {
			return nil, err
		}
		if pv.ID == *cur {
			return &no, nil
		}
	}
	return nil, nil
}
