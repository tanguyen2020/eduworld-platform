class_name ExamService
extends RefCounted
## Timed exams of the Exam Zone (F064, EXAM_ENGINE §6–§8, FR-EXM-003/005): `/exams/*` through ApiClient (session
## `X-Tenant-Id`, refresh, retries — F061). Server-authoritative (ADR-0016, AGENTS R5/R6): the server sets the deadline,
## closes the attempt and grades it; this service only sends raw answers, shows the server countdown (`ExamCountdown`)
## and asks for the result. The exam and the answers live in RAM only — never on disk (GODOT_ARCHITECTURE §8.2):
## answers that cannot reach the server wait in `PendingWrites` (latest per position) and are sent again with their
## `answered_at_client` (server clock) once the network is back (F064-AC-14). Start and submit carry an
## `Idempotency-Key` (UUIDv7) kept on retry (AGENTS R10); every exam call carries the install's exam device token
## (`X-Exam-Device-Token`, EXAM_ENGINE §12). Answers are never logged (AGENTS R14).

signal session_loaded(session: ExamSessionModel)
signal exam_started(attempt: AttemptModel)
## Submitted by the student, by the time-up flow or found already submitted (`409 EXAM_ALREADY_SUBMITTED`).
signal exam_submitted(result: Dictionary)
## `minutes` left reached (5, 1): shown once (F064-AC-04).
signal time_warning_reached(minutes: int)
## `deadline_at` reached on the server clock: input is locked (F064-AC-12).
signal time_expired()
## `422 EXAM_TIME_OVER`: the server closed the attempt; no retry (F064-AC-13).
signal attempt_closed()
## `409 EXAM_ACTIVE_ON_OTHER_DEVICE` while answering (F064-AC-16).
signal other_device_detected()
signal offline_changed(offline: bool)
signal answer_saved(position: int)
## A refusal that ends the session (`403 GAME_ACCESS_STUDENT_ONLY`…, F064-AC-10): handled like F061/F062.
signal access_denied(code: String)

const PATH_SESSIONS: String = "/exams/sessions"
const PATH_SESSION: String = "/exams/sessions/%s"
const PATH_START: String = "/exams/sessions/%s/attempts"
const PATH_ANSWER: String = "/exams/attempts/%s/answers/%d"
const PATH_SUBMIT: String = "/exams/attempts/%s:submit"
const PATH_RESULT: String = "/exams/attempts/%s/result"
const HEADER_DEVICE_TOKEN: String = "X-Exam-Device-Token"
const CODE_NOT_FOUND: String = "NOT_FOUND"
const CODE_TIME_OVER: String = "EXAM_TIME_OVER"
const CODE_ALREADY_SUBMITTED: String = "EXAM_ALREADY_SUBMITTED"
const CODE_OTHER_DEVICE: String = "EXAM_ACTIVE_ON_OTHER_DEVICE"
const CODE_RESULTS_NOT_RELEASED: String = "EXAM_RESULTS_NOT_RELEASED"
## `Limit` parameter (API_DESIGN pagination); pages are followed up to MAX_PAGES.
const PAGE_LIMIT: int = 50
const MAX_PAGES: int = 10
## Re-send of queued answers while offline (ApiClient already backs off inside one call).
const OFFLINE_RETRY_MS: int = 5_000
## Session-ending refusals (same list as AssignmentService, F062-AC-12); `TENANT_MISMATCH` is ApiClient's.
const ACCESS_DENIED_CODES: PackedStringArray = LearningService.ACCESS_DENIED_CODES

var api: RestApiClient = null
var clock: ClientClock = null
var sessions: Array[ExamSessionModel] = []
var session: ExamSessionModel = null
## Current attempt (RAM only); null after a 404 / access refusal so nothing of it can be shown (F064-AC-10).
var attempt: AttemptModel = null
var countdown: ExamCountdown = ExamCountdown.new()
var pending: PendingWrites = PendingWrites.new()
var offline: bool = false
## No more answers are taken (time up, submitted, closed by the server, another device).
var locked: bool = false
var submitted: bool = false
## `AttemptResult` of the submit (status only — the score is shown once results are released).
var submit_result: Dictionary = {}
## Code of the last failure (`NETWORK_ERROR`, `HTTP_503`, Problem code…), empty after a success.
var last_error_code: String = ""
var last_trace_id: String = ""
## position → latest response of this device (RAM): restores an answer when the student comes back to a question.
var _responses: Dictionary = {}
## Idempotency keys of writes not yet answered by the server, by write signature (kept on retry, AGENTS R10).
var _keys: Dictionary = {}
var _time_up_emitted: bool = false
var _flushing: bool = false
var _last_flush_ms: int = 0


