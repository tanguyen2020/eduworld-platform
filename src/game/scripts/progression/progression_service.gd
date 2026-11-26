class_name ProgressionService
extends GameApiService
## Progression of the signed-in student (F069 US1, PROGRESSION_SYSTEM §4.5–§4.6, GODOT_ARCHITECTURE §4 service →
## store → UI): `getProgression` on entering the world, `listXpHistory` (cursor, 20 lines) and `listProgressionLevels`
## for "Hành trình của tôi", and the realtime messages `progression.xp.updated` (values replaced, no polling,
## F069-AC-02) and `progression.level.reached` (queued for S07, F069-AC-03). The client never sends XP, levels or
## results (F069-AC-06, ADR-0016) and holds no curve or title list (FR-GAM-007).

## A level was reached (S07 queued); the wallet owner refreshes `getWallet` (Clarifications 2026-11-16).
signal level_reached(to_level: int)

const PATH: String = "/game/progression"
const PATH_HISTORY: String = "/game/progression/xp-history"
const PATH_LEVELS: String = "/game/progression/levels"
const PREFIX: String = "progression"
const MSG_XP_UPDATED: String = "progression.xp.updated"
const MSG_LEVEL_REACHED: String = "progression.level.reached"
## "Hành trình của tôi" page size (F069-AC-05).
const HISTORY_PAGE: int = 20

var store: ProgressionStore = ProgressionStore.new()
var queue: LevelUpQueue = null
var levels: Array[LevelModel] = []
var realtime: GameRealtimeClient = null
var _loading: bool = false


func _init(rest: RestApiClient = null, app_state: Object = null, realtime_client: GameRealtimeClient = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	queue = LevelUpQueue.new(app_state)
	realtime = realtime_client
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


func progression() -> ProgressionModel:
	return store.progression


## Coroutine: `getProgression` (F069-AC-01). Keeps the previous value on failure.
func refresh() -> ApiResult:
	_loading = true
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH), "getProgression")
	_loading = false
	if result.ok:
		var parsed: ProgressionModel = ProgressionModel.from_dict(result.data)
		if parsed == null:
			return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
		store.set_status(parsed)
	return result


## Coroutine: one page of `listXpHistory` → `{ok, entries: Array[XpEntryModel], next_cursor, code}`.
func load_history(cursor: String = "") -> Dictionary:
	var query: Dictionary = {"limit": HISTORY_PAGE}
	if not cursor.is_empty():
		query["cursor"] = cursor
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_HISTORY, query), "listXpHistory")
	if not result.ok:
		return {"ok": false, "entries": [] as Array[XpEntryModel], "next_cursor": "", "code": result.code}
	return {"ok": true, "entries": XpEntryModel.list_from(result.body().get("data")),
		"next_cursor": next_cursor(result.body()), "code": ""}


## Coroutine: `listProgressionLevels` (milestones of the curve in force, display only).
func load_levels() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_LEVELS), "listProgressionLevels")
	if result.ok:
		levels = LevelModel.list_from(result.body().get("levels"))
	return result


## Realtime dispatcher handler for `progression.*` (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	match type:
		MSG_XP_UPDATED:
			if store.apply_xp(payload) and not _loading:
				refresh()
		MSG_LEVEL_REACHED:
			var level_up: LevelUpModel = LevelUpModel.from_dict(payload)
			if level_up == null:
				return
			queue.push(level_up)
			level_reached.emit(level_up.to_level)
			if not _loading:
				refresh()


## Session ended (logout, takeover): drops everything of this student (F069-AC-28).
func clear() -> void:
	store.clear()
	queue.clear()
	levels.clear()
