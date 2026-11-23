extends GdUnitTestSuite
## F062-AC-07, F062-AC-12, F062-AC-23 (TC-LMS-002, TC-AUTH-014, ADR-0065; T014, T029): AssignmentService reads
## `listMyAssignments` through ApiClient with the session `X-Tenant-Id`, mirrors the list into AppState, refreshes the
## token once on 401, ends the session on account/tenant refusals (TENANT_MISMATCH, GAME_ACCESS_STUDENT_ONLY,
## ACCESS_OUTSIDE_ALLOWED_HOURS with the bootstrap access window) and keeps the previous list on network/5xx errors.
## Runs on FakeHttp/FakeClock/FakeSecureStorage (AuthHarness) with bodies from the contract fixtures.

const ROUTE: String = "GET /learning/my-assignments"
const WINDOW: String = "06:30-21:30 Asia/Ho_Chi_Minh"

var h: AuthHarness = null
var services: ServiceLocator = null
var denied: Array[String] = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	denied.clear()
	services.access_denied_handler = _on_denied


func after_test() -> void:
	h.dispose()
	var none: Array[MyAssignmentModel] = []
	AppState.set_assignments(none)


func _on_denied(code: String) -> void:
	denied.append(code)


func test_refresh_sends_session_headers_and_updates_app_state() -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	var counts: Array[int] = []
	services.assignments.assignments_updated.connect(func(n: int) -> void: counts.append(n))
	var ok: bool = await services.assignments.refresh()
	assert_bool(ok).is_true()
	assert_int(services.assignments.state).is_equal(AssignmentService.State.READY)
	assert_int(AppState.assignments.size()).is_equal(2)
	assert_array(counts).contains([2])
	var rec: FakeHttp.Recorded = h.http.last(ROUTE)
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
	assert_str(String(rec.headers.get("authorization", ""))).is_equal("Bearer " + AuthHarness.ACCESS_1)
	assert_str(rec.query).contains("limit=%d" % AssignmentService.PAGE_LIMIT)
	var first: MyAssignmentModel = AppState.assignments[0]
	assert_str(first.content_kind).is_equal("activity")
	assert_str(AppState.assignments[1].exam_session_id).is_not_empty()


func test_pages_are_followed_with_the_cursor() -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.page([AssignmentFixtures.item_sample(0)], true, "c2"))
	h.http.reply(ROUTE, 200, AssignmentFixtures.page([AssignmentFixtures.item_sample(1)]))
	assert_bool(await services.assignments.refresh()).is_true()
	assert_int(h.http.count(ROUTE)).is_equal(2)
	assert_str(h.http.last(ROUTE).query).contains("cursor=c2")
	assert_int(AppState.assignments.size()).is_equal(2)


func test_concurrent_refreshes_share_one_request() -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	services.assignments.refresh()
	var ok: bool = await services.assignments.refresh()
	assert_bool(ok).is_true()
	assert_int(h.http.count(ROUTE)).is_equal(1)