func _init(rest: RestApiClient = null, time_source: ClientClock = null) -> void:
	api = rest
	clock = time_source if time_source != null else ClientClock.new()


# ---------- sessions ----------

## Coroutine: the exam sessions of the student (`listExamSessions`, every page). Keeps the previous list on failure.
func list_sessions() -> Array[ExamSessionModel]:
	var collected: Array[ExamSessionModel] = []
	var cursor: String = ""
	for page: int in MAX_PAGES:
		var query: Dictionary = {"limit": PAGE_LIMIT}
		if not cursor.is_empty():
			query["cursor"] = cursor
		var result: ApiResult = await _send(ApiRequest.make_get(PATH_SESSIONS, query))
		if not result.ok or not result.body().get("data") is Array:
			_failed(result)
			return sessions
		collected.append_array(ExamSessionModel.list_from(result.body()["data"] as Array))
		var info: Variant = result.body().get("page")
		var next: Variant = (info as Dictionary).get("next_cursor") if info is Dictionary else null
		if not (info is Dictionary and bool((info as Dictionary).get("has_more", false)) and next is String):
			break
		cursor = next as String
	sessions = collected
	last_error_code = ""
	return sessions


## Coroutine: the rules of one exam (`getExamSession`, F064-AC-01). Null on failure (`last_error_code`).
func load_session(exam_session_id: String) -> ExamSessionModel:
	session = null
	if not Uuid.is_valid(exam_session_id):
		_failed(ApiResult.failure(CODE_NOT_FOUND))
		return null
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_SESSION % exam_session_id))
	if not result.ok:
		_failed(result)
		return null
	var loaded: ExamSessionModel = ExamSessionModel.from_dict(result.body())
	if loaded == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	session = loaded
	last_error_code = ""
	session_loaded.emit(loaded)
	return loaded


# ---------- attempt ----------

## Coroutine: starts the attempt — or gets back the one in progress with its saved answers and unchanged deadline
## (resume, F064-AC-15). `take_over` moves an attempt open on another device to this one (EXAM_ENGINE §12, a new
## `Idempotency-Key` since the body differs). Null on failure (`last_error_code`, e.g. `EXAM_NOT_OPEN`).
func start(exam_session_id: String, take_over: bool = false) -> AttemptModel:
	if not Uuid.is_valid(exam_session_id):
		_failed(ApiResult.failure(CODE_NOT_FOUND))
		return null
	var req: ApiRequest = ApiRequest.make_post(PATH_START % exam_session_id, {"take_over": true} if take_over else null)
	var signature: String = "start|%s|%s" % [exam_session_id, take_over]
	req.idempotency_key = _key(signature)
	_with_device(req)
	var result: ApiResult = await _send(req)
	if not result.is_transient():
		_forget(signature)
	if not result.ok:
		_failed(result)
		return null
	var loaded: AttemptModel = AttemptModel.from_dict(result.body(), clock.unix_time())
	if loaded == null:
		_failed(ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE))
		return null
	_begin(loaded)
	exam_started.emit(loaded)
	return loaded


## Coroutine: saves the answer of `position` (`saveExamAnswer`, F064-AC-05) — the raw `QuestionResponse` only: no score,
## no grading, no client-computed time (F064-AC-11, ADR-0016). Offline, the latest answer per position waits in RAM
## with its `answered_at_client` on the server clock (F064-AC-14). Returns true once the server stored it.
func save_answer(position: int, response: Dictionary) -> bool:
	if attempt == null or locked or response.is_empty():
		return false
	_responses[position] = response.duplicate(true)
	var body: Dictionary = {"kind": "question", "position": position, "response": response}
	var item: QuestionRenderModel = attempt.item_at(position)
	if item != null and Uuid.is_valid(item.question_version_id):
		body["question_version_id"] = item.question_version_id
	var req: ApiRequest = ApiRequest.make_put(PATH_ANSWER % [attempt.attempt_id, position], body)
	_with_device(req)
	if not pending.is_empty():
		_queue(req, position)
		return false
	var result: ApiResult = await _send(req)
	if result.is_transient():
		_queue(req, position)
		last_error_code = result.code
		return false
	if not result.ok:
		_refused(result)
		return false
	last_error_code = ""
	answer_saved.emit(position)
	return true


