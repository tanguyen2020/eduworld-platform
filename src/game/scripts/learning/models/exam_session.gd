class_name ExamSessionModel
extends RefCounted
## One exam session the student may take (`ExamSession`, specs/openapi/eduworld-api.v1.yaml, F042; F064-AC-01). Read-only
## view of server data kept in RAM (GODOT_ARCHITECTURE §8.2): the window, status, duration (accommodations included in
## `effective_duration_seconds`), navigation and result release are the server's (ADR-0016); the client only shows them
## on the rules screen. Unknown fields are ignored.

const NAV_FREE: String = "free"
const NAV_SEQUENTIAL: String = "sequential_no_return"
const RELEASE_IMMEDIATE: String = "immediate"
const RELEASE_AFTER_CLOSE: String = "after_close"
const RELEASE_MANUAL: String = "manual"
## `ExamSession.title` maxLength (specs/openapi).
const TITLE_MAX_LENGTH: int = 255

var exam_session_id: String = ""
## Shown verbatim (typed by the school; never translated, NFR-I18N-003).
var title: String = ""
var type: String = ""
var is_official: bool = false
var subject_id: String = ""
var window_start: String = ""
var window_end: String = ""
var duration_seconds: int = 0
## The student's own duration after extra time (0 = not sent).
var effective_duration_seconds: int = 0
var question_count: int = 0
var max_attempts: int = 0
var attempts_used: int = 0
var status: String = ""
## `free` (default of the blueprint) or `sequential_no_return`.
var navigation: String = NAV_FREE
var feedback_timing: String = ""
var result_release: String = ""
var assignment_id: String = ""
var server_time: String = ""


## Parses a decoded `ExamSession`; null when `exam_session_id` is missing or not a UUID.
static func from_dict(data: Dictionary) -> ExamSessionModel:
	var id: Variant = data.get("exam_session_id")
	if not id is String or not Uuid.is_valid(id as String):
		return null
	var s: ExamSessionModel = ExamSessionModel.new()
	s.exam_session_id = id as String
	s.title = _text(data, "title").left(TITLE_MAX_LENGTH)
	for key: String in ["type", "subject_id", "window_start", "window_end", "status", "feedback_timing", "result_release",
			"assignment_id", "server_time"]:
		s.set(key, _text(data, key))
	s.is_official = data.get("is_official") == true
	for key: String in ["duration_seconds", "effective_duration_seconds", "question_count", "max_attempts", "attempts_used"]:
		var v: Variant = data.get(key)
		s.set(key, maxi(0, int(v)) if v is int or v is float else 0)
	s.navigation = NAV_SEQUENTIAL if _text(data, "navigation") == NAV_SEQUENTIAL else NAV_FREE
	return s


## Parses the `data` array of `listExamSessions`; malformed items are skipped.
static func list_from(items: Array) -> Array[ExamSessionModel]:
	var out: Array[ExamSessionModel] = []
	for item: Variant in items:
		var s: ExamSessionModel = from_dict(item as Dictionary) if item is Dictionary else null
		if s != null:
			out.append(s)
	return out


## Seconds the student has (accommodations already applied by the server).
func duration_for_student() -> int:
	return effective_duration_seconds if effective_duration_seconds > 0 else duration_seconds


## Whole minutes shown on the rules screen (rounded up).
func duration_minutes() -> int:
	return int(ceil(float(duration_for_student()) / 60.0))


## `sequential_no_return`: no "Câu trước", the question grid only goes forward (F064-AC-06).
func forward_only() -> bool:
	return navigation == NAV_SEQUENTIAL


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
