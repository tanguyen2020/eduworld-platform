extends GdUnitTestSuite
## T035 (TC-AUTH-015, TC-AUTH-016, SC-004): two clients with different `device_id` and the same student against the
## backend compose — B gets `409` → takeover without the password → A is told it was replaced within one heartbeat.
## Skipped unless EDUWORLD_IT_API_BASE_URL is set (see LiveBackend).

const SKIP_REASON: String = "needs the backend compose (EDUWORLD_IT_API_BASE_URL)"


func test_takeover_between_two_devices(do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var a: LiveBackend = LiveBackend.create(self, Uuid.v4())
	var b: LiveBackend = LiveBackend.create(self, Uuid.v4())
	a.auth.sessions.heartbeat_interval_sec = 2.0
	await a.auth.login("hs.an.3a", LiveBackend.student_password())
	assert_int(a.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	await b.auth.login("hs.an.3a", LiveBackend.student_password())
	assert_int(b.last_state()).is_equal(GameStateMachine.State.TAKEOVER_CONFIRM)
	assert_bool(b.auth.holds_password()).is_false()
	await b.auth.confirm_takeover()
	assert_int(b.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	await await_millis(5000)
	assert_int(a.last_state()).is_equal(GameStateMachine.State.REPLACED_NOTICE)
	await b.auth.logout()
	a.dispose()
	b.dispose()