## Coroutine: re-sends the queued answers in order (network back). Stops at the first one still failing transiently;
## `EXAM_TIME_OVER` closes the attempt and drops the rest (F064-AC-13). Returns the number stored by the server.
func flush_pending() -> int:
	if _flushing or pending.is_empty() or api == null:
		return 0
	_flushing = true
	_last_flush_ms = clock.now_msec()
	var stored: int = 0
	while not pending.is_empty():
		var write: PendingWrites.PendingWrite = pending.peek()
		var result: ApiResult = await _send(write.to_request())
		if result.is_transient():
			break
		pending.drop_first()
		if result.ok:
			stored += 1
			answer_saved.emit(int(write.tag.get("position", 0)))
			continue
		_refused(result)
		if locked:
			pending.clear()
			break
	_flushing = false
	_set_offline(not pending.is_empty())
	return stored


## Coroutine: submits the attempt (F064-AC-07): queued answers first, then `submitExamAttempt` with a key kept on
## retry. `409 EXAM_ALREADY_SUBMITTED` counts as submitted (F064-AC-08), `422 EXAM_TIME_OVER` as closed by the server
## (F064-AC-13). Returns false while the network is down (the student can try again; the clock keeps running).
func submit() -> bool:
	if attempt == null or submitted:
		return submitted
	await flush_pending()
	if not pending.is_empty() and not locked:
		last_error_code = ApiResult.CODE_NETWORK_ERROR
		return false
	return await _submit_once()


## Coroutine: time up (F064-AC-12): input is already locked; queued answers are sent (the server takes those answered
## before `deadline_at` within `exam_offline_sync_grace`), then one submit, best effort — the server auto-submits
## anyway and decides the score.
func auto_submit() -> void:
	locked = true
	if attempt == null or submitted:
		return
	await flush_pending()
	await _submit_once()


## Coroutine: the result of an attempt (`getExamResult`, F064-AC-17). Null with `EXAM_RESULTS_NOT_RELEASED` before the
## school releases it (or while it is being graded).
func result(attempt_id: String) -> ExamResultModel:
	if not Uuid.is_valid(attempt_id):
		_failed(ApiResult.failure(CODE_NOT_FOUND))
		return null
	var res: ApiResult = await _send(ApiRequest.make_get(PATH_RESULT % attempt_id))
	if not res.ok:
		_failed(res)
		return null
	last_error_code = ""
	return ExamResultModel.from_dict(res.body())


## Once a second while the runner is shown: warnings, time up, re-sending queued answers (F064-AC-03/04/12/14).
func tick() -> void:
	if attempt == null:
		return
	var now: int = clock.now_msec()
	if not locked:
		for minutes: int in countdown.due_warnings(now):
			time_warning_reached.emit(minutes)
	if countdown.has_deadline() and countdown.remaining_ms(now) <= 0 and not _time_up_emitted and not submitted:
		_time_up_emitted = true
		# Listeners first save the answer being typed (it was given before the deadline), then input is locked.
		time_expired.emit()
		locked = true
	if offline and not _flushing and not locked and now - _last_flush_ms >= OFFLINE_RETRY_MS:
		flush_pending()


## F123-AC-16: an attempt is open and not submitted yet (notification toasts wait; only the badge changes).
func in_progress() -> bool:
	return attempt != null and not submitted


func remaining_ms() -> int:
	return countdown.remaining_ms(clock.now_msec())


func total_ms() -> int:
	return (session.duration_for_student() if session != null else 0) * 1000


func is_answered(position: int) -> bool:
	return _responses.has(position)


## The latest answer of `position` on this device (or saved on the server), {} when none.
func response_of(position: int) -> Dictionary:
	return (_responses.get(position, {}) as Dictionary).duplicate(true)


func answered_positions() -> Array[int]:
	var out: Array[int] = []
	for p: Variant in _responses.keys():
		out.append(int(p))
	out.sort()
	return out


