class_name KeyedWriteService
extends GameApiService
## F111: the common write plumbing of ClubService, LiveEventService and SeasonTrackService (AGENTS R10, ADR-0016): one
## Idempotency-Key (UUIDv7) per student INTENT (`intent` text, e.g. "claim:<event>:<milestone>") kept while the request
## is retried after a network failure or `503` (same key → the server does it once), dropped once the server answered;
## a busy counter for the buttons (no double tap); a problem becomes a friendly sentence `ui.error.<CODE>` (never the raw
## code) through `notice_requested`.

## A friendly sentence for the screen (`key` + format args), never a raw code.
signal notice_requested(key: String, args: Dictionary)
## A write started / finished (buttons show the busy state, no double tap).
signal busy_changed(busy: bool)

const KEY_GENERIC: String = "ui.error.generic"

var _intent_keys: Dictionary = {}
var _busy: int = 0


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)


func is_busy() -> bool:
	return _busy > 0


func is_offline() -> bool:
	return offline


## Coroutine: one REST write with the key of `intent` ("" = no key).
func send_keyed(req: ApiRequest, operation_id: String, intent: String) -> ApiResult:
	if not intent.is_empty():
		if not _intent_keys.has(intent):
			_intent_keys[intent] = Uuid.v7()
		req.idempotency_key = String(_intent_keys[intent])
	_set_busy(1)
	var result: ApiResult = await send_request(req, operation_id)
	_set_busy(-1)
	if not intent.is_empty() and not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	return result


## The friendly sentence of a problem code (`ui.error.<CODE>` when the client has it, else the generic one).
static func error_key(code: String) -> String:
	var key: String = "ui.error.%s" % code
	return key if not code.is_empty() and L10n.has_key(key) else KEY_GENERIC


## Says the problem of `result` (not for the session-ending refusals, which go to F061).
func notice_for(result: ApiResult) -> void:
	if result.ok or result.code in ACCESS_DENIED_CODES:
		return
	notice_requested.emit("ui.friends.offline" if result.network_error else error_key(result.code), {})


func forget_keys() -> void:
	_intent_keys.clear()


func _set_busy(delta: int) -> void:
	var before: bool = _busy > 0
	_busy = maxi(0, _busy + delta)
	if before != (_busy > 0):
		busy_changed.emit(_busy > 0)
