extends GdUnitTestSuite
## F073-AC-21 (TC-ACT-004; T032): the client picks the mini-game scene by `minigame_type` through MiniGameRegistry,
## restyles it from `presentation.scene_variant` (same scene, no new script), and a type without a scene shows
## "Cần cập nhật game", queues telemetry and never crashes — on the free-play screen (`minigame_host`) and inside the
## activity player (`activity_type = minigame`). FakeHttp/FakeClock with the F073 fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/minigames/MiniGameHost.tscn"
const ACTIVITY_HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"
const ALL_TYPES: PackedStringArray = [
	"multiple_choice", "true_false", "fill_in_blank", "drag_and_drop", "matching", "ordering", "calculation", "puzzle",
	"word_game", "science_simulation", "timed_challenge",
]

var h: AuthHarness = null
var services: ServiceLocator = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _telemetry() -> Array:
	var out: Array = []
	for event: Dictionary in services.telemetry.queue:
		out.append([String(event.get("error_code", "")), String(event.get("scene_code", ""))])
	return out


func _open_host(session: Dictionary) -> MiniGameHost:
	var host: MiniGameHost = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as MiniGameHost
	host.services = services
	host.grade_band = "g4_5"
	add_child(host)
	h.http.reply(MiniGameFixtures.ROUTE_START, 201, session)
	await host.open({"minigame_config_id": MiniGameFixtures.CONFIG_ID})
	return host


func test_every_type_of_the_catalog_has_a_scene_extending_minigame_base() -> void:
	assert_array(Array(MiniGameRegistry.types())).contains_exactly_in_any_order(Array(ALL_TYPES))
	for type: String in ALL_TYPES:
		assert_bool(MiniGameRegistry.has(type)).override_failure_message(type).is_true()
		var scene: MiniGameBase = auto_free(MiniGameRegistry.instantiate(type)) as MiniGameBase
		assert_object(scene).override_failure_message(type).is_not_null()
		assert_bool(scene is MiniGameBase).is_true()


func test_compatibility_table_of_question_types() -> void:
	assert_array(Array(MiniGameRegistry.question_types("matching"))).contains_exactly(["matching"])
	assert_array(Array(MiniGameRegistry.question_types("timed_challenge"))).contains_exactly(["single_choice", "true_false", "numeric"])
	assert_array(Array(MiniGameRegistry.question_types("science_simulation"))).contains_exactly(["interactive"])
	assert_bool(MiniGameRegistry.accepts("calculation", "numeric")).is_true()
	assert_bool(MiniGameRegistry.accepts("calculation", "matching")).is_false()
	assert_bool(MiniGameRegistry.accepts("puzzle", "interactive")).is_false()


func test_contract_spelling_drag_drop_maps_to_the_same_scene() -> void:
	assert_str(MiniGameRegistry.scene_path("drag_drop")).is_equal(MiniGameRegistry.scene_path("drag_and_drop"))


func test_scene_variant_restyles_the_same_scene() -> void:
	assert_str(MiniGameRegistry.variant_of("matching", "matching.market_cards")).is_equal("market_cards")
	assert_str(MiniGameRegistry.variant_of("matching", "ordering.train")).is_equal("")
	assert_str(MiniGameRegistry.variant_of("matching", "matching.unknown_skin")).is_equal("")
	var scene: MiniGameQuestionScene = auto_free(MiniGameRegistry.instantiate("matching")) as MiniGameQuestionScene
	add_child(scene)
	var script_before: Script = scene.get_script() as Script
	scene.setup(MiniGameFixtures.model("matching").config, MiniGameFixtures.model("matching"))
	assert_str(scene.variant).is_equal("market_cards")
	assert_str(String((scene.find_child("Stage", true, false) as PanelContainer).theme_type_variation)).is_equal("QuestCard")
	assert_object(scene.get_script()).is_same(script_before)


func test_unknown_variant_keeps_the_default_look() -> void:
	var data: Dictionary = MiniGameFixtures.session("ordering")
	((data["config"] as Dictionary)["presentation"] as Dictionary)["scene_variant"] = "ordering.rocket_9000"
	var scene: MiniGameQuestionScene = auto_free(MiniGameRegistry.instantiate("ordering")) as MiniGameQuestionScene
	add_child(scene)
	var model: MiniGameSessionModel = MiniGameSessionModel.from_dict(data)
	scene.setup(model.config, model)
	assert_str(scene.variant).is_equal("")
	assert_str(String((scene.find_child("Stage", true, false) as PanelContainer).theme_type_variation)).is_equal("")


func test_host_picks_the_scene_of_the_type() -> void:
	var host: MiniGameHost = await _open_host(MiniGameFixtures.session("matching"))
	assert_object(host.scene).is_not_null()
	assert_str(host.scene.scene_file_path).is_equal(MiniGameRegistry.scene_path("matching"))
	assert_int(host.state).is_equal(MiniGameHost.State.PLAYING)
	var rec: FakeHttp.Recorded = h.http.last(MiniGameFixtures.ROUTE_START)
	assert_str(rec.path).is_equal("/game/minigames/%s:start" % MiniGameFixtures.CONFIG_ID)
	assert_dict(rec.body).is_equal({"context_type": "free_play"})
	assert_bool(Uuid.is_v7(String(rec.headers.get("idempotency-key", "")))).is_true()


func test_unknown_type_shows_update_required_with_telemetry_and_no_crash() -> void:
	var data: Dictionary = MiniGameFixtures.session("matching")
	data["minigame_type"] = "space_racer"
	var host: MiniGameHost = await _open_host(data)
	assert_int(host.state).is_equal(MiniGameHost.State.UPDATE_REQUIRED)
	assert_object(host.scene).is_null()
	assert_object(host.update_view).is_not_null()
	assert_str((host.update_view.get_node("%Body") as Label).text).is_equal("ui.activity_player.update_required")
	assert_array(_telemetry()).contains([[LearningService.TELEMETRY_MINIGAME_UPDATE_REQUIRED, "space_racer"]])
	(host.update_view.get_node("%Back") as Button).pressed.emit()
	assert_bool(host.visible).is_false()
	assert_bool(QualityManager.suspend_reasons.has(&"minigame")).is_false()


func test_minigame_activity_type_is_registered_in_the_activity_player() -> void:
	assert_str(ActivityRendererRegistry.check("minigame", "1.2.3")).is_equal(ActivityRendererRegistry.REASON_OK)


func test_unknown_type_inside_the_activity_player_shows_its_update_screen() -> void:
	var activity_host: Control = auto_free((load(ACTIVITY_HOST_SCENE) as PackedScene).instantiate()) as Control
	activity_host.set("services", services)
	activity_host.set("client_version", "1.2.3")
	add_child(activity_host)
	var data: Dictionary = MiniGameFixtures.attempt("matching")
	(((data["render"] as Dictionary)["render_payload"]) as Dictionary)["minigame_type"] = "space_racer"
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await activity_host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	await await_idle_frame()
	assert_int(int(activity_host.get("state"))).is_equal(9)
	assert_object(activity_host.get("update_view")).is_not_null()
	assert_array(_telemetry()).contains([[LearningService.TELEMETRY_MINIGAME_UPDATE_REQUIRED, "space_racer"]])
	assert_array(_telemetry()).contains([[LearningService.TELEMETRY_UPDATE_REQUIRED, "minigame"]])
