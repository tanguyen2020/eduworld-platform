extends GdUnitTestSuite
## F062-AC-07..F062-AC-12 UI part (FR-LMS-006, STUDENT_GAME_UX_CORE S05, TC-LMS-002, TC-I18N-007, TC-ACT-004;
## T015, T016): the "Nhà trường giao" tab refreshes on open, shows title (verbatim), due date in the UI locale and
## status (icon + text) for each assignment in a virtualised list (≤ 12 row nodes); "Bắt đầu" emits `activity_host` /
## `exam_lobby` with the right arguments; a kind without a screen shows `ui.activity_player.update_required` and queues
## `client_unsupported_assignment_kind`; no other student's data, no score, no teacher comment; errors show "Thử lại"
## and keep the previous list.

const JOURNAL: String = "res://scenes/quest/QuestJournal.tscn"
const ROUTE: String = "GET /learning/my-assignments"

var h: AuthHarness = null
var _services: ServiceLocator = null
var _locale: String = ""
var _intents: Array[Array] = []


func before_test() -> void:
	_locale = String(Settings.locale)
	Settings.set_locale("vi-VN", false)
	_services = AppState.services
	h = AuthHarness.create()
	await h.sign_in()
	AppState.services = ServiceLocator.new(h.api, AppState, h.clock)
	_intents.clear()
	EventBus.ui_requested.connect(_on_ui_requested)


func after_test() -> void:
	EventBus.ui_requested.disconnect(_on_ui_requested)
	h.dispose()
	AppState.services = _services
	Settings.set_locale(_locale, false)


func _on_ui_requested(screen_id: StringName, args: Dictionary) -> void:
	_intents.append([screen_id, args])


func _open(body: Dictionary) -> Array:
	h.http.reply(ROUTE, 200, body)
	var runner: GdUnitSceneRunner = scene_runner(JOURNAL)
	await runner.simulate_frames(1)
	var journal: Control = runner.scene() as Control
	journal.call("open", {"tab": "assigned"})
	await runner.simulate_frames(6)
	return [runner, journal]


func _rows(journal: Control) -> Array[Control]:
	var out: Array[Control] = []
	for row: Control in journal.get("rows") as Array[Control]:
		if row.visible:
			out.append(row)
	return out


func _row_for(journal: Control, title: String) -> Control:
	for row: Control in _rows(journal):
		if (row.find_child("Title", true, false) as Label).text == title:
			return row
	return null


func _press_start(row: Control) -> void:
	(row.find_child("Start", true, false) as Button).pressed.emit()


func test_opening_refreshes_and_rows_show_title_due_and_status() -> void:
	var r: Array = await _open(AssignmentFixtures.all_statuses())
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var journal: Control = r[1] as Control
	assert_int(h.http.count(ROUTE)).is_equal(1)
	var rows: Array[Control] = _rows(journal)
	assert_int(rows.size()).is_equal(MyAssignmentModel.STATUSES.size())
	var seen_status: Array[String] = []
	for row: Control in rows:
		var model: MyAssignmentModel = row.get("assignment") as MyAssignmentModel
		var title: Label = row.find_child("Title", true, false) as Label
		assert_str(title.text).is_equal(model.title)
		assert_int(title.auto_translate_mode).is_equal(Node.AUTO_TRANSLATE_MODE_DISABLED)
		assert_str((row.find_child("StatusIcon", true, false) as Label).text).is_not_empty()
		var status: String = (row.find_child("Status", true, false) as Label).text
		assert_bool(status.is_empty() or status.begins_with("ui.")).is_false()
		seen_status.append(status)
		assert_str((row.find_child("Due", true, false) as Label).text).is_equal(_due_of(model))
	assert_int(seen_status.size()).is_equal(6)
	assert_bool((journal.find_child("StateLabel", true, false) as Label).visible).is_false()
	# Locale change: UI strings follow, titles stay verbatim.
	var due_vi: String = (rows[0].find_child("Due", true, false) as Label).text
	Settings.set_locale("en-US", false)
	await runner.simulate_frames(1)
	assert_str((rows[0].find_child("Due", true, false) as Label).text).is_not_equal(due_vi)
	assert_str((rows[0].find_child("Title", true, false) as Label).text).is_equal("Bài 1")


static func _due_of(model: MyAssignmentModel) -> String:
	var script: GDScript = load("res://scenes/quest/assignment_row.gd") as GDScript
	return String(script.call("due_text", model))


func test_long_lists_are_virtualised() -> void:
	var r: Array = await _open(AssignmentFixtures.many(100))
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var journal: Control = r[1] as Control
	assert_int((journal.get("rows") as Array).size()).is_less_equal(12)
	assert_int(AppState.assignments.size()).is_equal(100)
	var scroll: ScrollContainer = journal.find_child("List", true, false) as ScrollContainer
	scroll.scroll_vertical = int((journal.find_child("Content", true, false) as Control).custom_minimum_size.y)
	await runner.simulate_frames(3)
	assert_object(_row_for(journal, "Bài 100")).is_not_null()
	assert_int((journal.get("rows") as Array).size()).is_less_equal(12)


