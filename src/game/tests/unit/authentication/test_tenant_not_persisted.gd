extends GdUnitTestSuite
## F061-AC-15 (ADR-0086, ADR-0065): the chosen tenant is kept only inside the SessionStore record (for `X-Tenant-Id`
## on resume) — no plain `user://` file holds it, there is no `user://tenant.cfg`, TenantChoice pre-selects
## nothing, and the tenant never changes within a session.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _select_other_tenant() -> void:
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	h.http.reply("POST /auth/game/tenant:select", 200, AuthHarness.login_ok(AuthHarness.ACCESS_1, AuthHarness.REFRESH_1,
			AuthHarness.OTHER_TENANT_ID))
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.select_tenant(AuthHarness.OTHER_TENANT_ID)


func test_tenant_lives_only_in_the_secure_record() -> void:
	await _select_other_tenant()
	assert_str(String(h.stored_record()["tenant_id"])).is_equal(AuthHarness.OTHER_TENANT_ID)
	assert_bool(FileAccess.file_exists("user://tenant.cfg")).is_false()
	var dir: DirAccess = DirAccess.open("user://")
	for file_name: String in dir.get_files():
		var text: String = FileAccess.get_file_as_string("user://".path_join(file_name))
		assert_str(text).override_failure_message(file_name).not_contains(AuthHarness.OTHER_TENANT_ID)


func test_tenant_does_not_change_within_the_session() -> void:
	await _select_other_tenant()
	h.http.reply("GET /game/profile", 200, {})
	await h.api.send(ApiRequest.make_get("/game/profile"))
	assert_str(String(h.http.last("GET /game/profile").headers["x-tenant-id"])).is_equal(AuthHarness.OTHER_TENANT_ID)


func test_resume_answer_for_another_tenant_is_rejected() -> void:
	# The session tenant is fixed: a resume bootstrap for another tenant ends the stored session.
	h.store_session(AuthHarness.REFRESH_1, AuthHarness.OTHER_TENANT_ID)
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))
	await h.auth.start()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.http.count("POST /game/sessions")).is_equal(0)


func test_tenant_choice_screen_preselects_nothing() -> void:
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	var previous: Callable = AuthService.navigator
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	var saved: Array[ApiProblem.TenantChoice] = AuthService.choices.duplicate()
	AuthService.choices = h.auth.choices.duplicate()
	AuthService.navigator = func(_s: int) -> void: pass
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/auth/TenantChoice.tscn")
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	assert_int(int(scene.call("card_count"))).is_equal(2)
	var list: Node = scene.find_child("List", true, false)
	for card: Node in list.get_children():
		assert_bool((card as Button).button_pressed).is_false()
	assert_bool((scene.find_child("Enter", true, false) as Button).disabled).is_true()
	AuthService.choices = saved
	AuthService.navigator = previous
