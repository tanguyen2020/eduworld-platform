class_name AttemptModel
extends RefCounted
## One activity attempt (`Attempt`, ADR-0048, ADR-0059) as returned by start/get/pause/submit. Server-authoritative:
## status, deadline, feedback timing, result release and the pinned version are the server's (ADR-0016); the client
## keeps the attempt in RAM only (GODOT_ARCHITECTURE §8.1) and never stores the render payload or the answers.

const STATUS_IN_PROGRESS: String = "in_progress"
const STATUS_SUBMITTED: String = "submitted"
const STATUS_GRADED: String = "graded"
const STATUS_EXPIRED: String = "expired"
const RELEASE_IMMEDIATE: String = "immediate"
## Modes without hints (ACTIVITY_ENGINE §8.3, F063-AC-16): the hint button is hidden.
const NO_HINT_MODES: PackedStringArray = ["exam", "diagnostic", "competition"]
## `saved_responses[].hints_revealed` maximum (contract).
const HINTS_MAX: int = 5

var attempt_id: String = ""
var activity_id: String = ""
var activity_version_id: String = ""
var activity_type: String = ""
var mode: String = ""
var status: String = ""
var context_type: String = ""
var context_id: String = ""
var recommendation_id: String = ""
var started_at: String = ""
var deadline_at: String = ""
var submitted_at: String = ""
var server_time: String = ""
var paused_at: String = ""
var feedback_timing: String = ""
var result_release: String = ""
## Locale fixed for the whole attempt (`vi-VN`/`en-US`, ADR-0056): the question keeps it when the UI locale changes.
var display_language: String = ""
var items: Array[QuestionRenderModel] = []
## position → `{response: Dictionary, hints_revealed: int}` (resume, F063-AC-07).
var saved: Dictionary = {}
var render: ActivityRenderModel = null
## Media activities (F065): position to resume at and `ActivityProgressState` (`watched_ranges`, `pending_cue_ids`…).
var resume_position_ms: int = 0
var progress: Dictionary = {}
## `Attempt.evaluation` after submit (score, max_score, passed…; mini-game attempts add `game_score` and
## `completion_met`, F073). Top-level `game_score` / `completion_met` are kept here too. Display only (ADR-0016).
var evaluation: Dictionary = {}
## Local unix seconds when this attempt was received (server clock offset for the countdown).
var received_unix: float = 0.0


## Parses a decoded `Attempt`; null when `attempt_id`, `mode` or `status` is missing.
static func from_dict(data: Dictionary, now_unix: float = 0.0) -> AttemptModel:
	var id: Variant = data.get("attempt_id")
	if not id is String or not Uuid.is_valid(id as String) or not data.get("mode") is String \
			or not data.get("status") is String:
		return null
	var a: AttemptModel = AttemptModel.new()
	a.attempt_id = id as String
	a.mode = data["mode"] as String
	a.status = data["status"] as String
	for key: String in [
			"activity_id", "activity_version_id", "activity_type", "context_type", "context_id", "recommendation_id",
			"started_at", "deadline_at", "submitted_at", "server_time", "paused_at", "feedback_timing",
			"result_release", "display_language"]:
		a.set(key, _text(data, key))
	var raw_items: Variant = data.get("items", [])
	if raw_items is Array:
		for item: Variant in (raw_items as Array):
			var q: QuestionRenderModel = QuestionRenderModel.from_dict(item as Dictionary) if item is Dictionary else null
			if q != null:
				a.items.append(q)
	a.items.sort_custom(func(x: QuestionRenderModel, y: QuestionRenderModel) -> bool: return x.position < y.position)
	var raw_saved: Variant = data.get("saved_responses", [])
	if raw_saved is Array:
		for entry: Variant in (raw_saved as Array):
			if not entry is Dictionary:
				continue
			var e: Dictionary = entry as Dictionary
			var pos: Variant = e.get("position")
			if not (pos is int or pos is float):
				continue
			var response: Variant = e.get("response")
			a.saved[int(pos)] = {
				"response": response as Dictionary if response is Dictionary else {},
				"hints_revealed": clampi(int(e.get("hints_revealed", 0)), 0, HINTS_MAX),
			}
	var raw_render: Variant = data.get("render")
	if raw_render is Dictionary:
		a.render = ActivityRenderModel.from_dict(raw_render as Dictionary)
		if a.activity_type.is_empty():
			a.activity_type = a.render.activity_type
	var resume: Variant = data.get("resume_position_ms")
	a.resume_position_ms = maxi(0, int(resume)) if resume is int or resume is float else 0
	var prog: Variant = data.get("progress")
	a.progress = prog as Dictionary if prog is Dictionary else {}
	var ev: Variant = data.get("evaluation")
	a.evaluation = QuestionRenderModel.strip_answer_keys(ev) as Dictionary if ev is Dictionary else {}
	for key: String in ["game_score", "completion_met"]:
		if data.has(key) and not a.evaluation.has(key):
			a.evaluation[key] = data[key]
	a.received_unix = now_unix if now_unix > 0.0 else Time.get_unix_time_from_system()
	return a


func is_in_progress() -> bool:
	return status == STATUS_IN_PROGRESS


func is_finished() -> bool:
	return status == STATUS_SUBMITTED or status == STATUS_GRADED or status == STATUS_EXPIRED


## Results are released right after submit (`result_release = immediate` or not set by an assignment).
func results_immediate() -> bool:
	return result_release.is_empty() or result_release == RELEASE_IMMEDIATE


## Hints may be asked in this mode (the server still decides with HINTS_NOT_ALLOWED / HINTS_EXHAUSTED).
func hints_allowed() -> bool:
	return not mode in NO_HINT_MODES


func item_at(position: int) -> QuestionRenderModel:
	for q: QuestionRenderModel in items:
		if q.position == position:
			return q
	return null


func index_of(position: int) -> int:
	for i: int in items.size():
		if items[i].position == position:
			return i
	return -1


## Index of the item to show on resume: the first one without a saved answer, else the last one.
func resume_index() -> int:
	for i: int in items.size():
		if not saved.has(items[i].position):
			return i
	return maxi(0, items.size() - 1)


func saved_response(position: int) -> Dictionary:
	return (saved.get(position, {}) as Dictionary).get("response", {}) as Dictionary


func hints_revealed(position: int) -> int:
	return int((saved.get(position, {}) as Dictionary).get("hints_revealed", 0))


## Data language (`vi`/`en`) of the attempt's display language.
func content_language() -> String:
	var locale: String = display_language
	if locale.is_empty() and render != null:
		locale = render.display_language if not render.display_language.is_empty() else render.content_language
	return L10n.data_language(locale) if not locale.is_empty() else L10n.FALLBACK_LANGUAGE


func translation_missing() -> bool:
	return render != null and render.translation_missing


## Seconds left before `deadline_at` on the server clock (-1 when the attempt has no deadline).
func seconds_left(now_unix: float) -> int:
	if deadline_at.is_empty():
		return -1
	var deadline: float = Time.get_unix_time_from_datetime_string(deadline_at)
	var server_now: float = Time.get_unix_time_from_datetime_string(server_time) if not server_time.is_empty() \
			else received_unix
	return maxi(0, int(ceil(deadline - (server_now + (now_unix - received_unix)))))


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
