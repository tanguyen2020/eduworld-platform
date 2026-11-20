extends GdUnitTestSuite
## F063-AC-12, F063-AC-13 (TC-TEN-024, TC-I18N-003, TC-I18N-004; T015): text inputs are limited by the render payload
## and the contract (fill in blank `max_length` ≤ 200, short answer `max_length` ≤ 500, numeric ≤ 64) and never send a
## longer string; `translation_missing` shows the original text with `ui.learning.no_translation`; the question keeps
## the attempt's language when the UI locale changes.

var _locale: String = ""


func before_test() -> void:
	_locale = TranslationServer.get_locale()


func after_test() -> void:
	TranslationServer.set_locale(_locale)


func _scene(type: String, payload_override: Dictionary = {}) -> QuestionRendererBase:
	var item: Dictionary = LearningFixtures.item_of(type).duplicate(true)
	(item["render_payload"] as Dictionary).merge(payload_override, true)
	var q: QuestionRendererBase = auto_free(QuestionRendererRegistry.instantiate(type)) as QuestionRendererBase
	add_child(q)
	q.setup(QuestionRenderModel.from_dict(item), RendererContext.new())
	return q


func test_fill_in_blank_uses_the_blank_max_length() -> void:
	var q: QuestionRendererBase = _scene("fill_in_blank")
	var edit: LineEdit = (q.get("inputs") as Dictionary)["b_1"] as LineEdit
	assert_int(edit.max_length).is_equal(3)
	edit.text_changed.emit("123456")
	assert_dict(q.current_response()).is_equal({"blanks": {"b_1": "123"}})


func test_fill_in_blank_is_capped_by_the_contract() -> void:
	var model: FillInBlankAnswer = FillInBlankAnswer.new()
	model.setup({"blanks": [{"id": "b_1", "input": "text", "max_length": 5000}]})
	assert_int(model.max_length("b_1")).is_equal(FillInBlankAnswer.CONTRACT_MAX_LENGTH)
	model.set_blank("b_1", "x".repeat(300))
	assert_int(String((model.to_response()["blanks"] as Dictionary)["b_1"]).length()).is_equal(200)


func test_short_answer_uses_the_payload_limit() -> void:
	var q: QuestionRendererBase = _scene("short_answer")
	var edit: LineEdit = q.get("edit") as LineEdit
	assert_int(edit.max_length).is_equal(40)
	edit.text_changed.emit("a".repeat(60))
	assert_int(String(q.current_response()["text"]).length()).is_equal(40)
	assert_str((q.get("counter") as Label).text).is_equal("40/40")


func test_short_answer_default_and_contract_cap() -> void:
	var model: ShortAnswerAnswer = ShortAnswerAnswer.new()
	model.setup({})
	assert_int(model.limit).is_equal(ShortAnswerAnswer.DEFAULT_MAX_LENGTH)
	model.setup({"max_length": 9999})
	assert_int(model.limit).is_equal(ShortAnswerAnswer.CONTRACT_MAX_LENGTH)


func test_numeric_keeps_only_number_characters_and_64_max() -> void:
	var q: QuestionRendererBase = _scene("numeric")
	var edit: LineEdit = q.get("edit") as LineEdit
	assert_int(edit.max_length).is_equal(64)
	edit.text_changed.emit("-1a2,5b/3")
	assert_dict(q.current_response()).is_equal({"value": "-12,53"})
	var model: NumericAnswer = q.answer as NumericAnswer
	model.set_text("9".repeat(100))
	assert_int(String(q.current_response()["value"]).length()).is_equal(64)


func test_numeric_fraction_only_when_allowed() -> void:
	var model: NumericAnswer = NumericAnswer.new()
	model.setup({"allow_fraction_input": true, "decimal_separator": "."})
	model.set_text("3/4")
	assert_dict(model.to_response()).is_equal({"value": "3/4"})
	assert_bool("/" in model.pad_keys()).is_true()
	assert_bool("." in model.pad_keys()).is_true()


func test_translation_missing_shows_the_label() -> void:
	var h: AuthHarness = AuthHarness.create()
	await h.sign_in()
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	var host: Control = auto_free((load("res://scenes/learning/ActivityHost.tscn") as PackedScene).instantiate()) as Control
	host.set("services", services)
	host.set("client_version", "1.2.3")
	add_child(host)
	var data: Dictionary = LearningFixtures.fresh_attempt()
	(data["render"] as Dictionary)["translation_missing"] = true
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	var label: Label = host.find_child("NoTranslation", true, false) as Label
	assert_bool(label.visible).is_true()
	assert_str(label.text).is_equal("ui.learning.no_translation")
	assert_str(tr(label.text)).is_not_equal("ui.learning.no_translation")
	host.call("close")
	QualityManager.suspend_reasons.clear()
	h.dispose()


func test_question_keeps_the_attempt_language_when_the_locale_changes() -> void:
	var rich: Dictionary = {"blocks": [{"type": "paragraph", "text": {"vi": "Câu hỏi tiếng Việt", "en": "English question"}}]}
	var attempt_data: Dictionary = LearningFixtures.fresh_attempt()
	attempt_data["display_language"] = "vi-VN"
	var attempt: AttemptModel = AttemptModel.from_dict(attempt_data)
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = attempt.content_language()
	TranslationServer.set_locale("en")
	var view: RichContentView = auto_free((load("res://scenes/learning/RichContentView.tscn") as PackedScene).instantiate()) as RichContentView
	add_child(view)
	view.show_content(rich, ctx)
	assert_str((view.get_child(0) as Label).text).is_equal("Câu hỏi tiếng Việt")
	assert_str(RichText.plain_text(rich, ctx)).is_equal("Câu hỏi tiếng Việt")
