package battle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
)

// States of a match room (REALTIME_ARCHITECTURE §7.1). closing waits for the reveal data of module learning after the
// question closed; finalizing waits for the durable result.
const (
	StateLobby      = "lobby"
	StateCountdown  = "countdown"
	StateQuestion   = "question_shown"
	StateClosing    = "closing"
	StateReveal     = "reveal"
	StateFinalizing = "finalizing"
	StateEnded      = "ended"
	StateAborted    = "aborted"
)

// End reasons (competition.matches.end_reason).
const (
	EndFinished      = "finished"
	EndAbandon       = "abandon"
	EndForfeit       = "forfeit"
	EndAllLeft       = "all_left"
	EndServerFailure = "server_failure"
	EndPlanFailure   = "plan_failure"
)

// Outcomes of a participant (competition.match_participants.outcome).
const (
	OutcomeWin       = "win"
	OutcomeLoss      = "loss"
	OutcomeDraw      = "draw"
	OutcomeAbandoned = "abandoned"
	OutcomeForfeit   = "forfeit"
	OutcomeNoContest = "no_contest"
)

// Integrity signals of the room (competition.integrity_signals.signal_type, ANTI_CHEAT §3, §6): never a sanction.
const (
	SignalFastAnswer     = "fast_answer"
	SignalBeforeShown    = "answer_before_shown"
	SignalClosedQuestion = "answer_closed_question"
)

// Membership changes the app writes in multiplayer.session_memberships (MULTIPLAYER_GAMEPLAY §10).
const (
	MemberJoin       = "join"
	MemberDisconnect = "disconnect"
	MemberLeave      = "leave"
)

// Leave reasons (session_memberships.leave_reason).
const (
	LeaveTimeout  = "timeout"
	LeaveClosed   = "room_closed"
	LeaveReplaced = "replaced_by_other_device"
)

// Errors of a command on the room (mapped to the realtime ErrorCode by the presentation).
var (
	ErrNotMember        = errors.New("battle: not a member of the room")         // ROOM_NOT_MEMBER
	ErrNotActive        = errors.New("battle: the match does not take answers")  // MATCH_NOT_ACTIVE
	ErrNotOpen          = errors.New("battle: the question is not open")         // VALIDATION_FAILED (+ signal)
	ErrLate             = errors.New("battle: answer after the deadline")        // ANSWER_LATE
	ErrAlreadySubmitted = errors.New("battle: the question is already answered") // ANSWER_ALREADY_SUBMITTED
	ErrOut              = errors.New("battle: the player left the match")        // MATCH_NOT_ACTIVE
	ErrFull             = errors.New("battle: too many players for the mode")    // ROOM_FULL
	ErrInvalidSpec      = errors.New("battle: invalid room")
)

// Item is one question of the pinned plan: its instance id (never the question version id), type, time limit and the
// render payload of module learning (no answer data, ADR-0016).
type Item struct {
	Index    int             `json:"index"`
	Instance string          `json:"instance"`
	Type     string          `json:"type"`
	Limit    time.Duration   `json:"limit"`
	Render   json.RawMessage `json:"render"`
}

// InstanceID is the question_instance_id of question index of a match: stable per match (failover), it reveals nothing
// of the question version.
func InstanceID(match uuid.UUID, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", match, index)))
	return "qi_" + hex.EncodeToString(sum[:8])
}

// PlayerSpec is one student of the match.
type PlayerSpec struct {
	ID       uuid.UUID
	Tenant   uuid.UUID
	School   uuid.UUID
	Team     int
	Nickname string
}

// Spec builds a room.
type Spec struct {
	MatchID     uuid.UUID
	PlanID      uuid.UUID
	PlanVersion string
	Grade       int
	Ranked      bool
	IsBot       bool
	Tournament  bool
	Rules       Rules
	Players     []PlayerSpec
	Items       []Item
	Bot         *BotProfile
}

// Player is the state of one student in the room.
type Player struct {
	ID          uuid.UUID `json:"id"`
	Tenant      uuid.UUID `json:"tenant"`
	School      uuid.UUID `json:"school"`
	Team        int       `json:"team"`
	Nickname    string    `json:"nickname"`
	Session     uuid.UUID `json:"session"`
	Joined      bool      `json:"joined"`
	Connected   bool      `json:"connected"`
	Since       time.Time `json:"since"`
	ConnectedMS int64     `json:"connected_ms"`
	GraceUntil  time.Time `json:"grace_until"`
	Graces      int       `json:"graces"`
	Out         string    `json:"out,omitempty"`
	Score       int       `json:"score"`
	Correct     int       `json:"correct"`
	FastRun     int       `json:"fast_run"`
}

// Answer is the answer of one player to one question.
type Answer struct {
	Student  uuid.UUID       `json:"student"`
	At       time.Time       `json:"at"`
	Response json.RawMessage `json:"-"` // never checkpointed: a replayed question takes new answers
	State    string          `json:"state"`
	Correct  bool            `json:"correct"`
	Points   int             `json:"points"`
	Bonus    int             `json:"bonus"`
	Support  bool            `json:"support"`
	Fast     bool            `json:"fast"`
}

// Answer states.
const (
	answerPending = "pending"
	answerScored  = "scored"
	answerFailed  = "failed"
)

