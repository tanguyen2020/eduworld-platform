class_name CoopQuestService
extends GameApiService
## Party quests of the signed-in student (F110 US2/US3; QUEST_SYSTEM §3.2, §6; PARTY_SYSTEM §7.1): the shared progress
## of a party quest session (`quest.progress.updated.party_progress`, `QuestProgress.party_progress`), the roles and
## clue cards of an `interdependent` step (`quest.roles.assigned`, `getQuestStepClues`, `quest.clue.shared`,
## `shareQuestStepClue`), the leader's role choice (`assignQuestStepRoles`) and the student's OWN learning work
## (`startQuestActivity` through QuestService — never for another member, F110-AC-13/17). The personal part of
## `quest.progress.updated` stays with QuestService (F072). The client never builds, combines or guesses a clue: it shows
## what the server sent (F110-AC-15, ADR-0016); keys are kept while a write is retried (AGENTS R10).

## The shared progress of a step changed.
signal progress_changed(quest_id: String, step_id: String)
signal roles_changed(quest_id: String, step_id: String)
signal clues_changed(quest_id: String, step_id: String)
## This student now holds `role` too (a member left past the grace, F110-AC-18).
signal extra_role_taken(quest_id: String, step_id: String, role: String)
## The step / its clues are gone for this student (`404`): the panel closes and the quest list is read again.
signal step_closed(quest_id: String, step_id: String)
signal notice_requested(key: String, args: Dictionary)

const PREFIX: String = "quest"
const MSG_PROGRESS: String = "quest.progress.updated"
const MSG_ROLES: String = "quest.roles.assigned"
const MSG_CLUE: String = "quest.clue.shared"
const PATH_STEP: String = "/game/quests/%s/steps/%s"
const SHARE_CLUE_CARD: String = "clue_card"
const SHARE_CLUE_CARD_AND_CHAT: String = "clue_card_and_chat"
const ERROR_KEYS: Dictionary = {
	"PARTY_NOT_LEADER": "ui.error.PARTY_NOT_LEADER",
	"PERMISSION_DENIED": "ui.error.PERMISSION_DENIED",
	"NOT_FOUND": "ui.error.NOT_FOUND",
	"CONFLICT": "ui.error.CONFLICT",
	"RATE_LIMITED": "ui.error.RATE_LIMITED",
	"UNAVAILABLE": "ui.error.UNAVAILABLE",
	"NETWORK_ERROR": "ui.quest.coop.offline",
}

var store: CoopQuestStore = CoopQuestStore.new()
var quest: QuestService = null
## PartyStore of the student's party (leader check, members for the role choice); null = no party service.
var party_store: PartyStore = null
var app_state: Node = null
var _intent_keys: Dictionary = {}


