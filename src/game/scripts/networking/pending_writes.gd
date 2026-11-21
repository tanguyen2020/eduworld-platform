class_name PendingWrites
extends RefCounted
## Writes that could not reach the server (GODOT_ARCHITECTURE §9, GAME_NETWORKING §2.3, F063-AC-06): kept in RAM
## only — never on disk, so no render payload or answer is ever written to `user://` — and re-sent in arrival order
## with their original `Idempotency-Key` once the network is back (the server replays an already-applied write).
## A write is dropped once the server answered it (2xx or a definitive 4xx); a network error / 5xx keeps it.
## F064: an exam answer is a `PUT` that overwrites the answer of its position — idempotent by method, so it is queued
## without a key and only its latest version per position is kept (`enqueue_latest`, F064-AC-14); extra headers
## (`X-Exam-Device-Token`) are replayed with the write.

## Bounded queue: an attempt has at most 200 items (`Attempt.items maxItems`).
const MAX_SIZE: int = 200

var _queue: Array[PendingWrite] = []


class PendingWrite:
	extends RefCounted
	var method: String = "POST"
	var path: String = ""
	var body: Variant = null
	var idempotency_key: String = ""
	var enqueued_at_msec: int = 0
	## Caller data returned with the result (e.g. the item position).
	var tag: Dictionary = {}
	## Extra request headers replayed with the write (never logged).
	var headers: Dictionary = {}
	## Writes with the same non-empty merge key replace each other (latest wins).
	var merge_key: String = ""

	func to_request() -> ApiRequest:
		var req: ApiRequest = ApiRequest.make_post(path, body)
		req.method = method
		req.idempotency_key = idempotency_key
		req.headers = headers.duplicate()
		return req


## Queues a keyed write (unkeyed writes are never replayed, GAME_NETWORKING §2.3). Returns false when refused.
func enqueue(req: ApiRequest, now_msec: int, tag: Dictionary = {}) -> bool:
	if req.idempotency_key.is_empty() or _queue.size() >= MAX_SIZE:
		return false
	for w: PendingWrite in _queue:
		if w.idempotency_key == req.idempotency_key:
			return true
	var write: PendingWrite = PendingWrite.new()
	write.method = req.method
	write.path = req.path
	write.body = req.body
	write.idempotency_key = req.idempotency_key
	write.enqueued_at_msec = now_msec
	write.tag = tag
	write.headers = req.headers.duplicate()
	_queue.append(write)
	return true


## Queues an idempotent `PUT` (no key needed) and drops the queued write with the same `merge_key`: only the latest
## version is replayed, in the order of the latest changes. Returns false when refused (not a PUT, no merge key, full).
func enqueue_latest(req: ApiRequest, now_msec: int, merge_key: String, tag: Dictionary = {}) -> bool:
	if req.method != "PUT" or merge_key.is_empty():
		return false
	for i: int in range(_queue.size() - 1, -1, -1):
		if _queue[i].merge_key == merge_key:
			_queue.remove_at(i)
	if _queue.size() >= MAX_SIZE:
		return false
	var write: PendingWrite = PendingWrite.new()
	write.method = req.method
	write.path = req.path
	write.body = req.body
	write.idempotency_key = req.idempotency_key
	write.enqueued_at_msec = now_msec
	write.tag = tag
	write.headers = req.headers.duplicate()
	write.merge_key = merge_key
	_queue.append(write)
	return true


func size() -> int:
	return _queue.size()


func is_empty() -> bool:
	return _queue.is_empty()


## Idempotency keys of the queued writes, in sending order.
func keys() -> PackedStringArray:
	var out: PackedStringArray = []
	for w: PendingWrite in _queue:
		out.append(w.idempotency_key)
	return out


func peek() -> PendingWrite:
	return _queue[0] if not _queue.is_empty() else null


## Removes the first write (answered by the server).
func drop_first() -> void:
	if not _queue.is_empty():
		_queue.pop_front()


## Coroutine: re-sends the queue in order through `api`; stops at the first write that still fails transiently.
## Returns `[{write: PendingWrite, result: ApiResult}]` for every write the server answered (and dequeued).
func flush(api: RestApiClient) -> Array[Dictionary]:
	var done: Array[Dictionary] = []
	while not _queue.is_empty():
		var write: PendingWrite = _queue[0]
		var result: ApiResult = await api.send(write.to_request())
		if result.network_error or result.status >= 500 or result.status == 429:
			break
		_queue.pop_front()
		done.append({"write": write, "result": result})
	return done


## Drops everything (attempt closed, session ended).
func clear() -> void:
	_queue.clear()
