extends GdUnitTestSuite
## F064-AC-17, F064-AC-18, F064-AC-19 (TC-EXM-004, TC-EXM-005; T021, T022): after the submit the lobby calls
## `getExamResult`; `403 EXAM_RESULTS_NOT_RELEASED` shows `ui.exam.results_pending`; a released result shows the
## server score by grade band (`g1_3`: right answers with a star, no 10-point scale), the per-objective rows and the
## wrong questions with explanation / option feedback only when the server sent them — never a rank or another
## student; "Luyện LO này" asks `activity_host` to start `startAdHocAttempt` with `source.kind = selection` and the
## `objective_id`. "Xem kết quả" reopens the result of a submitted assignment. FakeHttp/FakeClock (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var lobby: Control = null
var intents: Array[Array] = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	lobby = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	add_child(lobby)
	intents.clear()
	EventBus.ui_requested.connect(_on_ui_requested)


func after_test() -> void:
	EventBus.ui_requested.disconnect(_on_ui_requested)
	QualityManager.suspend_reasons.clear()
	AppState.set_assignments([] as Array[MyAssignmentModel])
	h.dispose()


func _on_ui_requested(screen_id: StringName, args: Dictionary) -> void:
	intents.append([screen_id, args])


func _submit_and_get(result_status: int, result_body: Dictionary, band: String = "g4_5") -> Control:
	lobby.set("grade_band", band)
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session("free", "immediate"))
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID})
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 200, ExamFixtures.submit_result("graded"))
	h.http.reply(ExamFixtures.ROUTE_RESULT, result_status, result_body)
	(runner.find_child("SubmitConfirm", true, false) as Control).call("confirm")
	for i: int in 40:
		await await_idle_frame()
	return lobby.get("result_view") as Control


func _label(view: Control, node_name: String) -> Label:
	return view.find_child(node_name, true, false) as Label


func _texts(view: Control) -> String:
	var out: String = ""
	for node: Node in view.find_children("*", "Label", true, false):
		if (node as Label).visible:
			out += tr((node as Label).text) + "\n"
	return out


func test_results_not_released_show_pending() -> void:
	var view: Control = await _submit_and_get(403, ExamFixtures.problem(403, "EXAM_RESULTS_NOT_RELEASED"))
	assert_int(h.http.count(ExamFixtures.ROUTE_RESULT)).is_equal(1)
	assert_str(h.http.last(ExamFixtures.ROUTE_RESULT).path).is_equal("/exams/attempts/%s/result" % ExamFixtures.ATTEMPT_ID)
	assert_object(view).is_not_null()
	assert_bool(_label(view, "Pending").visible).is_true()
	assert_str(tr(_label(view, "Pending").text)).is_equal(tr("ui.exam.results_pending"))
	assert_bool(_label(view, "Score").visible).is_false()


func test_released_result_shows_score_objectives_and_wrong_items() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result())
	assert_bool(_label(view, "Pending").visible).is_false()
	assert_str(_label(view, "Score").text).is_equal(tr("ui.exam.result_score_scaled").format({"score": "8.50"}))
	assert_str(_label(view, "Correct").text).is_equal(tr("ui.exam.result_correct").format({"correct": 1, "total": 2}))
	var objectives: Array[Node] = (view.find_child("Objectives", true, false) as Control).get_children()
	# One row per objective of the items, tallied from the server verdicts (item 1 wrong, item 2 right).
	assert_int(objectives.size()).is_equal(2)
	assert_str((objectives[0] as Label).text).is_equal(
			tr("ui.exam.result_objective").format({"n": 1, "correct": 0, "total": 1}))
	assert_str((objectives[1] as Label).text).is_equal(
			tr("ui.exam.result_objective").format({"n": 2, "correct": 1, "total": 1}))
	var wrong: String = _texts(view.find_child("WrongList", true, false) as Control)
	assert_str(wrong).contains(tr("ui.exam.result_wrong_item").format({"n": 1}))
	assert_str(wrong).contains("Có 3 túi, mỗi túi 6 quả nên lấy 6 × 3 = 18.")
	assert_str(wrong).contains("Em đã cộng 3 + 6.")


