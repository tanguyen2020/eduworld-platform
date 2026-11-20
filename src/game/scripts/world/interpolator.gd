class_name Interpolator
extends RefCounted
## Display of the other players between server snapshots (GAME_NETWORKING §5.2; F092-AC-12): render at
## `render_time = server_now − delay`, lerp position / lerp_angle yaw between the two samples around it, animation of
## the nearest earlier sample; past the newest sample extrapolate with its velocity for at most EXTRAPOLATE_MAX_MS,
## then stand still. `delay` starts at `client_interpolation` (100 ms) and grows with the measured arrival jitter up
## to its ceiling (200 ms). Display only: the client never decides where another player really is (R5).

## CONFIG_DEFAULTS `client_interpolation`: 100 ms, at most 200 ms.
const BASE_DELAY_MS: int = 100
const MAX_DELAY_MS: int = 200
## GAME_NETWORKING §5.2: extrapolate at most 250 ms, then stand still.
const EXTRAPOLATE_MAX_MS: int = 250
## Weight of a new jitter sample in the moving average.
const JITTER_SMOOTHING: float = 0.1
## Delay = base + JITTER_FACTOR × jitter (clamped).
const JITTER_FACTOR: float = 2.0

var delay_ms: int = BASE_DELAY_MS
var jitter_ms: float = 0.0
var _last_transit_ms: int = 0
var _has_transit: bool = false


## Records one snapshot arrival (`arrival_ms` on the client's server-clock estimate, `server_ms` of the message) and
## adapts the delay to the transit-time variation (RFC 3550 style jitter).
func observe(arrival_ms: int, server_ms: int) -> void:
	var transit: int = arrival_ms - server_ms
	if _has_transit:
		var d: float = absf(float(transit - _last_transit_ms))
		jitter_ms += (d - jitter_ms) * JITTER_SMOOTHING
	_last_transit_ms = transit
	_has_transit = true
	delay_ms = clampi(BASE_DELAY_MS + int(JITTER_FACTOR * jitter_ms), BASE_DELAY_MS, MAX_DELAY_MS)


func render_time(server_now_ms: int) -> int:
	return server_now_ms - delay_ms


## State of one entity at `render_ms`: `{p: Vector3, yaw: float, anim: String, extrapolated: bool}`; empty when there
## is no sample.
static func sample_at(samples: Array, render_ms: int) -> Dictionary:
	if samples.is_empty():
		return {}
	var first: SnapshotSample = samples[0] as SnapshotSample
	if render_ms <= first.server_ms or samples.size() == 1:
		return {"p": first.p, "yaw": first.yaw, "anim": first.anim, "extrapolated": false} if render_ms <= first.server_ms \
				else _extrapolate(null, first, render_ms)
	for i: int in range(samples.size() - 1):
		var a: SnapshotSample = samples[i] as SnapshotSample
		var b: SnapshotSample = samples[i + 1] as SnapshotSample
		if render_ms >= a.server_ms and render_ms < b.server_ms:
			var t: float = float(render_ms - a.server_ms) / float(maxi(b.server_ms - a.server_ms, 1))
			return {"p": a.p.lerp(b.p, t), "yaw": lerp_angle(a.yaw, b.yaw, t), "anim": a.anim, "extrapolated": false}
	var last: SnapshotSample = samples[samples.size() - 1] as SnapshotSample
	var before: SnapshotSample = samples[samples.size() - 2] as SnapshotSample
	return _extrapolate(before, last, render_ms)


static func _extrapolate(before: SnapshotSample, last: SnapshotSample, render_ms: int) -> Dictionary:
	var ahead: int = mini(render_ms - last.server_ms, EXTRAPOLATE_MAX_MS)
	var p: Vector3 = last.p
	if before != null and ahead > 0 and last.server_ms > before.server_ms:
		var velocity: Vector3 = (last.p - before.p) / (float(last.server_ms - before.server_ms) / 1000.0)
		p = last.p + velocity * (float(ahead) / 1000.0)
	return {"p": p, "yaw": last.yaw, "anim": last.anim, "extrapolated": ahead > 0}
