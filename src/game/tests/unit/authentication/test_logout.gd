extends GdUnitTestSuite
## F061-AC-23 (TC-AUTH-004, TC-AUTH-088, ADR-0083, ADR-0086): Đăng xuất (and window close on a `shared_device`) →
## `endGameSession` FIRST, then `gameLogout` with the refresh token, then RAM + SessionStore erased; a failing
## `endGameSession` still logs out.

var h: AuthHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null


func _paths() -> PackedStringArray:
	var out: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.requests:
		out.append("%s %s" % [rec.method, rec.path])
	return out


func test_logout_ends_the_session_then_revokes_the_tokens() -> void:
	h = AuthHarness.create()
	var done: Array[int] = [0]
	h.auth.logout_completed.connect(func() -> void: done[0] += 1)
	await h.sign_in()
	await h.auth.logout()
	var paths: PackedStringArray = _paths()
	var end_at: int = paths.find("POST /game/sessions/%s:end" % AuthHarness.SESSION_ID)
	var logout_at: int = paths.find("POST /auth/game/logout")
	assert_int(end_at).is_greater(0)
	assert_int(logout_at).is_greater(end_at)
	var logout: FakeHttp.Recorded = h.http.last("POST /auth/game/logout")
	assert_str(String(logout.body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(logout.headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_1)
	assert_str(String(logout.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_str(h.auth.tokens.tenant_id).is_empty()
	assert_str(AppState.game_session_id).is_empty()
	assert_dict(AppState.player).is_empty()
	assert_dict(h.stored_record()).is_empty()
	assert_int(done[0]).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_failed_end_still_logs_out() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	h.http.fail("POST /game/sessions/*:end")
	await h.auth.logout()
	assert_int(h.http.count("POST /game/sessions/*:end")).is_equal(1)
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_dict(h.stored_record()).is_empty()


func test_window_close_on_a_shared_device_is_a_logout() -> void:
	h = AuthHarness.create(true)
	await h.sign_in()
	await h.auth.close_game()
	var paths: PackedStringArray = _paths()
	assert_int(paths.find("POST /auth/game/logout")).is_greater(paths.find("POST /game/sessions/%s:end" % AuthHarness.SESSION_ID))
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.quits).is_greater_equal(1)
