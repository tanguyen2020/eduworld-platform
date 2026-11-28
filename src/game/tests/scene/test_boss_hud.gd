extends GdUnitTestSuite
## T012 (F110-AC-02, F110-AC-03, F110-AC-06, F110-AC-07; CHALLENGE_BOSS_SYSTEM §3.5, STUDENT_GAME_UX_CORE S06):
## the lobby shows PartyReadyList (✓ / the clock); during a phase every other member shows only ✓ "Đã đạt phần" or the
## clock — never a count, never a ranking, nothing about who was wrong — and the student's own right answers only on
## their own chip; the requirement is said in words; the energy has its % in words, "Phase n/total"; a retry shows "Thử
## lại phase với câu hỏi mới" and keeps the progress of the phase before; a completed phase plays its transition in ≤ 2 s
## with the boss line.

var s: F110SceneHarness = null
var arena: BossArena = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	await s.with_party()
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200,
			F110Fixtures.encounter(s.self_id, "party", "lobby", 1))
	arena = auto_free((load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate()) as BossArena
	arena.services = s.services
	add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": "party"})
	await s.settle(2)


func after_test() -> void:
	if arena.visible:
		arena.close()
	s.dispose()


func test_lobby_shows_who_is_ready_with_a_tick_or_the_clock() -> void:
	var list: ContributionList = arena.members()
	assert_str(list.name).is_equal("PartyReadyList")
	assert_array(list.chip_texts(F110Fixtures.MATE)).contains([TranslationServer.translate("ui.boss.member.ready")])
	assert_array(list.chip_texts(F110Fixtures.THIRD)).contains([TranslationServer.translate("ui.boss.member.waiting")])
	var icon: TextureRect = list.chip(F110Fixtures.THIRD).find_child("StateIcon", true, false) as TextureRect
	assert_str(icon.texture.resource_path).ends_with("clock.svg")
	assert_bool(list.need_visible()).is_false()


func test_contributions_show_only_ticks_for_others_and_the_own_count() -> void:
	s.push("boss.phase.started", F110Fixtures.phase_started(0))
	s.push("boss.encounter.updated", F110Fixtures.updated(2, 60.0, [F110Fixtures.member(s.self_id, "MeoXanh42", true, true, false),
		F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK, true, true, true),
		F110Fixtures.member(F110Fixtures.THIRD, F110Fixtures.THIRD_NICK, true, true, false)]))
	s.push("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1"]))
	s.push("boss.answer.evaluated", F110Fixtures.evaluated("bq_p1_1", true))
	await s.settle(2)
	var list: ContributionList = arena.members()
	assert_str(list.name).is_equal("ContributionList")
	assert_array(list.chip_texts(F110Fixtures.MATE)).is_equal([F110Fixtures.MATE_NICK, TranslationServer.translate("ui.boss.member.met")])
	assert_array(list.chip_texts(F110Fixtures.THIRD)).is_equal([F110Fixtures.THIRD_NICK,
		TranslationServer.translate("ui.boss.member.trying")])
	var mine: PackedStringArray = list.chip_texts(s.self_id)
	assert_str(mine[mine.size() - 1]).is_equal(TranslationServer.translate("ui.boss.my_correct").format({"n": "1"}))
	assert_bool(list.need_visible()).is_true()
	# No digit at all on another member's chip (no count, no rank).
	var digits: RegEx = RegEx.create_from_string("[0-9]")
	for id: String in [F110Fixtures.MATE, F110Fixtures.THIRD]:
		for t: String in list.chip_texts(id):
			if t != F110Fixtures.MATE_NICK and t != F110Fixtures.THIRD_NICK:
				assert_object(digits.search(t)).is_null()
	# The server order is kept (never sorted by contribution).
	var order: PackedStringArray = []
	for c: Node in list.find_child("Chips", true, false).get_children():
		if c.name.begins_with("Member") or c is PanelContainer:
			order.append((c.find_child("Nickname", true, false) as Label).text)
	assert_array(order).is_equal([TranslationServer.translate("ui.boss.member.me"), F110Fixtures.MATE_NICK, F110Fixtures.THIRD_NICK])


func test_energy_and_phase_are_in_words_and_numbers() -> void:
	s.push("boss.phase.started", F110Fixtures.phase_started(0))
	s.push("boss.encounter.updated", F110Fixtures.updated(2, 52.0))
	await s.settle(1)
	assert_str(arena.energy_bar().text()).is_equal(TranslationServer.translate("ui.boss.energy").format({"pct": "52"}))
	assert_float(arena.energy_bar().bar_value()).is_equal_approx(0.52, 0.001)
	assert_str(arena.phase_text()).is_equal(TranslationServer.translate("ui.boss.phase").format({"n": "1", "total": "2"}))


func test_a_retry_shows_its_banner_and_keeps_the_previous_phase_progress() -> void:
	s.push("boss.phase.started", F110Fixtures.phase_started(0))
	s.push("boss.phase.completed", F110Fixtures.phase_completed(0, 50.0))
	s.push("boss.phase.started", F110Fixtures.phase_started(1, true))
	await s.settle(1)
	assert_str(arena.banner_text()).is_equal(TranslationServer.translate("ui.boss.retry_phase"))
	var e: BossEncounterModel = s.services.boss.store.encounter
	assert_int(e.phases_cleared).is_equal(1)
	assert_float(e.energy_remaining_pct).is_equal(50.0)
	assert_str(arena.phase_text()).is_equal(TranslationServer.translate("ui.boss.phase").format({"n": "2", "total": "2"}))


func test_a_completed_phase_plays_a_transition_of_at_most_two_seconds_with_the_boss_line() -> void:
	var was: bool = Settings.reduce_motion
	Settings.reduce_motion = false
	s.push("boss.phase.started", F110Fixtures.phase_started(0))
	s.push("boss.phase.completed", F110Fixtures.phase_completed(0, 50.0))
	await s.settle(1)
	assert_float(BossFigure.TRANSITION_SEC).is_less_equal(2.0)
	assert_str(arena.line_text()).is_equal(TranslationServer.translate("ui.boss.line.phase_cleared").format({"boss": arena._boss_title()}))
	assert_float(BossArena.LINE_SEC).is_less_equal(2.0)
	arena.advance(BossArena.LINE_SEC + 0.1)
	assert_str(arena.line_text()).is_empty()
	assert_int(arena.figure().phase_index).is_between(0, 1)
	Settings.reduce_motion = was


func test_the_phase_timer_of_young_grades_is_a_bar_without_seconds() -> void:
	s.push("boss.phase.started", F110Fixtures.phase_started(0, false, s.services.boss.now_ms() + 60_000))
	AppState.set_grade_band("g1_3")
	arena.advance(0.1)
	assert_bool(arena.timer().visible).is_true()
	assert_str(arena.timer().clock_text()).is_empty()
	AppState.set_grade_band("g6_7")
	arena.advance(0.1)
	assert_str(arena.timer().clock_text()).is_equal("01:00")
