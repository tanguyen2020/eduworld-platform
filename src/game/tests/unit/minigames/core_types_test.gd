extends GdUnitTestSuite
## F073-AC-22 (TC-LRN-016; T033): each of the 6 core mini-game scenes (multiple_choice, true_false, fill_in_blank,
## drag_and_drop, matching, ordering) draws its fixture item, emits only `response_prepared(item_id, response)` with
## exactly the `QuestionResponse` keys of the item's question type (no score, no verdict, no answer key), never touches
## the network, never decides right/wrong, plays the right/wrong effect only after `show_feedback` (or at the end when
## `feedback_display` defers it) and leaves no orphan node after `teardown`. Scenes run alone with the F073 fixtures.

## Allowed top-level keys of each `QuestionResponse` shape (specs/openapi, additionalProperties: false).
const SHAPES: Dictionary = {
	"single_choice": ["option_id"], "multiple_choice": ["option_ids"], "true_false": ["value"],
	"fill_in_blank": ["blanks"], "short_answer": ["text"], "numeric": ["value"], "matching": ["pairs"],
	"ordering": ["order"], "drag_drop": ["placements"], "image_based": ["points"],
	"interactive": ["state", "action_log"],
}
const FORBIDDEN_WORDS: PackedStringArray = ["score", "is_correct", "correct", "answer_key", "points", "game_points", "streak"]
## Scene scripts: no network, no service, no grading (AGENTS R1, R5; ADR-0016).
const SCENE_SCRIPTS: PackedStringArray = [
	"res://scripts/learning/minigame_base.gd",
	"res://scenes/minigames/types/minigame_question_scene.gd",
	"res://scenes/minigames/types/puzzle/puzzle_minigame.gd",
]
const NETWORK_TOKENS: PackedStringArray = [
	"ApiClient", "RestApiClient", "ApiRequest", "LearningService", "HTTPRequest", "HTTPClient", "get_services",
	"ServiceLocator", "WebSocket", "StreamPeer",
]

var _prepared: Array = []


func _ctx(band: String = "g4_5") -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	ctx.grade_band = band
	return ctx


func _scene(type: String, session: MiniGameSessionModel = null) -> MiniGameQuestionScene:
	var s: MiniGameSessionModel = session if session != null else MiniGameFixtures.model(type)
	var scene: MiniGameQuestionScene = auto_free(MiniGameRegistry.instantiate(type)) as MiniGameQuestionScene
	add_child(scene)
	_prepared.clear()
	scene.response_prepared.connect(func(item_id: String, response: Dictionary) -> void: _prepared.append([item_id, response]))
	scene.setup(s.config, s, _ctx())
	return scene


## Answers item `i` like a student does and returns the emitted response (asserting its shape).
func _answer(scene: MiniGameQuestionScene, i: int = 0) -> Dictionary:
	var item: QuestionRenderModel = scene.session.items()[i]
	scene.present_item(item)
	MiniGameFixtures.answer(scene)
	if _prepared.is_empty():
		assert_bool(scene.is_answer_complete()).override_failure_message(item.type + " not complete").is_true()
		(scene.find_child("Done", true, false) as Button).pressed.emit()
	assert_array(_prepared).override_failure_message(item.type).has_size(1)
	assert_str(String(_prepared[0][0])).is_equal(str(item.position))
	var response: Dictionary = _prepared[0][1] as Dictionary
	for key: Variant in response.keys():
		assert_bool(String(key) in (SHAPES[item.type] as Array)).override_failure_message("%s: %s" % [item.type, key]).is_true()
	var text: String = JSON.stringify(response)
	for word: String in FORBIDDEN_WORDS:
		assert_str(text).override_failure_message(item.type).not_contains('"%s"' % word)
	return response


func _result(position: int, correct: bool) -> ResponseResultModel:
	return ResponseResultModel.from_dict(MiniGameFixtures.result(position, correct))


func test_fixtures_carry_no_answer_key() -> void:
	for type: String in MiniGameFixtures.CORE_TYPES + MiniGameFixtures.EXTENDED_TYPES:
		assert_array(Array(MiniGameFixtures.forbidden_paths(MiniGameFixtures.session(type)))).override_failure_message(type).is_empty()


func test_multiple_choice_emits_the_choice_shapes() -> void:
	var scene: MiniGameQuestionScene = _scene("multiple_choice")
	assert_dict(_answer(scene, 0)).is_equal({"option_id": "o_b"})
	_prepared.clear()
	assert_dict(_answer(scene, 1)).is_equal({"option_ids": ["o_a"]})


func test_true_false_emits_value() -> void:
	assert_dict(_answer(_scene("true_false"))).is_equal({"value": false})


func test_fill_in_blank_emits_blanks_and_text() -> void:
	var scene: MiniGameQuestionScene = _scene("fill_in_blank")
	assert_dict(_answer(scene, 0)).is_equal({"blanks": {"b_1": "4"}})
	_prepared.clear()
	assert_dict(_answer(scene, 1)).is_equal({"text": "phép nhân"})


func test_drag_and_drop_emits_placements() -> void:
	assert_dict(_answer(_scene("drag_and_drop"))).is_equal({"placements": {"d_1": "z_1", "d_2": "z_2", "d_3": "z_1"}})


func test_matching_emits_pairs() -> void:
	assert_dict(_answer(_scene("matching"))).is_equal({"pairs": [["l_1", "r_2"], ["l_2", "r_1"], ["l_3", "r_3"]]})