// Bot is the practice bot of a bot match (team 1, never a participant row).
type Bot struct {
	ID       uuid.UUID  `json:"id"`
	Profile  BotProfile `json:"profile"`
	Score    int        `json:"score"`
	Correct  int        `json:"correct"`
	Answered bool       `json:"answered"`
	Due      time.Time  `json:"due"`
	Right    bool       `json:"right"`
	Points   int        `json:"points"`
}

// Room is the aggregate of a match room. Its exported fields are the checkpoint (ew:rt:room:<id>:ckpt).
type Room struct {
	ID          uuid.UUID                     `json:"match_id"`
	PlanID      uuid.UUID                     `json:"plan_id"`
	PlanVersion string                        `json:"plan_version"`
	Grade       int                           `json:"grade"`
	Ranked      bool                          `json:"ranked"`
	IsBot       bool                          `json:"is_bot"`
	Tournament  bool                          `json:"tournament"`
	R           Rules                         `json:"rules"`
	Items       []Item                        `json:"items"`
	Players     []*Player                     `json:"players"`
	Bot         *Bot                          `json:"bot,omitempty"`
	St          string                        `json:"state"`
	Version     int64                         `json:"state_version"`
	Cur         int                           `json:"current"`
	Shown       time.Time                     `json:"shown_at"`
	Deadline    time.Time                     `json:"deadline_at"`
	Closed      time.Time                     `json:"closed_at"`
	NextAt      time.Time                     `json:"next_at"`
	Answers     map[int]map[uuid.UUID]*Answer `json:"answers"`
	Results     []QuestionResult              `json:"results"`
	Replays     int                           `json:"replays"`
	Started     bool                          `json:"started"`
	StartedAt   time.Time                     `json:"started_at"`
	EndReason   string                        `json:"end_reason,omitempty"`
	Cancelled   bool                          `json:"cancelled"`
	EndedAt     time.Time                     `json:"ended_at"`
	RetryAt     time.Time                     `json:"retry_at"`
	CoopFires   int                           `json:"coop_fires"`
}

// QuestionResult is a closed question (match_results.summary.questions[]).
type QuestionResult struct {
	Index    int            `json:"index"`
	ClosedAt time.Time      `json:"closed_at"`
	Answers  []AnswerResult `json:"per_student"`
	BotRight *bool          `json:"bot_correct,omitempty"`
}

// AnswerResult is the result of one student for a closed question.
type AnswerResult struct {
	Student    uuid.UUID `json:"student_id"`
	Answered   bool      `json:"answered"`
	Correct    bool      `json:"is_correct"`
	Points     int       `json:"points"`
	Bonus      int       `json:"time_bonus"`
	ReceivedMS int64     `json:"received_ms"`
	Support    bool      `json:"support,omitempty"`
}

// Message is a realtime message for the room (To nil: every player).
type Message struct {
	Type    string
	Payload map[string]any
	To      []uuid.UUID
}

// Signal is an integrity signal to record (no raw answer).
type Signal struct {
	Student  uuid.UUID
	Type     string
	Index    int
	Evidence map[string]any
}

// Membership is a change of a session membership.
type Membership struct {
	Student uuid.UUID
	Session uuid.UUID
	Kind    string
	Reason  string
}

// Step is what a command or a tick asks the room host to do: publish Messages, record Signals and Memberships, write
// the checkpoint, publish match.started, fetch the reveal of a question, record the result or the abort.
type Step struct {
	Messages    []Message
	Signals     []Signal
	Memberships []Membership
	Checkpoint  bool
	Started     bool
	Reveal      *int
	Finish      bool
	Abort       string
}

func (s *Step) push(typ string, payload map[string]any, to ...uuid.UUID) {
	s.Messages = append(s.Messages, Message{Type: typ, Payload: payload, To: to})
}

func (s *Step) merge(o Step) {
	s.Messages = append(s.Messages, o.Messages...)
	s.Signals = append(s.Signals, o.Signals...)
	s.Memberships = append(s.Memberships, o.Memberships...)
	s.Checkpoint = s.Checkpoint || o.Checkpoint
	s.Started = s.Started || o.Started
	if o.Reveal != nil {
		s.Reveal = o.Reveal
	}
	s.Finish = s.Finish || o.Finish
	if o.Abort != "" {
		s.Abort = o.Abort
	}
}

// Pending is an accepted answer the app grades with module learning (scorePlanAnswer).
type Pending struct {
	Index      int
	Student    uuid.UUID
	ReceivedAt time.Time
	Response   json.RawMessage
}

// Reveal is what module learning returns once a question closed (revealPlanItem).
type Reveal struct {
	CorrectResponse json.RawMessage
	Explanation     string
	// Question is the render payload of the question (match review only, never sent before the match ended).
	Question json.RawMessage
}

// BotID is the id of the practice bot of a match (stable, never a student id).
func BotID(match uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("eduworld:battle:bot:"+match.String()))
}

