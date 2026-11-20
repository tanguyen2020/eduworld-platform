extends GdUnitTestSuite
## F063-AC-02, F063-AC-03 (TC-LRN-016, TC-LRN-007; T008): each of the 11 question scenes draws its contract fixture
## item, takes input the way a student gives it (click/tap; matching and drag & drop by tap-tap, the mobile path) and
## emits `response_prepared(position, response)` with exactly the `QuestionResponse` keys of its type — never a score,
## a verdict or an answer key.

## Allowed top-level keys of each `QuestionResponse` shape (specs/openapi, additionalProperties: false).
const SHAPES: Dictionary = {
	"single_choice": ["option_id"], "multiple_choice": ["option_ids"], "true_false": ["value"],
	"fill_in_blank": ["blanks"], "short_answer": ["text"], "numeric": ["value"], "matching": ["pairs"],
	"ordering": ["order"], "drag_drop": ["placements"], "image_based": ["points"],
	"interactive": ["state", "action_log"],
}
const FORBIDDEN: PackedStringArray = ["score", "is_correct", "correct", "answer_key", "points_awarded", "hints_used"]

var _prepared: Array = []


func _ctx() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	ctx.grade_band = "g4_5"
	return ctx


func _scene(type: String) -> QuestionRendererBase:
	var q: QuestionRendererBase = auto_free(QuestionRendererRegistry.instantiate(type)) as QuestionRendererBase
	add_child(q)
	q.setup(LearningFixtures.question(type), _ctx())
	_prepared.clear()
	q.response_prepared.connect(func(position: int, response: Dictionary) -> void: _prepared.append([position, response]))
	return q


func _commit(q: QuestionRendererBase, type: String) -> Dictionary:
	assert_bool(q.is_complete()).override_failure_message(type + " not complete").is_true()
	assert_bool(q.commit()).is_true()
	assert_array(_prepared).has_size(1)
	assert_int(int(_prepared[0][0])).is_equal(q.item.position)
	var response: Dictionary = _prepared[0][1] as Dictionary
	for key: Variant in response.keys():
		assert_bool(String(key) in (SHAPES[type] as Array)).override_failure_message("%s: %s" % [type, key]).is_true()
	var text: String = JSON.stringify(response)
	for word: String in FORBIDDEN:
		assert_str(text).override_failure_message(type).not_contains('"%s"' % word)
	return response


func test_every_question_type_has_a_scene() -> void:
	for type: String in QuestionRenderModel.TYPES:
		assert_bool(QuestionRendererRegistry.has(type)).override_failure_message(type).is_true()
		assert_bool(ResourceLoader.exists(QuestionRendererRegistry.scene_path(type))).is_true()


func test_incomplete_answer_is_not_sent() -> void:
	var q: QuestionRendererBase = _scene("single_choice")
	assert_bool(q.commit()).is_false()
	assert_array(_prepared).is_empty()


func test_single_choice() -> void:
	var q: QuestionRendererBase = _scene("single_choice")
	((q.get("buttons") as Dictionary)["o_b"] as Button).pressed.emit()
	assert_dict(_commit(q, "single_choice")).is_equal({"option_id": "o_b"})


func test_single_choice_keyboard_number_selects() -> void:
	var q: QuestionRendererBase = _scene("single_choice")
	q.select_index(2)
	assert_dict(_commit(q, "single_choice")).is_equal({"option_id": "o_c"})
	assert_str(((q.get("buttons") as Dictionary)["o_a"] as Button).text).starts_with("1. ")


func test_multiple_choice() -> void:
	var q: QuestionRendererBase = _scene("multiple_choice")
	var buttons: Dictionary = q.get("buttons") as Dictionary
	(buttons["o_c"] as Button).pressed.emit()
	(buttons["o_a"] as Button).pressed.emit()
	assert_dict(_commit(q, "multiple_choice")).is_equal({"option_ids": ["o_a", "o_c"]})


func test_true_false() -> void:
	var q: QuestionRendererBase = _scene("true_false")
	var pair: Array[Button] = (q.get("pairs") as Dictionary)[""] as Array[Button]
	assert_str(pair[0].text).is_equal("Đúng")
	pair[1].pressed.emit()
	assert_dict(_commit(q, "true_false")).is_equal({"value": false})


func test_fill_in_blank() -> void:
	var q: QuestionRendererBase = _scene("fill_in_blank")
	var edit: LineEdit = (q.get("inputs") as Dictionary)["b_1"] as LineEdit
	edit.text_changed.emit("4")
	assert_dict(_commit(q, "fill_in_blank")).is_equal({"blanks": {"b_1": "4"}})


func test_short_answer() -> void:
	var q: QuestionRendererBase = _scene("short_answer")
	(q.get("edit") as LineEdit).text_changed.emit("  phép nhân ")
	assert_dict(_commit(q, "short_answer")).is_equal({"text": "phép nhân"})


