package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// Keys of matchmaking (VALKEY_DESIGN §3.1, owner competition · matchmaking, real prefix ew:competition:mm:). Every key is
// built here; scripts receive them through KEYS[] only. Ticket fields that name keys (queue, students, party) never
// change after creation, so they are read before a script runs.
const mmPrefix = "ew:competition:mm:"

// MMQueueKey is ew:competition:mm:queue:<queue_key> (ZSET ticket_id → enqueued_at ms).
func MMQueueKey(queue string) string { return mmPrefix + "queue:" + queue }

// MMTicketKey is ew:competition:mm:ticket:<ticket_id> (HASH, TTL mmTicketTTL).
func MMTicketKey(id uuid.UUID) string { return mmPrefix + "ticket:" + id.String() }

// MMUserKey is ew:competition:mm:user:<student_id> (STRING ticket_id, NX: one active ticket per student).
func MMUserKey(id uuid.UUID) string { return mmPrefix + "user:" + id.String() }

// MMPartyKey is ew:competition:mm:party:<party_id> (STRING ticket_id, NX: one active ticket per party).
func MMPartyKey(id uuid.UUID) string { return mmPrefix + "party:" + id.String() }

// MMProposalKey is ew:competition:mm:proposal:<proposal_id> (HASH doc, status, deadline, n, acc:<student>).
func MMProposalKey(id uuid.UUID) string { return mmPrefix + "proposal:" + id.String() }

// MMLockKey is ew:competition:mm:lock:<queue_key> (STRING owner, NX PX: one matchmaker per queue).
func MMLockKey(queue string) string { return mmPrefix + "lock:" + queue }

// MMRateKey is ew:competition:mm:rate:<student_id> (ZSET, 1 h sliding window of mm_ticket_rate).
func MMRateKey(id uuid.UUID) string { return mmPrefix + "rate:" + id.String() }

// MMDeclinesKey is ew:competition:mm:declines:<student_id> (ZSET of declines/timeouts in the decline window).
func MMDeclinesKey(id uuid.UUID) string { return mmPrefix + "declines:" + id.String() }

// MMCooldownKey is ew:competition:mm:cooldown:<student_id> (STRING, PX = cooldown of mm_ticket_rate).
func MMCooldownKey(id uuid.UUID) string { return mmPrefix + "cooldown:" + id.String() }

// Set keys of matchmaking.
const (
	MMQueuesKey    = mmPrefix + "queues"    // SET of queue keys that may hold tickets
	MMProposalsKey = mmPrefix + "proposals" // ZSET proposal_id → deadline (open) or retry time (accepted)
)

// Lifetimes of the matchmaking keys.
const (
	mmTicketTTL      = 10 * time.Minute // ticket hash and holder keys (VALKEY_DESIGN: 10 min > mm_ticket_ttl)
	mmEndedTTL       = 10 * time.Minute // ended ticket hash kept for getMatchmakingTicket
	mmProposalMargin = time.Minute      // proposal hash kept after its deadline
	mmAcceptedTTL    = 10 * time.Minute // accepted proposal kept until its match exists
)

// MatchmakingStore implements port.MatchmakingStore on Valkey (ephemeral, ADR-0004).
type MatchmakingStore struct{ C valkey.Client }

var _ port.MatchmakingStore = MatchmakingStore{}

func ids(list []uuid.UUID) string {
	s := make([]string, len(list))
	for i, id := range list {
		s[i] = id.String()
	}
	return strings.Join(s, ",")
}

