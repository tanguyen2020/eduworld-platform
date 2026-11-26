extends Control
## S15 Cài đặt (F114, STUDENT_GAME_UX_SOCIAL §10): a centred window over the modal scrim — blue title bar "CÀI ĐẶT"
## with a red Close button (DangerButton, icon ✕), a tab column on the left (Ngôn ngữ, Đồ họa, Âm thanh, Trợ năng,
## Thông tin; each tab icon + words) with the red "Đăng xuất" at its foot, and the open tab's page on the right
## (scrolls when long); F123 adds the tab Thông báo (the popup switch of in-game notifications). Only the client settings that Settings / QualitySettings already hold (AGENTS R1, R34):
## language chips (F062-AC-06), graphics presets and toggles (F062 T027), three volume rows with − / + (step 10%),
## text size chips, high contrast and reduce motion switches, Mây's guide again (F096-AC-10), licences (F096-AC-04) and
## the game version. Every change applies at once through Settings and is saved locally. The first opening in a session
## shows Ngôn ngữ (its bilingual title helps a student who switched language by mistake), later openings the tab used
## last. Esc / Back closes the logout confirmation first, then the licences page, then the window (F114-AC-13).
## The chosen chip is yellow with a tick (F114-AC-05: never colour only). UI only.

signal closed()

const LOCALE_VI: String = "vi-VN"
const LOCALE_EN: String = "en-US"
const TABS: PackedStringArray = ["language", "graphics", "sound", "access", "notifications", "info"]
const TAB_NODES: Dictionary = {
	"language": "TabLanguage", "graphics": "TabGraphics", "sound": "TabSound", "access": "TabAccess",
	"notifications": "TabNotifications", "info": "TabInfo",
}
const PAGE_NODES: Dictionary = {
	"language": "PageLanguage", "graphics": "PageGraphics", "sound": "PageSound", "access": "PageAccess",
	"notifications": "PageNotifications", "info": "PageInfo",
}
const FIRST_TAB: String = "language"

## The tab shown when the window opens: Ngôn ngữ first, then the tab used last in this session (the panel lives as
## long as the world screen).
var current_tab: String = FIRST_TAB
var open_tween: Tween = null

var _language_group: ButtonGroup = ButtonGroup.new()
var _font_group: ButtonGroup = ButtonGroup.new()
var _tabs: Dictionary = {}
var _pages: Dictionary = {}
var _volume_rows: Array[VolumeRow] = []

@onready var _window: PanelContainer = %Window
@onready var _close: Button = %Close
@onready var _tab_column: PanelContainer = %TabColumn
@onready var _lang_vi: Button = %LangVi
@onready var _lang_en: Button = %LangEn
@onready var _font_scales: Container = %FontScales
@onready var _high_contrast: ToggleSwitch = %HighContrast
@onready var _reduce_motion: ToggleSwitch = %ReduceMotion
## F123-AC-16: short notification toasts while playing (on by default; off = bell badge and inbox only).
@onready var _notification_popups: ToggleSwitch = %NotificationPopups
@onready var _volumes: VBoxContainer = %Volumes
@onready var _logout: Button = %Logout
@onready var _logout_confirm: Control = %LogoutConfirm
@onready var _content: ScrollContainer = %Content
@onready var _licenses: Control = %LicensesPage
@onready var _open_licenses: Button = %OpenLicenses
@onready var _replay_guide: Button = %ReplayGuide
@onready var _back: Button = %Back
@onready var _license_text: Label = %LicenseText
@onready var _version: Label = %AboutVersion
@onready var _info_version: Label = %InfoVersion


