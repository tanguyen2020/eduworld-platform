extends GdUnitTestSuite
## F097 (student UX pass 2, STUDENT_GAME_UX_LEARNING §3–§4, DESIGN_SYSTEM §5.1–§5.2): the activity player looks like a
## game and stays readable for every grade band — answer tiles (F097-AC-01, AC-02), the question card with "Đọc to" and
## no stem repeated by the voice caption (AC-03, AC-04), the question trail (AC-05), verdicts with an icon and words,
## kind wording for a wrong answer, never colour alone (AC-06, AC-07), the coach Mây (AC-08) and the consistent
## exit / hint / main buttons with the band touch target (AC-10). FakeHttp + contract fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"
const STEM: String = "Bạn Nam có 3 túi, mỗi túi 6 quả cam. Nam có tất cả bao nhiêu quả cam?"
const TILE_TYPES: PackedStringArray = ["single_choice", "multiple_choice", "true_false", "matching", "ordering",
	"drag_drop"]

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


func _open(band: String, attempt: Dictionary = LearningFixtures.fresh_attempt()) -> void:
	AppState.set_grade_band(band)
	if host != null and is_instance_valid(host):
		host.free()
	host = (load(HOST_SCENE) as PackedScene).instantiate() as Control
	# The band Theme as Boot applies it to the window (the test runner may sit in its own viewport).
	host.theme = AppState.theme_service.current_theme
	host.set("services", services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	add_child(host)
	h.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 4:
		await await_idle_frame()


func _renderer() -> QuizActivityRenderer:
	return host.get("renderer") as QuizActivityRenderer


func _tiles(root: Node) -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in root.find_children("*", "Button", true, false):
		if String((node as Button).theme_type_variation).begins_with("AnswerTile"):
			out.append(node as Button)
	return out


func _visible_texts(root: Node, text: String) -> int:
	var n: int = 0
	for node: Node in root.find_children("*", "Label", true, false):
		var label: Label = node as Label
		if label.is_visible_in_tree() and label.text == text:
			n += 1
	return n


# ---------- AC-01, AC-02: answer tiles per grade band ----------

func test_choice_answers_are_big_coloured_tiles_with_number_badges() -> void:
	await _open("g1_3")
	var tiles: Array[Button] = _tiles(_renderer())
	assert_int(tiles.size()).is_equal(4)
	var kinds: Dictionary = {}
	for i: int in tiles.size():
		var badge: AnswerBadge = tiles[i].get_node("Badge") as AnswerBadge
		assert_int(badge.number).is_equal(i + 1)
		kinds[String(tiles[i].theme_type_variation)] = true
	# Four different colour sets, no "1." text prefix.
	assert_int(kinds.size()).is_equal(4)
	assert_str(tiles[0].text).is_equal("9 quả")


func test_tiles_meet_the_band_touch_target_and_stem_font_size() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		await _open(band)
		var minimum: float = float(UiTokens.DENSITY_ANSWER_MIN_HEIGHT[band])
		for tile: Button in _tiles(_renderer()):
			assert_float(tile.get_combined_minimum_size().y).override_failure_message("%s tile height" % band) \
					.is_greater_equal(minimum)
			assert_int(tile.get_theme_font_size(&"font_size")).override_failure_message("%s tile font" % band) \
					.is_equal(int(UiTokens.DENSITY_FONT_STEM[band]))
		# g1_3 ≥ 64 dp (NFR-A11Y-001); answer tiles are taller than buttons in every band.
		assert_float(minimum).is_greater_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band]))
		var stem: Label = _renderer().find_child("QuestionCard", true, false).find_children("*", "Label", true, false)[0]
		assert_int(stem.get_theme_font_size(&"font_size")).is_equal(int(UiTokens.DENSITY_FONT_STEM[band]))


func test_every_tile_question_type_uses_answer_tiles() -> void:
	for type: String in TILE_TYPES:
		var q: QuestionRendererBase = auto_free(QuestionRendererRegistry.instantiate(type)) as QuestionRendererBase
		add_child(q)
		q.setup(LearningFixtures.question(type), RendererContext.new())
		assert_int(_tiles(q).size()).override_failure_message(type).is_greater(0)


func test_selected_tile_shows_a_tick_not_only_a_colour() -> void:
	await _open("g1_3")
	_renderer().select_index(1)
	await await_idle_frame()
	var tiles: Array[Button] = _tiles(_renderer())
	assert_bool((tiles[1].get_node("Badge") as AnswerBadge).is_selected()).is_true()
	assert_bool((tiles[0].get_node("Badge") as AnswerBadge).is_selected()).is_false()


# ---------- AC-03, AC-04: question card, read aloud, no duplicated stem ----------

func test_stem_is_shown_once_in_the_question_card() -> void:
	await _open("g1_3")
	assert_int(_visible_texts(host, STEM)).is_equal(1)
	var voice: Node = host.find_child("VoiceOver", true, false)
	assert_bool(bool(voice.get("stem_on_screen"))).is_true()
	assert_bool(bool(voice.call("is_caption_visible"))).is_false()
	var card: Node = _renderer().find_child("QuestionCard", true, false)
	assert_object(card).is_not_null()
	assert_int(_visible_texts(card, STEM)).is_equal(1)


