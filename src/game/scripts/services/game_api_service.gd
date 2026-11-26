class_name GameApiService
extends RefCounted
## Common REST plumbing of the F069 domain services (GODOT_ARCHITECTURE §4, §9): every call goes through ApiClient
## (Bearer + `X-Tenant-Id` of the session, retries of GET / keyed writes with the same Idempotency-Key — AGENTS R10);
## a session-ending refusal (`403 GAME_ACCESS_STUDENT_ONLY`, account/tenant locks) is raised as `access_denied` for the
## F061 flow (ServiceLocator → AuthService: tokens dropped, back to sign-in; `TENANT_MISMATCH` is already handled by
## ApiClient, F069-AC-29); a request the client should never have sent (`428`, `IDEMPOTENCY_KEY_REUSED`,
## `422 VALIDATION_FAILED`) is a `client_bug` telemetry event with the operation id only — no PII (F069-AC-11,
## F069-AC-30, AGENTS R14). A network failure turns `offline` on until the next answer: writes are then locked in the
## UI (F069-AC-28).

signal access_denied(code: String)
signal offline_changed(offline: bool)

const ACCESS_DENIED_CODES: PackedStringArray = [
	"GAME_ACCESS_STUDENT_ONLY", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED", "CURFEW_ACTIVE", "DAILY_PLAY_LIMIT_REACHED",
	"ACCESS_OUTSIDE_ALLOWED_HOURS", "TENANT_ACCESS_DISABLED", "NO_ACTIVE_ENROLLMENT",
]
## Problems that mean this client built a wrong request (never the student's fault).
const CLIENT_BUG_CODES: PackedStringArray = ["PRECONDITION_REQUIRED", "IDEMPOTENCY_KEY_REUSED", "VALIDATION_FAILED"]
const TELEMETRY_CLIENT_BUG: String = "client_bug"
const PAGE_LIMIT: int = 100
## Pages read at most for a whole list (≤ 500 items per student, plan Scale/Scope).
const MAX_PAGES: int = 5

var api: RestApiClient = null
var telemetry: TelemetryService = null
var offline: bool = false
var last_error_code: String = ""


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null) -> void:
	api = rest
	telemetry = telemetry_service


## Coroutine: sends `req` (named `operation_id` in telemetry) and applies the common rules above.
func send_request(req: ApiRequest, operation_id: String) -> ApiResult:
	if api == null:
		_set_offline(true)
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await api.send(req)
	_set_offline(result.network_error)
	last_error_code = "" if result.ok else result.code
	if result.status == 403 and result.code in ACCESS_DENIED_CODES:
		access_denied.emit(result.code)
	if result.status == 428 or result.code in CLIENT_BUG_CODES:
		report_client_bug(operation_id, result)
	return result


## Coroutine: every page of a cursor list (`data` + `page.next_cursor`), up to MAX_PAGES. `ok` false keeps `items` of
## the pages read so far; the failing result is returned in `result`.
func send_all_pages(path: String, query: Dictionary, operation_id: String) -> Dictionary:
	var items: Array = []
	var cursor: String = ""
	var last: ApiResult = null
	for page: int in MAX_PAGES:
		var q: Dictionary = query.duplicate()
		q["limit"] = PAGE_LIMIT
		if not cursor.is_empty():
			q["cursor"] = cursor
		last = await send_request(ApiRequest.make_get(path, q), operation_id)
		if not last.ok:
			return {"ok": false, "items": items, "result": last}
		items.append_array(DataRead.arr_of(last.body(), "data"))
		cursor = next_cursor(last.body())
		if cursor.is_empty():
			break
	return {"ok": true, "items": items, "result": last}


## `page.next_cursor` when `page.has_more`, else "".
static func next_cursor(body: Dictionary) -> String:
	var page: Dictionary = DataRead.dict_of(body, "page")
	if not DataRead.bool_of(page, "has_more"):
		return ""
	return DataRead.str_of(page, "next_cursor")


func report_client_bug(operation_id: String, result: ApiResult) -> void:
	if telemetry != null:
		telemetry.queue_client_error(TELEMETRY_CLIENT_BUG, operation_id)
	Log.info("api_client_bug", {"operation_id": operation_id, "status": result.status, "code": result.code,
		"trace_id": result.trace_id})


func _set_offline(value: bool) -> void:
	if offline == value:
		return
	offline = value
	offline_changed.emit(value)
