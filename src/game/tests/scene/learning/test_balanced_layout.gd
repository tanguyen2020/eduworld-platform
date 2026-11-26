extends GdUnitTestSuite
## F099-AC-05, F099-AC-06 (STUDENT_GAME_UX_LEARNING §3.3a): the activity board is balanced. With free room the answer
## tiles grow — never below `density.<band>.answer.min_height`, never above `grow_max` × it on a landscape board
## (`grow_max_tall` × on a portrait board, the tile block growing as much as a four-choice block would) — and the
## question is centred in the body; a portrait board shows the choices in one column with Mây's row right under the
## answers; a board without room keeps the tiles at the band's height. FakeHttp + contract fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var vp: SubViewport = null
var host: Control = null
var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	AppState.start_ui(get_tree().root)
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	if vp != null and is_instance_valid(vp):
		vp.free()
	vp = null
	host = null
	QualityManager.suspend_reasons.clear()
	AppState.set_grade_band(_band)
	h.dispose()


func _open(types: PackedStringArray, band: String, window: Vector2i) -> void:
	AppState.set_grade_band(band)
	vp = SubViewport.new()
	vp.size = window
	vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
	add_child(vp)
	host = (load(HOST_SCENE) as PackedScene).instantiate() as Control
	host.theme = AppState.theme_service.current_theme
	host.set("services", services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	vp.add_child(host)
	h.http.clear(LearningFixtures.ROUTE_START)
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt_with_types(types))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 10:
		await await_idle_frame()


func _question() -> QuestionRendererBase:
	return (host.get("renderer") as QuizActivityRenderer).question


func _tiles() -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in _question().find_children("*", "Button", true, false):
		if String((node as Button).theme_type_variation).begins_with("AnswerTile"):
			out.append(node as Button)
	return out


func _tile_min(band: String) -> float:
	return float(UiTokens.DENSITY_ANSWER_MIN_HEIGHT[band])


func test_landscape_tiles_grow_within_the_cap_and_the_question_is_centred() -> void:
	await _open(PackedStringArray(["single_choice"]), "g8_9", Vector2i(1280, 720))
	var min_h: float = _tile_min("g8_9")
	for b: Button in _tiles():
		assert_float(b.size.y).is_greater(min_h + 1.0)
		assert_float(b.size.y).is_less_equal(min_h * UiTokens.COMPONENT_ANSWER_TILE_GROW_MAX + 1.0)
		assert_int(b.get_theme_font_size(&"font_size")).is_equal(int(UiTokens.DENSITY_FONT_STEM["g8_9"]))
	assert_int((_question().get_node("%Answers") as GridContainer).columns).is_equal(2)
	# Centred: the room above the question card equals the room under the answers (± 2 px).
	var body: Rect2 = (host.get_node("%Body") as Control).get_global_rect()
	var q: Rect2 = _question().get_global_rect()
	assert_float(absf((q.position.y - body.position.y) - (body.end.y - q.end.y))).is_less_equal(2.0)
	assert_bool((host.get_node("%TopSpacer") as Control).visible).is_false()


func test_portrait_board_stacks_one_column_with_the_coach_under_the_answers() -> void:
	await _open(PackedStringArray(["single_choice"]), "g4_5", Vector2i(1280, 2276))
	var min_h: float = _tile_min("g4_5")
	assert_int((_question().get_node("%Answers") as GridContainer).columns).is_equal(1)
	for b: Button in _tiles():
		assert_float(b.size.y).is_greater(min_h * UiTokens.COMPONENT_ANSWER_TILE_GROW_MAX)
		assert_float(b.size.y).is_less_equal(min_h * UiTokens.COMPONENT_ANSWER_TILE_GROW_MAX_TALL + 1.0)
	assert_bool((host.get_node("%TopSpacer") as Control).visible).is_true()
	assert_bool((host.get_node("%BottomSpacer") as Control).visible).is_true()
	# Mây's row follows the answers: the gap between the last tile and the coach row is one row separation.
	var layout: VBoxContainer = host.get_node("Frame/Layout") as VBoxContainer
	var sep: float = float(layout.get_theme_constant(&"separation"))
	var last: Rect2 = _tiles()[-1].get_global_rect()
	var coach_row: Rect2 = (host.get_node("%Coach") as Control).get_parent().get_global_rect()
	assert_float(coach_row.position.y - last.end.y).is_less_equal(sep + 2.0)
	# The spacers share the rest evenly above and below.
	var top: float = (host.get_node("%TopSpacer") as Control).size.y
	var bottom: float = (host.get_node("%BottomSpacer") as Control).size.y
	assert_float(absf(top - bottom)).is_less_equal(2.0)


func test_true_false_stacks_on_a_portrait_board() -> void:
	await _open(PackedStringArray(["true_false"]), "g1_3", Vector2i(1280, 2276))
	var tiles: Array[Button] = _tiles()
	assert_int(tiles.size()).is_equal(2)
	assert_float(tiles[1].get_global_rect().position.y).is_greater(tiles[0].get_global_rect().end.y)
	assert_float(tiles[0].size.y).is_greater(_tile_min("g1_3") * UiTokens.COMPONENT_ANSWER_TILE_GROW_MAX)


func test_no_room_keeps_the_band_height() -> void:
	await _open(PackedStringArray(["single_choice"]), "g1_3", Vector2i(1280, 520))
	for b: Button in _tiles():
		assert_float(b.custom_minimum_size.y).is_equal(0.0)
		assert_float(b.size.y + 0.5).is_greater_equal(_tile_min("g1_3"))


func test_every_band_never_shrinks_a_tile() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		for window: Vector2i in [Vector2i(1280, 720), Vector2i(1280, 750), Vector2i(1280, 2276)]:
			await _open(PackedStringArray(["single_choice"]), band, window)
			for b: Button in _tiles():
				assert_float(b.size.y + 0.5).override_failure_message("%s %s" % [band, window]) \
						.is_greater_equal(_tile_min(band))
			vp.free()
			vp = null


func test_result_screen_unstacks_the_board() -> void:
	await _open(PackedStringArray(["single_choice"]), "g4_5", Vector2i(1280, 2276))
	assert_bool((host.get_node("%TopSpacer") as Control).visible).is_true()
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	await host.call("_show_results", false)
	for i: int in 6:
		await await_idle_frame()
	assert_bool((host.get_node("%TopSpacer") as Control).visible).is_false()
	assert_int((host.get_node("%Body") as Control).size_flags_vertical).is_equal(Control.SIZE_EXPAND_FILL)
