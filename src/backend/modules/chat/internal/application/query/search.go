package query

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Limits of searchMessages (contract: q 2..100 characters; at most 8 words are searched).
const (
	searchMinRunes = 2
	searchMaxRunes = 100
	searchMaxWords = 8
)

// SearchInput is one searchMessages call.
type SearchInput struct {
	Q        string
	Channel  *uuid.UUID
	Sender   *uuid.UUID
	From, To *time.Time
	Cursor   string
	Limit    int
}

// SearchPage is a page of matches, newest first.
type SearchPage struct {
	Data []read.SearchHit
	Page page.Info
}

// Search is searchMessages (F120 US5): full-text with or without Vietnamese diacritics on the moderated display text of
// the persistent channels (club team, direct) the caller is a member of; never World/Area, the original of a masked
// message, recalled/removed/held/anonymised messages, another tenant or a cross-tenant parameter (F120-AC-22, AC-23).
type Search struct {
	Read    ReadSession
	Reader  read.SearchReader
	Cursors *page.Codec
}

// TSQuery turns a search text into a prefix tsquery of its words ("nhan vat" → "nhan:* & vat:*"); the database lowers
// and unaccents it (chat.f_unaccent) like search_tsv. "" when no word is left.
func TSQuery(q string) string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := make([]string, 0, len(words))
	for _, w := range words {
		if len(out) == searchMaxWords {
			break
		}
		out = append(out, w+":*")
	}
	return strings.Join(out, " & ")
}

// Search serves searchMessages.
func (q Search) Search(ctx context.Context, v Viewer, in SearchInput) (out SearchPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.SearchMessages")
	defer func() { done(err) }()
	if v.TenantID == uuid.Nil || v.UserID == uuid.Nil {
		return out, problem.New(problem.Unauthenticated, "")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Q)); n < searchMinRunes || n > searchMaxRunes {
		return out, problem.Invalid("q", problem.SubInvalid, "2..100 characters")
	}
	ts := TSQuery(in.Q)
	if ts == "" {
		return out, problem.Invalid("q", problem.SubInvalid, "at least one word")
	}
	limit := page.Limit(in.Limit)
	cur, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	sq := read.SearchQuery{Tenant: v.TenantID, Reader: v.UserID, TSQuery: ts, Channel: in.Channel, Sender: in.Sender, From: in.From,
		To: in.To, Limit: limit + 1}
	if cur != nil {
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		sq.AfterAt, sq.AfterID = &at, cur.ID
	}
	var hits []read.SearchHit
	if err := q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		hits, err = q.Reader.Search(ctx, sq)
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(hits) > limit {
		hits = hits[:limit]
		last := hits[len(hits)-1].Message.Envelope
		next := q.Cursors.Encode(page.Cursor{Key: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.MessageID})
		out.Page.NextCursor, out.Page.HasMore = &next, true
	}
	out.Data = hits
	return out, nil
}
