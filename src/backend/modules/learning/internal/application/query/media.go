package query

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MediaLibrary is the read side of the media library (F027 US2, learning_content.read): lists and details with
// variant URLs (signed for ew-media-private, F027-AC-06) and the versions using an asset.
type MediaLibrary struct {
	Session read.Session
	Model   read.MediaReadModel
	Signer  port.URLSigner
	Cursors *page.Codec
	Clock   func() time.Time
}

func (q MediaLibrary) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func (q MediaLibrary) authorize(ctx context.Context) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	return scope.RequireContent(ctx, p, authz.PermLearningContentRead, nil)
}

// List pages the library newest first, filtered by kind, status and q (F027-AC-09).
func (q MediaLibrary) List(ctx context.Context, f read.MediaFilter, in PageInput) (out Page[dto.MediaView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMediaAssets")
	defer func() { done(err) }()
	if err := q.authorize(ctx); err != nil {
		return out, err
	}
	if f.Kind != "" && !media.Kind(f.Kind).Known() {
		return out, problem.Invalid("kind", problem.SubInvalid, "is not a media kind")
	}
	if utf8.RuneCountInString(f.Q) > 200 {
		return out, problem.Invalid("q", problem.SubMaxLength, "is longer than 200 characters")
	}
	f.Q = strings.TrimSpace(f.Q)
	limit := page.Limit(in.Limit)
	cur, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur != nil {
		f.After = &cur.ID
	}
	f.Limit = limit + 1
	var rows []media.Asset
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Model.List(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out = Page[dto.MediaView]{Limit: limit, Data: []dto.MediaView{}}
	if len(rows) > limit {
		out.HasMore, rows = true, rows[:limit]
		s := q.Cursors.Encode(page.Cursor{ID: rows[limit-1].ID})
		out.NextCursor = &s
	}
	at := q.now()
	for _, a := range rows {
		out.Data = append(out.Data, dto.MediaViewOf(a, urls(q.Signer, at)))
	}
	return out, nil
}

func urls(s port.URLSigner, at time.Time) dto.URLFunc {
	if s == nil {
		return nil
	}
	return func(bucket, key string) (string, *time.Time) { return s.URL(bucket, key, at) }
}

// MaxUsedIn bounds MediaAsset.used_in (OpenAPI maxItems).
const MaxUsedIn = 500

// Get returns an asset with its pipeline state, variants and the versions using it (F027-AC-10); another tenant's
// asset is 404.
func (q MediaLibrary) Get(ctx context.Context, id uuid.UUID) (out dto.MediaView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetMediaAsset")
	defer func() { done(err) }()
	if err := q.authorize(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		a, err := q.Model.Asset(ctx, id)
		if err != nil {
			return err
		}
		us, err := q.Model.Usages(ctx, id, MaxUsedIn)
		if err != nil {
			return err
		}
		out = dto.MediaViewOf(a, urls(q.Signer, q.now()))
		for _, u := range us {
			out.UsedIn = append(out.UsedIn, dto.MediaUsageView{ItemType: u.ItemType, ItemID: u.ItemID, Version: u.VersionNo})
		}
		return nil
	})
	return out, err
}
