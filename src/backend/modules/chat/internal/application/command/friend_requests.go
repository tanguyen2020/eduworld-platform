package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RequestResult is the FriendRequest of the contract (201 of sendFriendRequest and redeemFriendCode) with the public
// cards of both students.
type RequestResult struct {
	Request   social.FriendRequest
	Requester write.Card
	Addressee write.Card
}

// SendFriendRequest implements sendFriendRequest (F095-AC-01…04, AC-15, AC-23).
func (h Social) SendFriendRequest(ctx context.Context, addressee uuid.UUID) (res RequestResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.SendFriendRequest")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	if addressee == me {
		return res, self("addressee_id")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if res, err = h.send(ctx, tenant, me, addressee, social.SourceDirect); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, res.Body())
	})
	return res, err
}

// send creates a pending request from me to addressee in the transaction of ctx: same checks for a direct request and
// a redeemed friend code (F095-AC-08).
func (h Social) send(ctx context.Context, tenant, me, addressee uuid.UUID, source string) (RequestResult, error) {
	now := h.now()
	t, err := h.tenant(ctx, tenant)
	if err != nil {
		return RequestResult{}, err
	}
	if !t.FriendsEnabled() { // F095-AC-23: before anything else, nothing written
		return RequestResult{}, h.scopeProblem(social.ErrFriendsDisabled, now)
	}
	from, err := h.caller(ctx, tenant, me)
	if err != nil {
		return RequestResult{}, err
	}
	to, err := h.target(ctx, tenant, addressee, true)
	if err != nil {
		return RequestResult{}, err
	}
	if err := h.Repo.LockUsers(ctx, me, addressee); err != nil {
		return RequestResult{}, err
	}
	blocked, err := h.Repo.Blocked(ctx, tenant, me, addressee)
	if err != nil {
		return RequestResult{}, err
	}
	if err := (social.ScopePolicy{}).Check(t, from, to, blocked); err != nil {
		return RequestResult{}, h.scopeProblem(err, now)
	}
	if err := h.Repo.ExpireStale(ctx, tenant, me, addressee, now); err != nil {
		return RequestResult{}, err
	}
	friends, err := h.Repo.Friends(ctx, tenant, social.NewPair(me, addressee))
	if err != nil {
		return RequestResult{}, err
	}
	pending, err := h.Repo.PendingFrom(ctx, tenant, me, addressee, now)
	if err != nil {
		return RequestResult{}, err
	}
	if friends || pending {
		return RequestResult{}, problem.New(problem.Conflict, "already friends or already requested")
	}
	var c social.Counters
	if c.Pending, err = h.Repo.CountPending(ctx, tenant, me, now); err != nil {
		return RequestResult{}, err
	}
	if c.Friends, err = h.Repo.CountFriends(ctx, tenant, me); err != nil {
		return RequestResult{}, err
	}
	if c.Today, err = h.Repo.CountSince(ctx, tenant, me, h.Config.Limits.DayStart(now)); err != nil {
		return RequestResult{}, err
	}
	if err := h.Config.Limits.CheckSend(c); err != nil {
		return RequestResult{}, h.scopeProblem(err, now)
	}
	r := social.FriendRequest{ID: ids.New(), TenantID: tenant, RequesterID: me, AddresseeID: addressee, Status: social.StatusPending,
		Source: source, CreatedAt: now, ExpiresAt: now.Add(h.Config.RequestTTL)}
	if err := h.Repo.CreateRequest(ctx, &r); err != nil {
		return RequestResult{}, err
	}
	if err := h.publish(ctx, "friend_request.sent", r.ID, tenant, now, map[string]any{"request_id": r.ID, "tenant_id": tenant,
		"requester_id": me, "addressee_id": addressee, "source": source, "expires_at": r.ExpiresAt}); err != nil {
		return RequestResult{}, err
	}
	if err := h.record(ctx, AuditFriendRequestSent, addressee, map[string]any{"request_id": r.ID.String(), "source": source}); err != nil {
		return RequestResult{}, err
	}
	cards, err := h.Directory.Cards(ctx, tenant, []uuid.UUID{me, addressee})
	if err != nil {
		return RequestResult{}, err
	}
	h.metric(port.ResultSent)
	return RequestResult{Request: r, Requester: card(cards, me), Addressee: card(cards, addressee)}, nil
}