func parseIDs(s string) []uuid.UUID {
	var out []uuid.UUID
	for _, p := range strings.Split(s, ",") {
		if id, err := uuid.Parse(p); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func ms(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func fromMS(s string) time.Time {
	n, _ := strconv.ParseInt(s, 10, 64)
	return time.UnixMilli(n).UTC()
}

func optID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func parseOpt(s string) *uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

func flag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// holders are the NX keys of a ticket: its students and its party.
func holders(t port.Ticket) []string {
	keys := make([]string, 0, len(t.Students)+1)
	for _, s := range t.Students {
		keys = append(keys, MMUserKey(s))
	}
	if t.PartyID != nil {
		keys = append(keys, MMPartyKey(*t.PartyID))
	}
	return keys
}

// createTicket takes every holder key NX (a holder pointing at a waiting ticket wins) then writes the ticket and its
// queue entry. KEYS[1] ticket, KEYS[2] queue, KEYS[3] queues set, KEYS[4..] holders; ARGV[1] ticket id, ARGV[2] ttl s,
// ARGV[3] enqueued ms, ARGV[4] queue key, ARGV[5..] field/value pairs. Returns "" or the id of the holding ticket.
var createTicket = valkey.NewLuaScript(`
for i = 4, #KEYS do
  local cur = redis.call('GET', KEYS[i])
  if cur and cur ~= ARGV[1] then return cur end
end
for i = 4, #KEYS do redis.call('SET', KEYS[i], ARGV[1], 'EX', ARGV[2]) end
redis.call('DEL', KEYS[1])
redis.call('HSET', KEYS[1], unpack(ARGV, 5))
redis.call('EXPIRE', KEYS[1], ARGV[2])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
redis.call('SADD', KEYS[3], ARGV[4])
return ''`)

// Create implements port.MatchmakingStore.
func (s MatchmakingStore) Create(ctx context.Context, t port.Ticket) (uuid.UUID, error) {
	keys := append([]string{MMTicketKey(t.ID), MMQueueKey(t.Queue), MMQueuesKey}, holders(t)...)
	args := []string{t.ID.String(), strconv.FormatInt(int64(mmTicketTTL/time.Second), 10), ms(t.EnqueuedAt), t.Queue}
	args = append(args, ticketFields(t)...)
	res, err := createTicket.Exec(ctx, s.C, keys, args).ToString()
	if err != nil {
		return uuid.Nil, fmt.Errorf("matchmaking: create ticket: %w", err)
	}
	if res != "" {
		held, _ := uuid.Parse(res)
		return held, port.ErrTicketActive
	}
	return uuid.Nil, nil
}

func ticketFields(t port.Ticket) []string {
	return []string{"id", t.ID.String(), "tenant", t.TenantID.String(), "owner", t.OwnerID.String(), "queue", t.Queue, "mode", t.Mode,
		"type", t.Type, "scope_kind", t.ScopeKind, "scope_id", t.ScopeID.String(), "grade_key", t.GradeKey, "subject", t.SubjectID.String(),
		"topic", optID(t.TopicID), "party", optID(t.PartyID), "allow_fill", flag(t.AllowFill), "team_size", strconv.Itoa(t.TeamSize),
		"students", ids(t.Students), "schools", ids(t.Schools), "grade", strconv.Itoa(t.Grade),
		"mu", strconv.FormatFloat(t.Rating.Mu, 'f', 3, 64), "rd", strconv.FormatFloat(t.Rating.RD, 'f', 3, 64), "enq", ms(t.EnqueuedAt),
		"status", t.Status, "requeued", "0", "bot", "0"}
}

func parseTicket(h map[string]string) *port.Ticket {
	id, err := uuid.Parse(h["id"])
	if err != nil {
		return nil
	}
	t := &port.Ticket{ID: id, Queue: h["queue"], Mode: h["mode"], Type: h["type"], ScopeKind: h["scope_kind"], GradeKey: h["grade_key"],
		TopicID: parseOpt(h["topic"]), PartyID: parseOpt(h["party"]), AllowFill: h["allow_fill"] == "1", Students: parseIDs(h["students"]),
		Schools: parseIDs(h["schools"]), EnqueuedAt: fromMS(h["enq"]), Status: h["status"],
		CancelReason: h["reason"], ProposalID: parseOpt(h["proposal"]), MatchID: parseOpt(h["match"]), Requeued: h["requeued"] == "1",
		BotOffered: h["bot"] == "1"}
	t.TenantID, _ = uuid.Parse(h["tenant"])
	t.OwnerID, _ = uuid.Parse(h["owner"])
	t.ScopeID, _ = uuid.Parse(h["scope_id"])
	t.SubjectID, _ = uuid.Parse(h["subject"])
	t.TeamSize, _ = strconv.Atoi(h["team_size"])
	t.Grade, _ = strconv.Atoi(h["grade"])
	t.Rating.Mu, _ = strconv.ParseFloat(h["mu"], 64)
	t.Rating.RD, _ = strconv.ParseFloat(h["rd"], 64)
	if v := h["offline"]; v != "" {
		at := fromMS(v)
		t.OfflineSince = &at
	}
	return t
}

// Ticket implements port.MatchmakingStore.
func (s MatchmakingStore) Ticket(ctx context.Context, id uuid.UUID) (*port.Ticket, error) {
	h, err := s.C.Do(ctx, s.C.B().Hgetall().Key(MMTicketKey(id)).Build()).AsStrMap()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("matchmaking: ticket: %w", err)
	}
	if len(h) == 0 {
		return nil, nil
	}
	return parseTicket(h), nil
}

// TicketOfParty implements port.MatchmakingStore.
func (s MatchmakingStore) TicketOfParty(ctx context.Context, party uuid.UUID) (uuid.UUID, error) {
	v, err := s.C.Do(ctx, s.C.B().Get().Key(MMPartyKey(party)).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("matchmaking: party ticket: %w", err)
	}
	id, _ := uuid.Parse(v)
	return id, nil
}

// endTicket ends a ticket in one of the from statuses. KEYS[1] ticket, KEYS[2] queue, KEYS[3..] holders; ARGV[1] status,
// ARGV[2] reason, ARGV[3] from (comma list), ARGV[4] ended ttl s, ARGV[5] ticket id.
var endTicket = valkey.NewLuaScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return 0 end
local ok = false
for f in string.gmatch(ARGV[3], '[^,]+') do if f == st then ok = true end end
if not ok then return 0 end
redis.call('HSET', KEYS[1], 'status', ARGV[1], 'reason', ARGV[2])
redis.call('ZREM', KEYS[2], ARGV[5])
for i = 3, #KEYS do
  if redis.call('GET', KEYS[i]) == ARGV[5] then redis.call('DEL', KEYS[i]) end
end
redis.call('EXPIRE', KEYS[1], ARGV[4])
return 1`)

// End implements port.MatchmakingStore.
func (s MatchmakingStore) End(ctx context.Context, id uuid.UUID, status, reason string, from []string) (bool, error) {
	t, err := s.Ticket(ctx, id)
	if err != nil || t == nil {
		return false, err
	}
	keys := append([]string{MMTicketKey(id), MMQueueKey(t.Queue)}, holders(*t)...)
	n, err := endTicket.Exec(ctx, s.C, keys, []string{status, reason, strings.Join(from, ","),
		strconv.FormatInt(int64(mmEndedTTL/time.Second), 10), id.String()}).AsInt64()
	if err != nil {
		return false, fmt.Errorf("matchmaking: end ticket: %w", err)
	}
	return n == 1, nil
}

// flagTicket sets or clears flags of an existing ticket only (never recreates an expired hash). KEYS[1] ticket;
// ARGV pairs field/value, value "" deletes the field.
var flagTicket = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
for i = 1, #ARGV, 2 do
  if ARGV[i + 1] == '' then redis.call('HDEL', KEYS[1], ARGV[i]) else redis.call('HSET', KEYS[1], ARGV[i], ARGV[i + 1]) end
end
return 1`)

// Flag implements port.MatchmakingStore.
func (s MatchmakingStore) Flag(ctx context.Context, id uuid.UUID, botOffered *bool, offlineSince *time.Time, clearOffline bool) error {
	var args []string
	if botOffered != nil {
		args = append(args, "bot", flag(*botOffered))
	}
	if offlineSince != nil {
		args = append(args, "offline", ms(*offlineSince))
	} else if clearOffline {
		args = append(args, "offline", "")
	}
	if len(args) == 0 {
		return nil
	}
	if err := flagTicket.Exec(ctx, s.C, []string{MMTicketKey(id)}, args).Error(); err != nil {
		return fmt.Errorf("matchmaking: flag ticket: %w", err)
	}
	return nil
}

// Queues implements port.MatchmakingStore.
func (s MatchmakingStore) Queues(ctx context.Context) ([]string, error) {
	qs, err := s.C.Do(ctx, s.C.B().Smembers().Key(MMQueuesKey).Build()).AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("matchmaking: queues: %w", err)
	}
	return qs, nil
}

