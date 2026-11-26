extends GdUnitTestSuite
## F063-AC-17 (T024): when the option feedback has a `worked_example_ref`, the strip shows
## "Xem ví dụ giải từng bước"; pressing it loads the lesson with `getLesson` and opens a read-only overlay in the
## attempt's language. Without a reference the button is hidden.

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func test_worked_example_button_opens_the_lesson_overlay() -> void:
	var host: Control = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as Control
	host.set("services", ServiceLocator.new(h.api, AppState, h.clock))
	host.set("client_version", "1.2.3")
	add_child(host)
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	# F098: the tap itself sends a single-choice answer in practice (no "Kiểm tra" press).
	(host.get("renderer") as QuizActivityRenderer).select_index(0)
	var strip: Control = host.get_node("%FeedbackStrip") as Control
	var button: Button = strip.get_node("%WorkedExample") as Button
	for i: int in 60:
		await await_idle_frame()
		if button.visible:
			break
	assert_bool(button.visible).is_true()
	assert_str(button.text).is_equal("ui.feedback.worked_example")
	h.http.reply(LearningFixtures.ROUTE_LESSON, 200, LearningFixtures.lesson())
	button.pressed.emit()
	var overlay: PanelContainer = host.get_node("%WorkedExample") as PanelContainer
	for i: int in 60:
		await await_idle_frame()
		if overlay.visible:
			break
	assert_bool(overlay.visible).is_true()
	assert_str(h.http.last(LearningFixtures.ROUTE_LESSON).path).is_equal("/learning/lessons/%s" % LearningFixtures.LESSON_ID)
	assert_str(String(overlay.call("title_text"))).is_equal("Ví dụ: nhân 3 túi cam")
	assert_int(overlay.find_children("*", "LineEdit", true, false).size()).is_equal(0)
	assert_int(overlay.find_children("*", "TextEdit", true, false).size()).is_equal(0)
	overlay.call("close")
	assert_bool(overlay.visible).is_false()
	host.call("close")


func test_no_reference_hides_the_button() -> void:
	var strip: VBoxContainer = auto_free((load("res://scenes/learning/FeedbackStrip.tscn") as PackedScene).instantiate()) as VBoxContainer
	add_child(strip)
	var data: Dictionary = LearningFixtures.response_result()
	(data["feedback"] as Dictionary).erase("worked_example_ref")
	await strip.call("show_result", ResponseResultModel.from_dict(data), null)
	assert_bool((strip.get_node("%WorkedExample") as Button).visible).is_false()
