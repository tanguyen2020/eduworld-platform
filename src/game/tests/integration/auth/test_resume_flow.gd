extends GdUnitTestSuite
## T053 (TC-AUTH-082, TC-AUTH-083, TC-AUTH-087, F061-AC-34): against the backend compose — log in, close the window
## (personal device: only `:end`), start a new client on the same stored session → ResumeConfirm without the form;
## a revoked session (logout from the second client) makes the next resume end at the form once, without a retry
## loop. Realm idle-timeout and password-reset variants stay manual (quickstart §2 steps 29, 34).
## Skipped unless EDUWORLD_IT_API_BASE_URL is set (see LiveBackend).

const SKIP_REASON: String = "needs the backend compose (EDUWORLD_IT_API_BASE_URL)"


func test_resume_after_restart_then_revoked(do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var device: String = Uuid.v4()
	var storage: FakeSecureStorage = FakeSecureStorage.new()
	var first: LiveBackend = LiveBackend.create(self, device, storage)
	await first.auth.login("hs.an.3a", LiveBackend.student_password())
	assert_int(first.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	await first.auth.close_game()
	first.dispose()
	var second: LiveBackend = LiveBackend.create(self, device, storage)
	await second.auth.start()
	assert_int(second.last_state()).is_equal(GameStateMachine.State.RESUME_CONFIRM)
	assert_array(second.states).not_contains([GameStateMachine.State.LOGIN])
	await second.auth.continue_resume()
	assert_int(second.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	var record: Dictionary = SessionStore.new(storage).load_record()
	await second.auth.logout()
	second.dispose()
	# Put the revoked refresh token back, as if logout had happened on another device.
	SessionStore.new(storage).save(String(record["refresh_token"]), String(record["tenant_id"]))
	var third: LiveBackend = LiveBackend.create(self, device, storage)
	await third.auth.start()
	assert_int(third.last_state()).is_equal(GameStateMachine.State.LOGIN)
	var refreshes: int = 0
	for rec: Dictionary in third.transport.sent:
		if String(rec["path"]) == "/auth/game/token:refresh":
			refreshes += 1
	assert_int(refreshes).is_equal(1)
	third.dispose()
