class_name NetStats
extends RefCounted
## Network quality counters of the realtime connection (F092 T028, GAME_NETWORKING §10): RTT samples from
## heartbeat pong, reconnects, resume results, bytes in/out. Kept in AppState for the batched telemetry of F080;
## no PII, no payload.

const RTT_WINDOW: int = 50

var rtt_samples: Array[int] = []
var reconnects: int = 0
var resume_ok: int = 0
var resume_failed: int = 0
var bytes_in: int = 0
var bytes_out: int = 0


func add_rtt(ms: int) -> void:
	rtt_samples.append(maxi(ms, 0))
	while rtt_samples.size() > RTT_WINDOW:
		rtt_samples.pop_front()


func rtt_percentile(pct: float) -> int:
	if rtt_samples.is_empty():
		return -1
	var sorted: Array[int] = rtt_samples.duplicate()
	sorted.sort()
	var index: int = clampi(int(ceil(pct / 100.0 * float(sorted.size()))) - 1, 0, sorted.size() - 1)
	return sorted[index]


func to_dict() -> Dictionary:
	return {
		"rtt_p50_ms": rtt_percentile(50.0), "rtt_p95_ms": rtt_percentile(95.0), "reconnects": reconnects,
		"resume_ok": resume_ok, "resume_failed": resume_failed, "bytes_in": bytes_in, "bytes_out": bytes_out,
	}
