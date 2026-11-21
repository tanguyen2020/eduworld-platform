class_name WorldService
extends RefCounted
## World of the signed-in student (F071, GODOT_ARCHITECTURE §4 service → state → UI): the areas the server lets this
## student see (`listWorldAreas`), the exploration and chapters (`getWorldExploration`), entering an area
## (`enterWorldArea`), level sessions (`startLevel` / `endLevelSession`), discoveries (`reportDiscovery`) and the NPC
## dialogues (`getNpcDialogue`, cached by ETag). Every decision (unlock, discovery, dialogue action) is the server's: the
## service only sends intents and keeps what came back (ADR-0016, AGENTS R1, R5). `world.area.unlocked` from the realtime
## gateway refreshes the areas. Nothing of the world is hard-coded (R2): names, portals and positions are data.

signal areas_updated(count: int)
signal exploration_updated()
signal area_unlocked(area_id: String, area_name: Dictionary)
signal discovery_recorded(area_id: String, interactable_id: String, completion_pct: float)
signal access_denied(code: String)

const PATH_AREAS: String = "/game/world/areas"
const PATH_MANIFEST: String = "/game/world/manifest"
const PATH_EXPLORATION: String = "/game/world/exploration"
const MSG_AREA_UNLOCKED: String = "world.area.unlocked"
const ACCESS_DENIED_CODES: PackedStringArray = ["GAME_ACCESS_STUDENT_ONLY", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED",
	"CURFEW_ACTIVE", "DAILY_PLAY_LIMIT_REACHED"]
## Problem codes of a refused area or discovery (shown as a friendly notice, never as raw codes).
const CODE_AREA_LOCKED: String = "AREA_LOCKED"
const CODE_ALREADY_FOUND: String = "DISCOVERY_ALREADY_FOUND"

var api: RestApiClient = null
var store: Node = null
var areas: Array[WorldAreaModel] = []
## `getWorldExploration` as returned: `areas[]` (AreaExploration) and `chapters[]` (StoryChapterProgress).
var exploration: Dictionary = {}
## Active level session (`LevelSession`), empty when none.
var level_session: Dictionary = {}
var _dialogues: Dictionary = {} # npc_id → {etag, body}


func _init(rest: RestApiClient = null, app_state: Node = null, realtime: GameRealtimeClient = null) -> void:
	api = rest
	store = app_state
	if realtime != null:
		realtime.message_received.connect(_on_message)


## Area by id (null when the server did not list it: hidden subject or unknown).
func area(area_id: String) -> WorldAreaModel:
	for a: WorldAreaModel in areas:
		if a.area_id == area_id:
			return a
	return null


func fetch_areas() -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_AREAS))
	if result.ok:
		var parsed: Array[WorldAreaModel] = []
		var data: Variant = result.body().get("data")
		if data is Array:
			for d: Variant in data as Array:
				var a: WorldAreaModel = WorldAreaModel.from_dict(d)
				if a != null:
					parsed.append(a)
		areas = parsed
		_publish()
		areas_updated.emit(areas.size())
	return result


func fetch_manifest() -> ApiResult:
	return await _send(ApiRequest.make_get(PATH_MANIFEST))


func fetch_exploration() -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_EXPLORATION))
	if result.ok:
		exploration = result.body()
		_publish()
		exploration_updated.emit()
	return result


## AreaExploration of `area_id` (empty when unknown).
func exploration_of(area_id: String) -> Dictionary:
	for a: Variant in exploration.get("areas", []) as Array:
		if a is Dictionary and (a as Dictionary).get("area_id") == area_id:
			return a as Dictionary
	return {}


## `enterWorldArea` for the game session of this device; `portal_id` is the wind gate walked through (the server picks
## the arrival spawn). 403 AREA_LOCKED carries `missing_requirements` in the problem body.
func enter_area(area_id: String, portal_id: String = "") -> ApiResult:
	var body: Dictionary = {"game_session_id": _session_id()}
	if not portal_id.is_empty():
		body["portal_id"] = portal_id
	return await _send(ApiRequest.make_post("/game/world/areas/%s:enter" % area_id.uri_encode(), body))


## Requirements still missing from an AREA_LOCKED refusal.
static func missing_requirements(result: ApiResult) -> Array[Dictionary]:
	return WorldAreaModel.requirements_from(result.body().get("missing_requirements"))


## `startLevel` (Idempotency-Key kept on retry): the level session pins the content versions (ADR-0043).
func start_level(level_id: String, manifest_version: int = 0) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post("/game/levels/%s:start" % level_id,
			{"game_session_id": _session_id(), "manifest_version": manifest_version})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok:
		level_session = result.body()
	return result


## `endLevelSession` of the active level session (`completed` or `abandoned`).
func end_level(outcome: String = "completed") -> ApiResult:
	var id: String = String(level_session.get("level_session_id", ""))
	if id.is_empty():
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var req: ApiRequest = ApiRequest.make_post("/game/level-sessions/%s:end" % id, {"outcome": outcome})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok or result.status == 409 or result.status == 404:
		level_session = {}
	return result


## `reportDiscovery`: the server checks the presence, the pinned layout and the distance; the client shows the result.
func report_discovery(area_id: String, interactable_id: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post("/game/world/areas/%s/discoveries/%s:discover" % [area_id.uri_encode(),
			interactable_id.uri_encode()])
	req.idempotency_key = Uuid.v7()
	req.max_rate_limit_retries = 0 # 429 discovery_report_rate is shown, not waited on
	var result: ApiResult = await _send(req)
	if result.ok:
		var pct: float = float(result.body().get("area_completion_pct", 0.0))
		discovery_recorded.emit(area_id, interactable_id, pct)
	return result


## `getNpcDialogue` with If-None-Match: a 304 keeps the cached graph of the same version.
func fetch_dialogue(npc_id: String) -> ApiResult:
	var cached: Dictionary = _dialogues.get(npc_id, {}) as Dictionary
	var req: ApiRequest = ApiRequest.make_get("/game/world/npcs/%s/dialogue" % npc_id.uri_encode())
	if not cached.is_empty():
		req.headers["If-None-Match"] = String(cached.get("etag", ""))
	var result: ApiResult = await _send(req)
	if result.status == 304 and not cached.is_empty():
		result.ok = true
		result.data = cached.get("body")
	elif result.ok:
		_dialogues[npc_id] = {"etag": String(result.headers.get("etag", "")), "body": result.body()}
	return result


func clear() -> void:
	areas.clear()
	exploration = {}
	level_session = {}
	_dialogues.clear()
	_publish()


func _session_id() -> String:
	return String(store.get("game_session_id")) if store != null else ""


func _publish() -> void:
	if store != null and store.has_method("set_world"):
		store.call("set_world", {"area_count": areas.size(), "exploration": exploration})


func _on_message(type: String, payload: Dictionary) -> void:
	if type != MSG_AREA_UNLOCKED:
		return
	var id: String = String(payload.get("area_id", ""))
	var name: Dictionary = payload.get("name", {}) as Dictionary if payload.get("name") is Dictionary else {}
	area_unlocked.emit(id, name)
	fetch_areas()


func _send(req: ApiRequest) -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await api.send(req)
	if result.status == 403 and result.code in ACCESS_DENIED_CODES:
		access_denied.emit(result.code)
	return result
