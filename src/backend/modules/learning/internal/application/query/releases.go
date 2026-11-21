package query

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/snapshot"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// releaseTypes are the content types with versions and releases in this release (activity: F030).
var releaseTypes = []contentmodel.ContentType{contentmodel.TypeQuestion, contentmodel.TypeQuestionSet, contentmodel.TypeLesson,
	contentmodel.TypeLearningMaterial, contentmodel.TypeAssessmentBlueprint, contentmodel.TypeActivity}

// ContentReader reads items, versions and snapshots of every learning content type in the scope of ctx: the shared
// read side of the CMS history (Releases) and the ReleaseResolver/VersionReader ports of the learning use cases (F030).
// It does not authorize: its callers do.
type ContentReader struct {
	Questions read.QuestionReadModel
	Docs      read.DocReadModel
	// Cache keeps snapshots (ew:learning:cv:*); nil or an error falls back to PostgreSQL (F026-AC-14, R9).
	Cache port.SnapshotCache
}

// itemInfo is the owner and release pointer of an item.
type itemInfo struct {
	Owner     contentmodel.Owner
	Current   *uuid.UUID
	LatestNo  int
	ExamOnly  bool
	CreatedBy uuid.UUID
}

func (r ContentReader) item(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (itemInfo, error) {
	if t == contentmodel.TypeQuestion {
		q, err := r.Questions.Question(ctx, id)
		return itemInfo{Owner: q.Owner, Current: q.CurrentVersionID, LatestNo: q.LatestVersionNo}, err
	}
	d, err := r.Docs.Doc(ctx, t, id)
	return itemInfo{Owner: d.Owner, Current: d.CurrentVersionID, LatestNo: d.LatestVersionNo}, err
}

// version is one version with its document (answers included) and its snapshot document.
type version struct {
	view    dto.ContentVersionView
	doc     json.RawMessage // authoring document with answers
	snapDoc json.RawMessage
	qtype   string
	grades  []int
}

func (r ContentReader) version(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, no int, it itemInfo) (version, error) {
	var out version
	if t == contentmodel.TypeQuestion {
		v, err := r.Questions.Version(ctx, id, no)
		if err != nil {
			return out, err
		}
		out.doc, out.snapDoc, out.qtype, out.grades = dto.QuestionDoc(v, true), dto.QuestionSnapshotDoc(v), v.Type, v.GradeLevels
		out.view = dto.ContentVersionView{ContentType: string(t), ContentID: id, ContentVersionID: v.ID, Version: v.VersionNo, Status: string(v.Status),
			OwnerScope: string(it.Owner.Scope), TenantID: it.Owner.TenantID, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID, PublishedAt: v.PublishedAt,
			UpdatedAt: v.UpdatedAt.UTC(), ContentLanguage: v.ContentLanguage, TranslationStatus: v.TranslationStatus,
			AudioExemptReason: v.AudioExemptReason, AudioExemptApprovedBy: v.AudioExemptApprovedBy, RowVersion: v.RowVersion}
		return out, nil
	}
	v, err := r.Docs.Version(ctx, t, id, no)
	if err != nil {
		return out, err
	}
	out.doc = dto.DocDoc(v)
	out.snapDoc, out.grades = out.doc, v.GradeLevels
	out.view = dto.ContentVersionView{ContentType: string(t), ContentID: id, ContentVersionID: v.ID, Version: v.VersionNo, Status: string(v.Status),
		OwnerScope: string(it.Owner.Scope), TenantID: it.Owner.TenantID, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID, PublishedAt: v.PublishedAt,
		UpdatedAt: v.UpdatedAt.UTC(), ContentLanguage: v.ContentLanguage, TranslationStatus: v.TranslationStatus,
		AudioExemptReason: v.AudioExemptReason, AudioExemptApprovedBy: v.AudioExemptApprovedBy, RowVersion: v.RowVersion}
	return out, nil
}

// Current is the ReleaseResolver (F026-AC-15): the version a new learning unit starts with, read from the release
// pointer at that time; an unpublished item is 409 CONTENT_NOT_PUBLISHED.
func (r ContentReader) Current(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (uuid.UUID, int, error) {
	it, err := r.item(ctx, t, id)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if it.Current == nil {
		return uuid.Nil, 0, problem.New(problem.ContentNotPublished, "the content is not published")
	}
	no, err := r.versionNo(ctx, t, id, *it.Current, it)
	return *it.Current, no, err
}

func (r ContentReader) versionNo(ctx context.Context, t contentmodel.ContentType, id, versionID uuid.UUID, it itemInfo) (int, error) {
	for no := it.LatestNo; no >= 1; no-- {
		v, err := r.version(ctx, t, id, no, it)
		if err == nil && v.view.ContentVersionID == versionID {
			return no, nil
		}
	}
	return 0, problem.New(problem.NotFound, "")
}

// Snapshot is the VersionReader (F026-AC-14, AC-15): the snapshot of a pinned version, from Valkey or rebuilt from
// PostgreSQL (and cached again) — the same bytes either way, whatever the release pointer is now.
func (r ContentReader) Snapshot(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, no int) ([]byte, error) {
	key := "ew:learning:cv:" + string(t) + ":" + id.String() + ":" + itoa(no)
	if r.Cache != nil {
		if b, err := r.Cache.Get(ctx, key); err == nil {
			return b, nil
		}
	}
	it, err := r.item(ctx, t, id)
	if err != nil {
		return nil, err
	}
	v, err := r.version(ctx, t, id, no, it)
	if err != nil {
		return nil, err
	}
	b, _, err := dto.BuildSnapshot(t, id, v.view.ContentVersionID, no, v.view.ContentLanguage, v.grades, v.qtype, v.snapDoc)
	if err != nil {
		return nil, err
	}
	if r.Cache != nil {
		_ = r.Cache.Put(ctx, key, b) // best effort: a cache outage is not an error (R9)
	}
	return b, nil
}

// Releases is the CMS read side of the release (F026 US5, US7): learning_content.read; answers of a version only with
// learning_content.review.
type Releases struct {
	Session read.Session
	Model   read.ReleaseReadModel
	Reader  ContentReader
	Cursors *page.Codec
}

func (q Releases) authorize(ctx context.Context, typ string) (authz.Principal, contentmodel.ContentType, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, "", err
	}
	if err := scope.RequireContent(ctx, p, authz.PermLearningContentRead, nil); err != nil {
		return p, "", err
	}
	t, err := contentmodel.ParseContentType(typ)
	if err != nil || !slices.Contains(releaseTypes, t) {
		return p, "", problem.Invalid("content_type", problem.SubInvalid, "has no versions in this release")
	}
	return p, t, nil
}

// VersionHistory is schema ContentVersionHistory.
type VersionHistory struct {
	ItemID         uuid.UUID            `json:"item_id"`
	CurrentVersion *int                 `json:"current_version,omitempty"`
	Data           []dto.VersionSummary `json:"data"`
	Page           PageInfo             `json:"page"`
}

// PageInfo is schema PageInfo.
type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

// History lists the versions of an item, newest first, with the current release (F026-AC-20); active_pinned_sessions
// is 0 until the attempts of F030 count them [Proposed].
func (q Releases) History(ctx context.Context, typ string, id uuid.UUID, status string, in PageInput) (out VersionHistory, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListContentVersions")
	defer func() { done(err) }()
	_, t, err := q.authorize(ctx, typ)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		it, err := q.Reader.item(ctx, t, id)
		if err != nil {
			return err
		}
		vs, err := q.Model.Versions(ctx, t, id)
		if err != nil {
			return err
		}
		out = VersionHistory{ItemID: id, Data: []dto.VersionSummary{}}
		var items []dto.VersionSummary
		for _, v := range vs {
			v.IsCurrent = it.Current != nil && *it.Current == v.ContentVersionID
			if v.IsCurrent {
				no := v.Version
				out.CurrentVersion = &no
			}
			if status == "" || v.Status == status {
				items = append(items, v)
			}
		}
		p, err := paginate(items, func(v dto.VersionSummary) uuid.UUID { return v.ContentVersionID }, in, q.Cursors)
		if err != nil {
			return err
		}
		out.Data = append(out.Data, p.Data...)
		out.Page = PageInfo{NextCursor: p.NextCursor, HasMore: p.HasMore, Limit: p.Limit}
		return nil
	})
	return out, err
}

