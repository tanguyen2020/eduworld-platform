extends GdUnitTestSuite
## F060-AC-25 (ADR-0086, R14, TC-AUTH-084): after save() of a unique sample, the plaintext appears nowhere in
## user:// (settings.cfg, device.cfg, secure records) nor in the Log ring buffer.
## Runs the file-backed adapter with the test cipher everywhere, and the real DPAPI adapter when the Windows
## GDExtension is installed (CI job platform-windows); the Android Keystore run needs a device/emulator.

const FakeCipher := preload("res://tests/fixtures/fake_cipher.gd")
const SECURE_DIR: String = "user://test_secure_at_rest"


func after_test() -> void:
	SecureStorageFactory.purge_file_records(SECURE_DIR)
	DirAccess.remove_absolute(SECURE_DIR)


func _assert_not_on_disk(sample: String) -> void:
	var needle: PackedByteArray = sample.to_utf8_buffer()
	for path: String in _all_files("user://"):
		var bytes: PackedByteArray = FileAccess.get_file_as_bytes(path)
		assert_bool(_contains(bytes, needle)).override_failure_message("plaintext found in %s" % path).is_false()
	assert_bool(JSON.stringify(Log.entries()).contains(sample)).is_false()


func test_file_cipher_adapter_keeps_only_ciphertext() -> void:
	var sample: String = "sample-%d-%d" % [Time.get_ticks_usec(), randi()]
	var storage: FileCipherSecureStorage = FileCipherSecureStorage.new(FakeCipher.new(), SECURE_DIR)
	assert_int(storage.save("eduworld.game.session", sample.to_utf8_buffer())).is_equal(OK)
	assert_bool(FileAccess.file_exists(storage.record_path("eduworld.game.session"))).is_true()
	_assert_not_on_disk(sample)
	assert_str(storage.load("eduworld.game.session").get_string_from_utf8()).is_equal(sample)


func test_dpapi_adapter_keeps_only_ciphertext() -> void:
	if DpapiSecureStorage.native_cipher() == null:
		# Requires the Windows GDExtension (built in CI job platform-windows); nothing else to check here.
		return
	var sample: String = "dpapi-%d-%d" % [Time.get_ticks_usec(), randi()]
	var storage: DpapiSecureStorage = DpapiSecureStorage.new(SECURE_DIR)
	assert_int(storage.save("eduworld.game.session", sample.to_utf8_buffer())).is_equal(OK)
	_assert_not_on_disk(sample)
	assert_str(storage.load("eduworld.game.session").get_string_from_utf8()).is_equal(sample)


static func _all_files(root: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var dir: DirAccess = DirAccess.open(root)
	if dir == null:
		return out
	for sub: String in dir.get_directories():
		out.append_array(_all_files(root.path_join(sub)))
	for file_name: String in dir.get_files():
		out.append(root.path_join(file_name))
	return out


static func _contains(haystack: PackedByteArray, needle: PackedByteArray) -> bool:
	if needle.is_empty() or haystack.size() < needle.size():
		return false
	for i: int in haystack.size() - needle.size() + 1:
		if haystack[i] == needle[0] and haystack.slice(i, i + needle.size()) == needle:
			return true
	return false