// New builds a room in the lobby: the players have lobby_wait to connect.
func New(s Spec, now time.Time) (*Room, error) {
	max := s.Rules.MaxPlayers
	if max <= 0 {
		max = 2
	}
	if len(s.Players) > max {
		return nil, fmt.Errorf("%w: %d players, at most %d", ErrFull, len(s.Players), max)
	}
	if s.MatchID == uuid.Nil || len(s.Players) == 0 || len(s.Items) == 0 {
		return nil, ErrInvalidSpec
	}
	r := &Room{ID: s.MatchID, PlanID: s.PlanID, PlanVersion: s.PlanVersion, Grade: s.Grade, Ranked: s.Ranked, IsBot: s.IsBot,
		Tournament: s.Tournament, R: s.Rules, Items: s.Items, St: StateLobby, Cur: -1, NextAt: now.Add(s.Rules.LobbyWait),
		Answers: map[int]map[uuid.UUID]*Answer{}, Version: 1}
	for _, p := range s.Players {
		r.Players = append(r.Players, &Player{ID: p.ID, Tenant: p.Tenant, School: p.School, Team: p.Team, Nickname: p.Nickname})
	}
	if s.Bot != nil {
		r.Bot = &Bot{ID: BotID(s.MatchID), Profile: *s.Bot}
	}
	return r, nil
}

// Restore reads a checkpoint.
func Restore(b []byte) (*Room, error) {
	var r Room
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("battle checkpoint: %w", err)
	}
	if r.ID == uuid.Nil || len(r.Players) == 0 || len(r.Items) == 0 {
		return nil, ErrInvalidSpec
	}
	if r.Answers == nil {
		r.Answers = map[int]map[uuid.UUID]*Answer{}
	}
	return &r, nil
}

// State is the state of the room.
func (r *Room) State() string { return r.St }

// Rules are the rules of the match.
func (r *Room) Rules() Rules { return r.R }

// Current is the index of the current question (-1 before the first).
func (r *Room) Current() int { return r.Cur }

// ShownAt is when the current question was shown (server time).
func (r *Room) ShownAt() time.Time { return r.Shown }

// DeadlineAt is the deadline of the current question.
func (r *Room) DeadlineAt() time.Time { return r.Deadline }

// CurrentInstance is the question_instance_id of the current question.
func (r *Room) CurrentInstance() string {
	if r.Cur < 0 || r.Cur >= len(r.Items) {
		return ""
	}
	return r.Items[r.Cur].Instance
}

// Done reports a room whose match is over (ended or aborted).
func (r *Room) Done() bool { return r.St == StateEnded || r.St == StateAborted }

