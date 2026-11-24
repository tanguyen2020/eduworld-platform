package tournament

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Node statuses (data-model §2).
const (
	NodePending         = "pending"
	NodeScheduled       = "scheduled"
	NodeInMatch         = "in_match"
	NodeAwaitingConfirm = "awaiting_confirm"
	NodeConfirmed       = "confirmed"
	NodeBye             = "bye"
	NodeOverridden      = "overridden"
	NodeVoid            = "void"
)

// Brackets of a node (same values as package bracket).
const (
	BracketWinners    = "winners"
	BracketLosers     = "losers"
	BracketGrandFinal = "grand_final"
	BracketGroup      = "group"
)

// Errors of the bracket graph.
var (
	// ErrNodeState: the node is not in a state accepting the operation (422 INVALID_STATE_TRANSITION).
	ErrNodeState = errors.New("tournament: node state")
	// ErrNextStarted: a next node already started, the result can no longer change (F107-AC-18).
	ErrNextStarted = errors.New("tournament: the next node already started")
	// ErrNotInNode: the entry is not one of the two entries of the node.
	ErrNotInNode = errors.New("tournament: entry not in node")
)

// Side is the result of one side of a match (from match.completed: correct answers, time of the correct answers, outcome).
type Side struct {
	Correct int    `json:"correct"`
	TimeMS  int64  `json:"time_ms"`
	Outcome string `json:"outcome"`
}

// Result is the stored result of a node (bracket_nodes.result).
type Result struct {
	A             Side   `json:"a"`
	B             Side   `json:"b"`
	DoubleForfeit bool   `json:"double_forfeit,omitempty"`
	Forfeit       string `json:"forfeit,omitempty"` // "a" or "b": that side did not show up / had too few members
	Played        bool   `json:"played"`            // a real match was completed (not a no-show)
}

// Node is a bracket node in memory.
type Node struct {
	ID               uuid.UUID
	Bracket          string
	Round, Position  int
	GroupID          *uuid.UUID
	A, B             *uuid.UUID
	Winner           *uuid.UUID
	Draw             bool
	Result           *Result
	MatchID          *uuid.UUID
	ScheduledAt      *time.Time
	StartedAt        *time.Time
	NextWin          *uuid.UUID
	NextWinSlot      int
	NextLose         *uuid.UUID
	NextLoseSlot     int
	Wave             int
	Attempts         int
	Reschedules      int
	Notices          int
	Status           string
	ConfirmAfter     *time.Time
	DecisionRequired bool
	OverrideReason   *string
	OverriddenBy     *uuid.UUID
	Version          int64
	Changed          bool
}

// Done reports a node whose outputs are known (its winner and loser moved on, or nothing to move).
func (n *Node) Done() bool {
	switch n.Status {
	case NodeAwaitingConfirm, NodeConfirmed, NodeOverridden, NodeBye, NodeVoid:
		return true
	}
	return false
}

// Final reports a node that will not change any more (complaint window over).
func (n *Node) Final() bool {
	switch n.Status {
	case NodeConfirmed, NodeOverridden, NodeBye, NodeVoid:
		return true
	}
	return false
}

// Started reports a node whose match began (or is over): its entries can no longer change (F107-AC-18).
func (n *Node) Started() bool {
	return n.StartedAt != nil || n.Status == NodeInMatch || (n.Done() && n.Status != NodeBye)
}

// Elimination reports a node of a knockout bracket (a group node never eliminates).
func (n *Node) Elimination() bool { return n.Bracket != BracketGroup }

// Slot returns the entry of slot 0 (A) or 1 (B).
func (n *Node) Slot(s int) *uuid.UUID {
	if s == 0 {
		return n.A
	}
	return n.B
}

func (n *Node) setSlot(s int, id *uuid.UUID) {
	if s == 0 {
		n.A = id
	} else {
		n.B = id
	}
	n.Changed = true
}

// Loser returns the other entry of a decided node.
func (n *Node) Loser() *uuid.UUID {
	if n.Winner == nil || n.A == nil || n.B == nil {
		return nil
	}
	if *n.Winner == *n.A {
		return n.B
	}
	return n.A
}

