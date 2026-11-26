class_name ChallengeClient
extends GameApiService
## Solo challenges of the signed-in student (F069 US3, CHALLENGE_BOSS_SYSTEM §2, STUDENT_GAME_UX_CORE S06; name of the
## GAME_NETWORKING §4.2 dispatcher): the board (`listGameChallenges`, `getGameChallenge`), a run (`startGameChallenge`
## → `nextChallengeActivity` → activity player on the learning attempt the server created → next … →
## `challenge.attempt.updated` / `done`), giving up (`abandonGameChallengeAttempt`) and resuming after a reconnect or
## on another device (`active_attempt_id` → `getGameChallengeAttempt`, F069-AC-20). Every write carries an
## Idempotency-Key kept on retry (start per intent, next per turn, abandon per run). The client never picks a question,
## never counts score, stamina or time (ADR-0016, R5): all of it comes from the server; the clock runs on `deadline_at`
## (ChallengeStore).

signal challenges_changed(count: int)
signal attempt_changed(attempt: ChallengeAttemptModel)
## The run ended (`succeeded` / `failed` / `abandoned`); the result screen opens.
signal challenge_finished(attempt: ChallengeAttemptModel)
## A friendly notice for the board/HUD (`ui.challenge.*` key + format args); never a raw code.
signal notice_requested(key: String, args: Dictionary)
## Another run of this student is in progress (`409 CONFLICT` or `active_attempt_id`): offer to continue it.
signal resume_offered(challenge_attempt_id: String)

const PATH_LIST: String = "/game/challenges"
const PATH_DETAIL: String = "/game/challenges/%s"
const PATH_START: String = "/game/challenges/%s:start"
const PATH_ATTEMPT: String = "/game/challenge-attempts/%s"
const PATH_NEXT: String = "/game/challenge-attempts/%s/activities:next"
const PATH_ABANDON: String = "/game/challenge-attempts/%s:abandon"
const PREFIX: String = "challenge"
const MSG_UPDATED: String = "challenge.attempt.updated"
const SCREEN_ACTIVITY_HOST: StringName = &"activity_host"
const SCREEN_RESULT: StringName = &"challenge_result"
## Extra waits on `503 UNAVAILABLE` of `nextChallengeActivity` after ApiClient's own retries (CONFIG_DEFAULTS
## `client_reconnect_backoff`, Backoff), same key, display unchanged (F069-AC-17).
const NEXT_UNAVAILABLE_RETRIES: int = 3
const CODE_NOT_ELIGIBLE: String = "CHALLENGE_NOT_ELIGIBLE"
const CODE_ATTEMPT_LIMIT: String = "CHALLENGE_ATTEMPT_LIMIT"
const CODE_CONFLICT: String = "CONFLICT"
const CODE_INVALID_STATE: String = "INVALID_STATE_TRANSITION"
const CODE_NOT_FOUND: String = "NOT_FOUND"
const CODE_UNAVAILABLE: String = "UNAVAILABLE"

var challenges: Array[ChallengeSummaryModel] = []
var detail: ChallengeDetailModel = null
var store: ChallengeStore = null
var clock: ClientClock = null
var realtime: GameRealtimeClient = null
var learning: LearningService = null
var rng: RandomNumberGenerator = null
## Learning attempt of the activity currently opened for this run.
var activity_attempt_id: String = ""
## `(screen_id: StringName, args: Dictionary) -> void`; default emits `EventBus.ui_requested`.
var intent_sink: Callable = Callable()
var _start_keys: Dictionary = {}
var _next_keys: Dictionary = {}
var _abandon_keys: Dictionary = {}
var _busy: bool = false
var _finished_id: String = ""
## The run ended while an activity was on screen: the result opens once the student sees its last feedback
## (activity submitted or closed — spec edge case "câu cuối").
var _result_pending: bool = false


