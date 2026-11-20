extends GdUnitTestSuite
## F061-AC-10 (TC-AUTH-060, ADR-0068, AGENTS R14, `game_login_body_logging` off): after every login branch the
## log ring buffer (attached to crash reports) and the telemetry queue contain no password, token or login ticket.

const SECRETS: PackedStringArray = [
	AuthHarness.PASSWORD, AuthHarness.ACCESS_1, AuthHarness.REFRESH_1, AuthHarness.ACCESS_2, AuthHarness.REFRESH_2,
	AuthHarness.TICKET,
]

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _assert_no_secret() -> void:
	var text: String = JSON.stringify(Log.entries()) + JSON.stringify(AppState.pending_telemetry)
	for secret: String in SECRETS:
		assert_str(text).override_failure_message("found %s" % secret).not_contains(secret)


func test_every_login_branch_keeps_secrets_out_of_logs_and_telemetry() -> void:
	Log.set_level_name("debug")
	h.http.reply("POST /auth/game/login", 401, AuthHarness.problem(401, "INVALID_CREDENTIALS"))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	h.http.reply("POST /auth/game/login", 429, AuthHarness.problem(429, "LOGIN_QUEUE_BUSY"), {"Retry-After": "1"})
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	h.http.reply("POST /auth/game/tenant:select", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.select_tenant(AuthHarness.TENANT_ID)
	h.http.reply("GET /game/profile", 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply("GET /game/profile", 200, {})
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	await h.api.send(ApiRequest.make_get("/game/profile"))
	h.http.reply("POST /auth/game/token:refresh", 401, AuthHarness.problem(401, "REFRESH_TOKEN_REUSED"))
	await h.auth.refresh()
	h.http.reply("POST /game/sessions/*:end", 204)
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.logout()
	Log.set_level_name("info")
	assert_int(Log.entries().size()).is_greater(3)
	_assert_no_secret()
	var events: PackedStringArray = []
	for e: Dictionary in AppState.pending_telemetry:
		events.append(String(e["event"]))
	assert_array(Array(events)).contains(["auth_refresh_reused"])


func test_debug_strings_of_secret_holders_are_redacted() -> void:
	await h.sign_in()
	assert_str(str(h.auth.tokens)).not_contains(AuthHarness.ACCESS_1)
	var buffer: PasswordBuffer = PasswordBuffer.new()
	buffer.set_text(AuthHarness.PASSWORD)
	assert_str(str(buffer)).not_contains(AuthHarness.PASSWORD)
	var redacted: Dictionary = Log.redact({"Authorization": "Bearer %s" % AuthHarness.ACCESS_1, "login_ticket": AuthHarness.TICKET,
			"password": AuthHarness.PASSWORD}) as Dictionary
	assert_str(JSON.stringify(redacted)).not_contains(AuthHarness.ACCESS_1)
	assert_str(JSON.stringify(redacted)).not_contains(AuthHarness.TICKET)
	buffer.wipe()
	assert_bool(buffer.is_empty()).is_true()


func test_request_logs_keep_only_method_path_status_code_trace() -> void:
	Log.set_level_name("debug")
	await h.sign_in()
	Log.set_level_name("info")
	var seen: int = 0
	for entry: Dictionary in Log.entries():
		if String(entry["event"]) != "api_request_completed":
			continue
		seen += 1
		assert_array((entry["fields"] as Dictionary).keys()).contains_exactly_in_any_order(
				["method", "path", "status", "code", "trace_id", "elapsed_ms"])
	# gameLogin, createGameSession and (F070-AC-01) getGameProfile.
	assert_int(seen).is_equal(3)
