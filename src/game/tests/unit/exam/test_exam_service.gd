extends GdUnitTestSuite
## F064-AC-02, F064-AC-05, F064-AC-07, F064-AC-08, F064-AC-11 (TC-EXM-002, TC-EXM-003; T007): ExamService starts the
## attempt with an `Idempotency-Key` (kept on retry) and the exam device token, saves each answer with
## `PUT …/answers/{position}` and an `ActivityResponseSubmission` `kind = question` without any score, grading or
## client-computed time, submits with a key kept on retry, and takes `409 EXAM_ALREADY_SUBMITTED` as submitted (no
## more answers). FakeHttp/FakeClock (AuthHarness) with contract fixtures (TEST-ONLY).

var h: AuthHarness = null
var service: ExamService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = ExamService.new(h.api, h.clock)


func after_test() -> void:
	h.dispose()


func _start(saved: Array = []) -> AttemptModel:
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt(5, saved))
	return await service.start(ExamFixtures.SESSION_ID)


func test_load_session_reads_the_rules() -> void:
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session("sequential_no_return"))
	var session: ExamSessionModel = await service.load_session(ExamFixtures.SESSION_ID)
	assert_object(session).is_not_null()
	assert_str(h.http.last(ExamFixtures.ROUTE_SESSION).path).is_equal("/exams/sessions/" + ExamFixtures.SESSION_ID)
	assert_int(session.question_count).is_equal(15)
	assert_int(session.duration_for_student()).is_equal(900)
	assert_bool(session.forward_only()).is_true()
	assert_str(session.result_release).is_equal("after_close")


func test_list_sessions_follows_the_pages() -> void:
	var second: Dictionary = ExamFixtures.session()
	second["exam_session_id"] = ExamFixtures.OTHER_SESSION_ID
	h.http.reply(ExamFixtures.ROUTE_LIST, 200, {"data": [ExamFixtures.session()], "page": {"has_more": true, "next_cursor": "c2"}})
	h.http.reply(ExamFixtures.ROUTE_LIST, 200, ExamFixtures.list_body([second]))
	var sessions: Array[ExamSessionModel] = await service.list_sessions()
	assert_int(sessions.size()).is_equal(2)
	assert_str(h.http.last(ExamFixtures.ROUTE_LIST).query).contains("cursor=c2")


func test_start_sends_a_v7_key_the_tenant_and_the_device_token() -> void:
	var started: Array[AttemptModel] = []
	service.exam_started.connect(func(a: AttemptModel) -> void: started.append(a))
	var attempt: AttemptModel = await _start()
	assert_object(attempt).is_not_null()
	assert_int(attempt.items.size()).is_equal(5)
	assert_array(started).has_size(1)
	var rec: FakeHttp.Recorded = h.http.last(ExamFixtures.ROUTE_START)
	assert_str(rec.path).is_equal("/exams/sessions/%s/attempts" % ExamFixtures.SESSION_ID)
	assert_bool(Uuid.is_v7(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
	assert_str(String(rec.headers.get("x-exam-device-token", ""))).is_equal(AuthHarness.DEVICE_ID)
	assert_str(rec.raw_body).is_empty()
	assert_int(service.remaining_ms()).is_equal(ExamFixtures.DURATION_SEC * 1000)


func test_retrying_a_start_after_a_network_error_reuses_the_key() -> void:
	h.http.fail(ExamFixtures.ROUTE_START)
	assert_object(await service.start(ExamFixtures.SESSION_ID)).is_null()
	assert_str(service.last_error_code).is_equal(ApiResult.CODE_NETWORK_ERROR)
	assert_object(await _start()).is_not_null()
	var keys: Array[String] = []
	for rec: FakeHttp.Recorded in h.http.find(ExamFixtures.ROUTE_START):
		var key: String = String(rec.headers.get("idempotency-key", ""))
		if not key in keys:
			keys.append(key)
	assert_array(keys).has_size(1)


func test_each_answer_is_a_put_with_the_raw_response_only() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	assert_bool(await service.save_answer(3, {"value": true})).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ExamFixtures.ROUTE_SAVE)
	assert_str(rec.method).is_equal("PUT")
	assert_str(rec.path).is_equal("/exams/attempts/%s/answers/3" % ExamFixtures.ATTEMPT_ID)
	assert_str(String(rec.headers.get("x-exam-device-token", ""))).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(rec.body["kind"])).is_equal("question")
	assert_int(int(rec.body["position"])).is_equal(3)
	assert_dict(rec.body["response"] as Dictionary).is_equal({"value": true})
	assert_bool(Uuid.is_valid(String(rec.body.get("question_version_id", "")))).is_true()
	# F064-AC-11: no score, grading or client-computed time is ever sent.
	for key: String in ["score", "is_correct", "client_elapsed_ms", "answered_at_client", "duration_ms"]:
		assert_bool(rec.body.has(key)).override_failure_message(key).is_false()
	assert_bool(service.is_answered(3)).is_true()


func test_changing_an_answer_saves_again_and_the_latest_wins() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	await service.save_answer(1, {"option_id": "o_a"})
	await service.save_answer(1, {"option_id": "o_b"})
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(2)
	assert_dict(h.http.last(ExamFixtures.ROUTE_SAVE).body["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_dict(service.response_of(1)).is_equal({"option_id": "o_b"})


func test_saved_responses_of_the_server_are_answered() -> void:
	await _start([{"position": 2, "response": {"option_ids": ["o_a"]}}])
	assert_bool(service.is_answered(2)).is_true()
	assert_dict(service.response_of(2)).is_equal({"option_ids": ["o_a"]})
	assert_bool(service.is_answered(1)).is_false()


func test_submit_sends_a_key_kept_on_retry() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SUBMIT)
	assert_bool(await service.submit()).is_false()
	assert_bool(service.submitted).is_false()
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 200, ExamFixtures.submit_result())
	var done: Array[Dictionary] = []
	service.exam_submitted.connect(func(r: Dictionary) -> void: done.append(r))
	assert_bool(await service.submit()).is_true()
	var keys: Array[String] = []
	for rec: FakeHttp.Recorded in h.http.find(ExamFixtures.ROUTE_SUBMIT):
		assert_str(rec.path).is_equal("/exams/attempts/%s:submit" % ExamFixtures.ATTEMPT_ID)
		var key: String = String(rec.headers.get("idempotency-key", ""))
		assert_bool(Uuid.is_v7(key)).is_true()
		if not key in keys:
			keys.append(key)
	assert_array(keys).has_size(1)
	assert_bool(service.submitted).is_true()
	assert_bool(service.locked).is_true()
	assert_array(done).has_size(1)
	assert_str(h.http.last(ExamFixtures.ROUTE_SUBMIT).raw_body).is_empty()


func test_already_submitted_counts_as_submitted_and_locks_answers() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 409, ExamFixtures.problem(409, "EXAM_ALREADY_SUBMITTED"))
	assert_bool(await service.submit()).is_true()
	assert_bool(service.submitted).is_true()
	assert_bool(service.locked).is_true()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(0)


func test_already_submitted_on_save_locks_the_attempt() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 409, ExamFixtures.problem(409, "EXAM_ALREADY_SUBMITTED"))
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_bool(service.submitted).is_true()
	assert_bool(service.locked).is_true()


func test_nothing_about_answers_is_logged() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	await service.save_answer(1, {"option_id": "o_secret_answer"})
	assert_str(JSON.stringify(Log.entries())).not_contains("o_secret_answer")