// Has reports whether entry plays in n.
func (n *Node) Has(entry uuid.UUID) bool {
	return (n.A != nil && *n.A == entry) || (n.B != nil && *n.B == entry)
}

// Graph is the bracket of a tournament.
type Graph struct {
	Nodes []*Node
	byID  map[uuid.UUID]*Node
}

// NewGraph indexes nodes.
func NewGraph(nodes []*Node) *Graph {
	g := &Graph{Nodes: nodes, byID: make(map[uuid.UUID]*Node, len(nodes))}
	for _, n := range nodes {
		g.byID[n.ID] = n
	}
	return g
}

// Node returns the node id (nil when unknown).
func (g *Graph) Node(id uuid.UUID) *Node { return g.byID[id] }

// Add appends a node (knockout after the groups).
func (g *Graph) Add(n *Node) {
	g.Nodes = append(g.Nodes, n)
	g.byID[n.ID] = n
}

// feeders returns the nodes sending their winner or loser into slot s of n.
func (g *Graph) feeders(n *Node, s int) []*Node {
	var out []*Node
	for _, f := range g.Nodes {
		if (f.NextWin != nil && *f.NextWin == n.ID && f.NextWinSlot == s) || (f.NextLose != nil && *f.NextLose == n.ID && f.NextLoseSlot == s) {
			out = append(out, f)
		}
	}
	return out
}

// slotState: filled (an entry), empty (nothing will ever come), waiting (a feeder is not done yet).
type slotState int

const (
	slotWaiting slotState = iota
	slotFilled
	slotEmpty
)

func (g *Graph) state(n *Node, s int) slotState {
	if n.Slot(s) != nil {
		return slotFilled
	}
	for _, f := range g.feeders(n, s) {
		if !f.Done() {
			return slotWaiting
		}
	}
	return slotEmpty
}

// Resolve settles the pending nodes until nothing changes (F107-AC-10, AC-16): a node with two entries is scheduled;
// one entry and an empty slot is a bye (the entry moves on); no entry is void (the next node gets a bye in turn).
func (g *Graph) Resolve() {
	for changed := true; changed; {
		changed = false
		for _, n := range g.Nodes {
			if n.Status != NodePending && n.Status != NodeScheduled {
				continue
			}
			a, b := g.state(n, 0), g.state(n, 1)
			switch {
			case a == slotFilled && b == slotFilled:
				if n.Status == NodePending {
					n.Status, n.Changed, changed = NodeScheduled, true, true
				}
			case a == slotWaiting || b == slotWaiting:
				if n.Status == NodeScheduled {
					n.Status, n.Changed, changed = NodePending, true, true
				}
			case a == slotEmpty && b == slotEmpty:
				n.Status, n.Changed, changed = NodeVoid, true, true
			default: // one entry, the other slot empty
				w := n.A
				if w == nil {
					w = n.B
				}
				n.Winner, n.Status, n.Changed, changed = w, NodeBye, true, true
				g.push(n, w, nil)
			}
		}
	}
}

func (g *Graph) push(n *Node, winner, loser *uuid.UUID) {
	if n.NextWin != nil && winner != nil {
		if nx := g.byID[*n.NextWin]; nx != nil {
			nx.setSlot(n.NextWinSlot, winner)
		}
	}
	if n.NextLose != nil && loser != nil {
		if nx := g.byID[*n.NextLose]; nx != nil {
			nx.setSlot(n.NextLoseSlot, loser)
		}
	}
}

// Withdraw takes entries out of the nodes not started yet (check-in closed, F107-AC-10): an elimination slot becomes
// empty (bye for the opponent, void when both left), a group match with a withdrawn entry is void.
func (g *Graph) Withdraw(entries map[uuid.UUID]bool) {
	for _, n := range g.Nodes {
		if n.Status != NodePending && n.Status != NodeScheduled {
			continue
		}
		for s := 0; s < 2; s++ {
			if e := n.Slot(s); e != nil && entries[*e] {
				if n.Elimination() {
					n.setSlot(s, nil)
					n.Status = NodePending
				} else {
					n.Status, n.Changed = NodeVoid, true
				}
			}
		}
	}
	g.Resolve()
}

