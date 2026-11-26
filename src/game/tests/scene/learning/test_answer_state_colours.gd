extends GdUnitTestSuite
## F099-AC-02, F099-AC-03, F099-AC-12 (STUDENT_GAME_UX_LEARNING §4.1b): once an answer is graded, only the chosen tile
## and — when the server sent `correct_response` — the right-answer tile carry the right / not-yet-right colours (with
## an icon badge and words); every other tile rests in the neutral `AnswerTileRest` look with its number kept, and no
## resting tile uses a state colour (background, border, badge). A hidden verdict changes no tile. Same for a question
## inside a video. FakeHttp + contract fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var host: Control = null
var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	AppState.start_ui(get_tree().root)
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	if host != null and is_instance_valid(host):
		host.free()
	host = null
	QualityManager.suspend_reasons.clear()
	AppState.set_grade_band(_band)
	h.dispose()


func _open(types: PackedStringArray, band: String = "g1_3") -> void:
	var a: Dictionary = LearningFixtures.attempt_with_types(types)
	a["mode"] = "practice"
	a["feedback_timing"] = "immediate"
	a["result_release"] = "immediate"
	AppState.set_grade_band(band)
	host = (load(HOST_SCENE) as PackedScene).instantiate() as Control
	host.theme = AppState.theme_service.current_theme
	host.set("services", services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	add_child(host)
	h.http.clear(LearningFixtures.ROUTE_START)
	h.http.reply(LearningFixtures.ROUTE_START, 201, a)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 3:
		await await_idle_frame()


func _tiles() -> Array[Button]:
	var q: QuestionRendererBase = (host.get("renderer") as QuizActivityRenderer).question
	return tiles_of(q)


static func tiles_of(q: Node) -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in q.find_children("*", "Button", true, false):
		if String((node as Button).theme_type_variation).begins_with("AnswerTile"):
			out.append(node as Button)
	return out


func _tap_and_grade(b: Button, result: Dictionary) -> void:
	h.http.clear(LearningFixtures.ROUTE_RESPOND)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, result)
	b.button_pressed = true
	b.pressed.emit()
	for i: int in 120:
		await await_idle_frame()
		if int(host.get("state")) == 4:
			break


static func state_colours() -> Array[Color]:
	return [UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BG, UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BORDER,
		UiTokens.COMPONENT_ANSWER_TILE_TRY_BG, UiTokens.COMPONENT_ANSWER_TILE_TRY_BORDER,
		UiTokens.COMPONENT_ANSWER_TILE_TAG_CORRECT_BG, UiTokens.COMPONENT_ANSWER_TILE_TAG_TRY_BG]


## Whether tile `b` draws any right / not-yet-right colour (any StyleBox state or its badge).
static func uses_state_colour(b: Button) -> bool:
	for style: StringName in [&"normal", &"hover", &"pressed", &"disabled", &"hover_pressed"]:
		var box: StyleBoxFlat = b.get_theme_stylebox(style) as StyleBoxFlat
		if box == null:
			continue
		for c: Color in state_colours():
			if box.bg_color.is_equal_approx(c) or box.border_color.is_equal_approx(c):
				return true
	var verdict: AnswerBadge.Verdict = QuestionRendererBase.tile_verdict(b)
	if verdict in [AnswerBadge.Verdict.CORRECT, AnswerBadge.Verdict.TRY, AnswerBadge.Verdict.ANSWER]:
		return true
	if verdict == AnswerBadge.Verdict.NONE:
		var badge: AnswerBadge = b.get_node_or_null("Badge") as AnswerBadge
		if badge != null:
			for c: Color in state_colours():
				if AnswerBadge.ACCENTS[badge.palette_index].is_equal_approx(c):
					return true
	return false


