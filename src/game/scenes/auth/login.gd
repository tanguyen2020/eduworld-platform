extends Control
## S01 login form (STUDENT_GAME_UX_CORE S01, F061-AC-02/05/06/09/12/30): a Godot form only — logo/title,
## language toggle, username, password (`secret`, show/hide), "Đăng nhập" (off while a field is empty),
## "Nhớ tên đăng nhập", "Quên mật khẩu? Liên hệ nhà trường để đặt lại" hint. No browser, no school picker.
## Renders `AuthService.form` and sends intents to AuthService (AGENTS R1); the password field is emptied after
## every final response.

const LOCALE_VI: String = "vi-VN"
const LOCALE_EN: String = "en-US"

var _seen_clear_count: int = -1

@onready var _username: LineEdit = %Username
@onready var _password: LineEdit = %Password
@onready var _show_password: Button = %ShowPassword
@onready var _sign_in: Button = %SignIn
@onready var _remember: CheckBox = %Remember
@onready var _forgot: Button = %Forgot
@onready var _forgot_hint: Label = %ForgotHint
@onready var _banner: Label = %Banner
@onready var _error: Label = %Error
@onready var _version: Label = %Version
@onready var _lang_vi: Button = %LangVi
@onready var _lang_en: Button = %LangEn


func _ready() -> void:
	_username.max_length = LoginFormModel.USERNAME_MAX_LENGTH
	_password.max_length = LoginFormModel.PASSWORD_MAX_LENGTH
	_password.secret = true
	_username.text = AuthService.form.username
	_password.text = ""
	_seen_clear_count = AuthService.form.password_clear_count
	_username.text_changed.connect(_on_text_changed)
	_password.text_changed.connect(_on_text_changed)
	_username.text_submitted.connect(_on_username_submitted)
	_password.text_submitted.connect(_on_password_submitted)
	_show_password.toggled.connect(_on_show_password_toggled)
	_sign_in.pressed.connect(_on_sign_in_pressed)
	_remember.toggled.connect(_on_remember_toggled)
	_forgot.pressed.connect(_on_forgot_pressed)
	_lang_vi.pressed.connect(_on_language_pressed.bind(LOCALE_VI))
	_lang_en.pressed.connect(_on_language_pressed.bind(LOCALE_EN))
	AuthService.form.changed.connect(_render)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()
	_focus_first_field.call_deferred()


func _exit_tree() -> void:
	if AuthService.form.changed.is_connected(_render):
		AuthService.form.changed.disconnect(_render)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func _process(_delta: float) -> void:
	if AuthService.form.rate_limited_until_msec > 0 and AuthService.clock != null:
		if AuthService.form.tick_rate_limit(AuthService.clock.now_msec()):
			_render()


func _render() -> void:
	var form: LoginFormModel = AuthService.form
	if form.password_clear_count != _seen_clear_count:
		_seen_clear_count = form.password_clear_count
		_password.clear()
		if not form.busy and is_inside_tree():
			_password.grab_focus.call_deferred()
	_banner.visible = not form.banner_code.is_empty()
	_banner.text = LoginFormModel.message_for(form.banner_code)
	var message: String = LoginFormModel.message_for(form.error_code)
	var left: int = form.rate_limit_left_sec(AuthService.clock.now_msec()) if AuthService.clock != null else 0
	if form.error_code == "RATE_LIMITED":
		message = tr("ui.login.rate_limited").format({"seconds": left})
	if form.show_wait_hint():
		message += "\n" + tr("ui.login.wait_hint")
	_error.visible = not message.is_empty()
	_error.text = message
	_remember.visible = form.remember_visible
	_remember.set_pressed_no_signal(form.remember_username)
	_sign_in.text = tr("ui.login.button_sign_in")
	if form.busy:
		var creating: bool = AuthService.state == GameStateMachine.State.CREATING_SESSION
		_sign_in.text = tr("ui.login.resolving_tenant") if creating else tr("ui.login.signing_in")
	_username.editable = not form.busy
	_password.editable = not form.busy
	_sign_in.disabled = form.busy or left > 0 or not LoginFormModel.can_submit(_username.text, _password.text)
	_version.text = tr("ui.login.version").format({"version": Config.client_version})


func _focus_first_field() -> void:
	if _username.text.is_empty():
		_username.grab_focus()
	else:
		_password.grab_focus()


func _on_text_changed(_text: String) -> void:
	_render()


func _on_username_submitted(_text: String) -> void:
	_password.grab_focus()


func _on_password_submitted(_text: String) -> void:
	if not _sign_in.disabled:
		_on_sign_in_pressed()


func _on_sign_in_pressed() -> void:
	AuthService.login(_username.text, _password.text)


func _on_show_password_toggled(shown: bool) -> void:
	_password.secret = not shown
	_show_password.text = tr("ui.login.hide_password") if shown else tr("ui.login.show_password")


func _on_remember_toggled(enabled: bool) -> void:
	AuthService.set_remember_username(enabled)


func _on_forgot_pressed() -> void:
	_forgot_hint.visible = not _forgot_hint.visible


func _on_language_pressed(locale: String) -> void:
	Settings.set_locale(locale)


func _on_locale_changed(_locale: String) -> void:
	_render()
