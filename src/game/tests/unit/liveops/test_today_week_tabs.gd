extends GdUnitTestSuite
## T024 (F111-AC-15; QUEST_SYSTEM §2.4, AC-UX-111, FR-EVT-007): the "Hôm nay" / "Tuần này" tabs read the daily / weekly
## quests through QuestService (`listQuests` with `quest_type`), show each quest's state and its progress from the
## server `step_states` (never counted by the client), the reset time from `available_until` (ICT, locale format); "Đi"
## hands the quest to F072 (accepted when still available, then tracked); once every daily quest is done the tab says
## "Tuyệt! Em đã xong nhiệm vụ hôm nay. Nghỉ ngơi chút nhé" and suggests nothing more.

var s: F111SceneHarness = null
var tab: QuestDayTab = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	tab = auto_free(QuestDayTab.new("daily")) as QuestDayTab
	add_child(tab)


func after_test() -> void:
	s.dispose()


func test_daily_quests_come_from_quest_service_by_type() -> void:
	var r: ApiResult = await s.services.quest.load_type("daily")
	assert_bool(r.ok).is_true()
	assert_str(s.h.http.last(F111Fixtures.ROUTE_QUESTS).query).contains("quest_type=daily")
	var list: Array[QuestModel] = s.services.quest.of_type("daily")
	assert_int(list.size()).is_equal(2)
	tab.show_quests(list, s.now_sec)
	var first: PackedStringArray = tab.row_texts(F111Fixtures.QUEST_DAILY_1)
	assert_array(Array(first)).contains(["Trả lời 5 câu Toán", TranslationServer.translate("ui.events.quest_status.active"),
		TranslationServer.translate("ui.events.quest_steps").format({"done": "1", "total": "2"})])
	var reset: String = tab.reset_text()
	var until: int = LiveOpsTime.unix_of(F111Fixtures.iso(s.now_sec + F111Fixtures.DAY_SEC))
	assert_str(reset).is_equal(TranslationServer.translate("ui.events.reset_time").format(
			{"time": LiveOpsTime.ict_time(until), "date": LiveOpsTime.ict_date(until)}))
	assert_bool(tab.all_done_visible()).is_false()


func test_go_hands_the_quest_to_quest_service() -> void:
	await s.services.quest.load_type("daily")
	s.h.http.reply("POST /game/quests/*:accept", 200, (F111Fixtures.quest(F111Fixtures.QUEST_DAILY_2, "daily", "Ghé thăm Rừng Ngôn Ngữ",
		"Visit the Language Forest", "active", s.now_sec + F111Fixtures.DAY_SEC)["progress"]))
	s.h.http.reply("GET /game/quests/*", 200, F111Fixtures.quest(F111Fixtures.QUEST_DAILY_2, "daily", "Ghé thăm Rừng Ngôn Ngữ",
		"Visit the Language Forest", "active", s.now_sec + F111Fixtures.DAY_SEC))
	assert_bool(await QuestGo.go(s.services.quest, F111Fixtures.QUEST_DAILY_2)).is_true()
	assert_int(s.h.http.count("POST /game/quests/*:accept")).is_equal(1)
	assert_str(s.services.quest.tracked_quest_id).is_equal(F111Fixtures.QUEST_DAILY_2)
	assert_bool(await QuestGo.go(s.services.quest, F111Fixtures.QUEST_DAILY_1)).is_true()
	assert_int(s.h.http.count("POST /game/quests/*:accept")).is_equal(1)
	assert_str(s.services.quest.tracked_quest_id).is_equal(F111Fixtures.QUEST_DAILY_1)


func test_all_done_says_rest_and_suggests_nothing_more() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_QUESTS_DAILY).reply(F111Fixtures.ROUTE_QUESTS_DAILY, 200, F111Fixtures.daily(s.now_sec, true))
	await s.services.quest.load_type("daily")
	tab.show_quests(s.services.quest.of_type("daily"), s.now_sec)
	assert_bool(tab.all_done_visible()).is_true()
	assert_str(tab.all_done_text()).is_equal(TranslationServer.translate("ui.events.all_done"))
	assert_int(tab.go_buttons().size()).is_equal(0)
	var weekly: QuestDayTab = auto_free(QuestDayTab.new("weekly")) as QuestDayTab
	add_child(weekly)
	await s.services.quest.load_type("weekly")
	weekly.show_quests(s.services.quest.of_type("weekly"), s.now_sec)
	assert_array(Array(weekly.row_texts(F111Fixtures.QUEST_WEEKLY))).contains(
			[TranslationServer.translate("ui.events.quest_steps").format({"done": "4", "total": "7"})])
