class_name LearningService
extends RefCounted
## Activity attempts for the activity player (F063, ADR-0059: one API family `/learning/activity-attempts`). Calls the
## REST API through ApiClient (session `X-Tenant-Id`, refresh, retries — F061) and keeps the current attempt in RAM
## only (GODOT_ARCHITECTURE §8.1). Server-authoritative (ADR-0016, AGENTS R5/R6): it sends raw answers and shows the
## server's grading; it never scores, never holds an answer key and never counts hints.
## Every write carries an `Idempotency-Key` (UUIDv7, AGENTS R10): a new key per answer / hint / submit, the same key
## when the same write is retried (manual retry or PendingWrites replay after a network loss, F063-AC-05/06/08).
## Answers and bodies are never logged (AGENTS R14); telemetry carries codes only.

signal attempt_loaded(attempt: AttemptModel)
signal response_result_received(result: ResponseResultModel)
signal hint_received(hint: HintResultModel)
signal attempt_submitted(attempt: AttemptModel)
signal attempt_failed(code: String)
signal offline_changed(offline: bool)
## A refusal that ends the session (`403 GAME_ACCESS_STUDENT_ONLY`…, F063-AC-11): handled like F061/F062.
signal access_denied(code: String)

const PATH_ACTIVITIES: String = "/learning/activities/%s/attempts"
const PATH_ATTEMPTS: String = "/learning/activity-attempts"
const PATH_LESSON: String = "/learning/lessons/%s"
## `startMiniGame` of module game (F073): free play / quest-step mini-game → `MiniGameSession`.
const PATH_MINIGAME_START: String = "/game/minigames/%s:start"
## `startMiniGame` body `context_type` enum (specs/openapi).
const MINIGAME_CONTEXTS: PackedStringArray = ["free_play", "quest_step", "assignment"]
const CODE_ALREADY_SUBMITTED: String = "ATTEMPT_ALREADY_SUBMITTED"
const CODE_NOT_SUPPORTED: String = "ACTIVITY_TYPE_NOT_SUPPORTED"
const CODE_HINTS_EXHAUSTED: String = "HINTS_EXHAUSTED"
const CODE_HINTS_NOT_ALLOWED: String = "HINTS_NOT_ALLOWED"
const CODE_NOT_FOUND: String = "NOT_FOUND"
const CODE_VIDEO_NOT_READY: String = "VIDEO_NOT_READY"
const CODE_SEEK_NOT_ALLOWED: String = "VIDEO_SEEK_NOT_ALLOWED"
const CODE_PLAY_LIMIT: String = "AUDIO_PLAY_LIMIT_REACHED"
const CODE_MEDIA_NOT_READY: String = "MEDIA_NOT_READY"
const CODE_LOG_INVALID: String = "SIMULATION_LOG_INVALID"
const CODE_RUN_LIMIT: String = "SIMULATION_RUN_LIMIT_REACHED"
## `ActivityAttemptStart` / `AdHocAttemptStart` enums (specs/openapi): other values are not sent.
const ACTIVITY_MODES: PackedStringArray = ["practice", "assignment", "minigame", "retry_wrong", "diagnostic"]
const ACTIVITY_CONTEXTS: PackedStringArray = ["free_play", "assignment", "lesson", "learning_path", "recommendation"]
const ADHOC_MODES: PackedStringArray = ["practice", "assignment", "retry_wrong", "diagnostic"]
const ADHOC_CONTEXTS: PackedStringArray = ["free_play", "assignment"]
## Session-ending refusals (same list as AssignmentService, F062-AC-12); `TENANT_MISMATCH` is ApiClient's.
const ACCESS_DENIED_CODES: PackedStringArray = [
	"GAME_ACCESS_STUDENT_ONLY", "ACCESS_OUTSIDE_ALLOWED_HOURS", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED",
	"NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED", "ACCOUNT_NOT_PROVISIONED",
]
const TELEMETRY_UPDATE_REQUIRED: String = "activity_update_required"
const TELEMETRY_MATH_FAILED: String = "math_svg_failed"
## A `minigame_type` this client has no scene for (F073-AC-21).
const TELEMETRY_MINIGAME_UPDATE_REQUIRED: String = "minigame_update_required"
const TELEMETRY_OPEN_KIND: String = "activity_open"
const SCENE_CODE: String = "activity_host"

