// Package battle is the battle behaviour of a bot (TEST_STRATEGY §8.1: one match every two sessions, 5–10
// questions; TC-PERF-004 matchmaking): createMatchmakingTicket over REST, accept the proposal, answer every
// battle.question.shown after a think time with a raw response (the server scores, ADR-0016: the bot never sends a
// score), until battle.match.ended.
package battle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// Options of one match.
type Options struct {
	Mode      string // e.g. ranked_1v1 (createMatchmakingTicket.mode)
	SubjectID string
	Think     func() time.Duration // time before answering a question (≥ 1 s, answer_rate)
	Timeout   time.Duration        // whole match including the queue
}

type question struct {
	MatchID  string `json:"match_id"`
	Question struct {
		InstanceID string `json:"question_instance_id"`
		Options    []struct {
			OptionID string `json:"option_id"`
		} `json:"options"`
	} `json:"question"`
}

// Play runs one match; it returns the number of questions answered.
func Play(ctx context.Context, rc *restcall.Client, c *wsclient.Conn, m *report.Metrics, o Options) (int, error) {
	if o.Think == nil {
		o.Think = func() time.Duration { return 3 * time.Second }
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	ended := make(chan struct{}, 1)
	var answered atomic.Int32
	c.On("matchmaking.proposal.created", func(e wsclient.Envelope) {
		var p struct {
			ProposalID string `json:"proposal_id"`
		}
		if json.Unmarshal(e.Payload, &p) != nil {
			return
		}
		go func() {
			_, rtt, err := c.Request(ctx, "matchmaking.proposal.accept", map[string]any{"proposal_id": p.ProposalID}, 5*time.Second)
			m.WS("mm_accept", rtt, err)
		}()
	})
	c.On("battle.question.shown", func(e wsclient.Envelope) {
		var q question
		if json.Unmarshal(e.Payload, &q) != nil || len(q.Question.Options) == 0 {
			m.Error("battle_unsupported_question")
			return
		}
		go func() {
			select {
			case <-time.After(o.Think()):
			case <-ctx.Done():
				return
			}
			_, rtt, err := c.Request(ctx, "battle.answer.submit", map[string]any{"match_id": q.MatchID, "question_instance_id": q.Question.InstanceID,
				"response": map[string]any{"option_id": q.Question.Options[0].OptionID}}, 5*time.Second)
			m.WS("battle_answer", rtt, err)
			if err == nil {
				answered.Add(1)
			}
		}()
	})
	c.On("battle.match.ended", func(wsclient.Envelope) {
		select {
		case ended <- struct{}{}:
		default:
		}
	})
	status, _, err := rc.Do(ctx, "createMatchmakingTicket", http.MethodPost, "/competition/matchmaking/tickets",
		map[string]any{"mode": o.Mode, "subject_id": o.SubjectID}, "")
	if err != nil {
		return 0, fmt.Errorf("battle: createMatchmakingTicket: %w", err)
	}
	if status != http.StatusCreated {
		return 0, fmt.Errorf("battle: createMatchmakingTicket: status %d", status)
	}
	select {
	case <-ended:
		return int(answered.Load()), nil
	case <-ctx.Done():
		return int(answered.Load()), fmt.Errorf("battle: match not ended: %w", ctx.Err())
	}
}
