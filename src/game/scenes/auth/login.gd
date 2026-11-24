extends Control
## S01 login form (STUDENT_GAME_UX_CORE S01, F061-AC-02/05/06/09/12/30): a Godot form only — logo, language toggle,
## username, password (`secret`, show/hide icon toggle), "Đăng nhập" (off while a field is empty), "Nhớ tên đăng
## nhập", "Quên mật khẩu? Liên hệ nhà trường để đặt lại" hint, and the guide Mây greeting the student. No browser, no
## school picker. Renders `AuthService.form` and sends intents to AuthService (AGENTS R1); the password field is
## emptied after every final response.
## Layout: three columns inside the screen margin — brand + Mây (left), the form card (centre, scrolls only when the
## window is shorter than the card), language (right). Widths come from UiTokens (`component.form_card.*`), styles
## from the generated Theme (R32); the stretch mode keeps the logical width ≥ 1280 px so the columns always fit.

const LOCALE_VI: String = "vi-VN"
const LOCALE_EN: String = "en-US"
const ICON_EYE: Texture2D = preload("res://resources/ui/icons/eye.svg")
const ICON_EYE_OFF: Texture2D = preload("res://resources/ui/icons/eye_off.svg")
const ICON_CHECK: Texture2D = preload("res://resources/ui/icons/check.svg")

var _seen_clear_count: int = -1
var _language_group: ButtonGroup = ButtonGroup.new()

@onready var _username: LineEdit = %Username
@onready var _password: LineEdit = %Password
@onready var _show_password: Button = %ShowPassword
@onready var _sign_in: Button = %SignIn
@onready var _remember: CheckBox = %Remember
@onready var _forgot: Button = %Forgot
@onready var _forgot_box: Control = %ForgotBox
@onready var _banner_box: Control = %BannerBox
@onready var _banner: Label = %Banner
@onready var _error_box: Control = %ErrorBox
@onready var _error: Label = %Error
@onready var _version: Label = %Version
@onready var _lang_vi: Button = %LangVi
@onready var _lang_en: Button = %LangEn
@onready var _lang_icon: TextureRect = %LangIcon
@onready var _spinner: Control = %Spinner
@onready var _card: Control = %Card
@onready var _brand_column: Control = %BrandColumn
@onready var _lang_column: Control = %LangColumn
@onready var _mascot: Control = %Mascot


func _ready() -> void:
	_apply_layout_tokens()
	_username.max_length = LoginFormModel.USERNAME_MAX_LENGTH
	_password.max_length = LoginFormModel.PASSWORD_MAX_LENGTH
	_password.secret = true
	_username.text = AuthService.form.username
	_password.text = ""
	_seen_clear_count = AuthService.form.password_clear_count
	_lang_vi.button_group = _language_group
	_lang_en.button_group = _language_group
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
	_render_static_labels()
	_brand_column.minimum_size_changed.connect(_fit_brand_column, CONNECT_DEFERRED)
	resized.connect(_fit_brand_column, CONNECT_DEFERRED)
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


## Column and icon sizes from the design tokens (no magic numbers in the scene, R32).
func _apply_layout_tokens() -> void:
	_card.custom_minimum_size.x = UiTokens.COMPONENT_FORM_CARD_WIDTH
	_brand_column.custom_minimum_size.x = UiTokens.COMPONENT_FORM_CARD_SIDE_WIDTH
	_lang_column.custom_minimum_size.x = UiTokens.COMPONENT_FORM_CARD_SIDE_WIDTH
	var icon: Vector2 = Vector2(UiTokens.COMPONENT_ICON_SIZE, UiTokens.COMPONENT_ICON_SIZE)
	for rect: TextureRect in [_lang_icon, %BannerBox.find_child("Icon") as TextureRect,
			%ErrorBox.find_child("Icon") as TextureRect]:
		rect.custom_minimum_size = icon
	# The spinner sits at the start of the sign-in button, vertically centred.
	var half: float = float(UiTokens.COMPONENT_ICON_SIZE) / 2.0
	_spinner.offset_left = UiTokens.SIZE_SPACE_16
	_spinner.offset_right = UiTokens.SIZE_SPACE_16 + UiTokens.COMPONENT_ICON_SIZE
	_spinner.offset_top = -half
	_spinner.offset_bottom = half
	# The show/hide toggle is a square touch target as tall as the field.
	_show_password.custom_minimum_size.x = _show_password.get_combined_minimum_size().y


## Mây steps aside when the brand column (logo, bubble, Mây, version) is taller than the window — long translations,
## larger font scale — so nothing in the column is cut off (S01 §5 hides the illustration when space is short).
func _fit_brand_column() -> void:
	var mascot_height: float = _mascot.get_combined_minimum_size().y \
			+ _mascot.get_parent_control().get_theme_constant(&"separation")
	var without_mascot: float = _brand_column.get_combined_minimum_size().y - (mascot_height if _mascot.visible else 0.0)
	# The room is the window inside the screen margin (the containers themselves grow with their content).
	var safe: Control = $Safe
	var room: float = size.y - safe.get_theme_constant(&"margin_top") - safe.get_theme_constant(&"margin_bottom")
	var fits: bool = without_mascot + mascot_height <= room
	if _mascot.visible != fits:
		_mascot.visible = fits


