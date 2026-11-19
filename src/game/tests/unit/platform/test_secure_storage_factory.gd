extends GdUnitTestSuite
## F060-AC-23 (ADR-0086, TC-AUTH-082 storage part): the factory picks the adapter per platform; every adapter
## implements is_available/save/load/erase with the same round-trip behaviour.

const FakeCipher := preload("res://tests/fixtures/fake_cipher.gd")
const TEST_DIR: String = "user://test_secure_factory"


func after_test() -> void:
	SecureStorageFactory.purge_file_records(TEST_DIR)
	DirAccess.remove_absolute(TEST_DIR)


func test_web_and_unknown_platforms_get_memory_only() -> void:
	assert_str(SecureStorageFactory.create(false, "web").adapter_name()).is_equal("memory_only")
	assert_str(SecureStorageFactory.create(false, "other").adapter_name()).is_equal("memory_only")
	assert_str(SecureStorageFactory.create(false, "ios").adapter_name()).is_equal("memory_only")


func test_windows_uses_dpapi_when_the_extension_is_installed() -> void:
	var storage: SecureStorage = SecureStorageFactory.create(false, "windows")
	var expected: String = "dpapi" if DpapiSecureStorage.native_cipher() != null else "memory_only"
	assert_str(storage.adapter_name()).is_equal(expected)


func test_android_uses_keystore_when_the_plugin_is_present() -> void:
	var storage: SecureStorage = SecureStorageFactory.create(false, "android")
	var expected: String = "android_keystore" if AndroidKeystoreSecureStorage.native_plugin() != null else "memory_only"
	assert_str(storage.adapter_name()).is_equal(expected)


func test_native_adapters_without_native_code_are_unavailable() -> void:
	if DpapiSecureStorage.native_cipher() == null:
		assert_bool(DpapiSecureStorage.new(TEST_DIR).is_available()).is_false()
	if AndroidKeystoreSecureStorage.native_plugin() == null:
		assert_bool(AndroidKeystoreSecureStorage.new().is_available()).is_false()


func test_every_adapter_round_trips() -> void:
	var adapters: Array[SecureStorage] = [
		MemoryOnlySecureStorage.new(),
		FileCipherSecureStorage.new(FakeCipher.new(), TEST_DIR),
	]
	if DpapiSecureStorage.native_cipher() != null:
		adapters.append(DpapiSecureStorage.new(TEST_DIR))
	for storage: SecureStorage in adapters:
		var value: PackedByteArray = "abc".to_utf8_buffer()
		assert_bool(storage.is_available()).is_true()
		assert_int(storage.save("t", value)).is_equal(OK)
		assert_str(storage.load("t").get_string_from_utf8()).is_equal("abc")
		assert_int(storage.erase("t")).is_equal(OK)
		assert_int(storage.load("t").size()).is_equal(0)
		assert_str(storage.last_error_code).is_equal(SecureStorage.CODE_NOT_FOUND)


func test_invalid_keys_are_rejected() -> void:
	var storage: SecureStorage = MemoryOnlySecureStorage.new()
	for key: String in ["", "../x", "Upper", "a/b"]:
		assert_int(storage.save(key, PackedByteArray([1]))).is_equal(ERR_INVALID_PARAMETER)
		assert_str(storage.last_error_code).is_equal(SecureStorage.CODE_INVALID_KEY)


func test_fake_secure_storage_records_calls() -> void:
	var fake: FakeSecureStorage = FakeSecureStorage.new()
	fake.save("eduworld.game.session", "x".to_utf8_buffer())
	fake.load("eduworld.game.session")
	fake.erase("eduworld.game.session")
	assert_array(fake.calls).is_equal(["save:eduworld.game.session", "load:eduworld.game.session", "erase:eduworld.game.session"])
