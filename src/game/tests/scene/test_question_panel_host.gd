extends GdUnitTestSuite
## T011 (F110-AC-04, F110-AC-05; CHALLENGE_BOSS_SYSTEM §2.3, §3.4, ADR-0016): the boss question is drawn by the F063
## renderer of its type inside QuestionPanelHost; one tap sends `boss.answer.submit` with every field (encounter,
## phase, requirement, question instance, the raw response, `client_elapsed_ms`); while the server has not acked the
## question is locked (no second submit); an answer that could not be sent is sent again with the SAME
## `question_instance_id`; right / wrong are shown to this student only — light rays or the playful strike back with the
## explanation, never the word "Sai"; the energy moves only with the server; `BOSS_PHASE_CLOSED` closes the question.

var s: F110SceneHarness = null
var arena: BossArena = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	arena = auto_free((load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate()) as BossArena
	arena.services = s.services
	add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": "solo", "title": "Rồng Phân Số"})
	s.ack_last("boss.encounter.ready")
	s.push("boss.phase.started", F110Fixtures.phase_started(0, false, s.services.boss.now_ms() + 90_000))
	s.push("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1", "bq_p1_2"]))
	await s.settle(3)


func after_test() -> void:
	if arena.visible:
		arena.close()
	s.dispose()


func test_the_question_uses_the_f063_renderer_and_one_tap_sends_every_field() -> void:
	var panel: QuestionPanelHost = arena.question_panel()
	assert_object(panel.renderer).is_not_null()
	assert_bool(panel.renderer is QuestionRendererBase).is_true()
	assert_str(panel.question_instance_id).is_equal("bq_p1_1")
	panel.choose(1)
	await s.settle(2)
	var sent: Array[Dictionary] = s.sent("boss.answer.submit")
	assert_int(sent.size()).is_equal(1)
	var p: Dictionary = sent[0]["payload"] as Dictionary
	assert_str(String(p["encounter_id"])).is_equal(F110Fixtures.ENCOUNTER_ID)
	assert_str(String(p["phase_id"])).is_equal("p1")
	assert_str(String(p["requirement_id"])).is_equal("r1")
	assert_str(String(p["question_instance_id"])).is_equal("bq_p1_1")
	assert_str(String((p["response"] as Dictionary)["option_id"])).is_equal("o_b")
	assert_bool(p.has("client_elapsed_ms")).is_true()
	# The payload has no score, energy or contribution of the client (R5).
	for k: String in ["score", "is_correct", "energy_remaining_pct", "contribution_met"]:
		assert_bool(p.has(k)).is_false()


func test_the_question_is_locked_while_waiting_for_the_ack() -> void:
	var panel: QuestionPanelHost = arena.question_panel()
	panel.choose(0)
	await s.settle(2)
	assert_bool(panel.is_locked()).is_true()
	assert_str(panel.state_text()).is_equal("ui.boss.sending")
	panel.choose(2)
	await s.settle(2)
	assert_int(s.sent("boss.answer.submit").size()).is_equal(1)
	var ok: bool = await s.services.boss.submit({"option_id": "o_c"})
	assert_bool(ok).is_false()
	assert_int(s.sent("boss.answer.submit").size()).is_equal(1)


func test_an_answer_not_taken_is_sent_again_with_the_same_question_instance() -> void:
	var panel: QuestionPanelHost = arena.question_panel()
	panel.choose(1)
	await s.settle(2)
	s.ack_last("boss.answer.submit", false, "INVALID_COMMAND")
	await s.settle(2)
	assert_bool(panel.is_locked()).is_false()
	assert_str(panel.question_instance_id).is_equal("bq_p1_1")
	panel.choose(1)
	await s.settle(2)
	var sent: Array[Dictionary] = s.sent("boss.answer.submit")
	assert_int(sent.size()).is_equal(2)
	assert_str(String((sent[1]["payload"] as Dictionary)["question_instance_id"])).is_equal("bq_p1_1")


func test_right_and_wrong_are_shown_to_this_student_only_without_the_word_wrong() -> void:
	var fig: BossFigure = arena.figure()
	s.push("boss.encounter.updated", F110Fixtures.updated(2, 70.0))
	arena.question_panel().choose(1)
	await s.settle(2)
	s.ack_last("boss.answer.submit")
	s.push("boss.answer.evaluated", F110Fixtures.evaluated("bq_p1_1", true))
	await s.settle(2)
	assert_int(fig.hits).is_equal(1)
	assert_str(arena.feedback_text()).is_equal(TranslationServer.translate("ui.boss.hit"))
	# The energy did not move with the client's own answer: only the server's update moves it.
	assert_float(arena.energy_bar().value).is_equal(70.0)
	assert_str(arena.question_panel().question_instance_id).is_equal("bq_p1_2")
	arena.question_panel().choose(0)
	await s.settle(2)
	s.ack_last("boss.answer.submit")
	s.push("boss.answer.evaluated", F110Fixtures.evaluated("bq_p1_2", false))
	await s.settle(2)
	assert_int(fig.strikes).is_equal(1)
	var text: String = arena.feedback_text()
	assert_str(text).contains(TranslationServer.translate("ui.boss.strike_back"))
	assert_str(text).contains("6 nhóm 3 là 18.")
	for word: String in ["Sai", "sai", "Wrong", "wrong"]:
		assert_bool(text.contains(word)).override_failure_message("negative word on screen: %s" % word).is_false()
	assert_float(arena.energy_bar().value).is_equal(70.0)
	s.push("boss.encounter.updated", F110Fixtures.updated(3, 40.0))
	assert_float(arena.energy_bar().value).is_equal(40.0)
	assert_str(arena.energy_bar().text()).contains("40")


func test_phase_closed_closes_the_question_and_waits_for_the_next_phase() -> void:
	arena.question_panel().choose(1)
	await s.settle(2)
	s.ack_last("boss.answer.submit", false, "BOSS_PHASE_CLOSED")
	await s.settle(2)
	var store: BossStore = s.services.boss.store
	assert_str(store.question("bq_p1_1").state).is_equal(BossQuestionModel.STATE_CLOSED)
	assert_str(arena.question_panel().question_instance_id).is_equal("bq_p1_2")
	s.push("boss.phase.completed", F110Fixtures.phase_completed(0, 50.0))
	await s.settle(2)
	assert_object(arena.question_panel().renderer).is_null()
	assert_str(arena.question_panel().state_text()).is_equal("ui.boss.next_phase_soon")
	assert_float(arena.energy_bar().value).is_equal(50.0)
