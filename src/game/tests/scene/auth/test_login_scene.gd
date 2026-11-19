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
