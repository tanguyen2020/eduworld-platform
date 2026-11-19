class_name DeviceIdentity
extends RefCounted
## Install id `device_id` (ADR-0031, GAME_NETWORKING §2.1 `X-Device-Id`): a UUID created on first start and kept
## in `user://device_id` (Web: localStorage). It is not a credential and not a hardware id. Debug builds accept
## `--device-id=<uuid>` so two local instances can test takeover (quickstart §1).

const DEFAULT_PATH: String = "user://device_id"
const WEB_STORAGE_KEY: String = "eduworld.device_id"
const ARG_PREFIX: String = "--device-id="


static func load_or_create(path: String = DEFAULT_PATH, user_args: PackedStringArray = OS.get_cmdline_user_args(),
		debug_build: bool = OS.is_debug_build(), is_web: bool = OS.has_feature("web")) -> String:
	if debug_build:
		for arg: String in user_args:
			if arg.begins_with(ARG_PREFIX) and Uuid.is_valid(arg.trim_prefix(ARG_PREFIX)):
				return arg.trim_prefix(ARG_PREFIX).to_lower()
	if is_web:
		return _load_or_create_web()
	if FileAccess.file_exists(path):
		var stored: String = FileAccess.get_file_as_string(path).strip_edges().to_lower()
		if Uuid.is_valid(stored):
			return stored
	var created: String = Uuid.v4()
	var file: FileAccess = FileAccess.open(path, FileAccess.WRITE)
	if file != null:
		file.store_string(created)
		file.close()
	return created


static func _load_or_create_web() -> String:
	var stored: Variant = JavaScriptBridge.eval("window.localStorage.getItem('%s') || ''" % WEB_STORAGE_KEY, true)
	if stored is String and Uuid.is_valid(stored as String):
		return (stored as String).to_lower()
	var created: String = Uuid.v4()
	JavaScriptBridge.eval("window.localStorage.setItem('%s', '%s')" % [WEB_STORAGE_KEY, created], true)
	return created
