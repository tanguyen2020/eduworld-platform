class_name MyAssignmentModel
extends RefCounted
## One school assignment of the signed-in student (`MyAssignment`, specs/openapi/eduworld-api.v1.yaml, F062-AC-08).
## Read-only view of server data kept in RAM (`AppState.assignments`, GODOT_ARCHITECTURE §8.2): the status, the
## deadline and whether the work may still be opened are decided by the server (ADR-0016); the client only shows them.
## Unknown fields are ignored; an unknown `status`/`content_kind` is kept as text and shown as "not supported".

const STATUSES: PackedStringArray = ["not_started", "in_progress", "submitted", "graded", "overdue", "excused"]
const KINDS: PackedStringArray = ["activity", "question_set", "assessment", "quest", "lesson"]
const KIND_ACTIVITY: String = "activity"
const KIND_ASSESSMENT: String = "assessment"
const STATUS_OVERDUE: String = "overdue"
## `MyAssignment.title` maxLength (specs/openapi).
const TITLE_MAX_LENGTH: int = 255

var assignment_id: String = ""
## Shown verbatim (the school typed it; never translated, NFR-I18N-003).
var title: String = ""
var subject_id: String = ""
var content_kind: String = ""
var content_ref: String = ""
var exam_session_id: String = ""
var open_at: String = ""
var due_at: String = ""
var status: String = ""
var attempt_id: String = ""
## `attempts_left` (null in the contract = unlimited → -1).
var attempts_left: int = -1
var result_release: String = ""
var allow_late: bool = false
var late_window_hours: int = 0


## Parses one decoded `MyAssignment`; null when a required field (`assignment_id`, `title`, `status`) is missing or
## has the wrong shape.
static func from_dict(data: Dictionary) -> MyAssignmentModel:
	var id: Variant = data.get("assignment_id")
	var raw_title: Variant = data.get("title")
	var raw_status: Variant = data.get("status")
	if not id is String or not Uuid.is_valid(id as String) or not raw_title is String or not raw_status is String:
		return null
	var m: MyAssignmentModel = MyAssignmentModel.new()
	m.assignment_id = id as String
	m.title = (raw_title as String).left(TITLE_MAX_LENGTH)
	m.status = raw_status as String
	m.subject_id = _uuid(data, "subject_id")
	m.content_kind = _text(data, "content_kind")
	m.content_ref = _uuid(data, "content_ref")
	m.exam_session_id = _uuid(data, "exam_session_id")
	m.attempt_id = _uuid(data, "attempt_id")
	m.open_at = _text(data, "open_at")
	m.due_at = _text(data, "due_at")
	m.result_release = _text(data, "result_release")
	var left: Variant = data.get("attempts_left")
	if left is int or left is float:
		m.attempts_left = maxi(0, int(left))
	var late: Variant = data.get("late_policy")
	if late is Dictionary:
		m.allow_late = bool((late as Dictionary).get("allow_late", false))
		m.late_window_hours = clampi(int((late as Dictionary).get("late_window_hours", 0)), 0, 168)
	return m


## Parses the `data` array of `listMyAssignments`; malformed items are skipped.
static func list_from(items: Array) -> Array[MyAssignmentModel]:
	var out: Array[MyAssignmentModel] = []
	for item: Variant in items:
		var m: MyAssignmentModel = from_dict(item as Dictionary) if item is Dictionary else null
		if m != null:
			out.append(m)
	return out


func is_known_status() -> bool:
	return status in STATUSES


func is_known_kind() -> bool:
	return content_kind in KINDS


## Unix seconds of `due_at` (0 when absent or not a timestamp).
func due_unix() -> int:
	if due_at.is_empty():
		return 0
	return int(Time.get_unix_time_from_datetime_string(due_at))


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""


static func _uuid(data: Dictionary, key: String) -> String:
	var v: String = _text(data, key)
	return v if Uuid.is_valid(v) else ""
