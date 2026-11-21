class_name ExamFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): exam zone bodies for F064 tests, built from the contract fixtures generated from the OpenAPI
## examples (`tests/fixtures/contract/openapi/{ExamSession,Attempt,AttemptResult,AttemptReview}.json`,
## src/game/tools/gen_fixtures.mjs) so they never drift from the contract. Variants (navigation, exam attempt with a
## server deadline, saved answers, released result…) are derived from those samples.

const FIXTURES: String = "res://tests/fixtures/contract/openapi"
const SESSION_ID: String = "0f640000-0000-4000-8000-000000000001"
const OTHER_SESSION_ID: String = "0f640000-0000-4000-8000-000000000002"
const ATTEMPT_ID: String = "0f640000-0000-7000-8000-000000000a01"
const OBJECTIVE_ID: String = "0f630000-0000-4000-8000-0000000000e1"
const ASSIGNMENT_ID: String = "0f640000-0000-4000-8000-0000000000c1"
## Server clock of the fixtures: the attempt is received at SERVER_TIME and ends DURATION_SEC later.
const SERVER_TIME: String = "2026-11-20T01:30:00Z"
const DEADLINE_AT: String = "2026-11-20T01:45:00Z"
const DURATION_SEC: int = 900
const ROUTE_LIST: String = "GET /exams/sessions"
const ROUTE_SESSION: String = "GET /exams/sessions/*"
const ROUTE_START: String = "POST /exams/sessions/*/attempts"
const ROUTE_SAVE: String = "PUT /exams/attempts/*/answers/*"
const ROUTE_SUBMIT: String = "POST /exams/attempts/*:submit"
const ROUTE_RESULT: String = "GET /exams/attempts/*/result"
const ROUTE_ADHOC: String = "POST /learning/activity-attempts"
const ROUTE_ASSIGNMENTS: String = "GET /learning/my-assignments"


static func _json(name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [FIXTURES, name])) as Dictionary


## The OpenAPI example (`navigation = free`, 15 questions, 15 minutes, results after close).
static func session(navigation: String = "free", release: String = "after_close") -> Dictionary:
	var s: Dictionary = _json("ExamSession")
	s["navigation"] = navigation
	s["result_release"] = release
	return s


static func list_body(sessions: Array) -> Dictionary:
	return {"data": sessions, "page": {"has_more": false, "next_cursor": null}}


## An exam attempt (mode `exam`, deadline on the server clock) with `count` items of the Attempt example.
static func attempt(count: int = 5, saved: Array = []) -> Dictionary:
	var a: Dictionary = _json("Attempt")
	a["attempt_id"] = ATTEMPT_ID
	a["mode"] = "exam"
	a["status"] = "in_progress"
	a["context_type"] = "exam_session"
	a["context_id"] = SESSION_ID
	a["started_at"] = SERVER_TIME
	a["server_time"] = SERVER_TIME
	a["deadline_at"] = DEADLINE_AT
	a["feedback_timing"] = "after_close"
	a["result_release"] = "after_close"
	a.erase("render")
	var items: Array = (a["items"] as Array).slice(0, count)
	a["items"] = items
	a["saved_responses"] = saved
	return a


## Item types of the Attempt example, by position.
static func item_type(position: int) -> String:
	return String(((_json("Attempt")["items"] as Array)[position - 1] as Dictionary)["type"])


static func submit_result(status: String = "submitted") -> Dictionary:
	var r: Dictionary = _json("AttemptResult")
	r["attempt_id"] = ATTEMPT_ID
	r["status"] = status
	r["score"] = null
	r["max_score"] = null
	r["correct_count"] = null
	r["rewards"] = []
	return r


## A released result (`AttemptReview` example with the exam fields).
static func result() -> Dictionary:
	var r: Dictionary = _json("AttemptReview")
	r["attempt_id"] = ATTEMPT_ID
	r["score_scaled"] = 8.5
	r["section_scores"] = [{"section_id": "sec_tn", "title": "Trắc nghiệm", "score": 7, "max_score": 8}]
	return r


## A released result without per-item answers, explanations or feedback (show_correct_answers/explanations off).
static func result_without_details() -> Dictionary:
	var r: Dictionary = result()
	for item: Variant in (r["items"] as Array):
		(item as Dictionary).erase("explanation")
		(item as Dictionary).erase("feedback")
	return r


static func problem(status: int, code: String) -> Dictionary:
	return AuthHarness.problem(status, code)
