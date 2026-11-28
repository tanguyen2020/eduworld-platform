extends GdUnitTestSuite
## T013 (F110-AC-08; CHALLENGE_BOSS_SYSTEM §3.5, STUDENT_GAME_UX_CORE S06 §8): `boss.encounter.ended` opens BossResult —
## `defeated` says "Hạ {boss}!" with THIS student's rewards from `rewards`; `time_up` / `abandoned` say "Em đã tới hiệp
## n/total" with a review hint and never the word "Thất bại"; the level-up modal waits until the result is closed
## (`AppState.blocking_activity` = `boss`, back to "" after); a party boss goes back to the lobby once the party session
## ended (F102).

var s: F110SceneHarness = null
var arena: BossArena = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	if arena != null and arena.visible:
		arena.close()
	s.dispose()


func _open(mode: String) -> void:
	if mode == "party":
		await s.with_party()
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200,
			F110Fixtures.encounter(s.self_id, mode, "phase_active", 1))
	arena = auto_free((load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate()) as BossArena
	arena.services = s.services
	add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": mode, "title": "Rồng Phân Số"})
	await s.settle(2)


func _all_text(root: Node) -> String:
	var parts: PackedStringArray = []
	for n: Node in root.find_children("*", "Label", true, false):
		if (n as Label).is_visible_in_tree():
			parts.append(TranslationServer.translate((n as Label).text))
	for n: Node in root.find_children("*", "Button", true, false):
		if (n as Button).is_visible_in_tree():
			parts.append(TranslationServer.translate((n as Button).text))
	return " ".join(parts)


func test_defeated_shows_the_boss_name_and_only_this_students_rewards() -> void:
	await _open("solo")
	assert_str(String(AppState.blocking_activity)).is_equal("boss")
	s.push("boss.encounter.ended", F110Fixtures.ended("defeated"))
	await s.settle(2)
	var r: BossResult = arena.result_view()
	assert_bool(r.visible).is_true()
	assert_str(r.headline()).is_equal(TranslationServer.translate("ui.boss.defeated").format({"boss": "Rồng Phân Số", "n": "2",
		"total": "2"}))
	var chips: PackedStringArray = r.reward_texts()
	assert_int(chips.size()).is_equal(2)
	assert_str(" ".join(chips)).contains("120")
	# Still blocking while the result is open (the level-up modal waits), cleared once it is closed.
	assert_str(String(AppState.blocking_activity)).is_equal("boss")
	r.back_button().pressed.emit()
	await s.settle(1)
	assert_bool(arena.visible).is_false()
	assert_str(String(AppState.blocking_activity)).is_equal("")


func test_time_up_and_abandoned_say_how_far_the_student_went_never_failed() -> void:
	for outcome: String in ["time_up", "abandoned"]:
		await _open("solo")
		s.push("boss.encounter.ended", F110Fixtures.ended(outcome, 1, false))
		await s.settle(2)
		var r: BossResult = arena.result_view()
		assert_str(r.headline()).is_equal(TranslationServer.translate("ui.boss.reached_phase").format({"boss": "Rồng Phân Số",
			"n": "1", "total": "2"}))
		assert_array(r.reward_texts()).is_empty()
		var text: String = _all_text(r)
		assert_str(text).contains(TranslationServer.translate("ui.boss.review_hint"))
		for word: String in ["Thất bại", "thất bại", "Failed", "failed", "Fail"]:
			assert_bool(text.contains(word)).override_failure_message("'%s' on the result" % word).is_false()
		arena.close()


func test_a_party_boss_goes_back_to_the_lobby() -> void:
	await _open("party")
	var opened: Array = []
	var sink: Callable = func(screen: StringName, _args: Dictionary) -> void: opened.append(screen)
	EventBus.ui_requested.connect(sink)
	s.push("boss.encounter.ended", F110Fixtures.ended("defeated"))
	var ended: Dictionary = PartyFixtures.as_self(RealtimeHarness.fixture("party.session.ended.completed")["payload"], s.self_id) as Dictionary
	s.push("party.session.ended", ended)
	await s.settle(2)
	assert_bool(arena.party_ended or not arena.visible).is_true()
	var r: BossResult = arena.result_view()
	assert_str(r.back_button().text).is_equal("ui.boss.back_lobby")
	r.back_button().pressed.emit()
	await s.settle(1)
	EventBus.ui_requested.disconnect(sink)
	assert_array(opened).contains([&"party"])


func test_the_party_session_ending_without_a_result_goes_back_to_the_lobby_at_once() -> void:
	await _open("party")
	var opened: Array = []
	var sink: Callable = func(screen: StringName, _args: Dictionary) -> void: opened.append(screen)
	EventBus.ui_requested.connect(sink)
	var ended: Dictionary = PartyFixtures.as_self(RealtimeHarness.fixture("party.session.ended.completed")["payload"], s.self_id) as Dictionary
	s.services.party.store.start_session({"room_id": F110Fixtures.ENCOUNTER_ID, "room_type": "boss"})
	s.push("party.session.ended", ended)
	await s.settle(2)
	EventBus.ui_requested.disconnect(sink)
	assert_bool(arena.visible).is_false()
	assert_array(opened).contains([&"party"])