func test_ordering_emits_order() -> void:
	assert_dict(_answer(_scene("ordering"))).is_equal({"order": ["i_2", "i_1", "i_3", "i_4"]})


func test_effects_play_only_after_show_feedback() -> void:
	for type: String in MiniGameFixtures.CORE_TYPES:
		var scene: MiniGameQuestionScene = _scene(type)
		_answer(scene)
		var effect: PanelContainer = scene.find_child("Effect", true, false) as PanelContainer
		assert_int(scene.effects_played).override_failure_message(type).is_equal(0)
		assert_int(scene.sounds_played).override_failure_message(type).is_equal(0)
		assert_bool(effect.visible).override_failure_message(type).is_false()
		assert_bool(scene.locked).override_failure_message(type).is_true()
		scene.show_feedback(_result(1, true))
		assert_int(scene.effects_played).override_failure_message(type).is_equal(1)
		assert_object(scene.last_icon).is_equal(MiniGameBase.ICON_CORRECT)
		assert_bool(effect.visible).is_true()
		assert_str((scene.find_child("EffectPoints", true, false) as Label).text).is_equal("+10")


func test_hidden_verdict_never_shows_right_or_wrong() -> void:
	var scene: MiniGameQuestionScene = _scene("matching")
	_answer(scene)
	scene.show_feedback(ResponseResultModel.from_dict(MiniGameFixtures.result_hidden(1)))
	assert_int(scene.effects_played).is_equal(0)
	assert_str(scene.last_effect).is_equal(MiniGameFeedbackSfx.KIND_SAVED)
	assert_str((scene.find_child("EffectText", true, false) as Label).text).is_equal("ui.feedback.saved")


func test_end_of_game_display_defers_effects_to_finish() -> void:
	var data: Dictionary = MiniGameFixtures.session("fill_in_blank")
	(data["config"] as Dictionary)["feedback_display"] = "end_of_game"
	var scene: MiniGameQuestionScene = _scene("fill_in_blank", MiniGameSessionModel.from_dict(data))
	_answer(scene, 0)
	scene.show_feedback(_result(1, true))
	_prepared.clear()
	_answer(scene, 1)
	scene.show_feedback(_result(2, false))
	assert_int(scene.effects_played).is_equal(0)
	assert_int(scene.sounds_played).is_equal(0)
	var summary: MiniGameSummaryModel = MiniGameSummaryModel.from_attempt(
			AttemptModel.from_dict(MiniGameFixtures.submitted("fill_in_blank", 10, true)))
	scene.finish(summary)
	assert_int(scene.effects_played).is_equal(2)
	assert_int(scene.sounds_played).is_equal(1)
	var marks: Label = scene.find_child("SummaryMarks", true, false) as Label
	assert_str(marks.text).contains("1 " + tr("ui.minigame.mark_correct")).contains("2 " + tr("ui.minigame.mark_incorrect"))
	assert_bool((scene.find_child("Summary", true, false) as Control).visible).is_true()


func test_scene_scripts_never_call_the_network_nor_grade() -> void:
	var grading: RegEx = RegEx.create_from_string("(is_correct|game_points|score|streak)\\s*(=|\\+=)[^=]")
	for path: String in SCENE_SCRIPTS:
		var code: PackedStringArray = []
		for line: String in FileAccess.get_file_as_string(path).split("\n"):
			if not line.strip_edges().begins_with("#"):
				code.append(line)
		var text: String = "\n".join(code)
		for token: String in NETWORK_TOKENS:
			assert_str(text).override_failure_message("%s uses %s" % [path, token]).not_contains(token)
		assert_object(grading.search(text)).override_failure_message(path + " assigns a verdict or points").is_null()


func test_scene_alone_has_no_network_node_nor_service() -> void:
	var scene: MiniGameQuestionScene = _scene("ordering")
	_answer(scene)
	scene.show_feedback(_result(1, true))
	scene.finish(MiniGameSummaryModel.new())
	assert_array(scene.find_children("*", "HTTPRequest", true, false)).is_empty()
	for property: Dictionary in scene.get_property_list():
		var type_name: String = String(property.get("class_name", ""))
		assert_bool(type_name in ["LearningService", "RestApiClient", "ServiceLocator"]) \
				.override_failure_message(String(property["name"])).is_false()


func test_teardown_leaves_no_orphan_nodes() -> void:
	for type: String in MiniGameFixtures.CORE_TYPES:
		await await_idle_frame()
		var before: int = int(Performance.get_monitor(Performance.OBJECT_ORPHAN_NODE_COUNT))
		var s: MiniGameSessionModel = MiniGameFixtures.model(type)
		var scene: MiniGameQuestionScene = MiniGameRegistry.instantiate(type) as MiniGameQuestionScene
		add_child(scene)
		scene.setup(s.config, s, _ctx())
		scene.present_item(s.items()[0])
		MiniGameFixtures.answer(scene)
		scene.show_feedback(_result(1, true))
		scene.teardown()
		remove_child(scene)
		scene.free()
		await await_idle_frame()
		await await_idle_frame()
		var after: int = int(Performance.get_monitor(Performance.OBJECT_ORPHAN_NODE_COUNT))
		assert_int(after).override_failure_message("%s leaks %d node(s)" % [type, after - before]).is_less_equal(before)
