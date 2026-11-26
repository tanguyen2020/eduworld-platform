extends GdUnitTestSuite
## F069 S06 solo challenge screens (T027, T028; F069-AC-14, AC-15, AC-18, AC-19): the board shows only the server's
## difficulties (g1_3 easy–hard, g6_7 + expert/legendary), a locked level only with the server's reason, no solo button
## for a party challenge, "no tries left" and the cooldown lock "Vào"; the HUD clock is a bar without seconds for g1_3
## and mm:ss for g6_7, giving up asks first; the result never says "Thất bại" and shows the server's rewards.

const BOARD: String = "res://scenes/challenge/ChallengeBoard.tscn"
const HUD: String = "res://scenes/challenge/ChallengeHud.tscn"
const RESULT: String = "res://scenes/challenge/ChallengeResult.tscn"

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func _board(band: String) -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(BOARD)
	await runner.simulate_frames(1)
	var board: Control = runner.scene() as Control
	board.call("open", {})
	await runner.simulate_frames(10)
	return [runner, board]


func test_board_shows_only_the_server_difficulties_per_grade() -> void:
	var r: Array = await _board("g1_3")
	assert_array(Array(r[1].call("difficulty_codes"))).contains_exactly(["easy", "normal", "hard"])
	s.h.http.clear(F069Fixtures.ROUTE_CHALLENGES)
	s.h.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	s.h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 200, F069Fixtures.challenges("g6_7"))
	s.h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail("g6_7"), {"Date": F069Fixtures.SERVER_DATE})
	var r2: Array = await _board("g6_7")
	assert_array(Array(r2[1].call("difficulty_codes"))).contains_exactly(["easy", "normal", "hard", "expert", "legendary"])


func test_locked_level_only_with_the_server_reason() -> void:
	s.h.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	s.h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail("g1_3", 3, "", true),
			{"Date": F069Fixtures.SERVER_DATE})
	var r: Array = await _board("g1_3")
	var board: Control = r[1] as Control
	# expert: locked with `grade_not_allowed`; legendary: unavailable without a reason → not shown.
	assert_array(Array(board.call("difficulty_codes"))).contains_exactly(["easy", "normal", "hard", "expert"])
	assert_array(Array(board.call("locked_codes"))).contains_exactly(["expert"])
	assert_array(Array(board.call("info_texts"))).contains(["%s: %s" % [tr("ui.challenge.difficulty.expert"),
		tr("ui.challenge.locked.grade_not_allowed")]])


func test_party_challenge_has_no_solo_button() -> void:
	var r: Array = await _board("g1_3")
	var board: Control = r[1] as Control
	s.h.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	var party: Dictionary = F069Fixtures.challenge_detail("g1_3")
	party["challenge_id"] = F069Fixtures.CHALLENGE_PARTY
	party["challenge_type"] = "coop"
	party["party_required"] = true
	s.h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, party)
	await board.call("select", F069Fixtures.CHALLENGE_PARTY)
	assert_object(board.call("enter_button")).is_null()
	assert_str(String(board.call("note_text"))).is_equal("ui.challenge.party_only")


func test_no_tries_left_and_cooldown_lock_the_enter_button() -> void:
	s.h.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	s.h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail("g1_3", 0, "2026-11-26T07:42:00Z"),
			{"Date": F069Fixtures.SERVER_DATE})
	var r: Array = await _board("g1_3")
	var board: Control = r[1] as Control
	assert_bool((board.call("enter_button") as Button).disabled).is_true()
	assert_str(String(board.call("note_text"))).is_equal(tr("ui.challenge.no_attempts"))
	assert_str(String(board.call("cooldown_text"))).is_equal(tr("ui.challenge.cooldown").format({"time": "12:00"}))


func test_enter_starts_the_run() -> void:
	var r: Array = await _board("g1_3")
	var board: Control = r[1] as Control
	s.services.challenge.intent_sink = func(_screen: StringName, _args: Dictionary) -> void: pass
	s.h.http.reply(F069Fixtures.ROUTE_START, 201, F069Fixtures.attempt(), {"Date": F069Fixtures.SERVER_DATE})
	s.h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_1))
	await board.call("enter")
	assert_str(String(s.h.http.last(F069Fixtures.ROUTE_START).body["difficulty"])).is_equal("easy")
	assert_bool(s.services.challenge.is_running()).is_true()
	s.services.challenge.dismiss_result()


