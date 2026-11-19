extends GdUnitTestSuite
## T008 (ADR-0031, GAME_NETWORKING §2.1): `device_id` is a UUID created once and kept in `user://device_id`; debug
## builds accept `--device-id=<uuid>` (two local instances for takeover tests); release builds ignore it.

const PATH: String = "user://test_f061_device_id"


func before_test() -> void:
	DirAccess.remove_absolute(PATH)


func after_test() -> void:
	DirAccess.remove_absolute(PATH)


func test_created_once_then_reused() -> void:
	var first: String = DeviceIdentity.load_or_create(PATH, PackedStringArray(), false, false)
	assert_bool(Uuid.is_valid(first)).is_true()
	assert_bool(FileAccess.file_exists(PATH)).is_true()
	assert_str(DeviceIdentity.load_or_create(PATH, PackedStringArray(), false, false)).is_equal(first)


func test_invalid_file_is_replaced() -> void:
	var file: FileAccess = FileAccess.open(PATH, FileAccess.WRITE)
	file.store_string("garbage")
	file.close()
	var id: String = DeviceIdentity.load_or_create(PATH, PackedStringArray(), false, false)
	assert_bool(Uuid.is_valid(id)).is_true()
	assert_str(FileAccess.get_file_as_string(PATH)).is_equal(id)


func test_debug_argument_overrides_only_in_debug_builds() -> void:
	var arg: PackedStringArray = ["--device-id=AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"]
	assert_str(DeviceIdentity.load_or_create(PATH, arg, true, false)).is_equal("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	assert_str(DeviceIdentity.load_or_create(PATH, arg, false, false)).is_not_equal("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")


func test_uuid_helpers() -> void:
	var v4: String = Uuid.v4()
	var v7: String = Uuid.v7()
	assert_bool(Uuid.is_valid(v4)).is_true()
	assert_bool(Uuid.is_v7(v4)).is_false()
	assert_bool(Uuid.is_v7(v7)).is_true()
	assert_str(Uuid.v7(0).substr(0, 8)).is_equal("00000000")
	assert_str(Uuid.v4()).is_not_equal(v4)
