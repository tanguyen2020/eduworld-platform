extends GdUnitTestSuite
## F063-AC-07 (TC-LRN-009; T012): leaving mid-attempt calls `pauseActivityAttempt` with a key; reopening rebuilds from
## `getActivityAttempt` at the first unanswered item with the saved answers and revealed hints, on the version pinned
## by the attempt (the renderer only reads `attempt.items`).

var h: AuthHarness = null
var service: LearningService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = LearningService.new(h.api, null, h.clock)


func after_test() -> void:
	h.dispose()


func test_pause_sends_a_keyed_request() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})
	var paused: Dictionary = LearningFixtures.attempt()
	paused["paused_at"] = "2026-11-20T02:10:00Z"
	h.http.reply(LearningFixtures.ROUTE_PAUSE, 200, paused)
	assert_bool(await service.pause()).is_true()
	var rec: FakeHttp.Recorded = h.http.last(LearningFixtures.ROUTE_PAUSE)
	assert_str(rec.path).is_equal("/learning/activity-attempts/%s:pause" % LearningFixtures.ATTEMPT_ID)
	assert_bool(Uuid.is_v7(String(rec.headers["idempotency-key"]))).is_true()
	assert_str(service.attempt.paused_at).is_not_empty()


func test_resume_rebuilds_position_answers_and_hints() -> void:
	var data: Dictionary = LearningFixtures.attempt()
	data["saved_responses"] = [
		{"position": 1, "response": {"option_id": "o_b"}, "hints_revealed": 2},
		{"position": 2, "response": {"option_ids": ["o_a", "o_c"]}, "hints_revealed": 0},
	]
	h.http.reply(LearningFixtures.ROUTE_GET, 200, data)
	var attempt: AttemptModel = await service.resume(LearningFixtures.ATTEMPT_ID)
	assert_object(attempt).is_not_null()
	assert_int(attempt.resume_index()).is_equal(2)
	assert_int(attempt.items[attempt.resume_index()].position).is_equal(3)
	assert_dict(attempt.saved_response(2)).is_equal({"option_ids": ["o_a", "o_c"]})
	assert_int(attempt.hints_revealed(1)).is_equal(2)


func test_renderer_resumes_at_the_saved_item_with_its_answer() -> void:
	var data: Dictionary = LearningFixtures.attempt()
	data["saved_responses"] = [{"position": 1, "response": {"option_id": "o_b"}}, {"position": 2, "response": {"option_ids": ["o_c"]}}]
	var attempt: AttemptModel = AttemptModel.from_dict(data)
	var renderer: QuizActivityRenderer = auto_free(ActivityRendererRegistry.instantiate("quiz")) as QuizActivityRenderer
	add_child(renderer)
	var ctx: RendererContext = RendererContext.new()
	renderer.setup(attempt, ctx)
	assert_int(renderer.current_position()).is_equal(3)
	renderer.show_index(1)
	assert_dict(renderer.question.current_response()).is_equal({"option_ids": ["o_c"]})


func test_the_pinned_version_is_kept_from_the_attempt() -> void:
	h.http.reply(LearningFixtures.ROUTE_GET, 200, LearningFixtures.attempt())
	var attempt: AttemptModel = await service.resume(LearningFixtures.ATTEMPT_ID)
	assert_str(attempt.activity_version_id).is_equal("0f630000-0000-4000-8000-0000000000a2")
	assert_str(attempt.items[0].question_version_id).is_equal("0f630000-0000-4000-8000-000000000101")
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await service.respond(1, {"option_id": "o_a"}, 0)
	assert_str(String(h.http.last(LearningFixtures.ROUTE_RESPOND).body["question_version_id"])) \
			.is_equal("0f630000-0000-4000-8000-000000000101")


func test_a_finished_attempt_is_recognised() -> void:
	h.http.reply(LearningFixtures.ROUTE_GET, 200, LearningFixtures.submitted("submitted", "manual"))
	var attempt: AttemptModel = await service.resume(LearningFixtures.ATTEMPT_ID)
	assert_bool(attempt.is_finished()).is_true()
	assert_bool(attempt.results_immediate()).is_false()
