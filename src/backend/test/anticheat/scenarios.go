package anticheat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
)

// Scenario is one row of ANTI_CHEAT §11.
type Scenario struct {
	ID    string // AC-CMP-2xx of ANTI_CHEAT §11
	AC    string // F132 acceptance criterion
	TC    string // TEST_STRATEGY test case(s), "" when none
	Title string
	Run   func(ctx context.Context, e *Env) error
}

// Scenarios are the 10 scenarios of ANTI_CHEAT §11 (SC-005: 10/10 on staging before a release).
var Scenarios = []Scenario{
	{"AC-CMP-201", "F132-AC-22", "TC-LRN-008", "Submit twice with the same Idempotency-Key → one result, same response", IdempotentSubmit},
	{"AC-CMP-202", "F132-AC-22", "TC-LRN-008", "Submit again with another key → 409 ATTEMPT_ALREADY_SUBMITTED", SecondSubmitConflict},
	{"AC-CMP-203", "F132-AC-21", "TC-LRN-007 · TC-CMP-007", "Client sends score/is_correct/xp/rating → ignored or 422, score computed by the server", ClientScoreIgnored},
	{"AC-CMP-204", "F132-AC-23", "TC-EXM-006", "Answer after the exam deadline + grace → refused", AnswerAfterDeadline},
	{"AC-CMP-205", "F132-AC-23", "", "Battle answer for a question never shown → ignored", BattleAnswerNotShown},
	{"AC-CMP-206", "F132-AC-22", "TC-CMP-009", "One finished match updates the rating exactly once", MatchCountedOnce},
	{"AC-CMP-207", "F132-AC-23", "", "Disconnect and resume within 30 s → the match goes on", BattleResume},
	{"AC-CMP-208", "F132-AC-24", "", "Speed hack in the hub → snap back + eduworld_rt_speed_violations_total; learning record unchanged", SpeedHack},
	{"AC-CMP-209", "F132-AC-22", "", "Same content completed 5 times → XP follows the decay (100/50/25/0/0 %)", XPDecay},
	{"AC-CMP-210", "F132-AC-23", "", "Replayed WebSocket command id → first reply returned, processed once", WSReplay},
}

// ---------- learning (REST) ----------

type renderedItem struct {
	Position          int    `json:"position"`
	QuestionVersionID string `json:"question_version_id"`
	RenderPayload     struct {
		Options []struct {
			OptionID string `json:"option_id"`
			ID       string `json:"id"`
		} `json:"options"`
	} `json:"render_payload"`
}

type attemptBody struct {
	AttemptID  string         `json:"attempt_id"`
	Status     string         `json:"status"`
	DeadlineAt *time.Time     `json:"deadline_at"`
	ServerTime *time.Time     `json:"server_time"`
	Items      []renderedItem `json:"items"`
}

func (it renderedItem) firstOption() string {
	if len(it.RenderPayload.Options) == 0 {
		return ""
	}
	if o := it.RenderPayload.Options[0].OptionID; o != "" {
		return o
	}
	return it.RenderPayload.Options[0].ID
}

func startPractice(ctx context.Context, p *Player, activity string) (attemptBody, error) {
	if activity == "" {
		return attemptBody{}, fmt.Errorf("%w: practice_activity_id missing", ErrSkip)
	}
	r, err := p.Do(ctx, http.MethodPost, "/learning/activities/"+activity+"/attempts", map[string]any{"mode": "practice"}, "")
	if err != nil {
		return attemptBody{}, err
	}
	if r.Status != http.StatusCreated && r.Status != http.StatusOK {
		return attemptBody{}, fmt.Errorf("startActivityAttempt: status %d %s", r.Status, r.Code())
	}
	var a attemptBody
	if err := json.Unmarshal(r.Body, &a); err != nil || a.AttemptID == "" {
		return a, fmt.Errorf("startActivityAttempt: unexpected body: %w", err)
	}
	return a, nil
}

