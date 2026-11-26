extends Control
## Cài đặt in the world (F062): language vi/en (applied at once, F062-AC-06), graphics quality (T027) and Tài khoản →
## "Đăng xuất" with LogoutConfirm (F061-AC-23, moved here from the removed WorldPlaceholder). F096: the chosen
## language and preset are `ChoiceChip`s with a check icon (not colour only, F096-AC-13); the **Thông tin** group has
## "Xem lại hướng dẫn của Mây" (F096-AC-10) and "Giấy phép" — a page with the game version, the MIT licence of Godot
## Engine and its third-party notices read from the engine, and the project's own licences (F096-AC-04). UI only.

signal closed()

const LOCALE_VI: String = "vi-VN"
const LOCALE_EN: String = "en-US"
const ICON_CHECK: Texture2D = preload("res://resources/ui/icons/check.svg")

var _language_group: ButtonGroup = ButtonGroup.new()

@onready var _close: Button = %Close
@onready var _lang_vi: Button = %LangVi
@onready var _lang_en: Button = %LangEn
@onready var _logout: Button = %Logout
@onready var _logout_confirm: PanelContainer = %LogoutConfirm
@onready var _main: Control = %Main
@onready var _licenses: Control = %LicensesPage
@onready var _open_licenses: Button = %OpenLicenses
@onready var _replay_guide: Button = %ReplayGuide
@onready var _back: Button = %Back
@onready var _license_text: Label = %LicenseText
@onready var _version: Label = %AboutVersion


func _ready() -> void:
	_close.pressed.connect(close)
	for chip: Button in [_lang_vi, _lang_en]:
		chip.toggle_mode = true
		chip.button_group = _language_group
	_lang_vi.pressed.connect(_on_language_pressed.bind(LOCALE_VI))
	_lang_en.pressed.connect(_on_language_pressed.bind(LOCALE_EN))
	_logout.pressed.connect(_on_logout_pressed)
	_open_licenses.pressed.connect(show_licenses)
	_replay_guide.pressed.connect(_on_replay_guide_pressed)
	_back.pressed.connect(show_main)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render_language()
	show_main()


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func open(_args: Dictionary = {}) -> void:
	show_main()
	show()


func close() -> void:
	_logout_confirm.hide()
	hide()
	closed.emit()


## Cài đặt → Thông tin → Giấy phép (F096-AC-04). The licence texts are legal texts shown verbatim in English.
func show_licenses() -> void:
	_main.hide()
	_licenses.show()
	_version.text = tr("ui.login.version").format({"version": Config.client_version})
	if _license_text.text.is_empty():
		_license_text.text = license_text()
	_back.grab_focus.call_deferred()


func show_main() -> void:
	_licenses.hide()
	_main.show()


func licenses_visible() -> bool:
	return _licenses.visible


## Godot Engine's MIT licence, then the copyright lines of its third-party components and their licence texts — all from
## the engine itself, so the notice always matches the engine that runs (Godot licence compliance).
static func license_text() -> String:
	var parts: PackedStringArray = [Engine.get_license_text().strip_edges()]
	var used: Dictionary = {}
	for component: Dictionary in Engine.get_copyright_info():
		var lines: PackedStringArray = [String(component.get("name", ""))]
		for part: Dictionary in component.get("parts", []) as Array:
			for line: String in part.get("copyright", PackedStringArray()) as PackedStringArray:
				lines.append("  © " + line)
			var license_id: String = String(part.get("license", ""))
			if not license_id.is_empty():
				lines.append("  License: " + license_id)
				used[license_id] = true
		parts.append("\n".join(lines))
	var texts: Dictionary = Engine.get_license_info()
	var shown: Dictionary = {}
	for license_id: String in used:
		for alternative: String in license_id.split(" or ", false):
			for id: String in alternative.split(" and ", false):
				var key: String = id.strip_edges().trim_prefix("(").trim_suffix(")")
				if texts.has(key) and not shown.has(key):
					shown[key] = true
					parts.append("— %s —\n%s" % [key, String(texts[key]).strip_edges()])
	return "\n\n".join(parts)


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		if _licenses.visible:
			show_main()
		else:
			close()
		get_viewport().set_input_as_handled()


## The chosen language is a selected chip with a check mark (F096-AC-13: not colour only).
func _render_language() -> void:
	var english: bool = String(Settings.locale).begins_with("en")
	_lang_vi.set_pressed_no_signal(not english)
	_lang_en.set_pressed_no_signal(english)
	_lang_vi.icon = null if english else ICON_CHECK
	_lang_en.icon = ICON_CHECK if english else null


func _on_language_pressed(locale: String) -> void:
	Settings.set_locale(locale)
	_render_language()


func _on_locale_changed(_locale: String) -> void:
	_render_language()
	if _licenses.visible:
		_version.text = tr("ui.login.version").format({"version": Config.client_version})


## F096-AC-10: forgets that the guide was shown and opens it again on the HUD right away.
func _on_replay_guide_pressed() -> void:
	Settings.set_onboarding_hub_seen(false)
	close()
	EventBus.onboarding_requested.emit()


func _on_logout_pressed() -> void:
	_logout_confirm.call("open")
