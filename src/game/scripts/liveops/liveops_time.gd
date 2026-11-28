class_name LiveOpsTime
extends RefCounted
## Time helpers of the Event center (F111 data-model §3, EVENT_SEASON_SYSTEM §9, FR-EVT-007, NFR-I18N-003): the server
## `Timestamp` (RFC 3339) as unix seconds, the time left as WHOLE DAYS only (rounded up — never hours, minutes or
## seconds, no countdown), and dates / reset times in Vietnam time (ICT, UTC+7) formatted by the locale (LocaleFormat).
## "Now" is the server-corrected time the caller passes (ApiClient clock), never a local ticking timer.

const DAY_SEC: int = 86400
## ICT = UTC+7 (CONFIG_DEFAULTS `quest_reset_time` is given in ICT).
const ICT_OFFSET_SEC: int = 7 * 3600


## Unix seconds of an RFC 3339 timestamp ("2026-11-15T07:30:00Z", "+07:00" offsets, fractions); 0 when unreadable.
static func unix_of(text: String) -> int:
	var t: String = text.strip_edges()
	if t.length() < 10:
		return 0
	var offset: int = 0
	if t.ends_with("Z") or t.ends_with("z"):
		t = t.left(t.length() - 1)
	elif t.length() > 19 and (t[t.length() - 6] == "+" or t[t.length() - 6] == "-"):
		var dir: int = 1 if t[t.length() - 6] == "+" else -1
		var hh: int = t.substr(t.length() - 5, 2).to_int()
		var mm: int = t.substr(t.length() - 2, 2).to_int()
		offset = dir * (hh * 3600 + mm * 60)
		t = t.left(t.length() - 6)
	var dot: int = t.find(".")
	if dot >= 0:
		t = t.left(dot)
	return maxi(0, int(Time.get_unix_time_from_datetime_string(t)) - offset)


## Whole days left until `ends_unix` (rounded up); 0 once it passed.
static func days_left(ends_unix: int, now_unix: int) -> int:
	if ends_unix <= now_unix:
		return 0
	return ceili(float(ends_unix - now_unix) / float(DAY_SEC))


## Date of `unix` in ICT, formatted by the current locale ("29/11/2026" / "Nov 29, 2026").
static func ict_date(unix: int) -> String:
	return LocaleFormat.date(Time.get_datetime_dict_from_unix_time(unix + ICT_OFFSET_SEC))


## Time of day of `unix` in ICT ("00:00" / "12:00 AM").
static func ict_time(unix: int) -> String:
	return LocaleFormat.time(Time.get_datetime_dict_from_unix_time(unix + ICT_OFFSET_SEC))
