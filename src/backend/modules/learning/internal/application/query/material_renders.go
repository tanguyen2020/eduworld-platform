package query

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MaterialRef is one reference of getLearningMaterialRenders (version nil = current release).
type MaterialRef struct {
	ID      uuid.UUID
	Version *int
}

// MaterialRender is the render of one published learning material: its title and display blocks only.
type MaterialRender struct {
	ID      uuid.UUID
	Version int
	Title   map[string]string
	Blocks  []map[string]any
}

// MaterialRendersHandler is the typed interface of the facade query getLearningMaterialRenders (ADR-0087).
type MaterialRendersHandler interface {
	Renders(ctx context.Context, refs []MaterialRef) ([]MaterialRender, error)
}

// MaterialRenders is the facade query getLearningMaterialRenders (x-allowed-callers: game, F103 research R3): the clue
// cards of an interdependent step are published learning materials; module game asks their render under the tenant
// of the session and decides who sees which card. Only rich_text and media blocks are returned — activity and
// worked_example blocks (questions, solutions) never leave module learning (ADR-0016). Read only.
type MaterialRenders struct {
	Session read.Session
	Model   read.DocReadModel
}

var _ MaterialRendersHandler = MaterialRenders{}

// displayBlocks are the block kinds a clue card may show (lesson.schema.json block kinds without answers).
var displayBlocks = []string{"rich_text", "media"}

// Renders implements MaterialRendersHandler: one render per reference, in order; an unknown or unpublished material
// is 404 NOT_FOUND.
func (q MaterialRenders) Renders(ctx context.Context, refs []MaterialRef) (out []MaterialRender, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.GetLearningMaterialRenders")
	defer func() { done(err) }()
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	if len(refs) == 0 || len(refs) > 4 {
		return nil, problem.Invalid("/refs", problem.SubInvalid, "1 to 4 references")
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		for _, r := range refs {
			v, err := q.version(ctx, r)
			if err != nil {
				return err
			}
			out = append(out, render(v))
		}
		return nil
	})
	return out, err
}

func (q MaterialRenders) version(ctx context.Context, r MaterialRef) (dto.DocVersion, error) {
	t := contentmodel.TypeLearningMaterial
	d, err := q.Model.Doc(ctx, t, r.ID)
	if err != nil {
		return dto.DocVersion{}, err
	}
	no := 0
	switch {
	case r.Version != nil:
		no = *r.Version
	case d.CurrentVersionID != nil:
		latest, err := q.Model.Latest(ctx, t, r.ID)
		if err != nil {
			return dto.DocVersion{}, err
		}
		if latest.ID == *d.CurrentVersionID {
			return published(latest)
		}
		return dto.DocVersion{}, problem.New(problem.NotFound, "the current release of the learning material is not readable")
	default:
		return dto.DocVersion{}, problem.New(problem.NotFound, "the learning material is not published")
	}
	v, err := q.Model.Version(ctx, t, r.ID, no)
	if err != nil {
		return dto.DocVersion{}, err
	}
	return published(v)
}

func published(v dto.DocVersion) (dto.DocVersion, error) {
	if v.Status != contentmodel.StatusPublished {
		return dto.DocVersion{}, problem.New(problem.NotFound, "the learning material version is not published")
	}
	return v, nil
}

func render(v dto.DocVersion) MaterialRender {
	out := MaterialRender{ID: v.ItemID, Version: v.VersionNo, Title: map[string]string{}, Blocks: []map[string]any{}}
	_ = json.Unmarshal(v.Title, &out.Title)
	var body struct {
		Blocks []map[string]any `json:"blocks"`
	}
	_ = json.Unmarshal(v.Body, &body)
	for _, b := range body.Blocks {
		if kind, _ := b["kind"].(string); slices.Contains(displayBlocks, kind) {
			out.Blocks = append(out.Blocks, b)
		}
	}
	return out
}
