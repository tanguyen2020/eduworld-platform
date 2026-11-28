extends GdUnitTestSuite
## T021 (F110-AC-14, F110-AC-16, F110-AC-17, F110-AC-18; QUEST_SYSTEM §3.2): the panel of an `interdependent` step
## shows the student's own role card with "Chia sẻ dữ kiện", the other role by nickname + name only; a card shared by a
## member appears on the board as soon as the server says so (one frame after the read); "Chat nhóm" only with
## `share_mode = clue_card_and_chat`; "Trả lời câu cuối" opens the student's own activity; a role taken over after a
## member left past the grace shows "Em giữ thêm vai {role}" and the shared cards stay; alone (`party_optional`) every
## role and card is the student's.

var s: F110SceneHarness = null
var panel: InterdependentStepPanel = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g1_3")
	await s.with_party()
	s.services.quest.quests.clear()
	s.services.quest.quests.append(QuestModel.from_dict(F110Fixtures.coop_quest()))
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id))
	await s.settle(3)
	panel = auto_free((load("res://scenes/quest/InterdependentStepPanel.tscn") as PackedScene).instantiate()) as InterdependentStepPanel
	panel.services = s.services
	add_child(panel)
	await panel.open({"quest_id": F110Fixtures.QUEST_ID, "step_id": F110Fixtures.STEP_ID})
	await s.settle(2)


func after_test() -> void:
	s.dispose()


func test_my_card_with_share_and_the_other_role_by_nickname_only() -> void:
	assert_int(panel.my_card_count()).is_equal(1)
	var card: ClueCard = panel.my_card(F110Fixtures.ROLE_MEASURE)
	assert_bool(card.share_button().visible).is_true()
	assert_array(card.texts()).contains(["Người đo", "Sợi dây dài 12 m."])
	var team: String = " ".join(panel.team_texts())
	assert_str(team).contains(F110Fixtures.MATE_NICK)
	# The other member's clue text is not on screen.
	assert_bool(team.contains("Cây cầu")).is_false()
	assert_int(panel.board().card_count()).is_equal(0)
	assert_bool(panel.assign_button().visible).is_false()


func test_a_shared_card_appears_on_the_board_right_after_the_message() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false),
			F110Fixtures.clue(F110Fixtures.ROLE_MAP, true, F110Fixtures.MATE_NICK)]))
	var started: int = Time.get_ticks_msec()
	s.push("quest.clue.shared", F110Fixtures.clue_shared(F110Fixtures.ROLE_MAP, F110Fixtures.MATE_NICK))
	await s.settle(2)
	assert_int(panel.board().card_count()).is_equal(1)
	assert_int(Time.get_ticks_msec() - started).is_less(1000)
	assert_array(panel.board().roles()).is_equal([F110Fixtures.ROLE_MAP])


func test_share_button_shares_my_card() -> void:
	panel.my_card(F110Fixtures.ROLE_MEASURE).share_button().pressed.emit()
	await s.settle(3)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_SHARE)).is_equal(1)
	assert_array(panel.board().roles()).is_equal([F110Fixtures.ROLE_MEASURE])
	assert_bool(panel.my_card(F110Fixtures.ROLE_MEASURE).share_button().visible).is_false()


func test_chat_only_when_the_step_shares_by_card_and_chat() -> void:
	assert_bool(panel.chat_button().visible).is_false()
	var q: QuestModel = s.services.quest.quest(F110Fixtures.QUEST_ID)
	q.steps[1]["share_mode"] = "clue_card_and_chat"
	panel.render()
	assert_bool(panel.chat_button().visible).is_true()


func test_answer_final_opens_my_own_activity() -> void:
	var opened: Array = []
	s.services.quest.intent_sink = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	s.h.http.reply(F110Fixtures.ROUTE_START_ACTIVITY, 201, {"attempt_id": "0193b0a0-0000-7000-8000-000000000f01",
		"activity_id": "0193b0a0-0000-7000-8000-000000000f02", "context_id": F110Fixtures.QUEST_ID})
	panel.final_button().pressed.emit()
	await s.settle(3)
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("activity_host")
	assert_str(String((opened[0][1] as Dictionary)["step_id"])).is_equal(F110Fixtures.STEP_ID)
	assert_str(s.h.http.last(F110Fixtures.ROUTE_START_ACTIVITY).path).contains("/steps/%s/activities:start" % F110Fixtures.STEP_ID)


func test_a_role_taken_over_shows_a_toast_and_the_shared_cards_stay() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false),
			F110Fixtures.clue(F110Fixtures.ROLE_MAP, true, F110Fixtures.MATE_NICK)]))
	s.push("quest.clue.shared", F110Fixtures.clue_shared(F110Fixtures.ROLE_MAP, F110Fixtures.MATE_NICK))
	await s.settle(3)
	# CaHeoVui07 left past the grace: the server gives the map role to this student.
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE, F110Fixtures.ROLE_MAP]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false)]))
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id, PackedStringArray([F110Fixtures.ROLE_MEASURE, F110Fixtures.ROLE_MAP])))
	await s.settle(3)
	assert_str(panel.notice_text()).is_equal(TranslationServer.translate("ui.quest.clue.extra_role").format({"role": "Người đọc bản đồ"}))
	assert_array(panel.board().roles()).is_equal([F110Fixtures.ROLE_MAP])


func test_alone_every_role_and_card_is_mine() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE, F110Fixtures.ROLE_MAP]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false),
			F110Fixtures.clue(F110Fixtures.ROLE_MAP, false)]))
	var alone: Dictionary = F110Fixtures.roles(s.self_id, PackedStringArray([F110Fixtures.ROLE_MEASURE, F110Fixtures.ROLE_MAP]))
	for a: Variant in alone["assignments"] as Array:
		(a as Dictionary)["student_id"] = s.self_id
		(a as Dictionary)["nickname"] = "MeoXanh42"
	s.services.coop_quest.store.clear()
	s.push("quest.roles.assigned", alone)
	await s.settle(4)
	assert_int(panel.my_card_count()).is_equal(2)
	assert_str(" ".join(panel.team_texts())).is_equal("ui.quest.coop.alone")


func test_the_leader_waiting_for_the_choice_assigns_roles() -> void:
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id, PackedStringArray([F110Fixtures.ROLE_MEASURE]), true))
	await s.settle(3)
	assert_bool(panel.assign_button().visible).is_true()
	panel.assign_button().pressed.emit()
	var dialog: RoleAssignDialog = panel.assign_dialog()
	assert_bool(dialog.visible).is_true()
	assert_int(dialog.choice().size()).is_equal(2)
	dialog.footer_button(RoleAssignDialog.ACT_ASSIGN).pressed.emit()
	await s.settle(3)
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_ROLES)
	assert_object(rec).is_not_null()
	assert_int((rec.body["assignments"] as Array).size()).is_equal(2)