// dropQueue removes an empty queue from the set. KEYS[1] queue, KEYS[2] queues set; ARGV[1] queue key.
var dropQueue = valkey.NewLuaScript(`
if redis.call('ZCARD', KEYS[1]) == 0 then redis.call('SREM', KEYS[2], ARGV[1]) end
return 1`)

// Queued implements port.MatchmakingStore: entries whose ticket hash vanished are removed from the queue.
func (s MatchmakingStore) Queued(ctx context.Context, queue string, limit int) ([]port.Ticket, error) {
	key := MMQueueKey(queue)
	members, err := s.C.Do(ctx, s.C.B().Zrange().Key(key).Min("0").Max(strconv.Itoa(limit-1)).Build()).AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("matchmaking: queue: %w", err)
	}
	var out []port.Ticket
	if len(members) > 0 {
		cmds := make(valkey.Commands, len(members))
		for i, m := range members {
			cmds[i] = s.C.B().Hgetall().Key(mmPrefix + "ticket:" + m).Build()
		}
		var stale []string
		for i, r := range s.C.DoMulti(ctx, cmds...) {
			h, err := r.AsStrMap()
			if err != nil && !valkey.IsValkeyNil(err) {
				return nil, fmt.Errorf("matchmaking: queued ticket: %w", err)
			}
			t := parseTicket(h)
			if t == nil || t.Status != mm.StatusQueued || t.Queue != queue {
				stale = append(stale, members[i])
				continue
			}
			out = append(out, *t)
		}
		if len(stale) > 0 {
			if err := s.C.Do(ctx, s.C.B().Zrem().Key(key).Member(stale...).Build()).Error(); err != nil {
				return nil, fmt.Errorf("matchmaking: queue cleanup: %w", err)
			}
		}
	}
	if len(out) == 0 {
		if err := dropQueue.Exec(ctx, s.C, []string{key, MMQueuesKey}, []string{queue}).Error(); err != nil {
			return nil, fmt.Errorf("matchmaking: drop queue: %w", err)
		}
	}
	return out, nil
}

