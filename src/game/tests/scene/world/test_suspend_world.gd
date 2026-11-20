extends GdUnitTestSuite
## F062-AC-18 (NFR-COMP-001, STUDENT_GAME_UX_LEARNING §2.1, CONFIG_DEFAULTS `battery_target`, TC-PERF-020; T024): a
## full-screen learning screen suspends the world — physics/animation stop, 3D rendering is replaced by a blurred
## snapshot — until every reason is resumed; an idle screen (> 10 s without input) renders at 15 FPS.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"

var _max_fps: int = 0


func before_test() -> void:
	_max_fps = Engine.max_fps
	QualityManager.apply_engine_fps = true
	AppState.spawn_point_id = ""


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	QualityManager.touch_activity()
	QualityManager.apply_engine_fps = false
	Engine.max_fps = _max_fps


func test_suspend_stops_the_world_and_shows_a_snapshot_until_resumed() -> void:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(2)
	var zone: ZoneRoot = runner.scene() as ZoneRoot
	var events: Array[bool] = []
	var on_suspended: Callable = func(suspended: bool) -> void: events.append(suspended)
	QualityManager.world_suspended.connect(on_suspended)
	QualityManager.suspend_world(&"activity_host")
	QualityManager.suspend_world(&"exam_lobby")
	assert_bool(QualityManager.is_world_suspended()).is_true()
	assert_int(zone.process_mode).is_equal(Node.PROCESS_MODE_DISABLED)
	assert_bool(zone.player.can_process()).is_false()
	assert_bool(get_viewport().disable_3d).is_true()
	assert_object(QualityManager.snapshot_layer).is_not_null()
	assert_int(QualityManager.snapshot_layer.layer).is_less(1)
	# Physics stopped: the player does not fall while suspended.
	var y: float = zone.player.global_position.y
	zone.player.global_position.y = 50.0
	await runner.simulate_frames(5)
	assert_float(zone.player.global_position.y).is_equal_approx(50.0, 0.001)
	zone.player.global_position.y = y
	QualityManager.resume_world(&"activity_host")
	assert_bool(QualityManager.is_world_suspended()).is_true()
	QualityManager.resume_world(&"exam_lobby")
	assert_bool(QualityManager.is_world_suspended()).is_false()
	assert_int(zone.process_mode).is_equal(Node.PROCESS_MODE_INHERIT)
	assert_bool(get_viewport().disable_3d).is_false()
	assert_object(QualityManager.snapshot_layer).is_null()
	QualityManager.world_suspended.disconnect(on_suspended)
	assert_array(events).is_equal([true, false])


func test_idle_screen_renders_at_fifteen_fps() -> void:
	QualityManager.idle_sec = QualityPolicy.IDLE_AFTER_SEC + 1.0
	QualityManager.touch_activity()
	QualityManager.tick(0.1)
	var active_cap: int = Engine.max_fps
	assert_int(active_cap).is_greater(QualityPolicy.IDLE_FPS)
	for i: int in 11:
		QualityManager.tick(1.0)
	assert_int(Engine.max_fps).is_equal(QualityPolicy.IDLE_FPS)
	var key: InputEventKey = InputEventKey.new()
	key.physical_keycode = KEY_W
	key.pressed = true
	get_tree().root.push_input(key)
	await get_tree().process_frame
	assert_int(Engine.max_fps).is_equal(active_cap)
	key.pressed = false
	get_tree().root.push_input(key)
