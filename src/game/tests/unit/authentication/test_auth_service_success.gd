extends GdUnitTestSuite
## F061-AC-04 (TC-AUTH-082, ADR-0083, ADR-0086) and F061-AC-28 (TC-AUTH-046): after `gameLogin` 200 the access token
## is in RAM only, refresh token + tenant in SessionStore, bootstrap in AppState, the password wiped; then
## `createGameSession` (`X-Device-Id`, `X-Tenant-Id`, new UUIDv7 `Idempotency-Key`) and only `201` stores
## `game_session_id` and enters the game. Any other failure than 409 → `gameLogout`, RAM + SessionStore erased, form.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_success_stores_tokens_and_creates_the_game_session() -> void:
	var created: Array[String] = []
	h.auth.session_started.connect(func(id: String) -> void: created.append(id))
	await h.sign_in()
	assert_str(h.auth.tokens.access_token).is_equal(AuthHarness.ACCESS_1)
	assert_str(h.auth.tokens.tenant_id).is_equal(AuthHarness.TENANT_ID)
	var record: Dictionary = h.stored_record()
	assert_array(record.keys()).contains_exactly_in_any_order(["refresh_token", "tenant_id", "saved_at"])
	assert_str(String(record["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(record["tenant_id"])).is_equal(AuthHarness.TENANT_ID)
	assert_str(String(AppState.player.get("tenant_id", ""))).is_equal(AuthHarness.TENANT_ID)
	assert_bool(h.auth.holds_password()).is_false()
	var session: FakeHttp.Recorded = h.http.last("POST /game/sessions")
	assert_str(String(session.headers["x-device-id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(session.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_str(String(session.headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_1)
	assert_bool(Uuid.is_v7(String(session.headers["idempotency-key"]))).is_true()
	assert_bool(bool(session.body["takeover"])).is_false()
	assert_str(String(session.body["client_platform"])).is_equal("windows")
	assert_str(String(session.body["client_version"])).is_equal(AuthHarness.VERSION)
	assert_str(AppState.game_session_id).is_equal(AuthHarness.SESSION_ID)
	assert_array(created).is_equal([AuthHarness.SESSION_ID])
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	# gameLogin happened before createGameSession.
	assert_str(h.http.requests[0].path).is_equal("/auth/game/login")
	assert_str(h.http.requests[1].path).is_equal("/game/sessions")


func test_nothing_secret_reaches_user_files() -> void:
	await h.sign_in()
	for path: String in _user_files("user://"):
		var text: String = FileAccess.get_file_as_bytes(path).get_string_from_ascii()
		for secret: String in [AuthHarness.PASSWORD, AuthHarness.ACCESS_1, AuthHarness.REFRESH_1]:
			assert_str(text).override_failure_message(path).not_contains(secret)


func test_session_failure_logs_out_and_never_enters_the_game() -> void:
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 503, AuthHarness.problem(503, "UNAVAILABLE"))
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	var tries: Array[FakeHttp.Recorded] = h.http.find("POST /game/sessions")
	assert_int(tries.size()).is_equal(4)
	for rec: FakeHttp.Recorded in tries:
		assert_str(String(rec.headers["idempotency-key"])).is_equal(String(tries[0].headers["idempotency-key"]))
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_str(String(h.http.last("POST /auth/game/logout").body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_dict(h.stored_record()).is_empty()
	assert_str(AppState.game_session_id).is_empty()
	assert_array(h.states).not_contains([GameStateMachine.State.IN_WORLD])
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(h.auth.form.banner_code).is_equal("UNAVAILABLE")


func test_session_refused_for_staff_or_old_client() -> void:
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_dict(h.stored_record()).is_empty()
	h.dispose()
	h = AuthHarness.create()
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 426, AuthHarness.problem(426, "CLIENT_VERSION_UNSUPPORTED",
			{"min_client_version": "2.0.0"}, "ClientVersionProblem"))
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.FORCE_UPDATE)
	assert_str(h.auth.min_client_version).is_equal("2.0.0")


static func _user_files(dir_path: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return out
	for file_name: String in dir.get_files():
		out.append(dir_path.path_join(file_name))
	for sub: String in dir.get_directories():
		if sub != "logs" and sub != "shader_cache" and sub != "vulkan":
			out.append_array(_user_files(dir_path.path_join(sub)))
	return out
