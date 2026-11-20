class_name GameAuthService
extends Node
## Autoload #5 `AuthService` (GODOT_ARCHITECTURE §4–§5, GAME_NETWORKING §3, F061). Application layer of the entry
## flow; scenes only call its intents and read its state (AGENTS R1).
## - VersionCheck → Resume (stored session, `refreshGameToken mode=resume`) → ResumeConfirm, or the S01 form;
## - `gameLogin` (in-game form, ADR-0068) → `TENANT_NOT_SELECTED` → `selectGameLoginTenant` (no password);
## - `createGameSession` right after (ADR-0083) → `409` takeover confirm (`takeover: true`, new key, no password);
## - access token in RAM only; refresh token + tenant through `SessionStore`/SecureStorage on a personal device,
##   MemoryOnly on Web/shared device (ADR-0086); single-flight refresh with rotation (`mode=renew`);
## - Đăng xuất: `endGameSession` → `gameLogout` → erase RAM + SecureStorage; window close on a personal device only
##   ends the game session; heartbeat 409/404 → ReplacedNotice; curfew (ADR-0078) keeps the stored session.
## The password lives in `PasswordBuffer` only while `gameLogin` is in flight. Nothing secret is logged (R14).

signal state_changed(state: int)
signal login_succeeded()
signal login_failed(code: String)
signal tenant_choice_required(choices: Array)
signal takeover_required(other_device: ApiProblem.OtherDevice)
signal session_started(game_session_id: String)
signal session_resumed(nickname: String)
signal resume_failed(code: String)
signal session_replaced()
signal logout_completed()
signal curfew_warning_raised(minutes_left: int)
signal queue_waited(seconds: float)
signal connection_retried(attempt: int)
signal refresh_finished(ok: bool)

const STATE := GameStateMachine.State
const PATH_LOGIN: String = "/auth/game/login"
const PATH_TENANT_SELECT: String = "/auth/game/tenant:select"
const PATH_REFRESH: String = "/auth/game/token:refresh"
const PATH_LOGOUT: String = "/auth/game/logout"
## GODOT_ARCHITECTURE §9: maintenance screen checks again every 60 s.
const MAINTENANCE_RECHECK_SEC: float = 60.0
## Hard deadline for the best-effort calls made when the window closes.
const CLOSE_GRACE_SEC: float = 3.0
const TELEMETRY_REFRESH_REUSED: String = "auth_refresh_reused"
const TELEMETRY_RESUME: String = "auth_resume_result"
const CODE_CURFEW: String = "CURFEW_ACTIVE"
const CODE_NO_SESSION: String = "NO_STORED_SESSION"

## Splash starts the flow automatically; tests turn it off.
var autostart: bool = true
var api: RestApiClient = null
var clock: ClientClock = null
var rng: RandomNumberGenerator = RandomNumberGenerator.new()
var tokens: TokenStore = TokenStore.new()
var session_store: SessionStore = null
var sessions: GameSessionService = null
var version_check: VersionCheck = VersionCheck.new()
var form: LoginFormModel = LoginFormModel.new()
var curfew: CurfewWatcher = CurfewWatcher.new()
var lifecycle: LifecycleWatcher = null
## `Settings` autoload (or a test instance) holding `remember_username` / `last_username`.
var settings: Node = null
var device_id: String = ""
var client_version: String = ""
var platform: String = ""
var shared_device: bool = true
## `(state: int) -> void`; default changes the scene through SceneRouter.
var navigator: Callable = Callable()
## `() -> void`; default quits the SceneTree.
var quitter: Callable = Callable()
var state: int = STATE.BOOT

# Flow data read by the scenes.
var choices: Array[ApiProblem.TenantChoice] = []
var other_device: ApiProblem.OtherDevice = null
var nickname: String = ""
var notice_code: String = ""
var reopen_text: String = ""
var min_client_version: String = ""
var store_url: String = ""
## True while resume waits for the network ("Đang kết nối…").
var connecting: bool = false
var signed_in: bool = false

var _password: PasswordBuffer = PasswordBuffer.new()
var _login_ticket: String = ""
var _login_generation: int = 0
var _resume_generation: int = 0
var _refresh_in_flight: bool = false
var _curfew_warned: bool = false
var _quitting: bool = false
var _pending_scene: String = ""


