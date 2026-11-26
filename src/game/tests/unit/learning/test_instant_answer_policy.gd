extends GdUnitTestSuite
## F098-AC-01, F098-AC-02, F098-AC-08 (STUDENT_GAME_UX_LEARNING §4.1b, ACTIVITY_ENGINE §8.1): InstantAnswerPolicy sends
## on tap only when the server's attempt shows per-question feedback right away (feedback_timing immediate, results
## released at once, not exam/diagnostic/competition) and the item is answered by one tap (single choice, true/false
## with one statement); the bands of `eduworld/learning/tap_confirm_bands` (default none) confirm with a second tap.
## Contract fixtures (TEST-ONLY).

var _saved_bands: Variant = null


func before_test() -> void:
	_saved_bands = ProjectSettings.get_setting(InstantAnswerPolicy.SETTING, PackedStringArray())


func after_test() -> void:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, _saved_bands)


func _attempt(mode: String = "practice", timing: String = "immediate", release: String = "immediate") -> AttemptModel:
	var data: Dictionary = LearningFixtures.fresh_attempt()
	data["mode"] = mode
	data["feedback_timing"] = timing
	data["result_release"] = release
	return AttemptModel.from_dict(data)


func test_practice_like_modes_with_immediate_feedback_allow_tap_to_answer() -> void:
	for mode: String in ["practice", "quest", "challenge", "minigame", "retry_wrong", "assignment"]:
		assert_bool(InstantAnswerPolicy.mode_allows(_attempt(mode))).override_failure_message(mode).is_true()
	# An assignment without its own release keeps the default (results released at once).
	assert_bool(InstantAnswerPolicy.mode_allows(_attempt("assignment", "immediate", ""))).is_true()


func test_exam_diagnostic_and_competition_never_send_on_tap() -> void:
	for mode: String in ["exam", "diagnostic", "competition"]:
		assert_bool(InstantAnswerPolicy.mode_allows(_attempt(mode))).override_failure_message(mode).is_false()


func test_feedback_later_or_results_later_never_send_on_tap() -> void:
	for timing: String in ["after_submit", "after_close", "never", ""]:
		assert_bool(InstantAnswerPolicy.mode_allows(_attempt("assignment", timing))).override_failure_message(timing) \
				.is_false()
	for release: String in ["after_close", "manual"]:
		assert_bool(InstantAnswerPolicy.mode_allows(_attempt("assignment", "immediate", release))) \
				.override_failure_message(release).is_false()


func test_a_finished_or_missing_attempt_never_sends_on_tap() -> void:
	var done: AttemptModel = _attempt()
	done.status = AttemptModel.STATUS_SUBMITTED
	assert_bool(InstantAnswerPolicy.mode_allows(done)).is_false()
	assert_bool(InstantAnswerPolicy.mode_allows(null)).is_false()


func test_only_one_step_items_send_on_tap() -> void:
	assert_bool(InstantAnswerPolicy.item_allows(LearningFixtures.question("single_choice"))).is_true()
	assert_bool(InstantAnswerPolicy.item_allows(LearningFixtures.question("true_false"))).is_true()
	for type: String in ["multiple_choice", "fill_in_blank", "short_answer", "numeric", "matching", "ordering",
			"drag_drop", "image_based", "interactive"]:
		assert_bool(InstantAnswerPolicy.item_allows(LearningFixtures.question(type))).override_failure_message(type) \
				.is_false()
	assert_bool(InstantAnswerPolicy.item_allows(null)).is_false()


func test_true_false_with_several_statements_keeps_the_check_step() -> void:
	var item: Dictionary = LearningFixtures.item_of("true_false").duplicate(true)
	(item["render_payload"] as Dictionary)["statements"] = [
		{"id": "s1", "content": {"blocks": [{"type": "paragraph", "text": "6 × 7 = 42"}]}},
		{"id": "s2", "content": {"blocks": [{"type": "paragraph", "text": "6 × 8 = 46"}]}},
	]
	assert_bool(InstantAnswerPolicy.item_allows(QuestionRenderModel.from_dict(item))).is_false()


func test_applies_needs_both_the_mode_and_the_item() -> void:
	assert_bool(InstantAnswerPolicy.applies(_attempt(), LearningFixtures.question("single_choice"))).is_true()
	assert_bool(InstantAnswerPolicy.applies(_attempt("exam"), LearningFixtures.question("single_choice"))).is_false()
	assert_bool(InstantAnswerPolicy.applies(_attempt(), LearningFixtures.question("ordering"))).is_false()


func test_double_tap_is_off_by_default_and_follows_the_client_setting() -> void:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, _saved_bands)
	for band: String in UiTokens.DENSITY_BANDS:
		assert_bool(InstantAnswerPolicy.double_tap(band)).override_failure_message(band).is_false()
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray(["g1_3"]))
	assert_bool(InstantAnswerPolicy.double_tap("g1_3")).is_true()
	assert_bool(InstantAnswerPolicy.double_tap("g6_7")).is_false()


func test_the_project_ships_the_setting_empty() -> void:
	var text: String = FileAccess.get_file_as_string("res://project.godot")
	assert_str(text).contains("learning/tap_confirm_bands=PackedStringArray()")
