class_name LoginFormModel
extends RefCounted
## State of the S01 login form (STUDENT_GAME_UX_CORE S01, F061-AC-02/05/06/09/12/14/30). Pure model: the Login
## scene renders it and sends intents to AuthService; it never touches the network or the password.
## Messages are chosen by Problem `code` (`ui.error.<CODE>` or a dedicated `ui.login.*` key), never `detail`.

signal changed()

## `GameLoginRequest` maxLength (specs/openapi/eduworld-api.v1.yaml, ADR-0041).
const USERNAME_MAX_LENGTH: int = 128
const PASSWORD_MAX_LENGTH: int = 256
## The "wait a few minutes" hint appears from the 3rd wrong password on this device (GODOT_ARCHITECTURE §9).
const WAIT_HINT_AFTER: int = 3

const KEY_INVALID_CREDENTIALS: String = "ui.login.invalid_credentials"
const KEY_WAIT_HINT: String = "ui.login.wait_hint"
const KEY_RATE_LIMITED: String = "ui.login.rate_limited"
const KEY_OFFLINE: String = "ui.login.offline"
const KEY_TICKET_EXPIRED: String = "ui.login.ticket_expired"
const KEY_SESSION_EXPIRED: String = "ui.login.session_expired"
const KEY_RELOGIN: String = "ui.login.relogin"
const CODE_OUTSIDE_HOURS: String = "ACCESS_OUTSIDE_ALLOWED_HOURS"

## Codes with a dedicated S01 message; anything else uses `ui.error.<CODE>` (L10n.error_text).
const CODE_KEYS: Dictionary = {
	"INVALID_CREDENTIALS": KEY_INVALID_CREDENTIALS,
	"RATE_LIMITED": KEY_RATE_LIMITED,
	"NETWORK_ERROR": KEY_OFFLINE,
	"LOGIN_TICKET_INVALID": KEY_TICKET_EXPIRED,
	"TENANT_MISMATCH": KEY_RELOGIN,
	"TENANT_HEADER_REQUIRED": KEY_RELOGIN,
	"REFRESH_TOKEN_INVALID": KEY_SESSION_EXPIRED,
	"REFRESH_TOKEN_REUSED": KEY_SESSION_EXPIRED,
	"REFRESH_TOKEN_DEVICE_MISMATCH": KEY_SESSION_EXPIRED,
}

var username: String = ""
var remember_username: bool = true
## False on a shared device: the "remember username" option is hidden and nothing is stored (F061-AC-12).
var remember_visible: bool = true
## Inline error under the password field (`code`), empty when none.
var error_code: String = ""
## Info banner above the form (`code`, e.g. REFRESH_TOKEN_INVALID → "Phiên đăng nhập đã hết…").
var banner_code: String = ""
## Values of the banner message (F062-AC-23: `{window}` = the bootstrap `access_window`).
var banner_args: Dictionary = {}
var failed_attempts: int = 0
var busy: bool = false
## Monotonic deadline of the `429 RATE_LIMITED` countdown (0 = none).
var rate_limited_until_msec: int = 0
## Incremented when the password field must be emptied (after every final login response).
var password_clear_count: int = 0


## Fills the form from the device settings (remembered username) and the device type.
func load_memory(remember: bool, last_username: String, shared_device: bool) -> void:
	remember_visible = not shared_device
	remember_username = remember and not shared_device
	username = last_username if remember_username else ""
	changed.emit()


## Username to store in `user://settings.cfg` after a successful login ("" = store nothing).
func username_to_store(shared_device: bool) -> String:
	return username if remember_username and not shared_device else ""


static func normalize_username(value: String) -> String:
	return value.strip_edges().left(USERNAME_MAX_LENGTH)


## The "Đăng nhập" button is enabled only when both fields are filled (F061-AC-02).
static func can_submit(user: String, password: String) -> bool:
	return not normalize_username(user).is_empty() and not password.is_empty() and password.length() <= PASSWORD_MAX_LENGTH


func begin_submit(user: String) -> void:
	username = normalize_username(user)
	error_code = ""
	banner_code = ""
	banner_args = {}
	busy = true
	changed.emit()


## Final response of a login/tenant:select: the password field is always emptied (F061-AC-04/05).
func end_submit(code: String) -> void:
	busy = false
	error_code = code
	if code == "INVALID_CREDENTIALS":
		failed_attempts += 1
	password_clear_count += 1
	changed.emit()


func set_rate_limited(now_msec: int, retry_after_sec: float) -> void:
	rate_limited_until_msec = now_msec + int(ceilf(retry_after_sec) * 1000.0)
	changed.emit()


## Ends an elapsed `RATE_LIMITED` countdown; true while it is still running.
func tick_rate_limit(now_msec: int) -> bool:
	if rate_limited_until_msec > 0 and now_msec >= rate_limited_until_msec:
		rate_limited_until_msec = 0
		changed.emit()
	return rate_limited_until_msec > 0


func rate_limit_left_sec(now_msec: int) -> int:
	return maxi(0, int(ceilf(float(rate_limited_until_msec - now_msec) / 1000.0)))


func show_banner(code: String, args: Dictionary = {}) -> void:
	banner_code = code
	banner_args = args
	error_code = ""
	busy = false
	password_clear_count += 1
	changed.emit()


## "Không phải tôi": the next form shows no username of the previous student (F061-AC-35).
func reset_for_new_user() -> void:
	username = ""
	error_code = ""
	banner_code = ""
	failed_attempts = 0
	busy = false
	password_clear_count += 1
	changed.emit()


func show_wait_hint() -> bool:
	return error_code == "INVALID_CREDENTIALS" and failed_attempts >= WAIT_HINT_AFTER


## S01b name (F061-AC-35): the game nickname only — never the real name (`user.display_name`),
## STUDENT_GAME_UX_CORE S01 §14, AGENTS R16. Empty when the student has no game profile yet.
static func nickname_of(bootstrap: Dictionary) -> String:
	var profile: Variant = bootstrap.get("game_profile")
	if profile is Dictionary and (profile as Dictionary).get("nickname") is String:
		return String((profile as Dictionary)["nickname"])
	return ""


## Translation key of a code ("" when the code has no dedicated key → use L10n.error_text).
static func key_for(code: String) -> String:
	return String(CODE_KEYS.get(code, ""))


## Translated message of a code in the current locale.
## Banner text: `ACCESS_OUTSIDE_ALLOWED_HOURS` with a known access window uses `ui.common.outside_access_window`
## (F062-AC-23); every other code uses `message_for`.
func banner_text() -> String:
	var window: String = String(banner_args.get("window", ""))
	if banner_code == CODE_OUTSIDE_HOURS and not window.is_empty():
		return TranslationServer.translate("ui.common.outside_access_window").format({"window": window})
	return message_for(banner_code)


static func message_for(code: String) -> String:
	if code.is_empty():
		return ""
	var key: String = key_for(code)
	return TranslationServer.translate(key) if not key.is_empty() else L10n.error_text(code)
