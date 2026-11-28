class_name BossClient
extends GameApiService
## Boss battles of the signed-in student, solo and party (F110 US1; CHALLENGE_BOSS_SYSTEM §3, STUDENT_GAME_UX_CORE S06,
## GAME_NETWORKING §4.2 dispatcher prefix `boss`). The only place of the client that calls the boss API (AGENTS R1):
## - REST: `listBosses` (the boss card of a gate / NPC), `createBossEncounter` (`mode = solo`, one Idempotency-Key per
##   intent kept while it is retried — F110-AC-01), `getBossEncounter` (enter, a version gap, back online, another
##   device: the open questions of `my_questions` come back — F110-AC-09), `leaveBossEncounter` (key per encounter,
##   F110-AC-10). A party encounter is never created here: it starts from the Party Lobby (`party.session.started`
##   `room_type = boss`, F110-AC-02).
## - Realtime: `boss.encounter.ready` once the arena is on screen (and again after a reconnect so the gateway joins the
##   room again), `boss.answer.submit` with the ids of the question, the raw response and `client_elapsed_ms` for
##   reference only. While the server has not acked / refused the answer the question stays locked; an answer that
##   could not be sent (offline) may be sent again with the SAME `question_instance_id` (F110-AC-04).
## - `boss.*` messages go to BossStore (state_version, server energy, own questions / answers, result).
## The client never computes energy, contribution or rewards (ADR-0016, R5). Ids only in logs, never a nickname (R14).

## A friendly notice for the arena (`key` of the catalog + format args), never a raw code.
signal notice_requested(key: String, args: Dictionary)
## The encounter is over for this student without a result to show: `target` is `hub` (gone / not a member), `zone`
## (left a solo boss) or `lobby` (left a party boss). `notice_key` may be empty.
signal exit_requested(target: String, notice_key: String)
## The student's writes are locked (no connection) or unlocked again.
signal connection_changed(offline: bool)
signal bosses_changed()

const PREFIX: String = "boss"
const PATH_BOSSES: String = "/game/bosses"
const PATH_CREATE: String = "/game/bosses/%s/encounters"
const PATH_ENCOUNTER: String = "/game/boss-encounters/%s"
const PATH_LEAVE: String = "/game/boss-encounters/%s:leave"
const CMD_READY: String = "boss.encounter.ready"
const CMD_SUBMIT: String = "boss.answer.submit"
const EXIT_HUB: String = "hub"
const EXIT_ZONE: String = "zone"
const EXIT_LOBBY: String = "lobby"
const CODE_PHASE_CLOSED: String = "BOSS_PHASE_CLOSED"
const CODE_NOT_MEMBER: String = "ROOM_NOT_MEMBER"
const CODE_NOT_FOUND: String = "NOT_FOUND"
const KEY_GENERIC: String = "ui.error.generic"
const KEY_GONE: String = "ui.boss.gone"
## Problem / command code → friendly sentence (F110-AC-01, AC-05, AC-10, AC-27: `ui.error.<CODE>`).
const ERROR_KEYS: Dictionary = {
	"CHALLENGE_NOT_ELIGIBLE": "ui.error.CHALLENGE_NOT_ELIGIBLE",
	"CHALLENGE_ATTEMPT_LIMIT": "ui.error.CHALLENGE_ATTEMPT_LIMIT",
	"BOSS_ENCOUNTER_FULL": "ui.error.BOSS_ENCOUNTER_FULL",
	"BOSS_PHASE_CLOSED": "ui.error.BOSS_PHASE_CLOSED",
	"ROOM_NOT_MEMBER": "ui.error.ROOM_NOT_MEMBER",
	"PERMISSION_DENIED": "ui.error.PERMISSION_DENIED",
	"NOT_FOUND": "ui.error.NOT_FOUND",
	"RATE_LIMITED": "ui.error.RATE_LIMITED",
	"UNAVAILABLE": "ui.error.UNAVAILABLE",
	"NETWORK_ERROR": "ui.boss.reconnecting",
	"OFFLINE": "ui.boss.reconnecting",
	"TIMEOUT": "ui.boss.reconnecting",
}

var realtime: GameRealtimeClient = null
var clock: ClientClock = null
var store: BossStore = null
var bosses: Array[BossSummary] = []
## Answers sent (tests count them; one per question unless it could not be sent).
var submits: int = 0
var _intent_keys: Dictionary = {}
var _was_reconnecting: bool = false
## Server unix ms minus device ms from the last REST `server_time` (0 = unknown).
var _rest_offset_ms: int = 0
var _has_rest_offset: bool = false