// answerAll answers every choice item with its first option; extra fields (a tampered client) are merged into each
// body. It returns how many answers were refused with 422.
func answerAll(ctx context.Context, p *Player, a attemptBody, extra map[string]any) (refused int, err error) {
	for _, it := range a.Items {
		opt := it.firstOption()
		if opt == "" {
			continue
		}
		body := map[string]any{"kind": "question", "position": it.Position, "question_version_id": it.QuestionVersionID,
			"response": map[string]any{"option_id": opt}}
		for k, v := range extra {
			body[k] = v
		}
		r, err := p.Do(ctx, http.MethodPost, "/learning/activity-attempts/"+a.AttemptID+"/responses", body, "")
		if err != nil {
			return refused, err
		}
		switch {
		case r.Status == http.StatusUnprocessableEntity && extra != nil:
			refused++
		case r.Status/100 != 2:
			return refused, fmt.Errorf("submitActivityResponse: status %d %s", r.Status, r.Code())
		}
	}
	return refused, nil
}

func submitAttempt(ctx context.Context, p *Player, attempt, key string, body any) (Response, error) {
	return p.Do(ctx, http.MethodPost, "/learning/activity-attempts/"+attempt+":submit", body, key)
}

func reviewScore(ctx context.Context, p *Player, attempt string) (score, max *float64, err error) {
	r, err := p.Do(ctx, http.MethodGet, "/learning/activity-attempts/"+attempt+"/review", nil, "")
	if err != nil {
		return nil, nil, err
	}
	if r.Status != http.StatusOK {
		return nil, nil, fmt.Errorf("getActivityAttemptReview: status %d %s (the fixture activity must release results immediately)", r.Status, r.Code())
	}
	var v struct {
		Score    *float64 `json:"score"`
		MaxScore *float64 `json:"max_score"`
	}
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return nil, nil, err
	}
	return v.Score, v.MaxScore, nil
}

func submittedAttempt(ctx context.Context, e *Env) (*Player, attemptBody, string, Response, error) {
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return nil, attemptBody{}, "", Response{}, err
	}
	a, err := startPractice(ctx, p, e.Cfg.Fixtures.PracticeActivityID)
	if err != nil {
		return nil, a, "", Response{}, err
	}
	if _, err := answerAll(ctx, p, a, nil); err != nil {
		return nil, a, "", Response{}, err
	}
	key := ids.V7(time.Now())
	r, err := submitAttempt(ctx, p, a.AttemptID, key, nil)
	if err != nil {
		return nil, a, "", r, err
	}
	if r.Status != http.StatusOK {
		return nil, a, "", r, fmt.Errorf("submitActivityAttempt: status %d %s", r.Status, r.Code())
	}
	return p, a, key, r, nil
}

// IdempotentSubmit is AC-CMP-201 (AGENTS R10, ADR-0010).
func IdempotentSubmit(ctx context.Context, e *Env) error {
	p, a, key, first, err := submittedAttempt(ctx, e)
	if err != nil {
		return err
	}
	again, err := submitAttempt(ctx, p, a.AttemptID, key, nil)
	if err != nil {
		return err
	}
	if again.Status != first.Status {
		return fmt.Errorf("replayed submit: status %d, first %d", again.Status, first.Status)
	}
	if !sameJSON(first.Body, again.Body) {
		return errors.New("replayed submit: response differs from the first one")
	}
	if again.Header.Get("Idempotent-Replayed") != "true" {
		return errors.New("replayed submit: header Idempotent-Replayed: true missing")
	}
	return nil
}

// SecondSubmitConflict is AC-CMP-202.
func SecondSubmitConflict(ctx context.Context, e *Env) error {
	p, a, _, _, err := submittedAttempt(ctx, e)
	if err != nil {
		return err
	}
	r, err := submitAttempt(ctx, p, a.AttemptID, ids.V7(time.Now()), nil)
	if err != nil {
		return err
	}
	if r.Status != http.StatusConflict || r.Code() != "ATTEMPT_ALREADY_SUBMITTED" {
		return fmt.Errorf("second submit with a new key: %d %s, want 409 ATTEMPT_ALREADY_SUBMITTED", r.Status, r.Code())
	}
	return nil
}

