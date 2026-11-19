extends GdUnitTestSuite
## F060-AC-17 (NFR-I18N-003, DESIGN_SYSTEM §5.5, TC-I18N-006 frame part): under the pseudo-locale every visible
## string of the frame and F061 entry screens is translated (no hard-coded text) and no text control can overflow: it wraps, or
## it ellipsises with a tooltip, and its minimum width fits the window.

const Pseudo := preload("res://tools/lib/pseudo.gd")
const SCENES: PackedStringArray = [
	"res://scenes/ui/debug/UiGallery.tscn",
	"res://scenes/bootstrap/Splash.tscn",
	"res://scenes/bootstrap/ConfigError.tscn",
	"res://scenes/bootstrap/ForceUpdate.tscn",
	"res://scenes/bootstrap/Maintenance.tscn",
	"res://scenes/auth/Login.tscn",
	"res://scenes/auth/LoginWaiting.tscn",
	"res://scenes/auth/ResumeSplash.tscn",
	"res://scenes/auth/ResumeConfirm.tscn",
	"res://scenes/auth/TenantChoice.tscn",
	"res://scenes/auth/NotStudent.tscn",
	"res://scenes/auth/AccountNotProvisioned.tscn",
	"res://scenes/auth/AccountBlocked.tscn",
	"res://scenes/auth/CurfewNotice.tscn",
	"res://scenes/ui/dialogs/TakeoverConfirm.tscn",
	"res://scenes/ui/dialogs/ReplacedNotice.tscn",
	"res://scenes/world/WorldPlaceholder.tscn",
]
const LETTER_PATTERN: String = "[A-Za-z\\x{00C0}-\\x{024F}\\x{1E00}-\\x{1EFF}]"

var _locale: String = ""
var _autostart: bool = true


func before() -> void:
	# Splash would start the F061 entry flow (network); these tests only render screens.
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	_locale = String(Settings.locale)


func after_test() -> void:
	Settings.set_locale(_locale, false)


func test_pseudo_locale_is_registered_and_expands_text() -> void:
	assert_bool(Settings.set_locale(L10n.pseudo_locale(), false)).is_true()
	var text: String = TranslationServer.translate("ui.gallery.button.primary")
	assert_str(text).starts_with(Pseudo.PREFIX)
	var en: String = "Start quest"
	assert_int(text.length()).is_greater_equal(ceili(en.length() * (1.0 + Pseudo.EXPANSION)))


func test_frame_screens_have_no_untranslated_text_nor_overflow() -> void:
	assert_bool(Settings.set_locale(L10n.pseudo_locale(), false)).is_true()
	var letter_re: RegEx = RegEx.create_from_string(LETTER_PATTERN)
	for scene_path: String in SCENES:
		var runner: GdUnitSceneRunner = scene_runner(scene_path)
		await runner.simulate_frames(3)
		var width: float = runner.scene().get_viewport().get_visible_rect().size.x
		for control: Control in _text_controls(runner.scene()):
			var raw: String = String(control.get("text"))
			var shown: String = control.tr(raw)
			if letter_re.search(shown) != null:
				assert_bool(shown.contains(Pseudo.PREFIX.strip_edges())).override_failure_message(
					"%s %s shows untranslated text '%s'" % [scene_path, control.name, shown]).is_true()
			assert_bool(_can_absorb_long_text(control)).override_failure_message(
				"%s %s neither wraps nor ellipsises with tooltip" % [scene_path, control.name]).is_true()
			assert_float(control.get_combined_minimum_size().x).override_failure_message(
				"%s %s is wider than the window" % [scene_path, control.name]).is_less_equal(width)


static func _can_absorb_long_text(control: Control) -> bool:
	if control is LineEdit:
		return true
	if control.get_parent() is FlowContainer:
		return true
	var wraps: bool = int(control.get("autowrap_mode")) != TextServer.AUTOWRAP_OFF
	var ellipsis: bool = int(control.get("text_overrun_behavior")) != TextServer.OVERRUN_NO_TRIMMING
	return wraps or (ellipsis and not control.tooltip_text.is_empty())


static func _text_controls(root: Node) -> Array[Control]:
	var out: Array[Control] = []
	for node: Node in root.find_children("*", "Control", true, false):
		var control: Control = node as Control
		if control.is_visible_in_tree() and (control is Label or control is BaseButton or control is LineEdit):
			if not String(control.get("text")).is_empty():
				out.append(control)
	return out
