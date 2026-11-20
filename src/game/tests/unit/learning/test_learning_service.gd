extends GdUnitTestSuite
## F063-AC-01, F063-AC-05, F063-AC-08, F063-AC-24, T037 (TC-LRN-002, TC-LRN-008; T007): LearningService starts an
## attempt with an `Idempotency-Key` and the intent body, routes a question-set assignment and a recommended practice
## to `startAdHocAttempt`, sends each answer with a new key (same key on retry), and submits with a key where
## `409 ATTEMPT_ALREADY_SUBMITTED` counts as success. Runs on FakeHttp/FakeClock (AuthHarness) with contract fixtures.

var h: AuthHarness = null
var service: LearningService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = LearningService.new(h.api, TelemetryService.new(h.api, h.clock), h.clock)


func after_test() -> void:
	h.dispose()


func _start_practice() -> AttemptModel:
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	return await service.start({
		"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play",
	})


func test_start_sends_the_intent_body_with_a_v7_key_and_tenant() -> void:
	var loaded: Array[AttemptModel] = []
	service.attempt_loaded.connect(func(a: AttemptModel) -> void: loaded.append(a))
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	var attempt: AttemptModel = await service.start({
		"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "assignment", "context_type": "assignment",
		"context_id": LearningFixtures.ASSIGNMENT_ID, "unexpected": "dropped",
	})
	assert_object(attempt).is_not_null()
	assert_int(attempt.items.size()).is_equal(11)
	assert_array(loaded).has_size(1)
	var rec: FakeHttp.Recorded = h.http.last(LearningFixtures.ROUTE_START)
	assert_str(rec.path).is_equal("/learning/activities/%s/attempts" % LearningFixtures.ACTIVITY_ID)
	assert_bool(Uuid.is_v7(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
	assert_dict(rec.body).is_equal({
		"mode": "assignment", "context_type": "assignment", "context_id": LearningFixtures.ASSIGNMENT_ID,
	})


func test_question_set_assignment_starts_an_ad_hoc_attempt() -> void:
	h.http.reply(LearningFixtures.ROUTE_ADHOC, 201, LearningFixtures.fresh_attempt())
	var attempt: AttemptModel = await service.start({
		"question_set_id": LearningFixtures.QUESTION_SET_ID, "mode": "assignment", "context_type": "assignment",
		"context_id": LearningFixtures.ASSIGNMENT_ID,
	})
	assert_object(attempt).is_not_null()
	var rec: FakeHttp.Recorded = h.http.last(LearningFixtures.ROUTE_ADHOC)
	assert_bool(Uuid.is_v7(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_dict(rec.body).is_equal({
		"mode": "assignment", "context_type": "assignment", "context_id": LearningFixtures.ASSIGNMENT_ID,
		"source": {"kind": "question_set", "question_set_id": LearningFixtures.QUESTION_SET_ID},
	})


func test_recommended_practice_sends_the_recommendation_id() -> void:
	var review: AttemptReviewModel = AttemptReviewModel.from_dict(LearningFixtures.review())
	var rec_model: AttemptReviewModel.Recommendation = review.recommendations[0]
	h.http.reply(LearningFixtures.ROUTE_ADHOC, 201, LearningFixtures.fresh_attempt())
	await service.start(LearningService.practice_args(rec_model))
	var body: Dictionary = h.http.last(LearningFixtures.ROUTE_ADHOC).body
	assert_str(String(body["recommendation_id"])).is_equal(rec_model.recommendation_id)
	assert_str(String((body["source"] as Dictionary)["question_set_id"])).is_equal(rec_model.ref_id)
	assert_str(String(body["mode"])).is_equal("practice")


func test_retrying_a_start_after_a_network_error_reuses_the_key() -> void:
	h.http.fail(LearningFixtures.ROUTE_START)
	var args: Dictionary = {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice"}
	assert_object(await service.start(args)).is_null()
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	assert_object(await service.start(args)).is_not_null()
	var keys: Array[String] = []
	for rec: FakeHttp.Recorded in h.http.find(LearningFixtures.ROUTE_START):
		if not String(rec.headers.get("idempotency-key", "")) in keys:
			keys.append(String(rec.headers.get("idempotency-key", "")))
	assert_array(keys).has_size(1)


func test_each_answer_gets_a_new_key_and_sends_no_score() -> void:
	await _start_practice()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	var first: ResponseResultModel = await service.respond(1, {"option_id": "o_a"}, 4200)
	var second: ResponseResultModel = await service.respond(2, {"option_ids": ["o_a", "o_c"]}, 3000)
	assert_object(first).is_not_null()
	assert_object(second).is_not_null()
	var recs: Array[FakeHttp.Recorded] = h.http.find(LearningFixtures.ROUTE_RESPOND)
	assert_array(recs).has_size(2)
	assert_str(String(recs[0].headers["idempotency-key"])).is_not_equal(String(recs[1].headers["idempotency-key"]))
	assert_bool(Uuid.is_v7(String(recs[0].headers["idempotency-key"]))).is_true()
	var body: Dictionary = recs[0].body
	assert_str(String(body["kind"])).is_equal("question")
	assert_int(int(body["position"])).is_equal(1)
	assert_dict(body["response"] as Dictionary).is_equal({"option_id": "o_a"})
	assert_int(int(body["client_elapsed_ms"])).is_equal(4200)
	assert_str(String(body["question_version_id"])).is_equal("0f630000-0000-4000-8000-000000000101")
	for forbidden: String in ["score", "is_correct", "correct", "points", "hints_used"]:
		assert_bool(body.has(forbidden)).override_failure_message(forbidden).is_false()
	assert_bool(recs[0].raw_body.contains("score")).is_false()


func test_retrying_the_same_answer_keeps_its_key_after_a_server_error() -> void:
	await _start_practice()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 503, LearningFixtures.problem(503, "UNAVAILABLE"))
	assert_object(await service.respond(1, {"option_id": "o_b"}, 100)).is_null()
	assert_bool(service.offline).is_true()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	var results: Array[ResponseResultModel] = await service.flush_pending()
	assert_array(results).has_size(1)
	var keys: Dictionary = {}
	for rec: FakeHttp.Recorded in h.http.find(LearningFixtures.ROUTE_RESPOND):
		keys[String(rec.headers["idempotency-key"])] = true
	assert_int(keys.size()).is_equal(1)
	assert_bool(service.offline).is_false()


func test_submit_sends_a_key_and_returns_the_submitted_attempt() -> void:
	await _start_practice()
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	var done: AttemptModel = await service.submit()
	assert_object(done).is_not_null()
	assert_str(done.status).is_equal("graded")
	assert_bool(Uuid.is_v7(String(h.http.last(LearningFixtures.ROUTE_SUBMIT).headers["idempotency-key"]))).is_true()


func test_already_submitted_counts_as_success_and_loads_the_attempt() -> void:
	await _start_practice()
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 409, LearningFixtures.problem(409, "ATTEMPT_ALREADY_SUBMITTED"))
	h.http.reply(LearningFixtures.ROUTE_GET, 200, LearningFixtures.submitted())
	var done: AttemptModel = await service.submit()
	assert_object(done).is_not_null()
	assert_str(done.status).is_equal("graded")
	assert_int(h.http.count(LearningFixtures.ROUTE_GET)).is_equal(1)
	assert_str(service.last_error_code).is_empty()


func test_submit_retry_after_network_loss_reuses_the_key() -> void:
	await _start_practice()
	h.http.fail(LearningFixtures.ROUTE_SUBMIT)
	assert_object(await service.submit()).is_null()
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	assert_object(await service.submit()).is_not_null()
	var keys: Dictionary = {}
	for rec: FakeHttp.Recorded in h.http.find(LearningFixtures.ROUTE_SUBMIT):
		keys[String(rec.headers["idempotency-key"])] = true
	assert_int(keys.size()).is_equal(1)


func test_answers_are_never_logged() -> void:
	await _start_practice()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await service.respond(5, {"text": "phep-nhan-unique-7731"}, 10)
	assert_str(JSON.stringify(Log.entries())).not_contains("phep-nhan-unique-7731")