// GetVersion returns a version read-only with its content (F026-AC-20): archived versions included; answer data only
// for a caller with learning_content.review.
func (q Releases) GetVersion(ctx context.Context, typ string, id uuid.UUID, no int) (out dto.ContentVersionDetail, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetContentVersion")
	defer func() { done(err) }()
	p, t, err := q.authorize(ctx, typ)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		it, err := q.Reader.item(ctx, t, id)
		if err != nil {
			return err
		}
		v, err := q.Reader.version(ctx, t, id, no, it)
		if err != nil {
			return err
		}
		content := v.doc
		if !p.Has(authz.PermLearningContentReview) {
			if content, err = snapshot.Strip(content); err != nil {
				return err
			}
		}
		v.view.Assigned = it.Owner.Platform() && p.TenantID != nil
		v.view.IsCurrent = it.Current != nil && *it.Current == v.view.ContentVersionID
		out = dto.ContentVersionDetail{ContentVersionView: v.view, Content: content}
		return nil
	})
	return out, err
}

// VersionDiff is schema ContentVersionDiff.
type VersionDiff struct {
	FromVersion int               `json:"from_version"`
	ToVersion   int               `json:"to_version"`
	Changes     []snapshot.Change `json:"changes"`
}

// Diff compares version with to version no by JSON pointer (F026-AC-21): answer pointers are dropped without
// learning_content.review.
func (q Releases) Diff(ctx context.Context, typ string, id uuid.UUID, no, with int) (out VersionDiff, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.DiffContentVersions")
	defer func() { done(err) }()
	p, t, err := q.authorize(ctx, typ)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		it, err := q.Reader.item(ctx, t, id)
		if err != nil {
			return err
		}
		a, err := q.Reader.version(ctx, t, id, with, it)
		if err != nil {
			return err
		}
		b, err := q.Reader.version(ctx, t, id, no, it)
		if err != nil {
			return err
		}
		cs := snapshot.Diff(a.doc, b.doc)
		if !p.Has(authz.PermLearningContentReview) {
			cs = snapshot.HideAnswers(cs)
		}
		if cs == nil {
			cs = []snapshot.Change{}
		}
		out = VersionDiff{FromVersion: with, ToVersion: no, Changes: cs}
		return nil
	})
	return out, err
}

