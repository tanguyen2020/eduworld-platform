class_name CurfewWatcher
extends RefCounted
## Client side of the curfew (ADR-0078, CHILD_SAFETY §4.1, F061-AC-36): reads the effective window
## `bootstrap.policies.play_time.curfew {start, end, warning_minutes}` (ICT) and the server clock
## (`bootstrap.server_time`) to warn `warning_minutes` before the start and to report when it begins. Display
## only — the server still enforces the curfew (`403 CURFEW_ACTIVE`, close 4410).

enum State { NONE, WARNING, CURFEW }

## Curfew times are ICT (Asia/Ho_Chi_Minh, UTC+7, no DST).
const ICT_OFFSET_SEC: int = 7 * 3600
const MINUTES_PER_DAY: int = 24 * 60

var enabled: bool = false
var start_minute: int = 0
var end_minute: int = 0
var warning_minutes: int = 0
## server unix − local unix at bootstrap.
var server_offset_sec: float = 0.0


func configure(bootstrap: Dictionary, local_unix: float) -> void:
	enabled = false
	var policies: Variant = bootstrap.get("policies", {})
	var play_time: Variant = (policies as Dictionary).get("play_time", {}) if policies is Dictionary else {}
	var curfew: Variant = (play_time as Dictionary).get("curfew") if play_time is Dictionary else null
	if not curfew is Dictionary:
		return
	var c: Dictionary = curfew as Dictionary
	var start: int = parse_hhmm(String(c.get("start", "")))
	var end: int = parse_hhmm(String(c.get("end", "")))
	if start < 0 or end < 0:
		return
	start_minute = start
	end_minute = end
	warning_minutes = maxi(0, int(c.get("warning_minutes", 0)))
	var server_unix: float = parse_iso_unix(String(bootstrap.get("server_time", "")))
	server_offset_sec = server_unix - local_unix if server_unix > 0.0 else 0.0
	enabled = true


func disable() -> void:
	enabled = false


func state_at(local_unix: float) -> State:
	if not enabled:
		return State.NONE
	var minute: int = ict_minute(local_unix + server_offset_sec)
	if in_window(minute, start_minute, end_minute):
		return State.CURFEW
	if warning_minutes > 0 and minutes_until_start(local_unix) <= warning_minutes:
		return State.WARNING
	return State.NONE


func minutes_until_start(local_unix: float) -> int:
	var minute: int = ict_minute(local_unix + server_offset_sec)
	return posmod(start_minute - minute, MINUTES_PER_DAY)


## End of the window as "HH:MM" in the current locale (reopen time).
func reopen_text() -> String:
	return LocaleFormat.time({"hour": end_minute / 60, "minute": end_minute % 60})


static func ict_minute(unix_seconds: float) -> int:
	var seconds: int = posmod(int(floor(unix_seconds)) + ICT_OFFSET_SEC, MINUTES_PER_DAY * 60)
	return seconds / 60


## `start` inclusive, `end` exclusive; a window may wrap past midnight (21:30–06:30).
static func in_window(minute: int, start: int, end: int) -> bool:
	if start == end:
		return false
	if start < end:
		return minute >= start and minute < end
	return minute >= start or minute < end


static func parse_hhmm(value: String) -> int:
	var parts: PackedStringArray = value.split(":")
	if parts.size() != 2 or not parts[0].is_valid_int() or not parts[1].is_valid_int():
		return -1
	var h: int = int(parts[0])
	var m: int = int(parts[1])
	if h < 0 or h > 23 or m < 0 or m > 59:
		return -1
	return h * 60 + m


## Unix seconds of an RFC 3339 timestamp (`Z` or `±hh:mm`); 0 when it cannot be parsed.
static func parse_iso_unix(value: String) -> float:
	var text: String = value.strip_edges()
	if text.length() < 19:
		return 0.0
	var offset_sec: int = 0
	var tail: String = text.substr(19)
	var zone_at: int = maxi(tail.find("+"), tail.find("-"))
	if zone_at >= 0:
		var zone: PackedStringArray = tail.substr(zone_at + 1).split(":")
		if zone.size() == 2 and zone[0].is_valid_int() and zone[1].is_valid_int():
			offset_sec = (int(zone[0]) * 3600 + int(zone[1]) * 60) * (1 if tail[zone_at] == "+" else -1)
	var unix: int = Time.get_unix_time_from_datetime_string(text.substr(0, 19))
	return float(unix - offset_sec)


## Reopen time "HH:MM" (ICT) of a `Problem.reopen_at` timestamp; empty when it cannot be parsed.
static func reopen_text_of(reopen_at: String) -> String:
	var unix: float = parse_iso_unix(reopen_at)
	if unix <= 0.0:
		return ""
	var minute: int = ict_minute(unix)
	return LocaleFormat.time({"hour": minute / 60, "minute": minute % 60})