func card(cards map[uuid.UUID]write.Card, id uuid.UUID) write.Card {
	if c, ok := cards[id]; ok {
		return c
	}
	return write.Card{UserID: id, Level: 1}
}

// pendingFor returns the live pending request id addressed to me, 404 for any other (F095-AC-05: someone else's
// request, expired or handled requests are indistinguishable). lock takes the row lock: only after the user locks, so a
// row lock is never held while waiting for an advisory lock (no deadlock between crossing accepts).
func (h Social) pendingFor(ctx context.Context, tenant, me, id uuid.UUID, now time.Time, lock bool) (*social.FriendRequest, error) {
	r, err := h.Repo.Request(ctx, tenant, id, lock)
	if err != nil {
		return nil, err
	}
	if r == nil || r.AddresseeID != me || !r.Live(now) {
		return nil, notFound()
	}
	return r, nil
}

// AcceptFriendRequest implements acceptFriendRequest (F095-AC-05): exactly one friendship per pair, the request
// accepted and a reverse pending request cancelled, friend.added + audit in the same transaction.
func (h Social) AcceptFriendRequest(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.AcceptFriendRequest")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	var pair social.Pair
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		r, err := h.pendingFor(ctx, tenant, me, id, now, false)
		if err != nil {
			return err
		}
		if err := h.Repo.LockUsers(ctx, r.RequesterID, me); err != nil {
			return err
		}
		if r, err = h.pendingFor(ctx, tenant, me, id, now, true); err != nil { // re-read under the user locks
			return err
		}
		if _, err := h.caller(ctx, tenant, me); err != nil {
			return err
		}
		if _, err := h.target(ctx, tenant, r.RequesterID, true); err != nil {
			return err
		}
		blocked, err := h.Repo.Blocked(ctx, tenant, me, r.RequesterID)
		if err != nil {
			return err
		}
		if blocked { // a block cancels pending requests; a leftover is never accepted
			return notFound()
		}
		pair = social.NewPair(me, r.RequesterID)
		already, err := h.Repo.Friends(ctx, tenant, pair)
		if err != nil {
			return err
		}
		if !already {
			for _, u := range []uuid.UUID{me, r.RequesterID} {
				n, err := h.Repo.CountFriends(ctx, tenant, u)
				if err != nil {
					return err
				}
				if err := h.Config.Limits.CheckFriends(n); err != nil {
					return h.scopeProblem(err, now)
				}
			}
		}
		if err := r.Accept(now); err != nil {
			return notFound()
		}
		if err := idempotency.SetResult(ctx, nil); err != nil {
			return err
		}
		if err := h.Repo.SaveRequest(ctx, r); err != nil {
			return err
		}
		// The reverse request (both sent almost at once) is cancelled: the friendship exists now (edge case).
		if _, err := h.Repo.CancelPending(ctx, tenant, me, r.RequesterID, now); err != nil {
			return err
		}
		if already {
			return nil
		}
		if err := h.Repo.CreateFriendship(ctx, tenant, pair); err != nil {
			return err
		}
		if err := h.publish(ctx, "friend.added", me, tenant, now, map[string]any{"tenant_id": tenant, "user_low_id": pair.Low,
			"user_high_id": pair.High, "source_version": version(now)}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditFriendAdded, r.RequesterID, map[string]any{"request_id": r.ID.String()}); err != nil {
			return err
		}
		return h.invalidate(ctx, me, r.RequesterID)
	})
	if err == nil {
		h.metric(port.ResultAccepted)
	}
	return err
}

