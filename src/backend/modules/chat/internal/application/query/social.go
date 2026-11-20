package query

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReadSession runs fn in a READ ONLY transaction with the scope of ctx (entx.ReadSession).
type ReadSession interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// RequestsLimit bounds listFriendRequests (no cursor in the contract): incoming requests are bounded by friend_limits
// of their senders, outgoing ones by the pending limit [Proposed F095].
const RequestsLimit = 100

// Social are the read use cases of friends, blocks and mutes (F095 US2, US3): always the relations of the calling
// student in the session tenant; who blocked or muted the caller is never readable (F095-AC-19).
type Social struct {
	Read      ReadSession
	Relations read.SocialReader
	Presence  port.PresenceStore // nil: everyone offline
	Cursors   *page.Codec
	Now       func() time.Time
}

// Card is a PlayerCard of the contract (no real name, school, class, score — R16).
type Card struct {
	UserID   uuid.UUID
	Nickname *string
	Level    int
	Avatar   []byte
	Presence string // empty: not part of the response
}

// Relation is a PlayerRelation (listBlocks, listMutes) or the card of a friend with created_at.
type Relation struct {
	Card
	CreatedAt time.Time
}

// RelationPage is a page of relations.
type RelationPage struct {
	Data []Relation
	Page page.Info
}

// Request is a FriendRequest of listFriendRequests.
type Request struct {
	ID        uuid.UUID
	Status    string
	CreatedAt time.Time
	ExpiresAt time.Time
	Requester Card
	Addressee Card
}

func (q Social) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

func caller(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

func cardOf(r read.Relation) Card {
	return Card{UserID: r.UserID, Nickname: r.Nickname, Level: r.Level, Avatar: r.Avatar}
}

type lister func(ctx context.Context, tenant, user uuid.UUID, after *read.After, limit int) ([]read.Relation, error)

func (q Social) list(ctx context.Context, name string, fn lister, limit int, cursor string) (out RelationPage, err error) {
	ctx, done := observability.UseCase(ctx, name)
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	cur, err := q.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var after *read.After
	if cur != nil {
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &read.After{At: at, ID: cur.ID}
	}
	var rows []read.Relation
	if err := q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = fn(ctx, tenant, me, after, limit+1)
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next := q.Cursors.Encode(page.Cursor{Key: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.RowID})
		out.Page.NextCursor, out.Page.HasMore = &next, true
	}
	out.Data = make([]Relation, len(rows))
	for i, r := range rows {
		out.Data[i] = Relation{Card: cardOf(r), CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// ListFriends implements listFriends (F095-AC-11, AC-22): the friends of the caller as PlayerCards with the last
// presence received (hidden → offline, none or expired → offline).
func (q Social) ListFriends(ctx context.Context, limit int, cursor string) (RelationPage, error) {
	out, err := q.list(ctx, "chat.query.ListFriends", q.Relations.Friends, limit, cursor)
	if err != nil {
		return out, err
	}
	states := map[uuid.UUID]social.Presence{}
	if q.Presence != nil && len(out.Data) > 0 {
		ids := make([]uuid.UUID, len(out.Data))
		for i, r := range out.Data {
			ids[i] = r.UserID
		}
		if states, err = q.Presence.Get(ctx, ids); err != nil {
			states = map[uuid.UUID]social.Presence{} // ephemeral: unknown presence is offline (ADR-0004)
		}
	}
	for i := range out.Data {
		out.Data[i].Presence = states[out.Data[i].UserID].Visible().Status
	}
	return out, nil
}

// ListBlocks implements listBlocks (F095-AC-19): only the blocks created by the caller.
func (q Social) ListBlocks(ctx context.Context, limit int, cursor string) (RelationPage, error) {
	return q.list(ctx, "chat.query.ListBlocks", q.Relations.Blocks, limit, cursor)
}

// ListMutes implements listMutes (F095-AC-19): only the mutes created by the caller.
func (q Social) ListMutes(ctx context.Context, limit int, cursor string) (RelationPage, error) {
	return q.list(ctx, "chat.query.ListMutes", q.Relations.Mutes, limit, cursor)
}

// ErrDirection is an unknown direction (422).
var ErrDirection = errors.New("direction must be incoming or outgoing")

// ListFriendRequests implements listFriendRequests (F095-AC-06, AC-13): incoming = live pending requests addressed
// to the caller; outgoing = requests sent by the caller that did not expire, a rejected one shown pending (silent
// rejection). The requests of other students never appear.
func (q Social) ListFriendRequests(ctx context.Context, direction string) (out []Request, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListFriendRequests")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if direction == "" {
		direction = "incoming"
	}
	if direction != "incoming" && direction != "outgoing" {
		return nil, problem.Invalid("direction", problem.SubInvalid, ErrDirection.Error())
	}
	now := q.now()
	var rows []read.Request
	if err := q.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Relations.Requests(ctx, tenant, me, direction == "incoming", now, RequestsLimit)
		return err
	}); err != nil {
		return nil, err
	}
	out = make([]Request, len(rows))
	for i, r := range rows {
		fr := social.FriendRequest{Status: r.Status, ExpiresAt: r.ExpiresAt}
		status := r.Status
		if direction == "outgoing" {
			status = fr.OutgoingStatus(now)
		}
		out[i] = Request{ID: r.ID, Status: status, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Requester: cardOf(r.Requester),
			Addressee: cardOf(r.Addressee)}
	}
	return out, nil
}
