package port

import "time"

// Results of eduworld_game_session_start_total{result}.
const (
	StartCreated      = "created"
	StartTakeover     = "takeover"
	StartConflict     = "conflict"
	StartDenied       = "denied"
	StartCurfew       = "curfew"
	StartUnsupported  = "client_version_unsupported"
	StartMaintenance  = "maintenance"
	StartNotProjected = "not_projected"
	StartError        = "error"
)

// SessionMetrics records the game session metrics (plan F014 "Quan sát", MONITORING).
type SessionMetrics interface {
	Started(result string, d time.Duration)
	Ended(reason string)
}
