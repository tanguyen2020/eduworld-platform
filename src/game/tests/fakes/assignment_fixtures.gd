class_name AssignmentFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): `listMyAssignments` bodies for F062 tests, derived from the generated contract fixtures
## (`tests/fixtures/contract/openapi/listMyAssignments.200.json` = the OpenAPI response example, `MyAssignment.json`
## = the schema sample; src/game/tools/gen_fixtures.mjs). Variants (empty, every status, unknown kind, long lists)
## are built from those samples so they never drift from the contract.

const FIXTURES: String = "res://tests/fixtures/contract/openapi"


static func _json(name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [FIXTURES, name])) as Dictionary


## The OpenAPI example: one `activity` (not_started) and one `assessment` with `exam_session_id` (in_progress).
static func two() -> Dictionary:
	return _json("listMyAssignments.200")


static func empty() -> Dictionary:
	var body: Dictionary = two()
	body["data"] = []
	return body


## One item per `status` value of the contract (activity kind).
static func all_statuses() -> Dictionary:
	var items: Array = []
	var i: int = 0
	for status: String in MyAssignmentModel.STATUSES:
		var item: Dictionary = item_sample(i)
		item["status"] = status
		items.append(item)
		i += 1
	return page(items)


## An item whose `content_kind` has no screen in this client (`quest`) plus one with an unknown kind and status.
static func unsupported_kinds() -> Dictionary:
	var quest: Dictionary = item_sample(1)
	quest["content_kind"] = "quest"
	var lesson: Dictionary = item_sample(2)
	lesson["content_kind"] = "lesson"
	var future: Dictionary = item_sample(3)
	future["content_kind"] = "hologram"
	future["status"] = "paused"
	future["unknown_field"] = {"x": 1}
	return page([quest, lesson, future])


## OQ F062-1 (T036): "Bài 1" carries `subject_name` (vi + en), "Bài 2" has no `subject_name` field at all (an
## older server / a subject without a name), "Bài 3" only the content language (vi).
static func subjects() -> Dictionary:
	var named: Dictionary = item_sample(0)
	named["subject_name"] = {"vi": "Toán", "en": "Mathematics"}
	var unnamed: Dictionary = item_sample(1)
	unnamed.erase("subject_name")
	var vi_only: Dictionary = item_sample(2)
	vi_only["subject_name"] = {"vi": "Tiếng Việt"}
	return page([named, unnamed, vi_only])


## OQ F062-2 (T037): one `question_set` assignment.
static func question_set() -> Dictionary:
	var item: Dictionary = item_sample(0)
	item["content_kind"] = MyAssignmentModel.KIND_QUESTION_SET
	return page([item])


static func many(count: int) -> Dictionary:
	var items: Array = []
	for i: int in count:
		items.append(item_sample(i))
	return page(items)


## The schema sample with a unique id/title (`index`) and a fixed future due date.
static func item_sample(index: int) -> Dictionary:
	var item: Dictionary = _json("MyAssignment")
	item["assignment_id"] = "0190f0a0-0000-7000-8000-%012d" % (1000 + index)
	item["content_ref"] = "0190f0a0-0000-7000-8000-%012d" % (2000 + index)
	item["title"] = "Bài %d" % (index + 1)
	item["content_kind"] = "activity"
	item["due_at"] = "2026-11-20T09:30:00Z"
	return item


static func page(items: Array, has_more: bool = false, next_cursor: String = "") -> Dictionary:
	var info: Dictionary = {"has_more": has_more, "limit": AssignmentService.PAGE_LIMIT}
	if not next_cursor.is_empty():
		info["next_cursor"] = next_cursor
	return {"data": items, "page": info}
