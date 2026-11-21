package battle

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Points are the points of one answer (COMPETITION_SYSTEM §3.1, §4, F105-AC-04): CorrectPoints when right, 0 otherwise;
// from speed_bonus_min_grade a bonus ≤ SpeedBonusMax linear in the time left, measured on the server clock minus the
// RTT allowance (never for an answer flagged fast, F105-AC-24); a support turn of a team battle earns SupportPercent;
// the last question counts LastMultiplier times.
func Points(r Rules, correct bool, elapsed time.Duration, last, support, fast bool) (points, bonus int) {
	if !correct {
		return 0, 0
	}
	base := r.CorrectPoints
	if r.SpeedBonus && !fast && r.TimeLimit > 0 {
		e := max(elapsed-r.RTT, 0)
		left := max(r.TimeLimit-e, 0)
		if elapsed >= r.TimeLimit {
			left = 0
		}
		bonus = int(math.Round(float64(r.SpeedBonusMax) * left.Seconds() / r.TimeLimit.Seconds()))
	}
	points = base + bonus
	if support && r.SupportPercent > 0 {
		points = int(math.Round(float64(points) * float64(r.SupportPercent) / 100))
		bonus = int(math.Round(float64(bonus) * float64(r.SupportPercent) / 100))
	}
	if last && r.LastMultiplier > 1 {
		points, bonus = points*r.LastMultiplier, bonus*r.LastMultiplier
	}
	return points, bonus
}

// score computes the points of the closed question i, updates the totals and returns the fast-answer signals: three
// answers in a row faster than the threshold drop the bonus of the whole run (F105-AC-24, never the points).
func (r *Room) score(i int) []Signal {
	var signals []Signal
	last := i == len(r.Items)-1
	res := QuestionResult{Index: i, ClosedAt: r.Closed}
	answered := r.Answers[i]
	for _, p := range r.Players {
		a := answered[p.ID]
		ar := AnswerResult{Student: p.ID}
		if a == nil {
			p.FastRun = 0
			res.Answers = append(res.Answers, ar)
			continue
		}
		elapsed := a.At.Sub(r.Shown)
		fast := elapsed < r.R.FastAnswer
		if fast {
			p.FastRun++
		} else {
			p.FastRun = 0
		}
		flagged := r.R.FastRun > 0 && p.FastRun >= r.R.FastRun
		a.Fast = flagged
		a.Points, a.Bonus = Points(r.R, a.Correct, elapsed, last, a.Support, flagged)
		if flagged && p.FastRun == r.R.FastRun {
			signals = append(signals, Signal{Student: p.ID, Type: SignalFastAnswer, Index: i,
				Evidence: map[string]any{"run": p.FastRun, "threshold_ms": r.R.FastAnswer.Milliseconds(), "elapsed_ms": elapsed.Milliseconds()}})
			r.dropRunBonus(p, r.R.FastRun-1)
		}
		p.Score += a.Points
		if a.Correct {
			p.Correct++
		}
		ar.Answered, ar.Correct, ar.Points, ar.Bonus, ar.ReceivedMS, ar.Support = true, a.Correct, a.Points, a.Bonus, elapsed.Milliseconds(), a.Support
		res.Answers = append(res.Answers, ar)
	}
	if r.Bot != nil {
		right := r.Bot.Answered && r.Bot.Right
		pts, _ := Points(r.R, right, r.Bot.Due.Sub(r.Shown), last, false, false)
		r.Bot.Points = pts
		r.Bot.Score += pts
		if right {
			r.Bot.Correct++
		}
		res.BotRight = &right
	}
	if r.R.Collective {
		for _, a := range res.Answers {
			if a.Correct {
				r.CoopFires++ // one fire out per question answered right by someone
				break
			}
		}
	}
	r.Results = append(r.Results, res)
	return signals
}

// dropRunBonus removes the speed bonus of the previous n answers of p (the start of a fast run).
func (r *Room) dropRunBonus(p *Player, n int) {
	for k := len(r.Results) - 1; k >= 0 && n > 0; k-- {
		for j := range r.Results[k].Answers {
			a := &r.Results[k].Answers[j]
			if a.Student != p.ID {
				continue
			}
			if a.Answered && a.Bonus > 0 {
				p.Score -= a.Bonus
				a.Points -= a.Bonus
				a.Bonus = 0
			}
			n--
		}
	}
}

// unscore reverts the points of question i (a failover replays it).
func (r *Room) unscore(i int) {
	if len(r.Results) == 0 || r.Results[len(r.Results)-1].Index != i {
		return
	}
	res := r.Results[len(r.Results)-1]
	for _, a := range res.Answers {
		if p := r.player(a.Student); p != nil && a.Answered {
			p.Score -= a.Points
			if a.Correct {
				p.Correct--
			}
		}
	}
	if r.Bot != nil && res.BotRight != nil {
		r.Bot.Score -= r.Bot.Points
		if *res.BotRight {
			r.Bot.Correct--
		}
	}
	if r.R.Collective {
		for _, a := range res.Answers {
			if a.Correct {
				r.CoopFires--
				break
			}
		}
	}
	r.Results = r.Results[:len(r.Results)-1]
}

// BotAnswer is the answer of the practice bot to question index of a match: its delay in [MinMS, MaxMS] and whether it
// is right with probability Accuracy — reproducible per match and question (failover, tests).
type BotAnswerPlan struct {
	Delay   time.Duration
	Correct bool
}

