extends GdUnitTestSuite
## F061-AC-32 (TC-AUTH-084, ADR-0086): on a `shared_device` (and on Web) SecureStorage is MemoryOnly: nothing of the
## session survives in the OS store, `user://` or localStorage; closing the window is a logout; the next start
## always shows the form.

var h: AuthHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null


func test_factory_uses_memory_only_for_shared_device_and_web() -> void:
	assert_bool(SecureStorageFactory.create(true, "windows").is_memory_only()).is_true()
	assert_bool(SecureStorageFactory.create(true, "android").is_memory_only()).is_true()
	assert_bool(SecureStorageFactory.create(false, "web").is_memory_only()).is_true()


func test_shared_device_session_is_ram_only_and_close_logs_out() -> void:
	h = AuthHarness.create(true, false)
	assert_bool(h.auth.session_store.is_persistent()).is_false()
	await h.sign_in()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_int(h.storage.calls.size()).is_equal(0)
	await h.auth.close_game()
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_dict(h.auth.session_store.load_record()).is_empty()
	var dir: DirAccess = DirAccess.open("user://")
	for file_name: String in dir.get_files():
		var text: String = FileAccess.get_file_as_bytes("user://".path_join(file_name)).get_string_from_ascii()
		assert_str(text).override_failure_message(file_name).not_contains(AuthHarness.REFRESH_1)


func test_next_start_shows_the_form() -> void:
	h = AuthHarness.create(true, false)
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	await h.auth.start()
	assert_array(h.states).is_equal([GameStateMachine.State.LOGIN])
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(0)
	assert_bool(h.auth.form.remember_visible).is_false()
