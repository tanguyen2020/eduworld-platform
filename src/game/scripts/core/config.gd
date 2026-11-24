extends Node
## Autoload #1 `Config` (GODOT_ARCHITECTURE §4, F060-AC-03, F060-AC-27).
## Reads the build environment from `res://config/env/<env>.json` (no secret, no OIDC key — ADR-0068) and the
## device configuration `device.cfg` (`shared_device`, `lan_cache_url` — ADR-0078, ADR-0086).
## The environment comes from `--env=<name>` (debug builds only) or the project setting `eduworld/config/env`
## (overridden per export preset through a custom feature tag). Debug builds may merge `user://config_override.json`.

const ENV_DIR: String = "res://config/env"
const OVERRIDE_PATH: String = "user://config_override.json"
const REQUIRED_KEYS: PackedStringArray = [
	"env", "api_base_url", "realtime_url", "cdn_base_url", "login_path", "client_version",
]
const STRING_KEYS: PackedStringArray = [
	"env", "api_base_url", "realtime_url", "cdn_base_url", "login_path", "client_version", "log_level",
]
const OPTIONAL_KEYS: PackedStringArray = ["is_mock", "feature_flags_default", "log_level"]
## Keys or key fragments that must never appear in a client configuration (secrets, OIDC — F060-AC-03).
const FORBIDDEN_KEY_FRAGMENTS: PackedStringArray = [
	"secret", "password", "oidc", "issuer", "client_id", "token", "private_key", "api_key",
]
const FORBIDDEN_VALUE_FRAGMENTS: PackedStringArray = ["/protocol/openid-connect/"]
const ENV_NAME_PATTERN: String = "^[a-z][a-z0-9_]{1,31}$"

const ERR_ENV_INVALID: String = "CONFIG_ENV_INVALID"
const ERR_FILE_MISSING: String = "CONFIG_FILE_MISSING"
const ERR_PARSE: String = "CONFIG_PARSE_ERROR"
const ERR_KEY_MISSING: String = "CONFIG_KEY_MISSING"
const ERR_KEY_FORBIDDEN: String = "CONFIG_KEY_FORBIDDEN"
const ERR_KEY_UNKNOWN: String = "CONFIG_KEY_UNKNOWN"
const ERR_TYPE: String = "CONFIG_VALUE_TYPE"

var env_name: String = ""
var api_base_url: String = ""
var realtime_url: String = ""
var cdn_base_url: String = ""
var login_path: String = ""
var client_version: String = ""
var log_level: String = "info"
var is_mock: bool = false
var feature_flags_default: Dictionary = {}
## Error code of the last load (empty = valid). Boot shows ConfigError when not empty.
var error_code: String = ""
## Offending key of the last load error (never a value).
var error_key: String = ""

var shared_device: bool = true
var lan_cache_url: String = ""


func _enter_tree() -> void:
	load_env(resolve_env_name(OS.get_cmdline_user_args(), OS.is_debug_build()))
	load_device_config(String(ProjectSettings.get_setting("eduworld/device/config_path", "user://device.cfg")),
		platform_key(OS.get_name(), OS.has_feature("web")))


func is_valid() -> bool:
	return error_code.is_empty()


## `--env=<name>` wins in debug builds; release builds always use the preset value.
static func resolve_env_name(user_args: PackedStringArray, debug_build: bool) -> String:
	if debug_build:
		for arg: String in user_args:
			if arg.begins_with("--env="):
				return arg.trim_prefix("--env=")
	return String(ProjectSettings.get_setting_with_override("eduworld/config/env"))


static func env_path(name: String) -> String:
	return "%s/%s.json" % [ENV_DIR, name]


## Loads and validates one environment file. Returns OK or an error; `error_code`/`error_key` describe it.
func load_env(name: String, allow_override: bool = OS.is_debug_build()) -> Error:
	_reset()
	var regex: RegEx = RegEx.create_from_string(ENV_NAME_PATTERN)
	if regex.search(name) == null:
		return _fail(ERR_ENV_INVALID, "env")
	var path: String = env_path(name)
	if not FileAccess.file_exists(path):
		return _fail(ERR_FILE_MISSING, "env")
	var data: Variant = JSON.parse_string(FileAccess.get_file_as_string(path))
	if not data is Dictionary:
		return _fail(ERR_PARSE, "env")
	var merged: Dictionary = data as Dictionary
	if allow_override and FileAccess.file_exists(OVERRIDE_PATH):
		var extra: Variant = JSON.parse_string(FileAccess.get_file_as_string(OVERRIDE_PATH))
		if extra is Dictionary:
			merged.merge(extra as Dictionary, true)
	var err: Error = validate(merged)
	if err != OK:
		return err
	_apply(merged)
	return OK


