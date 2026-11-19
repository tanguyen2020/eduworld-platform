package query

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DocMetrics observes the CMS search and the pool stats (eduworld_cms_search_duration_seconds is the use case
// duration; eduworld_pool_stats_insufficient_total counts insufficient rules); nil = not counted.
type DocMetrics interface {
	PoolInsufficient(n int)
}

// Docs is the read side of the composed content types (F024, learning_content.read): lists with full-text search
// and filters, details, pool stats.
type Docs struct {
	Session read.Session
	Model   read.DocReadModel
	Pool    read.DeliveryReader
	Cursors *page.Codec
	Metrics DocMetrics
}

// DocView is an item with one version.
type DocView struct {
	Doc     dto.Doc
	Version dto.DocVersion
}

func (q Docs) authorize(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	return p, authz.Require(ctx, p, authz.PermLearningContentRead, authz.Resource{})
}

// Get returns an item with its newest version.
func (q Docs) Get(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (out DocView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.Get."+string(t))
	defer func() { done(err) }()
	if _, err := q.authorize(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		d, err := q.Model.Doc(ctx, t, id)
		if err != nil {
			return err
		}
		v, err := q.Model.Latest(ctx, t, id)
		out = DocView{Doc: d, Version: v}
		return err
	})
	return out, err
}

// MaxQueryLength is the longest q of a CMS search (F024-AC-19).
const MaxQueryLength = 200

// List pages the items of a type (F024-AC-17, AC-18): q full-text without accents, filters, scope of the session.
func (q Docs) List(ctx context.Context, t contentmodel.ContentType, f read.DocFilter, in PageInput) (out Page[DocView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.cms.search."+string(t))
	defer func() { done(err) }()
	if _, err := q.authorize(ctx); err != nil {
		return out, err
	}
	if utf8.RuneCountInString(f.Q) > MaxQueryLength {
		return out, problem.Invalid("q", problem.SubMaxLength, "is longer than 200 characters")
	}
	limit := page.Limit(in.Limit)
	cur, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur != nil {
		f.After = &cur.ID
	}
	f.Limit = limit + 1
	var hits []read.DocHit
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		hits, err = q.Model.Search(ctx, t, f)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[DocView]{Limit: limit, Data: []DocView{}}
	if len(hits) > limit {
		out.HasMore, hits = true, hits[:limit]
		s := q.Cursors.Encode(page.Cursor{ID: hits[limit-1].Doc.ID})
		out.NextCursor = &s
	}
	for _, h := range hits {
		out.Data = append(out.Data, DocView{Doc: h.Doc, Version: h.Version})
	}
	return out, nil
}

// PoolStats returns the pool of every rule of a question set or blueprint version (F024-AC-04, AC-11): current
// releases visible to the scope.
func (q Docs) PoolStats(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, versionNo int) (out []questionset.PoolStat, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.PoolStats."+string(t))
	defer func() { done(err) }()
	if _, err := q.authorize(ctx); err != nil {
		return nil, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		v, err := q.Model.Version(ctx, t, id, versionNo)
		if err != nil {
			return err
		}
		body := map[string]json.RawMessage{}
		_ = json.Unmarshal(v.Body, &body)
		pool, err := q.Pool.Pool(ctx, read.PoolFilter{SubjectID: v.SubjectID})
		if err != nil {
			return err
		}
		switch t {
		case contentmodel.TypeQuestionSet:
			def, err := questionset.Parse(body["definition"])
			if err != nil {
				return err
			}
			out = def.Stats(pool)
		case contentmodel.TypeAssessmentBlueprint:
			def, err := blueprint.Parse(body["definition"])
			if err != nil {
				return err
			}
			out = def.Stats(pool)
		default:
			return problem.New(problem.NotFound, "")
		}
		return nil
	})
	if out == nil {
		out = []questionset.PoolStat{}
	}
	if q.Metrics != nil {
		n := 0
		for _, s := range out {
			if s.Insufficient {
				n++
			}
		}
		q.Metrics.PoolInsufficient(n)
	}
	return out, err
}