## Drops the exam and the queued answers (runner closed after submit, session end).
func clear() -> void:
	attempt = null
	session = null
	sessions.clear()
	pending.clear()
	_keys.clear()
	_responses.clear()
	submit_result = {}
	locked = false
	submitted = false
	_time_up_emitted = false
	last_error_code = ""
	_set_offline(false)


# ---------- internals ----------

func _begin(loaded: AttemptModel) -> void:
	if attempt == null or attempt.attempt_id != loaded.attempt_id:
		pending.clear()
		_responses.clear()
		_set_offline(false)
	attempt = loaded
	submitted = loaded.is_finished()
	locked = submitted
	_time_up_emitted = false
	submit_result = {}
	for position: Variant in loaded.saved.keys():
		if not _responses.has(position):
			_responses[int(position)] = loaded.saved_response(int(position))
	countdown.sync(loaded.deadline_at, loaded.server_time, clock.now_msec(), clock.unix_time())
	countdown.prime(clock.now_msec())
	last_error_code = ""


func _submit_once() -> bool:
	var attempt_id: String = attempt.attempt_id
	var signature: String = "submit|" + attempt_id
	var req: ApiRequest = ApiRequest.make_post(PATH_SUBMIT % attempt_id)
	req.idempotency_key = _key(signature)
	_with_device(req)
	var res: ApiResult = await _send(req)
	if res.is_transient():
		last_error_code = res.code
		return false
	_forget(signature)
	if res.ok or res.code == CODE_ALREADY_SUBMITTED or res.code == CODE_TIME_OVER:
		_mark_submitted(res.body() if res.ok else {"attempt_id": attempt_id, "status": AttemptModel.STATUS_SUBMITTED})
		if res.code == CODE_TIME_OVER:
			attempt_closed.emit()
		return true
	_refused(res)
	return false


func _mark_submitted(body: Dictionary) -> void:
	submitted = true
	locked = true
	pending.clear()
	submit_result = body
	last_error_code = ""
	_set_offline(false)
	exam_submitted.emit(body)


## A definitive refusal of an answer/submit: the attempt is closed for the codes that end it.
func _refused(result: ApiResult) -> void:
	match result.code:
		CODE_TIME_OVER:
			locked = true
			last_error_code = result.code
			pending.clear()
			attempt_closed.emit()
			return
		CODE_ALREADY_SUBMITTED:
			_mark_submitted({"attempt_id": attempt.attempt_id if attempt != null else "", "status": AttemptModel.STATUS_SUBMITTED})
			return
		CODE_OTHER_DEVICE:
			locked = true
			last_error_code = result.code
			pending.clear()
			other_device_detected.emit()
			return
	_failed(result)


func _queue(req: ApiRequest, position: int) -> void:
	var body: Dictionary = (req.body as Dictionary).duplicate(true)
	# EXAM_ENGINE §6: an answer sent late keeps the (server-clock) time it was given; the server accepts it only before
	# `deadline_at` and within `exam_offline_sync_grace`.
	body["answered_at_client"] = Time.get_datetime_string_from_unix_time(countdown.server_now_ms(clock.now_msec()) / 1000) + "Z"
	req.body = body
	pending.enqueue_latest(req, clock.now_msec(), "answer|%d" % position, {"position": position})
	if not offline:
		_last_flush_ms = clock.now_msec()
	_set_offline(true)


## Records a failure; a 404 or a session-ending 403 drops the exam (no data of it may stay on screen).
func _failed(result: ApiResult) -> void:
	last_error_code = result.code if not result.code.is_empty() else ApiResult.CODE_INVALID_RESPONSE
	last_trace_id = result.trace_id
	if result.status == 404 or (result.status == 403 and last_error_code in ACCESS_DENIED_CODES):
		attempt = null
		session = null
		pending.clear()
		_responses.clear()
		locked = true
	if result.status == 403 and last_error_code in ACCESS_DENIED_CODES:
		access_denied.emit(last_error_code)


func _with_device(req: ApiRequest) -> void:
	if api != null and not api.device_id.is_empty():
		req.headers[HEADER_DEVICE_TOKEN] = api.device_id


func _send(req: ApiRequest) -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	return await api.send(req)


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
