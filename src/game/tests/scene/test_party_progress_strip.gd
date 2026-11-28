extends GdUnitTestSuite
## T018 (F110-AC-12, F110-AC-13; QUEST_SYSTEM §6, PARTY_SYSTEM §7.1, US-PTY-003): `quest.progress.updated` with
## `party_progress` shows the PartyProgressStrip on the Quest Tracker — the group share (bar + "75%") and whether THIS
## student reached their part (✓ / the clock) — a frame step done for the party reads "Cả nhóm đã xong"; no number of
## another member is shown; a learning step of a party quest starts the student's own activity (`startQuestActivity` →
## ActivityHost, only for the one who does it).

var s: F110SceneHarness = null
var tracker: PanelContainer = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g1_3")
	s.services.quest.quests.clear()
	s.services.quest.quests.append(QuestModel.from_dict(F110Fixtures.coop_quest()))
	s.services.quest.track(F110Fixtures.QUEST_ID)
	tracker = auto_free((load("res://scenes/quest/QuestTracker.tscn") as PackedScene).instantiate()) as PanelContainer
	tracker.set("service", s.services.quest)
	tracker.set("coop", s.services.coop_quest)
	add_child(tracker)
	await s.settle(1)


func after_test() -> void:
	s.dispose()


func _strip() -> PartyProgressStrip:
	return tracker.get("strip") as PartyProgressStrip


func test_the_strip_shows_the_group_share_and_my_part_only() -> void:
	assert_bool(_strip().visible).is_false()
	s.push("quest.progress.updated", F110Fixtures.progress(0.75, false))
	await s.settle(1)
	var strip: PartyProgressStrip = _strip()
	assert_bool(strip.visible).is_true()
	assert_float(strip.bar_value()).is_equal_approx(0.75, 0.001)
	assert_str(strip.percent_text()).is_equal(TranslationServer.translate("ui.quest.coop.group_progress").format({"pct": "75"}))
	assert_str(strip.mine_text()).is_equal("ui.quest.coop.my_part_needed")
	s.push("quest.progress.updated", F110Fixtures.progress(0.8, true))
	await s.settle(1)
	assert_str(strip.mine_text()).is_equal("ui.quest.coop.my_part_done")
	# Only the group share and the own part: no other number on the strip.
	var digits: RegEx = RegEx.create_from_string("[0-9]+")
	var numbers: Array[String] = []
	for n: Node in strip.find_children("*", "Label", true, false):
		for m: RegExMatch in digits.search_all(TranslationServer.translate((n as Label).text)):
			numbers.append(m.get_string())
	assert_array(numbers).is_equal(["80"])


func test_a_frame_step_done_for_the_party_reads_done_for_everyone() -> void:
	s.push("quest.progress.updated", F110Fixtures.progress(1.0, false, true))
	await s.settle(1)
	assert_str(_strip().mine_text()).is_equal("ui.quest.coop.shared_done")
	assert_str(_strip().percent_text()).is_empty()


func test_the_rest_party_progress_of_the_quest_fills_the_strip_too() -> void:
	var body: Dictionary = F110Fixtures.coop_quest()
	(body["progress"] as Dictionary)["party_progress"] = [{"step_id": "collect_answers", "collective_progress": 0.5,
		"my_contribution_met": true}]
	s.services.quest._merge(body)
	await s.settle(1)
	assert_float(_strip().bar_value()).is_equal_approx(0.5, 0.001)


func test_a_learning_step_starts_the_students_own_activity() -> void:
	var opened: Array = []
	s.services.quest.intent_sink = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	s.h.http.reply(F110Fixtures.ROUTE_START_ACTIVITY, 201, {"attempt_id": "0193b0a0-0000-7000-8000-000000000f01",
		"activity_id": "0193b0a0-0000-7000-8000-000000000f02", "context_id": F110Fixtures.QUEST_ID})
	var r: ApiResult = await s.services.coop_quest.start_my_activity(F110Fixtures.QUEST_ID, "collect_answers")
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_START_ACTIVITY)
	assert_str(rec.path).is_equal("/game/quests/%s/steps/collect_answers/activities:start" % F110Fixtures.QUEST_ID)
	# The body names no other student (the server takes the student from the session).
	assert_bool(rec.raw_body.contains(F110Fixtures.MATE)).is_false()
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("activity_host")
