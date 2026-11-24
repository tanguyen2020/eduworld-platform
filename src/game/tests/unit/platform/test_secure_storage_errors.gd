extends GdUnitTestSuite
## F060-AC-26 (ADR-0086): an unreadable record (corrupt file, other OS user, deleted key) makes load() return an
## empty array with an error code, without crashing; erase() removes it; a failed save returns an error and logs
## `secure_storage_write_failed` without the value.

const FakeCipher := preload("res://tests/fixtures/fake_cipher.gd")
const TEST_DIR: String = "user://test_secure_errors"
const SAMPLE: String = "refresh-token-sample-7f3a"

var _storage: FileCipherSecureStorage = null
var _cipher: RefCounted = null


func before_test() -> void:
	_cipher = FakeCipher.new()
	_storage = FileCipherSecureStorage.new(_cipher, TEST_DIR)
	Log.clear()


func after_test() -> void:
	SecureStorageFactory.purge_file_records(TEST_DIR)
	DirAccess.remove_absolute(TEST_DIR)


func test_corrupt_record_loads_empty_with_code_and_can_be_erased() -> void:
	assert_int(_storage.save("t", SAMPLE.to_utf8_buffer())).is_equal(OK)
	var file: FileAccess = FileAccess.open(_storage.record_path("t"), FileAccess.WRITE)
	var noise: PackedByteArray = PackedByteArray()
	for i: int in 64:
		noise.append(randi() % 256)
	file.store_buffer(noise)
	file.close()
	assert_int(_storage.load("t").size()).is_equal(0)
	assert_int(_storage.last_error).is_equal(ERR_FILE_CORRUPT)
	assert_str(_storage.last_error_code).is_equal(SecureStorage.CODE_UNREADABLE)
	assert_int(_storage.erase("t")).is_equal(OK)
	assert_bool(FileAccess.file_exists(_storage.record_path("t"))).is_false()


func test_record_of_another_cipher_key_is_unreadable() -> void:
	assert_int(_storage.save("t", SAMPLE.to_utf8_buffer())).is_equal(OK)
	var bytes: PackedByteArray = FileAccess.get_file_as_bytes(_storage.record_path("t"))
	bytes[bytes.size() - 1] = (bytes[bytes.size() - 1] + 1) % 256
	var file: FileAccess = FileAccess.open(_storage.record_path("t"), FileAccess.WRITE)
	file.store_buffer(bytes)
	file.close()
	assert_int(_storage.load("t").size()).is_equal(0)
	assert_str(_storage.last_error_code).is_equal(SecureStorage.CODE_UNREADABLE)


func test_failed_save_returns_error_and_logs_without_value() -> void:
	_cipher.set("fail_protect", true)
	var err: Error = _storage.save("eduworld.game.session", SAMPLE.to_utf8_buffer())
	assert_int(err).is_not_equal(OK)
	assert_str(_storage.last_error_code).is_equal(SecureStorage.CODE_WRITE_FAILED)
	var entries: Array[Dictionary] = Log.entries()
	var events: Array[String] = []
	for entry: Dictionary in entries:
		events.append(String(entry["event"]))
	assert_array(events).contains([SecureStorage.LOG_WRITE_FAILED])
	assert_bool(JSON.stringify(entries).contains(SAMPLE)).is_false()
	assert_bool(FileAccess.file_exists(_storage.record_path("eduworld.game.session"))).is_false()


func test_unavailable_cipher_never_writes_plaintext() -> void:
	_cipher.set("available", false)
	assert_int(_storage.save("t", SAMPLE.to_utf8_buffer())).is_equal(ERR_UNAVAILABLE)
	assert_bool(FileAccess.file_exists(_storage.record_path("t"))).is_false()