func _run() -> void:
	s.services.challenge.intent_sink = func(_screen: StringName, _args: Dictionary) -> void: pass
	await s.services.challenge.list()
	await s.services.challenge.get_challenge(F069Fixtures.CHALLENGE_TIME)
	s.h.http.reply(F069Fixtures.ROUTE_START, 201, F069Fixtures.attempt("in_progress", 2, 2), {"Date": F069Fixtures.SERVER_DATE})
	s.h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_1, 2),
			{"Date": F069Fixtures.SERVER_DATE})
	await s.services.challenge.start(F069Fixtures.CHALLENGE_TIME, "hard")


func test_hud_clock_bar_for_g1_3_and_digits_for_g6_7_and_abandon_confirms() -> void:
	await _run()
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(1)
	var hud: ChallengeHud = runner.scene() as ChallengeHud
	hud.bind(s.services.challenge)
	await runner.simulate_frames(1)
	assert_bool(hud.visible).is_true()
	assert_str(hud.clock_text()).is_empty()
	assert_float(hud.bar_value()).is_greater(0.0)
	assert_int((hud.find_child("Lives", true, false) as Node).get_child_count()).is_equal(s.services.challenge.attempt().lives_left)
	AppState.set_grade_band("g6_7")
	s.services.challenge.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID,
		"status": "in_progress", "activities_completed": 3, "lives_left": 1})
	assert_str(hud.clock_text()).is_equal("03:00")
	assert_int((hud.find_child("Lives", true, false) as Node).get_child_count()).is_equal(1)
	s.h.clock.now_ms += 61_000
	hud.render_clock()
	assert_str(hud.clock_text()).is_equal("01:59")
	(hud.find_child("Stop", true, false) as Button).pressed.emit()
	assert_bool(hud.confirm_visible()).is_true()
	assert_int(s.h.http.count(F069Fixtures.ROUTE_ABANDON)).is_equal(0)
	(hud.find_child("KeepPlaying", true, false) as Button).pressed.emit()
	assert_bool(hud.confirm_visible()).is_false()
	s.h.http.reply(F069Fixtures.ROUTE_ABANDON, 200, F069Fixtures.attempt("abandoned", 3))
	hud.ask_abandon()
	await hud.confirm_abandon()
	assert_int(s.h.http.count(F069Fixtures.ROUTE_ABANDON)).is_equal(1)
	assert_bool(hud.visible).is_false()
	s.services.challenge.dismiss_result()


func _result(status: String, completed: int, rewards: Array) -> Control:
	await _run()
	s.services.challenge.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID,
		"status": status, "activities_completed": completed, "rewards": rewards})
	var runner: GdUnitSceneRunner = scene_runner(RESULT)
	await runner.simulate_frames(1)
	var result: Control = runner.scene() as Control
	result.call("open", {})
	await runner.simulate_frames(2)
	return result


func test_failed_result_is_kind_and_shows_server_rewards() -> void:
	var result: Control = await _result("failed", 4, [{"kind": "xp", "amount": 10}])
	var texts: PackedStringArray = result.call("texts") as PackedStringArray
	assert_str(texts[0]).is_equal(tr("ui.challenge.result.completed_n").format({"n": "4"}))
	assert_str(texts[1]).is_equal(tr("ui.challenge.result.review_hint"))
	assert_array(Array(texts)).contains([tr("ui.reward.xp").format({"amount": "10"})])
	for t: String in texts:
		for word: String in ["thất bại", "thua", "failed", "lose", "phạt"]:
			assert_str(t.to_lower()).not_contains(word)
	assert_str(String(AppState.blocking_activity)).is_equal("challenge")
	result.call("close")
	assert_str(String(AppState.blocking_activity)).is_empty()


func test_succeeded_and_abandoned_results() -> void:
	var result: Control = await _result("succeeded", 6, [{"kind": "currency", "amount": 30}])
	assert_str((result.call("texts") as PackedStringArray)[0]).is_equal(tr("ui.challenge.result.succeeded"))
	result.call("close")
	for locale: String in ["vi-VN", "en-US"]:
		for key: String in ["ui.challenge.result.completed_n", "ui.challenge.result.review_hint",
				"ui.challenge.result.abandoned", "ui.challenge.result.abandoned_line", "ui.challenge.stop_confirm"]:
			var text: String = String(TranslationServer.get_translation_object(locale).get_message(key)).to_lower()
			for word: String in ["thất bại", "thua", "fail", "lose", "phạt", "penalty"]:
				assert_str(text).override_failure_message("%s %s" % [locale, key]).not_contains(word)
