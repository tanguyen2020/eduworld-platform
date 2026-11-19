class_name FakeSecureStorage
extends MemoryOnlySecureStorage
## Test double for F061 (AuthService tests, GODOT_ARCHITECTURE §12): an in-memory SecureStorage that can be told to
## fail writes or reads, and records the calls. Lives under tests/ (never exported).

var fail_save: bool = false
var fail_load: bool = false
var calls: Array[String] = []


func adapter_name() -> String:
	return "fake"


func is_memory_only() -> bool:
	return false


func save(key: String, data: PackedByteArray) -> Error:
	calls.append("save:%s" % key)
	if fail_save:
		_set_error(ERR_CANT_CREATE, CODE_WRITE_FAILED)
		_log_warn(LOG_WRITE_FAILED, key, ERR_CANT_CREATE)
		return ERR_CANT_CREATE
	return super(key, data)


func load(key: String) -> PackedByteArray:
	calls.append("load:%s" % key)
	if fail_load:
		_set_error(ERR_FILE_CORRUPT, CODE_UNREADABLE)
		return PackedByteArray()
	return super(key)


func erase(key: String) -> Error:
	calls.append("erase:%s" % key)
	return super(key)
