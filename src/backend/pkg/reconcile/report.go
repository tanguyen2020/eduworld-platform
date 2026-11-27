package reconcile

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
)

// MaxMismatches is the cap of Report.Mismatches (reconcile-report.schema.json); counts stay complete.
const MaxMismatches = 100_000

// Exit codes of `worker reconcile` (F134-AC-03).
const (
	ExitOK          = 0
	ExitBadArgs     = 1
	ExitUnexplained = 2
	ExitIncomplete  = 3
)

// Report is specs/schemas/ops/reconcile-report.schema.json: IDs, types, consumers, tenants, times and classes only —
// never a payload nor personal data (AGENTS R14).
type Report struct {
	RunID            uuid.UUID   `json:"run_id"`
	Module           string      `json:"module"`
	RestorePoint     time.Time   `json:"restore_point"`
	WindowFrom       time.Time   `json:"window_from"`
	WindowTo         time.Time   `json:"window_to"`
	SettleSeconds    int64       `json:"settle_seconds"`
	GeneratedAt      time.Time   `json:"generated_at"`
	Status           string      `json:"status"` // complete | incomplete
	CompensationHint string      `json:"compensation_hint,omitempty"`
	Peers            []PeerState `json:"peers"`
	Counts           Counts      `json:"counts"`
	Mismatches       []Mismatch  `json:"mismatches"`
	Truncated        bool        `json:"truncated"`
}

// PeerState is the outcome of reading one module in one direction.
type PeerState struct {
	Module    string    `json:"module"`
	Direction Direction `json:"direction"`
	Status    string    `json:"status"` // ok | failed
	Consumers []string  `json:"consumers,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
}

// Counts per class.
type Counts struct {
	LostAfterRestore int `json:"lost_after_restore"`
	PendingPublish   int `json:"pending_publish"`
	PendingDelivery  int `json:"pending_delivery"`
	DeadLettered     int `json:"dead_lettered"`
	Unexplained      int `json:"unexplained"`
}

// Of returns the count of class c.
func (c Counts) Of(cl Class) int {
	switch cl {
	case LostAfterRestore:
		return c.LostAfterRestore
	case PendingPublish:
		return c.PendingPublish
	case PendingDelivery:
		return c.PendingDelivery
	case DeadLettered:
		return c.DeadLettered
	default:
		return c.Unexplained
	}
}

func (c *Counts) add(cl Class) {
	switch cl {
	case LostAfterRestore:
		c.LostAfterRestore++
	case PendingPublish:
		c.PendingPublish++
	case PendingDelivery:
		c.PendingDelivery++
	case DeadLettered:
		c.DeadLettered++
	case Unexplained:
		c.Unexplained++
	}
}

// Mismatch is one (consumer, event) the two sides disagree on.
type Mismatch struct {
	EventID        uuid.UUID  `json:"event_id"`
	EventType      string     `json:"event_type,omitempty"`
	Consumer       string     `json:"consumer"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty"`
	EventTime      time.Time  `json:"event_time"`
	Classification Class      `json:"classification"`
	Direction      Direction  `json:"direction"`
}

func (r *Report) record(m Mismatch) {
	r.Counts.add(m.Classification)
	if len(r.Mismatches) >= MaxMismatches {
		r.Truncated = true
		return
	}
	r.Mismatches = append(r.Mismatches, m)
}

// Complete reports whether every module could be read.
func (r Report) Complete() bool { return r.Status == "complete" }

// ExitCode is 2 when an unexplained mismatch was found, 3 when a module could not be read (and nothing unexplained was
// found in what was read), 0 otherwise (F134-AC-03). A definite unexplained mismatch wins over an incomplete run.
func (r Report) ExitCode() int {
	switch {
	case r.Counts.Unexplained > 0:
		return ExitUnexplained
	case !r.Complete():
		return ExitIncomplete
	default:
		return ExitOK
	}
}

// Summary writes the human summary table (stderr of the job): peers, then counts per class.
func (r Report) Summary(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "reconcile-%s\trun %s\tstatus %s\texit %d\n", r.Module, r.RunID, r.Status, r.ExitCode())
	fmt.Fprintf(tw, "window\t%s → %s\tT %s\tsettle %ds\n", r.WindowFrom.Format(time.RFC3339), r.WindowTo.Format(time.RFC3339),
		r.RestorePoint.Format(time.RFC3339), r.SettleSeconds)
	peers := append([]PeerState(nil), r.Peers...)
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Direction != peers[j].Direction {
			return peers[i].Direction > peers[j].Direction // outbound first
		}
		return peers[i].Module < peers[j].Module
	})
	for _, p := range peers {
		fmt.Fprintf(tw, "peer\t%s\t%s\t%s %s\n", p.Direction, p.Module, p.Status, p.ErrorCode)
	}
	for _, c := range Classes {
		fmt.Fprintf(tw, "count\t%s\t%d\t\n", c, r.Counts.Of(c))
	}
	if r.Truncated {
		fmt.Fprintf(tw, "note\tmismatches truncated at %d\t\t\n", MaxMismatches)
	}
	return tw.Flush()
}
