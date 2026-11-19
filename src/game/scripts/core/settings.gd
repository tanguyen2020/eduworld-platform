extends Node
## Autoload #11 `Settings` (GODOT_ARCHITECTURE §4, §10, F060-AC-09, F060-AC-14, F060-AC-15).
## Per-device user preferences in `user://settings.cfg`: language, font scale (3 levels), high contrast, reduced
## motion, volumes, and (F061) the remembered login username `last_username` (F061-AC-12, ADR-0086). Only the keys
## in SCHEMA are ever written — never business data, tokens or passwords.

signal font_scale_changed(level: int)
signal high_contrast_changed(enabled: bool)
signal reduce_motion_changed(enabled: bool)
signal volume_changed(bus_name: String, value: float)

const DEFAULT_PATH: String = "user://settings.cfg"
const SECTION_UI: String = "ui"
const SECTION_AUDIO: String = "audio"
const SECTION_LOGIN: String = "login"
## `GameLoginRequest.username` maxLength (specs/openapi).
const USERNAME_MAX_LENGTH: int = 128
const FONT_SCALE_MAX: int = 2
const AUDIO_KEYS: PackedStringArray = ["music", "sfx", "voice"]
## Allowed keys per section; anything else in the file is ignored and never written back.
const SCHEMA: Dictionary = {
	"ui": ["locale", "font_scale", "high_contrast", "reduce_motion"],
	"audio": ["music", "sfx", "voice"],
	"login": ["remember_username", "last_username"],
}

var path: String = DEFAULT_PATH
var locale: String = ""
var font_scale: int = 0
var high_contrast: bool = false
var reduce_motion: bool = false
var volumes: Dictionary = {"music": 1.0, "sfx": 1.0, "voice": 1.0}
## "Nhớ tên đăng nhập" (CONFIG_DEFAULTS `game_remember_username`: on; AuthService never stores it on a shared device).
var remember_username: bool = true
var last_username: String = ""


func _ready() -> void:
	register_pseudo_locale(OS.is_debug_build())
	load_from(path)
	apply_locale(locale, false)


## Debug builds only: loads the generated pseudo-locale (`xx-PS`, F060-AC-17). Release presets exclude the file.
static func register_pseudo_locale(debug_build: bool) -> bool:
	var pseudo_path: String = String(ProjectSettings.get_setting("eduworld/i18n/pseudo_locale_path", ""))
	if not debug_build or pseudo_path.is_empty() or not ResourceLoader.exists(pseudo_path):
		return false
	var translation: Translation = load(pseudo_path) as Translation
	if translation == null:
		return false
	if TranslationServer.get_translation_object(translation.locale) != translation:
		TranslationServer.add_translation(translation)
	return true


## Reads the file; invalid or unknown values fall back to defaults (a locale outside `supported_locales`
## becomes the default locale).
func load_from(file_path: String) -> void:
	path = file_path
	_reset()
	var cfg: ConfigFile = ConfigFile.new()
	if not FileAccess.file_exists(file_path) or cfg.load(file_path) != OK:
		locale = L10n.default_locale()
		return
	locale = L10n.resolve_locale(String(cfg.get_value(SECTION_UI, "locale", "")))
	var scale: Variant = cfg.get_value(SECTION_UI, "font_scale", 0)
	if scale is int:
		font_scale = clampi(scale as int, 0, FONT_SCALE_MAX)
	var contrast: Variant = cfg.get_value(SECTION_UI, "high_contrast", false)
	high_contrast = contrast is bool and (contrast as bool)
	var motion: Variant = cfg.get_value(SECTION_UI, "reduce_motion", false)
	reduce_motion = motion is bool and (motion as bool)
	for bus_key: String in AUDIO_KEYS:
		var value: Variant = cfg.get_value(SECTION_AUDIO, bus_key, 1.0)
		if value is float or value is int:
			volumes[bus_key] = clampf(float(value), 0.0, 1.0)
	var remember: Variant = cfg.get_value(SECTION_LOGIN, "remember_username", true)
	remember_username = not (remember is bool) or (remember as bool)
	var username: Variant = cfg.get_value(SECTION_LOGIN, "last_username", "")
	if username is String and remember_username:
		last_username = (username as String).strip_edges().left(USERNAME_MAX_LENGTH)


## Writes only the SCHEMA keys (fresh ConfigFile, so stray keys never survive).
func save() -> Error:
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value(SECTION_UI, "locale", locale)
	cfg.set_value(SECTION_UI, "font_scale", font_scale)
	cfg.set_value(SECTION_UI, "high_contrast", high_contrast)
	cfg.set_value(SECTION_UI, "reduce_motion", reduce_motion)
	for bus_key: String in AUDIO_KEYS:
		cfg.set_value(SECTION_AUDIO, bus_key, float(volumes[bus_key]))
	cfg.set_value(SECTION_LOGIN, "remember_username", remember_username)
	if remember_username and not last_username.is_empty():
		cfg.set_value(SECTION_LOGIN, "last_username", last_username)
	var err: Error = cfg.save(path)
	if err != OK:
		_log_warn("settings_save_failed", {"error": err})
	return err


## Changes the language at runtime (F060-AC-14): validates against `supported_locales`, sets the
## TranslationServer locale, emits EventBus.locale_changed. Returns false when the locale is not allowed.
func set_locale(value: String, persist: bool = true) -> bool:
	if not L10n.is_supported(value):
		_log_warn("locale_rejected", {"locale": value})
		return false
	apply_locale(value, true)
	if persist:
		save()
	return true


func apply_locale(value: String, notify: bool) -> void:
	locale = value if L10n.is_supported(value) else L10n.default_locale()
	TranslationServer.set_locale(locale)
	if notify:
		var bus: Node = get_node_or_null("/root/EventBus")
		if bus != null:
			bus.emit_signal("locale_changed", locale)


func set_font_scale(level: int) -> void:
	var next: int = clampi(level, 0, FONT_SCALE_MAX)
	if next == font_scale:
		return
	font_scale = next
	save()
	font_scale_changed.emit(font_scale)


func set_high_contrast(enabled: bool) -> void:
	if enabled == high_contrast:
		return
	high_contrast = enabled
	save()
	high_contrast_changed.emit(high_contrast)


func set_reduce_motion(enabled: bool) -> void:
	if enabled == reduce_motion:
		return
	reduce_motion = enabled
	save()
	reduce_motion_changed.emit(reduce_motion)


func set_volume(bus_key: String, value: float) -> void:
	if not bus_key in AUDIO_KEYS:
		return
	volumes[bus_key] = clampf(value, 0.0, 1.0)
	save()
	volume_changed.emit(bus_key, float(volumes[bus_key]))


## Remembered username (F061-AC-12): kept only while `remember` is on; the password is never stored.
func set_login_memory(remember: bool, username: String) -> void:
	remember_username = remember
	last_username = username.strip_edges().left(USERNAME_MAX_LENGTH) if remember else ""
	save()


func get_volume(bus_key: String) -> float:
	return float(volumes.get(bus_key, 1.0))


func _reset() -> void:
	locale = L10n.default_locale()
	font_scale = 0
	high_contrast = false
	reduce_motion = false
	volumes = {"music": 1.0, "sfx": 1.0, "voice": 1.0}
	remember_username = true
	last_username = ""


func _log_warn(event: String, fields: Dictionary) -> void:
	var log_node: Node = get_node_or_null("/root/Log")
	if log_node != null:
		log_node.call("warn", event, fields)
