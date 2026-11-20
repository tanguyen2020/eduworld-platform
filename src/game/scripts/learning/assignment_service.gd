class_name AssignmentService
extends RefCounted
## "Nhà trường giao" (FR-LMS-006, STUDENT_GAME_UX_CORE S03/S05, F062 US2). Reads the student's own assignments with
## `listMyAssignments` (`GET /learning/my-assignments`) through ApiClient — the session `X-Tenant-Id`, the 401 →
## single-flight refresh → replay and the `TENANT_MISMATCH` logout are ApiClient/AuthService behaviour (F061,
## ADR-0065). Keeps the list in RAM (`AppState.assignments`), never on disk (GODOT_ARCHITECTURE §8.2). Refreshed on
## entering/returning to the Hub and when the quest journal opens; no polling (spec Clarifications).
## `start()` turns "Bắt đầu" into a UI intent (`EventBus.ui_requested`); it never decides whether the work is open —
## the server answers that (e.g. `ASSIGNMENT_NOT_OPEN`, ADR-0016).

signal assignments_updated(count: int)
signal state_changed(state: int)
## An account/tenant refusal (`403`) that ends the session (F062-AC-12, F062-AC-23).
signal access_denied(code: String)
signal refresh_finished(ok: bool)

enum State { IDLE, LOADING, READY, ERROR }

const PATH: String = "/learning/my-assignments"
## `Limit` parameter (API_DESIGN pagination); pages are followed up to MAX_PAGES.
const PAGE_LIMIT: int = 50
const MAX_PAGES: int = 10
const SCREEN_ACTIVITY_HOST: StringName = &"activity_host"
const SCREEN_EXAM_LOBBY: StringName = &"exam_lobby"
const TELEMETRY_UNSUPPORTED_KIND: String = "client_unsupported_assignment_kind"
const SCENE_CODE: String = "quest_journal"
## Session-ending refusals handled here; `TENANT_MISMATCH` is already handled by ApiClient → AuthService.
const ACCESS_DENIED_CODES: PackedStringArray = [
	"GAME_ACCESS_STUDENT_ONLY", "ACCESS_OUTSIDE_ALLOWED_HOURS", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED",
	"NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED", "ACCOUNT_NOT_PROVISIONED",
]

var api: RestApiClient = null
## Store mirrored on every update (`AppState`, or null in unit tests).
var store: Node = null
var telemetry: TelemetryService = null
## `(screen_id: StringName, args: Dictionary) -> void`; default emits `EventBus.ui_requested`.
var intent_sink: Callable = Callable()
var state: State = State.IDLE
var assignments: Array[MyAssignmentModel] = []
## Error code of the last failed refresh (`NETWORK_ERROR`, `HTTP_503`, Problem code…), empty when none.
var last_error_code: String = ""
var _refreshing: bool = false


func _init(rest: RestApiClient = null, app_state: Node = null, telemetry_service: TelemetryService = null) -> void:
	api = rest
	store = app_state
	telemetry = telemetry_service


## Coroutine: reloads every page of `listMyAssignments` (single-flight). On a network/5xx error the previous list is
## kept and `state` is ERROR (the journal shows "Thử lại", F062-AC-12). Returns true on success.
func refresh() -> bool:
	if _refreshing:
		await refresh_finished
		return state == State.READY
	if api == null:
		return false
	_refreshing = true
	_set_state(State.LOADING)
	var collected: Array[MyAssignmentModel] = []
	var cursor: String = ""
	var ok: bool = true
	for page: int in MAX_PAGES:
		var query: Dictionary = {"limit": PAGE_LIMIT}
		if not cursor.is_empty():
			query["cursor"] = cursor
		var result: ApiResult = await api.send(ApiRequest.make_get(PATH, query))
		if not result.ok or not result.body().get("data") is Array:
			ok = false
			_on_failure(result)
			break
		collected.append_array(MyAssignmentModel.list_from(result.body()["data"] as Array))
		var info: Variant = result.body().get("page")
		var next: Variant = (info as Dictionary).get("next_cursor") if info is Dictionary else null
		if not (info is Dictionary and bool((info as Dictionary).get("has_more", false)) and next is String):
			break
		cursor = next as String
	if ok:
		last_error_code = ""
		assignments = collected
		_publish()
		_set_state(State.READY)
	_refreshing = false
	refresh_finished.emit(ok)
	return ok


## F062-AC-09/10: "Bắt đầu". `activity` → `activity_host`, `assessment` → `exam_lobby`; any other kind has no
## screen in this client → returns empty (the UI shows `ui.activity_player.update_required`) and queues the
## `client_unsupported_assignment_kind` telemetry. Returns the requested screen id.
func start(assignment: MyAssignmentModel) -> StringName:
	var screen: StringName = &""
	var args: Dictionary = {}
	match assignment.content_kind:
		MyAssignmentModel.KIND_ACTIVITY:
			screen = SCREEN_ACTIVITY_HOST
			args = {
				"activity_id": assignment.content_ref, "mode": "assignment", "context_type": "assignment",
				"context_id": assignment.assignment_id,
			}
		MyAssignmentModel.KIND_ASSESSMENT:
			screen = SCREEN_EXAM_LOBBY
			args = {"exam_session_id": assignment.exam_session_id, "assignment_id": assignment.assignment_id}
	if screen.is_empty():
		if telemetry != null:
			telemetry.queue_client_error(TELEMETRY_UNSUPPORTED_KIND, SCENE_CODE)
		return &""
	if intent_sink.is_valid():
		intent_sink.call(screen, args)
	else:
		var bus: Node = _autoload("EventBus")
		if bus != null:
			bus.emit_signal("ui_requested", screen, args)
	return screen


func count() -> int:
	return assignments.size()


## Drops the list (logout / session end).
func clear() -> void:
	assignments.clear()
	last_error_code = ""
	_publish()
	_set_state(State.IDLE)


func _on_failure(result: ApiResult) -> void:
	last_error_code = result.code if not result.code.is_empty() else ApiResult.CODE_INVALID_RESPONSE
	if result.status == 403 and result.code in ACCESS_DENIED_CODES:
		assignments.clear()
		_publish()
		_set_state(State.ERROR)
		access_denied.emit(result.code)
		return
	_set_state(State.ERROR)


func _publish() -> void:
	if store != null:
		store.call("set_assignments", assignments)
	assignments_updated.emit(assignments.size())


func _set_state(next: State) -> void:
	state = next
	state_changed.emit(next)


static func _autoload(autoload_name: String) -> Node:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null(autoload_name) if tree != null else null
