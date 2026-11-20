extends GdUnitTestSuite
## F063-AC-10, F063-AC-11 (TC-LRN-008, TC-AUTH-014, TC-TEN-001; T014): attempt errors map to `ui.error.<CODE>`
## messages (never the server text) and keep the answer; `404` (attempt of another student/tenant) and
## `403 GAME_ACCESS_STUDENT_ONLY` drop the attempt from RAM so nothing of it can be shown, the latter ending the
## session like F061.

const CODES: Dictionary = {
	"ATTEMPT_EXPIRED": 410, "ASSIGNMENT_NOT_OPEN": 403, "ASSIGNMENT_MAX_ATTEMPTS": 409,
	"ATTEMPT_NOT_IN_PROGRESS": 409, "INVALID_RESPONSE": 422,
}

var h: AuthHarness = null
var service: LearningService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = LearningService.new(h.api, null, h.clock)


func after_test() -> void:
	h.dispose()


func test_each_attempt_error_has_its_own_message() -> void:
	for code: Variant in CODES.keys():
		var key: String = "ui.error." + String(code)
		assert_bool(key in L10n.ERROR_KEYS).override_failure_message(key).is_true()
		assert_bool(L10n.has_key(key)).override_failure_message(key).is_true()
		assert_str(L10n.error_text(String(code))).is_not_equal(tr("ui.error.generic"))


func test_refused_answer_keeps_the_attempt_and_reports_the_code() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})
	for code: Variant in CODES.keys():
		h.http.reply(LearningFixtures.ROUTE_RESPOND, int(CODES[code]), LearningFixtures.problem(int(CODES[code]), String(code)))
		assert_object(await service.respond(1, {"option_id": "o_a"}, 0)).is_null()
		assert_str(service.last_error_code).is_equal(String(code))
		assert_object(service.attempt).override_failure_message(String(code)).is_not_null()
		assert_bool(service.offline).is_false()


func test_start_refusals_report_their_code() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 403, LearningFixtures.problem(403, "ASSIGNMENT_NOT_OPEN"))
	assert_object(await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})).is_null()
	assert_str(service.last_error_code).is_equal("ASSIGNMENT_NOT_OPEN")


func test_attempt_of_another_student_shows_nothing() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})
	h.http.reply("GET /learning/activity-attempts/" + LearningFixtures.OTHER_ATTEMPT_ID, 404, LearningFixtures.problem(404, "NOT_FOUND"))
	assert_object(await service.resume(LearningFixtures.OTHER_ATTEMPT_ID)).is_null()
	assert_object(service.attempt).is_null()
	assert_str(service.last_error_code).is_equal("NOT_FOUND")


func test_respond_404_drops_the_attempt() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 404, LearningFixtures.problem(404, "NOT_FOUND"))
	await service.respond(1, {"option_id": "o_a"}, 0)
	assert_object(service.attempt).is_null()


func test_non_student_token_ends_the_session_like_f061() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	services.access_denied_handler = h.auth.handle_access_denied
	h.http.reply(LearningFixtures.ROUTE_START, 403, LearningFixtures.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	var denied: Array[String] = []
	services.learning.access_denied.connect(func(code: String) -> void: denied.append(code))
	assert_object(await services.learning.start({"activity_id": LearningFixtures.ACTIVITY_ID})).is_null()
	assert_array(denied).is_equal(["GAME_ACCESS_STUDENT_ONLY"])
	assert_object(services.learning.attempt).is_null()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.NOT_STUDENT)
	assert_bool(h.api.tokens.has_access()).is_false()


func test_unknown_codes_fall_back_to_the_generic_message() -> void:
	assert_str(L10n.error_text("SOMETHING_NEW")).is_equal(tr("ui.error.generic"))
