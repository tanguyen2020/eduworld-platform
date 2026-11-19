class_name AndroidKeystoreSecureStorage
extends SecureStorage
## Android adapter (ADR-0086): the Android plugin `EduWorldPlatform` (addons/eduworld_platform/android, Kotlin)
## seals data with an AES-GCM 256 key generated inside Android Keystore (alias `eduworld.session`, not exportable)
## and keeps only ciphertext in the app's `noBackupFilesDir`. Without the plugin `is_available()` is false and
## SecureStorageFactory falls back to MemoryOnly.

const SINGLETON: StringName = &"EduWorldPlatform"

var _plugin: Object = null


func _init(plugin: Object = null) -> void:
	_plugin = plugin if plugin != null else native_plugin()


func adapter_name() -> String:
	return "android_keystore"


static func native_plugin() -> Object:
	if Engine.has_singleton(SINGLETON):
		return Engine.get_singleton(SINGLETON)
	return null


func is_available() -> bool:
	return _plugin != null and bool(_plugin.call("secureIsAvailable"))


func save(key: String, data: PackedByteArray) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	if not is_available():
		_set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)
		_log_warn(LOG_WRITE_FAILED, key, ERR_UNAVAILABLE)
		return ERR_UNAVAILABLE
	var result: int = int(_plugin.call("secureSave", key, data))
	if result != OK:
		_set_error(result as Error, CODE_WRITE_FAILED)
		_log_warn(LOG_WRITE_FAILED, key, result as Error)
		return result as Error
	_clear_error()
	return OK


func load(key: String) -> PackedByteArray:
	if not is_valid_key(key):
		_set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
		return PackedByteArray()
	if not is_available():
		_set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)
		return PackedByteArray()
	var plain: PackedByteArray = _plugin.call("secureLoad", key) as PackedByteArray
	var status: int = int(_plugin.call("secureLastError"))
	if status == ERR_FILE_NOT_FOUND:
		_set_error(ERR_FILE_NOT_FOUND, CODE_NOT_FOUND)
		return PackedByteArray()
	if status != OK or plain.is_empty():
		_set_error(ERR_FILE_CORRUPT, CODE_UNREADABLE)
		_log_warn(LOG_READ_FAILED, key, ERR_FILE_CORRUPT)
		return PackedByteArray()
	_clear_error()
	return plain


func erase(key: String) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	if _plugin == null:
		_clear_error()
		return OK
	var result: int = int(_plugin.call("secureErase", key))
	if result != OK:
		return _set_error(ERR_FILE_CANT_WRITE, CODE_ERASE_FAILED)
	_clear_error()
	return OK


## Removes every record of the plugin (used when the device is switched to shared, F060-AC-24).
func erase_all() -> Error:
	if _plugin == null:
		return OK
	return int(_plugin.call("secureEraseAll")) as Error