func _ready() -> void:
	_setup_defaults.call_deferred()


## Test seam: wires fakes (FakeHttp through `rest`, FakeClock, FakeSecureStorage) and the device context.
func configure(rest: RestApiClient, time_source: ClientClock, storage: SecureStorage, settings_node: Node,
		install_id: String, version: String, client_platform: String, is_shared_device: bool) -> void:
	api = rest
	clock = time_source
	settings = settings_node
	device_id = install_id
	client_version = version
	platform = client_platform
	shared_device = is_shared_device
	session_store = SessionStore.new(storage)
	tokens = rest.tokens if rest.tokens != null else tokens
	rest.tokens = tokens
	rest.refresh_handler = refresh
	rest.auth_error_handler = _on_api_auth_error
	sessions = GameSessionService.new(rest, time_source, version, client_platform)
	sessions.session_replaced.connect(_on_session_replaced)
	sessions.heartbeat_ticked.connect(_on_heartbeat_ticked)
	_load_form_memory()


func _setup_defaults() -> void:
	if api != null:
		return
	# Autoloads by path (not by global name) so tool scripts can load this class before autoloads exist.
	var rest: RestApiClient = get_node("/root/ApiClient") as RestApiClient
	var config: Node = get_node("/root/Config")
	var version: String = String(config.get("client_version"))
	var id: String = DeviceIdentity.load_or_create()
	var client_platform: String = RestApiClient.client_platform()
	if rest.clock == null:
		rest.clock = ClientClock.new()
	rest.tokens = tokens
	rest.device_id = id
	rest.client_version = version
	rest.platform = client_platform
	configure(rest, rest.clock, SecureStorageFactory.create_default(), get_node("/root/Settings"), id, version,
			client_platform, bool(config.get("shared_device")))
	lifecycle = LifecycleWatcher.new()
	lifecycle.name = "LifecycleWatcher"
	add_child(lifecycle)
	lifecycle.close_requested.connect(close_game)
	lifecycle.app_resumed.connect(on_app_resumed)
	lifecycle.idle_limit_reached.connect(on_idle_limit_reached)
	var router: Node = get_node_or_null("/root/SceneRouter")
	if router != null:
		router.connect("scene_changed", _on_scene_changed)


# ---------- start-up: VersionCheck → Resume / Login ----------

## Coroutine: entry flow from Splash (F061-AC-01, F061-AC-29).
func start() -> void:
	_set_state(STATE.VERSION_CHECK, false)
	while true:
		var outcome: VersionCheck.Outcome = await version_check.run(api, platform, client_version)
		if outcome == VersionCheck.Outcome.FORCE_UPDATE:
			min_client_version = version_check.config.min_client_version
			_set_state(STATE.FORCE_UPDATE)
			return
		if outcome != VersionCheck.Outcome.MAINTENANCE:
			break
		_set_state(STATE.MAINTENANCE)
		await clock.wait(MAINTENANCE_RECHECK_SEC)
	if session_store.has_record():
		await _resume()
	else:
		_show_login("")


