class_name ClientClock
extends RefCounted
## Time source of the network and auth services (research R3): token expiry uses the monotonic clock
## (`Time.get_ticks_msec()`, unaffected by a changed system clock); waits use SceneTree timers. Tests replace it
## with `FakeClock` (tests/fakes) so retries, backoff and heartbeats run without real sleeping.


func now_msec() -> int:
	return Time.get_ticks_msec()


## Wall-clock unix seconds (used only with the server time offset, e.g. curfew warnings).
func unix_time() -> float:
	return Time.get_unix_time_from_system()


## Coroutine: waits `seconds` (process-always timer, ignores pause and time scale).
func wait(seconds: float) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	if tree == null or seconds <= 0.0:
		return
	await tree.create_timer(seconds, true, false, true).timeout
