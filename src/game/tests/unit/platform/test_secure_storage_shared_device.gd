extends GdUnitTestSuite
## F060-AC-24 (ADR-0086, TC-AUTH-084): a shared device always gets MemoryOnly; nothing is written to user://, and
## records left by an earlier personal-device setup are removed.

const FakeCipher := preload("res://tests/fixtures/fake_cipher.gd")
const SECURE_DIR: String = FileCipherSecureStorage.DEFAULT_DIR


func after_test() -> void:
	SecureStorageFactory.purge_file_records(SECURE_DIR)


func _files_in(dir_path: String) -> PackedStringArray:
	var dir: DirAccess = DirAccess.open(dir_path)
	return dir.get_files() if dir != null else PackedStringArray()


func test_shared_device_gets_memory_only_on_every_platform() -> void:
	for platform: String in ["windows", "android", "ios", "web", "other"]:
		var storage: SecureStorage = SecureStorageFactory.create(true, platform)
		assert_str(storage.adapter_name()).is_equal("memory_only")
		assert_bool(storage.is_memory_only()).is_true()


func test_memory_only_writes_nothing_and_forgets_on_restart() -> void:
	var before: PackedStringArray = _files_in("user://")
	var storage: SecureStorage = SecureStorageFactory.create(true, "windows")
	assert_int(storage.save("eduworld.game.session", "refresh-sample".to_utf8_buffer())).is_equal(OK)
	assert_str(storage.load("eduworld.game.session").get_string_from_utf8()).is_equal("refresh-sample")
	assert_array(Array(_files_in("user://"))).contains_exactly_in_any_order(Array(before))
	assert_int(_files_in(SECURE_DIR).size()).is_equal(0)
	# A new process creates a new adapter: nothing survives.
	var restarted: SecureStorage = SecureStorageFactory.create(true, "windows")
	assert_int(restarted.load("eduworld.game.session").size()).is_equal(0)


func test_switching_to_shared_device_removes_old_records() -> void:
	var personal: FileCipherSecureStorage = FileCipherSecureStorage.new(FakeCipher.new(), SECURE_DIR)
	assert_int(personal.save("eduworld.game.session", "old".to_utf8_buffer())).is_equal(OK)
	assert_int(_files_in(SECURE_DIR).size()).is_equal(1)
	SecureStorageFactory.create(true, "windows")
	assert_int(_files_in(SECURE_DIR).size()).is_equal(0)