func test_start_activity_and_assessment_emit_the_right_intents() -> void:
	var r: Array = await _open(AssignmentFixtures.two())
	var journal: Control = r[1] as Control
	var data: Array = AssignmentFixtures.two()["data"] as Array
	var activity: Dictionary = data[0] as Dictionary
	var exam: Dictionary = data[1] as Dictionary
	_press_start(_row_for(journal, String(activity["title"])))
	_press_start(_row_for(journal, String(exam["title"])))
	assert_int(_intents.size()).is_equal(2)
	assert_str(String(_intents[0][0])).is_equal("activity_host")
	assert_dict(_intents[0][1] as Dictionary).is_equal({
		"activity_id": activity["content_ref"], "mode": "assignment", "context_type": "assignment",
		"context_id": activity["assignment_id"],
	})
	assert_str(String(_intents[1][0])).is_equal("exam_lobby")
	assert_dict(_intents[1][1] as Dictionary).is_equal({
		"exam_session_id": exam["exam_session_id"], "assignment_id": exam["assignment_id"],
	})


func test_kind_without_screen_shows_update_required_and_telemetry() -> void:
	var r: Array = await _open(AssignmentFixtures.unsupported_kinds())
	var journal: Control = r[1] as Control
	var telemetry: TelemetryService = AppState.services.telemetry
	for title: String in ["Bài 2", "Bài 3", "Bài 4"]:
		_press_start(_row_for(journal, title))
	assert_array(_intents).is_empty()
	var notice: Control = journal.find_child("Notice", true, false) as Control
	assert_bool(notice.visible).is_true()
	assert_str((notice.find_child("Body", true, false) as Label).text).is_equal(tr("ui.activity_player.update_required"))
	assert_int(telemetry.queue.size()).is_equal(3)
	for event: Dictionary in telemetry.queue:
		assert_str(String(event["type"])).is_equal("client_error")
		assert_str(String(event["error_code"])).is_equal("client_unsupported_assignment_kind")
	# An unknown status is shown as "not supported", never as a raw value.
	var status: String = (_row_for(journal, "Bài 4").find_child("Status", true, false) as Label).text
	assert_str(status).is_equal(tr("ui.quest.status.unsupported"))


func test_no_other_student_data_score_or_teacher_comment() -> void:
	var body: Dictionary = AssignmentFixtures.two()
	for item: Dictionary in body["data"] as Array:
		item["score"] = 9.5
		item["student_name"] = "Nguyen Van Khac"
		item["teacher_comment"] = "Nhan xet giao vien"
		item["classmates"] = [{"nickname": "BanKhac", "score": 10}]
	var r: Array = await _open(body)
	var journal: Control = r[1] as Control
	var texts: PackedStringArray = []
	for node: Node in journal.find_children("*", "Label", true, false):
		texts.append((node as Label).text)
	var all: String = "\n".join(texts)
	for forbidden: String in ["9.5", "9,5", "Nguyen Van Khac", "Nhan xet", "BanKhac"]:
		assert_str(all).override_failure_message(forbidden).not_contains(forbidden)
	var props: PackedStringArray = []
	for p: Dictionary in MyAssignmentModel.new().get_property_list():
		props.append(String(p["name"]))
	for field: String in ["score", "student_name", "teacher_comment", "classmates", "display_name"]:
		assert_bool(field in props).override_failure_message(field).is_false()


func test_error_shows_retry_and_keeps_the_previous_list() -> void:
	var r: Array = await _open(AssignmentFixtures.two())
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var journal: Control = r[1] as Control
	var retry: Button = journal.find_child("Retry", true, false) as Button
	assert_bool(retry.visible).is_false()
	h.http.reply(ROUTE, 503, AuthHarness.problem(503, "UNAVAILABLE"))
	journal.call("refresh")
	await runner.simulate_frames(12)
	assert_bool(retry.visible).is_true()
	assert_str((journal.find_child("StateLabel", true, false) as Label).text).is_equal(tr("ui.quest.load_failed"))
	assert_int(_rows(journal).size()).is_equal(2)
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	retry.pressed.emit()
	await runner.simulate_frames(6)
	assert_bool(retry.visible).is_false()


func test_empty_list_shows_the_empty_state() -> void:
	var r: Array = await _open(AssignmentFixtures.empty())
	var journal: Control = r[1] as Control
	var state: Label = journal.find_child("StateLabel", true, false) as Label
	assert_bool(state.visible).is_true()
	assert_str(state.text).is_equal(tr("ui.quest.empty"))
	assert_int(_rows(journal).size()).is_equal(0)
