extends GdUnitTestSuite
## F098 (student UX pass 3, STUDENT_GAME_UX_LEARNING §4.1b): tap-to-answer in the activity player. A tap (or key 1–6)
## on a single-choice / one-statement true-false tile sends the answer at once when the attempt shows feedback right
## away (AC-01) and only then (AC-02); the tiles lock with "Đang chấm…" and nothing is sent twice (AC-03); the graded
## tile shows the verdict with an icon and words and the right answer only when the server sent it (AC-04); Mây and
## the main button follow (AC-05); a refused answer can be sent again (AC-06); the instruction says "tap", "Kiểm tra"
## is hidden, keys act as taps and focus is visible (AC-07); bands may confirm with a second tap (AC-08).
## FakeHttp + contract fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var host: Control = null
var _band: String = ""
var _saved_bands: Variant = null


func before_test() -> void:
	_band = AppState.grade_band
	_saved_bands = ProjectSettings.get_setting(InstantAnswerPolicy.SETTING, PackedStringArray())
	AppState.start_ui(get_tree().root)
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	if host != null and is_instance_valid(host):
		host.free()
	host = null
	QualityManager.suspend_reasons.clear()
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, _saved_bands)
	AppState.set_grade_band(_band)
	h.dispose()


func _attempt(types: PackedStringArray = ["single_choice", "true_false"], mode: String = "practice",
		timing: String = "immediate", release: String = "immediate") -> Dictionary:
	var a: Dictionary = LearningFixtures.attempt_with_types(types)
	a["mode"] = mode
	a["feedback_timing"] = timing
	a["result_release"] = release
	return a


func _open(attempt: Dictionary, band: String = "g1_3") -> void:
	AppState.set_grade_band(band)
	host = (load(HOST_SCENE) as PackedScene).instantiate() as Control
	host.theme = AppState.theme_service.current_theme
	host.set("services", services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	add_child(host)
	h.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": String(attempt["mode"]),
		"context_type": "free_play"})
	for i: int in 3:
		await await_idle_frame()


func _renderer() -> QuizActivityRenderer:
	return host.get("renderer") as QuizActivityRenderer


func _question() -> QuestionRendererBase:
	return _renderer().question


func _tiles() -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in _question().find_children("*", "Button", true, false):
		var b: Button = node as Button
		if String(b.theme_type_variation).begins_with("AnswerTile"):
			out.append(b)
	return out


func _state() -> int:
	return int(host.get("state"))


func _primary() -> Button:
	return host.find_child("Primary", true, false) as Button


func _tag(b: Button) -> Label:
	return b.get_node_or_null(QuestionRendererBase.TAG_NAME) as Label


func _settle(max_frames: int = 120) -> void:
	for i: int in max_frames:
		await await_idle_frame()
		if not _state() in [1, 3, 5]:
			return


## A real tap: the tile's `pressed` (what a click / touch / ui_accept on the focused tile emits).
func _tap(b: Button) -> void:
	b.button_pressed = not b.button_pressed if b.toggle_mode else b.button_pressed
	b.pressed.emit()


func _key(code: Key) -> void:
	var key: InputEventKey = InputEventKey.new()
	key.keycode = code
	key.pressed = true
	host.call("_unhandled_input", key)


# ---------- AC-01: a tap sends at once where feedback is immediate ----------

func test_tap_sends_the_answer_at_once_in_practice() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(_tiles()[1])
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
	var body: Dictionary = h.http.last(LearningFixtures.ROUTE_RESPOND).body
	assert_dict(body["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_str(String(body["kind"])).is_equal("question")
	# The client never sends a score or a verdict (R5).
	assert_bool(body.has("is_correct") or body.has("score")).is_false()
	assert_int(_state()).is_equal(4)


func test_every_feedback_mode_sends_on_tap() -> void:
	for mode: String in ["quest", "challenge", "minigame", "retry_wrong", "assignment"]:
		await _open(_attempt(PackedStringArray(["single_choice"]), mode))
		h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
		var before: int = h.http.count(LearningFixtures.ROUTE_RESPOND)
		_tap(_tiles()[0])
		await _settle()
		assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).override_failure_message(mode).is_equal(before + 1)
		host.free()


func test_number_key_acts_as_a_tap() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_key(KEY_3)
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
	assert_dict(h.http.last(LearningFixtures.ROUTE_RESPOND).body["response"] as Dictionary).is_equal({"option_id": "o_c"})


func test_one_statement_true_false_sends_on_tap() -> void:
	await _open(_attempt(PackedStringArray(["true_false"])))
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(_tiles()[0])
	await _settle()
	assert_dict(h.http.last(LearningFixtures.ROUTE_RESPOND).body["response"] as Dictionary).is_equal({"value": true})


func test_a_resumed_item_with_a_saved_answer_is_not_sent_by_itself() -> void:
	var a: Dictionary = _attempt(PackedStringArray(["single_choice"]))
	a["saved_responses"] = [{"position": 1, "response": {"option_id": "o_b"}, "hints_revealed": 0}]
	await _open(a)
	await _settle(10)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)
	assert_int(_state()).is_equal(2)