## Coroutine: resume with the stored refresh token (ADR-0086). 200 → ResumeConfirm; fatal → form; transient →
## keep the record, "Đang kết nối…", retry with backoff; 426 → ForceUpdate; CURFEW_ACTIVE → CurfewNotice.
func _resume() -> void:
	var record: Dictionary = session_store.load_record()
	if record.is_empty():
		_show_login("")
		return
	tokens.clear()
	tokens.refresh_token = String(record["refresh_token"])
	tokens.tenant_id = String(record["tenant_id"])
	_resume_generation += 1
	var generation: int = _resume_generation
	connecting = false
	_set_state(STATE.RESUMING)
	var attempt: int = 0
	while generation == _resume_generation:
		var result: ApiResult = await _call_refresh("resume")
		if generation != _resume_generation:
			return
		if result.ok:
			connecting = false
			if not _apply_bootstrap(result.body().get("bootstrap")):
				_end_stored_session(ApiResult.CODE_INVALID_RESPONSE)
				_show_login("REFRESH_TOKEN_INVALID")
				return
			_telemetry(TELEMETRY_RESUME, {"result": "resumed"})
			_set_state(STATE.RESUME_CONFIRM)
			session_resumed.emit(nickname)
			return
		if result.code == CODE_CURFEW:
			connecting = false
			_enter_curfew(result)
			return
		if result.status == 426:
			connecting = false
			_enter_force_update(result)
			return
		if GameStateMachine.is_refresh_fatal(result.status, result.code) or not result.is_transient():
			connecting = false
			_telemetry(TELEMETRY_RESUME, {"result": "expired", "code": result.code})
			_end_stored_session(result.code)
			resume_failed.emit(result.code)
			_show_after_fatal(result.code)
			return
		attempt += 1
		connecting = true
		_telemetry(TELEMETRY_RESUME, {"result": "network", "code": result.code})
		connection_retried.emit(attempt)
		var wait: float = result.retry_after_sec(Backoff.delay(mini(attempt, 6), rng)) if result.status == 429 \
				else Backoff.delay(mini(attempt, 6), rng)
		await clock.wait(wait)


## S01b "Tiếp tục" (F061-AC-35): create the game session.
func continue_resume() -> void:
	await _start_game_session(false)


## S01b "Không phải tôi" (F061-AC-35) and ResumeSplash "Đăng nhập bằng tài khoản khác" (F061-AC-31): stop the resume
## retries, log out best effort, erase the stored session and show an empty form (no username of the previous
## student).
func switch_account() -> void:
	_resume_generation += 1
	connecting = false
	await _remote_logout()
	_clear_all(true)
	form.reset_for_new_user()
	logout_completed.emit()
	_set_state(STATE.LOGIN)


# ---------- login form ----------

## Coroutine: `gameLogin` (F061-AC-03…09). `username` is trimmed; the password goes to PasswordBuffer only.
func login(username: String, password: String) -> void:
	if form.busy or not LoginFormModel.can_submit(username, password):
		return
	_password.set_text(password)
	form.begin_submit(username)
	_login_generation += 1
	var generation: int = _login_generation
	_set_state(STATE.SIGNING_IN, false)
	var result: ApiResult = null
	while true:
		var body: Dictionary = {
			"username": form.username,
			"password": _password.text(),
			"device_id": device_id,
			"client_version": client_version,
			"platform": platform,
			"locale": RestApiClient.accept_language(),
		}
		var req: ApiRequest = ApiRequest.make_post(PATH_LOGIN, body).as_public()
		req.max_rate_limit_retries = 0
		result = await api.send(req)
		req.body = null
		body.clear()
		if generation != _login_generation:
			_password.wipe()
			return
		if result.status == 429 and result.code == "LOGIN_QUEUE_BUSY":
			var wait: float = Backoff.queue_delay(result.retry_after_sec(), rng)
			_set_state(STATE.LOGIN_WAITING)
			queue_waited.emit(wait)
			await clock.wait(wait)
			if generation != _login_generation:
				_password.wipe()
				return
			continue
		break
	_password.wipe()
	if result.ok:
		var data: Dictionary = result.body()
		if _accept_session(data.get("tokens"), data.get("bootstrap")):
			form.end_submit("")
			_remember_username()
			login_succeeded.emit()
			await _start_game_session(false)
			return
		result.code = ApiResult.CODE_INVALID_RESPONSE
	_handle_login_failure(result)


## LoginWaiting "Hủy" (F061-AC-06): stop the queue retries and wipe the password.
func cancel_login() -> void:
	_login_generation += 1
	_password.wipe()
	form.end_submit("")
	_set_state(STATE.LOGIN)


## True while a password is held (tests: only during an in-flight gameLogin).
func holds_password() -> bool:
	return not _password.is_empty()


func set_remember_username(enabled: bool) -> void:
	form.remember_username = enabled and not shared_device
	if settings != null and not shared_device and not form.remember_username:
		settings.call("set_login_memory", false, "")


# ---------- tenant choice ----------