var api: RestApiClient = null
var telemetry: TelemetryService = null
var clock: ClientClock = null
## Current attempt (RAM only); null after a 404 / access refusal so nothing of it can be shown (F063-AC-11).
var attempt: AttemptModel = null
var pending: PendingWrites = PendingWrites.new()
var offline: bool = false
## Code of the last failure (`NETWORK_ERROR`, `HTTP_503`, Problem code…), empty after a success.
var last_error_code: String = ""
## Trace id of the last failure (shown in the "Chi tiết" view for support).
var last_trace_id: String = ""
## `ContentManifest.activity_min_client_versions` once a manifest is loaded (FR-EXR-015); empty = not known.
var activity_min_client_versions: Dictionary = {}
## Game rewards the server returned with the submit (`AttemptResult.rewards` shape when present; F063 does not
## compute any reward — the result screen hides the region when empty, spec Clarifications 2026-11-16).
var last_rewards: Array = []
## Idempotency keys of writes not yet answered by the server, by write signature (kept on retry, AGENTS R10).
var _keys: Dictionary = {}


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null, time_source: ClientClock = null) -> void:
	api = rest
	telemetry = telemetry_service
	clock = time_source if time_source != null else ClientClock.new()


# ---------- requests ----------

## Request opening an attempt for an `activity_host` intent (F063-AC-01, T037, F063-AC-24):
## - `{activity_id, mode?, context_type?, context_id?, recommendation_id?}` → `startActivityAttempt`;
## - `{question_set_id, mode?, context_type?, context_id?, recommendation_id?}` → `startAdHocAttempt`
##   (`source.kind = question_set`, default mode `assignment` for a school assignment);
## - `{objective_id, mode?, context_type?}` → `startAdHocAttempt` (`source.kind = selection`; "Luyện LO này" of an
##   exam result, F064-AC-19);
## - `{attempt_id}` → `getActivityAttempt` (resume). Null when the intent has none of these ids.
static func start_request(args: Dictionary) -> ApiRequest:
	var activity_id: String = _uuid(args, "activity_id")
	var set_id: String = _uuid(args, "question_set_id")
	if not activity_id.is_empty():
		var body: Dictionary = {}
		_put_enum(body, "mode", args, ACTIVITY_MODES)
		_put_enum(body, "context_type", args, ACTIVITY_CONTEXTS)
		_put_uuid(body, "context_id", args)
		_put_uuid(body, "recommendation_id", args)
		return ApiRequest.make_post(PATH_ACTIVITIES % activity_id, body)
	if not set_id.is_empty():
		var adhoc: Dictionary = {"mode": "assignment", "source": {"kind": "question_set", "question_set_id": set_id}}
		_put_enum(adhoc, "mode", args, ADHOC_MODES)
		_put_enum(adhoc, "context_type", args, ADHOC_CONTEXTS)
		_put_uuid(adhoc, "context_id", args)
		_put_uuid(adhoc, "recommendation_id", args)
		return ApiRequest.make_post(PATH_ATTEMPTS, adhoc)
	var objective_id: String = _uuid(args, "objective_id")
	if not objective_id.is_empty():
		var practice: Dictionary = {"mode": "practice", "source": {"kind": "selection", "objective_id": objective_id}}
		_put_enum(practice, "mode", args, ADHOC_MODES)
		_put_enum(practice, "context_type", args, ADHOC_CONTEXTS)
		return ApiRequest.make_post(PATH_ATTEMPTS, practice)
	var attempt_id: String = _uuid(args, "attempt_id")
	if not attempt_id.is_empty():
		return ApiRequest.make_get("%s/%s" % [PATH_ATTEMPTS, attempt_id])
	return null