// tampered are the server-authoritative fields a modified client adds (ANTI_CHEAT §4).
var tampered = map[string]any{"score": 999, "is_correct": true, "xp": 9999, "rating": 3000}

// ClientScoreIgnored is AC-CMP-203 (ADR-0016, AGENTS R5): the same answers with and without client-sent score fields
// get the same server score; a 422 refusal is the other acceptable answer.
func ClientScoreIgnored(ctx context.Context, e *Env) error {
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return err
	}
	plain, err := startPractice(ctx, p, e.Cfg.Fixtures.PracticeActivityID)
	if err != nil {
		return err
	}
	if _, err := answerAll(ctx, p, plain, nil); err != nil {
		return err
	}
	if r, err := submitAttempt(ctx, p, plain.AttemptID, "", nil); err != nil {
		return fmt.Errorf("plain submit: %w", err)
	} else if r.Status != http.StatusOK {
		return fmt.Errorf("plain submit: status %d %s", r.Status, r.Code())
	}
	bad, err := startPractice(ctx, p, e.Cfg.Fixtures.PracticeActivityID)
	if err != nil {
		return err
	}
	refused, err := answerAll(ctx, p, bad, tampered)
	if err != nil {
		return err
	}
	sub, err := submitAttempt(ctx, p, bad.AttemptID, "", tampered)
	if err != nil {
		return err
	}
	if refused > 0 {
		if refused != countChoice(bad.Items) {
			return fmt.Errorf("tampered answers: %d refused with 422, others accepted", refused)
		}
		return nil // refused by the schema: nothing client-sent was scored
	}
	if sub.Status != http.StatusOK && sub.Status != http.StatusUnprocessableEntity {
		return fmt.Errorf("tampered submit: status %d %s", sub.Status, sub.Code())
	}
	want, max, err := reviewScore(ctx, p, plain.AttemptID)
	if err != nil {
		return err
	}
	got, _, err := reviewScore(ctx, p, bad.AttemptID)
	if err != nil {
		return err
	}
	return compareScores(got, want, max)
}

func compareScores(got, want, max *float64) error {
	if got == nil || want == nil {
		return errors.New("review has no score: the fixture activity must release results immediately")
	}
	if *got != *want {
		return fmt.Errorf("server score with client fields = %v, without = %v: a client value was used", *got, *want)
	}
	if max != nil && *got > *max {
		return fmt.Errorf("score %v above max_score %v", *got, *max)
	}
	return nil
}

func countChoice(items []renderedItem) int {
	n := 0
	for _, it := range items {
		if it.firstOption() != "" {
			n++
		}
	}
	return n
}