func (r *Room) player(id uuid.UUID) *Player {
	for _, p := range r.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Member reports a player of the room.
func (r *Room) Member(id uuid.UUID) bool { return r.player(id) != nil }

// Tenants are the tenants of the players.
func (r *Room) Tenants() []uuid.UUID {
	var out []uuid.UUID
	for _, p := range r.Players {
		if !slices.Contains(out, p.Tenant) {
			out = append(out, p.Tenant)
		}
	}
	return out
}

func (r *Room) ids(except ...uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{}
	for _, p := range r.Players {
		if !slices.Contains(except, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out
}

func (r *Room) transition(state string) {
	r.St = state
	r.Version++
}

func ms(t time.Time) int64 { return t.UnixMilli() }

func (r *Room) running() bool {
	switch r.St {
	case StateCountdown, StateQuestion, StateClosing, StateReveal:
		return true
	}
	return false
}

// Join connects (or reconnects) a player (F105-AC-11, AC-14, AC-22): a first join records the membership, a new game
// session of the same student (takeover, ADR-0031) closes the old membership as replaced_by_other_device; back within
// the grace the others see it connected; the player gets the snapshot of the current state.
func (r *Room) Join(student, session uuid.UUID, now time.Time) (Step, error) {
	var s Step
	p := r.player(student)
	if p == nil {
		return s, ErrNotMember
	}
	if p.Out != "" {
		return s, ErrOut
	}
	switch {
	case !p.Joined:
		p.Joined = true
		s.Memberships = append(s.Memberships, Membership{Student: student, Session: session, Kind: MemberJoin})
	case session != uuid.Nil && p.Session != uuid.Nil && session != p.Session:
		s.Memberships = append(s.Memberships, Membership{Student: student, Session: p.Session, Kind: MemberLeave, Reason: LeaveReplaced},
			Membership{Student: student, Session: session, Kind: MemberJoin})
	}
	if session != uuid.Nil {
		p.Session = session
	}
	if !p.Connected && !r.Done() {
		wasAway := !p.GraceUntil.IsZero()
		p.Connected, p.GraceUntil, p.Since = true, time.Time{}, now
		if wasAway {
			s.push("battle.participant.status_changed", map[string]any{"match_id": r.ID.String(), "user_id": student.String(),
				"connected": true}, r.ids(student)...)
		}
		s.Checkpoint = true
	}
	s.Messages = append(s.Messages, r.Snapshot(student))
	return s, nil
}

// Snapshot is battle.state.snapshot (coop.state.snapshot for a co-op room) for viewer: the current question without
// answer data; in a timed quiz the scores of the other students are not shown.
func (r *Room) Snapshot(viewer uuid.UUID) Message {
	parts := []map[string]any{}
	for _, p := range r.Players {
		e := map[string]any{"player": map[string]any{"user_id": p.ID.String(), "nickname": p.Nickname}, "team": p.Team, "connected": p.Connected}
		if r.R.Mode != ModeTimedQuiz || p.ID == viewer {
			e["score"] = p.Score
		}
		parts = append(parts, e)
	}
	if r.Bot != nil {
		parts = append(parts, map[string]any{"player": map[string]any{"user_id": r.Bot.ID.String(), "nickname": "Robot"}, "team": 1,
			"connected": true, "score": r.Bot.Score, "bot": true})
	}
	if r.R.Collective {
		payload := map[string]any{"room_id": r.ID.String(), "state_version": r.Version, "puzzle_state": r.coopState(),
			"members": parts, "shared_progress": r.progress()}
		return Message{Type: "coop.state.snapshot", Payload: payload, To: []uuid.UUID{viewer}}
	}
	state := r.St
	switch state {
	case StateClosing:
		state = StateReveal
	case StateAborted:
		state = "cancelled"
	case StateEnded:
		if r.Cancelled {
			state = "cancelled"
		}
	}
	payload := map[string]any{"match_id": r.ID.String(), "mode": r.R.Mode, "state": state, "state_version": r.Version, "round": 1,
		"question_index": r.Cur, "question_count": len(r.Items), "participants": parts, "speed_bonus_enabled": r.R.SpeedBonus}
	if chat := r.chatChannels(viewer); chat != nil {
		payload["chat_channel_ids"] = chat
	}
	if r.St == StateQuestion {
		payload["current_question"] = r.render(r.Cur)
		payload["deadline_at"] = ms(r.Deadline)
		_, locked := r.Answers[r.Cur][viewer]
		payload["my_answer_locked"] = locked
		if primary := r.primaryTurns(); primary != nil {
			payload["primary_user_ids"] = primary
		}
	}
	return Message{Type: "battle.state.snapshot", Payload: payload, To: []uuid.UUID{viewer}}
}

// primaryTurns are the players answering the main turn of the current question, one per team in team order (team
// battle rotation, F105-AC-17); nil outside a team battle.
func (r *Room) primaryTurns() []string {
	if !r.R.Rotation {
		return nil
	}
	var teams []int
	for _, p := range r.Players {
		if !slices.Contains(teams, p.Team) {
			teams = append(teams, p.Team)
		}
	}
	slices.Sort(teams)
	out := make([]string, 0, len(teams))
	for _, t := range teams {
		if id := r.MainTurn(t); id != uuid.Nil {
			out = append(out, id.String())
		}
	}
	return out
}

// render is the QuestionRender of question i: the render payload of module learning (no answer data, ADR-0016) with
// both option keys (id canonical, option_id alias).
func (r *Room) render(i int) map[string]any {
	it := r.Items[i]
	q := map[string]any{}
	_ = json.Unmarshal(it.Render, &q)
	aliasOptions(q)
	q["question_instance_id"] = it.Instance
	q["question_type"] = it.Type
	q["time_limit_ms"] = it.Limit.Milliseconds()
	return q
}

// Disconnect holds the seat of a player whose connection ended (F105-AC-11): the grace of the mode starts, the clock
// keeps running; a tournament player past its last grace forfeits at once (F105-AC-21).
func (r *Room) Disconnect(student uuid.UUID, now time.Time) Step {
	var s Step
	p := r.player(student)
	if p == nil || p.Out != "" || !p.Connected || r.Done() || r.St == StateFinalizing {
		return s
	}
	p.Connected = false
	if !p.Since.IsZero() {
		p.ConnectedMS += now.Sub(p.Since).Milliseconds()
	}
	s.Memberships = append(s.Memberships, Membership{Student: student, Session: p.Session, Kind: MemberDisconnect})
	s.Checkpoint = true
	if r.St == StateLobby {
		return s // the grace starts with the countdown
	}
	p.Graces++
	p.GraceUntil = now.Add(r.R.Grace)
	s.push("battle.participant.status_changed", map[string]any{"match_id": r.ID.String(), "user_id": student.String(),
		"connected": false, "grace_until": ms(p.GraceUntil)}, r.ids(student)...)
	if r.R.MaxGraces > 0 && p.Graces > r.R.MaxGraces {
		s.merge(r.leave(p, now))
	}
	return s
}

// Submit takes an answer at the server time now (F105-AC-03): only from a player, for the open question of the plan,
// once, until deadline + answer grace. Answers to a question not shown yet, already closed or outside the plan are
// refused and signalled.
func (r *Room) Submit(student uuid.UUID, instance string, response json.RawMessage, now time.Time) (Step, Pending, error) {
	var s Step
	p := r.player(student)
	if p == nil {
		return s, Pending{}, ErrNotMember
	}
	if p.Out != "" {
		return s, Pending{}, ErrOut
	}
	idx := -1
	for _, it := range r.Items {
		if it.Instance == instance {
			idx = it.Index
		}
	}
	if !r.running() {
		return s, Pending{}, ErrNotActive
	}
	switch {
	case idx < 0 || idx > r.Cur:
		s.Signals = append(s.Signals, Signal{Student: student, Type: SignalBeforeShown, Index: r.Cur,
			Evidence: map[string]any{"current_index": r.Cur, "known_instance": idx >= 0}})
		return s, Pending{}, ErrNotOpen
	case idx < r.Cur || r.St != StateQuestion:
		if idx < r.Cur {
			s.Signals = append(s.Signals, Signal{Student: student, Type: SignalClosedQuestion, Index: idx,
				Evidence: map[string]any{"current_index": r.Cur, "answer_index": idx}})
		}
		return s, Pending{}, ErrLate
	}
	if now.After(r.Deadline.Add(r.R.AnswerGrace)) {
		return s, Pending{}, ErrLate
	}
	if _, done := r.Answers[idx][student]; done {
		return s, Pending{}, ErrAlreadySubmitted
	}
	if !ValidResponse(r.Items[idx].Type, response) {
		return s, Pending{}, ErrInvalidResponse
	}
	if r.Answers[idx] == nil {
		r.Answers[idx] = map[uuid.UUID]*Answer{}
	}
	r.Answers[idx][student] = &Answer{Student: student, At: now, Response: response, State: answerPending,
		Support: r.R.Rotation && r.MainTurn(p.Team) != student}
	if !r.R.Collective { // co-op: the team sees the progress at the reveal
		s.push("battle.answer.recorded", map[string]any{"match_id": r.ID.String(), "question_instance_id": instance,
			"user_id": student.String(), "answered_count": len(r.Answers[idx])})
	}
	return s, Pending{Index: idx, Student: student, ReceivedAt: now, Response: response}, nil
}

// Scored applies the grade of module learning to a pending answer.
func (r *Room) Scored(index int, student uuid.UUID, correct bool) Step {
	if a := r.Answers[index][student]; a != nil && a.State == answerPending && index == r.Cur {
		a.State, a.Correct = answerScored, correct
	}
	return Step{}
}

// ScoreFailed marks a pending answer whose grading failed: the question is played again when it closes.
func (r *Room) ScoreFailed(index int, student uuid.UUID) Step {
	if a := r.Answers[index][student]; a != nil && a.State == answerPending && index == r.Cur {
		a.State = answerFailed
	}
	return Step{}
}

// MainTurn is the player of team answering the main turn of the current question (team battle rotation, F105-AC-17):
// the team members in order, skipping those who left.
func (r *Room) MainTurn(team int) uuid.UUID {
	var members []*Player
	for _, p := range r.Players {
		if p.Team == team {
			members = append(members, p)
		}
	}
	if len(members) == 0 {
		return uuid.Nil
	}
	i := max(r.Cur, 0)
	for k := 0; k < len(members); k++ {
		m := members[(i+k)%len(members)]
		if m.Out == "" {
			return m.ID
		}
	}
	return members[i%len(members)].ID
}

// Tick advances the room to now.
func (r *Room) Tick(now time.Time) Step {
	var s Step
	switch r.St {
	case StateLobby:
		s.merge(r.tickLobby(now))
	case StateCountdown:
		s.merge(r.expire(now))
		if r.St == StateCountdown && !now.Before(r.NextAt) {
			s.Started = !r.Started
			if !r.Started {
				r.Started, r.StartedAt = true, now
			}
			s.merge(r.show(0, now))
		}
	case StateQuestion:
		s.merge(r.expire(now))
		if r.St == StateQuestion {
			s.merge(r.tickQuestion(now))
		}
	case StateClosing:
		s.merge(r.expire(now))
		if r.St == StateClosing && !now.Before(r.NextAt) {
			s.merge(r.Revealed(r.Cur, Reveal{}, now)) // module learning did not answer in time: reveal without the answer
		}
	case StateReveal:
		s.merge(r.expire(now))
		if r.St == StateReveal && !now.Before(r.NextAt) {
			if r.Cur+1 < len(r.Items) {
				s.merge(r.show(r.Cur+1, now))
			} else {
				s.merge(r.finalize(EndFinished, now))
			}
		}
	case StateFinalizing:
		if !now.Before(r.RetryAt) { // the result is not recorded yet: ask again
			r.RetryAt = now.Add(2 * time.Second)
			s.Finish = true
		}
	}
	return s
}

func (r *Room) tickLobby(now time.Time) Step {
	var s Step
	joined, all := 0, true
	for _, p := range r.Players {
		if p.Joined {
			joined++
		}
		if !p.Connected {
			all = false
		}
	}
	if !all && now.Before(r.NextAt) {
		return s
	}
	if joined == 0 {
		return r.finalize(EndAllLeft, now)
	}
	// Players who never came get the grace of the mode from now (no-show).
	for _, p := range r.Players {
		if !p.Connected {
			p.Graces++
			p.GraceUntil = now.Add(r.R.Grace)
		}
	}
	r.transition(StateCountdown)
	r.NextAt = now.Add(r.R.Countdown)
	s.Checkpoint = true
	if r.R.Collective {
		return s
	}
	s.push("battle.countdown.started", map[string]any{"match_id": r.ID.String(), "starts_at": ms(r.NextAt),
		"question_count": len(r.Items), "rounds": 1, "speed_bonus_enabled": r.R.SpeedBonus})
	return s
}

// show opens question i for everyone at the same server time (F105-AC-01).
func (r *Room) show(i int, now time.Time) Step {
	var s Step
	r.transition(StateQuestion)
	r.Cur, r.Shown, r.Deadline = i, now, now.Add(r.Items[i].Limit)
	r.Answers[i] = map[uuid.UUID]*Answer{}
	if r.Bot != nil {
		b := BotAnswer(r.Bot.Profile, r.ID, i)
		r.Bot.Answered, r.Bot.Due, r.Bot.Right, r.Bot.Points = false, now.Add(b.Delay), b.Correct, 0
	}
	s.Checkpoint = true
	q := r.render(i)
	if r.R.Collective {
		for _, p := range r.Players {
			if p.Out == "" {
				s.push("coop.task.assigned", map[string]any{"room_id": r.ID.String(), "task_id": r.Items[i].Instance,
					"assignee_id": p.ID.String(), "question": q}, p.ID)
			}
		}
		return s
	}
	shown := map[string]any{"match_id": r.ID.String(), "round": 1, "question_index": i, "question": q,
		"shown_at": ms(r.Shown), "deadline_at": ms(r.Deadline)}
	if primary := r.primaryTurns(); primary != nil {
		shown["primary_user_ids"] = primary
	}
	s.push("battle.question.shown", shown)
	return s
}

func (r *Room) tickQuestion(now time.Time) Step {
	var s Step
	if r.Bot != nil && !r.Bot.Answered && !now.Before(r.Bot.Due) && !r.Bot.Due.After(r.Deadline) {
		r.Bot.Answered = true
		s.push("battle.answer.recorded", map[string]any{"match_id": r.ID.String(), "question_instance_id": r.Items[r.Cur].Instance,
			"user_id": r.Bot.ID.String(), "answered_count": len(r.Answers[r.Cur]) + 1})
	}
	answers := r.Answers[r.Cur]
	everyone := r.Bot == nil || r.Bot.Answered || r.Bot.Due.After(r.Deadline)
	for _, p := range r.Players {
		if p.Out != "" {
			continue
		}
		if _, ok := answers[p.ID]; !ok {
			everyone = false
		}
	}
	pending, failed := false, false
	for _, a := range answers {
		switch a.State {
		case answerPending:
			pending = true
		case answerFailed:
			failed = true
		}
	}
	closeAt := r.Deadline.Add(r.R.AnswerGrace)
	if !everyone && now.Before(closeAt) {
		return s
	}
	if pending && now.Before(closeAt.Add(r.R.ScoreWait)) {
		return s // module learning is still grading
	}
	if pending || failed {
		r.Replays++
		if r.Replays > r.R.MaxReplays {
			s.merge(r.Abort(EndServerFailure, now))
			return s
		}
		s.merge(r.show(r.Cur, now)) // the question is played again
		return s
	}
	s.merge(r.closeQuestion(now))
	return s
}

// closeQuestion computes the points of the closed question and asks the app for the reveal data (F105-AC-05).
func (r *Room) closeQuestion(now time.Time) Step {
	var s Step
	r.transition(StateClosing)
	r.Closed, r.NextAt = now, now.Add(r.R.RevealWait)
	s.Signals = r.score(r.Cur)
	idx := r.Cur
	s.Reveal = &idx
	s.Checkpoint = true
	return s
}

// Revealed publishes the reveal of question index at once to everyone: each player its own evaluation with the correct
// answer, everyone the scoreboard (timed quiz: each student only its own line; co-op: the shared progress).
func (r *Room) Revealed(index int, rv Reveal, now time.Time) Step {
	var s Step
	if r.St != StateClosing || index != r.Cur {
		return s
	}
	r.transition(StateReveal)
	r.NextAt = now.Add(r.R.RevealFor)
	s.Checkpoint = true
	inst := r.Items[index].Instance
	res := r.Results[len(r.Results)-1]
	right, answered := 0, 0
	for _, a := range res.Answers {
		if a.Answered {
			answered++
		}
		if a.Correct {
			right++
		}
	}
	rate := 0.0
	if answered > 0 {
		rate = float64(right) / float64(answered)
	}
	var correct any
	if len(rv.CorrectResponse) > 0 {
		var v any
		if json.Unmarshal(rv.CorrectResponse, &v) == nil {
			correct = v
		}
	}
	board := func(only uuid.UUID) []map[string]any {
		out := []map[string]any{}
		for _, a := range res.Answers {
			if only != uuid.Nil && a.Student != only {
				continue
			}
			p := r.player(a.Student)
			out = append(out, map[string]any{"user_id": a.Student.String(), "team": p.Team, "is_correct": a.Correct, "score": p.Score})
		}
		if r.Bot != nil && only == uuid.Nil {
			out = append(out, map[string]any{"user_id": r.Bot.ID.String(), "team": 1, "is_correct": res.BotRight != nil && *res.BotRight,
				"score": r.Bot.Score})
		}
		return out
	}
	for _, a := range res.Answers {
		if r.R.Collective {
			if !a.Answered {
				continue
			}
			ev := map[string]any{"room_id": r.ID.String(), "task_id": inst, "is_correct": a.Correct, "attempts_left": 0}
			if rv.Explanation != "" {
				ev["explanation"] = map[string]any{"text": rv.Explanation}
			}
			s.push("coop.answer.evaluated", ev, a.Student)
			continue
		}
		ev := map[string]any{"match_id": r.ID.String(), "question_instance_id": inst, "answered": a.Answered, "is_correct": a.Correct,
			"points": a.Points, "time_bonus": a.Bonus}
		if correct != nil {
			ev["correct_response"] = correct
		}
		if rv.Explanation != "" {
			ev["explanation"] = map[string]any{"text": rv.Explanation}
		}
		s.push("battle.answer.evaluated", ev, a.Student)
	}
	next := ms(r.NextAt)
	switch {
	case r.R.Collective:
		contrib := map[string]any{}
		for _, p := range r.Players {
			contrib[p.ID.String()] = p.Correct
		}
		s.push("coop.progress.updated", map[string]any{"room_id": r.ID.String(), "shared_progress": r.progress(), "step_id": inst,
			"contributions": contrib})
	case r.R.Mode == ModeTimedQuiz:
		for _, a := range res.Answers {
			s.push("battle.question.revealed", map[string]any{"match_id": r.ID.String(), "question_instance_id": inst,
				"correct_rate": rate, "scoreboard": board(a.Student), "next_at": next}, a.Student)
		}
	default:
		s.push("battle.question.revealed", map[string]any{"match_id": r.ID.String(), "question_instance_id": inst,
			"correct_rate": rate, "scoreboard": board(uuid.Nil), "next_at": next})
	}
	return s
}

// finalize ends the match (finished, abandon, forfeit, all_left): the app records the result once, then calls Ended.
func (r *Room) finalize(reason string, now time.Time) Step {
	var s Step
	if r.St == StateFinalizing || r.Done() {
		return s
	}
	r.EndReason = reason
	r.Cancelled = reason == EndAllLeft
	r.EndedAt = now
	for _, p := range r.Players {
		if p.Connected && !p.Since.IsZero() {
			p.ConnectedMS += now.Sub(p.Since).Milliseconds()
			p.Since = now
		}
	}
	r.transition(StateFinalizing)
	r.RetryAt = now.Add(2 * time.Second)
	s.Finish, s.Checkpoint = true, true
	return s
}

// Abort stops the match without a result (server failure, plan failure, no checkpoint — F105-AC-15): no rating, no
// abandon; every player is told.
func (r *Room) Abort(reason string, now time.Time) Step {
	var s Step
	if r.Done() {
		return s
	}
	r.EndReason, r.EndedAt = reason, now
	r.transition(StateAborted)
	s.Abort, s.Checkpoint = reason, true
	for _, p := range r.Players {
		if p.Joined && p.Out == "" {
			s.Memberships = append(s.Memberships, Membership{Student: p.ID, Session: p.Session, Kind: MemberLeave, Reason: LeaveClosed})
		}
	}
	if r.R.Collective {
		s.push("coop.session.ended", map[string]any{"room_id": r.ID.String(), "outcome": "aborted", "quest_step_completed": false,
			"rewards_pending": false})
		return s
	}
	results := []map[string]any{}
	for _, p := range r.Players {
		results = append(results, map[string]any{"user_id": p.ID.String(), "team": p.Team, "outcome": OutcomeNoContest, "score": p.Score,
			"correct_count": p.Correct})
	}
	s.push("battle.match.ended", map[string]any{"match_id": r.ID.String(), "status": "aborted", "results": results, "rated": false,
		"end_reason": reason, "review_available": false})
	return s
}

// Ended closes the room once the result is durable (F105-AC-07: battle.match.ended only after the commit).
func (r *Room) Ended(now time.Time) Step { return r.EndedWith(now, nil) }

// EndedWith is Ended with the private encouragements of the players who ended a loss streak (COMPETITION_SYSTEM §12):
// each of them gets its own battle.match.ended with `encouragement`, the others the message of the room.
func (r *Room) EndedWith(now time.Time, enc map[uuid.UUID]Encouragement) Step {
	var s Step
	if r.St != StateFinalizing {
		return s
	}
	r.transition(StateEnded)
	s.Checkpoint = true
	for _, p := range r.Players {
		if p.Joined && p.Out == "" {
			s.Memberships = append(s.Memberships, Membership{Student: p.ID, Session: p.Session, Kind: MemberLeave, Reason: LeaveClosed})
		}
	}
	res := r.Result()
	if r.R.Collective {
		outcome := "failed"
		switch {
		case r.Cancelled:
			outcome = "abandoned"
		case res.GroupGoal != nil && res.GroupGoal.Reached:
			outcome = "completed"
		}
		s.push("coop.session.ended", map[string]any{"room_id": r.ID.String(), "outcome": outcome, "quest_step_completed": outcome == "completed",
			"rewards_pending": false})
		return s
	}
	results := []map[string]any{}
	for _, p := range res.Players {
		results = append(results, map[string]any{"user_id": p.ID.String(), "team": p.Team, "outcome": p.Outcome, "score": p.Score,
			"correct_count": p.Correct})
	}
	status := res.Status
	base := func() map[string]any {
		return map[string]any{"match_id": r.ID.String(), "status": status, "end_reason": res.EndReason, "rated": res.Rated,
			"review_available": status == "completed"}
	}
	if r.R.Mode != ModeTimedQuiz {
		m := base()
		if r.Bot != nil {
			results = append(results, map[string]any{"user_id": r.Bot.ID.String(), "team": 1, "outcome": botOutcome(res), "score": r.Bot.Score,
				"correct_count": r.Bot.Correct})
		}
		m["results"] = results
		if len(enc) == 0 {
			s.push("battle.match.ended", m)
			return s
		}
		var others []uuid.UUID
		for _, p := range r.Players {
			e, ok := enc[p.ID]
			if !ok {
				others = append(others, p.ID)
				continue
			}
			own := maps.Clone(m)
			own["encouragement"] = e.Payload()
			s.push("battle.match.ended", own, p.ID)
		}
		if len(others) > 0 {
			s.push("battle.match.ended", m, others...)
		}
		return s
	}
	// Timed quiz: one message per student — from grade 4 the top and its own rank, grades 1–3 their own result and the grade goal only (ADR-0095).
	ranks := rank(res.Players)
	top := []map[string]any{}
	for _, p := range res.Players {
		if ranks[p.ID] <= r.R.Top {
			top = append(top, map[string]any{"user_id": p.ID.String(), "rank": ranks[p.ID], "score": p.Score})
		}
	}
	slices.SortFunc(top, func(a, b map[string]any) int { return a["rank"].(int) - b["rank"].(int) })
	if len(top) > r.R.Top {
		top = top[:r.R.Top]
	}
	for _, p := range res.Players {
		m := base()
		m["results"] = []map[string]any{{"user_id": p.ID.String(), "team": p.Team, "outcome": p.Outcome, "score": p.Score,
			"correct_count": p.Correct}}
		if r.R.Ranking {
			m["top"], m["my_rank"] = top, ranks[p.ID]
		} else if res.GroupGoal != nil {
			// grade_goal (ADR-0095): the collective progress of the challenge; the number of contributors is hidden
			// below 5 students (no small cells).
			goal := map[string]any{"target": res.GroupGoal.Target, "current": res.GroupGoal.Achieved, "reached": res.GroupGoal.Reached}
			if len(res.Players) >= 5 {
				goal["contributors_count"] = len(res.Players)
			}
			m["grade_goal"] = goal
		}
		if e, ok := enc[p.ID]; ok {
			m["encouragement"] = e.Payload()
		}
		s.push("battle.match.ended", m, p.ID)
	}
	return s
}

// Resume restarts a room restored from its checkpoint on a new host (failover, F105-AC-15): the question in progress is
// played again with a new clock; a room in the lobby or the countdown restarts its countdown.
func (r *Room) Resume(now time.Time) Step {
	var s Step
	for _, p := range r.Players { // connections are re-established by the clients (battle.state.request)
		if p.Connected {
			p.Since = now
		}
	}
	switch r.St {
	case StateLobby, StateCountdown:
		r.transition(StateCountdown)
		r.NextAt = now.Add(r.R.Countdown)
		s.push("battle.match.resumed", map[string]any{"match_id": r.ID.String(), "replay_current_question": false})
		s.Checkpoint = true
	case StateQuestion, StateClosing:
		s.push("battle.match.resumed", map[string]any{"match_id": r.ID.String(), "replay_current_question": true})
		if r.St == StateClosing && len(r.Results) > 0 && r.Results[len(r.Results)-1].Index == r.Cur {
			r.unscore(r.Cur)
		}
		s.merge(r.show(r.Cur, now))
	case StateReveal:
		s.push("battle.match.resumed", map[string]any{"match_id": r.ID.String(), "replay_current_question": false})
		r.NextAt = now.Add(r.R.RevealFor)
		s.Checkpoint = true
	case StateFinalizing:
		r.RetryAt = now
	}
	return s
}

// leave takes a player out past its grace (abandoned, tournament: forfeit) and ends the match when the mode says so.
func (r *Room) leave(p *Player, now time.Time) Step {
	var s Step
	p.Out = r.R.OutOutcome
	p.GraceUntil = time.Time{}
	s.Memberships = append(s.Memberships, Membership{Student: p.ID, Session: p.Session, Kind: MemberLeave, Reason: LeaveTimeout})
	s.Checkpoint = true
	active := map[int]int{}  // team → players still in
	present := map[int]int{} // team → players connected
	for _, q := range r.Players {
		if q.Out == "" {
			active[q.Team]++
			if q.Connected {
				present[q.Team]++
			}
		}
	}
	teams := map[int]bool{}
	for _, q := range r.Players {
		teams[q.Team] = true
	}
	switch {
	case len(active) == 0:
		s.merge(r.finalize(EndAllLeft, now))
	case r.IsBot:
		s.merge(r.finalize(EndAllLeft, now)) // the student left the practice bot: nothing to record against anyone
	case r.R.EndOnAbandon:
		other := 1 - p.Team
		if present[other] == 0 {
			s.merge(r.finalize(EndAllLeft, now)) // both sides gone: cancelled, not rated (F105-AC-12)
			break
		}
		reason := EndAbandon
		if p.Out == OutcomeForfeit {
			reason = EndForfeit
		}
		s.merge(r.finalize(reason, now))
	case len(teams) > 1 && active[p.Team] == 0:
		s.merge(r.finalize(EndAbandon, now)) // a whole team left: the other team wins
	}
	return s
}

// expire takes out the players whose grace ran out (not in a timed quiz: the seat is held to the end).
func (r *Room) expire(now time.Time) Step {
	var s Step
	if r.R.HoldSeats {
		return s
	}
	for _, p := range r.Players {
		if r.St == StateFinalizing || r.Done() {
			break
		}
		if p.Out == "" && !p.Connected && !p.GraceUntil.IsZero() && !now.Before(p.GraceUntil) {
			s.merge(r.leave(p, now))
		}
	}
	return s
}

func (r *Room) progress() float64 {
	if r.R.CoopTarget <= 0 {
		return 0
	}
	return min(1, float64(r.CoopFires)/float64(r.R.CoopTarget))
}

func (r *Room) coopState() map[string]any {
	st := map[string]any{"state": r.St, "question_index": r.Cur, "question_count": len(r.Items), "target": r.R.CoopTarget,
		"achieved": r.CoopFires}
	if r.St == StateQuestion {
		st["current_question"] = r.render(r.Cur)
		st["deadline_at"] = ms(r.Deadline)
	}
	return st
}

// MatchID is the id of the match (= room id).
func (r *Room) MatchID() uuid.UUID { return r.ID }
