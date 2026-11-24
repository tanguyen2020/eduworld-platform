class_name SecureStorage
extends RefCounted
## Platform secure storage interface (ADR-0086, GODOT_ARCHITECTURE §5.2, F060-AC-23…26).
## Keeps the secrets of a game session (refresh token + tenant of the session, written by F061) in the operating
## system's protected store; adapters never log values and never write plaintext to disk.
## `load()` returns an empty array on failure and sets `last_error` / `last_error_code` instead of crashing.

const CODE_OK: String = ""
const CODE_INVALID_KEY: String = "SECURE_STORAGE_INVALID_KEY"
const CODE_UNAVAILABLE: String = "SECURE_STORAGE_UNAVAILABLE"
const CODE_NOT_FOUND: String = "SECURE_STORAGE_NOT_FOUND"
const CODE_UNREADABLE: String = "SECURE_STORAGE_UNREADABLE"
const CODE_WRITE_FAILED: String = "SECURE_STORAGE_WRITE_FAILED"
const CODE_ERASE_FAILED: String = "SECURE_STORAGE_ERASE_FAILED"
const LOG_WRITE_FAILED: String = "secure_storage_write_failed"
const LOG_READ_FAILED: String = "secure_storage_read_failed"
const KEY_PATTERN: String = "^[a-z0-9][a-z0-9._-]{0,63}$"

var last_error: Error = OK
var last_error_code: String = CODE_OK


## Name of the adapter for logs and tests (`memory_only`, `file_cipher`, `android_keystore`).
func adapter_name() -> String:
	return "abstract"


## True when the backing store can be used on this device.
func is_available() -> bool:
	return false


## True when the adapter keeps data only in RAM (nothing survives the process).
func is_memory_only() -> bool:
	return false


func save(_key: String, _data: PackedByteArray) -> Error:
	return _set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)


func load(_key: String) -> PackedByteArray:
	_set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)
	return PackedByteArray()


func erase(_key: String) -> Error:
	return _set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)


static func is_valid_key(key: String) -> bool:
	return RegEx.create_from_string(KEY_PATTERN).search(key) != null


func _set_error(err: Error, code: String) -> Error:
	last_error = err
	last_error_code = code
	return err


func _clear_error() -> void:
	last_error = OK
	last_error_code = CODE_OK


## Logs through the `Log` autoload with the key and error only — never the value (AGENTS R14).
func _log_warn(event: String, key: String, err: Error) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	if tree == null:
		return
	var log_node: Node = tree.root.get_node_or_null("Log")
	if log_node != null:
		log_node.call("warn", event, {"adapter": adapter_name(), "key": key, "error": err, "code": last_error_code})