func test_numeric_uses_the_number_pad() -> void:
	var q: QuestionRendererBase = _scene("numeric")
	var pad: Array[Button] = q.get("pad_buttons") as Array[Button]
	for b: Button in pad:
		if b.text == "6":
			b.pressed.emit()
	assert_bool((q.get("edit") as LineEdit).virtual_keyboard_enabled).is_false()
	assert_dict(_commit(q, "numeric")).is_equal({"value": "6"})


func test_matching_tap_left_then_right() -> void:
	var q: QuestionRendererBase = _scene("matching")
	var left: Dictionary = q.get("left_buttons") as Dictionary
	var right: Dictionary = q.get("right_buttons") as Dictionary
	for pair: Array in [["l_1", "r_2"], ["l_2", "r_1"], ["l_3", "r_3"]]:
		(left[pair[0]] as Button).pressed.emit()
		(right[pair[1]] as Button).pressed.emit()
	assert_dict(_commit(q, "matching")).is_equal({"pairs": [["l_1", "r_2"], ["l_2", "r_1"], ["l_3", "r_3"]]})


func test_matching_drag_pairs_on_pc() -> void:
	var q: QuestionRendererBase = _scene("matching")
	var right: DropButton = (q.get("right_buttons") as Dictionary)["r_3"] as DropButton
	var left: DropButton = (q.get("left_buttons") as Dictionary)["l_1"] as DropButton
	assert_str(left.drag_source_id).is_equal("l_1")
	var payload: Dictionary = {DropButton.DRAG_KEY: left.drag_source_id}
	assert_bool(right._can_drop_data(Vector2.ZERO, payload)).is_true()
	right._drop_data(Vector2.ZERO, payload)
	assert_str((q.answer as MatchingAnswer).partner_of("l_1")).is_equal("r_3")


func test_ordering_buttons_move_cards() -> void:
	var q: QuestionRendererBase = _scene("ordering")
	assert_bool(q.is_complete()).is_false()
	var row: HBoxContainer = (q.get("rows") as Dictionary)["i_2"] as HBoxContainer
	(row.get_child(1) as Button).pressed.emit()
	assert_dict(_commit(q, "ordering")).is_equal({"order": ["i_2", "i_1", "i_3", "i_4"]})


func test_drag_drop_tap_card_then_zone() -> void:
	var q: QuestionRendererBase = _scene("drag_drop")
	var cards: Dictionary = q.get("cards") as Dictionary
	var zones: Dictionary = q.get("zones") as Dictionary
	for move: Array in [["d_1", "z_1"], ["d_2", "z_2"], ["d_3", "z_1"]]:
		(cards[move[0]] as Button).pressed.emit()
		(zones[move[1]] as Button).pressed.emit()
	assert_dict(_commit(q, "drag_drop")).is_equal({"placements": {"d_1": "z_1", "d_2": "z_2", "d_3": "z_1"}})


func test_image_hotspot_records_original_coordinates() -> void:
	var q: QuestionRendererBase = _scene("image_based")
	var image: TextureRect = q.get("image") as TextureRect
	await await_idle_frame()
	var click: InputEventMouseButton = InputEventMouseButton.new()
	click.button_index = MOUSE_BUTTON_LEFT
	click.pressed = true
	click.position = image.size * 0.25
	image.gui_input.emit(click)
	# image_size of the fixture is 800 × 450: a quarter of the shown image is (200, 112.5) in original coordinates.
	assert_dict(_commit(q, "image_based")).is_equal({"points": [{"x": 200.0, "y": 112.5}]})


func test_interactive_number_line() -> void:
	var q: QuestionRendererBase = _scene("interactive")
	(q.get("slider") as HSlider).value = 18.0
	var response: Dictionary = _commit(q, "interactive")
	assert_float(float((response["state"] as Dictionary)["value"])).is_equal(18.0)
	assert_str(String(((response["action_log"] as Array)[0] as Dictionary)["action"])).is_equal("set_value")


func test_locked_question_ignores_input() -> void:
	var q: QuestionRendererBase = _scene("single_choice")
	q.select_index(0)
	q.set_locked(true)
	assert_bool(((q.get("buttons") as Dictionary)["o_b"] as Button).disabled).is_true()
	assert_bool(q.commit()).is_false()


func test_saved_response_is_restored() -> void:
	var q: QuestionRendererBase = _scene("multiple_choice")
	q.restore({"option_ids": ["o_d", "o_b"]})
	assert_bool(((q.get("buttons") as Dictionary)["o_b"] as Button).button_pressed).is_true()
	assert_dict(q.current_response()).is_equal({"option_ids": ["o_b", "o_d"]})