func test_stem_is_shown_once_in_every_band_and_when_muted() -> void:
	var bus: int = AudioServer.get_bus_index(&"Voice")
	AudioServer.set_bus_mute(bus, true)
	for band: String in ["g1_3", "g6_7"]:
		await _open(band)
		assert_int(_visible_texts(host, STEM)).override_failure_message(band).is_equal(1)
	AudioServer.set_bus_mute(bus, false)


func test_read_aloud_button_on_the_card_replays_the_voice_over() -> void:
	await _open("g1_3")
	var button: Button = _renderer().find_child("ReadAloud", true, false) as Button
	assert_object(button).is_not_null()
	assert_object(button.icon).is_not_null()
	var voice: Node = host.find_child("VoiceOver", true, false)
	var wav: AudioStreamWAV = AudioStreamWAV.new()
	var data: PackedByteArray = PackedByteArray()
	data.resize(8000)
	wav.data = data
	voice.call("present", STEM, wav, false)
	button.pressed.emit()
	assert_bool(bool(voice.call("is_playing"))).is_true()
	# Still no caption repeating the stem.
	assert_int(_visible_texts(host, STEM)).is_equal(1)


# ---------- AC-05: question trail ----------

func test_header_shows_a_question_trail_and_the_count() -> void:
	await _open("g1_3")
	var trail: ProgressTrail = host.find_child("Trail", true, false) as ProgressTrail
	assert_bool(trail.visible).is_true()
	assert_int(trail.total).is_equal(11)
	assert_str(trail.mark_of(1)).is_equal("current")
	assert_str(trail.mark_of(2)).is_equal("todo")
	trail.set_progress(3, 11)
	assert_str(trail.mark_of(1)).is_equal("done")
	assert_bool(trail.uses_bar()).is_false()
	trail.set_progress(5, 20)
	assert_bool(trail.uses_bar()).is_true()
	assert_str((host.find_child("Progress", true, false) as Label).text).is_equal("1/11")


# ---------- AC-06, AC-07, AC-08: verdict, kind wording, coach ----------

func test_wrong_answer_is_kind_with_icon_and_words_and_the_coach_encourages() -> void:
	await _open("g1_3")
	host.call("_apply_result", ResponseResultModel.from_dict(LearningFixtures.response_result()))
	var strip: Node = host.find_child("FeedbackStrip", true, false)
	assert_str(String(strip.call("verdict_icon"))).is_equal("incorrect")
	assert_object((strip.get_node("%Icon") as TextureRect).texture).is_not_null()
	var words: String = String(strip.call("verdict_text"))
	assert_str(words).is_not_empty()
	assert_str(words).is_equal(tr("ui.feedback.incorrect"))
	var coach: CloudyMascot = host.find_child("Coach", true, false) as CloudyMascot
	assert_int(coach.mood).is_equal(CloudyMascot.Mood.ENCOURAGE)
	# The chosen answer stays visible after grading (the bubble does not hide the tiles' selection).
	assert_bool((host.find_child("CoachLine", true, false) as Label).visible).is_false()


func test_right_answer_has_icon_and_words_and_the_coach_is_happy() -> void:
	await _open("g1_3")
	host.call("_apply_result", ResponseResultModel.from_dict(LearningFixtures.response_result_correct()))
	var strip: Node = host.find_child("FeedbackStrip", true, false)
	assert_str(String(strip.call("verdict_icon"))).is_equal("correct")
	assert_str(String(strip.call("verdict_text"))).is_equal(tr("ui.feedback.correct"))
	assert_int((host.find_child("Coach", true, false) as CloudyMascot).mood).is_equal(CloudyMascot.Mood.HAPPY)


func test_wrong_answer_wording_has_no_penalty_words() -> void:
	for locale: String in ["vi", "en"]:
		var text: String = TranslationServer.get_translation_object(locale).get_message("ui.feedback.incorrect")
		assert_str(text).is_not_empty()
		for word: String in ["sai rồi", "trừ", "phạt", "wrong", "penalty", "lose"]:
			assert_str(text.to_lower()).override_failure_message("%s: %s" % [locale, word]).not_contains(word)


func test_coach_cheers_when_the_bubble_is_empty() -> void:
	await _open("g1_3")
	var line: Label = host.find_child("CoachLine", true, false) as Label
	assert_bool(line.is_visible_in_tree()).is_true()
	assert_str(line.text).is_equal("ui.activity_player.coach_cheer_hint")


# ---------- AC-10: consistent buttons, touch targets ----------

func test_exit_hint_and_main_buttons_have_icons_and_meet_the_touch_target() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		await _open(band)
		var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
		for name: String in ["Exit", "Primary", "HintButton", "ReadAloud"]:
			var b: Button = host.find_child(name, true, false) as Button
			assert_object(b).override_failure_message(name).is_not_null()
			assert_object(b.icon).override_failure_message("%s icon" % name).is_not_null()
			assert_float(b.get_combined_minimum_size().y).override_failure_message("%s %s" % [band, name]) \
					.is_greater_equal(minimum - 0.5)
		for node: Node in host.find_children("*", "BaseButton", true, false):
			var c: Control = node as Control
			if c.is_visible_in_tree():
				assert_float(c.get_combined_minimum_size().y).override_failure_message("%s %s" % [band, c.name]) \
						.is_greater_equal(minimum - 0.5)