// Outcome is the decided result of a node: a winner, a draw (group matches only) or a double forfeit.
type Outcome struct {
	Winner        *uuid.UUID
	Draw          bool
	DoubleForfeit bool
	Result        Result
}

// Apply records the outcome of a node (F107-AC-16, AC-17): the winner and the loser move to their next nodes at once
// (the next match may start during the complaint window), the node waits for confirmation until confirmAfter. A double
// forfeit in a knockout voids the node (both eliminated, the next node gets a bye); in a group both lose. The node
// must be scheduled or in a match; anything else is ErrNodeState (a replayed result changes nothing).
func (g *Graph) Apply(n *Node, o Outcome, confirmAfter time.Time) error {
	if n.Status != NodeScheduled && n.Status != NodeInMatch {
		return ErrNodeState
	}
	if o.Winner != nil && !n.Has(*o.Winner) {
		return ErrNotInNode
	}
	r := o.Result
	n.Result, n.Changed = &r, true
	n.DecisionRequired = false
	if o.DoubleForfeit && n.Elimination() {
		n.Winner, n.Status = nil, NodeVoid
		g.Resolve()
		return nil
	}
	n.Winner, n.Draw = o.Winner, o.Draw && !n.Elimination()
	n.Status = NodeAwaitingConfirm
	ca := confirmAfter
	n.ConfirmAfter = &ca
	if n.Winner != nil {
		g.push(n, n.Winner, n.Loser())
	}
	g.Resolve()
	return nil
}

// Confirm closes the complaint window of a decided node (F107-AC-17): awaiting_confirm with a result → confirmed.
func (n *Node) Confirm() error {
	if n.Status != NodeAwaitingConfirm || n.DecisionRequired || (n.Winner == nil && !n.Draw && (n.Result == nil || !n.Result.DoubleForfeit)) {
		return ErrNodeState
	}
	n.Status, n.Changed = NodeConfirmed, true
	return nil
}

// RequireDecision marks a node the organizer must decide (after the last reschedule of a server failure, F107-AC-19).
func (n *Node) RequireDecision() {
	n.Status, n.DecisionRequired, n.Changed = NodeAwaitingConfirm, true, true
	n.ConfirmAfter = nil
}

// Override changes the winner of a node in its complaint window or of a node awaiting the organizer's decision
// (F107-AC-18, AC-19): the next nodes must not have started; the entries already moved on are swapped in the same
// change set. now is checked against the complaint window.
func (g *Graph) Override(n *Node, winner uuid.UUID, reason string, by uuid.UUID, now time.Time) error {
	inWindow := n.Status == NodeAwaitingConfirm && (n.DecisionRequired || (n.ConfirmAfter != nil && now.Before(*n.ConfirmAfter)))
	if !inWindow {
		return ErrNodeState
	}
	if n.A == nil || n.B == nil || !n.Has(winner) {
		return ErrNotInNode
	}
	for _, id := range []*uuid.UUID{n.NextWin, n.NextLose} {
		if id == nil {
			continue
		}
		if nx := g.byID[*id]; nx != nil && (nx.Started() || nx.Status == NodeBye || nx.Status == NodeVoid) {
			return ErrNextStarted
		}
	}
	w := winner
	n.Winner, n.Draw, n.DecisionRequired = &w, false, false
	g.push(n, n.Winner, n.Loser())
	r := reason
	b := by
	n.Status, n.OverrideReason, n.OverriddenBy, n.Changed = NodeOverridden, &r, &b, true
	g.Resolve()
	return nil
}

// Complete reports whether every node is final (the tournament or its stage is over).
func (g *Graph) Complete(filter func(*Node) bool) bool {
	seen := false
	for _, n := range g.Nodes {
		if filter != nil && !filter(n) {
			continue
		}
		seen = true
		if !n.Final() {
			return false
		}
	}
	return seen
}