## Coroutine: `selectGameLoginTenant` with the one-time `login_ticket` (F061-AC-13/14). No password.
func select_tenant(tenant_id: String) -> void:
	if _login_ticket.is_empty():
		_show_login("LOGIN_TICKET_INVALID")
		return
	var ticket: String = _login_ticket
	_login_ticket = ""
	form.begin_submit(form.username)
	var body: Dictionary = {"login_ticket": ticket, "tenant_id": tenant_id, "device_id": device_id}
	var req: ApiRequest = ApiRequest.make_post(PATH_TENANT_SELECT, body).as_public()
	req.max_rate_limit_retries = 0
	var result: ApiResult = await api.send(req)
	req.body = null
	choices.clear()
	if result.ok:
		var data: Dictionary = result.body()
		if _accept_session(data.get("tokens"), data.get("bootstrap")):
			form.end_submit("")
			_remember_username()
			login_succeeded.emit()
			await _start_game_session(false)
			return
		result.code = ApiResult.CODE_INVALID_RESPONSE
	if result.code == "LOGIN_TICKET_INVALID" or result.code == "TENANT_MISMATCH":
		_show_login("LOGIN_TICKET_INVALID")
		return
	_handle_login_failure(result)


# ---------- game session ----------

## TakeoverConfirm "Tiếp tục ở đây" (F061-AC-21): same tokens, `takeover: true`, new Idempotency-Key.
func confirm_takeover() -> void:
	await _start_game_session(true)


## TakeoverConfirm "Ở lại máy kia" (F061-AC-21): revoke the tokens just issued and go back to the form.
func decline_takeover() -> void:
	await _remote_logout()
	_clear_all(true)
	_show_login("", true)


## ReplacedNotice, account notices, TenantChoice "Đăng xuất": forget the ticket/choices and go back to the form.
func back_to_login() -> void:
	_clear_all(false)
	_show_login("", true)


# ---------- refresh (single-flight) ----------

## Coroutine: single-flight `refreshGameToken mode=renew` (F061-AC-18, F061-AC-33). Every caller waits for the
## same call; a replaced refresh token is never sent again. Returns true when a valid access token is available.
func refresh(used_access_token: String = "") -> bool:
	if _refresh_in_flight:
		return bool(await refresh_finished)
	if not used_access_token.is_empty() and used_access_token != tokens.access_token \
			and not tokens.access_expiring(clock.now_msec()):
		return true
	if tokens.refresh_token.is_empty():
		return false
	_refresh_in_flight = true
	var result: ApiResult = await _call_refresh("renew")
	var ok: bool = result.ok
	if not ok:
		if result.code == CODE_CURFEW:
			_enter_curfew(result)
		elif GameStateMachine.is_refresh_fatal(result.status, result.code):
			_expire_session(result.code)
	_refresh_in_flight = false
	refresh_finished.emit(ok)
	return ok


# ---------- logout and lifecycle ----------

## Coroutine: Đăng xuất (F061-AC-23): `endGameSession` first (best effort), then `gameLogout` with the refresh
## token, then erase RAM + SecureStorage and show the form.
func logout() -> void:
	await sessions.end()
	await _remote_logout()
	_clear_all(true)
	logout_completed.emit()
	_show_login("", true)


## Window close (F061-AC-23/26/32): personal device → `endGameSession` only, keep the stored session; shared
## device or Web → log out. Always quits within CLOSE_GRACE_SEC. Also the CurfewNotice "Đóng" button.
func close_game() -> void:
	if _quitting:
		return
	_quitting = true
	_quit_after_grace()
	if shared_device or not session_store.is_persistent():
		if signed_in or not tokens.refresh_token.is_empty():
			await sessions.end()
			await _remote_logout()
		_clear_all(true)
	else:
		await sessions.end()
		_clear_all(false)
	_quit_now()


## Mobile back from background (F061-AC-25): a session older than `ws_timeout` was closed by the server → create a
## new one with the kept refresh token (no password); otherwise send a heartbeat now.
func on_app_resumed(paused_sec: float) -> void:
	if not signed_in:
		return
	if paused_sec >= GameSessionService.WS_TIMEOUT_SEC or not sessions.has_session():
		sessions.clear()
		await _start_game_session(false)
	else:
		await sessions.beat()


## Shared device idle for `shared_device_idle_logout` (F061-AC-25) → log out.
func on_idle_limit_reached() -> void:
	if shared_device and signed_in:
		await logout()