# ---------- AC-02: no send on tap elsewhere ----------

func test_exam_diagnostic_and_competition_only_select() -> void:
	for mode: String in ["exam", "diagnostic", "competition"]:
		await _open(_attempt(PackedStringArray(["single_choice"]), mode, "after_submit"))
		_tap(_tiles()[1])
		await _settle(10)
		assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).override_failure_message(mode).is_equal(0)
		assert_bool(_primary().visible).override_failure_message(mode).is_true()
		assert_bool(_primary().disabled).override_failure_message(mode).is_false()
		assert_str(_primary().text).is_equal("ui.activity_player.check")
		host.free()


func test_competition_with_immediate_timing_still_only_selects() -> void:
	await _open(_attempt(PackedStringArray(["single_choice"]), "competition", "immediate"))
	_tap(_tiles()[1])
	await _settle(10)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)


func test_feedback_after_submit_or_results_later_only_select() -> void:
	for pair: Array in [["after_submit", "immediate"], ["immediate", "after_close"], ["never", "manual"]]:
		await _open(_attempt(PackedStringArray(["single_choice"]), "assignment", String(pair[0]), String(pair[1])))
		_tap(_tiles()[0])
		await _settle(10)
		assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).override_failure_message(str(pair)).is_equal(0)
		assert_str((_question().get_node("%Prompt") as Label).text).is_equal("ui.question.select_answer")
		host.free()


func test_multi_step_types_keep_the_check_button() -> void:
	await _open(_attempt(PackedStringArray(["multiple_choice", "ordering", "matching"])))
	_key(KEY_1)
	await _settle(10)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)
	assert_bool(_primary().visible).is_true()
	assert_str(_primary().text).is_equal("ui.activity_player.check")


# ---------- AC-03: lock while grading, no double send, same key on retry ----------

func test_tiles_lock_and_show_grading_until_the_result() -> void:
	await _open(_attempt())
	h.http.delay_frames = 20
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	var tiles: Array[Button] = _tiles()
	_tap(tiles[1])
	await await_idle_frame()
	assert_int(_state()).is_equal(3)
	var tag: Label = _tag(tiles[1])
	assert_object(tag).is_not_null()
	assert_str(tag.text).is_equal("ui.question.grading")
	assert_str(String(tag.theme_type_variation)).is_equal("TileTagPending")
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.PENDING)
	for b: Button in tiles:
		assert_int(b.mouse_filter).is_equal(Control.MOUSE_FILTER_IGNORE)
		assert_int(b.focus_mode).is_equal(Control.FOCUS_NONE)
	# More taps and keys while grading change nothing and send nothing.
	_tap(tiles[2])
	_tap(tiles[1])
	_key(KEY_4)
	h.http.delay_frames = 1
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
	assert_dict(_question().current_response()).is_equal({"option_id": "o_b"})
	assert_object(_tag(tiles[2])).is_null()


