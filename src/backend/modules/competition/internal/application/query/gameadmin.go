package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/gameadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Page sizes of the console listings (API_DESIGN §5: default 50, at most 200).
const (
	configPageDefault = 50
	configPageMax     = 200
)

// GameConfigs are the read use cases of the competition configurations (F108 US1): list, version, versions, diff and
// releases, read under the platform scope (ReadSession, no write, no event). Every listing is a keyset page paged in SQL.
type GameConfigs struct {
	Read   read.ReadSession
	Reader read.GameConfigReader
}

// ConfigPage is a page of listGameConfigs (data + page).
type ConfigPage struct {
	Data []gameadmin.ConfigVersionView `json:"data"`
	Page PageInfo                      `json:"page"`
}

// PageInfo is PageInfo of the contract.
type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

// VersionSummary is ContentVersionSummary of the contract.
type VersionSummary struct {
	Version          int        `json:"version"`
	ContentVersionID uuid.UUID  `json:"content_version_id"`
	Status           string     `json:"status"`
	IsCurrent        bool       `json:"is_current"`
	ChangeNote       *string    `json:"change_note,omitempty"`
	AuthorID         *uuid.UUID `json:"author_id,omitempty"`
	ReviewerID       *uuid.UUID `json:"reviewer_id,omitempty"`
	PublishedBy      *uuid.UUID `json:"published_by,omitempty"`
	PublishedAt      *string    `json:"published_at,omitempty"`
	EffectiveFrom    *string    `json:"effective_from,omitempty"`
}

// VersionHistory is ContentVersionHistory of the contract.
type VersionHistory struct {
	ItemID         uuid.UUID        `json:"item_id"`
	CurrentVersion *int             `json:"current_version,omitempty"`
	Data           []VersionSummary `json:"data"`
	Page           PageInfo         `json:"page"`
}

// VersionDiff is ContentVersionDiff of the contract.
type VersionDiff struct {
	FromVersion int                 `json:"from_version"`
	ToVersion   int                 `json:"to_version"`
	Changes     []gameconfig.Change `json:"changes"`
}

// ReleasePage is the page of listGameConfigReleases.
type ReleasePage struct {
	Data []gameadmin.ReleaseView `json:"data"`
	Page PageInfo                `json:"page"`
}

func cfgPageLimit(n int) int {
	switch {
	case n <= 0:
		return configPageDefault
	case n > configPageMax:
		return configPageMax
	}
	return n
}

func cfgEncodeCursor(s string) *string {
	c := base64.RawURLEncoding.EncodeToString([]byte(s))
	return &c
}

func cfgDecodeCursor(c string) (string, error) {
	if c == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || len(b) == 0 {
		return "", problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
	}
	return string(b), nil
}

func cfgUUIDCursor(c string) (*uuid.UUID, error) {
	s, err := cfgDecodeCursor(c)
	if err != nil || s == "" {
		return nil, err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
	}
	return &id, nil
}

// cfgReadScope authorizes a read of the type and returns its platform scope.
func cfgReadScope(ctx context.Context, configType string) (gameconfig.Type, tenancy.Scope, error) {
	t, err := gameadmin.Type(configType)
	if err != nil {
		return t, nil, err
	}
	p, sc, err := gameadmin.Principal(ctx)
	if err != nil {
		return t, nil, err
	}
	if err := gameadmin.RequireRead(ctx, p, configType); err != nil {
		return t, nil, err
	}
	return t, sc, nil
}

// List is listGameConfigs_competition: the newest version of each configuration of the type, optional status filter,
// keyset by config_id (F108-AC-01, AC-10).
func (q GameConfigs) List(ctx context.Context, configType, status, cursor string, limit int) (out ConfigPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListGameConfigs")
	defer func() { done(err) }()
	t, sc, err := cfgReadScope(ctx, configType)
	if err != nil {
		return out, err
	}
	if status != "" && !gameconfig.ValidStatus(status) {
		return out, problem.Invalid("status", problem.SubInvalid, "not a content status")
	}
	after, err := cfgUUIDCursor(cursor)
	if err != nil {
		return out, err
	}
	limit = cfgPageLimit(limit)
	err = q.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		rows, err := q.Reader.Configs(ctx, t.Name, status, after, limit)
		if err != nil {
			return err
		}
		out = ConfigPage{Data: []gameadmin.ConfigVersionView{}, Page: PageInfo{Limit: limit}}
		if len(rows) > limit {
			rows = rows[:limit]
			out.Page.HasMore = true
			out.Page.NextCursor = cfgEncodeCursor(rows[limit-1].Item.ConfigID.String())
		}
		for _, r := range rows {
			out.Data = append(out.Data, gameadmin.VersionView(t, r.Item, r.Version, nil))
		}
		return nil
	})
	return out, err
}

func (q GameConfigs) loadVersion(ctx context.Context, t gameconfig.Type, configID uuid.UUID, no int) (*port.ConfigItem, *port.ConfigVersion, error) {
	it, err := q.Reader.Item(ctx, t.Name, configID)
	if err != nil {
		return nil, nil, err
	}
	if it == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	v, err := q.Reader.Version(ctx, configID, no)
	if err != nil {
		return nil, nil, err
	}
	if v == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	return it, v, nil
}

