extends GdUnitTestSuite
## F110 compact polish (F125 phone canvas, STUDENT_GAME_UX_CORE S06 §6b; F110-AC-02, AC-05, AC-06, AC-09, AC-16):
## - the boss HUD is two short rows on a phone: name · round · timer · energy "52%" · buttons, then the chips; a chip = the ✓ / clock icon + the
##   nickname in bold (+ the student's own count in its pill), the state words in its tooltip — except "Đang kết nối lại",
##   always written; on a big screen the words stay beside the nickname;
## - the requirement in words is never shown in a solo boss; on a phone it is said in the party lobby, not under the
##   chips during a round;
## - on a phone the message strip (boss line, retry, reconnecting) and the answer feedback sit beside the chips in the
##   HUD (not above the question); the feedback takes the place of the boss line and the start line is skipped, so the
##   answer tiles keep their room (F110Matrix.answers_cut checks they are on screen);
## - the Vietnamese boss strings say "hiệp", never the English "phase";
## - the co-op step on a phone has two tabs ("Vai của em" / "Bảng chung của nhóm") in its action row; a big screen shows
##   everything at once without tabs.

var s: F110SceneHarness = null
var arena: BossArena = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	if arena != null and is_instance_valid(arena) and arena.visible:
		arena.close()
	arena = null
	AppState.set_compact(false)
	s.dispose()


func _open_boss(mode: String, status: String) -> void:
	if mode == "party":
		await s.with_party()
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200,
			F110Fixtures.encounter(s.self_id, mode, status, 1))
	arena = auto_free((load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate()) as BossArena
	arena.services = s.services
	add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": mode})
	await s.settle(2)


func _members(mate_connected: bool = true) -> Array:
	return [F110Fixtures.member(s.self_id, "MeoXanh42", true, true, false),
		F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK, true, mate_connected, true),
		F110Fixtures.member(F110Fixtures.THIRD, F110Fixtures.THIRD_NICK, true, true, false)]


func _start_phase(mate_connected: bool = true) -> void:
	s.push("boss.phase.started", F110Fixtures.phase_started(0))
	s.push("boss.encounter.updated", F110Fixtures.updated(2, 52.0, _members(mate_connected)))
	s.push("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1"]))
	await s.settle(2)


func test_compact_chips_are_icon_and_nickname_with_the_words_in_the_tooltip() -> void:
	AppState.set_compact(true)
	await _open_boss("party", "lobby")
	await _start_phase(false)
	var list: ContributionList = arena.members()
	assert_array(list.shown_texts(F110Fixtures.THIRD)).is_equal([F110Fixtures.THIRD_NICK])
	var tip: String = list.chip(F110Fixtures.THIRD).tooltip_text
	assert_str(tip).contains(TranslationServer.translate("ui.boss.member.trying"))
	# A reconnecting member keeps its words on screen (F110-AC-09), faded.
	assert_array(list.shown_texts(F110Fixtures.MATE)).is_equal([F110Fixtures.MATE_NICK,
		TranslationServer.translate("ui.boss.member.reconnecting")])
	assert_str(String(list.chip(F110Fixtures.MATE).theme_type_variation)).is_equal("PartyMemberCardAway")
	# The own chip: "Em" + the count in its own pill (separate parts, never run together).
	var mine: PackedStringArray = list.shown_texts(s.self_id)
	assert_array(mine).is_equal([TranslationServer.translate("ui.boss.member.me"),
		TranslationServer.translate("ui.boss.my_correct").format({"n": "0"})])
	var count: Label = list.chip(s.self_id).find_child("MyCorrect", true, false) as Label
	assert_str(String(count.theme_type_variation)).is_equal("SectionTag")
	var nick: Label = list.chip(s.self_id).find_child("Nickname", true, false) as Label
	assert_str(String(nick.theme_type_variation)).is_equal("ChatNameLabel")
	# The energy bar moved up into the first HUD row and says only the % (its words in the tooltip).
	assert_str(String(arena.energy_bar().get_parent().name)).is_equal("Top")
	assert_str(arena.energy_bar().text()).is_equal(TranslationServer.translate("ui.boss.energy_short").format({"pct": "52"}))
	assert_str(arena.energy_bar().tooltip_text).is_equal(TranslationServer.translate("ui.boss.energy").format({"pct": "52"}))
	assert_bool(list.need_visible()).is_false()


func test_big_screen_chips_keep_the_words_and_the_energy_row() -> void:
	await _open_boss("party", "lobby")
	await _start_phase()
	var list: ContributionList = arena.members()
	assert_array(list.shown_texts(F110Fixtures.THIRD)).is_equal([F110Fixtures.THIRD_NICK,
		TranslationServer.translate("ui.boss.member.trying")])
	assert_str(String(arena.energy_bar().get_parent().name)).is_not_equal("Top")
	assert_str(arena.energy_bar().text()).is_equal(TranslationServer.translate("ui.boss.energy").format({"pct": "52"}))
	assert_bool(list.need_visible()).is_true()


func test_requirement_words_in_the_lobby_on_a_phone_and_never_in_solo() -> void:
	AppState.set_compact(true)
	await _open_boss("party", "lobby")
	assert_bool(arena.members().need_visible()).is_true()
	assert_bool(ContributionList.need_shown(false, false, false)).is_false()
	assert_bool(ContributionList.need_shown(true, false, true)).is_false()
	assert_bool(ContributionList.need_shown(false, true, true)).is_false()


func test_solo_never_says_the_party_requirement() -> void:
	await _open_boss("solo", "phase_active")
	await _start_phase()
	assert_bool(arena.members().need_visible()).is_false()


