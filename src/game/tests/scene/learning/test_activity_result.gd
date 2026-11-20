extends GdUnitTestSuite
## F063-AC-21…24 (TC-LRN-004, TC-LMS-002; T029, T030): the result screen has a "Kết quả học tập" region from the
## review and a "Phần thưởng trong game" region only from server rewards (hidden when none); `g1_3` sees a count with a
## star and never a 10-point scale; nothing compares with other students or shows a teacher comment; results released
## later show `results_later` without asking the review; Mây's recommendation starts `startAdHocAttempt` with its
## `recommendation_id`.

const RESULT_SCENE: String = "res://scenes/learning/ActivityResult.tscn"
const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"
## Words that would compare the student with others or quote a teacher (AGENTS R16, ADR-0072).
const FORBIDDEN_WORDS: PackedStringArray = ["xếp hạng", "hạng", "trung bình lớp", "bạn khác", "giáo viên", "nhận xét",
	"rank", "class average", "teacher"]


func _view() -> VBoxContainer:
	var view: VBoxContainer = auto_free((load(RESULT_SCENE) as PackedScene).instantiate()) as VBoxContainer
	add_child(view)
	return view


func _texts(root: Node) -> String:
	var parts: PackedStringArray = []
	for node: Node in root.find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree():
			parts.append(tr((node as Label).text))
	for node: Node in root.find_children("*", "Button", true, false):
		if (node as Button).is_visible_in_tree():
			parts.append(tr((node as Button).text))
	return "\n".join(parts).to_lower()


func test_two_regions_and_hidden_rewards() -> void:
	var view: VBoxContainer = _view()
	view.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), "g4_5", [])
	assert_str(String(view.call("score_text"))).is_equal(tr("ui.result.correct_count").format({"correct": 1, "total": 2}))
	assert_bool(bool(view.call("points_visible"))).is_true()
	assert_str((view.get_node("%Points") as Label).text).is_equal(tr("ui.result.score").format({"score": "7", "max": "8"}))
	assert_bool((view.get_node("%Weak") as Label).visible).is_true()
	assert_bool(bool(view.call("rewards_visible"))).is_false()
	assert_bool((view.get_node("%Learning") as Control).visible).is_true()


func test_server_rewards_are_listed_apart_from_the_score() -> void:
	var view: VBoxContainer = _view()
	view.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), "g6_7",
			[{"kind": "xp", "amount": 40}, {"kind": "badge"}])
	assert_bool(bool(view.call("rewards_visible"))).is_true()
	var list: VBoxContainer = view.get_node("%RewardList") as VBoxContainer
	assert_str((list.get_child(0) as Label).text).is_equal(tr("ui.result.reward_xp").format({"amount": 40}))
	assert_bool(list.is_ancestor_of(view.get_node("%Score"))).is_false()


func test_g1_3_sees_a_count_with_a_star_and_no_ten_point_scale() -> void:
	var view: VBoxContainer = _view()
	view.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), "g1_3", [])
	assert_str(String(view.call("score_text"))).starts_with("⭐")
	assert_str(String(view.call("score_text"))).contains("1/2")
	assert_bool(bool(view.call("points_visible"))).is_false()
	assert_str(String(view.call("score_text"))).not_contains("/10")


func test_no_comparison_and_no_teacher_comment() -> void:
	var view: VBoxContainer = _view()
	var data: Dictionary = LearningFixtures.review()
	data["teacher_comment"] = "Giáo viên nhận xét: cần cố gắng"
	data["class_average"] = 6.5
	view.call("show_review", AttemptReviewModel.from_dict(data), "g6_7", [])
	var text: String = _texts(view)
	for word: String in FORBIDDEN_WORDS:
		assert_str(text).override_failure_message(word).not_contains(word)


func test_results_later_message() -> void:
	var view: VBoxContainer = _view()
	view.call("show_results_later", [])
	assert_bool((view.get_node("%Later") as Label).visible).is_true()
	assert_str(String(view.call("score_text"))).is_empty()
	assert_bool(bool(view.call("rewards_visible"))).is_false()