## WebSocket `session.session.ended` (F092, AsyncAPI): `curfew` → CurfewNotice keeping the stored session;
## other reasons (close 4403, access revoked) → erase and show the form.
func handle_session_ended(reason: String, reopen_at: String = "") -> void:
	sessions.clear()
	if reason == "curfew":
		reopen_text = CurfewWatcher.reopen_text_of(reopen_at) if not reopen_at.is_empty() else curfew.reopen_text()
		_clear_all(false)
		_set_state(STATE.CURFEW)
		return
	_clear_all(true)
	_show_login("TENANT_MISMATCH", true)


## A game API refused the session while playing (F062-AC-12, F062-AC-23): `403 GAME_ACCESS_STUDENT_ONLY`, account or
## tenant codes → same path as a fatal refresh (erase, `EventBus.session_expired`, F061 screen);
## `ACCESS_OUTSIDE_ALLOWED_HOURS` → the form with `ui.common.outside_access_window` and the bootstrap access window.
func handle_access_denied(code: String) -> void:
	if code != LoginFormModel.CODE_OUTSIDE_HOURS:
		_expire_session(code)
		return
	var window: String = ""
	var app_state: Node = _autoload("AppState")
	if app_state != null:
		var play_time: Variant = (app_state.get("school_policies") as Dictionary).get("play_time")
		if play_time is Dictionary and (play_time as Dictionary).get("access_window") is String:
			window = String((play_time as Dictionary)["access_window"])
	sessions.clear()
	_end_stored_session(code)
	var bus: Node = _autoload("EventBus")
	if bus != null:
		bus.emit_signal("session_expired")
	_load_form_memory()
	form.show_banner(code, {"window": window})
	_set_state(STATE.LOGIN)


## ForceUpdate "Cập nhật": opens the store page given by the server (never a login page).
func open_store() -> void:
	if store_url.begins_with("https://"):
		OS.shell_open(store_url)


# ---------- internals ----------

func _call_refresh(mode: String) -> ApiResult:
	var body: Dictionary = {"refresh_token": tokens.refresh_token, "device_id": device_id, "mode": mode}
	if not client_version.is_empty():
		body["client_version"] = client_version
	var req: ApiRequest = ApiRequest.make_post(PATH_REFRESH, body)
	req.authenticated = false
	req.send_tenant = true
	req.allow_refresh = false
	req.max_rate_limit_retries = 0
	var result: ApiResult = await api.send(req)
	req.body = null
	body.clear()
	if result.ok:
		if not tokens.apply(result.body(), clock.now_msec()):
			result.ok = false
			result.code = ApiResult.CODE_INVALID_RESPONSE
			return result
		# Rotation: the new refresh token replaces the stored one before any other request (ADR-0086).
		session_store.save(tokens.refresh_token, tokens.tenant_id)
	elif result.code == "REFRESH_TOKEN_REUSED":
		_telemetry(TELEMETRY_REFRESH_REUSED, {"mode": mode})
	return result


## Applies `tokens` + `bootstrap` of gameLogin / tenant:select (F061-AC-04).
func _accept_session(token_set: Variant, bootstrap: Variant) -> bool:
	if not token_set is Dictionary or not bootstrap is Dictionary:
		return false
	var b: Dictionary = bootstrap as Dictionary
	var tenant: Variant = b.get("tenant_id")
	if not tenant is String or not Uuid.is_valid(tenant as String):
		return false
	if not tokens.apply(token_set as Dictionary, clock.now_msec()):
		return false
	tokens.tenant_id = tenant as String
	session_store.save(tokens.refresh_token, tokens.tenant_id)
	return _apply_bootstrap(b)


func _apply_bootstrap(bootstrap: Variant) -> bool:
	if not bootstrap is Dictionary:
		return false
	var b: Dictionary = bootstrap as Dictionary
	var tenant: Variant = b.get("tenant_id")
	if not tenant is String or (tenant as String) != tokens.tenant_id:
		return false
	nickname = LoginFormModel.nickname_of(b)
	curfew.configure(b, clock.unix_time())
	_curfew_warned = false
	var app_state: Node = _autoload("AppState")
	if app_state != null:
		app_state.call("set_player", b)
	return true


