class_name PositionCorrection
extends RefCounted
## Server correction of the local player (`hub.player.corrected`, ADR-0011; F092-AC-14): the offset between the
## position the server accepted and the local one is applied over BLEND_SEC (200 ms) instead of a teleport. The local
## movement keeps running on top (client-side movement, no prediction/rollback).

## GAME_NETWORKING §5.1: blend back within 200 ms.
const BLEND_SEC: float = 0.2

var remaining: Vector3 = Vector3.ZERO
var time_left: float = 0.0


func start(current: Vector3, target: Vector3) -> void:
	remaining = target - current
	time_left = BLEND_SEC


func is_active() -> bool:
	return time_left > 0.0


## Displacement to add this frame; the sum over BLEND_SEC equals the full offset.
func step(delta: float) -> Vector3:
	if time_left <= 0.0:
		return Vector3.ZERO
	var used: float = minf(delta, time_left)
	var part: Vector3 = remaining * (used / time_left)
	remaining -= part
	time_left -= used
	if time_left <= 0.0:
		part += remaining
		remaining = Vector3.ZERO
	return part