func _ready() -> void:
	_tab_column.custom_minimum_size.x = float(UiTokens.COMPONENT_SETTINGS_TAB_COLUMN_WIDTH)
	for tab: String in TABS:
		var button: Button = find_child(String(TAB_NODES[tab]), true, false) as Button
		button.pressed.connect(show_tab.bind(tab))
		_tabs[tab] = button
		_pages[tab] = find_child(String(PAGE_NODES[tab]), true, false) as Control
	_close.pressed.connect(close)
	for chip: Button in [_lang_vi, _lang_en]:
		chip.toggle_mode = true
		chip.button_group = _language_group
	_lang_vi.pressed.connect(_on_language_pressed.bind(LOCALE_VI))
	_lang_en.pressed.connect(_on_language_pressed.bind(LOCALE_EN))
	var chips: Array[Button] = font_chips()
	for level: int in chips.size():
		chips[level].toggle_mode = true
		chips[level].button_group = _font_group
		chips[level].pressed.connect(_on_font_scale_pressed.bind(level))
	_high_contrast.toggled.connect(_on_high_contrast_toggled)
	_reduce_motion.toggled.connect(_on_reduce_motion_toggled)
	_notification_popups.toggled.connect(_on_notification_popups_toggled)
	for key: String in Settings.AUDIO_KEYS:
		var row: VolumeRow = VolumeRow.new()
		row.setup(key, Settings)
		_volumes.add_child(row)
		_volume_rows.append(row)
	_logout.pressed.connect(_on_logout_pressed)
	_logout_confirm.connect("dismissed", _on_logout_dismissed)
	_open_licenses.pressed.connect(show_licenses)
	_replay_guide.pressed.connect(_on_replay_guide_pressed)
	_back.pressed.connect(show_main)
	UiBuild.size_buttons(_window)
	EventBus.locale_changed.connect(_on_locale_changed)
	Settings.font_scale_changed.connect(_on_display_setting_changed)
	Settings.high_contrast_changed.connect(_on_display_setting_changed)
	Settings.reduce_motion_changed.connect(_on_display_setting_changed)
	_render()
	show_main()
	_show_page(current_tab)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	for sig: Signal in [Settings.font_scale_changed, Settings.high_contrast_changed, Settings.reduce_motion_changed]:
		if sig.is_connected(_on_display_setting_changed):
			sig.disconnect(_on_display_setting_changed)


func open(_args: Dictionary = {}) -> void:
	show_main()
	_render()
	show()
	show_tab(current_tab)
	open_tween = UiMotion.pop_in(_window, Settings.reduce_motion)


func close() -> void:
	_logout_confirm.hide()
	hide()
	closed.emit()


## Opens a tab (F114-AC-08): the tab becomes the selected variant, its page shows, the licences page closes and the
## tab keeps the focus (Phải → into the page, Trái → back to the column).
func show_tab(tab: String) -> void:
	if not tab in TABS:
		return
	current_tab = tab
	show_main()
	_show_page(tab)
	if visible and is_inside_tree():
		# Focus for the keyboard / gamepad; the ring shows only once the student navigates (hide_focus).
		(_tabs[tab] as Button).grab_focus.call_deferred(true)


func tab_button(tab: String) -> Button:
	return _tabs.get(tab) as Button


func page(tab: String) -> Control:
	return _pages.get(tab) as Control


func font_chips() -> Array[Button]:
	var chips: Array[Button] = []
	for child: Node in _font_scales.get_children():
		chips.append(child as Button)
	return chips


func volume_row(bus_key: String) -> VolumeRow:
	for row: VolumeRow in _volume_rows:
		if row.bus_key == bus_key:
			return row
	return null


## Cài đặt → Thông tin → Giấy phép (F096-AC-04): a page inside the content area with a Back button. The licence texts
## are legal texts shown verbatim in English.
func show_licenses() -> void:
	current_tab = "info"
	_show_page(current_tab)
	_content.hide()
	_licenses.show()
	_version.text = _version_text()
	if _license_text.text.is_empty():
		_license_text.text = license_text()
	_back.grab_focus.call_deferred(true)


func show_main() -> void:
	_licenses.hide()
	_content.show()


func licenses_visible() -> bool:
	return _licenses.visible


