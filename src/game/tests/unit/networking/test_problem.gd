extends GdUnitTestSuite
## F061-AC-20 (ADR-0010, GAME_NETWORKING §2.2): Problem Details parse into typed results — `code`, `trace_id`,
## `errors[]` and the extension members `choices`, `login_ticket`, `other_device`, `min_client_version`,
## `store_url`, `reopen_at`; ApiResult maps network errors, problems and non-problem errors to a `code`.


func _response(status: int, body: Variant, headers: PackedStringArray = []) -> HttpTransport.Response:
	var r: HttpTransport.Response = HttpTransport.Response.new()
	r.result = HTTPRequest.RESULT_SUCCESS
	r.status = status
	r.headers = headers
	r.body = JSON.stringify(body).to_utf8_buffer() if body != null else PackedByteArray()
	return r


func test_tenant_choice_problem_is_typed() -> void:
	var problem: ApiProblem = ApiProblem.from_dict(AuthHarness.tenant_choices())
	assert_str(problem.code).is_equal("TENANT_NOT_SELECTED")
	assert_int(problem.status).is_equal(403)
	assert_str(problem.login_ticket).is_equal(AuthHarness.TICKET)
	assert_int(problem.choices.size()).is_equal(2)
	var first: ApiProblem.TenantChoice = problem.choices[0]
	assert_str(first.tenant_id).is_equal(AuthHarness.TENANT_ID)
	assert_str(first.name).is_equal("TH Demo A")
	assert_str(first.school_name).is_not_empty()


func test_active_session_conflict_has_other_device() -> void:
	var problem: ApiProblem = ApiProblem.from_dict(AuthHarness.problem(409, "ACTIVE_SESSION_ON_OTHER_DEVICE", {},
			"ActiveSessionConflict"))
	assert_object(problem.other_device).is_not_null()
	assert_str(problem.other_device.platform).is_equal("windows")
	assert_str(problem.other_device.last_seen_at).is_equal("2026-11-15T07:30:00Z")


func test_client_version_problem_and_curfew_fields() -> void:
	var version: ApiProblem = ApiProblem.from_dict(AuthHarness.problem(426, "CLIENT_VERSION_UNSUPPORTED",
			{"min_client_version": "1.4.0"}, "ClientVersionProblem"))
	assert_str(version.min_client_version).is_equal("1.4.0")
	assert_str(version.store_url).starts_with("https://")
	var curfew: ApiProblem = ApiProblem.from_dict(AuthHarness.problem(403, "CURFEW_ACTIVE", {"reopen_at": "2026-11-20T23:30:00Z"}))
	assert_str(curfew.reopen_at).is_equal("2026-11-20T23:30:00Z")


func test_validation_errors_and_unknown_code() -> void:
	var body: Dictionary = AuthHarness.problem(422, "VALIDATION_FAILED", {"errors": [{"field": "username", "code": "too_long"}, "bad"]})
	var problem: ApiProblem = ApiProblem.from_dict(body)
	assert_int(problem.errors.size()).is_equal(1)
	assert_str(String(problem.errors[0]["field"])).is_equal("username")
	var unknown: ApiProblem = ApiProblem.from_dict(AuthHarness.problem(403, "SOMETHING_NEW"))
	assert_str(unknown.message()).is_equal(TranslationServer.translate("ui.error.generic"))


func test_api_result_codes() -> void:
	var problem: ApiResult = ApiResult.from_response(_response(401, AuthHarness.problem(401, "INVALID_CREDENTIALS"),
			PackedStringArray(["Retry-After: 12"])))
	assert_bool(problem.ok).is_false()
	assert_str(problem.code).is_equal("INVALID_CREDENTIALS")
	assert_object(problem.problem).is_not_null()
	assert_float(problem.retry_after_sec()).is_equal(12.0)
	var plain: ApiResult = ApiResult.from_response(_response(500, null))
	assert_str(plain.code).is_equal("HTTP_500")
	assert_bool(plain.is_transient()).is_true()
	var offline: ApiResult = ApiResult.from_response(HttpTransport.Response.new())
	assert_bool(offline.network_error).is_true()
	assert_str(offline.code).is_equal(ApiResult.CODE_NETWORK_ERROR)
	var ok: ApiResult = ApiResult.from_response(_response(201, AuthHarness.session_ok()))
	assert_bool(ok.ok).is_true()
	assert_str(String(ok.body()["game_session_id"])).is_equal(AuthHarness.SESSION_ID)
	assert_str(ok.code).is_empty()
