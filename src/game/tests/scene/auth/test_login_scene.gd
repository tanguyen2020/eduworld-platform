extends GdUnitTestSuite
## F061-AC-02 (TC-AUTH-048, TC-TEN-014, ADR-0068, STUDENT_GAME_UX_CORE S01): the S01 form is drawn by Godot and has
## only title/logo, language toggle, username, password (`secret`, show/hide), "Đăng nhập" (off while a field is
## empty), "Nhớ tên đăng nhập" and the forgot-password hint; no browser, WebView, school picker or school code.

const SCENE: String = "res://scenes/auth/Login.tscn"
const SCRIPT: String = "res://scenes/auth/login.gd"

var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart
	AuthService.form.load_memory(true, "", false)


func before_test() -> void:
	AuthService.form.load_memory(true, "", false)
	AuthService.form.end_submit("")


func test_form_has_exactly_the_s01_controls() -> void:
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	var username: LineEdit = scene.find_child("Username", true, false) as LineEdit
	var password: LineEdit = scene.find_child("Password", true, false) as LineEdit
	assert_int(username.max_length).is_equal(LoginFormModel.USERNAME_MAX_LENGTH)
	assert_int(password.max_length).is_equal(LoginFormModel.PASSWORD_MAX_LENGTH)
	assert_bool(password.secret).is_true()
	assert_bool(password.context_menu_enabled).is_false()
	assert_int(password.virtual_keyboard_type).is_equal(LineEdit.KEYBOARD_TYPE_PASSWORD)
	for name: String in ["ShowPassword", "SignIn", "Remember", "Forgot", "LangVi", "LangEn", "Version"]:
		assert_object(scene.find_child(name, true, false)).override_failure_message(name).is_not_null()
	assert_int(scene.find_children("*", "LineEdit", true, false).size()).is_equal(2)
	for node: Node in scene.find_children("*", "", true, false):
		var lower: String = String(node.name).to_lower()
		assert_bool(lower.contains("school") or lower.contains("tenant") or lower.contains("code")).override_failure_message(
				String(node.name)).is_false()
		assert_bool(node.get_class().contains("WebView")).is_false()
	var source: String = FileAccess.get_file_as_string(SCRIPT)
	for forbidden: String in ["shell_open", "OS.execute", "JavaScriptBridge", "HTTPRequest", "ApiClient", "openid"]:
		assert_str(source).override_failure_message(forbidden).not_contains(forbidden)


func test_sign_in_is_enabled_only_when_both_fields_are_filled() -> void:
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	var username: LineEdit = scene.find_child("Username", true, false) as LineEdit
	var password: LineEdit = scene.find_child("Password", true, false) as LineEdit
	var sign_in: Button = scene.find_child("SignIn", true, false) as Button
	assert_bool(sign_in.disabled).is_true()
	username.text = "hs.an.3a"
	username.text_changed.emit(username.text)
	assert_bool(sign_in.disabled).is_true()
	password.text = "secret"
	password.text_changed.emit(password.text)
	assert_bool(sign_in.disabled).is_false()
	username.text = ""
	username.text_changed.emit(username.text)
	assert_bool(sign_in.disabled).is_true()
	var show: Button = scene.find_child("ShowPassword", true, false) as Button
	show.button_pressed = true
	assert_bool(password.secret).is_false()
	show.button_pressed = false
	assert_bool(password.secret).is_true()


func test_remembered_username_is_prefilled_and_password_empty() -> void:
	AuthService.form.load_memory(true, "hs.an.3a", false)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	assert_str((scene.find_child("Username", true, false) as LineEdit).text).is_equal("hs.an.3a")
	assert_str((scene.find_child("Password", true, false) as LineEdit).text).is_empty()
	assert_bool((scene.find_child("Remember", true, false) as CheckBox).visible).is_true()


func test_shared_device_hides_remember_username() -> void:
	AuthService.form.load_memory(true, "hs.an.3a", true)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	assert_bool((scene.find_child("Remember", true, false) as CheckBox).visible).is_false()
	assert_str((scene.find_child("Username", true, false) as LineEdit).text).is_empty()


func test_error_and_banner_follow_the_form_model() -> void:
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	var password: LineEdit = scene.find_child("Password", true, false) as LineEdit
	password.text = "typed"
	AuthService.form.end_submit("INVALID_CREDENTIALS")
	await runner.simulate_frames(1)
	var error: Label = scene.find_child("Error", true, false) as Label
	assert_bool(error.visible).is_true()
	assert_str(error.text).is_equal(tr("ui.login.invalid_credentials"))
	assert_str(password.text).is_empty()
	AuthService.form.show_banner("REFRESH_TOKEN_INVALID")
	await runner.simulate_frames(1)
	var banner: Label = scene.find_child("Banner", true, false) as Label
	assert_bool(banner.visible).is_true()
	assert_str(banner.text).is_equal(tr("ui.login.session_expired"))


