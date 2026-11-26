extends GdUnitTestSuite
## F063-AC-14 (TC-LRN-012; T022): after grading, the feedback strip shows ✓/✗ with text (never colour alone) in the
## Theme feedback panel, the feedback of the chosen option and the explanation; when FeedbackTiming hides the verdict
## (`is_correct`, `feedback`, `explanation` null) it says only that the answer is saved — no right/wrong.

const STRIP_SCENE: String = "res://scenes/learning/FeedbackStrip.tscn"

var strip: VBoxContainer = null


func before_test() -> void:
	strip = auto_free((load(STRIP_SCENE) as PackedScene).instantiate()) as VBoxContainer
	add_child(strip)


func _show(data: Dictionary) -> void:
	await strip.call("show_result", ResponseResultModel.from_dict(data), null)


func _node(name: String) -> Control:
	return strip.get_node("%" + name) as Control


func test_wrong_answer_shows_cross_text_option_feedback_and_explanation() -> void:
	await _show(LearningFixtures.response_result())
	assert_bool(strip.visible).is_true()
	var panel: PanelContainer = _node("VerdictPanel") as PanelContainer
	assert_bool(panel.visible).is_true()
	assert_str(String(panel.theme_type_variation)).is_equal("FeedbackIncorrectPanel")
	assert_object((_node("Icon") as TextureRect).texture).is_equal(preload("res://resources/ui/icons/cross.svg"))
	assert_str(String(strip.call("verdict_text"))).is_equal(tr("ui.feedback.incorrect"))
	assert_str(String(strip.call("verdict_icon"))).is_equal("incorrect")
	var feedback: RichTextLabel = _node("FeedbackText") as RichTextLabel
	assert_bool(feedback.visible).is_true()
	assert_str(feedback.get_parsed_text()).contains("Em đã cộng 3 + 6")
	var explanation: RichTextLabel = _node("Explanation") as RichTextLabel
	assert_str(explanation.get_parsed_text()).contains("6 × 3 = 18")
	assert_bool(_node("Saved").visible).is_false()


func test_right_answer_shows_check_and_text() -> void:
	await _show(LearningFixtures.response_result_correct())
	assert_str(String((_node("VerdictPanel") as PanelContainer).theme_type_variation)).is_equal("FeedbackCorrectPanel")
	assert_str(String(strip.call("verdict_text"))).is_equal(tr("ui.feedback.correct"))
	assert_str(String(strip.call("verdict_icon"))).is_equal("correct")
	assert_bool(_node("FeedbackText").visible).is_false()


func test_hidden_grading_never_shows_right_or_wrong() -> void:
	await _show(LearningFixtures.response_result_hidden())
	assert_bool(strip.visible).is_true()
	assert_bool(_node("VerdictPanel").visible).is_false()
	assert_str(String(strip.call("verdict_text"))).is_empty()
	assert_bool(_node("FeedbackText").visible).is_false()
	assert_bool(_node("Explanation").visible).is_false()
	assert_bool(_node("Saved").visible).is_true()
	assert_bool(_node("WorkedExample").visible).is_false()


func test_feedback_panels_come_from_the_generated_theme() -> void:
	var theme: Theme = load(UiTokens.THEME_PATHS["g4_5"]) as Theme
	for variation: String in ["FeedbackCorrectPanel", "FeedbackIncorrectPanel", "FeedbackLabel"]:
		assert_bool(theme.get_type_variation_base(variation) != &"").override_failure_message(variation).is_true()
	var correct: StyleBoxFlat = theme.get_stylebox("panel", "FeedbackCorrectPanel") as StyleBoxFlat
	assert_object(correct).is_not_null()
	assert_bool(correct.bg_color.is_equal_approx(UiTokens.COLOR_FEEDBACK_CORRECT)).is_true()
	assert_bool(theme.get_color("font_color", "FeedbackLabel").is_equal_approx(UiTokens.COLOR_TEXT_ON_ACCENT)).is_true()


func test_clear_hides_the_strip() -> void:
	await _show(LearningFixtures.response_result())
	strip.call("clear")
	assert_bool(strip.visible).is_false()
