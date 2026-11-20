class_name AudioPlayTracker
extends RefCounted
## Listening plays of an audio lesson on the client (F066-AC-19, research R1): a play is a start from the beginning or
## a replay after the end; pause, resume and seeking inside a play are not plays. The server counts (`plays_used` of
## `ActivityProgressState`); the client only asks with `play_started = true` and starts the audio once the server
## accepted. Display state only — the count shown is always the server's.

const CODE_LIMIT: String = "AUDIO_PLAY_LIMIT_REACHED"

## -1 = unlimited (`max_plays` null).
var max_plays: int = -1
var plays_used: int = 0
## A play_started report is waiting for the server.
var awaiting: bool = false
## The current play reached the end (the next start is a new play).
var ended: bool = true
var exhausted: bool = false


func configure(max_value: Variant, used: int) -> void:
	max_plays = int(max_value) if max_value != null and (max_value is int or max_value is float) else -1
	plays_used = maxi(0, used)
	exhausted = max_plays >= 0 and plays_used >= max_plays


func limited() -> bool:
	return max_plays >= 0


func plays_left() -> int:
	return -1 if max_plays < 0 else maxi(0, max_plays - plays_used)


## Whether pressing play now starts a new play (asks the server first) rather than resuming the current one.
func needs_new_play(position_ms: int) -> bool:
	return ended or position_ms <= 0


## The student asks for a new play: false when the plays are used (button greyed out) or a request is pending.
func request() -> bool:
	if exhausted or awaiting:
		return false
	awaiting = true
	return true


## The server state after a heartbeat: true when the pending play was accepted (plays_used grew) — start the audio.
func accepted(state: Dictionary) -> bool:
	if state.has("max_plays"):
		var mx: Variant = state.get("max_plays")
		max_plays = int(mx) if mx is int or mx is float else -1
	var used: int = int(state.get("plays_used", plays_used))
	var grew: bool = used > plays_used
	plays_used = maxi(plays_used, used)
	exhausted = max_plays >= 0 and plays_used >= max_plays
	if awaiting and grew:
		awaiting = false
		ended = false
		return true
	return false


## The server refused the play (`AUDIO_PLAY_LIMIT_REACHED`) or the request failed.
func refused(code: String) -> void:
	awaiting = false
	if code == CODE_LIMIT:
		exhausted = true
		if max_plays >= 0:
			plays_used = max_plays


func finished() -> void:
	ended = true
