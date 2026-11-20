class_name LearningFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): activity player bodies for F063 tests, built from the contract fixtures generated from the
## OpenAPI examples (`tests/fixtures/contract/openapi/{Attempt,ActivityResponseResult,HintResult,AttemptReview}.json`,
## src/game/tools/gen_fixtures.mjs) so they never drift from the contract. Variants (no feedback, other mode,
## deadline, unknown type…) are derived from those samples.

const FIXTURES: String = "res://tests/fixtures/contract/openapi"
const MATH_SVG_PATH: String = "res://tests/fixtures/learning/math_three_quarters.svg.txt"
const ATTEMPT_ID: String = "0f630000-0000-7000-8000-000000000001"
const ACTIVITY_ID: String = "0f630000-0000-4000-8000-0000000000a1"
const OTHER_ATTEMPT_ID: String = "0f630000-0000-7000-8000-0000000000ff"
const ASSIGNMENT_ID: String = "0f630000-0000-4000-8000-0000000000b1"
const QUESTION_SET_ID: String = "0f630000-0000-4000-8000-0000000000f1"
const LESSON_ID: String = "0f630000-0000-4000-8000-0000000000d1"
const MATH_SHA: String = "a5099375262cf82eb71a0aa9ecc4c61135a9e3f7fbbf335f0ee4527fcc2bf5e1"
const ROUTE_START: String = "POST /learning/activities/*/attempts"
const ROUTE_ADHOC: String = "POST /learning/activity-attempts"
const ROUTE_GET: String = "GET /learning/activity-attempts/" + ATTEMPT_ID
const ROUTE_RESPOND: String = "POST /learning/activity-attempts/*/responses"
const ROUTE_HINT: String = "POST /learning/activity-attempts/*/items/*/hints:next"
const ROUTE_PAUSE: String = "POST /learning/activity-attempts/*:pause"
const ROUTE_SUBMIT: String = "POST /learning/activity-attempts/*:submit"
const ROUTE_REVIEW: String = "GET /learning/activity-attempts/*/review"
const ROUTE_LESSON: String = "GET /learning/lessons/*"
const ROUTE_MATH: String = "GET https://cdn.example.invalid/math/*"


static func _json(name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [FIXTURES, name])) as Dictionary


## The OpenAPI example: practice quiz, immediate feedback, 11 items (one per QuestionType), item 1 answered.
static func attempt() -> Dictionary:
	return _json("Attempt")


## The example with no saved answer (starts at item 1).
static func fresh_attempt() -> Dictionary:
	var a: Dictionary = attempt()
	a["saved_responses"] = []
	return a


## Only the items of the given types (keeps their positions renumbered from 1).
static func attempt_with_types(types: PackedStringArray) -> Dictionary:
	var a: Dictionary = fresh_attempt()
	var kept: Array = []
	for item: Variant in (a["items"] as Array):
		if String((item as Dictionary)["type"]) in types:
			var copy: Dictionary = (item as Dictionary).duplicate(true)
			copy["position"] = kept.size() + 1
			kept.append(copy)
	a["items"] = kept
	return a


static func item_of(type: String) -> Dictionary:
	for item: Variant in (attempt()["items"] as Array):
		if String((item as Dictionary)["type"]) == type:
			return item as Dictionary
	return {}


static func question(type: String) -> QuestionRenderModel:
	return QuestionRenderModel.from_dict(item_of(type))


static func submitted(status: String = "graded", release: String = "immediate") -> Dictionary:
	var a: Dictionary = attempt()
	a["status"] = status
	a["result_release"] = release
	a["submitted_at"] = "2026-11-20T02:20:00Z"
	return a


static func response_result() -> Dictionary:
	return _json("ActivityResponseResult")


## Grading hidden by FeedbackTiming (`after_submit`): every verdict field null.
static func response_result_hidden() -> Dictionary:
	var r: Dictionary = response_result()
	r["is_correct"] = null
	r["score"] = null
	r["feedback"] = null
	r.erase("explanation")
	r["explanation_math_assets"] = []
	return r


static func response_result_correct() -> Dictionary:
	var r: Dictionary = response_result()
	r["is_correct"] = true
	r["score"] = 1
	r["feedback"] = null
	return r


static func hint(level: int = 1, total: int = 2) -> Dictionary:
	var h: Dictionary = _json("HintResult")
	h["level"] = level
	h["total_levels"] = total
	return h


static func review() -> Dictionary:
	return _json("AttemptReview")


static func lesson() -> Dictionary:
	return {
		"lesson_id": LESSON_ID, "version": 1, "activities": [],
		"title": {"vi": "Ví dụ: nhân 3 túi cam", "en": "Example: three bags of oranges"},
		"summary": {"vi": "Có 3 túi, mỗi túi 6 quả: 6 × 3 = 18."},
	}


static func math_svg() -> PackedByteArray:
	return FileAccess.get_file_as_bytes(MATH_SVG_PATH)


static func problem(status: int, code: String) -> Dictionary:
	return AuthHarness.problem(status, code)
