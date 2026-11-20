package port

import "time"

// Results of eduworld_game_session_start_total{result}.
const (
	StartCreated      = "created"
	StartTakeover     = "takeover"
	StartConflict     = "conflict"
	StartDenied       = "denied"
	StartCurfew       = "curfew"
	StartDailyLimit   = "daily_limit"
	StartUnsupported  = "client_version_unsupported"
	StartMaintenance  = "maintenance"
	StartNotProjected = "not_projected"
	StartError        = "error"
)

// SessionMetrics records the game session metrics (MONITORING §3: eduworld_game_logins_total,
// eduworld_game_sessions_active, eduworld_game_session_takeovers_total, eduworld_game_session_ended_total,
// eduworld_game_access_denied_total; plan F014: start outcome and duration).
type SessionMetrics interface {
	Started(result string, d time.Duration)
	Login(tenant, platform string)
	Takeover(platform string)
	Ended(reason string)
	Denied(reason string)
	// Active sets the active sessions per platform (gauge, refreshed by the job session-timeout).
	Active(byPlatform map[string]int)
}