## Intent args of a practice recommended on the result screen (F063-AC-24).
static func practice_args(rec: AttemptReviewModel.Recommendation) -> Dictionary:
	var args: Dictionary = {"question_set_id": rec.ref_id, "mode": "practice", "context_type": "free_play"}
	if not rec.recommendation_id.is_empty():
		args["recommendation_id"] = rec.recommendation_id
	return args


# ---------- attempt lifecycle ----------

## Coroutine: starts (or resumes) the attempt of an intent. The server returns the in-progress attempt of the same
## source when there is one (resume). Returns null on failure (`last_error_code`).
func start(args: Dictionary) -> AttemptModel:
	var req: ApiRequest = start_request(args)
	if req == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	var signature: String = "start|%s|%s" % [req.path, JSON.stringify(req.body)]
	if req.method == "POST":
		req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if not result.is_transient():
		_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	return _loaded(result)


## Coroutine: starts (or resumes) a mini-game from its configuration (`startMiniGame`, F073; MINIGAME_ENGINE §5):
## `POST /game/minigames/{id}:start` with an Idempotency-Key kept on retry and `{context_type, context_id?}`. The
## module game pins the configuration version and creates the attempt (mode `minigame`) through module learning; the
## answers then go through `respond()` / `submit()` like every activity attempt. Null on failure (`last_error_code`).
func start_minigame(config_id: String, context_type: String = "free_play", context_id: String = "") -> MiniGameSessionModel:
	if not Uuid.is_valid(config_id):
		_failed(ApiResult.failure(CODE_NOT_FOUND))
		return null
	var body: Dictionary = {"context_type": context_type if context_type in MINIGAME_CONTEXTS else "free_play"}
	if Uuid.is_valid(context_id):
		body["context_id"] = context_id
	var req: ApiRequest = ApiRequest.make_post(PATH_MINIGAME_START % config_id, body)
	var signature: String = "minigame|%s|%s" % [config_id, JSON.stringify(body)]
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if not result.is_transient():
		_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	var session: MiniGameSessionModel = MiniGameSessionModel.from_dict(result.body(), _now_unix())
	if session == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	if attempt == null or attempt.attempt_id != session.attempt.attempt_id:
		pending.clear()
		_set_offline(false)
	attempt = session.attempt
	last_error_code = ""
	attempt_loaded.emit(session.attempt)
	return session


## Coroutine: reloads an attempt from `getActivityAttempt` (resume on this or another device, F063-AC-07).
func resume(attempt_id: String) -> AttemptModel:
	if not Uuid.is_valid(attempt_id):
		_failed(ApiResult.failure(CODE_NOT_FOUND))
		return null
	var result: ApiResult = await _send(ApiRequest.make_get("%s/%s" % [PATH_ATTEMPTS, attempt_id]))
	if not result.ok:
		_failed(result)
		return null
	return _loaded(result)


## Coroutine: sends one answer (F063-AC-05): `{kind: question, position, question_version_id, response,
## client_elapsed_ms}`. A network loss queues it in PendingWrites with its key and returns null (`offline` = true,
## F063-AC-06). Returns the server result otherwise (null + `last_error_code` on a refusal).
func respond(position: int, response: Dictionary, client_elapsed_ms: int, extra: Dictionary = {}) -> ResponseResultModel:
	if attempt == null:
		return null
	var body: Dictionary = {
		"kind": "question", "position": position, "response": response, "client_elapsed_ms": maxi(0, client_elapsed_ms),
	}
	# F065: an embedded question of a video is `{kind: cue_answer, cue_id}` (the server maps it to its item).
	for key: String in ["kind", "cue_id"]:
		if extra.get(key) is String and not (extra[key] as String).is_empty():
			body[key] = (extra[key] as String).left(64)
	var item: QuestionRenderModel = attempt.item_at(position)
	if item != null and Uuid.is_valid(item.question_version_id):
		body["question_version_id"] = item.question_version_id
	var signature: String = "respond|%s|%d|%s" % [attempt.attempt_id, position, JSON.stringify(response)]
	var req: ApiRequest = ApiRequest.make_post("%s/%s/responses" % [PATH_ATTEMPTS, attempt.attempt_id], body)
	req.idempotency_key = _key(signature)
	if not pending.is_empty():
		_queue(req, position)
		return null
	var result: ApiResult = await _send(req)
	if result.is_transient():
		_queue(req, position)
		last_error_code = result.code
		return null
	_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	return _graded(result)