## Language chips: wide enough for the check icon + label (they ellipsise with a tooltip instead of wrapping), at
## least square, and both fit the language column beside the globe.
func _fit_language_chips() -> void:
	var gap: int = _lang_icon.get_parent_control().get_theme_constant(&"separation")
	var room: float = (float(UiTokens.COMPONENT_FORM_CARD_SIDE_WIDTH) - _lang_icon.custom_minimum_size.x - gap * 2.0) / 2.0
	for chip: Button in [_lang_vi, _lang_en]:
		var font: Font = chip.get_theme_font(&"font")
		var text_width: float = font.get_string_size(tr(chip.text), HORIZONTAL_ALIGNMENT_LEFT, -1.0,
				chip.get_theme_font_size(&"font_size")).x
		var natural: float = text_width + chip.get_theme_constant(&"icon_max_width") \
				+ chip.get_theme_constant(&"h_separation") + chip.get_theme_stylebox(&"normal").get_minimum_size().x
		var height: float = chip.get_theme_stylebox(&"normal").get_minimum_size().y + chip.get_theme_constant(&"icon_max_width")
		chip.custom_minimum_size.x = clampf(natural, height, maxf(room, height))


func _render() -> void:
	var form: LoginFormModel = AuthService.form
	if form.password_clear_count != _seen_clear_count:
		_seen_clear_count = form.password_clear_count
		_password.clear()
		if not form.busy and is_inside_tree():
			_password.grab_focus.call_deferred()
	_banner.visible = not form.banner_code.is_empty()
	_banner.text = form.banner_text()
	_banner_box.visible = _banner.visible
	var message: String = LoginFormModel.message_for(form.error_code)
	var left: int = form.rate_limit_left_sec(AuthService.clock.now_msec()) if AuthService.clock != null else 0
	if form.error_code == "RATE_LIMITED":
		message = tr("ui.login.rate_limited").format({"seconds": left})
	if form.show_wait_hint():
		message += "\n" + tr("ui.login.wait_hint")
	_error.visible = not message.is_empty()
	_error.text = message
	_error_box.visible = _error.visible
	_remember.visible = form.remember_visible
	_remember.set_pressed_no_signal(form.remember_username)
	_sign_in.text = tr("ui.login.button_sign_in")
	if form.busy:
		var creating: bool = AuthService.state == GameStateMachine.State.CREATING_SESSION
		_sign_in.text = tr("ui.login.resolving_tenant") if creating else tr("ui.login.signing_in")
	_spinner.visible = form.busy
	_username.editable = not form.busy
	_password.editable = not form.busy
	_sign_in.disabled = form.busy or left > 0 or not LoginFormModel.can_submit(_username.text, _password.text)
	_version.text = tr("ui.login.version").format({"version": Config.client_version})


## Texts that are not translation keys on a node: tooltips, screen-reader names, the selected language chip.
func _render_static_labels() -> void:
	_render_show_password(_show_password.button_pressed)
	var selected_en: bool = String(Settings.locale).begins_with("en")
	_lang_vi.set_pressed_no_signal(not selected_en)
	_lang_en.set_pressed_no_signal(selected_en)
	# The selected chip carries a check mark too: the choice is not shown by colour alone.
	_lang_vi.icon = null if selected_en else ICON_CHECK
	_lang_en.icon = ICON_CHECK if selected_en else null
	_lang_vi.tooltip_text = tr("ui.login.language_vi")
	_lang_en.tooltip_text = tr("ui.login.language_en")
	_lang_vi.accessibility_name = tr("ui.login.language_vi")
	_lang_en.accessibility_name = tr("ui.login.language_en")
	_lang_icon.tooltip_text = tr("ui.login.language_label")
	_fit_language_chips()
	_fit_brand_column.call_deferred()


func _render_show_password(shown: bool) -> void:
	var label: String = tr("ui.login.hide_password") if shown else tr("ui.login.show_password")
	_show_password.icon = ICON_EYE_OFF if shown else ICON_EYE
	_show_password.tooltip_text = label
	_show_password.accessibility_name = label


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
	_render_show_password(shown)


func _on_remember_toggled(enabled: bool) -> void:
	AuthService.set_remember_username(enabled)


func _on_forgot_pressed() -> void:
	_forgot_box.visible = not _forgot_box.visible


func _on_language_pressed(locale: String) -> void:
	Settings.set_locale(locale)


func _on_locale_changed(_locale: String) -> void:
	_render()
	_render_static_labels()