// Get is getGameConfigVersion_competition (ETag = the revision of the version).
func (q GameConfigs) Get(ctx context.Context, configType string, configID uuid.UUID, no int) (out gameadmin.ConfigVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetGameConfigVersion")
	defer func() { done(err) }()
	t, sc, err := cfgReadScope(ctx, configType)
	if err != nil {
		return out, err
	}
	err = q.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, v, err := q.loadVersion(ctx, t, configID, no)
		if err != nil {
			return err
		}
		var basedOn *int
		if v.BasedOn != nil {
			nos, err := q.Reader.VersionNos(ctx, configID)
			if err != nil {
				return err
			}
			if n, ok := nos[*v.BasedOn]; ok {
				basedOn = &n
			}
		}
		out = gameadmin.VersionView(t, *it, *v, basedOn)
		return nil
	})
	return out, err
}

func cfgTimeString(v *port.ConfigVersion, published bool) *string {
	if published && v.PublishedAt != nil {
		s := v.PublishedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00")
		return &s
	}
	if !published && v.EffectiveFrom != nil {
		s := v.EffectiveFrom.UTC().Format("2006-01-02T15:04:05.999999Z07:00")
		return &s
	}
	return nil
}

// Versions is listGameConfigVersions_competition: every version newest first (keyset by version number) and the current
// pointer.
func (q GameConfigs) Versions(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (out VersionHistory, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListGameConfigVersions")
	defer func() { done(err) }()
	t, sc, err := cfgReadScope(ctx, configType)
	if err != nil {
		return out, err
	}
	before := 0
	if s, err := cfgDecodeCursor(cursor); err != nil {
		return out, err
	} else if s != "" {
		if before, err = strconv.Atoi(s); err != nil || before < 1 {
			return out, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
		}
	}
	limit = cfgPageLimit(limit)
	err = q.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := q.Reader.Item(ctx, t.Name, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		rows, err := q.Reader.Versions(ctx, configID, before, limit)
		if err != nil {
			return err
		}
		out = VersionHistory{ItemID: configID, Data: []VersionSummary{}, Page: PageInfo{Limit: limit}}
		if it.Current != nil {
			nos, err := q.Reader.VersionNos(ctx, configID)
			if err != nil {
				return err
			}
			if n, ok := nos[*it.Current]; ok {
				out.CurrentVersion = &n
			}
		}
		if len(rows) > limit {
			rows = rows[:limit]
			out.Page.HasMore = true
			out.Page.NextCursor = cfgEncodeCursor(strconv.Itoa(rows[limit-1].No))
		}
		for i := range rows {
			v := rows[i]
			out.Data = append(out.Data, VersionSummary{Version: v.No, ContentVersionID: v.ID, Status: v.Status,
				IsCurrent: it.Current != nil && *it.Current == v.ID, ChangeNote: v.ChangeNote, AuthorID: v.AuthorID, ReviewerID: v.ReviewerID,
				PublishedBy: v.PublishedBy, PublishedAt: cfgTimeString(&v, true), EffectiveFrom: cfgTimeString(&v, false)})
		}
		return nil
	})
	return out, err
}

// Diff is diffGameConfigVersions_competition: the changes from version `with` to version no, field by field.
func (q GameConfigs) Diff(ctx context.Context, configType string, configID uuid.UUID, no, with int) (out VersionDiff, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.DiffGameConfigVersions")
	defer func() { done(err) }()
	t, sc, err := cfgReadScope(ctx, configType)
	if err != nil {
		return out, err
	}
	if with < 1 {
		return out, problem.Invalid("with", problem.SubInvalid, "a version number")
	}
	err = q.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		_, a, err := q.loadVersion(ctx, t, configID, with)
		if err != nil {
			return err
		}
		_, b, err := q.loadVersion(ctx, t, configID, no)
		if err != nil {
			return err
		}
		var da, db map[string]any
		_ = json.Unmarshal(a.Payload, &da)
		_ = json.Unmarshal(b.Payload, &db)
		changes, _ := gameconfig.Diff(da, db)
		if changes == nil {
			changes = []gameconfig.Change{}
		}
		out = VersionDiff{FromVersion: with, ToVersion: no, Changes: changes}
		return nil
	})
	return out, err
}

// Releases is listGameConfigReleases_competition: publish/rollback/unpublish newest first with their reason, actor and
// effective time (keyset by release).
func (q GameConfigs) Releases(ctx context.Context, configType string, configID uuid.UUID, cursor string, limit int) (out ReleasePage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListGameConfigReleases")
	defer func() { done(err) }()
	t, sc, err := cfgReadScope(ctx, configType)
	if err != nil {
		return out, err
	}
	after, err := cfgUUIDCursor(cursor)
	if err != nil {
		return out, err
	}
	limit = cfgPageLimit(limit)
	err = q.Read.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		it, err := q.Reader.Item(ctx, t.Name, configID)
		if err != nil {
			return err
		}
		if it == nil {
			return problem.New(problem.NotFound, "")
		}
		rows, err := q.Reader.Releases(ctx, configID, after, limit)
		if err != nil {
			return err
		}
		nos, err := q.Reader.VersionNos(ctx, configID)
		if err != nil {
			return err
		}
		out = ReleasePage{Data: []gameadmin.ReleaseView{}, Page: PageInfo{Limit: limit}}
		if len(rows) > limit {
			rows = rows[:limit]
			out.Page.HasMore = true
			out.Page.NextCursor = cfgEncodeCursor(rows[limit-1].ID.String())
		}
		for _, r := range rows {
			out.Data = append(out.Data, gameadmin.ReleaseOf(t.Name, r, nos))
		}
		return nil
	})
	return out, err
}