// Lock implements port.MatchmakingStore.
func (s MatchmakingStore) Lock(ctx context.Context, queue, owner string, ttl time.Duration) (bool, error) {
	err := s.C.Do(ctx, s.C.B().Set().Key(MMLockKey(queue)).Value(owner).Nx().PxMilliseconds(ttl.Milliseconds()).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("matchmaking: lock: %w", err)
	}
	return true, nil
}

var unlockQueue = valkey.NewLuaScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0`)

// Unlock implements port.MatchmakingStore.
func (s MatchmakingStore) Unlock(ctx context.Context, queue, owner string) error {
	if err := unlockQueue.Exec(ctx, s.C, []string{MMLockKey(queue)}, []string{owner}).Error(); err != nil {
		return fmt.Errorf("matchmaking: unlock: %w", err)
	}
	return nil
}

// proposalDoc is the immutable part of a proposal (JSON in the proposal hash): ids only, no name (R14, R16).
type proposalDoc struct {
	ID               uuid.UUID               `json:"id"`
	Queue            string                  `json:"queue,omitempty"`
	Tickets          []mm.ProposalTicket     `json:"tickets"`
	Teams            [2][]uuid.UUID          `json:"teams"`
	Deadline         int64                   `json:"deadline"`
	TenantIDs        []uuid.UUID             `json:"tenant_ids"`
	Mode             string                  `json:"mode"`
	Type             string                  `json:"type"`
	ScopeKind        string                  `json:"scope_kind"`
	SubjectID        uuid.UUID               `json:"subject_id"`
	TopicID          *uuid.UUID              `json:"topic_id,omitempty"`
	Grade            int                     `json:"grade"`
	Ranked           bool                    `json:"ranked"`
	RulesetVersionID uuid.UUID               `json:"ruleset_version_id"`
	TournamentNodeID *uuid.UUID              `json:"tournament_node_id,omitempty"`
	TournamentID     *uuid.UUID              `json:"tournament_id,omitempty"`
	Direct           bool                    `json:"direct,omitempty"`
	Schools          map[uuid.UUID]uuid.UUID `json:"schools"`
	Tenants          map[uuid.UUID]uuid.UUID `json:"tenants"`
	Parties          map[uuid.UUID]uuid.UUID `json:"parties,omitempty"`
	Ratings          map[uuid.UUID]mm.Rating `json:"ratings,omitempty"`
	CreatedAt        int64                   `json:"created_at"`
}

// proposeTickets creates a proposal and moves its tickets from their queue to proposed. KEYS[1] proposal, KEYS[2]
// proposals zset, then (ticket, queue) pairs; ARGV[1] proposal id, ARGV[2] deadline ms, ARGV[3] doc, ARGV[4] students,
// ARGV[5] ttl ms, ARGV[6..] ticket ids (same order as the pairs).
var proposeTickets = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
local n = (#KEYS - 2) / 2
for i = 1, n do
  if redis.call('HGET', KEYS[1 + 2 * i], 'status') ~= 'queued' then return 0 end
end
for i = 1, n do
  redis.call('ZREM', KEYS[2 + 2 * i], ARGV[5 + i])
  redis.call('HSET', KEYS[1 + 2 * i], 'status', 'proposed', 'proposal', ARGV[1])
end
redis.call('HSET', KEYS[1], 'doc', ARGV[3], 'status', 'open', 'deadline', ARGV[2], 'n', ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
redis.call('ZADD', KEYS[2], ARGV[2], ARGV[1])
return 1`)

