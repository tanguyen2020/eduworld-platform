package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/params"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// QuestionView is a question with one version as the CMS sees it (answer key, option feedback merged back, hints).
type QuestionView struct {
	Question dto.Question
	Version  dto.QuestionVersion
	View     dto.QuestionAuthoringView
}

func viewOf(p authz.Principal, q dto.Question, v dto.QuestionVersion) QuestionView {
	var fb map[string]question.OptionFeedback
	if len(v.OptionFeedback) > 0 && string(v.OptionFeedback) != "null" {
		_ = json.Unmarshal(v.OptionFeedback, &fb)
	}
	render, err := question.MergeFeedback(v.RenderPayload, fb)
	if err != nil {
		render = v.RenderPayload
	}
	return QuestionView{Question: q, Version: v, View: dto.QuestionJSON(q, v, render, p.TenantID)}
}

// Questions is the CMS read side of the question bank (F022, learning_content.read).
type Questions struct {
	Session read.Session
	Model   read.QuestionReadModel
	Cursors *page.Codec
}

func (q Questions) cms(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	return p, authz.Require(ctx, p, authz.PermLearningContentRead, authz.Resource{})
}

// GetQuestion returns a question with its newest version, answer key included (F022-AC-10).
func (q Questions) GetQuestion(ctx context.Context, id uuid.UUID) (out QuestionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetCmsQuestion")
	defer func() { done(err) }()
	p, err := q.cms(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		qu, err := q.Model.Question(ctx, id)
		if err != nil {
			return err
		}
		v, err := q.Model.Latest(ctx, id)
		if err != nil {
			return err
		}
		out = viewOf(p, qu, v)
		return nil
	})
	return out, err
}

// GetQuestionVersion returns one version of a visible question.
func (q Questions) GetQuestionVersion(ctx context.Context, id uuid.UUID, versionNo int) (out QuestionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetCmsQuestionVersion")
	defer func() { done(err) }()
	p, err := q.cms(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		qu, err := q.Model.Question(ctx, id)
		if err != nil {
			return err
		}
		v, err := q.Model.Version(ctx, id, versionNo)
		if err != nil {
			return err
		}
		out = viewOf(p, qu, v)
		return nil
	})
	return out, err
}

// Search lists the newest version of each visible question matching f (F022-AC-13): source own/assigned for tenant
// users, platform for platform staff; System Admin reads the tenants of tenant_ids (CrossTenantScope of httpx).
func (q Questions) Search(ctx context.Context, f read.QuestionFilter, in PageInput) (out Page[QuestionView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.SearchQuestions")
	defer func() { done(err) }()
	p, err := q.cms(ctx)
	if err != nil {
		return out, err
	}
	switch f.Source {
	case "", "own", "assigned", "platform":
	default:
		return out, problem.Invalid("source", problem.SubInvalid, "must be own, assigned or platform")
	}
	if f.Source == "platform" && p.TenantID != nil {
		return out, problem.Invalid("source", problem.SubInvalid, "platform is for platform staff; tenants read assigned questions")
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		hits, err := q.Model.Search(ctx, f)
		if err != nil {
			return err
		}
		views := make([]QuestionView, len(hits))
		for i, h := range hits {
			views[i] = viewOf(p, h.Question, h.Version)
		}
		out, err = paginate(views, func(v QuestionView) uuid.UUID { return v.Question.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// Delivery implements the port Delivery of the question bank (QUESTION_BANK §1) for the learning use cases of
// F030 and the question plans of F105: Select, Render, Grade and Explanation, in the tenant scope of the student.
// The pool and the render data come from content.question_delivery_v; only Grade reads the answer key.
type Delivery struct {
	Session  read.Session
	Reader   read.DeliveryReader
	Cache    read.PoolCache
	Registry *question.Registry
	Schema   port.QuestionSchema
	Metrics  DeliveryMetrics
}

// DeliveryMetrics counts grades and selections (eduworld_qb_grade_total, eduworld_qb_grade_invalid_response_total,
// eduworld_qb_select_pool_insufficient_total, eduworld_qb_pool_cache_hit_ratio); nil = not counted.
type DeliveryMetrics interface {
	Graded(questionType, result string)
	// IntegrityMismatch counts a forged interactive response (F023); ParamSamplingFailed a failed sampling.
	IntegrityMismatch(widget string)
	ParamSamplingFailed()
	PoolInsufficient()
	PoolCache(hit bool)
}

func sessionTenant(ctx context.Context) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Internal, "delivery needs the tenant scope of the student")
	}
	id, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.Internal, "delivery needs one session tenant")
	}
	return id, nil
}

