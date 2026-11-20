extends GdUnitTestSuite
## F063-AC-25, F063-AC-26 (TC-ACT-004; T032): an `activity_type` without a renderer in this build, a content minimum
## version above the client (`ActivityRender.min_client_version` or the manifest map) or `422
## ACTIVITY_TYPE_NOT_SUPPORTED` shows "Cần cập nhật game" with an update button and queues telemetry — no crash; an item
## whose question type this client cannot draw shows the notice inside the item and the student can still leave
## (pause keeps the progress).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var host: Control = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	host = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as Control
	host.set("services", services)
	host.set("client_version", "1.2.3")
	add_child(host)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	ActivityRendererRegistry.register("interactive_exercise",
			"res://scenes/learning/activity_renderers/interactive_exercise/InteractiveExerciseRenderer.tscn")
	h.dispose()


func _telemetry_codes() -> Array:
	var out: Array = []
	for event: Dictionary in services.telemetry.queue:
		out.append([String(event.get("error_code", "")), String(event.get("scene_code", ""))])
	return out


func _assert_update_screen(activity_type: String) -> void:
	assert_int(int(host.get("state"))).is_equal(9)
	var view: Control = host.get("update_view") as Control
	assert_object(view).is_not_null()
	assert_str((view.get_node("%Body") as Label).text).is_equal("ui.activity_player.update_required")
	assert_bool((view.get_node("%Update") as Button).visible).is_true()
	assert_object(host.get("renderer")).is_null()
	assert_array(_telemetry_codes()).contains([[LearningService.TELEMETRY_UPDATE_REQUIRED, activity_type]])


func test_registry_reasons() -> void:
	assert_str(ActivityRendererRegistry.check("quiz", "1.2.3")).is_equal(ActivityRendererRegistry.REASON_OK)
	assert_str(ActivityRendererRegistry.check("minigame", "1.2.3")).is_equal(ActivityRendererRegistry.REASON_UNKNOWN_TYPE)
	assert_str(ActivityRendererRegistry.check("quiz", "1.2.3", ["1.3.0"])).is_equal(ActivityRendererRegistry.REASON_CLIENT_TOO_OLD)
	assert_str(ActivityRendererRegistry.check("quiz", "1.2.3", ["", "1.2.3"])).is_equal(ActivityRendererRegistry.REASON_OK)


func test_activity_type_without_renderer() -> void:
	var data: Dictionary = LearningFixtures.fresh_attempt()
	data["activity_type"] = "minigame"
	(data["render"] as Dictionary)["activity_type"] = "minigame"
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	_assert_update_screen("minigame")


func test_known_type_of_the_intent_is_checked_before_start() -> void:
	ActivityRendererRegistry.unregister("interactive_exercise")
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "activity_type": "interactive_exercise"})
	_assert_update_screen("interactive_exercise")
	assert_int(h.http.count(LearningFixtures.ROUTE_START)).is_equal(0)


func test_content_newer_than_the_client() -> void:
	var data: Dictionary = LearningFixtures.fresh_attempt()
	(data["render"] as Dictionary)["min_client_version"] = "9.0.0"
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	_assert_update_screen("quiz")


func test_manifest_minimum_version() -> void:
	services.learning.activity_min_client_versions = {"quiz": "2.0.0"}
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	_assert_update_screen("quiz")


func test_server_says_not_supported() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 422, LearningFixtures.problem(422, "ACTIVITY_TYPE_NOT_SUPPORTED"))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "activity_type": "quiz"})
	_assert_update_screen("quiz")


func test_update_screen_back_closes_the_player() -> void:
	var closed: Array[bool] = []
	host.connect("closed", func() -> void: closed.append(true))
	h.http.reply(LearningFixtures.ROUTE_START, 422, LearningFixtures.problem(422, "ACTIVITY_TYPE_NOT_SUPPORTED"))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	((host.get("update_view") as Control).get_node("%Back") as Button).pressed.emit()
	assert_array(closed).has_size(1)
	assert_bool(QualityManager.suspend_reasons.has(&"activity")).is_false()


func test_unknown_item_type_shows_the_notice_in_the_item_and_allows_leaving() -> void:
	var data: Dictionary = LearningFixtures.attempt_with_types(["single_choice", "true_false"])
	((data["items"] as Array)[0] as Dictionary)["type"] = "hologram"
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	var renderer: QuizActivityRenderer = host.get("renderer") as QuizActivityRenderer
	assert_bool(renderer.current_item_unsupported()).is_true()
	var notice: Label = renderer.get_node("%Content").get_child(0) as Label
	assert_str(notice.text).is_equal("ui.activity_player.update_required")
	assert_bool((host.get_node("%Primary") as Button).disabled).is_true()
	host.call("ask_exit")
	assert_bool((host.get_node("%ExitConfirm") as Control).visible).is_true()
	h.http.reply(LearningFixtures.ROUTE_PAUSE, 200, data)
	(host.get_node("%Pause") as Button).pressed.emit()
	for i: int in 60:
		await await_idle_frame()
		if not host.visible:
			break
	assert_int(h.http.count(LearningFixtures.ROUTE_PAUSE)).is_equal(1)
	assert_bool(host.visible).is_false()


func test_unknown_interactive_widget_shows_the_notice_in_the_item() -> void:
	var item: Dictionary = LearningFixtures.item_of("interactive").duplicate(true)
	(item["render_payload"] as Dictionary)["widget"] = "molecule_builder"
	var q: QuestionRendererBase = auto_free(QuestionRendererRegistry.instantiate("interactive")) as QuestionRendererBase
	add_child(q)
	q.setup(QuestionRenderModel.from_dict(item), RendererContext.new())
	assert_bool(bool(q.call("is_supported"))).is_false()
	assert_str((q.get("notice") as Label).text).is_equal("ui.activity_player.update_required")
	assert_bool(q.commit()).is_false()
