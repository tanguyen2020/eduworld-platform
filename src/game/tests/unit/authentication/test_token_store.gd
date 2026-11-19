extends GdUnitTestSuite
## T009 (F061-AC-04, ADR-0086, F060-AC-26): TokenStore keeps the access token in RAM only (no API touches the disk);
## SessionStore saves/loads/erases `eduworld.game.session` through SecureStorage, erases a malformed record and
## keeps working in RAM when a write fails; PasswordBuffer wipes its bytes.


func test_token_store_apply_expiry_and_clear() -> void:
	var store: TokenStore = TokenStore.new()
	assert_bool(store.apply(AuthHarness.token_set(AuthHarness.ACCESS_1, AuthHarness.REFRESH_1, 300), 10_000)).is_true()
	assert_bool(store.access_expiring(10_000)).is_false()
	assert_bool(store.access_expiring(10_000 + 240_001)).is_true()
	assert_bool(store.apply({"access_token": 1}, 10_000)).is_false()
	assert_str(store.access_token).is_equal(AuthHarness.ACCESS_1)
	store.clear()
	assert_bool(store.has_access()).is_false()
	assert_str(store.refresh_token).is_empty()


func test_ram_only_classes_have_no_disk_api() -> void:
	for path: String in ["res://scripts/authentication/token_store.gd", "res://scripts/authentication/password_buffer.gd"]:
		var source: String = ""
		for line: String in FileAccess.get_file_as_string(path).split("\n"):
			if not line.strip_edges().begins_with("#"):
				source += line + "\n"
		for forbidden: String in ["FileAccess", "ConfigFile", "user://", "SecureStorage", "JavaScriptBridge"]:
			assert_str(source).override_failure_message("%s uses %s" % [path, forbidden]).not_contains(forbidden)


func test_session_store_round_trip_and_erase() -> void:
	var storage: FakeSecureStorage = FakeSecureStorage.new()
	var store: SessionStore = SessionStore.new(storage)
	assert_bool(store.is_persistent()).is_true()
	assert_bool(store.save(AuthHarness.REFRESH_1, AuthHarness.TENANT_ID)).is_true()
	var record: Dictionary = store.load_record()
	assert_str(String(record["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(record["tenant_id"])).is_equal(AuthHarness.TENANT_ID)
	assert_bool(String(record["saved_at"]).ends_with("Z")).is_true()
	assert_bool(record.has("access_token")).is_false()
	store.erase()
	assert_dict(store.load_record()).is_empty()
	assert_bool(SessionStore.new(MemoryOnlySecureStorage.new()).is_persistent()).is_false()


func test_malformed_or_unreadable_record_is_erased() -> void:
	var storage: FakeSecureStorage = FakeSecureStorage.new()
	var store: SessionStore = SessionStore.new(storage)
	storage.save(SessionStore.KEY, JSON.stringify({"refresh_token": "x", "tenant_id": "not-a-uuid"}).to_utf8_buffer())
	assert_dict(store.load_record()).is_empty()
	assert_bool(storage.calls.has("erase:%s" % SessionStore.KEY)).is_true()
	store.save(AuthHarness.REFRESH_1, AuthHarness.TENANT_ID)
	storage.fail_load = true
	storage.calls.clear()
	assert_dict(store.load_record()).is_empty()
	assert_bool(storage.calls.has("erase:%s" % SessionStore.KEY)).is_true()


func test_write_failure_is_reported_and_login_continues() -> void:
	var h: AuthHarness = AuthHarness.create()
	h.storage.fail_save = true
	await h.sign_in()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_str(h.auth.tokens.refresh_token).is_equal(AuthHarness.REFRESH_1)
	var events: Array[String] = []
	for e: Dictionary in AppState.pending_telemetry:
		events.append(String(e["event"]))
	assert_array(events).contains(["secure_storage_write_failed"])
	assert_str(JSON.stringify(AppState.pending_telemetry)).not_contains(AuthHarness.REFRESH_1)
	h.dispose()


func test_password_buffer_wipes() -> void:
	var buffer: PasswordBuffer = PasswordBuffer.new()
	buffer.set_text(AuthHarness.PASSWORD)
	assert_str(buffer.text()).is_equal(AuthHarness.PASSWORD)
	buffer.wipe()
	assert_bool(buffer.is_empty()).is_true()
	assert_str(buffer.text()).is_empty()