// Propose implements port.MatchmakingStore (a direct proposal has no ticket to move).
func (s MatchmakingStore) Propose(ctx context.Context, p port.Proposal) (bool, error) {
	doc := proposalDoc{ID: p.ID, Queue: p.Queue, Tickets: p.Tickets, Teams: p.Teams, Deadline: p.Deadline.UnixMilli(), TenantIDs: p.TenantIDs,
		Mode: p.Mode, Type: p.Type, ScopeKind: p.ScopeKind, SubjectID: p.SubjectID, TopicID: p.TopicID, Grade: p.Grade, Ranked: p.Ranked,
		RulesetVersionID: p.RulesetVersionID, TournamentNodeID: p.TournamentNodeID, TournamentID: p.TournamentID, Direct: p.Direct, Schools: p.Schools, Tenants: p.Tenants,
		Parties: p.Parties, Ratings: p.Ratings, CreatedAt: p.CreatedAt.UnixMilli()}
	b, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}
	keys := []string{MMProposalKey(p.ID), MMProposalsKey}
	ttl := p.Deadline.Sub(p.CreatedAt) + mmProposalMargin
	if ttl < mmProposalMargin {
		ttl = mmProposalMargin
	}
	args := []string{p.ID.String(), strconv.FormatInt(p.Deadline.UnixMilli(), 10), string(b), strconv.Itoa(len(p.Students())),
		strconv.FormatInt(ttl.Milliseconds(), 10)}
	if !p.Direct {
		for _, t := range p.Tickets {
			keys = append(keys, MMTicketKey(t.TicketID), MMQueueKey(p.Queue))
			args = append(args, t.TicketID.String())
		}
	}
	n, err := proposeTickets.Exec(ctx, s.C, keys, args).AsInt64()
	if err != nil {
		return false, fmt.Errorf("matchmaking: propose: %w", err)
	}
	return n == 1, nil
}

// Proposal implements port.MatchmakingStore.
func (s MatchmakingStore) Proposal(ctx context.Context, id uuid.UUID) (*port.Proposal, error) {
	h, err := s.C.Do(ctx, s.C.B().Hgetall().Key(MMProposalKey(id)).Build()).AsStrMap()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("matchmaking: proposal: %w", err)
	}
	if h["doc"] == "" {
		return nil, nil
	}
	var d proposalDoc
	if err := json.Unmarshal([]byte(h["doc"]), &d); err != nil {
		return nil, fmt.Errorf("matchmaking: proposal doc: %w", err)
	}
	p := &port.Proposal{Proposal: mm.Proposal{ID: d.ID, Queue: d.Queue, Tickets: d.Tickets, Teams: d.Teams, Accepted: map[uuid.UUID]bool{},
		Deadline: time.UnixMilli(d.Deadline).UTC(), Status: h["status"]}, TenantIDs: d.TenantIDs, Mode: d.Mode, Type: d.Type,
		ScopeKind: d.ScopeKind, SubjectID: d.SubjectID, TopicID: d.TopicID, Grade: d.Grade, Ranked: d.Ranked, RulesetVersionID: d.RulesetVersionID,
		TournamentNodeID: d.TournamentNodeID, TournamentID: d.TournamentID, Direct: d.Direct, Schools: d.Schools, Tenants: d.Tenants, Parties: d.Parties, Ratings: d.Ratings,
		CreatedAt: time.UnixMilli(d.CreatedAt).UTC()}
	for k := range h {
		if strings.HasPrefix(k, "acc:") {
			if sid, err := uuid.Parse(k[4:]); err == nil {
				p.Accepted[sid] = true
			}
		}
	}
	if v := h["accepted_at"]; v != "" {
		at := fromMS(v)
		p.AcceptedAt = &at
	}
	return p, nil
}

