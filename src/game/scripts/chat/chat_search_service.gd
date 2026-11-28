class_name ChatSearchService
extends GameApiService
## F112 (F112-AC-15, AC-16; CHAT_SYSTEM §8.4): "Tìm tin nhắn" of the HUD chat — `searchMessages` over the student's
## clubs and DMs (only channels whose `features.search` is on; never World / Area), the whole set ("Tất cả") or one channel
## (`channel_id`), paged by `cursor` ("Xem thêm"). The query is checked against the contract bounds (2–100 characters);
## outside them nothing is sent (the button is grey). Each hit keeps the public card of the sender (nickname, avatar —
## AGENTS R16), the channel, the time and the excerpt as PLAIN text (HighlightText). The query and the excerpts live
## in RAM only: never logged, stored or sent in telemetry (AGENTS R14); logs carry counts and codes.

signal results_changed()
signal busy_changed(busy: bool)

const PATH_SEARCH: String = "/chat/search"
## `searchMessages` `q` bounds of the contract (minLength / maxLength).
const Q_MIN: int = 2
const Q_MAX: int = 100
const PAGE_SIZE: int = 20

## The hits shown, newest page last: `{message: ChatMessageModel, channel_id, channel_type, excerpt, ranges}`.
var hits: Array[Dictionary] = []
var query: String = ""
var scope_channel_id: String = ""
var has_more: bool = false
var loaded: bool = false
var busy: bool = false
var _cursor: String = ""


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)


## The query can be sent (the contract bounds, trimmed).
static func query_ok(text: String) -> bool:
	var q: String = text.strip_edges()
	return q.length() >= Q_MIN and q.length() <= Q_MAX


## Coroutine: a new search (`channel_id` "" = every searchable channel). False when the query is out of bounds.
func search(text: String, channel_id: String = "") -> ApiResult:
	if not query_ok(text) or busy:
		return ApiResult.failure("VALIDATION_FAILED")
	query = text.strip_edges()
	scope_channel_id = channel_id if Uuid.is_valid(channel_id) else ""
	hits.clear()
	_cursor = ""
	has_more = false
	loaded = false
	return await _page()


## Coroutine: the next page ("Xem thêm").
func more() -> ApiResult:
	if not has_more or _cursor.is_empty() or busy:
		return ApiResult.failure("NO_MORE")
	return await _page()


## The hit of `message_id` left the list (its channel answered `404`, F112-AC-17).
func drop(message_id: String) -> void:
	for i: int in hits.size():
		var m: ChatMessageModel = hits[i]["message"] as ChatMessageModel
		if m != null and m.message_id == message_id:
			hits.remove_at(i)
			results_changed.emit()
			return


func clear() -> void:
	hits.clear()
	query = ""
	scope_channel_id = ""
	has_more = false
	loaded = false
	_cursor = ""
	results_changed.emit()


func _page() -> ApiResult:
	var q: Dictionary = {"q": query, "limit": PAGE_SIZE}
	if not scope_channel_id.is_empty():
		q["channel_id"] = scope_channel_id
	if not _cursor.is_empty():
		q["cursor"] = _cursor
	_set_busy(true)
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_SEARCH, q), "searchMessages")
	_set_busy(false)
	if result.ok:
		for raw: Variant in DataRead.arr_of(result.body(), "data"):
			var hit: Dictionary = hit_of(raw, query)
			if not hit.is_empty():
				hits.append(hit)
		_cursor = GameApiService.next_cursor(result.body())
		has_more = not _cursor.is_empty()
		loaded = true
	Log.info("chat_search", {"ok": result.ok, "code": result.code, "hits": hits.size()})
	results_changed.emit()
	return result


## One `MessageSearchHit` → `{message, channel_id, channel_type, excerpt, ranges}` (empty when unusable). The excerpt
## is `highlight` (else the message body) as plain text; the keyword ranges are the server's when valid, else a match
## of `q` without accents.
static func hit_of(raw: Variant, q: String) -> Dictionary:
	if not raw is Dictionary:
		return {}
	var d: Dictionary = raw as Dictionary
	var m: ChatMessageModel = ChatMessageModel.from_dict(d.get("message"))
	var channel_id: String = DataRead.str_of(d, "channel_id")
	if m == null or not Uuid.is_valid(channel_id):
		return {}
	var source: String = DataRead.str_of(d, "highlight")
	var from_server: bool = not source.is_empty()
	if source.is_empty():
		source = m.body
	var excerpt: String = HighlightText.plain(source)
	var ranges: Array[Vector2i] = []
	if from_server and excerpt == source:
		ranges = HighlightText.server_ranges(excerpt, d.get("highlight_ranges"))
	if ranges.is_empty():
		ranges = HighlightText.match_ranges(excerpt, q)
	return {"message": m, "channel_id": channel_id, "channel_type": DataRead.str_of(d, "channel_type"),
		"excerpt": excerpt, "ranges": ranges}


func _set_busy(value: bool) -> void:
	if busy == value:
		return
	busy = value
	busy_changed.emit(value)
