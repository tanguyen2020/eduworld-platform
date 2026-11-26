class_name ChallengeStore
extends RefCounted
## RAM store of the solo challenge run (ADR-0064, F069-AC-28) and its server clock (F069-AC-18): the time left is
## `deadline_at − server now`, where server now = a server time captured when an attempt arrives (realtime clock
## offset, else the HTTP `Date` header, else the device time at that moment) plus the MONOTONIC time elapsed since —
## changing the device clock during the run changes nothing (SC-005). Only messages of the current attempt are taken
## (data-model §3). While a run is open (Starting … Result) `AppState.blocking_activity` = `challenge`, which holds the
## level-up modal (F069-AC-03).

signal attempt_changed(attempt: ChallengeAttemptModel)

const BLOCKING_ACTIVITY: StringName = &"challenge"

var attempt: ChallengeAttemptModel = null
## Details of the challenge being played (title, limits) when known.
var detail: ChallengeDetailModel = null
var countdown: ExamCountdown = ExamCountdown.new()
var clock: ClientClock = null
## AppState (or a test fake) with `begin_blocking(name)` / `end_blocking(name)`.
var app_state: Object = null
var _blocking: bool = false


func _init(time_source: ClientClock = null, store: Object = null) -> void:
	clock = time_source if time_source != null else ClientClock.new()
	app_state = store


## Takes a server attempt; `server_ms` = server unix ms at this moment (≤ 0 → device time now).
func set_attempt(value: ChallengeAttemptModel, server_ms: int = 0) -> void:
	if value == null:
		return
	if attempt != null and attempt.challenge_attempt_id == value.challenge_attempt_id:
		value.activity_seq = maxi(value.activity_seq, attempt.activity_seq)
	attempt = value
	sync_clock(server_ms)
	_block(true)
	attempt_changed.emit(attempt)


## Applies a `challenge.attempt.updated` payload of the CURRENT attempt; false (ignored) for any other attempt.
func apply_message(payload: Dictionary) -> bool:
	if attempt == null or DataRead.str_of(payload, "challenge_attempt_id") != attempt.challenge_attempt_id:
		return false
	attempt.apply_update(payload)
	countdown.deadline_ms = ExamCountdown.iso_to_ms(attempt.deadline_at)
	attempt_changed.emit(attempt)
	return true


## Aligns the server clock on `server_ms` (server unix ms now); ≤ 0 uses the device time once.
func sync_clock(server_ms: int = 0) -> void:
	var local: int = clock.now_msec()
	var server: int = server_ms if server_ms > 0 else int(clock.unix_time() * 1000.0)
	countdown.deadline_ms = ExamCountdown.iso_to_ms(attempt.deadline_at) if attempt != null else 0
	countdown.offset_ms = server - local


func has_deadline() -> bool:
	return attempt != null and countdown.has_deadline()


## Milliseconds left until `deadline_at` (-1 without a deadline).
func remaining_ms() -> int:
	return countdown.remaining_ms(clock.now_msec()) if has_deadline() else -1


## Server unix milliseconds now (monotonic since the last sync).
func server_now_ms() -> int:
	return countdown.server_now_ms(clock.now_msec())


func is_running() -> bool:
	return attempt != null and not attempt.is_finished()


## The result screen was closed: the run is over for the client, the level-up modal may show.
func clear() -> void:
	attempt = null
	detail = null
	countdown = ExamCountdown.new()
	_block(false)
	attempt_changed.emit(null)


func _block(on: bool) -> void:
	if on == _blocking:
		return
	_blocking = on
	if app_state != null and app_state.has_method("begin_blocking"):
		app_state.call("begin_blocking" if on else "end_blocking", BLOCKING_ACTIVITY)