func _init(rest: RestApiClient = null, realtime: GameRealtimeClient = null, quest_service: QuestService = null,
		app_state_node: Node = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	quest = quest_service
	app_state = app_state_node
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
	if quest != null:
		quest.progress_updated.connect(_on_quest_progress)


func self_id() -> String:
	return CompetitionPlayer.self_id(app_state) if app_state != null else ""


## Coroutine: `getQuestStepClues` — the cards of the student's roles + the shared ones (F110-AC-15).
func load_clues(quest_id: String, step_id: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_get((PATH_STEP + "/clues") % [quest_id.uri_encode(), step_id.uri_encode()])
	var result: ApiResult = await send_request(req, "getQuestStepClues")
	_after(result, quest_id, step_id)
	return result


## Coroutine: "Chia sẻ dữ kiện" — `shareQuestStepClue` of a role this student holds (key kept on retry, F110-AC-16).
func share(quest_id: String, step_id: String, role: String) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post((PATH_STEP + "/clues/%s:share") % [quest_id.uri_encode(),
			step_id.uri_encode(), role.uri_encode()])
	return await _write(req, "shareQuestStepClue", "share:%s:%s:%s" % [quest_id, step_id, role], quest_id, step_id)


## Coroutine: the leader's role choice — `assignQuestStepRoles` (`[{role, student_id}]`; `403 PARTY_NOT_LEADER` → a
## friendly sentence, F110-AC-14).
func assign_roles(quest_id: String, step_id: String, assignments: Array[Dictionary]) -> ApiResult:
	var rows: Array = []
	for a: Dictionary in assignments:
		rows.append({"role": String(a.get("role", "")), "student_id": String(a.get("student_id", ""))})
	var req: ApiRequest = ApiRequest.make_put((PATH_STEP + "/roles") % [quest_id.uri_encode(), step_id.uri_encode()],
			{"assignments": rows})
	var result: ApiResult = await _write(req, "assignQuestStepRoles", "roles:%s:%s" % [quest_id, step_id], quest_id,
			step_id)
	return result


## Coroutine: the student's own learning work of the step ("Trả lời câu cuối" / a learning step) — QuestService
## `startQuestActivity` → ActivityHost for this student only (F110-AC-13, AC-17). There is no way to answer for
## another member or to see their answer.
func start_my_activity(quest_id: String, step_id: String) -> ApiResult:
	if quest == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	return await quest.start_activity(quest_id, step_id)


func is_leader() -> bool:
	return party_store != null and party_store.is_leader()


## "Chọn vai" is shown to the leader only while the server waits for the leader (`pending_leader`).
func can_assign(quest_id: String, step_id: String) -> bool:
	return is_leader() and store.pending_leader(quest_id, step_id)


## `share_mode` of the step from the quest delivery (`clue_card` when the server gives none — the safe default: no
## typing needed, QUEST_SYSTEM §3.2).
func share_mode(quest_id: String, step_id: String) -> String:
	var q: QuestModel = quest.quest(quest_id) if quest != null else null
	if q != null:
		for s: Dictionary in q.steps:
			if String(s.get("step_id", "")) == step_id and String(s.get("share_mode", "")) == SHARE_CLUE_CARD_AND_CHAT:
				return SHARE_CLUE_CARD_AND_CHAT
	return SHARE_CLUE_CARD


## Members the leader may give a role to (party members, nickname only).
func role_candidates() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if party_store != null and party_store.party != null:
		for m: PartyMemberModel in party_store.party.members:
			out.append({"student_id": m.student_id, "nickname": m.nickname})
	return out


static func error_key(code: String) -> String:
	return String(ERROR_KEYS.get(code, "ui.error.generic"))


## Realtime dispatcher handler for `quest.*`: the shared progress, the roles, a shared clue (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	var quest_id: String = DataRead.str_of(payload, "quest_id")
	if quest_id.is_empty():
		return
	match type:
		MSG_PROGRESS:
			if payload.get("party_progress") is Dictionary and store.apply_progress(quest_id, payload["party_progress"]):
				progress_changed.emit(quest_id, DataRead.str_of(payload["party_progress"] as Dictionary, "step_id"))
		MSG_ROLES:
			var step_id: String = DataRead.str_of(payload, "step_id")
			for role: String in store.apply_roles(payload, self_id()):
				extra_role_taken.emit(quest_id, step_id, role)
			roles_changed.emit(quest_id, step_id)
			# The cards of the new roles (own role, or the one taken over) come from the server.
			load_clues(quest_id, step_id)
		MSG_CLUE:
			var step: String = DataRead.str_of(payload, "step_id")
			if store.is_new_share(payload):
				load_clues(quest_id, step)


func clear() -> void:
	store.clear()
	_intent_keys.clear()


# ---------- internals ----------

func _write(req: ApiRequest, operation_id: String, intent: String, quest_id: String, step_id: String) -> ApiResult:
	if not _intent_keys.has(intent):
		_intent_keys[intent] = Uuid.v7()
	req.idempotency_key = String(_intent_keys[intent])
	var result: ApiResult = await send_request(req, operation_id)
	if not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	_after(result, quest_id, step_id)
	return result


func _after(result: ApiResult, quest_id: String, step_id: String) -> void:
	if result.ok:
		var body: Dictionary = result.body()
		if body.get("clues") is Array:
			store.set_clues(body)
			clues_changed.emit(quest_id, step_id)
			roles_changed.emit(quest_id, step_id)
		elif body.get("assignments") is Array:
			store.apply_roles(body, self_id())
			roles_changed.emit(quest_id, step_id)
		return
	if result.status == 404:
		step_closed.emit(quest_id, step_id)
		if quest != null:
			quest.refresh()
		return
	if not result.code in ACCESS_DENIED_CODES:
		notice_requested.emit(error_key(result.code), {})


## The REST delivery / progress of a quest (QuestService) carries `QuestProgress.party_progress`.
func _on_quest_progress(quest_id: String) -> void:
	var q: QuestModel = quest.quest(quest_id) if quest != null else null
	if q != null and not q.party_progress.is_empty() and store.apply_progress(quest_id, q.party_progress):
		for row: Variant in q.party_progress:
			if row is Dictionary:
				progress_changed.emit(quest_id, DataRead.str_of(row as Dictionary, "step_id"))
