class_name FakeClock
extends ClientClock
## TEST-ONLY (AGENTS R17): controllable clock for retry/backoff/heartbeat/expiry tests. `wait()` records the
## delay, advances time and resumes on the next frame; delays of HOLD_THRESHOLD_SEC or more (heartbeat in the
## harness) stay pending until `release()`.

signal released()

const HOLD_THRESHOLD_SEC: float = 1000.0

var now_ms: int = 1_000_000
var unix: float = 1_763_600_000.0
var waits: Array[float] = []
var held: int = 0


func now_msec() -> int:
	return now_ms


func unix_time() -> float:
	return unix


func wait(seconds: float) -> void:
	waits.append(seconds)
	if seconds >= HOLD_THRESHOLD_SEC:
		held += 1
		await released
		held -= 1
		return
	advance(seconds)
	await (Engine.get_main_loop() as SceneTree).process_frame


func advance(seconds: float) -> void:
	now_ms += int(seconds * 1000.0)
	unix += seconds


## Resumes every held wait (one heartbeat tick).
func release() -> void:
	released.emit()


## Sum of the recorded waits shorter than the hold threshold.
func waited_total() -> float:
	var total: float = 0.0
	for w: float in waits:
		if w < HOLD_THRESHOLD_SEC:
			total += w
	return total
