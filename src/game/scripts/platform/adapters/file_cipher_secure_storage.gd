class_name FileCipherSecureStorage
extends SecureStorage
## File-backed adapter whose bytes are sealed by an OS cipher (ADR-0086): only ciphertext reaches the disk, in
## `<dir>/<key>.bin`, written to a temporary file then renamed. The cipher object provides
## `is_available() -> bool`, `protect(PackedByteArray) -> PackedByteArray` and `unprotect(PackedByteArray) ->
## PackedByteArray` (empty result = failure); the Windows DPAPI GDExtension is one such cipher.

const DEFAULT_DIR: String = "user://secure"
const FILE_EXTENSION: String = ".bin"
const TEMP_SUFFIX: String = ".tmp"

var directory: String = DEFAULT_DIR
var _cipher: Object = null


func _init(cipher: Object = null, dir: String = DEFAULT_DIR) -> void:
	_cipher = cipher
	directory = dir


func adapter_name() -> String:
	return "file_cipher"


func is_available() -> bool:
	return _cipher != null and _cipher.has_method("protect") and bool(_cipher.call("is_available"))


func record_path(key: String) -> String:
	return directory.path_join(key + FILE_EXTENSION)


func save(key: String, data: PackedByteArray) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	if not is_available():
		_set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)
		_log_warn(LOG_WRITE_FAILED, key, ERR_UNAVAILABLE)
		return ERR_UNAVAILABLE
	var sealed: PackedByteArray = _cipher.call("protect", data) as PackedByteArray
	if sealed.is_empty():
		return _write_failed(key, ERR_CANT_CREATE)
	var err: Error = DirAccess.make_dir_recursive_absolute(directory)
	if err != OK:
		return _write_failed(key, err)
	var final_path: String = record_path(key)
	var temp_path: String = final_path + TEMP_SUFFIX
	var file: FileAccess = FileAccess.open(temp_path, FileAccess.WRITE)
	if file == null:
		return _write_failed(key, FileAccess.get_open_error())
	file.store_buffer(sealed)
	file.close()
	if FileAccess.file_exists(final_path):
		DirAccess.remove_absolute(final_path)
	err = DirAccess.rename_absolute(temp_path, final_path)
	if err != OK:
		DirAccess.remove_absolute(temp_path)
		return _write_failed(key, err)
	_clear_error()
	return OK


func load(key: String) -> PackedByteArray:
	if not is_valid_key(key):
		_set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
		return PackedByteArray()
	var path: String = record_path(key)
	if not FileAccess.file_exists(path):
		_set_error(ERR_FILE_NOT_FOUND, CODE_NOT_FOUND)
		return PackedByteArray()
	if not is_available():
		_set_error(ERR_UNAVAILABLE, CODE_UNAVAILABLE)
		return PackedByteArray()
	var sealed: PackedByteArray = FileAccess.get_file_as_bytes(path)
	var plain: PackedByteArray = PackedByteArray()
	if not sealed.is_empty():
		plain = _cipher.call("unprotect", sealed) as PackedByteArray
	if plain.is_empty():
		# Corrupt file, another OS user, or a deleted key: the record cannot be used (F060-AC-26).
		_set_error(ERR_FILE_CORRUPT, CODE_UNREADABLE)
		_log_warn(LOG_READ_FAILED, key, ERR_FILE_CORRUPT)
		return PackedByteArray()
	_clear_error()
	return plain


func erase(key: String) -> Error:
	if not is_valid_key(key):
		return _set_error(ERR_INVALID_PARAMETER, CODE_INVALID_KEY)
	var path: String = record_path(key)
	for candidate: String in [path, path + TEMP_SUFFIX]:
		if FileAccess.file_exists(candidate) and DirAccess.remove_absolute(candidate) != OK:
			return _set_error(ERR_FILE_CANT_WRITE, CODE_ERASE_FAILED)
	_clear_error()
	return OK


func _write_failed(key: String, err: Error) -> Error:
	_set_error(err, CODE_WRITE_FAILED)
	_log_warn(LOG_WRITE_FAILED, key, err)
	return err
