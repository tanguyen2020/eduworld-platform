class_name MemoryOnlySecureStorage
extends SecureStorage
## RAM-only store (ADR-0086): Web, `shared_device = true`, platforms without a native adapter, or a native store
## that is not available. Nothing is written to `user://`, the OS store or localStorage; data ends with the process.

var _items: Dictionary = {}


func adapter_name() -> String:
	return "memory_only"


func is_available() -> bool:
	return true


func is_memory_only() -> bool:
	return true


func save(key: String, data: PackedByteArray) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	_items[key] = data.duplicate()
	_clear_error()
	return OK


func load(key: String) -> PackedByteArray:
	if not is_valid_key(key):
		_set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
		return PackedByteArray()
	if not _items.has(key):
		_set_error(ERR_FILE_NOT_FOUND, CODE_NOT_FOUND)
		return PackedByteArray()
	_clear_error()
	return (_items[key] as PackedByteArray).duplicate()


func erase(key: String) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	_items.erase(key)
	_clear_error()
	return OK
