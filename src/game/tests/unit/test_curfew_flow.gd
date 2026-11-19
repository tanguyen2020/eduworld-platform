extends GdUnitTestSuite
## F061-AC-36 (TC-AUTH-092, ADR-0078, AC-UX-0115): `403 CURFEW_ACTIVE` from login, resume or createGameSession →
## CurfewNotice with the reopen time from `reopen_at`, no form, SecureStorage kept; while playing, a warning
## `warning_minutes` before the window (bootstrap + server time) and CurfewNotice when it starts or when
## `session.session.ended` (`curfew`) arrives — the stored session is kept.

## 2026-11-20T14:20:00Z = 21:20 ICT.
const SERVER_21_20: String = "2026-11-20T14:20:00Z"
const REOPEN_06_30: String = "2026-11-20T23:30:00Z"

var h: AuthHarness = null


func before_test() -> void:
	TranslationServer.set_locale("vi-VN")
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _curfew_problem() -> Dictionary:
	return AuthHarness.problem(403, "CURFEW_ACTIVE", {"reopen_at": REOPEN_06_30})


func test_login_in_curfew_shows_the_reopen_time() -> void:
	h.http.reply("POST /auth/game/login", 403, _curfew_problem())
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CURFEW)
	assert_str(h.auth.reopen_text).is_equal("06:30")
	assert_str(tr("ui.login.curfew").format({"time": h.auth.reopen_text})).contains("06:30")


func test_resume_in_curfew_keeps_the_stored_session() -> void:
	h.store_session()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/token:refresh", 403, _curfew_problem())
	await h.auth.start()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CURFEW)
	assert_array(h.states).not_contains([GameStateMachine.State.LOGIN])
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)


func test_session_create_in_curfew_keeps_the_stored_session() -> void:
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 403, _curfew_problem())
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CURFEW)
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(0)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)


func test_warning_then_curfew_while_playing() -> void:
	var login: Dictionary = AuthHarness.login_ok()
	var b: Dictionary = login["bootstrap"] as Dictionary
	b["policies"] = {"play_time": {"curfew": {"start": "21:30", "end": "06:30", "warning_minutes": 10}}}
	b["server_time"] = SERVER_21_20
	h.http.reply("POST /auth/game/login", 200, login)
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	h.http.reply("POST /game/sessions/*:heartbeat", 204)
	h.http.reply("POST /game/sessions/*:end", 204)
	var warnings: Array[int] = []
	h.auth.curfew_warning_raised.connect(func(m: int) -> void: warnings.append(m))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.auth.curfew.state_at(h.clock.unix_time())).is_equal(CurfewWatcher.State.WARNING)
	h.clock.release()
	for i: int in 4:
		await get_tree().process_frame
	assert_array(warnings).is_equal([10])
	h.clock.advance(10 * 60)
	h.clock.release()
	for i: int in 6:
		await get_tree().process_frame
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CURFEW)
	assert_str(h.auth.reopen_text).is_equal("06:30")
	assert_int(h.http.count("POST /game/sessions/*:end")).is_equal(1)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)


func test_session_ended_for_curfew_keeps_the_record() -> void:
	await h.sign_in()
	h.auth.handle_session_ended("curfew", REOPEN_06_30)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CURFEW)
	assert_str(h.auth.reopen_text).is_equal("06:30")
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	h.auth.handle_session_ended("access_revoked")
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_window_math() -> void:
	assert_bool(CurfewWatcher.in_window(21 * 60 + 30, 21 * 60 + 30, 6 * 60 + 30)).is_true()
	assert_bool(CurfewWatcher.in_window(3 * 60, 21 * 60 + 30, 6 * 60 + 30)).is_true()
	assert_bool(CurfewWatcher.in_window(6 * 60 + 30, 21 * 60 + 30, 6 * 60 + 30)).is_false()
	assert_bool(CurfewWatcher.in_window(12 * 60, 21 * 60 + 30, 6 * 60 + 30)).is_false()
	assert_int(CurfewWatcher.parse_hhmm("24:00")).is_equal(-1)
	assert_float(CurfewWatcher.parse_iso_unix("2026-11-20T21:20:00+07:00")).is_equal(CurfewWatcher.parse_iso_unix(SERVER_21_20))