func _init(rest: RestApiClient = null, app_state: Object = null, time_source: ClientClock = null,
		realtime_client: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	clock = time_source if time_source != null else ClientClock.new()
	store = BossStore.new(app_state)
	store.resync_needed.connect(resync)
	realtime = realtime_client
	offline_changed.connect(func(value: bool) -> void: connection_changed.emit(value or _reconnecting()))
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


# ---------- reads ----------

## Coroutine: `listBosses` of an area (every page; the server filters grade, tenant and availability).
func list(area_id: String = "") -> ApiResult:
	var query: Dictionary = {"area_id": area_id.left(64)} if not area_id.is_empty() else {}
	var pages: Dictionary = await send_all_pages(PATH_BOSSES, query, "listBosses")
	if bool(pages["ok"]):
		bosses = BossSummary.list_from(pages["items"])
		bosses_changed.emit()
	return pages["result"] as ApiResult if pages["result"] != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


func boss(boss_id: String) -> BossSummary:
	for b: BossSummary in bosses:
		if b.boss_id == boss_id:
			return b
	return null


# ---------- intents ----------

## Coroutine: "Chơi một mình" — `createBossEncounter` `mode = solo` (F110-AC-01). The key of the intent is kept while the
## request is retried after a network failure (AGENTS R10). A boss without `solo` is refused here (only "Rủ bạn").
func start_solo(summary: BossSummary) -> ApiResult:
	if summary == null or not summary.allows_solo():
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_post(PATH_CREATE % summary.boss_id, {"mode": BossSummary.MODE_SOLO})
	var result: ApiResult = await _write(req, "createBossEncounter", "solo:%s" % summary.boss_id)
	if result.ok:
		_sync_offset(result.body())
		store.apply_snapshot(BossEncounterModel.from_dict(result.body()), DataRead.arr_of(result.body(), "my_questions"))
	return result


## Coroutine: the arena is on screen for `encounter_id` (solo after `createBossEncounter`, party after the lobby
## countdown): read the encounter, then say ready (F110-AC-02).
func enter(encounter_id: String) -> ApiResult:
	if not Uuid.is_valid(encounter_id):
		return ApiResult.failure("VALIDATION_FAILED")
	if store.encounter != null and store.encounter.encounter_id != encounter_id:
		store.clear()
	var result: ApiResult = await load_encounter(encounter_id)
	if result.ok:
		# Not awaited: the arena shows the encounter at once; the ack (or a refusal) comes later.
		send_ready()
	return result


## Coroutine: `getBossEncounter`. `404` / `ROOM_NOT_MEMBER` → the encounter is gone for this student (F110-AC-10/30).
func load_encounter(encounter_id: String) -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_ENCOUNTER % encounter_id), "getBossEncounter")
	if result.ok:
		_sync_offset(result.body())
		var next: BossEncounterModel = BossEncounterModel.from_dict(result.body())
		if next != null:
			store.apply_snapshot(next, DataRead.arr_of(result.body(), "my_questions"))
	elif result.status == 404 or result.code == CODE_NOT_MEMBER:
		_gone()
	return result


## Coroutine: `boss.encounter.ready` (`ready = true`).
func send_ready() -> CommandResult:
	var id: String = store.encounter_id()
	if realtime == null or id.is_empty():
		return CommandResult.failure(CommandResult.CODE_OFFLINE)
	var outcome: CommandResult = await realtime.send_command(CMD_READY, {"encounter_id": id, "ready": true})
	if not outcome.ok and outcome.code == CODE_NOT_MEMBER:
		_gone()
	return outcome


## Coroutine: the student's answer to the question on screen. Returns true when the server took it. Locked while it
## is on its way (F110-AC-04); refused at once when there is no connection (the student may answer again once back).
func submit(response: Dictionary) -> bool:
	var q: BossQuestionModel = store.current_question()
	if q == null or not q.is_open() or response.is_empty() or is_offline():
		if is_offline():
			notice_requested.emit("ui.boss.reconnecting", {})
		return false
	q.state = BossQuestionModel.STATE_SENDING
	store.question_changed.emit(q)
	submits += 1
	var outcome: CommandResult = await realtime.send_command(CMD_SUBMIT, q.submit_payload(response, clock.now_msec())) \
			if realtime != null else CommandResult.failure(CommandResult.CODE_OFFLINE)
	if outcome.ok:
		return true
	match outcome.code:
		CODE_PHASE_CLOSED:
			store.close_question(q.question_instance_id)
			notice_requested.emit(error_key(outcome.code), {})
		CODE_NOT_MEMBER:
			_gone()
		_:
			# Not taken (offline, timeout, invalid): the same question may be answered again — same instance id.
			if q.state == BossQuestionModel.STATE_SENDING:
				q.state = BossQuestionModel.STATE_OPEN
			store.question_changed.emit(store.current_question())
			notice_requested.emit(error_key(outcome.code), {})
			if outcome.code == CommandResult.CODE_TIMEOUT:
				resync()
	return false


