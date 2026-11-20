class_name ActivityRenderModel
extends RefCounted
## Display part of the activity version pinned by an attempt (`ActivityRender`, ADR-0043, ADR-0048). No answer,
## scoring rule, per-option feedback or hint (ADR-0016); unknown answer-side keys are dropped like in
## QuestionRenderModel. Strings are shown verbatim in the attempt's `display_language` (ADR-0056).

## `ActivityRender.title` / `instructions` maxLength (specs/openapi).
const TITLE_MAX_LENGTH: int = 255
const INSTRUCTIONS_MAX_LENGTH: int = 2000

var activity_id: String = ""
var activity_version_id: String = ""
var activity_type: String = ""
var title: String = ""
var instructions: String = ""
var time_limit_seconds: int = 0
var render_payload: Dictionary = {}
var math_assets: Array[MathAssetRefModel] = []
var media: Array[Dictionary] = []
var min_client_version: String = ""
var content_language: String = ""
var display_language: String = ""
## The student's language has no translation: the original language is shown with `ui.learning.no_translation`.
var translation_missing: bool = false


static func from_dict(data: Dictionary) -> ActivityRenderModel:
	var r: ActivityRenderModel = ActivityRenderModel.new()
	r.activity_id = _text(data, "activity_id")
	r.activity_version_id = _text(data, "activity_version_id")
	r.activity_type = _text(data, "activity_type")
	r.title = _text(data, "title").left(TITLE_MAX_LENGTH)
	r.instructions = _text(data, "instructions").left(INSTRUCTIONS_MAX_LENGTH)
	var limit: Variant = data.get("time_limit_seconds")
	if limit is int or limit is float:
		r.time_limit_seconds = maxi(0, int(limit))
	var payload: Variant = data.get("render_payload", {})
	if payload is Dictionary:
		r.render_payload = QuestionRenderModel.strip_answer_keys(payload) as Dictionary
	r.math_assets = MathAssetRefModel.list_from(data.get("math_assets"))
	var raw_media: Variant = data.get("media", [])
	if raw_media is Array:
		for item: Variant in (raw_media as Array):
			if item is Dictionary:
				r.media.append(item as Dictionary)
	r.min_client_version = _text(data, "min_client_version")
	r.content_language = _text(data, "content_language")
	r.display_language = _text(data, "display_language")
	r.translation_missing = data.get("translation_missing") == true
	return r


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
