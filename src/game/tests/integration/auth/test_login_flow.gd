extends GdUnitTestSuite
## T044 (TC-AUTH-001, TC-AUTH-014, SC-005): against the backend compose — `hs.an.3a` logs in and every request after
## login carries `X-Tenant-Id` while the login itself carries none; staff (`ad.truong.a`) is refused with
## GAME_ACCESS_STUDENT_ONLY. Skipped unless EDUWORLD_IT_API_BASE_URL is set (see LiveBackend).

const SKIP_REASON: String = "needs the backend compose (EDUWORLD_IT_API_BASE_URL)"


func test_student_login_sends_the_tenant_header_after_login(do_skip: bool = LiveBackend.skip(),
		skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	assert_int(live.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	live.api.send(ApiRequest.make_get("/game/content-manifest"))
	await await_millis(1500)
	var after_login: bool = false
	for rec: Dictionary in live.transport.sent:
		var path: String = String(rec["path"])
		var headers: Dictionary = rec["headers"] as Dictionary
		if path == "/auth/game/login":
			assert_bool(headers.has("x-tenant-id")).is_false()
			after_login = true
		elif after_login:
			assert_bool(headers.has("x-tenant-id")).override_failure_message(path).is_true()
	await live.auth.logout()
	live.dispose()


func test_staff_account_cannot_enter_the_game(do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("ad.truong.a", LiveBackend.staff_password())
	assert_int(live.last_state()).is_equal(GameStateMachine.State.NOT_STUDENT)
	assert_bool(live.auth.tokens.has_access()).is_false()
	live.dispose()