func test_compact_feedback_is_a_strip_under_the_hud_and_replaces_the_boss_line() -> void:
	AppState.set_compact(true)
	await _open_boss("party", "lobby")
	await _start_phase()
	# Compact: no start line (the question keeps its room); a cleared round says its line in the strip (F110-AC-07).
	assert_str(arena.banner_text()).is_empty()
	s.push("boss.phase.completed", F110Fixtures.phase_completed(0, 50.0))
	await s.settle(1)
	assert_str(arena.banner_text()).is_not_empty()
	s.push("boss.answer.evaluated", F110Fixtures.evaluated("bq_p1_1", false))
	await s.settle(2)
	assert_str(arena.feedback_text()).starts_with(TranslationServer.translate("ui.boss.strike_back"))
	assert_str(arena.feedback_text()).not_contains("\n")
	assert_str(arena.banner_text()).is_empty()
	var feedback: Control = arena.find_child("Feedback", true, false) as Control
	assert_bool(arena.question_panel().is_ancestor_of(feedback)).is_false()
	# Beside the chips in the second HUD row, with the banner strip (no strip of its own above the question).
	assert_object(feedback.get_parent()).is_same(arena.find_child("Second", true, false))
	assert_object((arena.find_child("Banner", true, false) as Node).get_parent()).is_same(feedback.get_parent())
	# Back on a big screen it returns above the question, in the board.
	AppState.set_compact(false)
	await s.settle(1)
	assert_bool(arena.find_child("Board", true, false).is_ancestor_of(feedback)).is_true()


func test_vietnamese_boss_strings_say_hiep_never_phase() -> void:
	var vi: Dictionary = (preload("res://tools/lib/po_file.gd")).parse(FileAccess.get_file_as_string("res://locale/vi.po"))
	assert_str(String(vi["ui.boss.phase"])).is_equal("Hiệp {n}/{total}")
	var english: PackedStringArray = []
	for key: Variant in vi.keys():
		var k: String = String(key)
		if (k.begins_with("ui.boss.") or k.begins_with("ui.error.BOSS_")) and String(vi[key]).to_lower().contains("phase"):
			english.append(k)
	assert_array(english).is_empty()
	Settings.set_locale("vi-VN", false)
	await _open_boss("solo", "phase_active")
	await _start_phase()
	assert_str(arena.phase_text()).starts_with("Hiệp ")


func test_compact_coop_step_has_two_tabs_in_its_action_row() -> void:
	await s.with_party()
	s.services.quest.quests.clear()
	s.services.quest.quests.append(QuestModel.from_dict(F110Fixtures.coop_quest()))
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id))
	await s.settle(3)
	AppState.set_compact(true)
	var panel: InterdependentStepPanel = auto_free((load("res://scenes/quest/InterdependentStepPanel.tscn") as PackedScene) \
			.instantiate()) as InterdependentStepPanel
	panel.services = s.services
	add_child(panel)
	await panel.open({"quest_id": F110Fixtures.QUEST_ID, "step_id": F110Fixtures.STEP_ID})
	await s.settle(2)
	assert_str(panel.current_tab()).is_equal(InterdependentStepPanel.TAB_MINE)
	assert_bool(panel.tab_button(InterdependentStepPanel.TAB_MINE).is_visible_in_tree()).is_true()
	assert_bool(panel.actions_row().is_ancestor_of(panel.tab_button(InterdependentStepPanel.TAB_BOARD))).is_true()
	assert_bool(panel.my_card(F110Fixtures.ROLE_MEASURE).is_visible_in_tree()).is_true()
	assert_bool(panel.board().is_visible_in_tree()).is_false()
	panel.tab_button(InterdependentStepPanel.TAB_BOARD).pressed.emit()
	await s.settle(1)
	assert_str(panel.current_tab()).is_equal(InterdependentStepPanel.TAB_BOARD)
	assert_bool(panel.board().is_visible_in_tree()).is_true()
	assert_bool(panel.my_card(F110Fixtures.ROLE_MEASURE).is_visible_in_tree()).is_false()
	assert_bool(panel.final_button().is_visible_in_tree()).is_true()
	# A big screen: no tabs, the role card and the board at once.
	AppState.set_compact(false)
	await s.settle(1)
	assert_bool(panel.tab_button(InterdependentStepPanel.TAB_MINE).is_visible_in_tree()).is_false()
	assert_bool(panel.board().is_visible_in_tree()).is_true()
	assert_bool(panel.my_card(F110Fixtures.ROLE_MEASURE).is_visible_in_tree()).is_true()
	panel.close()


## The English screens on both phones: the answer tiles on screen without scrolling, the role card whole, nothing cut
## (F110Matrix: BandLayoutProbe + answers_cut + the role card check; the Vietnamese ones run in test_f110_band_matrix).
func test_english_phone_screens_keep_the_answers_and_the_role_card_on_screen() -> void:
	Settings.set_locale("en-US", false)
	var phones: Dictionary = {"844x390": BandLayoutProbe.SIZES["844x390"], "740x360": BandLayoutProbe.SIZES["740x360"]}
	var failures: PackedStringArray = []
	for view: String in ["boss_question", "boss_wrong", "boss_reconnect", "boss_retry", "coop_step"]:
		failures.append_array(await F110Matrix.run(self, s, view, phones, PackedStringArray(["g1_3", "g8_9"])))
	Settings.set_locale("vi-VN", false)
	assert_array(failures).override_failure_message("
".join(failures)).is_empty()
