package read

import (
	"context"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// PublishedContentModel implements read.PublishedContentReadModel with explicit SQL on the platform rows of
// content.activities, content.question_sets and content.lessons (tenant_id IS NULL, owner_scope platform, deleted_at
// filtered) and their published versions (F078-AC-07: references of a game config).
type PublishedContentModel struct{}

var _ read.PublishedContentReadModel = PublishedContentModel{}

// publishedTables maps a content type to its item and version tables (closed map, never from input).
var publishedTables = map[string][4]string{
	"activity":     {"content.activities", "activity_id", "content.activity_versions", "activity_version_id"},
	"question_set": {"content.question_sets", "question_set_id", "content.question_set_versions", "question_set_version_id"},
	"lesson":       {"content.lessons", "lesson_id", "content.lesson_versions", "lesson_version_id"},
}

// Published implements read.PublishedContentReadModel.
func (PublishedContentModel) Published(ctx context.Context, refs []read.PublishedRef) ([]bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]bool, len(refs))
	for i, r := range refs {
		tb, ok := publishedTables[r.ContentType]
		if !ok {
			return nil, fmt.Errorf("published content: unknown type %q", r.ContentType)
		}
		// tenancyguard:ok platform content only (tenant_id IS NULL, owner_scope platform) with deleted_at filtered on both tables
		ok, err := existsQuery(ctx, tx, fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s i JOIN %s v ON v.%s = i.%s AND v.deleted_at IS NULL
			WHERE i.%s = $1 AND i.tenant_id IS NULL AND i.owner_scope = 'platform' AND i.deleted_at IS NULL AND i.current_version_id IS NOT NULL
			AND v.status = 'published' AND ($2::int IS NULL OR v.version_no = $2) AND ($2::int IS NOT NULL OR v.%s = i.current_version_id))`,
			tb[0], tb[2], tb[1], tb[1], tb[1], tb[3]), r.ContentID, r.Version)
		if err != nil {
			return nil, fmt.Errorf("published content: %w", err)
		}
		out[i] = ok
	}
	return out, nil
}

// existsQuery runs a SELECT EXISTS query.
func existsQuery(ctx context.Context, q querier, query string, args ...any) (bool, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	ok := false
	for rows.Next() {
		if err := rows.Scan(&ok); err != nil {
			return false, err
		}
	}
	return ok, rows.Err()
}
