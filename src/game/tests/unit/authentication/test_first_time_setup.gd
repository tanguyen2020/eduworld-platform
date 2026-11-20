extends GdUnitTestSuite
## F070-AC-01 (client part; TC-GAM-011, CHARACTER_SYSTEM §5.0, STUDENT_GAME_UX_CORE S02 §3): once the game session is
## created the client reads `GET /game/profile`; `setup_required = true` opens S02 Character Creation before the Main
## Hub, otherwise the student goes straight to the hub. S02 confirmed → hub. The resume path behaves the same.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_new_student_goes_to_character_creation_before_the_hub() -> void:
	ProfileFixtures.script_first_time(h.http)
	await h.sign_in()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CHARACTER_CREATION)
	assert_array(h.states).not_contains([GameStateMachine.State.IN_WORLD])
	assert_str(GameStateMachine.scene_for(GameStateMachine.State.CHARACTER_CREATION)) \
			.is_equal("res://scenes/character/CharacterCreation.tscn")
	# The profile is read after the session exists (EnsureGameProfile runs in that transaction).
	var order: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.requests:
		order.append("%s %s" % [rec.method, rec.path])
	assert_int(order.find("GET /game/profile")).is_greater(order.find("POST /game/sessions"))
	assert_str(String(h.http.last(ProfileFixtures.ROUTE_PROFILE).headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)


func test_student_with_avatar_and_nickname_goes_straight_to_the_hub() -> void:
	await h.sign_in()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_array(h.states).not_contains([GameStateMachine.State.CHARACTER_CREATION])
	assert_int(h.http.count(ProfileFixtures.ROUTE_PROFILE)).is_equal(1)


func test_profile_read_failure_never_enters_the_hub() -> void:
	h.http.clear(ProfileFixtures.ROUTE_PROFILE)
	h.http.reply(ProfileFixtures.ROUTE_PROFILE, 500, AuthHarness.problem(500, "INTERNAL"))
	await h.sign_in()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CHARACTER_CREATION)


func test_confirmed_character_creation_enters_the_hub() -> void:
	ProfileFixtures.script_first_time(h.http)
	await h.sign_in()
	h.auth.complete_character_creation()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


func test_completion_is_ignored_outside_character_creation() -> void:
	await h.sign_in()
	var before: int = h.states.size()
	h.auth.complete_character_creation()
	assert_int(h.states.size()).is_equal(before)


func test_resume_of_a_student_without_avatar_opens_character_creation() -> void:
	h.store_session()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/token:refresh", 200,
			AuthHarness.refresh_ok(AuthHarness.ACCESS_1, AuthHarness.REFRESH_2, true))
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	ProfileFixtures.script_first_time(h.http)
	await h.auth.start()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.RESUME_CONFIRM)
	await h.auth.continue_resume()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.CHARACTER_CREATION)


func test_nickname_from_the_profile_reaches_app_state_but_not_the_log() -> void:
	await h.sign_in()
	assert_str(LoginFormModel.nickname_of(AppState.player)).is_equal("Cú Mèo Chăm Chỉ 27")
	assert_bool(JSON.stringify(Log.entries()).contains("Cú Mèo")).is_false()