func confirm_visible() -> bool:
	return _logout_confirm.visible


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


## Esc / Back (F114-AC-13): the logout confirmation first, then the licences page, then the window.
func _unhandled_input(event: InputEvent) -> void:
	if not visible or not event.is_action_pressed("ui_cancel"):
		return
	go_back()
	get_viewport().set_input_as_handled()


## One step back (Esc / Back / gamepad B).
func go_back() -> void:
	if _logout_confirm.visible:
		_logout_confirm.call("cancel")
	elif _licenses.visible:
		show_main()
		(_tabs[current_tab] as Button).grab_focus.call_deferred(true)
	else:
		close()


func _show_page(tab: String) -> void:
	for key: String in TABS:
		var selected: bool = key == tab
		(_pages[key] as Control).visible = selected
		(_tabs[key] as Button).theme_type_variation = &"TabButtonSelected" if selected else &"TabButton"
	_link_focus(tab)


## Phải from any tab goes to the first control of the open page; Trái from the page comes back by geometry.
func _link_focus(tab: String) -> void:
	var first: Control = _first_focusable(_pages[tab] as Control)
	for key: String in TABS:
		var button: Button = _tabs[key] as Button
		button.focus_neighbor_right = button.get_path_to(first) if first != null else NodePath()


static func _first_focusable(root: Control) -> Control:
	for node: Node in root.find_children("*", "Control", true, false):
		var control: Control = node as Control
		if control.focus_mode == Control.FOCUS_ALL and control.visible:
			return control
	return null


func _render() -> void:
	_render_language()
	_render_font_scale()
	_high_contrast.show_state(Settings.high_contrast)
	_reduce_motion.show_state(Settings.reduce_motion)
	_notification_popups.show_state(Settings.notification_popups)
	for row: VolumeRow in _volume_rows:
		row.render()
	_info_version.text = _version_text()


func _version_text() -> String:
	return tr("ui.login.version").format({"version": Config.client_version})


## The chosen language is a selected (yellow) chip with a check mark (F114-AC-05: not colour only).
func _render_language() -> void:
	var english: bool = String(Settings.locale).begins_with("en")
	_lang_vi.set_pressed_no_signal(not english)
	_lang_en.set_pressed_no_signal(english)
	UiBuild.mark_chips([_lang_vi, _lang_en])


func _render_font_scale() -> void:
	var chips: Array[Button] = font_chips()
	for level: int in chips.size():
		chips[level].set_pressed_no_signal(level == Settings.font_scale)
	UiBuild.mark_chips(chips)


func _on_language_pressed(locale: String) -> void:
	Settings.set_locale(locale)
	_render_language()


func _on_font_scale_pressed(level: int) -> void:
	Settings.set_font_scale(level)
	_render_font_scale()


func _on_high_contrast_toggled(on: bool) -> void:
	Settings.set_high_contrast(on)


func _on_reduce_motion_toggled(on: bool) -> void:
	Settings.set_reduce_motion(on)


func _on_notification_popups_toggled(on: bool) -> void:
	Settings.set_notification_popups(on)


## Text size / contrast / motion changed (here or elsewhere): ThemeService rebuilds the Theme of the window; the open
## tab stays and the controls show the saved values.
func _on_display_setting_changed(_value: Variant) -> void:
	_render_font_scale()
	_high_contrast.show_state(Settings.high_contrast)
	_reduce_motion.show_state(Settings.reduce_motion)


func _on_locale_changed(_locale: String) -> void:
	_render()
	if _licenses.visible:
		_version.text = _version_text()


## F096-AC-10: forgets that the guide was shown and opens it again on the HUD right away.
func _on_replay_guide_pressed() -> void:
	Settings.set_onboarding_hub_seen(false)
	close()
	EventBus.onboarding_requested.emit()


func _on_logout_pressed() -> void:
	_logout_confirm.call("open")


func _on_logout_dismissed() -> void:
	_logout.grab_focus.call_deferred(true)