// AnswerAfterDeadline is AC-CMP-204 (ANTI_CHEAT §3): the exam answer after deadline_at + grace is refused. The exam
// engine answers 422 EXAM_TIME_OVER (EXAM_ENGINE §6); ATTEMPT_DEADLINE_PASSED is the code of timed activity attempts.
func AnswerAfterDeadline(ctx context.Context, e *Env) error {
	f := e.Cfg.Fixtures
	if f.ExamSessionID == "" {
		return fmt.Errorf("%w: exam_session_id missing", ErrSkip)
	}
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return err
	}
	r, err := p.Do(ctx, http.MethodPost, "/exams/sessions/"+f.ExamSessionID+"/attempts", map[string]any{"take_over": true}, "")
	if err != nil {
		return err
	}
	if r.Status != http.StatusCreated && r.Status != http.StatusOK {
		return fmt.Errorf("%w: startExamAttempt %d %s (the fixture exam must accept a new attempt)", ErrSkip, r.Status, r.Code())
	}
	var a attemptBody
	if err := json.Unmarshal(r.Body, &a); err != nil {
		return fmt.Errorf("startExamAttempt: %w", err)
	}
	if a.AttemptID == "" || a.DeadlineAt == nil || len(a.Items) == 0 {
		return errors.New("startExamAttempt: attempt without attempt_id/deadline_at/items")
	}
	now := time.Now()
	if a.ServerTime != nil {
		now = *a.ServerTime
	}
	wait := a.DeadlineAt.Sub(now) + f.DeadlineGrace + 2*time.Second
	if wait > 30*time.Minute {
		return fmt.Errorf("%w: exam deadline %s away (use a short fixture exam)", ErrSkip, wait.Round(time.Second))
	}
	if wait > 0 {
		e.Sleep(wait)
	}
	it := a.Items[0]
	ans, err := p.Do(ctx, http.MethodPut, fmt.Sprintf("/exams/attempts/%s/answers/%d", a.AttemptID, it.Position),
		map[string]any{"kind": "question", "question_version_id": it.QuestionVersionID, "response": map[string]any{"option_id": it.firstOption()}}, "")
	if err != nil {
		return err
	}
	if ans.Status/100 == 2 {
		return fmt.Errorf("answer after deadline accepted: %d", ans.Status)
	}
	switch ans.Code() {
	case "EXAM_TIME_OVER", "ATTEMPT_DEADLINE_PASSED", "EXAM_ALREADY_SUBMITTED", "ATTEMPT_ALREADY_SUBMITTED":
		return nil // refused; the worker may already have auto-submitted the attempt at the deadline
	}
	return fmt.Errorf("answer after deadline: %d %s, want EXAM_TIME_OVER/ATTEMPT_DEADLINE_PASSED", ans.Status, ans.Code())
}

// ---------- battle (one shared match for AC-CMP-205, 206, 207) ----------

type battleOutcome struct {
	mu             sync.Mutex // fields written by the realtime handlers
	err            error
	unshownAcked   bool   // the answer to a never-shown question was acked (must not)
	unshownCode    string // system.command.failed code of that answer
	resumed        bool   // the match went on after a resume within the grace
	resumeErr      error
	playedBefore   int
	playedAfter    int
	ratingReadable bool
}