func test_a_lost_answer_is_queued_and_resent_with_the_same_key() -> void:
	await _open(_attempt())
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	var tiles: Array[Button] = _tiles()
	_tap(tiles[1])
	await _settle()
	assert_bool((host.find_child("Offline", true, false) as Control).visible).is_true()
	# Still grading: locked, tag kept, nothing else sent.
	assert_str(_tag(tiles[1]).text).is_equal("ui.question.grading")
	_tap(tiles[0])
	var first_key: String = String(h.http.last(LearningFixtures.ROUTE_RESPOND).headers["idempotency-key"])
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	await host.call("_flush")
	await _settle()
	# ApiClient's own retries and the queue re-send all carry the key of the first send, with the same answer.
	var recs: Array[FakeHttp.Recorded] = h.http.find(LearningFixtures.ROUTE_RESPOND)
	assert_int(recs.size()).is_greater_equal(2)
	for rec: FakeHttp.Recorded in recs:
		assert_str(String(rec.headers["idempotency-key"])).is_equal(first_key)
		assert_dict(rec.body["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.CORRECT)


# ---------- AC-04, AC-05: verdict on the tile, right answer only from the server, Mây ----------

func test_right_answer_turns_the_tile_green_with_a_tick_and_words() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	var tiles: Array[Button] = _tiles()
	_tap(tiles[1])
	await _settle()
	assert_str(String(tiles[1].theme_type_variation)).is_equal("AnswerTileCorrect")
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.CORRECT)
	assert_str(_tag(tiles[1]).text).is_equal("ui.question.tile_correct")
	assert_str(String(_tag(tiles[1]).theme_type_variation)).is_equal("TileTagCorrect")
	for i: int in [0, 2, 3]:
		assert_object(_tag(tiles[i])).is_null()
	var coach: CloudyMascot = host.find_child("Coach", true, false) as CloudyMascot
	assert_int(coach.mood).is_equal(CloudyMascot.Mood.HAPPY)
	assert_str(String((host.find_child("FeedbackStrip", true, false) as Control).call("verdict_icon"))).is_equal("correct")
	assert_str(_primary().text).is_equal("ui.activity_player.next")
	assert_bool(_primary().visible).is_true()


func test_wrong_answer_is_warm_with_a_cross_and_kind_words_and_marks_the_server_answer() -> void:
	await _open(_attempt())
	var result: Dictionary = LearningFixtures.response_result()
	result["correct_response"] = {"option_id": "o_b"}
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, result)
	var tiles: Array[Button] = _tiles()
	_tap(tiles[0])
	await _settle()
	assert_str(String(tiles[0].theme_type_variation)).is_equal("AnswerTileTry")
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.TRY)
	assert_str(_tag(tiles[0]).text).is_equal("ui.question.tile_try")
	assert_str(String(_tag(tiles[0]).theme_type_variation)).is_equal("TileTagTry")
	assert_str(String(tiles[1].theme_type_variation)).is_equal("AnswerTileCorrect")
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.ANSWER)
	assert_str(_tag(tiles[1]).text).is_equal("ui.question.tile_answer")
	var coach: CloudyMascot = host.find_child("Coach", true, false) as CloudyMascot
	assert_int(coach.mood).is_equal(CloudyMascot.Mood.ENCOURAGE)
	# Kind words, no penalty words, in both languages.
	for locale: String in ["vi-VN", "en-US"]:
		var t: Translation = TranslationServer.get_translation_object(locale)
		var words: String = String(t.get_message("ui.question.tile_try")).to_lower()
		assert_str(words).is_not_empty()
		for bad: String in ["sai rồi", "trừ", "phạt", "wrong", "penalty", "lose"]:
			assert_str(words).override_failure_message("%s %s" % [locale, bad]).not_contains(bad)


func test_without_correct_response_no_other_tile_is_marked() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	var tiles: Array[Button] = _tiles()
	_tap(tiles[0])
	await _settle()
	assert_int(QuestionRendererBase.tile_verdict(tiles[0])).is_equal(AnswerBadge.Verdict.TRY)
	# F099: the other tiles rest in the neutral look (no tag, no right-answer mark).
	for i: int in [1, 2, 3]:
		assert_int(QuestionRendererBase.tile_verdict(tiles[i])).is_equal(AnswerBadge.Verdict.REST)
		assert_object(_tag(tiles[i])).is_null()
		assert_str(String(tiles[i].theme_type_variation)).is_equal("AnswerTileRest")


func test_hidden_verdict_marks_no_tile() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_hidden())
	var tiles: Array[Button] = _tiles()
	_tap(tiles[0])
	await _settle()
	for b: Button in tiles:
		assert_int(QuestionRendererBase.tile_verdict(b)).is_equal(AnswerBadge.Verdict.NONE)
		assert_object(_tag(b)).is_null()


func test_graded_tiles_never_rely_on_colour_alone() -> void:
	await _open(_attempt())
	var theme: Theme = AppState.theme_service.current_theme
	for variant: StringName in [&"AnswerTileCorrect", &"AnswerTileTry"]:
		assert_object(theme.get_stylebox(&"normal", variant)).is_not_null()
		assert_object(theme.get_stylebox(&"focus", variant)).is_not_null()
	for variant: StringName in [&"TileTagCorrect", &"TileTagTry", &"TileTagPending"]:
		assert_object(theme.get_stylebox(&"normal", variant)).is_not_null()
	# Every verdict key has words in vi and en (the tag), next to a badge shape.
	for key: String in [QuestionRendererBase.GRADING_KEY, QuestionRendererBase.TILE_CORRECT_KEY,
			QuestionRendererBase.TILE_TRY_KEY, QuestionRendererBase.TILE_ANSWER_KEY]:
		for locale: String in ["vi-VN", "en-US"]:
			assert_str(String(TranslationServer.get_translation_object(locale).get_message(key))).is_not_empty()


