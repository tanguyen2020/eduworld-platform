class_name SnapshotBuffer
extends RefCounted
## Snapshots of the other players by entity id (GAME_NETWORKING §5.2, F092-AC-12): samples ordered by server time,
## kept for KEEP_MS (1 s, at least two per entity) so the Interpolator always finds the pair around `render_time`.
## A delta sample inherits the fields it does not carry from the previous one.

const KEEP_MS: int = 1000
const MIN_SAMPLES: int = 2

## eid → Array[SnapshotSample] (ascending server_ms).
var _by_eid: Dictionary = {}


func push(eid: int, sample: SnapshotSample) -> void:
	var list: Array = _by_eid.get(eid, []) as Array
	if not list.is_empty():
		var last: SnapshotSample = list[list.size() - 1] as SnapshotSample
		if sample.server_ms < last.server_ms or (sample.server_ms == last.server_ms and sample.tick <= last.tick):
			return
		sample.complete_from(last)
	list.append(sample)
	while list.size() > MIN_SAMPLES and sample.server_ms - (list[0] as SnapshotSample).server_ms > KEEP_MS:
		list.pop_front()
	_by_eid[eid] = list


func samples(eid: int) -> Array:
	return _by_eid.get(eid, []) as Array


func latest(eid: int) -> SnapshotSample:
	var list: Array = samples(eid)
	return list[list.size() - 1] as SnapshotSample if not list.is_empty() else null


func has(eid: int) -> bool:
	return _by_eid.has(eid)


func remove(eid: int) -> void:
	_by_eid.erase(eid)


func clear() -> void:
	_by_eid.clear()


func size() -> int:
	return _by_eid.size()