## Coroutine: re-sends the queued answers in order with their keys (network back). Returns the results received.
func flush_pending() -> Array[ResponseResultModel]:
	var out: Array[ResponseResultModel] = []
	if pending.is_empty() or api == null:
		return out
	for entry: Dictionary in await pending.flush(api):
		var write: PendingWrites.PendingWrite = entry["write"] as PendingWrites.PendingWrite
		var result: ApiResult = entry["result"] as ApiResult
		_forget_key(write.idempotency_key)
		if result.ok:
			out.append(_graded(result))
		else:
			_failed(result)
	_set_offline(not pending.is_empty())
	return out


## Coroutine: next hint level of an item (F063-AC-15). `409 HINTS_EXHAUSTED` / `403 HINTS_NOT_ALLOWED` return null
## with that code (the panel greys out / hides, F063-AC-16).
func request_next_hint(attempt_id: String, position: int) -> HintResultModel:
	var signature: String = "hint|%s|%d" % [attempt_id, position]
	var req: ApiRequest = ApiRequest.make_post("%s/%s/items/%d/hints:next" % [PATH_ATTEMPTS, attempt_id, position])
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if result.is_transient():
		last_error_code = result.code
		return null
	_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	var hint: HintResultModel = HintResultModel.from_dict(result.body())
	if hint == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	if hint.position == 0:
		hint.position = position
	last_error_code = ""
	hint_received.emit(hint)
	return hint


## Coroutine: the branch chosen at a branch point of an interactive video (F065-AC-24): `{kind: branch_choice,
## branch_point_id, option_id}` with a new key (same key on retry); the result names `next_segment_id`.
func choose_branch(branch_point_id: String, option_id: String) -> ResponseResultModel:
	if attempt == null:
		return null
	var body: Dictionary = {"kind": "branch_choice", "branch_point_id": branch_point_id.left(64), "option_id": option_id.left(64)}
	var signature: String = "branch|%s|%s|%s" % [attempt.attempt_id, branch_point_id, option_id]
	var req: ApiRequest = ApiRequest.make_post("%s/%s/responses" % [PATH_ATTEMPTS, attempt.attempt_id], body)
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if result.is_transient():
		last_error_code = result.code
		return null
	_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	last_error_code = ""
	return ResponseResultModel.from_dict(result.body())


## Coroutine: a heartbeat of a media attempt (F065-AC-19): `ActivityProgressReport` → `ActivityProgressState`. Not
## keyed: `client_seq` makes it idempotent and every report carries all watched ranges, so a lost one loses nothing.
## Returns the state, or {} with `last_error_code` (e.g. `VIDEO_SEEK_NOT_ALLOWED`).
func report_progress(report: Dictionary) -> Dictionary:
	if attempt == null or report.is_empty():
		return {}
	var result: ApiResult = await _send(ApiRequest.make_post("%s/%s/progress" % [PATH_ATTEMPTS, attempt.attempt_id], report))
	if not result.ok:
		last_error_code = result.code if not result.code.is_empty() else ApiResult.CODE_INVALID_RESPONSE
		if result.status == 404 or (result.status == 403 and last_error_code in ACCESS_DENIED_CODES):
			_failed(result)
		return {}
	last_error_code = ""
	attempt.progress = result.body()
	return result.body()


