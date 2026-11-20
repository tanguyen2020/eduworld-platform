class_name VideoProgressReporter
extends RefCounted
## Heartbeats of a video attempt (F065-AC-19, VIDEO_LEARNING §4): while the video plays the watched span grows from
## the position where playback (re)started; every `heartbeat_seconds` of real time — and on pause, seek or exit — it
## produces an `ActivityProgressReport` `{client_seq, position_ms, watched_ranges, playback_rate}`. `client_seq` is the
## unix time in ms, strictly increasing, so a resumed attempt (another session or device) is never ignored by the
## server. The reporter never decides completion: the server does.

const DEFAULT_HEARTBEAT_SEC: float = 10.0

var ranges: WatchedRanges = WatchedRanges.new()
var heartbeat_sec: float = DEFAULT_HEARTBEAT_SEC
var playback_rate: float = 1.0
var last_seq: int = 0
## Start of the span being watched (-1 while paused).
var span_start_ms: int = -1
var position_ms: int = 0
var _elapsed: float = 0.0
## Unix ms source (tests inject a fixed clock).
var now_ms: Callable = func() -> int: return int(Time.get_unix_time_from_system() * 1000.0)


## Playback started or resumed at `at_ms`.
func started(at_ms: int) -> void:
	span_start_ms = at_ms
	position_ms = at_ms


## Playback advanced to `at_ms` after `delta` seconds of real time; returns a report when a heartbeat is due.
func advanced(at_ms: int, delta: float) -> Dictionary:
	if span_start_ms < 0:
		span_start_ms = at_ms
	position_ms = at_ms
	_elapsed += delta
	if _elapsed < heartbeat_sec:
		return {}
	return report()


## Paused (or seeking away): closes the span and returns the report to send now.
func paused(at_ms: int) -> Dictionary:
	position_ms = at_ms
	var out: Dictionary = report()
	span_start_ms = -1
	return out


## Builds the report and closes the current span into the ranges.
func report() -> Dictionary:
	if span_start_ms >= 0 and position_ms > span_start_ms:
		ranges.add(span_start_ms, position_ms)
		span_start_ms = position_ms
	_elapsed = 0.0
	last_seq = maxi(last_seq + 1, int(now_ms.call()))
	return {"client_seq": last_seq, "position_ms": maxi(0, position_ms), "watched_ranges": ranges.to_payload(),
		"playback_rate": clampf(playback_rate, 0.5, 2.0)}
