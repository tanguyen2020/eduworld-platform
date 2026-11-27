extends Node
## Autoload #1 `Config` (GODOT_ARCHITECTURE §4, F060-AC-03, F060-AC-27).
## Reads the build environment from `res://config/env/<env>.json` (no secret, no OIDC key — ADR-0068) and the
## device configuration `device.cfg` (`shared_device`, `lan_cache_url` — ADR-0078, ADR-0086).
## The environment comes from `--env=<name>` (debug builds only) or the project setting `eduworld/config/env`
## (overridden per export preset through a custom feature tag). Debug builds may merge `user://config_override.json`;
## release builds never do (F133-AC-02). A release file is written by scripts/godot/gen-config.mjs (F133 T003) and adds
## `build_number` (github.run_number), `store_urls` (store page per platform for the "Cần cập nhật" screen),
## `bundles_base_url`/`releases_base_url` (public CDN of zone bundles / release manifests) and `manifest_public_key`
## (RSA public key checking the signed bundle index and release manifest — ManifestVerifier, research R2).

const ENV_DIR: String = "res://config/env"
const OVERRIDE_PATH: String = "user://config_override.json"
const REQUIRED_KEYS: PackedStringArray = [
	"env", "api_base_url", "realtime_url", "cdn_base_url", "login_path", "client_version",
]
const STRING_KEYS: PackedStringArray = [
	"env", "api_base_url", "realtime_url", "cdn_base_url", "login_path", "client_version", "log_level",
]
const OPTIONAL_KEYS: PackedStringArray = [
	"is_mock", "feature_flags_default", "log_level", "build_number", "store_urls", "bundles_base_url",
	"releases_base_url", "manifest_public_key",
]
const OPTIONAL_STRING_KEYS: PackedStringArray = ["bundles_base_url", "releases_base_url", "manifest_public_key"]
const STORE_PLATFORMS: PackedStringArray = ["android", "ios"]
const URL_MAX: int = 2048
const PUBLIC_KEY_MAX: int = 4096
## Keys or key fragments that must never appear in a client configuration (secrets, OIDC — F060-AC-03).
const FORBIDDEN_KEY_FRAGMENTS: PackedStringArray = [
	"secret", "password", "oidc", "issuer", "client_id", "token", "private_key", "api_key",
]
const FORBIDDEN_VALUE_FRAGMENTS: PackedStringArray = ["/protocol/openid-connect/", "PRIVATE KEY"]
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
## Release build number (github.run_number); 0 in local/dev files.
var build_number: int = 0
## Store page per platform (`android`, `ios`): https only.
var store_urls: Dictionary = {}
var bundles_base_url: String = ""
var releases_base_url: String = ""
var manifest_public_key: String = ""
## Error code of the last load (empty = valid). Boot shows ConfigError when not empty.
var error_code: String = ""
## Offending key of the last load error (never a value).
var error_key: String = ""

var shared_device: bool = true
var lan_cache_url: String = ""
## Whole device.cfg (F133 T036: low_data_mode, preinstalled_bundles_dir, grade_bands, warnings).
var device: DeviceConfig = DeviceConfig.parse("", "other")


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


## Only debug builds may merge `user://config_override.json` (F133-AC-02).
static func override_allowed(debug_build: bool) -> bool:
	return debug_build


static func env_path(name: String) -> String:
	return "%s/%s.json" % [ENV_DIR, name]


## Loads and validates one environment file. Returns OK or an error; `error_code`/`error_key` describe it.
func load_env(name: String, allow_override: bool = override_allowed(OS.is_debug_build())) -> Error:
	_reset()
	var regex: RegEx = RegEx.create_from_string(ENV_NAME_PATTERN)
	if regex.search(name) == null:
		return _fail(ERR_ENV_INVALID, "env")
	return load_file(env_path(name), allow_override)


## Loads and validates one configuration file (an env file, or a generated release file in tests).
func load_file(path: String, allow_override: bool = override_allowed(OS.is_debug_build())) -> Error:
	_reset()
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
	return _validate_release_keys(data)


func _validate_release_keys(data: Dictionary) -> Error:
	if data.has("build_number") and not (data["build_number"] is float or data["build_number"] is int):
		return _fail(ERR_TYPE, "build_number")
	for key: String in OPTIONAL_STRING_KEYS:
		if data.has(key) and not data[key] is String:
			return _fail(ERR_TYPE, key)
	for key: String in ["bundles_base_url", "releases_base_url"]:
		if data.has(key) and (String(data[key]).length() > URL_MAX or not String(data[key]).begins_with("http")):
			return _fail(ERR_TYPE, key)
	if data.has("manifest_public_key"):
		var pem: String = String(data["manifest_public_key"])
		if pem.length() > PUBLIC_KEY_MAX or not pem.begins_with("-----BEGIN PUBLIC KEY-----"):
			return _fail(ERR_TYPE, "manifest_public_key")
	if data.has("store_urls"):
		if not data["store_urls"] is Dictionary:
			return _fail(ERR_TYPE, "store_urls")
		var stores: Dictionary = data["store_urls"] as Dictionary
		for platform: Variant in stores.keys():
			var url: Variant = stores[platform]
			if not String(platform) in STORE_PLATFORMS or not url is String:
				return _fail(ERR_TYPE, "store_urls")
			if not (url as String).begins_with("https://") or (url as String).length() > URL_MAX:
				return _fail(ERR_TYPE, "store_urls")
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


## Reads `device.cfg` through DeviceConfig (F133 T036). A missing file or key falls back to the platform default
## (`other`: shared); an invalid `lan_cache_url` is ignored with a warning (F133-AC-18). Web is always treated as shared
## (MemoryOnly, ADR-0086) and has no LAN cache or preinstalled packs (ADR-0101).
func load_device_config(path: String, platform: String) -> void:
	device = DeviceConfig.load_file(path, platform, shared_device_default(platform))
	shared_device = device.shared_device
	lan_cache_url = device.lan_cache_url
	for warning: String in device.warnings:
		push_warning("device.cfg: %s" % warning)


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
	build_number = 0
	store_urls = {}
	bundles_base_url = ""
	releases_base_url = ""
	manifest_public_key = ""


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
	build_number = int(data.get("build_number", 0))
	store_urls = (data.get("store_urls", {}) as Dictionary).duplicate()
	bundles_base_url = String(data.get("bundles_base_url", ""))
	releases_base_url = String(data.get("releases_base_url", ""))
	manifest_public_key = String(data.get("manifest_public_key", ""))