func _init(rest: RestApiClient = null, app_state: Object = null, time_source: ClientClock = null,
		realtime_client: GameRealtimeClient = null, learning_service: LearningService = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	clock = time_source if time_source != null else ClientClock.new()
	store = ChallengeStore.new(clock, app_state)
	store.attempt_changed.connect(func(a: ChallengeAttemptModel) -> void: attempt_changed.emit(a))
	realtime = realtime_client
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
	learning = learning_service
	if learning != null:
		learning.attempt_submitted.connect(_on_learning_submitted)


func attempt() -> ChallengeAttemptModel:
	return store.attempt


func is_running() -> bool:
	return store.is_running()


## Writes (start, continue, give up) are locked while offline (F069-AC-28); reading stays possible.
func can_write() -> bool:
	return not offline and not _busy


# ---------- board ----------

## Coroutine: every page of `listGameChallenges` (`type`, `area_id` filters). A run in progress of this student
## (`active_attempt_id`) is offered for resuming (F069-AC-20). The previous list is replaced (a new grade after a
## school-year change never keeps old entries).
func list(filter: Dictionary = {}) -> ApiResult:
	var query: Dictionary = {}
	for key: String in ["type", "area_id"]:
		if filter.get(key) is String and not String(filter[key]).is_empty():
			query[key] = String(filter[key])
	var page: Dictionary = await send_all_pages(PATH_LIST, query, "listGameChallenges")
	if bool(page["ok"]):
		challenges = ChallengeSummaryModel.list_from(page["items"])
		challenges_changed.emit(challenges.size())
		var active: String = active_attempt_id()
		if not active.is_empty() and (store.attempt == null or store.attempt.challenge_attempt_id != active):
			resume_offered.emit(active)
	return page["result"] as ApiResult


func summary(challenge_id: String) -> ChallengeSummaryModel:
	for s: ChallengeSummaryModel in challenges:
		if s.challenge_id == challenge_id:
			return s
	return null


## `active_attempt_id` of the first listed challenge with a run in progress ("" when none / not sent).
func active_attempt_id() -> String:
	for s: ChallengeSummaryModel in challenges:
		if not s.active_attempt_id.is_empty():
			return s.active_attempt_id
	return ""


## Coroutine: `getGameChallenge` (F069-AC-15). `404` → the list is refreshed and "Thử thách không còn mở".
func get_challenge(challenge_id: String) -> ChallengeDetailModel:
	if not Uuid.is_valid(challenge_id):
		return null
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_DETAIL % challenge_id), "getGameChallenge")
	if result.status == 404:
		notice_requested.emit("ui.challenge.gone", {})
		await list()
		return null
	if not result.ok:
		return null
	# The cooldown is shown on the server clock: align it on this answer (a run in progress keeps its deadline).
	store.sync_clock(server_ms_of(result))
	detail = ChallengeDetailModel.detail_from(result.data)
	return detail


## True while the detail's cooldown (`cooldown_until`) is in the future on the server clock.
func cooldown_left_ms(of: ChallengeDetailModel) -> int:
	if of == null or of.cooldown_until.is_empty():
		return 0
	return maxi(0, ExamCountdown.iso_to_ms(of.cooldown_until) - store.server_now_ms())


# ---------- a run ----------

## Coroutine (F069-AC-16): `startGameChallenge {difficulty}` with an Idempotency-Key per intent (same key on retry
## until a definitive answer). `201` → the run starts and the first activity is fetched. Refusals become friendly
## notices with the time the student may retry; `409 CONFLICT` offers to continue the run in progress.
func start(challenge_id: String, difficulty: String) -> ApiResult:
	if not Uuid.is_valid(challenge_id) or not difficulty in ChallengeSummaryModel.DIFFICULTIES:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	var intent: String = "%s|%s" % [challenge_id, difficulty]
	var key: String = String(_start_keys.get(intent, ""))
	if key.is_empty():
		key = Uuid.v7()
		_start_keys[intent] = key
	var req: ApiRequest = ApiRequest.make_post(PATH_START % challenge_id, {"difficulty": difficulty})
	req.idempotency_key = key
	_busy = true
	var result: ApiResult = await send_request(req, "startGameChallenge")
	_busy = false
	if result.is_transient():
		notice_requested.emit("ui.challenge.notice.unavailable", {})
		return result
	_start_keys.erase(intent)
	if result.ok:
		var started: ChallengeAttemptModel = ChallengeAttemptModel.from_dict(result.data)
		if started == null:
			return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
		store.detail = detail if detail != null and detail.challenge_id == challenge_id else null
		store.set_attempt(started, server_ms_of(result))
		await next()
		return result
	match result.code:
		CODE_NOT_ELIGIBLE:
			notice_requested.emit("ui.challenge.notice.not_eligible", {})
		CODE_ATTEMPT_LIMIT:
			var at: String = retry_time_text(result)
			notice_requested.emit("ui.challenge.notice.attempt_limit_at" if not at.is_empty()
					else "ui.challenge.no_attempts", {"time": at})
		CODE_CONFLICT:
			var active: String = active_attempt_id()
			if active.is_empty():
				await list()
				active = active_attempt_id()
			notice_requested.emit("ui.challenge.notice.in_progress", {})
			if not active.is_empty():
				resume_offered.emit(active)
		CODE_NOT_FOUND:
			notice_requested.emit("ui.challenge.gone", {})
			await list()
		_:
			notice_requested.emit("ui.error.generic", {})
	return result