## Coroutine: "Rời" confirmed — `leaveBossEncounter` (key per encounter). Back to the zone (solo) or the lobby (party);
## `ROOM_NOT_MEMBER` / `404` → back to the Hub with a general sentence (F110-AC-10).
func leave() -> ApiResult:
	var e: BossEncounterModel = store.encounter
	if e == null:
		exit_requested.emit(EXIT_HUB, "")
		return ApiResult.failure(CODE_NOT_FOUND)
	var party: bool = e.is_party()
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_LEAVE % e.encounter_id), "leaveBossEncounter",
			"leave:%s" % e.encounter_id, PackedStringArray([CODE_NOT_MEMBER, CODE_NOT_FOUND]))
	if result.ok:
		store.clear()
		exit_requested.emit(EXIT_LOBBY if party else EXIT_ZONE, "")
	elif result.status == 404 or result.code in [CODE_NOT_MEMBER, CODE_NOT_FOUND]:
		_gone()
	return result


## Coroutine: back online, a version gap, a timeout: read the encounter again and say ready again so the gateway joins
## the room again; the open question of this student comes back from `my_questions` (F110-AC-09).
func resync() -> void:
	var id: String = store.encounter_id()
	if id.is_empty() or (store.encounter != null and store.encounter.is_over()):
		return
	var result: ApiResult = await load_encounter(id)
	if result.ok:
		send_ready()


# ---------- state for the screens ----------

func is_offline() -> bool:
	return offline or _reconnecting()


## Server time in unix ms (realtime clock offset when known, else the last REST `server_time`, else the device time).
func now_ms() -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	if _has_rest_offset:
		return clock.now_msec() + _rest_offset_ms
	return int(clock.unix_time() * 1000.0)


## Milliseconds left in the phase (`phase_deadline_at`, else the encounter `deadline_at`); -1 without a deadline.
func phase_left_ms() -> int:
	var e: BossEncounterModel = store.encounter
	if e == null:
		return -1
	var deadline: int = e.phase_deadline_ms if e.phase_deadline_ms > 0 else e.deadline_ms
	return maxi(0, deadline - now_ms()) if deadline > 0 else -1


static func error_key(code: String) -> String:
	return String(ERROR_KEYS.get(code, KEY_GENERIC))


## Realtime dispatcher handler for `boss.*` (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	store.apply_message(type, payload)


## Session ended / replaced: nothing of the encounter is kept (F110-AC-29).
func clear() -> void:
	store.clear()
	bosses.clear()
	_intent_keys.clear()
	submits = 0
	_has_rest_offset = false


# ---------- internals ----------

func _write(req: ApiRequest, operation_id: String, intent: String, silent: PackedStringArray = PackedStringArray()) -> ApiResult:
	if not _intent_keys.has(intent):
		_intent_keys[intent] = Uuid.v7()
	req.idempotency_key = String(_intent_keys[intent])
	var result: ApiResult = await send_request(req, operation_id)
	if not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	if not result.ok and not result.code in silent and not result.code in ACCESS_DENIED_CODES:
		notice_requested.emit(error_key(result.code), {})
	return result


func _gone() -> void:
	store.clear()
	exit_requested.emit(EXIT_HUB, KEY_GONE)


func _sync_offset(body: Dictionary) -> void:
	var server: int = ExamCountdown.iso_to_ms(DataRead.str_of(body, "server_time"))
	if server > 0:
		_rest_offset_ms = server - clock.now_msec()
		_has_rest_offset = true


func _reconnecting() -> bool:
	return realtime != null and realtime.is_reconnecting()


func _on_connection_state_changed(_state: int) -> void:
	var now: bool = _reconnecting()
	if now != _was_reconnecting:
		connection_changed.emit(now or offline)
	if _was_reconnecting and not now and realtime.is_online():
		resync()
	_was_reconnecting = now