## Coroutine: one run of a simulation (F066-AC-22): `SimulationRunInput` → `SimulationRunResult`. The same run keeps
## its `Idempotency-Key` until the server answered (a retry after a network loss is the same run, never a new one).
## Returns the result, or {} with `last_error_code` (`SIMULATION_LOG_INVALID`, `SIMULATION_RUN_LIMIT_REACHED`, network).
func submit_simulation_run(run: Dictionary) -> Dictionary:
	if attempt == null or run.is_empty():
		return {}
	var signature: String = "simrun|%s|%s" % [attempt.attempt_id, JSON.stringify(run)]
	var req: ApiRequest = ApiRequest.make_post("%s/%s/simulation-runs" % [PATH_ATTEMPTS, attempt.attempt_id], run)
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if result.is_transient():
		last_error_code = result.code
		_set_offline(true)
		return {}
	_forget(signature)
	_set_offline(false)
	if not result.ok:
		last_error_code = result.code if not result.code.is_empty() else ApiResult.CODE_INVALID_RESPONSE
		if result.status == 404 or (result.status == 403 and last_error_code in ACCESS_DENIED_CODES):
			_failed(result)
		return {}
	last_error_code = ""
	return result.body()


## Coroutine: pauses the attempt when the student leaves (F063-AC-07). Best effort: returns false on failure.
func pause() -> bool:
	if attempt == null or not attempt.is_in_progress():
		return false
	var signature: String = "pause|" + attempt.attempt_id
	var req: ApiRequest = ApiRequest.make_post("%s/%s:pause" % [PATH_ATTEMPTS, attempt.attempt_id])
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if result.is_transient():
		return false
	_forget(signature)
	if not result.ok:
		_failed(result)
		return false
	var paused: AttemptModel = AttemptModel.from_dict(result.body(), _now_unix())
	if paused != null:
		attempt = paused
	return true


## Coroutine: submits the attempt (F063-AC-08). Queued answers are sent first; `409 ATTEMPT_ALREADY_SUBMITTED` is a
## success (the attempt is reloaded). Returns the submitted attempt or null.
func submit() -> AttemptModel:
	if attempt == null:
		return null
	if not pending.is_empty():
		await flush_pending()
		if not pending.is_empty():
			last_error_code = ApiResult.CODE_NETWORK_ERROR
			return null
	var attempt_id: String = attempt.attempt_id
	var signature: String = "submit|" + attempt_id
	var req: ApiRequest = ApiRequest.make_post("%s/%s:submit" % [PATH_ATTEMPTS, attempt_id])
	req.idempotency_key = _key(signature)
	var result: ApiResult = await _send(req)
	if result.is_transient():
		last_error_code = result.code
		return null
	_forget(signature)
	if not result.ok and result.status == 409 and result.code == CODE_ALREADY_SUBMITTED:
		result = await _send(ApiRequest.make_get("%s/%s" % [PATH_ATTEMPTS, attempt_id]))
	if not result.ok:
		_failed(result)
		return null
	var rewards: Variant = result.body().get("rewards")
	last_rewards = (rewards as Array).filter(func(r: Variant) -> bool: return r is Dictionary) if rewards is Array else []
	var submitted: AttemptModel = AttemptModel.from_dict(result.body(), _now_unix())
	if submitted == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	attempt = submitted
	last_error_code = ""
	attempt_submitted.emit(submitted)
	return submitted


## Coroutine: the review of the current attempt (F063-AC-21). Null when not available (e.g. results not released).
func review() -> AttemptReviewModel:
	if attempt == null:
		return null
	var result: ApiResult = await _send(ApiRequest.make_get("%s/%s/review" % [PATH_ATTEMPTS, attempt.attempt_id]))
	if not result.ok:
		_failed(result)
		return null
	last_error_code = ""
	return AttemptReviewModel.from_dict(result.body())