## Coroutine (F069-AC-17): `nextChallengeActivity` with the key of this turn (`activity_seq`). An attempt in the answer
## opens the activity player on it (the server chose the content); `done` or a finished status ends the run. A
## `503 UNAVAILABLE` is retried with the same key after a backoff, without changing what the student sees.
func next() -> ApiResult:
	var run: ChallengeAttemptModel = store.attempt
	if run == null or run.is_finished():
		return ApiResult.failure(CODE_INVALID_STATE)
	var seq: int = run.activity_seq + 1
	var turn: String = "%s|%d" % [run.challenge_attempt_id, seq]
	var key: String = String(_next_keys.get(turn, ""))
	if key.is_empty():
		key = Uuid.v7()
		_next_keys[turn] = key
	var result: ApiResult = null
	_busy = true
	for attempt_no: int in NEXT_UNAVAILABLE_RETRIES + 1:
		var req: ApiRequest = ApiRequest.make_post(PATH_NEXT % run.challenge_attempt_id, {})
		req.idempotency_key = key
		result = await send_request(req, "nextChallengeActivity")
		if not (result.status == 503 or result.code == CODE_UNAVAILABLE) or attempt_no == NEXT_UNAVAILABLE_RETRIES:
			break
		await clock.wait(maxf(result.retry_after_sec(0.0), Backoff.delay(attempt_no + 1, rng)))
	_busy = false
	if result.is_transient():
		notice_requested.emit("ui.challenge.notice.reconnecting", {})
		return result
	_next_keys.erase(turn)
	if result.code == CODE_INVALID_STATE:
		await resume(run.challenge_attempt_id)
		return result
	if not result.ok:
		return result
	run.activity_seq = seq
	var body: Dictionary = result.body()
	var updated: Dictionary = DataRead.dict_of(body, "challenge_attempt")
	if not updated.is_empty() and DataRead.str_of(updated, "challenge_attempt_id") == run.challenge_attempt_id:
		run.apply_update(updated)
		store.sync_clock(server_ms_of(result))
		attempt_changed.emit(run)
	var activity: Dictionary = DataRead.dict_of(body, "attempt")
	if DataRead.bool_of(body, "done") or run.is_finished() or activity.is_empty():
		_finish()
		return result
	activity_attempt_id = DataRead.str_of(activity, "attempt_id")
	_open(SCREEN_ACTIVITY_HOST, {"attempt_id": activity_attempt_id, "mode": "challenge",
		"context_type": "challenge_attempt", "context_id": run.challenge_attempt_id,
		"activity_type": DataRead.str_of(activity, "activity_type")})
	return result


## Opens again the activity of the run (the student closed the player, or after a resume) — or fetches the next one.
func continue_run() -> ApiResult:
	if not is_running():
		return ApiResult.failure(CODE_INVALID_STATE)
	if not activity_attempt_id.is_empty():
		_open(SCREEN_ACTIVITY_HOST, {"attempt_id": activity_attempt_id, "mode": "challenge",
			"context_type": "challenge_attempt", "context_id": store.attempt.challenge_attempt_id})
		var ok: ApiResult = ApiResult.new()
		ok.ok = true
		return ok
	return await next()


## Coroutine (F069-AC-19): gives up after the student confirmed — `abandonGameChallengeAttempt` (key per run). The
## learning attempts already recorded stay (server).
func abandon() -> ApiResult:
	var run: ChallengeAttemptModel = store.attempt
	if run == null or run.is_finished():
		return ApiResult.failure(CODE_INVALID_STATE)
	var key: String = String(_abandon_keys.get(run.challenge_attempt_id, ""))
	if key.is_empty():
		key = Uuid.v7()
		_abandon_keys[run.challenge_attempt_id] = key
	var req: ApiRequest = ApiRequest.make_post(PATH_ABANDON % run.challenge_attempt_id, {})
	req.idempotency_key = key
	var result: ApiResult = await send_request(req, "abandonGameChallengeAttempt")
	if result.is_transient():
		notice_requested.emit("ui.challenge.notice.reconnecting", {})
		return result
	_abandon_keys.erase(run.challenge_attempt_id)
	if result.code == CODE_INVALID_STATE:
		await resume(run.challenge_attempt_id)
		return result
	if result.ok:
		var ended: ChallengeAttemptModel = ChallengeAttemptModel.from_dict(result.data)
		if ended != null:
			run.apply_update(result.body())
			attempt_changed.emit(run)
		_finish()
	return result


