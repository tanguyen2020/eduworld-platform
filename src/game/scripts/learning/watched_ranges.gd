class_name WatchedRanges
extends RefCounted
## Watched parts of a video on the client (F065-AC-17, AC-19): merged ranges sent in every heartbeat (cumulative, so a
## lost heartbeat loses nothing) and the seek lock of `allow_seek`. The server decides what counts (plausibility,
## VIDEO_LEARNING §4); the client only mirrors the accepted ranges it receives back.

const SEEK_FREE: String = "free"
const SEEK_BACKWARD_ONLY: String = "backward_only"
const SEEK_WATCHED_ONLY: String = "watched_only"
## ActivityProgressReport.watched_ranges maxItems.
const MAX_RANGES: int = 64

## [start_ms, end_ms] pairs, sorted and merged.
var ranges: Array[Vector2i] = []
var duration_ms: int = 0


func clear() -> void:
	ranges.clear()


## Replaces the ranges with the server's accepted ones (`[{start_ms, end_ms}]`).
func load_from(items: Variant) -> void:
	ranges.clear()
	if items is Array:
		for item: Variant in (items as Array):
			if item is Dictionary:
				add(int((item as Dictionary).get("start_ms", 0)), int((item as Dictionary).get("end_ms", 0)))


## Adds a watched span and merges.
func add(start_ms: int, end_ms: int) -> void:
	var s: int = maxi(0, start_ms)
	var e: int = end_ms if duration_ms <= 0 else mini(end_ms, duration_ms)
	if e <= s:
		return
	ranges.append(Vector2i(s, e))
	ranges.sort_custom(func(a: Vector2i, b: Vector2i) -> bool: return a.x < b.x)
	var merged: Array[Vector2i] = []
	for r: Vector2i in ranges:
		if not merged.is_empty() and r.x <= merged[merged.size() - 1].y:
			var last: Vector2i = merged[merged.size() - 1]
			merged[merged.size() - 1] = Vector2i(last.x, maxi(last.y, r.y))
		else:
			merged.append(r)
	while merged.size() > MAX_RANGES:
		var best: int = 1
		for i: int in range(2, merged.size()):
			if merged[i].x - merged[i - 1].y < merged[best].x - merged[best - 1].y:
				best = i
		merged[best - 1] = Vector2i(merged[best - 1].x, merged[best].y)
		merged.remove_at(best)
	ranges = merged


## Furthest watched position.
func frontier() -> int:
	var f: int = 0
	for r: Vector2i in ranges:
		f = maxi(f, r.y)
	return f


func watched_ms() -> int:
	var n: int = 0
	for r: Vector2i in ranges:
		n += r.y - r.x
	return n


## The position a seek may reach: past the frontier is refused for watched_only/backward_only until completed.
func clamp_seek(target_ms: int, policy: String, completed: bool) -> int:
	if completed or policy == SEEK_FREE:
		return maxi(0, target_ms)
	return clampi(target_ms, 0, frontier())


func is_locked_beyond(target_ms: int, policy: String, completed: bool) -> bool:
	return clamp_seek(target_ms, policy, completed) != maxi(0, target_ms)


## `watched_ranges` of ActivityProgressReport.
func to_payload() -> Array:
	var out: Array = []
	for r: Vector2i in ranges:
		out.append({"start_ms": r.x, "end_ms": r.y})
	return out
