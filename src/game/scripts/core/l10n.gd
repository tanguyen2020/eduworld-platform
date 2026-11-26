class_name L10n
extends RefCounted
## i18n helpers (GODOT_ARCHITECTURE §10.1, ADR-0056, F060-AC-12, F060-AC-15).
## - Static UI strings use gettext keys `ui.<screen>.<element>[.<state>]` (`tr()` / Control auto-translate).
## - Data strings from the backend (`localized_text`, `i18nText`) are picked with `pick()` (CONTENT_MODEL §8).
## - API error codes map to `ui.error.<CODE>`; an unknown key falls back to `ui.error.generic` and queues the
##   telemetry event `i18n.missing_key` (never shows a raw key to the student).

const ERROR_PREFIX: String = "ui.error."
const ERROR_GENERIC: String = "ui.error.generic"
const TELEMETRY_MISSING_KEY: String = "i18n.missing_key"
const FALLBACK_LANGUAGE: String = "vi"

## Problem codes the client renders with a dedicated message (GODOT_ARCHITECTURE §9). Keys listed literally so
## src/game/tools/extract_pot.gd finds them; any other code shows `ui.error.generic`.
const ERROR_KEYS: PackedStringArray = [
	"ui.error.generic",
	"ui.error.ACCOUNT_NOT_PROVISIONED",
	"ui.error.ACCOUNT_LOCKED",
	"ui.error.ACCOUNT_DEACTIVATED",
	"ui.error.NO_ACTIVE_ENROLLMENT",
	"ui.error.TENANT_ACCESS_DISABLED",
	"ui.error.ACCESS_OUTSIDE_ALLOWED_HOURS",
	"ui.error.CURFEW_ACTIVE",
	"ui.error.INVALID_CREDENTIALS",
	"ui.error.LOGIN_TICKET_INVALID",
	"ui.error.REFRESH_TOKEN_INVALID",
	"ui.error.REFRESH_TOKEN_REUSED",
	"ui.error.REFRESH_TOKEN_DEVICE_MISMATCH",
	"ui.error.CLIENT_VERSION_UNSUPPORTED",
	"ui.error.LOGIN_QUEUE_BUSY",
	"ui.error.TENANT_HEADER_REQUIRED",
	"ui.error.TENANT_NOT_SELECTED",
	"ui.error.TENANT_MISMATCH",
	"ui.error.GAME_ACCESS_STUDENT_ONLY",
	"ui.error.ACTIVE_SESSION_ON_OTHER_DEVICE",
	"ui.error.ATTEMPT_ALREADY_SUBMITTED",
	"ui.error.RATE_LIMITED",
	"ui.error.INTERNAL",
	"ui.error.UNAVAILABLE",
	"ui.error.MAINTENANCE",
	"ui.error.VALIDATION_FAILED",
	"ui.error.ASSIGNMENT_NOT_OPEN",
	"ui.error.ASSIGNMENT_MAX_ATTEMPTS",
	"ui.error.ATTEMPT_EXPIRED",
	"ui.error.ATTEMPT_NOT_IN_PROGRESS",
	"ui.error.INVALID_RESPONSE",
	"ui.error.HINTS_NOT_ALLOWED",
	"ui.error.HINTS_EXHAUSTED",
	"ui.error.ACTIVITY_TYPE_NOT_SUPPORTED",
	"ui.error.NOT_FOUND",
	"ui.error.ITEM_NOT_OWNED",
	"ui.error.EQUIP_SLOT_INVALID",
	"ui.error.NICKNAME_UNAVAILABLE",
	"ui.error.NICKNAME_CHANGE_LIMIT",
	"ui.error.EXAM_NOT_ASSIGNED",
	"ui.error.EXAM_NOT_OPEN",
	"ui.error.EXAM_WINDOW_CLOSED",
	"ui.error.EXAM_MAX_ATTEMPTS",
	"ui.error.EXAM_ACTIVE_ON_OTHER_DEVICE",
	"ui.error.EXAM_TIME_OVER",
	"ui.error.EXAM_ALREADY_SUBMITTED",
	"ui.error.EXAM_RESULTS_NOT_RELEASED",
]