# ---------- AC-06: refused answer, send again ----------

func test_refused_answer_unlocks_and_offers_send_again() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 422, LearningFixtures.problem(422, "INVALID_RESPONSE"))
	var tiles: Array[Button] = _tiles()
	_tap(tiles[1])
	await _settle()
	assert_int(_state()).is_equal(2)
	assert_bool((host.get_node("%Status") as Label).visible).is_true()
	assert_dict(_question().current_response()).is_equal({"option_id": "o_b"})
	assert_object(_tag(tiles[1])).is_null()
	assert_int(tiles[1].mouse_filter).is_equal(Control.MOUSE_FILTER_STOP)
	assert_bool(_primary().visible).is_true()
	assert_bool(_primary().disabled).is_false()
	assert_str(_primary().text).is_equal("ui.activity_player.resend")
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	await host.call("press_primary")
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(2)
	assert_int(QuestionRendererBase.tile_verdict(tiles[1])).is_equal(AnswerBadge.Verdict.CORRECT)


func test_tapping_the_chosen_tile_again_resends_after_a_refusal() -> void:
	await _open(_attempt())
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 422, LearningFixtures.problem(422, "INVALID_RESPONSE"))
	var tiles: Array[Button] = _tiles()
	_tap(tiles[1])
	await _settle()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(tiles[1])
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(2)
	assert_int(_state()).is_equal(4)


# ---------- AC-07: instruction, no "Kiểm tra", focus ----------

func test_instruction_says_tap_and_the_check_button_is_hidden() -> void:
	await _open(_attempt())
	assert_str((_question().get_node("%Prompt") as Label).text).is_equal("ui.question.tap_answer")
	assert_bool(_primary().visible).is_false()
	for locale: String in ["vi-VN", "en-US"]:
		var t: Translation = TranslationServer.get_translation_object(locale)
		assert_str(String(t.get_message("ui.question.tap_answer"))).is_not_empty()
	# Every tile takes keyboard focus and draws the Theme's focus ring.
	for b: Button in _tiles():
		assert_int(b.focus_mode).is_equal(Control.FOCUS_ALL)
		assert_object(b.get_theme_stylebox(&"focus")).is_not_null()
	# The true/false item says it too.
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(_tiles()[0])
	await _settle()
	await host.call("press_primary")
	await await_idle_frame()
	assert_int(_renderer().current_position()).is_equal(2)
	assert_str((_question().get_node("%Prompt") as Label).text).is_equal("ui.question.tap_answer")


# ---------- AC-08: optional second tap per band ----------

func test_two_tap_band_waits_for_a_second_tap_on_the_same_tile() -> void:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray(["g1_3"]))
	await _open(_attempt())
	assert_str((_question().get_node("%Prompt") as Label).text).is_equal("ui.question.tap_answer_twice")
	var tiles: Array[Button] = _tiles()
	_tap(tiles[0])
	await _settle(5)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)
	var line: Label = host.find_child("CoachLine", true, false) as Label
	assert_str(line.text).is_equal("ui.activity_player.tap_again")
	assert_bool(line.visible).is_true()
	# Another tile only moves the choice.
	_tap(tiles[2])
	await _settle(5)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)
	assert_dict(_question().current_response()).is_equal({"option_id": "o_c"})
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(tiles[2])
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
	assert_dict(h.http.last(LearningFixtures.ROUTE_RESPOND).body["response"] as Dictionary).is_equal({"option_id": "o_c"})


func test_two_tap_band_confirms_with_the_same_key_or_the_main_button() -> void:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray(["g4_5"]))
	await _open(_attempt(), "g4_5")
	_key(KEY_2)
	await _settle(5)
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(0)
	assert_bool(_primary().visible).is_true()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_key(KEY_2)
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
	# Next item (true/false): the main button confirms too.
	await host.call("press_primary")
	await await_idle_frame()
	_key(KEY_1)
	await _settle(5)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	await host.call("press_primary")
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(2)


func test_other_bands_stay_one_tap_when_one_band_confirms() -> void:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray(["g1_3"]))
	await _open(_attempt(), "g6_7")
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_tap(_tiles()[0])
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)
