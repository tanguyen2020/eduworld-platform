class_name ExamCountdown
extends RefCounted
## Server-clock countdown of a timed exam (F064-AC-03, F064-AC-04; GAME_NETWORKING §6, EXAM_ENGINE §6). The server
## decides the deadline (`deadline_at`, accommodations included) and when the attempt ends (ADR-0016, AGENTS R5): this
## only shows `deadline_at − server now`, where server now = the attempt's `server_time` plus the monotonic time elapsed
## since it was received (`ClientClock.now_msec()`, unaffected by a changed device clock). The shown seconds are rounded
## up, so they are never more than 1 s ahead of the server. Warnings at 5 and 1 minutes left fire once each
## (STUDENT_GAME_UX_LEARNING §9.4); `g1_3` shows a sand glass without seconds (LEARNING_EXPERIENCE §8). Pure logic.

## Minutes-left thresholds of the gentle warning, largest first (STUDENT_GAME_UX_LEARNING §9.4, F064-AC-04).
const WARNING_MINUTES: Array[int] = [5, 1]
## Bands whose clock is a sand glass without digits (STUDENT_GAME_UX_LEARNING §9.4).
const SAND_GLASS_BANDS: PackedStringArray = ["g1_3"]
const MS_PER_MINUTE: int = 60_000

## Server unix milliseconds of `deadline_at` (0 = no deadline).
var deadline_ms: int = 0
## Server unix milliseconds minus local monotonic milliseconds when the attempt was received.
var offset_ms: int = 0
## Thresholds already shown (or already past when the runner opened).
var _done: Array[int] = []


## Aligns on the attempt received at `local_ms` (monotonic): `server_time` gives the server clock; without it the
## device unix time `fallback_unix` stands in.
func sync(deadline_at: String, server_time: String, local_ms: int, fallback_unix: float) -> void:
	deadline_ms = iso_to_ms(deadline_at)
	var server_ms: int = iso_to_ms(server_time)
	if server_ms <= 0:
		server_ms = int(fallback_unix * 1000.0)
	offset_ms = server_ms - local_ms
	_done.clear()


func has_deadline() -> bool:
	return deadline_ms > 0


func server_now_ms(local_ms: int) -> int:
	return local_ms + offset_ms


func remaining_ms(local_ms: int) -> int:
	return remaining_between(deadline_ms, server_now_ms(local_ms)) if has_deadline() else -1


## Marks the thresholds already reached when the runner opens (resume): they are never shown afterwards.
func prime(local_ms: int) -> void:
	_done.clear()
	var left: int = remaining_ms(local_ms)
	for minutes: int in WARNING_MINUTES:
		if left >= 0 and left <= minutes * MS_PER_MINUTE:
			_done.append(minutes)


## Thresholds reached since the last call, each reported once. When several are crossed at once (the app was in the
## background) only the smallest is reported.
func due_warnings(local_ms: int) -> Array[int]:
	var out: Array[int] = []
	var left: int = remaining_ms(local_ms)
	if left < 0:
		return out
	var crossed: Array[int] = []
	for minutes: int in WARNING_MINUTES:
		if left <= minutes * MS_PER_MINUTE and not minutes in _done:
			_done.append(minutes)
			crossed.append(minutes)
	if not crossed.is_empty():
		out.append(crossed[crossed.size() - 1])
	return out


static func remaining_between(deadline: int, server_now: int) -> int:
	return maxi(0, deadline - server_now)


## 0 = normal, 1 = 5 minutes or less, 2 = 1 minute or less. Depends on the time left only: steady, never blinking.
static func warning_level(left_ms: int) -> int:
	var level: int = 0
	for i: int in WARNING_MINUTES.size():
		if left_ms <= WARNING_MINUTES[i] * MS_PER_MINUTE:
			level = i + 1
	return level


## Seconds shown for `left_ms` (rounded up: shown − real ∈ [0, 1) s, SC-001).
static func display_seconds(left_ms: int) -> int:
	return int(ceil(float(maxi(0, left_ms)) / 1000.0))


## `mm:ss`, or `h:mm:ss` from one hour.
static func clock_text(left_ms: int) -> String:
	var total: int = display_seconds(left_ms)
	var hours: int = total / 3600
	var minutes: int = (total % 3600) / 60
	var seconds: int = total % 60
	if hours > 0:
		return "%d:%02d:%02d" % [hours, minutes, seconds]
	return "%02d:%02d" % [minutes, seconds]


static func shows_seconds(grade_band: String) -> bool:
	return not grade_band in SAND_GLASS_BANDS


## Share of the time left (sand glass fill): 0 … 1.
static func sand_fraction(left_ms: int, total_ms: int) -> float:
	if total_ms <= 0:
		return 0.0
	return clampf(float(left_ms) / float(total_ms), 0.0, 1.0)


## RFC 3339 timestamp → unix milliseconds (fraction kept to the millisecond, `Z` or `±hh:mm` offset). 0 when invalid.
static func iso_to_ms(text: String) -> int:
	var s: String = text.strip_edges()
	if s.length() < 19 or s[4] != "-" or s[10] != "T":
		return 0
	var base: String = s.substr(0, 19)
	var rest: String = s.substr(19)
	var millis: int = 0
	if rest.begins_with("."):
		var digits: String = ""
		var i: int = 1
		while i < rest.length() and rest[i] >= "0" and rest[i] <= "9":
			digits += rest[i]
			i += 1
		millis = int((digits + "000").substr(0, 3))
		rest = rest.substr(i)
	var offset_sec: int = 0
	if rest.length() == 6 and (rest[0] == "+" or rest[0] == "-"):
		var sign: int = 1 if rest[0] == "+" else -1
		offset_sec = sign * (int(rest.substr(1, 2)) * 3600 + int(rest.substr(4, 2)) * 60)
	elif not (rest.is_empty() or rest == "Z" or rest == "z"):
		return 0
	var unix: int = Time.get_unix_time_from_datetime_string(base)
	if unix <= 0:
		return 0
	return (unix - offset_sec) * 1000 + millis