func test_recommendations_offer_practice_only_for_question_sets() -> void:
	var view: VBoxContainer = _view()
	var data: Dictionary = LearningFixtures.review()
	(data["recommendations"] as Array).append({"kind": "lesson", "ref_id": LearningFixtures.LESSON_ID, "title": "Xem lại bài nhân"})
	view.call("show_review", AttemptReviewModel.from_dict(data), "g4_5", [])
	var buttons: Array[Button] = view.call("recommendation_buttons") as Array[Button]
	assert_int(buttons.size()).is_equal(1)
	assert_str(buttons[0].text).is_equal("Luyện 3 câu nhân 7")
	assert_bool((view.get_node("%Recommendations") as Control).visible).is_true()


# ---------- through the activity host ----------

var h: AuthHarness = null


func _host() -> Control:
	h = AuthHarness.create()
	await h.sign_in()
	var host: Control = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as Control
	host.set("services", ServiceLocator.new(h.api, AppState, h.clock))
	host.set("client_version", "1.2.3")
	host.set("grade_band", "g4_5")
	add_child(host)
	return host


func _finish(host: Control) -> void:
	(host.get("renderer") as QuizActivityRenderer).select_index(0)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_hidden())
	host.call("press_primary")
	await _until(func() -> bool: return int(host.get("state")) == 4)
	host.call("press_primary")
	await _until(func() -> bool: return host.get("result_view") != null and int(host.get("state")) in [6, 7])


func _until(condition: Callable) -> void:
	for i: int in 200:
		if bool(condition.call()):
			return
		await await_idle_frame()


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	if h != null:
		h.dispose()
		h = null


func test_manual_release_shows_results_later_without_a_review_call() -> void:
	var host: Control = await _host()
	var data: Dictionary = LearningFixtures.attempt_with_types(["true_false"])
	data["mode"] = "assignment"
	data["result_release"] = "manual"
	data["feedback_timing"] = "after_close"
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "assignment"})
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted("submitted", "manual"))
	await _finish(host)
	var view: Control = host.get("result_view") as Control
	assert_bool(bool(view.get("results_later"))).is_true()
	assert_int(h.http.count(LearningFixtures.ROUTE_REVIEW)).is_equal(0)
	host.call("close")


func test_reopened_unreleased_attempt_asks_the_review_once() -> void:
	var host: Control = await _host()
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.submitted("submitted", "manual"))
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 409, LearningFixtures.problem(409, "CONFLICT"))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	await _until(func() -> bool: return int(host.get("state")) == 7)
	assert_int(h.http.count(LearningFixtures.ROUTE_REVIEW)).is_equal(1)
	assert_bool(bool((host.get("result_view") as Control).get("results_later"))).is_true()
	host.call("close")


func test_immediate_release_shows_the_review_and_practice_starts_with_the_recommendation() -> void:
	var host: Control = await _host()
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt_with_types(["true_false"]))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	await _finish(host)
	var view: Control = host.get("result_view") as Control
	assert_bool(bool(view.get("results_later"))).is_false()
	assert_bool(bool(view.call("rewards_visible"))).is_false()
	var buttons: Array[Button] = view.call("recommendation_buttons") as Array[Button]
	assert_int(buttons.size()).is_equal(1)
	h.http.reply(LearningFixtures.ROUTE_ADHOC, 201, LearningFixtures.fresh_attempt())
	buttons[0].pressed.emit()
	await _until(func() -> bool: return h.http.count(LearningFixtures.ROUTE_ADHOC) == 1 and host.get("renderer") != null)
	var body: Dictionary = h.http.last(LearningFixtures.ROUTE_ADHOC).body
	assert_str(String(body["recommendation_id"])).is_equal("0f630000-0000-4000-8000-0000000000f2")
	assert_str(String((body["source"] as Dictionary)["question_set_id"])).is_equal(LearningFixtures.QUESTION_SET_ID)
	assert_bool(Uuid.is_v7(String(h.http.last(LearningFixtures.ROUTE_ADHOC).headers["idempotency-key"]))).is_true()
	host.call("close")