// ListReleases lists the publishes, rollbacks and unpublishes of an item, newest first (F026-AC-22).
func (q Releases) ListReleases(ctx context.Context, typ string, id uuid.UUID, in PageInput) (out Page[dto.ContentReleaseView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListContentReleases")
	defer func() { done(err) }()
	_, t, err := q.authorize(ctx, typ)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	cur, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var after *uuid.UUID
	if cur != nil {
		after = &cur.ID
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.Reader.item(ctx, t, id); err != nil {
			return err
		}
		vs, err := q.Model.Versions(ctx, t, id)
		if err != nil {
			return err
		}
		nos := map[uuid.UUID]int{}
		for _, v := range vs {
			nos[v.ContentVersionID] = v.Version
		}
		rows, err := q.Model.Releases(ctx, t, id, after, limit+1)
		if err != nil {
			return err
		}
		out = Page[dto.ContentReleaseView]{Limit: limit, Data: []dto.ContentReleaseView{}}
		if len(rows) > limit {
			out.HasMore, rows = true, rows[:limit]
			s := q.Cursors.Encode(page.Cursor{ID: rows[limit-1].ID})
			out.NextCursor = &s
		}
		for _, r := range rows {
			v := dto.ContentReleaseView{ReleaseID: r.ID, ItemType: r.ItemType, ItemID: r.ItemID, Action: r.Action, ScheduledFor: r.ScheduledFor,
				Status: "applied", ReleasedBy: r.ReleasedBy, ReleasedAt: r.ReleasedAt}
			if r.Reason != nil {
				v.Reason = *r.Reason
			}
			if r.FromVersionID != nil {
				if n, ok := nos[*r.FromVersionID]; ok {
					v.FromVersion = &n
				}
			}
			if r.ToVersionID != nil {
				if n, ok := nos[*r.ToVersionID]; ok {
					v.ToVersion = &n
				}
			}
			out.Data = append(out.Data, v)
		}
		return nil
	})
	return out, err
}

// Usages lists where an item is referenced in module learning in the caller's scope (F026-AC-27): question sets for a
// question; activities, assignments and exam sessions are added by F030, F041, F042 [Proposed]. pinned_sessions is 0
// until F030 counts the attempts.
func (q Releases) Usages(ctx context.Context, typ string, id uuid.UUID) (out []dto.ContentUsageView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetContentUsages")
	defer func() { done(err) }()
	_, t, err := q.authorize(ctx, typ)
	if err != nil {
		return out, err
	}
	out = []dto.ContentUsageView{}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.Reader.item(ctx, t, id); err != nil {
			return err
		}
		if t != contentmodel.TypeQuestion {
			return nil
		}
		us, err := q.Model.QuestionUsages(ctx, id)
		if err != nil {
			return err
		}
		out = append(out, us...)
		return nil
	})
	if errors.Is(err, port.ErrCacheMiss) {
		err = nil
	}
	return out, err
}

func itoa(n int) string { return strconv.Itoa(n) }