func test_401_refreshes_the_token_once_then_replays() -> void:
	h.http.reply(ROUTE, 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	h.http.reply("POST /auth/game/token:refresh", 200,
			AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	assert_bool(await services.assignments.refresh()).is_true()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	assert_int(h.http.count(ROUTE)).is_equal(2)
	assert_str(String(h.http.last(ROUTE).headers["authorization"])).is_equal("Bearer " + AuthHarness.ACCESS_2)
	assert_int(AppState.assignments.size()).is_equal(2)


func test_401_after_refresh_is_not_retried_again() -> void:
	h.http.reply(ROUTE, 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply("POST /auth/game/token:refresh", 200,
			AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	assert_bool(await services.assignments.refresh()).is_false()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	assert_int(h.http.count(ROUTE)).is_equal(2)
	assert_int(services.assignments.state).is_equal(AssignmentService.State.ERROR)


func test_tenant_mismatch_erases_the_session_and_shows_login() -> void:
	h.http.reply(ROUTE, 403, AuthHarness.problem(403, "TENANT_MISMATCH"))
	assert_bool(await services.assignments.refresh()).is_false()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_bool(h.api.tokens.has_access()).is_false()
	assert_bool(h.stored_record().is_empty()).is_true()
	assert_bool(AppState.assignments.is_empty()).is_true()


func test_student_only_refusal_ends_the_session_like_f061() -> void:
	services.access_denied_handler = h.auth.handle_access_denied
	h.http.reply(ROUTE, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	var expired: Array[bool] = []
	var on_expired: Callable = func() -> void: expired.append(true)
	EventBus.session_expired.connect(on_expired)
	assert_bool(await services.assignments.refresh()).is_false()
	EventBus.session_expired.disconnect(on_expired)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.NOT_STUDENT)
	assert_bool(h.api.tokens.has_access()).is_false()
	assert_bool(h.stored_record().is_empty()).is_true()
	assert_array(expired).is_not_empty()


func test_network_error_and_5xx_keep_the_previous_list() -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	assert_bool(await services.assignments.refresh()).is_true()
	h.http.fail(ROUTE)
	assert_bool(await services.assignments.refresh()).is_false()
	assert_int(services.assignments.state).is_equal(AssignmentService.State.ERROR)
	assert_str(services.assignments.last_error_code).is_equal(ApiResult.CODE_NETWORK_ERROR)
	assert_int(AppState.assignments.size()).is_equal(2)
	h.http.reply(ROUTE, 503, AuthHarness.problem(503, "UNAVAILABLE"))
	assert_bool(await services.assignments.refresh()).is_false()
	assert_int(AppState.assignments.size()).is_equal(2)
	assert_array(denied).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


## F062-AC-23: outside the access window → the form with the bootstrap `access_window`, session erased.
func test_outside_access_window_returns_to_login_with_the_window() -> void:
	services.access_denied_handler = h.auth.handle_access_denied
	AppState.school_policies = {"play_time": {"access_window": WINDOW}}
	h.http.reply(ROUTE, 403, AuthHarness.problem(403, "ACCESS_OUTSIDE_ALLOWED_HOURS"))
	assert_bool(await services.assignments.refresh()).is_false()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(h.auth.form.banner_code).is_equal("ACCESS_OUTSIDE_ALLOWED_HOURS")
	var expected: String = TranslationServer.translate("ui.common.outside_access_window").format({"window": WINDOW})
	assert_str(h.auth.form.banner_text()).is_equal(expected)
	assert_str(h.auth.form.banner_text()).contains(WINDOW)
	assert_bool(h.api.tokens.has_access()).is_false()
	assert_bool(h.stored_record().is_empty()).is_true()


func test_model_parses_contract_and_ignores_unknown_fields() -> void:
	var items: Array = AssignmentFixtures.unsupported_kinds()["data"] as Array
	var list: Array[MyAssignmentModel] = MyAssignmentModel.list_from(items + [{"title": "no id"}, "x"])
	assert_int(list.size()).is_equal(3)
	assert_bool(list[2].is_known_kind()).is_false()
	assert_bool(list[2].is_known_status()).is_false()
	var sample: MyAssignmentModel = MyAssignmentModel.from_dict(AssignmentFixtures.item_sample(0))
	assert_bool(sample.allow_late).is_false()
	assert_int(sample.due_unix()).is_greater(0)
	var unlimited: Dictionary = AssignmentFixtures.item_sample(1)
	unlimited["attempts_left"] = null
	unlimited["late_policy"] = {"allow_late": true, "late_window_hours": 999}
	var m: MyAssignmentModel = MyAssignmentModel.from_dict(unlimited)
	assert_int(m.attempts_left).is_equal(-1)
	assert_bool(m.allow_late).is_true()
	assert_int(m.late_window_hours).is_equal(168)
	assert_object(MyAssignmentModel.from_dict({"assignment_id": "not-a-uuid", "title": "x", "status": "graded"})).is_null()


func test_model_reads_subject_name_and_ignores_a_malformed_one() -> void:
	# OQ F062-1 (T036): `subject_name` is a LearningName {vi?, en?}; absent or malformed → no subject shown.
	var items: Array = AssignmentFixtures.subjects()["data"] as Array
	var named: MyAssignmentModel = MyAssignmentModel.from_dict(items[0] as Dictionary)
	assert_dict(named.subject_name).is_equal({"vi": "Toán", "en": "Mathematics"})
	assert_str(named.subject_text("vi-VN")).is_equal("Toán")
	assert_str(named.subject_text("en-US")).is_equal("Mathematics")
	assert_str(MyAssignmentModel.from_dict(items[1] as Dictionary).subject_text("vi-VN")).is_empty()
	var odd: Dictionary = AssignmentFixtures.item_sample(5)
	for bad: Variant in ["Toán", 3, {"vi": 7, "en": ""}, null]:
		odd["subject_name"] = bad
		assert_str(MyAssignmentModel.from_dict(odd).subject_text("vi-VN")).override_failure_message(str(bad)).is_empty()
	odd["subject_name"] = {"vi": "x".repeat(400)}
	assert_int(MyAssignmentModel.from_dict(odd).subject_text("vi-VN").length()).is_equal(MyAssignmentModel.NAME_MAX_LENGTH)
