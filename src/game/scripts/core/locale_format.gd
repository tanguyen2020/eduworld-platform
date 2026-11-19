class_name LocaleFormat
extends RefCounted
## Locale formatting of numbers, dates, times and durations (GODOT_ARCHITECTURE §10.1, NFR-I18N-003, F060-AC-16).
## Independent of the operating system locale: separators, patterns and unit words are translation entries
## `ui.format.*` of the requested locale (vi.po / en.po), so no UI text lives in code.

const KEY_DECIMAL: String = "ui.format.decimal_separator"
const KEY_GROUP: String = "ui.format.group_separator"
const KEY_DATE: String = "ui.format.date"
const KEY_TIME: String = "ui.format.time"
const KEY_AM: String = "ui.format.am"
const KEY_PM: String = "ui.format.pm"
const KEY_DURATION_HMS: String = "ui.format.duration.hms"
const KEY_DURATION_MS: String = "ui.format.duration.ms"
const KEY_DURATION_S: String = "ui.format.duration.s"
const MONTH_KEYS: PackedStringArray = [
	"ui.format.month_short.1", "ui.format.month_short.2", "ui.format.month_short.3", "ui.format.month_short.4",
	"ui.format.month_short.5", "ui.format.month_short.6", "ui.format.month_short.7", "ui.format.month_short.8",
	"ui.format.month_short.9", "ui.format.month_short.10", "ui.format.month_short.11", "ui.format.month_short.12",
]
const SECONDS_PER_MINUTE: int = 60
const SECONDS_PER_HOUR: int = 3600
const HOURS_PER_HALF_DAY: int = 12


## Formats a number with the locale's separators. `decimals < 0` keeps the shortest exact representation.
static func number(value: float, locale: String = "", decimals: int = -1) -> String:
	var magnitude: float = absf(value)
	var text: String = String.num(magnitude, decimals)
	if decimals < 0:
		text = String.num_int64(int(magnitude)) if is_equal_approx(magnitude, floorf(magnitude)) else String.num(magnitude)
	var parts: PackedStringArray = text.split(".")
	var integer_part: String = parts[0]
	var grouped: String = ""
	var count: int = 0
	for i: int in range(integer_part.length() - 1, -1, -1):
		if count > 0 and count % 3 == 0:
			grouped = _msg(locale, KEY_GROUP) + grouped
		grouped = integer_part[i] + grouped
		count += 1
	var out: String = grouped
	if parts.size() > 1 and not parts[1].is_empty():
		out += _msg(locale, KEY_DECIMAL) + parts[1]
	if value < 0.0 and out.strip_edges() != "0":
		out = "-" + out
	return out


## `datetime` is a dictionary with `year`, `month`, `day` (as from Time.get_datetime_dict_*).
static func date(datetime: Dictionary, locale: String = "") -> String:
	var month: int = int(datetime.get("month", 1))
	return _msg(locale, KEY_DATE).format({
		"d": str(int(datetime.get("day", 1))),
		"dd": "%02d" % int(datetime.get("day", 1)),
		"mm": "%02d" % month,
		"mon": _msg(locale, MONTH_KEYS[clampi(month, 1, 12) - 1]),
		"yyyy": "%04d" % int(datetime.get("year", 1970)),
	})


## `datetime` has `hour`, `minute` (24 h). The pattern decides 24 h (`{HH}`) or 12 h (`{h}` + `{ampm}`).
static func time(datetime: Dictionary, locale: String = "") -> String:
	var hour: int = int(datetime.get("hour", 0))
	var hour12: int = hour % HOURS_PER_HALF_DAY
	if hour12 == 0:
		hour12 = HOURS_PER_HALF_DAY
	return _msg(locale, KEY_TIME).format({
		"HH": "%02d" % hour,
		"h": str(hour12),
		"MM": "%02d" % int(datetime.get("minute", 0)),
		"ampm": _msg(locale, KEY_AM if hour < HOURS_PER_HALF_DAY else KEY_PM),
	})


static func duration(total_seconds: int, locale: String = "") -> String:
	var seconds: int = maxi(total_seconds, 0)
	var h: int = seconds / SECONDS_PER_HOUR
	var m: int = (seconds % SECONDS_PER_HOUR) / SECONDS_PER_MINUTE
	var s: int = seconds % SECONDS_PER_MINUTE
	var values: Dictionary = {"h": str(h), "m": str(m), "s": str(s)}
	if h > 0:
		return _msg(locale, KEY_DURATION_HMS).format(values)
	if m > 0:
		return _msg(locale, KEY_DURATION_MS).format(values)
	return _msg(locale, KEY_DURATION_S).format(values)


## Local date/time dictionary of a unix timestamp (seconds), using the device time zone offset.
static func from_unix(unix_seconds: int, local: bool = true) -> Dictionary:
	var offset: int = 0
	if local:
		offset = int(Time.get_time_zone_from_system().get("bias", 0)) * SECONDS_PER_MINUTE
	return Time.get_datetime_dict_from_unix_time(unix_seconds + offset)


## Message of `key` in `locale` (current locale when empty). The pseudo-locale formats like en-US.
static func _msg(locale: String, key: String) -> String:
	var wanted: String = locale if not locale.is_empty() else TranslationServer.get_locale()
	var translation: Translation = TranslationServer.get_translation_object(wanted)
	if translation != null:
		var text: String = String(translation.get_message(key))
		if not text.is_empty():
			return text
	return String(TranslationServer.translate(key))
