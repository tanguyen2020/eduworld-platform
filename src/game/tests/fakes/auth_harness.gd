class_name AuthHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an AuthService + ApiClient pair on FakeHttp, FakeClock and FakeSecureStorage,
## records navigation and quit requests, and builds response bodies from the contract fixtures generated from
## specs/openapi (tests/fixtures/contract/openapi, src/game/tools/gen_fixtures.mjs).

const SettingsScript := preload("res://scripts/core/settings.gd")
const FIXTURES: String = "res://tests/fixtures/contract/openapi"
const SETTINGS_PATH: String = "user://test_f061_settings.cfg"
const DEVICE_ID: String = "11111111-1111-4111-8111-111111111111"
const TENANT_ID: String = "00000000-0000-4000-8000-000000000001"
const OTHER_TENANT_ID: String = "22222222-2222-4222-8222-222222222222"
const SESSION_ID: String = "33333333-3333-4333-8333-333333333333"
const VERSION: String = "1.2.3"
const PLATFORM: String = "windows"
## Unique secrets so tests can search logs/storage for them.
const PASSWORD: String = "Pw-F061-unique-7731"
const ACCESS_1: String = "access-F061-aaaa-0001"
const REFRESH_1: String = "refresh-F061-rrrr-0001"
const ACCESS_2: String = "access-F061-aaaa-0002"
const REFRESH_2: String = "refresh-F061-rrrr-0002"
const ACCESS_3: String = "access-F061-aaaa-0003"
const REFRESH_3: String = "refresh-F061-rrrr-0003"
const TICKET: String = "ticket-F061-tttt-000000001"

var http: FakeHttp = FakeHttp.new()
var clock: FakeClock = FakeClock.new()
var storage: FakeSecureStorage = FakeSecureStorage.new()
var api: RestApiClient = null
var auth: GameAuthService = null
var settings: Node = null
var states: Array[int] = []
var quits: int = 0


static func create(shared_device: bool = false, persistent: bool = true) -> AuthHarness:
	var h: AuthHarness = AuthHarness.new()
	h._build(shared_device, persistent)
	return h


func _build(shared_device: bool, persistent: bool) -> void:
	DirAccess.remove_absolute(SETTINGS_PATH)
	settings = SettingsScript.new() as Node
	settings.call("load_from", SETTINGS_PATH)
	api = RestApiClient.new()
	api.configure(http, clock, TokenStore.new(), DEVICE_ID, http.base_url, VERSION, PLATFORM)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 61
	api.rng = rng
	auth = GameAuthService.new()
	auth.rng = rng
	var store: SecureStorage = storage if persistent else MemoryOnlySecureStorage.new()
	auth.configure(api, clock, store, settings, DEVICE_ID, VERSION, PLATFORM, shared_device)
	auth.sessions.heartbeat_interval_sec = FakeClock.HOLD_THRESHOLD_SEC
	auth.navigator = _on_navigate
	auth.quitter = _on_quit
	# F070-AC-01: after the game session the client reads `getGameProfile`; by default the student already has an
	# avatar and a nickname (straight to the hub). First-time tests call ProfileFixtures.script_first_time().
	http.reply(ProfileFixtures.ROUTE_PROFILE, 200, ProfileFixtures.profile_ready())
	Log.clear()
	AppState.pending_telemetry.clear()


func dispose() -> void:
	if auth != null:
		auth.sessions.stop_heartbeat()
		clock.release()
		auth.free()
	if api != null:
		api.free()
	if settings != null:
		settings.free()
	DirAccess.remove_absolute(SETTINGS_PATH)
	AppState.clear_player()
	AppState.game_session_id = ""


func _on_navigate(state: int) -> void:
	states.append(state)


func _on_quit() -> void:
	quits += 1


func last_state() -> int:
	return states[states.size() - 1] if not states.is_empty() else -1


## Stores a session record as a previous run would have (personal device).
func store_session(refresh_token: String = REFRESH_1, tenant_id: String = TENANT_ID) -> void:
	SessionStore.new(storage).save(refresh_token, tenant_id)


func stored_record() -> Dictionary:
	return SessionStore.new(storage).load_record()


# ---------- response bodies from the contract fixtures ----------

static func fixture(schema: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [FIXTURES, schema])) as Dictionary


static func token_set(access: String, refresh: String, expires_in: int = 300) -> Dictionary:
	var t: Dictionary = (fixture("GameLoginResponse")["tokens"] as Dictionary).duplicate(true)
	t["access_token"] = access
	t["refresh_token"] = refresh
	t["expires_in"] = expires_in
	t["refresh_expires_in"] = 604800
	return t


static func bootstrap(tenant_id: String = TENANT_ID, nickname: String = "MeoXanh42") -> Dictionary:
	var b: Dictionary = (fixture("GameLoginResponse")["bootstrap"] as Dictionary).duplicate(true)
	b["tenant_id"] = tenant_id
	if b.get("game_profile") is Dictionary:
		(b["game_profile"] as Dictionary)["nickname"] = nickname
	else:
		b["game_profile"] = {"student_id": TENANT_ID, "nickname": nickname, "level": 1, "xp_total": 0}
	b.erase("policies")
	b["policies"] = {}
	return b


static func login_ok(access: String = ACCESS_1, refresh: String = REFRESH_1, tenant_id: String = TENANT_ID) -> Dictionary:
	var body: Dictionary = fixture("GameLoginResponse")
	body["tokens"] = token_set(access, refresh)
	body["bootstrap"] = bootstrap(tenant_id)
	return body


static func refresh_ok(access: String, refresh: String, with_bootstrap: bool, tenant_id: String = TENANT_ID) -> Dictionary:
	var body: Dictionary = fixture("GameTokenRefreshResponse")
	body.merge(token_set(access, refresh), true)
	if with_bootstrap:
		body["bootstrap"] = bootstrap(tenant_id)
	else:
		body.erase("bootstrap")
	return body


static func session_ok(id: String = SESSION_ID) -> Dictionary:
	var body: Dictionary = fixture("GameSession")
	body["game_session_id"] = id
	return body


## Problem body: generic Problem fixture (or `schema`) with status/code and extension members.
static func problem(status: int, code: String, extra: Dictionary = {}, schema: String = "Problem") -> Dictionary:
	var body: Dictionary = fixture(schema)
	body.erase("reopen_at")
	body["status"] = status
	body["code"] = code
	body.merge(extra, true)
	return body


static func tenant_choices() -> Dictionary:
	var base: Dictionary = fixture("GameLoginProblem")
	var first: Dictionary = ((base["choices"] as Array)[0] as Dictionary).duplicate(true)
	first["name"] = "TH Demo A"
	var second: Dictionary = first.duplicate(true)
	second["tenant_id"] = OTHER_TENANT_ID
	second["name"] = "THCS Demo B"
	base["status"] = 403
	base["code"] = "TENANT_NOT_SELECTED"
	base["choices"] = [first, second]
	base["login_ticket"] = TICKET
	base.erase("reopen_at")
	return base


# ---------- common scripted flows ----------

func script_login_and_session() -> void:
	http.reply("POST /auth/game/login", 200, login_ok())
	http.reply("POST /game/sessions", 201, session_ok())
	http.reply("POST /game/sessions/*:end", 204)
	http.reply("POST /auth/game/logout", 204)


## Coroutine: logs in successfully and creates the game session.
func sign_in() -> void:
	script_login_and_session()
	await auth.login("hs.an.3a", PASSWORD)