// acceptProposal records a ready. KEYS[1] proposal, KEYS[2] proposals zset; ARGV[1] student, ARGV[2] now ms, ARGV[3]
// proposal id, ARGV[4] retry delay ms, ARGV[5] accepted ttl ms. Returns -1 closed, 1 recorded, 2 last accept.
var acceptProposal = valkey.NewLuaScript(`
if redis.call('HGET', KEYS[1], 'status') ~= 'open' then return -1 end
if tonumber(redis.call('HGET', KEYS[1], 'deadline')) <= tonumber(ARGV[2]) then return -1 end
redis.call('HSET', KEYS[1], 'acc:' .. ARGV[1], '1')
local n = tonumber(redis.call('HGET', KEYS[1], 'n'))
local acc = 0
for _, f in ipairs(redis.call('HKEYS', KEYS[1])) do
  if string.sub(f, 1, 4) == 'acc:' then acc = acc + 1 end
end
if acc >= n then
  redis.call('HSET', KEYS[1], 'status', 'accepted', 'accepted_at', ARGV[2])
  redis.call('PEXPIRE', KEYS[1], ARGV[5])
  redis.call('ZADD', KEYS[2], tonumber(ARGV[2]) + tonumber(ARGV[4]), ARGV[3])
  return 2
end
return 1`)

// AcceptRetry is how long an accepted proposal waits before the matchmaker retries its match.
const AcceptRetry = 5 * time.Second

// Accept implements port.MatchmakingStore.
func (s MatchmakingStore) Accept(ctx context.Context, id, student uuid.UUID, now time.Time) (*port.Proposal, bool, error) {
	n, err := acceptProposal.Exec(ctx, s.C, []string{MMProposalKey(id), MMProposalsKey}, []string{student.String(), ms(now), id.String(),
		strconv.FormatInt(AcceptRetry.Milliseconds(), 10), strconv.FormatInt(mmAcceptedTTL.Milliseconds(), 10)}).AsInt64()
	if err != nil {
		return nil, false, fmt.Errorf("matchmaking: accept: %w", err)
	}
	if n < 0 {
		return nil, false, mm.ErrProposalClosed
	}
	p, err := s.Proposal(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if p == nil {
		return nil, false, mm.ErrProposalClosed
	}
	return p, n == 2, nil
}

// cancelProposal cancels a proposal in one of the from statuses, requeues and ends tickets. KEYS[1] proposal, KEYS[2]
// proposals zset, KEYS[3] queues set, then per requeued ticket (ticket, queue), then per cancelled ticket (ticket,
// queue, holders…) delimited by ARGV counts. ARGV[1] proposal id, ARGV[2] from, ARGV[3] reason, ARGV[4] ended ttl s,
// ARGV[5] number requeued, ARGV[6] queue key, ARGV[7..] for requeued: ticket id; then for cancelled: ticket id,
// number of holder keys.
var cancelProposal = valkey.NewLuaScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return 0 end
local ok = false
for f in string.gmatch(ARGV[2], '[^,]+') do if f == st then ok = true end end
if not ok then return 0 end
redis.call('HSET', KEYS[1], 'status', 'cancelled')
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('PEXPIRE', KEYS[1], 60000)
local k, a = 4, 7
for i = 1, tonumber(ARGV[5]) do
  local tk, q, id = KEYS[k], KEYS[k + 1], ARGV[a]
  if redis.call('HGET', tk, 'status') == 'proposed' and redis.call('HGET', tk, 'proposal') == ARGV[1] then
    redis.call('HSET', tk, 'status', 'queued', 'requeued', '1')
    redis.call('HDEL', tk, 'proposal')
    redis.call('ZADD', q, redis.call('HGET', tk, 'enq'), id)
    redis.call('SADD', KEYS[3], ARGV[6])
  end
  k, a = k + 2, a + 1
end
while a <= #ARGV do
  local tk, q, id, nh = KEYS[k], KEYS[k + 1], ARGV[a], tonumber(ARGV[a + 1])
  if redis.call('HGET', tk, 'status') == 'proposed' and redis.call('HGET', tk, 'proposal') == ARGV[1] then
    redis.call('HSET', tk, 'status', 'cancelled', 'reason', ARGV[3])
    redis.call('ZREM', q, id)
    for j = 1, nh do
      if redis.call('GET', KEYS[k + 1 + j]) == id then redis.call('DEL', KEYS[k + 1 + j]) end
    end
    redis.call('EXPIRE', tk, ARGV[4])
  end
  k, a = k + 2 + nh, a + 2
end
return 1`)

// Cancel implements port.MatchmakingStore.
func (s MatchmakingStore) Cancel(ctx context.Context, id uuid.UUID, res mm.Resolution, from []string) (bool, error) {
	queue := ""
	keys := []string{MMProposalKey(id), MMProposalsKey, MMQueuesKey}
	var reqArgs, cancelArgs []string
	var cancelKeys []string
	for _, tid := range res.Requeue {
		t, err := s.Ticket(ctx, tid)
		if err != nil {
			return false, err
		}
		if t == nil {
			continue
		}
		queue = t.Queue
		keys = append(keys, MMTicketKey(tid), MMQueueKey(t.Queue))
		reqArgs = append(reqArgs, tid.String())
	}
	for _, tid := range res.Cancel {
		t, err := s.Ticket(ctx, tid)
		if err != nil {
			return false, err
		}
		if t == nil {
			continue
		}
		hs := holders(*t)
		cancelKeys = append(cancelKeys, MMTicketKey(tid), MMQueueKey(t.Queue))
		cancelKeys = append(cancelKeys, hs...)
		cancelArgs = append(cancelArgs, tid.String(), strconv.Itoa(len(hs)))
	}
	keys = append(keys, cancelKeys...)
	args := []string{id.String(), strings.Join(from, ","), res.Reason, strconv.FormatInt(int64(mmEndedTTL/time.Second), 10),
		strconv.Itoa(len(reqArgs)), queue}
	args = append(append(args, reqArgs...), cancelArgs...)
	n, err := cancelProposal.Exec(ctx, s.C, keys, args).AsInt64()
	if err != nil {
		return false, fmt.Errorf("matchmaking: cancel proposal: %w", err)
	}
	return n == 1, nil
}

// completeProposal marks an accepted proposal and its tickets matched. KEYS[1] proposal, KEYS[2] proposals zset, then
// per ticket (ticket, holders…); ARGV[1] proposal id, ARGV[2] match id, ARGV[3] ended ttl s, ARGV[4..] per ticket:
// ticket id, number of holder keys.
var completeProposal = valkey.NewLuaScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if st ~= 'accepted' and st ~= 'matched' then return 0 end
redis.call('HSET', KEYS[1], 'status', 'matched')
redis.call('ZREM', KEYS[2], ARGV[1])
local k, a = 3, 4
while a <= #ARGV do
  local tk, id, nh = KEYS[k], ARGV[a], tonumber(ARGV[a + 1])
  if redis.call('HGET', tk, 'proposal') == ARGV[1] then
    redis.call('HSET', tk, 'status', 'matched', 'match', ARGV[2])
    for j = 1, nh do
      if redis.call('GET', KEYS[k + j]) == id then redis.call('DEL', KEYS[k + j]) end
    end
    redis.call('EXPIRE', tk, ARGV[3])
  end
  k, a = k + 1 + nh, a + 2
end
return 1`)