// BotAnswer draws the answer of the bot (MATCHMAKING §9).
func BotAnswer(p BotProfile, match uuid.UUID, index int) BotAnswerPlan {
	var seed uint64
	for _, b := range match[:8] {
		seed = seed<<8 | uint64(b)
	}
	rng := rand.New(rand.NewPCG(seed, uint64(index)+0x9e3779b97f4a7c15))
	span := max(p.MaxMS-p.MinMS, 0)
	d := p.MinMS
	if span > 0 {
		d += rng.IntN(span + 1)
	}
	return BotAnswerPlan{Delay: time.Duration(d) * time.Millisecond, Correct: rng.Float64() < p.Accuracy}
}

// Goal is the collective goal of a co-op challenge or a timed quiz of grades 1–3 (no personal rank).
type Goal struct {
	Target   int  `json:"target"`
	Achieved int  `json:"achieved"`
	Reached  bool `json:"reached"`
}

// PlayerResult is the result of one student.
type PlayerResult struct {
	ID          uuid.UUID
	Tenant      uuid.UUID
	School      uuid.UUID
	Team        int
	Outcome     string
	Score       int
	Correct     int
	Disconnects int
	ConnectedMS int64
}

// BotResult is the result of the practice bot (match_results.summary.bot).
type BotResult struct {
	Score   int `json:"score"`
	Correct int `json:"correct"`
}

// Result is the result of the match (competition.match_results): status completed or cancelled, outcomes, totals,
// rated (data-model §3: ranked, completed, not bot, not tournament, not a server failure).
type Result struct {
	MatchID    uuid.UUID
	Status     string
	EndReason  string
	WinnerTeam *int
	Rated      bool
	EndedAt    time.Time
	Players    []PlayerResult
	Questions  []QuestionResult
	Bot        *BotResult
	GroupGoal  *Goal
}

// Result computes the result of the room as it stands.
func (r *Room) Result() Result {
	res := Result{MatchID: r.ID, Status: "completed", EndReason: r.EndReason, EndedAt: r.EndedAt, Questions: r.Results}
	if r.Cancelled {
		res.Status = "cancelled"
	}
	team := map[int]int{}
	for _, p := range r.Players {
		team[p.Team] += p.Score
	}
	if r.Bot != nil {
		team[1] += r.Bot.Score
		res.Bot = &BotResult{Score: r.Bot.Score, Correct: r.Bot.Correct}
	}
	versus := !r.R.Collective && r.R.Mode != ModeTimedQuiz
	if versus && !r.Cancelled && r.St != StateAborted {
		var in []int // teams with a player still in (abandon/forfeit: the side that stayed wins)
		for _, p := range r.Players {
			if p.Out == "" && !slices.Contains(in, p.Team) {
				in = append(in, p.Team)
			}
		}
		if r.Bot != nil && !slices.Contains(in, 1) {
			in = append(in, 1)
		}
		switch {
		case (r.EndReason == EndAbandon || r.EndReason == EndForfeit) && len(in) == 1:
			w := in[0]
			res.WinnerTeam = &w
		case team[0] > team[1]:
			w := 0
			res.WinnerTeam = &w
		case team[1] > team[0]:
			w := 1
			res.WinnerTeam = &w
		}
	}
	for _, p := range r.Players {
		pr := PlayerResult{ID: p.ID, Tenant: p.Tenant, School: p.School, Team: p.Team, Score: p.Score, Correct: p.Correct,
			Disconnects: p.Graces, ConnectedMS: p.ConnectedMS}
		switch {
		case r.Cancelled || r.St == StateAborted: // nobody abandons a cancelled or aborted match
			pr.Outcome = OutcomeNoContest
		case p.Out != "":
			pr.Outcome = p.Out
		case !versus:
			pr.Outcome = OutcomeNoContest
		case res.WinnerTeam == nil:
			pr.Outcome = OutcomeDraw
		case *res.WinnerTeam == p.Team:
			pr.Outcome = OutcomeWin
		default:
			pr.Outcome = OutcomeLoss
		}
		res.Players = append(res.Players, pr)
	}
	if r.R.GroupGoal {
		g := Goal{}
		if r.R.Collective {
			g.Target, g.Achieved = r.R.CoopTarget, r.CoopFires
		} else {
			total := 0
			for _, p := range r.Players {
				total += p.Correct
			}
			g.Target = int(math.Ceil(float64(len(r.Players)*len(r.Items)*r.R.GoalPercent) / 100))
			g.Achieved = total
		}
		g.Reached = g.Achieved >= g.Target
		res.GroupGoal = &g
	}
	res.Rated = r.Ranked && res.Status == "completed" && r.St != StateAborted && !r.IsBot && !r.Tournament &&
		r.EndReason != EndServerFailure && r.EndReason != EndPlanFailure && versus
	return res
}

// rank is the 1-based rank of each student by score (ties share a rank).
func rank(ps []PlayerResult) map[uuid.UUID]int {
	sorted := slices.Clone(ps)
	slices.SortStableFunc(sorted, func(a, b PlayerResult) int { return b.Score - a.Score })
	out := map[uuid.UUID]int{}
	for i, p := range sorted {
		if i > 0 && p.Score == sorted[i-1].Score {
			out[p.ID] = out[sorted[i-1].ID]
			continue
		}
		out[p.ID] = i + 1
	}
	return out
}

func botOutcome(res Result) string {
	switch {
	case res.Status != "completed":
		return OutcomeNoContest
	case res.WinnerTeam == nil:
		return OutcomeDraw
	case *res.WinnerTeam == 1:
		return OutcomeWin
	}
	return OutcomeLoss
}
