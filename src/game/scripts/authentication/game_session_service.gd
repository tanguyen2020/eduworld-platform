class_name GameSessionService
extends RefCounted
## Game session of this device (module game, ADR-0031, ADR-0083; F061-AC-04/21/22/24/28): `createGameSession`
## right after login/tenant:select/resume with a fresh UUIDv7 `Idempotency-Key` per logical call (kept by
## ApiClient on network retries; a new key for `takeover: true` — a different body with the old key would get
## `409 IDEMPOTENCY_KEY_REUSED`); REST heartbeat every `ws_heartbeat` while RealtimeClient is not Online — when it is
## Online the heartbeat goes over the WebSocket and the REST call is skipped (F092-AC-09; the tick still runs for the
## curfew checks); `:end` (best effort). Heartbeat `409`/`404` or `session.session.replaced` (F092) →
## `session_replaced`.

signal session_replaced(code: String)
signal heartbeat_ticked()

## CONFIG_DEFAULTS `ws_heartbeat` / `ws_timeout` [Proposed] — reused for the REST heartbeat until a dedicated
## parameter exists (spec Assumptions).
const WS_HEARTBEAT_SEC: float = 15.0
const WS_TIMEOUT_SEC: float = 45.0
const PATH_SESSIONS: String = "/game/sessions"
const CODE_ACTIVE_SESSION: String = "ACTIVE_SESSION_ON_OTHER_DEVICE"

var api: RestApiClient = null
var clock: ClientClock = null
var client_version: String = ""
var platform: String = ""
var heartbeat_interval_sec: float = WS_HEARTBEAT_SEC
var game_session_id: String = ""
## Set by AuthService from RealtimeClient: true while the WebSocket is Online (no REST heartbeat).
var realtime_online: bool = false
## REST heartbeats sent (tests).
var rest_beats: int = 0
## Idempotency-Key of the last createGameSession call (tests check it changes for takeover).
var last_idempotency_key: String = ""
var _heartbeat_generation: int = 0


func _init(rest: RestApiClient, time_source: ClientClock, version: String, client_platform: String) -> void:
	api = rest
	clock = time_source
	client_version = version
	platform = client_platform


func has_session() -> bool:
	return not game_session_id.is_empty()


## Coroutine: `POST /game/sessions` (`GameSessionCreate`). On 201 stores `game_session_id`.
func create(takeover: bool) -> ApiResult:
	last_idempotency_key = Uuid.v7()
	var body: Dictionary = {"client_platform": platform, "client_version": client_version, "takeover": takeover}
	var req: ApiRequest = ApiRequest.make_post(PATH_SESSIONS, body)
	req.idempotency_key = last_idempotency_key
	var result: ApiResult = await api.send(req)
	if result.ok:
		var id: Variant = result.body().get("game_session_id")
		if id is String and Uuid.is_valid(id as String):
			game_session_id = id as String
		else:
			result.ok = false
			result.code = ApiResult.CODE_INVALID_RESPONSE
	return result


func start_heartbeat() -> void:
	_heartbeat_generation += 1
	_heartbeat_loop(_heartbeat_generation)


func stop_heartbeat() -> void:
	_heartbeat_generation += 1


## Coroutine: one `heartbeatGameSession` (`X-Device-Id` is a standard header). 409/404 → session replaced.
func beat() -> void:
	if not has_session():
		return
	var id: String = game_session_id
	rest_beats += 1
	var result: ApiResult = await api.send(ApiRequest.make_post("%s/%s:heartbeat" % [PATH_SESSIONS, id]))
	if id != game_session_id:
		return
	if result.status == 409 or result.status == 404:
		stop_heartbeat()
		game_session_id = ""
		session_replaced.emit(result.code)


## Coroutine: `endGameSession` (best effort, never retried); forgets the session either way.
func end() -> void:
	stop_heartbeat()
	if not has_session():
		return
	var id: String = game_session_id
	game_session_id = ""
	await api.send(ApiRequest.make_post("%s/%s:end" % [PATH_SESSIONS, id]))


## The realtime connection reported the takeover (`session.session.replaced` + close 4409, F092-AC-04) or the ticket
## endpoint answered that the session is gone: same path as a heartbeat 409/404.
func notify_replaced(code: String) -> void:
	if not has_session():
		return
	stop_heartbeat()
	game_session_id = ""
	session_replaced.emit(code)


## Forgets the session without calling the server (it was ended or replaced server-side).
func clear() -> void:
	stop_heartbeat()
	game_session_id = ""


func _heartbeat_loop(generation: int) -> void:
	while generation == _heartbeat_generation:
		await clock.wait(heartbeat_interval_sec)
		if generation != _heartbeat_generation:
			return
		if not realtime_online:
			await beat()
		if generation == _heartbeat_generation:
			heartbeat_ticked.emit()