## Coroutine (F069-AC-20): `getGameChallengeAttempt` after a reconnect, a restart or a takeover. A run still in
## progress is taken back (the student continues until `deadline_at`); a finished one shows the server's result.
func resume(challenge_attempt_id: String) -> ApiResult:
	if not Uuid.is_valid(challenge_attempt_id):
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_ATTEMPT % challenge_attempt_id),
			"getGameChallengeAttempt")
	if not result.ok:
		return result
	var loaded: ChallengeAttemptModel = ChallengeAttemptModel.from_dict(result.data)
	if loaded == null:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	if store.attempt == null or store.attempt.challenge_attempt_id != loaded.challenge_attempt_id:
		activity_attempt_id = ""
	store.set_attempt(loaded, server_ms_of(result))
	if loaded.is_finished():
		_finish()
	return result


## Realtime dispatcher handler (`challenge.*`): score, stamina, progress and end of the CURRENT run only.
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type != MSG_UPDATED or not store.apply_message(payload):
		return
	if not store.attempt.is_finished():
		return
	if not activity_attempt_id.is_empty():
		_result_pending = true
		return
	_finish()


## WorldRoot: the activity player closed. A result held for it opens now.
func on_activity_closed() -> void:
	if _result_pending:
		_result_pending = false
		activity_attempt_id = ""
		_finish()


## The result screen was closed: the run is over for the client (level-up modal may show now).
func dismiss_result() -> void:
	activity_attempt_id = ""
	_result_pending = false
	store.clear()


## Session ended: drops the run, the list and the pending keys (F069-AC-28).
func clear() -> void:
	challenges.clear()
	detail = null
	activity_attempt_id = ""
	_start_keys.clear()
	_next_keys.clear()
	_abandon_keys.clear()
	store.clear()


## Server unix ms of a response: the realtime clock when synced, else its `Date` header, else 0 (device time).
func server_ms_of(result: ApiResult) -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	return http_date_ms(String(result.headers.get("date", ""))) if result != null else 0


## "Em có thể thử lại lúc 07:30" from the problem's `retry_after` (seconds) or the `Retry-After` header; "" if none.
func retry_time_text(result: ApiResult) -> String:
	var seconds: float = DataRead.num_of(result.body(), "retry_after", -1.0)
	if seconds < 0.0:
		seconds = DataRead.num_of(DataRead.dict_of(result.body(), "details"), "retry_after", -1.0)
	if seconds < 0.0 and result.headers.has("retry-after"):
		seconds = result.retry_after_sec(-1.0)
	if seconds < 0.0:
		return ""
	var server_ms: int = server_ms_of(result)
	if server_ms <= 0:
		server_ms = int(clock.unix_time() * 1000.0)
	return LocaleFormat.time(LocaleFormat.from_unix(int(server_ms / 1000) + int(seconds)))


## RFC 7231 HTTP date ("Thu, 26 Nov 2026 07:30:00 GMT") → unix ms; 0 when absent or invalid.
static func http_date_ms(text: String) -> int:
	var parts: PackedStringArray = text.strip_edges().split(" ", false)
	if parts.size() < 5:
		return 0
	var months: PackedStringArray = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
	var month: int = months.find(parts[2]) + 1
	if month < 1 or not parts[1].is_valid_int() or not parts[3].is_valid_int():
		return 0
	var iso: String = "%04d-%02d-%02dT%s" % [int(parts[3]), month, int(parts[1]), parts[4]]
	var unix: int = Time.get_unix_time_from_datetime_string(iso)
	return unix * 1000 if unix > 0 else 0


func _finish() -> void:
	var run: ChallengeAttemptModel = store.attempt
	if run == null or run.challenge_attempt_id == _finished_id:
		return
	_finished_id = run.challenge_attempt_id
	challenge_finished.emit(run)
	_open(SCREEN_RESULT, {"challenge_attempt_id": run.challenge_attempt_id})


func _on_learning_submitted(submitted: AttemptModel) -> void:
	if submitted == null or submitted.attempt_id != activity_attempt_id:
		return
	if _result_pending:
		on_activity_closed()
		return
	if not is_running():
		return
	activity_attempt_id = ""
	await next()


func _open(screen: StringName, args: Dictionary) -> void:
	if intent_sink.is_valid():
		intent_sink.call(screen, args)
		return
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var bus: Node = tree.root.get_node_or_null("EventBus") if tree != null else null
	if bus != null:
		bus.emit_signal("ui_requested", screen, args)
