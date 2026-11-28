extends GdUnitTestSuite
## T021 (F111-AC-09, F111-AC-12, F111-AC-13; EVENT_SEASON_SYSTEM §5, §6, §9, ADR-0095, AGENTS R19): an EventCard says the
## time left in whole days ("Còn 5 ngày", "Hôm nay là ngày cuối", "Sắp diễn ra") — no hour / minute / second, nothing
## blinking; a paused event says "Sự kiện tạm dừng" and its quests and milestones are hidden; an ended event lets only
## reached milestones be claimed and says "Phần thưởng chưa nhận sẽ được gửi tự động"; the CommunityGoalBar shows
## total / target, the 25/50/75/100 marks, "Em đã góp sức", never a personal rank; the per-grade rows (only those the
## server sent) are sorted by grade, without names or ranks, and not shown to lớp 1–3.

const NOW: int = 1_763_600_000
const CLOCK_WORDS: PackedStringArray = ["giờ", "phút", "giây", "hour", "minute", "second"]

var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	Settings.set_locale("vi-VN", false)


func after_test() -> void:
	AppState.set_grade_band(_band)


func _event(status: String = "live", detail: bool = false, by_grade: Array = []) -> LiveEventModel:
	var d: Dictionary = F111Fixtures.detail(NOW, true, by_grade) if detail else \
			(F111Fixtures.events(NOW)["data"] as Array)[0] as Dictionary
	d["status"] = status
	return LiveEventModel.from_dict(d)


func _no_clock(texts: PackedStringArray) -> void:
	for t: String in texts:
		for w: String in CLOCK_WORDS:
			assert_bool(t.to_lower().contains(w)).override_failure_message("'%s' in '%s'" % [w, t]).is_false()
		assert_bool(RegEx.create_from_string("\\d+:\\d\\d").search(t) == null).override_failure_message(t).is_true()


func test_event_card_counts_days_only_and_never_blinks() -> void:
	var card: EventCard = auto_free(EventCard.new()) as EventCard
	add_child(card)
	card.show_event(_event(), NOW)
	var texts: PackedStringArray = card.texts()
	assert_array(Array(texts)).contains(["Lễ hội Trăng Rằm", "Còn 5 ngày"])
	_no_clock(texts)
	assert_array(card.find_children("*", "AnimationPlayer", true, false)).is_empty()
	var upcoming: LiveEventModel = LiveEventModel.from_dict((F111Fixtures.events(NOW)["data"] as Array)[2])
	card.show_event(upcoming, NOW)
	assert_array(Array(card.texts())).contains(["Sắp diễn ra"])
	card.show_event(_event("suspended"), NOW)
	assert_array(Array(card.texts())).contains(["Sự kiện tạm dừng"])


func test_paused_detail_hides_the_event_parts() -> void:
	var panel: EventDetailPanel = auto_free(EventDetailPanel.new()) as EventDetailPanel
	add_child(panel)
	panel.show_event(_event("suspended", true), NOW, "g4_5", {})
	assert_bool(panel.paused_visible()).is_true()
	assert_bool(panel.quests_visible()).is_false()
	assert_bool(panel.milestones_visible()).is_false()
	assert_bool(panel.join_button().visible).is_false()


func test_ended_detail_claims_only_reached_milestones_until_the_deadline() -> void:
	var panel: EventDetailPanel = auto_free(EventDetailPanel.new()) as EventDetailPanel
	add_child(panel)
	panel.show_event(_event("ended", true), NOW, "g4_5", {})
	assert_bool(panel.join_button().visible).is_false()
	assert_bool(panel.ended_note_visible()).is_true()
	assert_object(panel.claim_button("m1")).is_not_null()
	assert_bool(panel.claim_button("m1").visible).is_true()
	assert_object(panel.claim_button("m2")).is_null()
	_no_clock(panel.texts())
	# After the claim deadline nothing can be claimed any more.
	panel.show_event(_event("ended", true), NOW + 30 * 86400, "g4_5", {})
	assert_object(panel.claim_button("m1")).is_null()


func test_community_goal_bar_has_no_rank_and_sorted_grades_hidden_from_g1_3() -> void:
	var bar: CommunityGoalBar = auto_free(CommunityGoalBar.new()) as CommunityGoalBar
	add_child(bar)
	var g: CommunityGoalModel = CommunityGoalModel.from_dict(F111Fixtures.goal(6200, [{"grade_level": 7, "progress": 300},
		{"grade_level": 4, "progress": 900}]))
	bar.show_goal(g, "g6_7")
	var texts: PackedStringArray = bar.texts()
	assert_array(Array(texts)).contains(["Mục tiêu cả trường", "6.200/10.000", "Em đã góp sức"])
	assert_bool(bar.mark_reached(25) and bar.mark_reached(50)).is_true()
	assert_bool(bar.mark_reached(75)).is_false()
	assert_array(Array(bar.grade_texts())).is_equal(["Khối 4", "Khối 7"])
	for t: String in texts:
		for w: String in ["hạng", "top", "#", "xếp"]:
			assert_bool(t.to_lower().contains(w)).override_failure_message(t).is_false()
	bar.show_goal(g, "g1_3")
	assert_array(Array(bar.grade_texts())).is_empty()
	var none: CommunityGoalModel = CommunityGoalModel.from_dict(F111Fixtures.goal(100, [], false))
	bar.show_goal(none, "g4_5")
	assert_array(Array(bar.texts())).contains([TranslationServer.translate("ui.events.not_contributed")])
