extends GdUnitTestSuite
## F096-AC-10 (STUDENT_GAME_UX_CORE S03 "Hướng dẫn lần đầu"): Mây's guide shows on the first entry on a device or right
## after S02, has at most 3 steps each with a translated sentence and an icon, "Tiếp" walks to the end and "Bỏ qua"
## closes at once; NextStepHint (F096-AC-11) suggests school work first, else the World Map.


func test_shows_on_a_new_device_or_after_character_creation() -> void:
	assert_bool(OnboardingGuide.should_show(false, false)).is_true()
	assert_bool(OnboardingGuide.should_show(true, true)).is_true()
	assert_bool(OnboardingGuide.should_show(true, false)).is_false()


func test_at_most_three_short_steps_with_icons() -> void:
	var guide: OnboardingGuide = OnboardingGuide.new()
	assert_int(guide.count()).is_less_equal(OnboardingGuide.MAX_STEPS)
	assert_int(OnboardingGuide.MAX_STEPS).is_equal(3)
	for step: Dictionary in OnboardingGuide.STEPS:
		assert_bool(ResourceLoader.exists(String(step["icon"]))).override_failure_message(String(step["icon"])).is_true()
		for locale: String in ["vi-VN", "en-US"]:
			TranslationServer.set_locale(locale)
			var text: String = tr(String(step["key"]))
			assert_bool(text.begins_with("ui.")).override_failure_message(String(step["key"])).is_false()
			# One short sentence or two: grade 1-3 reading (STUDENT_GAME_UX §4.3).
			assert_int(text.split(" ", false).size()).override_failure_message(text).is_less_equal(16)
	TranslationServer.set_locale("vi-VN")


func test_next_walks_to_the_end_and_skip_closes() -> void:
	var guide: OnboardingGuide = OnboardingGuide.new()
	assert_bool(guide.is_last()).is_false()
	for i: int in guide.count() - 1:
		assert_bool(guide.next()).is_true()
	assert_bool(guide.is_last()).is_true()
	assert_bool(guide.next()).is_false()
	assert_bool(guide.finished).is_true()
	guide.restart()
	assert_int(guide.step).is_equal(0)
	guide.skip()
	assert_bool(guide.finished).is_true()


func test_next_step_prefers_school_work_then_exploring() -> void:
	var assigned: Dictionary = NextStepHint.for_assignments(2)
	assert_str(String(assigned["kind"])).is_equal(NextStepHint.KIND_ASSIGNED)
	assert_str(String(assigned["screen_id"])).is_equal("quest_journal")
	assert_str(String((assigned["screen_args"] as Dictionary)["tab"])).is_equal("assigned")
	assert_int(int((assigned["args"] as Dictionary)["count"])).is_equal(2)
	var explore: Dictionary = NextStepHint.for_assignments(0)
	assert_str(String(explore["kind"])).is_equal(NextStepHint.KIND_EXPLORE)
	assert_str(String(explore["screen_id"])).is_equal("world_map")
	for hint: Dictionary in [assigned, explore]:
		assert_bool(ResourceLoader.exists(String(hint["icon"]))).is_true()
		assert_bool(UiScreenRegistry.has(hint["screen_id"] as StringName)).is_true()