## Owner report 2026-11-24 ("giao diện đăng nhập của game bị vỡ"): at the base window every control keeps its own
## size (icon toggles are squares, fields are one line) and reaches the grade-band touch target; the whole form,
## even with the longest error, fits the window without scrolling or clipping (STUDENT_GAME_UX_CORE S01 §5, §8;
## DESIGN_SYSTEM §5.2).
func test_layout_fits_the_window_with_band_sized_controls() -> void:
	AuthService.form.load_memory(true, "hs.an.3a", false)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(3)
	var scene: Control = runner.scene() as Control
	var min_height: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[ThemeService.default_band()])
	var view: Rect2 = scene.get_viewport_rect().grow(0.5)
	for name: String in ["Username", "Password", "ShowPassword", "SignIn", "Remember", "Forgot", "LangVi", "LangEn"]:
		var control: Control = scene.find_child(name, true, false) as Control
		assert_bool(view.encloses(control.get_global_rect())).override_failure_message(
				"%s %s is outside the window %s" % [name, control.get_global_rect(), view]).is_true()
		assert_float(control.size.y).override_failure_message("%s is %.0f px tall" % [name, control.size.y]) \
				.is_greater_equal(min_height)
	var username: LineEdit = scene.find_child("Username", true, false) as LineEdit
	assert_float(username.size.y).is_less(min_height * 1.5)
	var show: Button = scene.find_child("ShowPassword", true, false) as Button
	assert_float(absf(show.size.x - show.size.y)).is_less_equal(1.0)
	for name: String in ["LangVi", "LangEn"]:
		var chip: Button = scene.find_child(name, true, false) as Button
		assert_float(chip.size.x).is_greater_equal(chip.size.y)
	AuthService.form.failed_attempts = LoginFormModel.WAIT_HINT_AFTER - 1
	AuthService.form.end_submit("INVALID_CREDENTIALS")
	await runner.simulate_frames(3)
	var scroll: ScrollContainer = scene.find_child("Scroll", true, false) as ScrollContainer
	var card: Control = scene.find_child("Card", true, false) as Control
	assert_bool((scene.find_child("ErrorBox", true, false) as Control).visible).is_true()
	# One spacing step of slack: line breaks differ slightly between the headless and the rendering text server.
	var room: float = scroll.size.y - float(UiTokens.SIZE_SPACE_16)
	assert_float(card.size.y).override_failure_message("card %.0f px > room %.0f px" % [card.size.y, room]) \
			.is_less_equal(room)


## Show/hide is an icon toggle with a spoken name; the selected language chip carries a check mark, so the choice is
## not told by colour alone (S01 §12, WORLD_DESIGN §11 accessibility).
func test_icon_toggles_have_names_and_non_colour_state() -> void:
	var locale: String = String(Settings.locale)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	var show: Button = scene.find_child("ShowPassword", true, false) as Button
	var hidden_icon: Texture2D = show.icon
	assert_object(hidden_icon).is_not_null()
	assert_str(show.accessibility_name).is_equal(tr("ui.login.show_password"))
	show.button_pressed = true
	assert_object(show.icon).is_not_same(hidden_icon)
	assert_str(show.accessibility_name).is_equal(tr("ui.login.hide_password"))
	assert_str(show.tooltip_text).is_equal(tr("ui.login.hide_password"))
	var vi: Button = scene.find_child("LangVi", true, false) as Button
	var en: Button = scene.find_child("LangEn", true, false) as Button
	Settings.set_locale("en-US", false)
	await runner.simulate_frames(1)
	assert_bool(en.button_pressed).is_true()
	assert_object(en.icon).is_not_null()
	assert_object(vi.icon).is_null()
	assert_str(en.accessibility_name).is_equal(tr("ui.login.language_en"))
	Settings.set_locale("vi-VN", false)
	await runner.simulate_frames(1)
	assert_bool(vi.button_pressed).is_true()
	assert_object(vi.icon).is_not_null()
	assert_object(en.icon).is_null()
	Settings.set_locale(locale, false)


## F096-AC-16 (STUDENT_GAME_UX_CORE S01 §8): "Đang vào…" can be cancelled after 10 s — the "Hủy" button is hidden
## before, appears at 10 s (touch target of the grade band), and pressing it ends the sign-in (form usable again).
func test_slow_sign_in_offers_cancel_after_ten_seconds() -> void:
	var states: Array[int] = []
	var navigator: Callable = AuthService.navigator
	AuthService.navigator = func(state: int) -> void: states.append(state)
	AuthService.form.begin_submit("hs.an.3a")
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	await runner.simulate_frames(2)
	var scene: Node = runner.scene()
	var cancel: Button = scene.find_child("CancelSignIn", true, false) as Button
	assert_bool(cancel.visible).is_false()
	scene.set("busy_since_msec", int(scene.get("busy_since_msec")) - 9000)
	await runner.simulate_frames(2)
	assert_bool(cancel.visible).override_failure_message("not before 10 s").is_false()
	scene.set("busy_since_msec", int(scene.get("busy_since_msec")) - 2000)
	await runner.simulate_frames(2)
	assert_bool(cancel.visible).is_true()
	assert_str(tr(cancel.text)).is_equal(tr("ui.login.cancel"))
	var band: String = AppState.grade_band if ThemeService.is_valid_band(AppState.grade_band) else ThemeService.default_band()
	assert_float(cancel.get_combined_minimum_size().y).is_greater_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band]))
	cancel.pressed.emit()
	await runner.simulate_frames(2)
	assert_bool(AuthService.form.busy).is_false()
	assert_bool(AuthService.holds_password()).is_false()
	assert_bool(cancel.visible).is_false()
	assert_array(states).contains([GameStateMachine.State.LOGIN])
	AuthService.navigator = navigator