## Validates a configuration dictionary without applying it (also used by tests and the artifact scan).
func validate(data: Dictionary) -> Error:
	for key: Variant in data.keys():
		var k: String = String(key).to_lower()
		for fragment: String in FORBIDDEN_KEY_FRAGMENTS:
			if k.contains(fragment):
				return _fail(ERR_KEY_FORBIDDEN, String(key))
		if not (k in REQUIRED_KEYS or k in OPTIONAL_KEYS):
			return _fail(ERR_KEY_UNKNOWN, String(key))
		var value: Variant = data[key]
		if value is String:
			for fragment: String in FORBIDDEN_VALUE_FRAGMENTS:
				if (value as String).contains(fragment):
					return _fail(ERR_KEY_FORBIDDEN, String(key))
	for key: String in REQUIRED_KEYS:
		if not data.has(key) or String(data[key]).strip_edges().is_empty():
			return _fail(ERR_KEY_MISSING, key)
	for key: String in STRING_KEYS:
		if data.has(key) and not data[key] is String:
			return _fail(ERR_TYPE, key)
	if data.has("is_mock") and not data["is_mock"] is bool:
		return _fail(ERR_TYPE, "is_mock")
	if data.has("feature_flags_default") and not data["feature_flags_default"] is Dictionary:
		return _fail(ERR_TYPE, "feature_flags_default")
	return OK


## Platform key used for `eduworld/device/shared_device_default/<key>` (CONFIG_DEFAULTS `shared_device_default`).
## Operating systems that are not released (Windows, macOS, Linux: editor or debug runs only, ADR-0101) are `other`.
static func platform_key(os_name: String, is_web: bool) -> String:
	if is_web:
		return "web"
	match os_name:
		"Android":
			return "android"
		"iOS":
			return "ios"
		_:
			return "other"


static func shared_device_default(platform: String) -> bool:
	var key: String = "eduworld/device/shared_device_default/%s" % platform
	if ProjectSettings.has_setting(key):
		return bool(ProjectSettings.get_setting(key))
	return bool(ProjectSettings.get_setting("eduworld/device/shared_device_default/other", true))


## Reads `device.cfg` ([device] shared_device, lan_cache_url). A missing file or key falls back to the platform
## default (`other`: shared). Web is always treated as shared (MemoryOnly, ADR-0086).
func load_device_config(path: String, platform: String) -> void:
	shared_device = shared_device_default(platform)
	lan_cache_url = ""
	var cfg: ConfigFile = ConfigFile.new()
	if FileAccess.file_exists(path) and cfg.load(path) == OK:
		if cfg.has_section_key("device", "shared_device") and cfg.get_value("device", "shared_device") is bool:
			shared_device = cfg.get_value("device", "shared_device") as bool
		var lan: Variant = cfg.get_value("device", "lan_cache_url", "")
		if lan is String:
			lan_cache_url = lan as String
	if platform == "web":
		shared_device = true


func _reset() -> void:
	error_code = ""
	error_key = ""
	env_name = ""
	api_base_url = ""
	realtime_url = ""
	cdn_base_url = ""
	login_path = ""
	client_version = ""
	log_level = "info"
	is_mock = false
	feature_flags_default = {}


func _fail(code: String, key: String) -> Error:
	error_code = code
	error_key = key
	return ERR_INVALID_DATA


func _apply(data: Dictionary) -> void:
	env_name = String(data["env"])
	api_base_url = String(data["api_base_url"])
	realtime_url = String(data["realtime_url"])
	cdn_base_url = String(data["cdn_base_url"])
	login_path = String(data["login_path"])
	client_version = String(data["client_version"])
	log_level = String(data.get("log_level", "info"))
	is_mock = bool(data.get("is_mock", false))
	feature_flags_default = data.get("feature_flags_default", {}) as Dictionary