func poolKey(f read.PoolFilter) string {
	s := "all"
	if f.SubjectID != nil {
		s = f.SubjectID.String()
	}
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// Pool returns the current releases visible to the tenant of ctx (own or assigned), of one subject or of every subject
// (cached in Valkey ew:learning:qb:pool:<tenant_id>:<filter_hash>, QUESTION_BANK §6); used by the selections of F032
// (siblings, interleaving, diagnostic) that pick from the pool themselves.
func (d Delivery) Pool(ctx context.Context, subjectID *uuid.UUID) (pool []selection.Candidate, err error) {
	tenant, err := sessionTenant(ctx)
	if err != nil {
		return nil, err
	}
	f := read.PoolFilter{SubjectID: subjectID}
	key := poolKey(f)
	hit := false
	if d.Cache != nil {
		pool, hit = d.Cache.Get(ctx, tenant, key)
	}
	if d.Metrics != nil {
		d.Metrics.PoolCache(hit)
	}
	if hit {
		return pool, nil
	}
	if err := d.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		pool, err = d.Reader.Pool(ctx, f)
		return err
	}); err != nil {
		return nil, err
	}
	if d.Cache != nil {
		d.Cache.Put(ctx, tenant, key, pool)
	}
	return pool, nil
}

// Select picks questions by rule for the tenant of ctx (F022-AC-26, AC-27): only current releases visible to the
// tenant (own or assigned), reproducible by seed; QUESTION_POOL_INSUFFICIENT when the relaxed rule cannot be met.
func (d Delivery) Select(ctx context.Context, req selection.Request) (out []selection.Candidate, err error) {
	ctx, done := observability.UseCase(ctx, "learning.qb.select")
	defer func() { done(err) }()
	pool, err := d.Pool(ctx, req.Rule.Filters.SubjectID)
	if err != nil {
		return nil, err
	}
	out, err = selection.Select(pool, req)
	if errors.Is(err, selection.ErrPoolInsufficient) {
		if d.Metrics != nil {
			d.Metrics.PoolInsufficient()
		}
		return nil, problem.New(problem.QuestionPoolInsufficient, "the question pool cannot satisfy the rule")
	}
	return out, err
}

// needsKeyToRender: an ordering is never shown in the right order and multiple_choice may show how many answers are
// right (show_correct_count); the key is read server side for that and never serialized (F022 clarification).
func needsKeyToRender(t question.Type) bool {
	return t == question.Ordering || t == question.MultipleChoice
}

// Render builds the render DTOs of published versions with a reproducible shuffle (F022-AC-22, AC-23, AC-25).
func (d Delivery) Render(ctx context.Context, versionIDs []uuid.UUID, seed uint64) (out []question.Rendered, pres []question.Presentation, err error) {
	ctx, done := observability.UseCase(ctx, "learning.qb.render")
	defer func() { done(err) }()
	err = d.Session.Read(ctx, func(ctx context.Context) error {
		cs, err := d.Reader.Deliverable(ctx, versionIDs)
		if err != nil {
			return err
		}
		for _, c := range cs {
			if needsKeyToRender(c.Type) {
				g, err := d.Reader.Grading(ctx, c.VersionID)
				if err != nil {
					return err
				}
				c.AnswerKey = g.AnswerKey
			}
			r, p, err := d.Registry.Render(c, seed)
			if errors.Is(err, params.ErrInfeasible) && d.Metrics != nil {
				d.Metrics.ParamSamplingFailed()
			}
			if err != nil {
				return err
			}
			out, pres = append(out, r), append(pres, p)
		}
		return nil
	})
	return out, pres, err
}

// Grade scores a response of a published version on the server (F022-AC-14…21): the response is checked against
// $defs.<type>Response (422 INVALID_RESPONSE, never 500); score and correctness never come from the client.
func (d Delivery) Grade(ctx context.Context, versionID uuid.UUID, pres question.Presentation, response json.RawMessage) (out question.GradeResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.qb.grade")
	defer func() { done(err) }()
	var c question.Content
	if err := d.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		c, err = d.Reader.Grading(ctx, versionID)
		return err
	}); err != nil {
		return out, err
	}
	invalid := func() error {
		if d.Metrics != nil {
			d.Metrics.Graded(string(c.Type), "invalid_response")
		}
		return problem.New(problem.InvalidResponse, "the response does not match the question type")
	}
	if d.Schema != nil && d.Schema.ValidateResponse(string(c.Type), response) != nil {
		return out, invalid()
	}
	out, err = d.Registry.GradeIn(c, pres, response)
	if errors.Is(err, question.ErrInvalidResponse) {
		return out, invalid()
	}
	if err == nil && d.Metrics != nil && out.IntegrityMismatch {
		d.Metrics.IntegrityMismatch(question.WidgetOf(c.Render))
	}
	if err == nil && d.Metrics != nil {
		result := "incorrect"
		if out.IsCorrect {
			result = "correct"
		}
		d.Metrics.Graded(string(c.Type), result)
	}
	return out, err
}

// OptionFeedback returns the option feedback of a pinned version (given after grading by F030, ACTIVITY_ENGINE §8.2).
func (d Delivery) OptionFeedback(ctx context.Context, versionID uuid.UUID) (out map[string]question.OptionFeedback, err error) {
	err = d.Session.Read(ctx, func(ctx context.Context) error {
		out, err = d.Reader.OptionFeedback(ctx, versionID)
		return err
	})
	return out, err
}

// Explanation returns the explanation of a published version (given after grading by F030).
func (d Delivery) Explanation(ctx context.Context, versionID uuid.UUID) (out json.RawMessage, err error) {
	err = d.Session.Read(ctx, func(ctx context.Context) error {
		out, err = d.Reader.Explanation(ctx, versionID)
		return err
	})
	return out, err
}