func test_explanation_and_feedback_only_when_sent() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result_without_details())
	var wrong: String = _texts(view.find_child("WrongList", true, false) as Control)
	assert_str(wrong).contains(tr("ui.exam.result_wrong_item").format({"n": 1}))
	assert_str(wrong).not_contains("6 × 3 = 18")
	assert_str(wrong).not_contains("Em đã cộng")


func test_g1_3_shows_right_answers_with_a_star_and_no_scale() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result(), "g1_3")
	assert_bool(_label(view, "Score").visible).is_false()
	assert_str(_label(view, "Correct").text).is_equal(tr("ui.exam.result_correct").format({"correct": 1, "total": 2}))
	assert_bool((view.find_child("StarIcon", true, false) as TextureRect).visible).is_true()


func test_no_rank_and_no_other_student() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result())
	for node: Node in view.find_children("*", "", true, false):
		assert_str(String(node.name).to_lower()).not_contains("rank")
		assert_str(String(node.name).to_lower()).not_contains("leaderboard")
	assert_str(_texts(view)).not_contains("hs.")


func test_practice_objective_starts_a_selection_attempt() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result())
	var buttons: Array[Button] = []
	for node: Node in (view.find_child("Practice", true, false) as Control).get_children():
		if node is Button and (node as Button).text == "ui.exam.practice_objective":
			buttons.append(node as Button)
	assert_int(buttons.size()).is_equal(1)
	buttons[0].pressed.emit()
	assert_int(intents.size()).is_equal(1)
	assert_str(String(intents[0][0])).is_equal("activity_host")
	var args: Dictionary = intents[0][1] as Dictionary
	assert_dict(args).is_equal({"objective_id": ExamFixtures.OBJECTIVE_ID, "mode": "practice", "context_type": "free_play"})
	var req: ApiRequest = LearningService.start_request(args)
	assert_str(req.method).is_equal("POST")
	assert_str(req.path).is_equal("/learning/activity-attempts")
	assert_dict(req.body as Dictionary).is_equal({
		"mode": "practice", "context_type": "free_play",
		"source": {"kind": "selection", "objective_id": ExamFixtures.OBJECTIVE_ID},
	})


func test_recommended_practice_is_offered_too() -> void:
	var view: Control = await _submit_and_get(200, ExamFixtures.result())
	var titles: PackedStringArray = []
	for node: Node in (view.find_child("Practice", true, false) as Control).get_children():
		if node is Button:
			titles.append((node as Button).text)
	assert_bool("Luyện 3 câu nhân 7" in titles).is_true()


func test_view_result_of_a_submitted_assignment() -> void:
	var a: MyAssignmentModel = MyAssignmentModel.new()
	a.assignment_id = ExamFixtures.ASSIGNMENT_ID
	a.title = "Kiểm tra: Phép nhân"
	a.status = "graded"
	a.content_kind = MyAssignmentModel.KIND_ASSESSMENT
	a.exam_session_id = ExamFixtures.SESSION_ID
	a.attempt_id = ExamFixtures.ATTEMPT_ID
	AppState.set_assignments([a] as Array[MyAssignmentModel])
	lobby.set("grade_band", "g6_7")
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session())
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID, "assignment_id": ExamFixtures.ASSIGNMENT_ID})
	var button: Button = lobby.find_child("ViewResult", true, false) as Button
	assert_bool(button.visible).is_true()
	h.http.reply(ExamFixtures.ROUTE_RESULT, 200, ExamFixtures.result())
	button.pressed.emit()
	for i: int in 20:
		await await_idle_frame()
	assert_str(h.http.last(ExamFixtures.ROUTE_RESULT).path).is_equal("/exams/attempts/%s/result" % ExamFixtures.ATTEMPT_ID)
	assert_bool(_label(lobby.get("result_view") as Control, "Score").visible).is_true()