## F096-AC-17: a gateway/service outage without a Problem body (ApiResult code `HTTP_<status>`) reads like
## `UNAVAILABLE` — "Hệ thống đang bận, em thử lại sau ít phút nhé." — instead of the generic message.
const CODE_ALIASES: Dictionary = {"HTTP_502": "UNAVAILABLE", "HTTP_503": "UNAVAILABLE", "HTTP_504": "UNAVAILABLE",
	"SERVICE_UNAVAILABLE": "UNAVAILABLE"}


## Supported locales (CONFIG_DEFAULTS `supported_locales`).
static func supported_locales() -> PackedStringArray:
	return ProjectSettings.get_setting("eduworld/i18n/supported_locales", PackedStringArray()) as PackedStringArray


## Locale used before bootstrap and when nothing else applies (CONFIG_DEFAULTS `tenant_default_locale`).
static func default_locale() -> String:
	return String(ProjectSettings.get_setting("eduworld/i18n/default_locale", "vi-VN"))


static func pseudo_locale() -> String:
	return String(ProjectSettings.get_setting("eduworld/i18n/pseudo_locale", "xx-PS"))


## Accepts a supported locale; the pseudo-locale is accepted only in debug builds.
static func is_supported(locale: String, debug_build: bool = OS.is_debug_build()) -> bool:
	if locale in supported_locales():
		return true
	return debug_build and locale == pseudo_locale()


## Before bootstrap: the saved device choice if supported, else the default (F060-AC-15).
## After bootstrap: the student's profile language, then the tenant default, then the platform default.
static func resolve_locale(saved: String, bootstrapped: bool = false, profile: String = "",
		tenant_default: String = "") -> String:
	var candidates: Array[String] = []
	if bootstrapped:
		candidates.append_array([profile, tenant_default])
	else:
		candidates.append(saved)
	for candidate: String in candidates:
		if not candidate.is_empty() and is_supported(candidate):
			return candidate
	return default_locale()


## Two-letter data language (`vi`/`en`) of a locale such as `vi-VN` (CONTENT_MODEL §8).
static func data_language(locale: String) -> String:
	return locale.substr(0, 2).to_lower()


## Picks the text for the current locale from `{ "vi": "...", "en": "..." }`; falls back to the content
## language, then Vietnamese, then any non-empty value.
static func pick(text: Dictionary, locale: String = "", content_language: String = "") -> String:
	var language: String = pick_language(text, locale, content_language)
	return String(text.get(language, "")) if not language.is_empty() else ""


## Language actually displayed by `pick()` (empty when the dictionary has no text).
static func pick_language(text: Dictionary, locale: String = "", content_language: String = "") -> String:
	var wanted: String = data_language(locale if not locale.is_empty() else TranslationServer.get_locale())
	for language: String in [wanted, content_language, FALLBACK_LANGUAGE]:
		if not language.is_empty() and not String(text.get(language, "")).is_empty():
			return language
	for key: Variant in text.keys():
		if not String(text[key]).is_empty():
			return String(key)
	return ""


## Translated message for an API problem code (F060-AC-12).
static func error_text(code: String) -> String:
	var key: String = ERROR_PREFIX + String(CODE_ALIASES.get(code, code))
	if has_key(key):
		return TranslationServer.translate(key)
	queue_missing_key(key)
	return TranslationServer.translate(ERROR_GENERIC)


## True when the current locale (or its fallback) has a translation for `key`.
static func has_key(key: String) -> bool:
	return TranslationServer.translate(key) != key


## Queues `i18n.missing_key` in AppState.pending_telemetry (sent by F061 through /game/telemetry).
static func queue_missing_key(key: String) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	if tree == null:
		return
	var app_state: Node = tree.root.get_node_or_null("AppState")
	if app_state != null and app_state.has_method("queue_telemetry"):
		app_state.call("queue_telemetry", TELEMETRY_MISSING_KEY, {"key": key, "locale": TranslationServer.get_locale()})