// Complete implements port.MatchmakingStore.
func (s MatchmakingStore) Complete(ctx context.Context, id, match uuid.UUID) error {
	p, err := s.Proposal(ctx, id)
	if err != nil || p == nil {
		return err
	}
	keys := []string{MMProposalKey(id), MMProposalsKey}
	args := []string{id.String(), match.String(), strconv.FormatInt(int64(mmEndedTTL/time.Second), 10)}
	if !p.Direct {
		for _, pt := range p.Tickets {
			t, err := s.Ticket(ctx, pt.TicketID)
			if err != nil {
				return err
			}
			if t == nil {
				continue
			}
			hs := holders(*t)
			keys = append(append(keys, MMTicketKey(pt.TicketID)), hs...)
			args = append(args, pt.TicketID.String(), strconv.Itoa(len(hs)))
		}
	}
	if err := completeProposal.Exec(ctx, s.C, keys, args).Error(); err != nil {
		return fmt.Errorf("matchmaking: complete proposal: %w", err)
	}
	return nil
}

// Due implements port.MatchmakingStore (stale is applied when the proposal is accepted: AcceptRetry).
func (s MatchmakingStore) Due(ctx context.Context, now time.Time, _ time.Duration) ([]uuid.UUID, error) {
	members, err := s.C.Do(ctx, s.C.B().Zrangebyscore().Key(MMProposalsKey).Min("-inf").Max(ms(now)).Limit(0, 500).Build()).AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("matchmaking: due proposals: %w", err)
	}
	out := make([]uuid.UUID, 0, len(members))
	var gone []string
	for _, m := range members {
		id, err := uuid.Parse(m)
		if err != nil {
			gone = append(gone, m)
			continue
		}
		out = append(out, id)
	}
	if len(gone) > 0 {
		_ = s.C.Do(ctx, s.C.B().Zrem().Key(MMProposalsKey).Member(gone...).Build()).Error()
	}
	return out, nil
}

