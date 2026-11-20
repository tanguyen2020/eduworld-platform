class_name QuestionRenderModel
extends RefCounted
## One question item of an attempt (`QuestionRender`, specs/openapi; render part of specs/schemas/question.schema.json
## plus `stem`). Kept in RAM only for the attempt (GODOT_ARCHITECTURE §8.1). The render payload never carries an
## answer key, scoring rule, explanation, per-option feedback or hints (ADR-0016, AGENTS R6): should one appear by a
## server bug, `from_dict` drops it so no renderer can read or show it (F063-AC-03).

const TYPES: PackedStringArray = [
	"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
	"ordering", "drag_drop", "image_based", "interactive",
]
## Answer-side keys that must never reach a renderer (QuestionRender.render_payload `not` list in the contract).
const FORBIDDEN_KEYS: PackedStringArray = [
	"answer_key", "scoring_rule", "accepted_answers", "correct_option_id", "correct_option_ids", "explanation",
	"feedback", "option_feedback", "hints", "misconception_tag",
]
## `hint_count` maximum (contract) and the value used when the server did not send it.
const HINT_COUNT_MAX: int = 5
const HINT_COUNT_UNKNOWN: int = -1

var position: int = 0
var question_version_id: String = ""
var type: String = ""
var render_payload: Dictionary = {}
## `media[]`: `{asset_id, url, kind}`.
var media: Array[Dictionary] = []
var max_score: float = 0.0
## Number of hint levels (`QuestionRender.hint_count`, F063 T004); HINT_COUNT_UNKNOWN when absent.
var hint_count: int = HINT_COUNT_UNKNOWN


## Parses one decoded `QuestionRender`; null when `position`, `question_version_id`, `type` or `render_payload` is
## missing or has the wrong shape. An unknown `type` is kept (the player shows "update required" for that item).
static func from_dict(data: Dictionary) -> QuestionRenderModel:
	var pos: Variant = data.get("position")
	var version_id: Variant = data.get("question_version_id")
	var raw_type: Variant = data.get("type")
	var payload: Variant = data.get("render_payload")
	if not (pos is int or pos is float) or int(pos) < 1 or not version_id is String or not raw_type is String \
			or not payload is Dictionary:
		return null
	var q: QuestionRenderModel = QuestionRenderModel.new()
	q.position = int(pos)
	q.question_version_id = version_id as String
	q.type = raw_type as String
	q.render_payload = strip_answer_keys(payload) as Dictionary
	var raw_media: Variant = data.get("media", [])
	if raw_media is Array:
		for item: Variant in (raw_media as Array):
			if item is Dictionary and (item as Dictionary).get("url") is String:
				q.media.append(item as Dictionary)
	var score: Variant = data.get("max_score")
	if score is int or score is float:
		q.max_score = float(score)
	var hints: Variant = data.get("hint_count")
	if hints is int or hints is float:
		q.hint_count = clampi(int(hints), 0, HINT_COUNT_MAX)
	return q


## Deep copy of `value` without the FORBIDDEN_KEYS at any depth.
static func strip_answer_keys(value: Variant) -> Variant:
	if value is Dictionary:
		var out: Dictionary = {}
		for key: Variant in (value as Dictionary).keys():
			if String(key) in FORBIDDEN_KEYS:
				continue
			out[key] = strip_answer_keys((value as Dictionary)[key])
		return out
	if value is Array:
		var list: Array = []
		for item: Variant in (value as Array):
			list.append(strip_answer_keys(item))
		return list
	return value


## JSON paths of every FORBIDDEN_KEYS entry in `value` (contract scan, F063-AC-03 / SC-002).
static func find_answer_keys(value: Variant, path: String = "$") -> PackedStringArray:
	var found: PackedStringArray = []
	if value is Dictionary:
		for key: Variant in (value as Dictionary).keys():
			var child: String = "%s.%s" % [path, key]
			if String(key) in FORBIDDEN_KEYS:
				found.append(child)
			found.append_array(find_answer_keys((value as Dictionary)[key], child))
	elif value is Array:
		var i: int = 0
		for item: Variant in (value as Array):
			found.append_array(find_answer_keys(item, "%s[%d]" % [path, i]))
			i += 1
	return found


func is_known_type() -> bool:
	return type in TYPES


## The stem as a rich content (`{blocks: [...], audio_ref?}`), empty when absent.
func stem() -> Dictionary:
	var s: Variant = render_payload.get("stem")
	return s as Dictionary if s is Dictionary else {}


## URL of a media asset of this item (empty when unknown).
func media_url(asset_id: String) -> String:
	for m: Dictionary in media:
		if String(m.get("asset_id", "")) == asset_id:
			return String(m.get("url", ""))
	return ""


## The read-aloud audio of the stem (`stem.audio_ref`, else the first `media.kind = audio`), empty when none.
func voice_url() -> String:
	var ref: Variant = stem().get("audio_ref")
	if ref is String and not media_url(ref as String).is_empty():
		return media_url(ref as String)
	for m: Dictionary in media:
		if String(m.get("kind", "")) == "audio":
			return String(m.get("url", ""))
	return ""


## Whether the server says this item has hints (unknown → true: the server answers HINTS_EXHAUSTED/NOT_ALLOWED).
func may_have_hints() -> bool:
	return hint_count != 0
