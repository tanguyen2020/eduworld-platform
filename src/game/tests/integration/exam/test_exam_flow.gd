extends GdUnitTestSuite
## T011 (F064-AC-01/02/05/07/09/10, TC-EXM-002, TC-EXM-003, TC-TEN-001, TC-AUTH-014): against the backend compose —
## `hs.an.3a` opens an exam assigned to 3A (rules from `getExamSession`), starts it, answers every item with a
## well-formed response of its type (never knowing the key) and submits; `hs.chi.3b` gets `EXAM_NOT_ASSIGNED` (no
## runner, no attempt); `hs.dung.7a` (other tenant) gets `404` for the session and the result of th-demo-a. Skipped
## unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend). Needs F041/F042 seed (an open exam session for 3A).

const SKIP_REASON: String = "needs the backend compose with an open exam for 3A (F041/F042, EDUWORLD_IT_API_BASE_URL)"
const ActivityFlow := preload("res://tests/integration/learning/test_activity_flow.gd")


func test_student_takes_an_assigned_exam_and_others_cannot(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var exams: ExamService = ExamService.new(live.api)
	var open: Array[ExamSessionModel] = []
	for s: ExamSessionModel in await exams.list_sessions():
		if s.status == "open":
			open.append(s)
	assert_array(open).override_failure_message("no open exam for 3A in the seed").is_not_empty()
	var session: ExamSessionModel = await exams.load_session(open[0].exam_session_id)
	assert_object(session).is_not_null()
	assert_int(session.duration_for_student()).is_greater(0)
	var attempt: AttemptModel = await exams.start(session.exam_session_id)
	assert_object(attempt).override_failure_message(exams.last_error_code).is_not_null()
	assert_int(exams.remaining_ms()).is_greater(0)
	for item: QuestionRenderModel in attempt.items:
		assert_bool(await exams.save_answer(item.position, ActivityFlow.any_answer(item))) \
				.override_failure_message("%s %s" % [item.type, exams.last_error_code]).is_true()
	assert_bool(await exams.submit()).is_true()
	var attempt_id: String = attempt.attempt_id
	await live.auth.logout()
	live.dispose()
	var other_class: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await other_class.auth.login("hs.chi.3b", LiveBackend.student_password())
	var refused: ExamService = ExamService.new(other_class.api)
	assert_object(await refused.start(session.exam_session_id)).is_null()
	assert_str(refused.last_error_code).is_in(["EXAM_NOT_ASSIGNED", "NOT_FOUND"])
	assert_object(refused.attempt).is_null()
	await other_class.auth.logout()
	other_class.dispose()
	var other: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await other.auth.login("hs.dung.7a", LiveBackend.student_password())
	var intruder: ExamService = ExamService.new(other.api)
	assert_object(await intruder.load_session(session.exam_session_id)).is_null()
	assert_str(intruder.last_error_code).is_equal("NOT_FOUND")
	assert_object(await intruder.result(attempt_id)).is_null()
	assert_str(intruder.last_error_code).is_equal("NOT_FOUND")
	await other.auth.logout()
	other.dispose()