// RejectFriendRequest implements rejectFriendRequest (F095-AC-06): silent, no event, no notification; the requester
// keeps seeing pending until the request expires.
func (h Social) RejectFriendRequest(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RejectFriendRequest")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		r, err := h.pendingFor(ctx, tenant, me, id, now, true)
		if err != nil {
			return err
		}
		if err := r.Reject(now); err != nil {
			return notFound()
		}
		return h.Repo.SaveRequest(ctx, r)
	})
	if err == nil {
		h.metric(port.ResultRejected)
	}
	return err
}

// CodeResult is the FriendCode of the contract.
type CodeResult struct {
	Code      string
	ExpiresAt time.Time
}

// codeAttempts bounds the retries on a code already live in the tenant (36^8 codes: a retry is rare).
const codeAttempts = 5

// CreateFriendCode implements createFriendCode (F095-AC-08, AC-23): a new code replaces the previous one of the
// student and expires after friend_code_ttl.
func (h Social) CreateFriendCode(ctx context.Context) (res CodeResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateFriendCode")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		t, err := h.tenant(ctx, tenant)
		if err != nil {
			return err
		}
		if !t.FriendsEnabled() {
			return h.scopeProblem(social.ErrFriendsDisabled, now)
		}
		if _, err := h.caller(ctx, tenant, me); err != nil {
			return err
		}
		if err := h.Repo.LockUsers(ctx, me); err != nil {
			return err
		}
		if err := h.Repo.ReplaceCode(ctx, tenant, me); err != nil {
			return err
		}
		res.ExpiresAt = now.Add(h.Config.CodeTTL)
		for i := 0; i < codeAttempts; i++ {
			if res.Code, err = h.Codes.NewCode(); err != nil {
				return err
			}
			err = h.Repo.CreateCode(ctx, tenant, me, res.Code, res.ExpiresAt)
			if err == nil {
				return idempotency.SetResult(ctx, res.Body())
			}
			if !errors.Is(err, write.ErrCodeTaken) {
				return err
			}
		}
		return problem.New(problem.Conflict, "no free friend code").Retry()
	})
	return res, err
}

// RedeemFriendCode implements redeemFriendCode (F095-AC-08): a live code of another student of the tenant creates a
// friend request with the same checks as sendFriendRequest; unknown or expired → 404 FRIEND_CODE_INVALID, malformed
// → 422.
func (h Social) RedeemFriendCode(ctx context.Context, code string) (res RequestResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RedeemFriendCode")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	if !social.ValidCode(code) {
		return res, problem.Invalid("code", problem.SubInvalid, "must match ^[A-Z0-9]{6,12}$")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		t, err := h.tenant(ctx, tenant)
		if err != nil {
			return err
		}
		if !t.FriendsEnabled() {
			return h.scopeProblem(social.ErrFriendsDisabled, now)
		}
		owner, err := h.Repo.CodeOwner(ctx, tenant, code, now)
		if err != nil {
			return err
		}
		if owner == nil {
			return problem.New(problem.FriendCodeInvalid, "")
		}
		if *owner == me {
			return self("code")
		}
		if res, err = h.send(ctx, tenant, me, *owner, social.SourceFriendCode); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, res.Body())
	})
	return res, err
}

// CardBody is the PlayerCard of the contract for a write.Card (nickname, level, avatar only — R16).
func CardBody(c write.Card) map[string]any {
	m := map[string]any{"user_id": c.UserID, "nickname": "", "level": c.Level}
	if c.Nickname != nil {
		m["nickname"] = *c.Nickname
	}
	if len(c.Avatar) > 0 {
		m["avatar_config"] = c.Avatar
	}
	return m
}

// Body is the FriendRequest of the contract (201 response, also stored as the idempotent result).
func (r RequestResult) Body() map[string]any {
	return map[string]any{"request_id": r.Request.ID, "status": r.Request.Status, "created_at": r.Request.CreatedAt.UTC(),
		"expires_at": r.Request.ExpiresAt.UTC(), "requester": CardBody(r.Requester), "addressee": CardBody(r.Addressee)}
}

// Body is the FriendCode of the contract.
func (r CodeResult) Body() map[string]any {
	return map[string]any{"code": r.Code, "expires_at": r.ExpiresAt.UTC()}
}