func _start_game_session(takeover: bool) -> void:
	_set_state(STATE.CREATING_SESSION, false)
	form.busy = true
	form.changed.emit()
	var result: ApiResult = await sessions.create(takeover)
	form.busy = false
	form.changed.emit()
	if result.ok:
		signed_in = true
		other_device = null
		var app_state: Node = _autoload("AppState")
		if app_state != null:
			app_state.set("game_session_id", sessions.game_session_id)
		if lifecycle != null:
			lifecycle.idle_enabled = shared_device
			lifecycle.touch()
		sessions.start_heartbeat()
		_set_state(STATE.IN_WORLD)
		session_started.emit(sessions.game_session_id)
		return
	if result.status == 409 and result.code == GameSessionService.CODE_ACTIVE_SESSION:
		other_device = result.problem.other_device if result.problem != null else null
		_set_state(STATE.TAKEOVER_CONFIRM)
		takeover_required.emit(other_device)
		return
	if result.code == CODE_CURFEW:
		_enter_curfew(result)
		return
	# F061-AC-28: no game session → revoke the tokens and never enter the game.
	await _remote_logout()
	_clear_all(true)
	if result.status == 426:
		_enter_force_update(result)
		return
	_show_login(result.code)


func _handle_login_failure(result: ApiResult) -> void:
	var code: String = result.code
	login_failed.emit(code)
	var next: GameStateMachine.State = GameStateMachine.state_for_login_failure(code)
	if next == STATE.TENANT_CHOICE:
		choices = result.problem.choices if result.problem != null else choices
		_login_ticket = result.problem.login_ticket if result.problem != null else ""
		form.end_submit("")
		if choices.is_empty() or _login_ticket.is_empty():
			_show_login(ApiResult.CODE_INVALID_RESPONSE)
			return
		_set_state(STATE.TENANT_CHOICE)
		tenant_choice_required.emit(choices)
	elif next == STATE.FORCE_UPDATE:
		form.end_submit("")
		_enter_force_update(result)
	elif next == STATE.CURFEW:
		form.end_submit("")
		_enter_curfew(result)
	elif next == STATE.LOGIN:
		if code == "RATE_LIMITED":
			form.set_rate_limited(clock.now_msec(), result.retry_after_sec())
		form.end_submit(code)
		_set_state(STATE.LOGIN)
	else:
		notice_code = code
		form.end_submit("")
		_clear_all(code == "ACCOUNT_NOT_PROVISIONED")
		_set_state(next)


func _enter_curfew(result: ApiResult) -> void:
	var reopen_at: String = result.problem.reopen_at if result.problem != null else ""
	reopen_text = CurfewWatcher.reopen_text_of(reopen_at)
	sessions.clear()
	signed_in = false
	# The stored session is kept (F061-AC-36); RAM tokens are dropped.
	tokens.clear()
	_set_state(STATE.CURFEW)


func _enter_force_update(result: ApiResult) -> void:
	if result.problem != null:
		min_client_version = result.problem.min_client_version
		store_url = result.problem.store_url
	_set_state(STATE.FORCE_UPDATE)


## Refresh failed for good while playing (F061-AC-18/30/33/34): erase, `EventBus.session_expired`, form.
func _expire_session(code: String) -> void:
	sessions.clear()
	_end_stored_session(code)
	var bus: Node = _autoload("EventBus")
	if bus != null:
		bus.emit_signal("session_expired")
	_show_after_fatal(code)


func _end_stored_session(_code: String) -> void:
	_clear_all(true)


func _show_after_fatal(code: String) -> void:
	if code.begins_with("REFRESH_TOKEN_") or code == "UNAUTHENTICATED" or code == ApiResult.CODE_INVALID_RESPONSE:
		_show_login("REFRESH_TOKEN_INVALID", true)
		return
	var next: GameStateMachine.State = GameStateMachine.state_for_login_failure(code)
	if next == STATE.LOGIN:
		_show_login(code, true)
		return
	notice_code = code
	_set_state(next)


