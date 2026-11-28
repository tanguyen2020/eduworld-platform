class_name QuestService
extends RefCounted
## Quests of the signed-in student (F072, QUEST_SYSTEM §8, GODOT_ARCHITECTURE §4 service → state → UI): the list the
## server computes for the grade and tenant of the student (`listQuests`), the delivery of a quest (`getQuest`),
## accepting and abandoning (`acceptQuest`, `abandonQuest`), reporting an interaction step (`reportQuestStep`) and
## starting a learning step (`startQuestActivity` → activity player). The client never decides that a step is done: it
## reports what the player touched and shows what the server answers or pushes (`quest.progress.updated`, ADR-0016,
## AGENTS R1, R5). Every write carries an Idempotency-Key. Nothing of the quests is hard-coded (R2).

signal quests_updated(count: int)
signal progress_updated(quest_id: String)
signal quest_completed(quest_id: String)
signal tracked_changed(quest_id: String)
signal notice_requested(key: String)
signal access_denied(code: String)

const PATH: String = "/game/quests"
const PAGE_LIMIT: int = 50
const MAX_PAGES: int = 4
const MSG_PROGRESS: String = "quest.progress.updated"
const SCREEN_ACTIVITY_HOST: StringName = &"activity_host"
const ACCESS_DENIED_CODES: PackedStringArray = ["GAME_ACCESS_STUDENT_ONLY", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED",
	"CURFEW_ACTIVE", "DAILY_PLAY_LIMIT_REACHED"]
## Friendly notices of the refusals the player can meet (never the raw codes).
const NOTICES: Dictionary = {
	"QUEST_PREREQUISITE_NOT_MET": "ui.quest.notice.prerequisite", "QUEST_ACTIVE_LIMIT": "ui.quest.notice.active_limit",
	"QUEST_ALREADY_COMPLETED": "ui.quest.notice.already_completed", "QUEST_COOLDOWN": "ui.quest.notice.cooldown",
	"QUEST_STEP_ATTEMPT_LIMIT": "ui.quest.notice.come_back_tomorrow", "STEP_PRECONDITION_FAILED": "ui.quest.notice.not_here",
	"UNAVAILABLE": "ui.quest.notice.unavailable",
}

var api: RestApiClient = null
## Store holding `game_session_id` (AppState, or a fake in unit tests).
var store: Node = null
var quests: Array[QuestModel] = []
var tracked_quest_id: String = ""
## `(screen_id: StringName, args: Dictionary) -> void`; default emits `EventBus.ui_requested`.
var intent_sink: Callable = Callable()


func _init(rest: RestApiClient = null, app_state: Node = null, realtime: GameRealtimeClient = null) -> void:
	api = rest
	store = app_state
	if realtime != null:
		realtime.message_received.connect(_on_message)


func quest(quest_id: String) -> QuestModel:
	for q: QuestModel in quests:
		if q.quest_id == quest_id:
			return q
	return null


func active() -> Array[QuestModel]:
	var out: Array[QuestModel] = []
	for q: QuestModel in quests:
		if q.is_active():
			out.append(q)
	return out


## Coroutine: reloads the list (every page) and the delivery of the active quests (their steps).
func refresh() -> ApiResult:
	var collected: Array[QuestModel] = []
	var cursor: String = ""
	var last: ApiResult = null
	for page: int in MAX_PAGES:
		var query: Dictionary = {"limit": PAGE_LIMIT}
		if not cursor.is_empty():
			query["cursor"] = cursor
		last = await _send(ApiRequest.make_get(PATH, query))
		if not last.ok or not last.body().get("data") is Array:
			return last
		for d: Variant in last.body()["data"] as Array:
			var q: QuestModel = QuestModel.from_dict(d)
			if q != null:
				var old: QuestModel = quest(q.quest_id)
				if old != null and q.steps.is_empty():
					q.steps = old.steps
				collected.append(q)
		var info: Variant = last.body().get("page")
		var next: Variant = (info as Dictionary).get("next_cursor") if info is Dictionary else null
		if not (info is Dictionary and bool((info as Dictionary).get("has_more", false)) and next is String):
			break
		cursor = next as String
	quests = collected
	for q: QuestModel in active():
		if q.steps.is_empty():
			await fetch_detail(q.quest_id)
	if tracked_quest_id.is_empty() or quest(tracked_quest_id) == null or not quest(tracked_quest_id).is_active():
		var act: Array[QuestModel] = active()
		track(act[0].quest_id if not act.is_empty() else "")
	quests_updated.emit(quests.size())
	return last