func gamesPlayed(ctx context.Context, p *Player, mode, subject string) (int, bool, error) {
	r, err := p.Do(ctx, http.MethodGet, "/competition/ratings/me", nil, "")
	if err != nil {
		return 0, false, err
	}
	if r.Status != http.StatusOK {
		return 0, false, fmt.Errorf("getMyRatings: %d %s", r.Status, r.Code())
	}
	var v struct {
		Data []struct {
			Mode        string `json:"mode"`
			SubjectID   string `json:"subject_id"`
			GamesPlayed int    `json:"games_played"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return 0, false, err
	}
	for _, d := range v.Data {
		if d.Mode == mode && (subject == "" || d.SubjectID == subject) {
			return d.GamesPlayed, true, nil
		}
	}
	return 0, true, nil // no rating yet for this mode/subject
}

func (e *Env) runBattle(ctx context.Context) *battleOutcome {
	if e.battle != nil {
		return e.battle
	}
	o := &battleOutcome{}
	e.battle = o
	f := e.Cfg.Fixtures
	if f.BattleSubject == "" {
		o.err = fmt.Errorf("%w: battle_subject_id missing", ErrSkip)
		return o
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	a, err := e.Player(ctx, "primary")
	if err != nil {
		o.err = err
		return o
	}
	b, err := e.Player(ctx, "opponent")
	if err != nil {
		o.err = err
		return o
	}
	if o.playedBefore, o.ratingReadable, err = gamesPlayed(ctx, a, f.BattleMode, f.BattleSubject); err != nil {
		o.err = err
		return o
	}
	ended := make(chan struct{})
	var once sync.Once
	var probed, dropped sync.Once
	for i, pl := range []*Player{a, b} {
		c, err := pl.Connect(ctx)
		if err != nil {
			o.err = err
			return o
		}
		isPrimary := i == 0
		wire(ctx, c, isPrimary, o, &probed, &dropped, func() { once.Do(func() { close(ended) }) }, a)
		r, err := pl.Do(ctx, http.MethodPost, "/competition/matchmaking/tickets", map[string]any{"mode": f.BattleMode, "subject_id": f.BattleSubject}, "")
		if err != nil {
			o.err = fmt.Errorf("createMatchmakingTicket: %w", err)
			return o
		}
		if r.Status != http.StatusCreated {
			o.err = fmt.Errorf("createMatchmakingTicket: status %d %s", r.Status, r.Code())
			return o
		}
	}
	select {
	case <-ended:
	case <-ctx.Done():
		o.err = fmt.Errorf("battle: match did not end: %w", ctx.Err())
		return o
	}
	// The rating projection is asynchronous (match.completed → ranking): wait until it moves, then once more to see
	// that it does not move twice.
	for i := 0; i < 30; i++ {
		if o.playedAfter, _, err = gamesPlayed(ctx, a, f.BattleMode, f.BattleSubject); err == nil && o.playedAfter > o.playedBefore {
			break
		}
		e.Sleep(2 * time.Second)
	}
	e.Sleep(10 * time.Second)
	o.playedAfter, _, _ = gamesPlayed(ctx, a, f.BattleMode, f.BattleSubject)
	return o
}

type shownQuestion struct {
	MatchID  string `json:"match_id"`
	Question struct {
		InstanceID string `json:"question_instance_id"`
		Options    []struct {
			OptionID string `json:"option_id"`
		} `json:"options"`
	} `json:"question"`
}

// wire installs the battle handlers of one player: accept proposals, answer shown questions with the first option;
// the primary also answers a never-shown question once (AC-CMP-205) and drops/resumes its connection once
// (AC-CMP-207).
func wire(ctx context.Context, c *wsclient.Conn, primary bool, o *battleOutcome, probed, dropped *sync.Once, end func(), pa *Player) {
	c.On("matchmaking.proposal.created", func(env wsclient.Envelope) {
		var p struct {
			ProposalID string `json:"proposal_id"`
		}
		if json.Unmarshal(env.Payload, &p) == nil {
			go func() {
				_, _, _ = c.Request(ctx, "matchmaking.proposal.accept", map[string]any{"proposal_id": p.ProposalID}, 5*time.Second)
			}()
		}
	})
	c.On("battle.question.shown", func(env wsclient.Envelope) {
		var q shownQuestion
		if json.Unmarshal(env.Payload, &q) != nil || len(q.Question.Options) == 0 {
			return
		}
		go func() {
			if primary {
				probed.Do(func() { // a question instance the server never showed (a tampered client guessing ahead)
					_, _, err := c.Request(ctx, "battle.answer.submit", map[string]any{"match_id": q.MatchID,
						"question_instance_id": ids.V4(), "response": map[string]any{"option_id": q.Question.Options[0].OptionID}}, 5*time.Second)
					var fe *wsclient.FailedError
					o.mu.Lock()
					defer o.mu.Unlock()
					switch {
					case err == nil:
						o.unshownAcked = true
					case errors.As(err, &fe):
						o.unshownCode = fe.Code
					}
				})
			}
			time.Sleep(1500 * time.Millisecond) // answer_rate: ≥ 1 s per question, not a too-fast answer
			_, _, _ = c.Request(ctx, "battle.answer.submit", map[string]any{"match_id": q.MatchID, "question_instance_id": q.Question.InstanceID,
				"response": map[string]any{"option_id": q.Question.Options[0].OptionID}}, 5*time.Second)
			if primary {
				dropped.Do(func() {
					ok, err := dropAndResume(ctx, pa, end)
					o.mu.Lock()
					o.resumed, o.resumeErr = ok, err
					o.mu.Unlock()
				})
			}
		}()
	})
	c.On("battle.match.ended", func(wsclient.Envelope) { end() })
}

// dropAndResume closes the primary connection and comes back within the 30 s grace with system.session.resume.
func dropAndResume(ctx context.Context, p *Player, end func()) (bool, error) {
	old := p.WS
	var w struct {
		ResumeToken string `json:"resume_token"`
	}
	_ = json.Unmarshal(old.Welcome().Payload, &w)
	recv, _, _ := old.Stats()
	_ = old.Close()
	time.Sleep(3 * time.Second)
	c, err := p.Connect(ctx)
	if err != nil {
		return false, err
	}
	back := make(chan struct{}, 1)
	signal := func(wsclient.Envelope) {
		select {
		case back <- struct{}{}:
		default:
		}
	}
	for _, t := range []string{"battle.question.shown", "battle.state.snapshot", "battle.round.ended", "battle.match.resumed"} {
		c.On(t, signal)
	}
	c.On("battle.match.ended", func(env wsclient.Envelope) { signal(env); end() })
	if _, _, err := c.Request(ctx, "system.session.resume", map[string]any{"resume_token": w.ResumeToken, "last_seq": recv}, 5*time.Second); err != nil {
		return false, fmt.Errorf("system.session.resume: %w", err)
	}
	select {
	case <-back:
		return true, nil
	case <-time.After(30 * time.Second):
		return false, errors.New("no battle message within 30 s after the resume")
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// BattleAnswerNotShown is AC-CMP-205 (ANTI_CHEAT §3, §8): never acked; the room answers VALIDATION_FAILED and records a
// signal.
func BattleAnswerNotShown(ctx context.Context, e *Env) error {
	o := e.runBattle(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return o.err
	}
	if o.unshownAcked {
		return errors.New("an answer to a question never shown was acked")
	}
	if o.unshownCode == "" {
		return errors.New("the answer to a question never shown got no system.command.failed")
	}
	return nil
}

// MatchCountedOnce is AC-CMP-206 / TC-CMP-009: a finished match moves games_played by exactly one (RecordMatchResult
// is idempotent per match_id; no REST route lets a client post a result — checked by the authz matrix and the spec).
func MatchCountedOnce(ctx context.Context, e *Env) error {
	o := e.runBattle(ctx)
	if o.err != nil {
		return o.err
	}
	if !o.ratingReadable {
		return errors.New("getMyRatings not readable")
	}
	if d := o.playedAfter - o.playedBefore; d != 1 {
		return fmt.Errorf("games_played moved by %d after one match, want exactly 1", d)
	}
	return nil
}

// BattleResume is AC-CMP-207.
func BattleResume(ctx context.Context, e *Env) error {
	o := e.runBattle(ctx)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return o.err
	}
	if !o.resumed {
		return fmt.Errorf("match did not go on after a resume within 30 s: %w", o.resumeErr)
	}
	return nil
}

// ---------- hub ----------

// SpeedHack is AC-CMP-208 / F132-AC-24 (ANTI_CHEAT §2, §3, §10).
func SpeedHack(ctx context.Context, e *Env) error {
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return err
	}
	before, err := p.Do(ctx, http.MethodGet, "/learning/progress/me", nil, "")
	if err := expectOK("getMyProgress before", before, err); err != nil {
		return err
	}
	var metricBefore float64
	if e.Probe != nil {
		if metricBefore, err = e.Probe(ctx, "sum(eduworld_rt_speed_violations_total)"); err != nil {
			return fmt.Errorf("prometheus: %w", err)
		}
	}
	c, err := p.Connect(ctx)
	if err != nil {
		return err
	}
	assigned := make(chan hubInstance, 1)
	c.On("hub.instance.assigned", func(env wsclient.Envelope) {
		var in hubInstance
		if json.Unmarshal(env.Payload, &in) == nil {
			select {
			case assigned <- in:
			default:
			}
		}
	})
	corrected := make(chan string, 4)
	c.On("hub.player.corrected", func(env wsclient.Envelope) {
		var v struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(env.Payload, &v)
		select {
		case corrected <- v.Reason:
		default:
		}
	})
	if _, _, err := c.Request(ctx, "hub.instance.join", map[string]any{"area_id": e.Cfg.Fixtures.HubArea}, 10*time.Second); err != nil {
		return fmt.Errorf("hub.instance.join: %w", err)
	}
	var in hubInstance
	select {
	case in = <-assigned:
	case <-time.After(10 * time.Second):
		return errors.New("no hub.instance.assigned")
	}
	speed := in.MaxSpeed
	if speed <= 0 {
		speed = 5
	}
	// One tick later the avatar claims to be 20 s of max speed away: far above max_speed × 1.2.
	jump := []float64{round2(in.Spawn[0] + 20*speed), in.Spawn[1], in.Spawn[2]}
	e.Sleep(150 * time.Millisecond)
	if _, err := c.Send(ctx, "hub.player.move", map[string]any{"room_id": in.RoomID, "client_seq": 1, "p": jump, "r": 0, "a": "walk"}); err != nil {
		return err
	}
	select {
	case reason := <-corrected:
		if reason != "speed" {
			return fmt.Errorf("hub.player.corrected reason %q, want speed", reason)
		}
	case <-time.After(5 * time.Second):
		return errors.New("speed hack not corrected (no hub.player.corrected)")
	}
	if e.Probe != nil {
		ok := false
		for i := 0; i < 12 && !ok; i++ { // scrape interval
			v, err := e.Probe(ctx, "sum(eduworld_rt_speed_violations_total)")
			ok = err == nil && v > metricBefore
			if !ok {
				e.Sleep(5 * time.Second)
			}
		}
		if !ok {
			return errors.New("eduworld_rt_speed_violations_total did not increase")
		}
	}
	after, err := p.Do(ctx, http.MethodGet, "/learning/progress/me", nil, "")
	if err := expectOK("getMyProgress after", after, err); err != nil {
		return err
	}
	if !sameJSON(before.Body, after.Body) {
		return errors.New("the learning record changed after a cheat signal (ANTI_CHEAT §9: never automatic)")
	}
	return nil
}

type hubInstance struct {
	RoomID   string     `json:"room_id"`
	Spawn    [3]float64 `json:"spawn"`
	MaxSpeed float64    `json:"max_speed_mps"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ---------- XP ----------

type xpEntry struct {
	Amount     int       `json:"amount"`
	SourceType string    `json:"source_type"`
	CreatedAt  time.Time `json:"created_at"`
}

// xpEntries reads the latest XP ledger entries of the player (listXpHistory, newest first).
func xpEntries(ctx context.Context, p *Player) ([]xpEntry, error) {
	r, err := p.Do(ctx, http.MethodGet, "/game/progression/xp-history?limit=50", nil, "")
	if err != nil {
		return nil, err
	}
	if r.Status != http.StatusOK {
		return nil, fmt.Errorf("listXpHistory: %d %s", r.Status, r.Code())
	}
	var v struct {
		Data []xpEntry `json:"data"`
	}
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return nil, err
	}
	return v.Data, nil
}

// newXP sums the entries of cur absent from prev (no clock comparison: server and runner clocks may differ).
func newXP(prev, cur []xpEntry) int {
	seen := map[string]int{}
	for _, x := range prev {
		seen[x.key()]++
	}
	sum := 0
	for _, x := range cur {
		if seen[x.key()] > 0 {
			seen[x.key()]--
			continue
		}
		sum += x.Amount
	}
	return sum
}

func (x xpEntry) key() string {
	return x.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + fmt.Sprint(x.Amount) + "|" + x.SourceType
}

// XPDecay is AC-CMP-209 (PROGRESSION_SYSTEM §3.2 owns the decay; ANTI_CHEAT §7): five completions of the same content
// earn decay[i] % of the first grant. XP comes from domain events (asynchronous), so each grant is read after a settle.
func XPDecay(ctx context.Context, e *Env) error {
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return err
	}
	decay := e.Cfg.Fixtures.XPDecay
	got := make([]int, 0, len(decay))
	for range decay {
		prev, err := xpEntries(ctx, p)
		if err != nil {
			return err
		}
		a, err := startPractice(ctx, p, e.Cfg.Fixtures.PracticeActivityID)
		if err != nil {
			return err
		}
		if _, err := answerAll(ctx, p, a, nil); err != nil {
			return err
		}
		if r, err := submitAttempt(ctx, p, a.AttemptID, "", nil); expectOK("submit", r, err) != nil {
			return expectOK("submit", r, err)
		}
		e.Sleep(8 * time.Second)
		cur, err := xpEntries(ctx, p)
		if err != nil {
			return err
		}
		got = append(got, newXP(prev, cur))
	}
	return checkDecay(got, decay)
}

// checkDecay compares the XP of each completion with decay (percent of the first), ±1 XP for rounding.
func checkDecay(got, decay []int) error {
	if len(got) == 0 || got[0] <= 0 {
		return fmt.Errorf("%w: first completion earned no XP (content already completed today, or capped: use a fresh fixture activity/day)", ErrSkip)
	}
	for i := range got {
		want := float64(got[0]) * float64(decay[i]) / float64(decay[0])
		if math.Abs(float64(got[i])-want) > 1 {
			return fmt.Errorf("XP of completions = %v, want %v %% of the first", got, decay)
		}
	}
	return nil
}

// ---------- realtime replay ----------

// WSReplay is AC-CMP-210 (ANTI_CHEAT §4): the same envelope id sent twice — the second time with another payload —
// gets the first reply back; the command ran once.
func WSReplay(ctx context.Context, e *Env) error {
	ch := e.Cfg.Fixtures.ChatChannelID
	if ch == "" {
		return fmt.Errorf("%w: chat_channel_id missing", ErrSkip)
	}
	p, err := e.Player(ctx, "primary")
	if err != nil {
		return err
	}
	c, err := p.Connect(ctx)
	if err != nil {
		return err
	}
	id := ids.V7(time.Now())
	cmA, cmB := ids.V4(), ids.V4()
	first, _, err := c.RequestID(ctx, id, "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": cmA, "kind": "text", "body": "kiểm tra 1"}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("first send: %w", err)
	}
	again, _, err := c.RequestID(ctx, id, "chat.message.send", map[string]any{"channel_id": ch, "client_message_id": cmB, "kind": "text", "body": "kiểm tra 2"}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("replayed send: %w", err)
	}
	return checkReplay(first.Payload, again.Payload, cmA)
}

func checkReplay(first, again json.RawMessage, firstClientID string) error {
	type ack struct {
		Result struct {
			MessageID       string `json:"message_id"`
			ClientMessageID string `json:"client_message_id"`
		} `json:"result"`
	}
	var a, b ack
	_ = json.Unmarshal(first, &a)
	_ = json.Unmarshal(again, &b)
	if a.Result.MessageID == "" {
		return errors.New("first ack has no message_id")
	}
	if b.Result.MessageID != a.Result.MessageID || b.Result.ClientMessageID != firstClientID {
		return errors.New("replayed id was processed again (different message)")
	}
	return nil
}

// expectOK turns a transport error or a non-200 answer of op into an error (nil when 200).
func expectOK(op string, r Response, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if r.Status != http.StatusOK {
		return fmt.Errorf("%s: status %d %s", op, r.Status, r.Code())
	}
	return nil
}

// sameJSON compares two JSON documents semantically.
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}