## Coroutine: a lesson shown read-only as a worked example (F063-AC-17). Empty on failure.
func get_lesson(lesson_id: String) -> Dictionary:
	if not Uuid.is_valid(lesson_id):
		return {}
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_LESSON % lesson_id))
	if not result.ok:
		_failed(result)
		return {}
	return result.body()


## Drops the attempt and the queued writes (player closed after submit, session end).
func clear() -> void:
	attempt = null
	pending.clear()
	_keys.clear()
	last_rewards = []
	last_error_code = ""
	_set_offline(false)


# ---------- telemetry (codes only, no PII, AGENTS R14) ----------

func report_update_required(activity_type: String) -> void:
	if telemetry != null:
		telemetry.queue_client_error(TELEMETRY_UPDATE_REQUIRED, activity_type.left(TelemetryService.CODE_MAX_LENGTH))


func report_minigame_update_required(minigame_type: String) -> void:
	if telemetry != null:
		telemetry.queue_client_error(TELEMETRY_MINIGAME_UPDATE_REQUIRED, minigame_type.left(TelemetryService.CODE_MAX_LENGTH))


func report_math_failed() -> void:
	if telemetry != null:
		telemetry.queue_client_error(TELEMETRY_MATH_FAILED, SCENE_CODE)


func report_open_time(ms: int) -> void:
	if telemetry != null:
		telemetry.queue_event(TelemetryService.TYPE_LOAD_TIME, {"kind": TELEMETRY_OPEN_KIND, "ms": maxi(0, ms)})


# ---------- internals ----------

func _send(req: ApiRequest) -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	return await api.send(req)


func _loaded(result: ApiResult) -> AttemptModel:
	var loaded: AttemptModel = AttemptModel.from_dict(result.body(), _now_unix())
	if loaded == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	if attempt == null or attempt.attempt_id != loaded.attempt_id:
		pending.clear()
		_set_offline(false)
	attempt = loaded
	last_error_code = ""
	attempt_loaded.emit(loaded)
	return loaded


func _graded(result: ApiResult) -> ResponseResultModel:
	var graded: ResponseResultModel = ResponseResultModel.from_dict(result.body())
	last_error_code = ""
	response_result_received.emit(graded)
	return graded


func _queue(req: ApiRequest, position: int) -> void:
	pending.enqueue(req, clock.now_msec(), {"position": position})
	_set_offline(true)


## Records a failure; a 404 or a session-ending 403 drops the attempt (no data of it may stay on screen).
func _failed(result: ApiResult) -> void:
	last_error_code = result.code if not result.code.is_empty() else ApiResult.CODE_INVALID_RESPONSE
	last_trace_id = result.trace_id
	if result.status == 404:
		attempt = null
		pending.clear()
	if result.status == 403 and last_error_code in ACCESS_DENIED_CODES:
		attempt = null
		pending.clear()
		access_denied.emit(last_error_code)
	attempt_failed.emit(last_error_code)


func _set_offline(value: bool) -> void:
	if offline != value:
		offline = value
		offline_changed.emit(value)


func _key(signature: String) -> String:
	if not _keys.has(signature):
		_keys[signature] = Uuid.v7()
	return String(_keys[signature])


func _forget(signature: String) -> void:
	_keys.erase(signature)


func _forget_key(key: String) -> void:
	for signature: Variant in _keys.keys():
		if String(_keys[signature]) == key:
			_keys.erase(signature)
			return


func _now_unix() -> float:
	return clock.unix_time() if clock != null else Time.get_unix_time_from_system()


static func _uuid(args: Dictionary, key: String) -> String:
	var v: Variant = args.get(key)
	return v as String if v is String and Uuid.is_valid(v as String) else ""


static func _put_uuid(body: Dictionary, key: String, args: Dictionary) -> void:
	var v: String = _uuid(args, key)
	if not v.is_empty():
		body[key] = v


static func _put_enum(body: Dictionary, key: String, args: Dictionary, allowed: PackedStringArray) -> void:
	var v: Variant = args.get(key)
	if v is String and (v as String) in allowed:
		body[key] = v
