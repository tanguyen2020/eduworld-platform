extends GdUnitTestSuite
## F063-AC-03, SC-002 (TC-LRN-007; T009): the activity-player fixtures generated from the OpenAPI examples parse into
## the client models, cover the 11 question types, and carry no answer-side key (`answer_key`, `correct_option_id(s)`,
## `feedback`, `hints`, `explanation`…) before grading. Should a payload ever contain one, the model drops it.

const FIXTURES: String = "res://tests/fixtures/contract/openapi"


func _json(name: String) -> Dictionary:
	var parser: JSON = JSON.new()
	assert_int(parser.parse(FileAccess.get_file_as_string("%s/%s.json" % [FIXTURES, name]))).is_equal(OK)
	return parser.data as Dictionary


func test_attempt_parses_with_the_eleven_question_types() -> void:
	var data: Dictionary = _json("Attempt")
	var attempt: AttemptModel = AttemptModel.from_dict(data)
	assert_object(attempt).is_not_null()
	assert_int(attempt.items.size()).is_equal(11)
	var types: Array = []
	for q: QuestionRenderModel in attempt.items:
		types.append(q.type)
		assert_bool(q.is_known_type()).is_true()
		assert_bool(Uuid.is_valid(q.question_version_id)).is_true()
		assert_bool(q.stem().has("blocks")).override_failure_message(q.type).is_true()
	assert_array(types).contains_exactly_in_any_order(Array(QuestionRenderModel.TYPES))
	assert_object(attempt.render).is_not_null()
	assert_str(attempt.render.title).is_not_empty()
	assert_int(attempt.render.math_assets.size()).is_equal(1)
	assert_str(attempt.render.math_assets[0].sha256).is_equal(LearningFixtures.MATH_SHA)
	assert_dict(attempt.saved_response(1)).is_equal({"option_id": "o_b"})
	assert_int(attempt.hints_revealed(1)).is_equal(1)
	assert_int(attempt.items[0].hint_count).is_equal(2)


func test_render_payloads_have_no_answer_side_key_before_grading() -> void:
	var data: Dictionary = _json("Attempt")
	assert_array(Array(QuestionRenderModel.find_answer_keys(data["items"]))).is_empty()
	assert_array(Array(QuestionRenderModel.find_answer_keys((data["render"] as Dictionary)["render_payload"]))).is_empty()


func test_an_answer_key_sent_by_mistake_is_dropped() -> void:
	var item: Dictionary = LearningFixtures.item_of("single_choice").duplicate(true)
	var payload: Dictionary = item["render_payload"] as Dictionary
	payload["answer_key"] = {"correct_option_id": "o_b"}
	payload["correct_option_id"] = "o_b"
	((payload["options"] as Array)[0] as Dictionary)["feedback"] = "leak"
	((payload["options"] as Array)[0] as Dictionary)["misconception_tag"] = "add_instead_of_multiply"
	payload["hints"] = [{"text": "leak"}]
	var q: QuestionRenderModel = QuestionRenderModel.from_dict(item)
	assert_array(Array(QuestionRenderModel.find_answer_keys(q.render_payload))).is_empty()
	assert_str(JSON.stringify(q.render_payload)).not_contains("leak")
	assert_int((q.render_payload["options"] as Array).size()).is_equal(4)


func test_math_blocks_carry_a_key_and_speech_text_only() -> void:
	var q: QuestionRenderModel = AttemptModel.from_dict(_json("Attempt")).item_at(6)
	var formulas: Array[MathAssetRefModel] = RichText.math_of(q.stem())
	assert_int(formulas.size()).is_equal(1)
	assert_str(formulas[0].math_ref).is_equal("math/%s.svg" % LearningFixtures.MATH_SHA)
	assert_str(formulas[0].sha256).is_equal(LearningFixtures.MATH_SHA)
	assert_str(formulas[0].speech_text).is_equal("ba phần tư")
	assert_bool(formulas[0].is_block()).is_true()
	assert_str(JSON.stringify(q.stem())).not_contains("latex")


func test_response_result_hint_and_review_parse() -> void:
	var result: ResponseResultModel = ResponseResultModel.from_dict(_json("ActivityResponseResult"))
	assert_bool(result.has_verdict).is_true()
	assert_bool(result.is_correct).is_false()
	assert_str(result.feedback.option_id).is_equal("o_a")
	assert_bool(Uuid.is_valid(result.feedback.worked_example_ref)).is_true()
	var hint: HintResultModel = HintResultModel.from_dict(_json("HintResult"))
	assert_int(hint.level).is_equal(1)
	assert_int(hint.total_levels).is_equal(2)
	assert_int(hint.math_assets.size()).is_equal(1)
	var review: AttemptReviewModel = AttemptReviewModel.from_dict(_json("AttemptReview"))
	assert_int(review.items.size()).is_equal(2)
	assert_int(review.correct_count()).is_equal(1)
	assert_int(review.weak_objectives.size()).is_equal(1)
	assert_bool(review.recommendations[0].is_practice()).is_true()
	assert_bool(Uuid.is_valid(review.recommendations[0].recommendation_id)).is_true()


func test_problem_codes_with_a_message_exist_in_the_contract() -> void:
	var spec_path: String = ProjectSettings.globalize_path("res://").path_join("../../specs/openapi/eduworld-api.v1.yaml").simplify_path()
	var spec: String = FileAccess.get_file_as_string(spec_path)
	for code: String in ["ATTEMPT_EXPIRED", "ATTEMPT_NOT_IN_PROGRESS", "ASSIGNMENT_MAX_ATTEMPTS", "INVALID_RESPONSE",
			"HINTS_NOT_ALLOWED", "HINTS_EXHAUSTED", "ACTIVITY_TYPE_NOT_SUPPORTED", "ASSIGNMENT_NOT_OPEN"]:
		assert_bool(("ui.error." + code) in L10n.ERROR_KEYS).override_failure_message(code).is_true()
		assert_str(spec).contains(code)