// slidingWindow counts one hit in a sorted-set window when below limit. KEYS[1] zset; ARGV[1] now ms, ARGV[2] window
// ms, ARGV[3] limit, ARGV[4] member. Returns 0 when counted, else the wait in ms.
var slidingWindow = valkey.NewLuaScript(`
local now, win, limit = tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - win)
if redis.call('ZCARD', KEYS[1]) >= limit then
  local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  local wait = tonumber(oldest[2]) + win - now
  if wait < 1 then wait = 1 end
  return wait
end
redis.call('ZADD', KEYS[1], now, ARGV[4])
redis.call('PEXPIRE', KEYS[1], win)
return 0`)

// TicketRate implements port.MatchmakingStore (mm_ticket_rate, 1 h sliding window).
func (s MatchmakingStore) TicketRate(ctx context.Context, student uuid.UUID, now time.Time, perHour int) (time.Duration, error) {
	if perHour <= 0 {
		return 0, nil
	}
	wait, err := slidingWindow.Exec(ctx, s.C, []string{MMRateKey(student)}, []string{ms(now), strconv.FormatInt(time.Hour.Milliseconds(), 10),
		strconv.Itoa(perHour), uuid.NewString()}).AsInt64()
	if err != nil {
		return 0, fmt.Errorf("matchmaking: ticket rate: %w", err)
	}
	return time.Duration(wait) * time.Millisecond, nil
}

// Cooldown implements port.MatchmakingStore.
func (s MatchmakingStore) Cooldown(ctx context.Context, student uuid.UUID) (time.Duration, error) {
	n, err := s.C.Do(ctx, s.C.B().Pttl().Key(MMCooldownKey(student)).Build()).AsInt64()
	if err != nil {
		return 0, fmt.Errorf("matchmaking: cooldown: %w", err)
	}
	if n <= 0 {
		return 0, nil
	}
	return time.Duration(n) * time.Millisecond, nil
}

// blame counts a decline/timeout and starts the cooldown at limit. KEYS[1] declines zset, KEYS[2] cooldown; ARGV[1] now
// ms, ARGV[2] window ms, ARGV[3] limit, ARGV[4] cooldown ms, ARGV[5] member.
var blame = valkey.NewLuaScript(`
local now, win = tonumber(ARGV[1]), tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - win)
redis.call('ZADD', KEYS[1], now, ARGV[5])
redis.call('PEXPIRE', KEYS[1], win)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[4])
  redis.call('DEL', KEYS[1])
end
return 1`)

// Blame implements port.MatchmakingStore.
func (s MatchmakingStore) Blame(ctx context.Context, student uuid.UUID, now time.Time, limit int, window, cooldown time.Duration) error {
	if limit <= 0 || window <= 0 || cooldown <= 0 {
		return nil
	}
	if err := blame.Exec(ctx, s.C, []string{MMDeclinesKey(student), MMCooldownKey(student)}, []string{ms(now),
		strconv.FormatInt(window.Milliseconds(), 10), strconv.Itoa(limit), strconv.FormatInt(cooldown.Milliseconds(), 10), uuid.NewString()}).Error(); err != nil {
		return fmt.Errorf("matchmaking: blame: %w", err)
	}
	return nil
}

// Statuses implements port.MatchmakingStore.
func (s MatchmakingStore) Statuses(ctx context.Context, list []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(list))
	if len(list) == 0 {
		return out, nil
	}
	cmds := make(valkey.Commands, len(list))
	for i, id := range list {
		cmds[i] = s.C.B().Hget().Key(MMTicketKey(id)).Field("status").Build()
	}
	for i, r := range s.C.DoMulti(ctx, cmds...) {
		v, err := r.ToString()
		if valkey.IsValkeyNil(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("matchmaking: statuses: %w", err)
		}
		out[list[i]] = v
	}
	return out, nil
}