## Shows the S01 form; `reload_memory` refills the username from the device settings (empty on a shared device).
func _show_login(banner_code: String, reload_memory: bool = false) -> void:
	if reload_memory or form.username.is_empty():
		_load_form_memory()
	if banner_code.is_empty():
		form.end_submit("")
	else:
		form.show_banner(banner_code)
	_set_state(STATE.LOGIN)


func _remote_logout() -> void:
	if tokens.refresh_token.is_empty() or not tokens.has_access():
		return
	var req: ApiRequest = ApiRequest.make_post(PATH_LOGOUT, {"refresh_token": tokens.refresh_token})
	req.allow_refresh = false
	req.max_rate_limit_retries = 0
	await api.send(req)
	req.body = null


## Drops every token/tenant/session of RAM; `erase_store` also erases the SecureStorage record.
func _clear_all(erase_store: bool) -> void:
	tokens.clear()
	_password.wipe()
	_login_ticket = ""
	choices.clear()
	other_device = null
	signed_in = false
	if sessions != null:
		sessions.clear()
	curfew.disable()
	if lifecycle != null:
		lifecycle.idle_enabled = false
	if erase_store and session_store != null:
		session_store.erase()
	var app_state: Node = _autoload("AppState")
	if app_state != null:
		app_state.set("game_session_id", "")
		app_state.call("clear_player")


func _remember_username() -> void:
	if settings == null:
		return
	if shared_device:
		settings.call("set_login_memory", false, "")
		return
	settings.call("set_login_memory", form.remember_username, form.username_to_store(shared_device))


func _load_form_memory() -> void:
	var remember: bool = true
	var last: String = ""
	if settings != null:
		remember = bool(settings.get("remember_username"))
		last = String(settings.get("last_username"))
	form.load_memory(remember, last, shared_device)


func _on_session_replaced(_code: String) -> void:
	# F061-AC-22: the server revoked this device's session; nothing is kept and no reconnect is attempted.
	_clear_all(true)
	session_replaced.emit()
	_set_state(STATE.REPLACED_NOTICE)


func _on_heartbeat_ticked() -> void:
	match curfew.state_at(clock.unix_time()):
		CurfewWatcher.State.WARNING:
			if not _curfew_warned:
				_curfew_warned = true
				curfew_warning_raised.emit(curfew.minutes_until_start(clock.unix_time()))
		CurfewWatcher.State.CURFEW:
			reopen_text = curfew.reopen_text()
			await sessions.end()
			signed_in = false
			tokens.clear()
			_set_state(STATE.CURFEW)


func _on_api_auth_error(code: String) -> void:
	# F061-AC-19: tenant header missing/mismatch after login → erase and ask to log in again.
	sessions.clear()
	_clear_all(true)
	_show_login(code, true)


func _set_state(next: int, navigate: bool = true) -> void:
	state = next
	state_changed.emit(next)
	if not navigate:
		return
	if navigator.is_valid():
		navigator.call(next)
		return
	var path: String = GameStateMachine.scene_for(next as GameStateMachine.State)
	if path.is_empty():
		return
	var router: Node = _autoload("SceneRouter")
	if router == null:
		return
	if bool(router.call("is_loading")):
		_pending_scene = path
		return
	if String(router.get("current_path")) != path:
		router.call("goto", path)


func _on_scene_changed(path: String) -> void:
	if _pending_scene.is_empty() or _pending_scene == path:
		_pending_scene = ""
		return
	var next: String = _pending_scene
	_pending_scene = ""
	var router: Node = _autoload("SceneRouter")
	if router != null:
		router.call("goto", next)


func _quit_after_grace() -> void:
	await clock.wait(CLOSE_GRACE_SEC)
	_quit_now()


func _quit_now() -> void:
	if quitter.is_valid():
		quitter.call()
		return
	var tree: SceneTree = get_tree() if is_inside_tree() else Engine.get_main_loop() as SceneTree
	if tree != null:
		tree.quit()


func _telemetry(event_name: String, fields: Dictionary) -> void:
	var app_state: Node = _autoload("AppState")
	if app_state != null:
		app_state.call("queue_telemetry", event_name, fields)


static func _autoload(autoload_name: String) -> Node:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null(autoload_name) if tree != null else null