func _assert_rests(b: Button, number: int) -> void:
	assert_str(String(b.theme_type_variation)).is_equal("AnswerTileRest")
	assert_int(QuestionRendererBase.tile_verdict(b)).is_equal(AnswerBadge.Verdict.REST)
	assert_bool(uses_state_colour(b)).override_failure_message("%s uses a state colour" % b.text).is_false()
	assert_object(b.get_node_or_null(QuestionRendererBase.TAG_NAME)).is_null()
	# Readable and numbered: the label keeps the main text colour and the badge keeps its key number.
	assert_bool(b.get_theme_color(&"font_color").is_equal_approx(UiTokens.COMPONENT_ANSWER_TILE_REST_TEXT)).is_true()
	assert_int((b.get_node("Badge") as AnswerBadge).number).is_equal(number)


func test_not_yet_right_with_the_right_answer_rests_the_other_tiles() -> void:
	await _open(PackedStringArray(["single_choice"]))
	var r: Dictionary = LearningFixtures.response_result()
	r["correct_response"] = {"option_id": "o_b"}
	var tiles: Array[Button] = _tiles()
	await _tap_and_grade(tiles[0], r)
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.TRY)
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.ANSWER)
	_assert_rests(tiles[2], 3)
	_assert_rests(tiles[3], 4)
	# Exactly two tiles carry a state colour: the chosen one and the right answer.
	var coloured: int = 0
	for b: Button in tiles:
		coloured += 1 if uses_state_colour(b) else 0
	assert_int(coloured).is_equal(2)


func test_right_answer_rests_the_other_tiles() -> void:
	await _open(PackedStringArray(["single_choice"]))
	var tiles: Array[Button] = _tiles()
	await _tap_and_grade(tiles[1], LearningFixtures.response_result_correct())
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.CORRECT)
	for i: int in [0, 2, 3]:
		_assert_rests(tiles[i], i + 1)


func test_not_yet_right_without_the_right_answer_marks_no_green_tile() -> void:
	await _open(PackedStringArray(["single_choice"]))
	var tiles: Array[Button] = _tiles()
	await _tap_and_grade(tiles[0], LearningFixtures.response_result())
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.TRY)
	for i: int in [1, 2, 3]:
		_assert_rests(tiles[i], i + 1)


func test_hidden_verdict_changes_no_tile() -> void:
	await _open(PackedStringArray(["single_choice"]))
	var tiles: Array[Button] = _tiles()
	var types: Array[StringName] = []
	for b: Button in tiles:
		types.append(b.theme_type_variation)
	await _tap_and_grade(tiles[0], LearningFixtures.response_result_hidden())
	for i: int in tiles.size():
		assert_str(String(tiles[i].theme_type_variation)).is_equal(String(types[i]))
		assert_int(QuestionRendererBase.tile_verdict(tiles[i])).is_equal(AnswerBadge.Verdict.NONE)


func test_true_false_rests_the_other_tile() -> void:
	await _open(PackedStringArray(["true_false"]))
	var tiles: Array[Button] = _tiles()
	assert_int(tiles.size()).is_equal(2)
	await _tap_and_grade(tiles[0], LearningFixtures.response_result_correct())
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.CORRECT)
	_assert_rests(tiles[1], 2)


func test_candy_tiles_never_use_a_state_colour_before_grading() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		await _open(PackedStringArray(["single_choice"]), band)
		for b: Button in _tiles():
			assert_bool(uses_state_colour(b)).override_failure_message("%s %s" % [band, b.theme_type_variation]).is_false()
		host.free()
		host = null


## F099-AC-12: a question inside a video uses the same tiles, the same graded look and the board background.
func test_video_question_tiles_follow_the_same_rule() -> void:
	var v: VideoRendererHarness = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	v.renderer.theme = AppState.theme_service.current_theme
	v.play_for(31000)
	var q: QuestionRendererBase = v.renderer.question
	assert_object(q).is_not_null()
	q.select_index(0)
	var r: Dictionary = LearningFixtures.response_result()
	r["correct_response"] = {"option_id": "o_b"}
	v.renderer.show_result(ResponseResultModel.from_dict(r))
	var tiles: Array[Button] = tiles_of(q)
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.TRY)
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.ANSWER)
	_assert_rests(tiles[2], 3)
	assert_str(String((v.renderer.get_node("%Overlay") as PanelContainer).theme_type_variation)).is_equal("ActivityBoard")
	v.dispose()
