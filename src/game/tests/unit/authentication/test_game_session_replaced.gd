extends GdUnitTestSuite
## F061-AC-22 (TC-AUTH-015, ADR-0086): heartbeat `409` (replaced) or `404` → gameplay stops, ReplacedNotice that
## never closes by itself, RAM + SessionStore erased, no reconnect, then the form.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _tick() -> void:
	h.clock.release()
	for i: int in 4:
		await get_tree().process_frame


func test_heartbeat_conflict_or_not_found_ends_the_device_session() -> void:
	for status: int in [409, 404]:
		h.dispose()
		h = AuthHarness.create()
		var replaced: Array[int] = [0]
		h.auth.session_replaced.connect(func() -> void: replaced[0] += 1)
		await h.sign_in()
		h.http.reply("POST /game/sessions/*:heartbeat", status, AuthHarness.problem(status, "CONFLICT"))
		await _tick()
		assert_int(replaced[0]).is_equal(1)
		assert_int(h.last_state()).override_failure_message(str(status)).is_equal(GameStateMachine.State.REPLACED_NOTICE)
		assert_bool(h.auth.tokens.has_access()).is_false()
		assert_dict(h.stored_record()).is_empty()
		assert_str(AppState.game_session_id).is_empty()
		var beats: int = h.http.count("POST /game/sessions/*:heartbeat")
		await _tick()
		await _tick()
		assert_int(h.http.count("POST /game/sessions/*:heartbeat")).is_equal(beats)
		assert_int(h.http.count("POST /game/sessions")).is_equal(1)
		assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(0)
		h.auth.back_to_login()
		assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