## F111-AC-15 (Event center "Hôm nay" / "Tuần này"): `listQuests` with `quest_type` (daily / weekly), merged into the
## list (a known quest keeps its steps). The progress shown is the server's `QuestProgress`; nothing is counted here.
func load_type(quest_type: String) -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH, {"quest_type": quest_type, "limit": PAGE_LIMIT}))
	if not result.ok or not result.body().get("data") is Array:
		return result
	for d: Variant in result.body()["data"] as Array:
		var q: QuestModel = QuestModel.from_dict(d)
		if q == null:
			continue
		var old: QuestModel = quest(q.quest_id)
		if old == null:
			quests.append(q)
		else:
			old.apply(d as Dictionary)
	quests_updated.emit(quests.size())
	return result


## The quests of `quest_type` the list holds (server order).
func of_type(quest_type: String) -> Array[QuestModel]:
	var out: Array[QuestModel] = []
	for q: QuestModel in quests:
		if q.quest_type == quest_type:
			out.append(q)
	return out


## `getQuest`: merges the delivery (steps, texts, progress) into the list.
func fetch_detail(quest_id: String) -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get("%s/%s" % [PATH, quest_id.uri_encode()]))
	if result.ok:
		_merge(result.body())
	return result


## `acceptQuest` (Idempotency-Key): the server checks grade, assignment, prerequisites and limits.
func accept(quest_id: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post("%s/%s:accept" % [PATH, quest_id.uri_encode()], {})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok:
		_merge_progress(quest_id, result.body())
		await fetch_detail(quest_id)
		track(quest_id)
	else:
		_notice(result)
	return result


## `abandonQuest` (Idempotency-Key): attempts and learning records stay.
func abandon(quest_id: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post("%s/%s:abandon" % [PATH, quest_id.uri_encode()], {})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok:
		_merge_progress(quest_id, result.body())
		if tracked_quest_id == quest_id:
			track("")
	return result


func track(quest_id: String) -> void:
	if quest_id == tracked_quest_id:
		return
	tracked_quest_id = quest_id
	tracked_changed.emit(quest_id)


## `reportQuestStep` of an interaction step (talk_npc / go_to / collect) with the game session of the device.
func report_step(quest_id: String, step_id: String, area_id: String, interactable_id: String = "", npc_id: String = "") -> ApiResult:
	var body: Dictionary = {"game_session_id": _session_id(), "area_id": area_id}
	if not interactable_id.is_empty():
		body["interactable_id"] = interactable_id
	if not npc_id.is_empty():
		body["npc_id"] = npc_id
	var req: ApiRequest = ApiRequest.make_post("%s/%s/steps/%s:report" % [PATH, quest_id.uri_encode(), step_id.uri_encode()], body)
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok:
		_merge_progress(quest_id, result.body())
	else:
		_notice(result)
	return result


## A world interaction (NPC, quest object): reports it for the first active quest step it matches; false when no
## active step is about it (the zone then opens its usual screen).
func on_interaction(area_id: String, interactable_id: String, npc_id: String = "") -> bool:
	var order: Array[QuestModel] = []
	var tracked: QuestModel = quest(tracked_quest_id)
	if tracked != null and tracked.is_active():
		order.append(tracked)
	for q: QuestModel in active():
		if q != tracked:
			order.append(q)
	for q: QuestModel in order:
		var step: Dictionary = q.step_for(area_id, interactable_id, npc_id)
		if not step.is_empty():
			await report_step(q.quest_id, String(step.get("step_id", "")), area_id, interactable_id, npc_id)
			return true
	return false


## `startQuestActivity` (Idempotency-Key): opens the activity player on the attempt the server created (mode quest).
func start_activity(quest_id: String, step_id: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post("%s/%s/steps/%s/activities:start" % [PATH, quest_id.uri_encode(), step_id.uri_encode()],
			{"game_session_id": _session_id()} if not _session_id().is_empty() else {})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await _send(req)
	if result.ok:
		var b: Dictionary = result.body()
		_open(SCREEN_ACTIVITY_HOST, {"attempt_id": String(b.get("attempt_id", "")), "activity_id": String(b.get("activity_id", "")),
			"mode": "quest", "context_type": "quest_step", "context_id": String(b.get("context_id", "")), "quest_id": quest_id, "step_id": step_id})
	else:
		_notice(result)
	return result


## Dialogue actions of the NPCs (F071 DialogueActions.quest_api): `give_quest` accepts, `complete_step` reports the
## talk_npc step of the dialogue's NPC.
func dialogue_action(action: Dictionary) -> ApiResult:
	var quest_id: String = String(action.get("quest_id", ""))
	match String(action.get("type", "")):
		"give_quest":
			return await accept(quest_id)
		"complete_step":
			var area: String = String(store.get("current_zone")) if store != null and store.get("current_zone") != null else ""
			return await report_step(quest_id, String(action.get("step_id", "")), area, "", String(action.get("npc_id", "")))
	return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)


func clear() -> void:
	quests.clear()
	track("")
	quests_updated.emit(0)


func _merge(body: Dictionary) -> void:
	var id: String = String(body.get("quest_id", ""))
	var q: QuestModel = quest(id)
	if q == null:
		q = QuestModel.from_dict(body)
		if q == null:
			return
		quests.append(q)
	else:
		q.apply(body)
	progress_updated.emit(id)


func _merge_progress(quest_id: String, progress: Dictionary) -> void:
	var q: QuestModel = quest(quest_id)
	if q == null:
		q = QuestModel.new()
		q.quest_id = quest_id
		quests.append(q)
	q.apply_progress(progress)
	q.status = String(progress.get("status", q.status))
	progress_updated.emit(quest_id)
	if q.status == QuestModel.STATUS_COMPLETED:
		quest_completed.emit(quest_id)


func _on_message(type: String, payload: Dictionary) -> void:
	if type != MSG_PROGRESS:
		return
	var id: String = String(payload.get("quest_id", ""))
	var q: QuestModel = quest(id)
	if q == null:
		refresh()
		return
	q.apply_update(payload)
	if payload.get("passed") == false:
		notice_requested.emit("ui.quest.notice.try_again")
	progress_updated.emit(id)
	if q.status == QuestModel.STATUS_COMPLETED:
		quest_completed.emit(id)


func _notice(result: ApiResult) -> void:
	var key: String = String(NOTICES.get(result.code, ""))
	if result.status >= 500 or result.code == ApiResult.CODE_NETWORK_ERROR:
		key = String(NOTICES["UNAVAILABLE"])
	if not key.is_empty():
		notice_requested.emit(key)


func _open(screen: StringName, args: Dictionary) -> void:
	if intent_sink.is_valid():
		intent_sink.call(screen, args)
		return
	var bus: Node = Engine.get_main_loop().root.get_node_or_null("EventBus") if Engine.get_main_loop() is SceneTree else null
	if bus != null:
		bus.emit_signal("ui_requested", screen, args)


func _session_id() -> String:
	return String(store.get("game_session_id")) if store != null and store.get("game_session_id") != null else ""


func _send(req: ApiRequest) -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await api.send(req)
	if result.status == 403 and result.code in ACCESS_DENIED_CODES:
		access_denied.emit(result.code)
	return result
