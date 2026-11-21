extends GdUnitTestSuite
## F064-AC-09, F064-AC-10 (TC-EXM-002, TC-AUTH-014, TC-TEN-001; T010): a refused start (`403 EXAM_NOT_ASSIGNED`,
## `EXAM_NOT_OPEN`, `EXAM_WINDOW_CLOSED`, `409 EXAM_MAX_ATTEMPTS`) returns no attempt and its own `ui.error.<CODE>`
## message; a `404` (exam/attempt of another tenant or student) and `403 GAME_ACCESS_STUDENT_ONLY` leave nothing of the
## exam in RAM, the latter ending the session like F061 (`access_denied`).

const START_CODES: Dictionary = {
	"EXAM_NOT_ASSIGNED": 403, "EXAM_NOT_OPEN": 403, "EXAM_WINDOW_CLOSED": 403, "EXAM_MAX_ATTEMPTS": 409,
}
const OTHER_CODES: PackedStringArray = [
	"EXAM_ACTIVE_ON_OTHER_DEVICE", "EXAM_TIME_OVER", "EXAM_ALREADY_SUBMITTED", "EXAM_RESULTS_NOT_RELEASED",
]

var h: AuthHarness = null
var service: ExamService = null
var denied: Array[String] = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = ExamService.new(h.api, h.clock)
	denied.clear()
	service.access_denied.connect(func(code: String) -> void: denied.append(code))


func after_test() -> void:
	h.dispose()


func test_every_exam_code_has_its_own_message() -> void:
	var codes: PackedStringArray = PackedStringArray(START_CODES.keys())
	codes.append_array(OTHER_CODES)
	for code: String in codes:
		var key: String = "ui.error." + code
		assert_bool(key in L10n.ERROR_KEYS).override_failure_message(key).is_true()
		assert_bool(L10n.has_key(key)).override_failure_message(key).is_true()
		assert_str(L10n.error_text(code)).is_not_equal(tr("ui.error.generic"))


func test_refused_start_returns_no_attempt_and_the_code() -> void:
	for code: Variant in START_CODES.keys():
		var status: int = int(START_CODES[code])
		h.http.reply(ExamFixtures.ROUTE_START, status, ExamFixtures.problem(status, String(code)))
		assert_object(await service.start(ExamFixtures.SESSION_ID)).is_null()
		assert_str(service.last_error_code).is_equal(String(code))
		assert_object(service.attempt).is_null()
	assert_array(denied).is_empty()


func test_not_found_session_leaves_nothing_in_ram() -> void:
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session())
	await service.load_session(ExamFixtures.SESSION_ID)
	h.http.reply(ExamFixtures.ROUTE_SESSION, 404, ExamFixtures.problem(404, "NOT_FOUND"))
	assert_object(await service.load_session(ExamFixtures.OTHER_SESSION_ID)).is_null()
	assert_object(service.session).is_null()
	assert_str(service.last_error_code).is_equal("NOT_FOUND")


func test_not_found_attempt_is_dropped() -> void:
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await service.start(ExamFixtures.SESSION_ID)
	h.http.reply(ExamFixtures.ROUTE_SAVE, 404, ExamFixtures.problem(404, "NOT_FOUND"))
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_object(service.attempt).is_null()
	assert_bool(service.locked).is_true()


func test_staff_token_ends_the_session_and_shows_nothing() -> void:
	h.http.reply(ExamFixtures.ROUTE_SESSION, 403, ExamFixtures.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	assert_object(await service.load_session(ExamFixtures.SESSION_ID)).is_null()
	assert_object(service.session).is_null()
	assert_array(denied).is_equal(["GAME_ACCESS_STUDENT_ONLY"])
	h.http.reply(ExamFixtures.ROUTE_START, 403, ExamFixtures.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	assert_object(await service.start(ExamFixtures.SESSION_ID)).is_null()
	assert_object(service.attempt).is_null()
	assert_array(denied).has_size(2)


func test_result_of_another_tenant_is_not_found() -> void:
	h.http.reply(ExamFixtures.ROUTE_RESULT, 404, ExamFixtures.problem(404, "NOT_FOUND"))
	assert_object(await service.result(ExamFixtures.ATTEMPT_ID)).is_null()
	assert_str(service.last_error_code).is_equal("NOT_FOUND")
