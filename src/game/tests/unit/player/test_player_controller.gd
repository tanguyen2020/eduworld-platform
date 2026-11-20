extends GdUnitTestSuite
## F062-AC-02, F062-AC-03 (FR-GAME-003, FR-GAME-010, STUDENT_GAME_UX §4; T008): movement is relative to the camera
## (keyboard runs, the joystick scales between walk and run), jump only from the floor, tap-to-move only for `g1_3`,
## and one AnimationTree state machine picks idle/walk/run/jump from the velocity alone.

const PLAYER: String = "res://scenes/character/Player.tscn"

var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band


func after_test() -> void:
	AppState.set_grade_band(_band)
	for action: String in ["move_forward", "move_back", "move_left", "move_right"]:
		Input.action_release(action)


func _player() -> PlayerController:
	var runner: GdUnitSceneRunner = scene_runner(PLAYER)
	await runner.simulate_frames(1)
	return runner.scene() as PlayerController


func test_velocity_is_relative_to_the_camera_and_scaled_by_the_stick() -> void:
	var p: PlayerController = await _player()
	var forward: Vector3 = p.desired_velocity(Vector2(0.0, 1.0), 0.0)
	assert_vector(forward).is_equal_approx(Vector3(0.0, 0.0, -p.run_speed), Vector3.ONE * 0.001)
	var turned: Vector3 = p.desired_velocity(Vector2(0.0, 1.0), PI / 2.0)
	assert_vector(turned).is_equal_approx(Vector3(-p.run_speed, 0.0, 0.0), Vector3.ONE * 0.001)
	var right: Vector3 = p.desired_velocity(Vector2(1.0, 0.0), 0.0)
	assert_vector(right).is_equal_approx(Vector3(p.run_speed, 0.0, 0.0), Vector3.ONE * 0.001)
	assert_float(p.desired_velocity(Vector2(0.0, 0.4), 0.0).length()).is_equal_approx(p.walk_speed, 0.001)
	assert_vector(p.desired_velocity(Vector2.ZERO, 0.0)).is_equal(Vector3.ZERO)


func test_keyboard_actions_and_joystick_feed_the_input() -> void:
	var p: PlayerController = await _player()
	Input.action_press("move_forward")
	assert_vector(p.read_input()).is_equal_approx(Vector2(0.0, 1.0), Vector2.ONE * 0.001)
	Input.action_release("move_forward")
	p.set_move_input(Vector2(0.0, 2.0))
	assert_vector(p.read_input()).is_equal_approx(Vector2(0.0, 1.0), Vector2.ONE * 0.001)
	p.input_enabled = false
	assert_vector(p.read_input()).is_equal(Vector2.ZERO)
	for action: String in ["move_forward", "move_back", "move_left", "move_right", "jump", "interact", "open_journal",
			"camera_rotate", "camera_zoom_in", "camera_zoom_out"]:
		assert_bool(InputMap.has_action(action)).override_failure_message(action).is_true()
		assert_bool(InputMap.action_get_events(action).is_empty()).override_failure_message(action).is_false()


func test_animation_state_follows_velocity() -> void:
	assert_str(String(AnimDriver.state_for(Vector3.ZERO, true, 0.2, 3.5))).is_equal("idle")
	assert_str(String(AnimDriver.state_for(Vector3(1.0, 0.0, 1.0), true, 0.2, 3.5))).is_equal("walk")
	assert_str(String(AnimDriver.state_for(Vector3(0.0, 0.0, -5.0), true, 0.2, 3.5))).is_equal("run")
	assert_str(String(AnimDriver.state_for(Vector3(0.0, 4.0, 0.0), false, 0.2, 3.5))).is_equal("jump")
	# Vertical speed alone (landing on a slope) is not walking.
	assert_str(String(AnimDriver.state_for(Vector3(0.0, -3.0, 0.0), true, 0.2, 3.5))).is_equal("idle")


func test_one_animation_tree_state_machine_drives_the_avatar() -> void:
	var p: PlayerController = await _player()
	assert_int(p.find_children("*", "AnimationTree", true, false).size()).is_equal(1)
	var tree: AnimationTree = p.get_node("AnimationTree") as AnimationTree
	assert_bool(tree.active).is_true()
	var machine: AnimationNodeStateMachine = tree.tree_root as AnimationNodeStateMachine
	for s: StringName in AnimDriver.STATES:
		assert_bool(machine.has_node(s)).override_failure_message(String(s)).is_true()
	var driver: AnimDriver = p.anim_driver
	assert_str(String(driver.update(Vector3(0.0, 0.0, -5.0), true))).is_equal("run")
	assert_str(String(driver.update(Vector3(0.0, 2.0, 0.0), false))).is_equal("jump")
	assert_str(String(driver.update(Vector3.ZERO, true))).is_equal("idle")
	assert_object(driver.playback()).is_not_null()


func test_tap_to_move_only_for_g1_3() -> void:
	var p: PlayerController = await _player()
	var tap: TapToMove = p.tap_to_move
	AppState.set_grade_band("g4_5")
	assert_bool(tap.enabled).is_false()
	tap.set_target(Vector3(5.0, 0.0, 0.0))
	assert_bool(tap.has_target).is_false()
	AppState.set_grade_band("g1_3")
	assert_bool(tap.enabled).is_true()
	assert_bool(TapToMove.is_enabled_for("g1_3")).is_true()
	for band: String in ["g4_5", "g6_7", "g8_9"]:
		assert_bool(TapToMove.is_enabled_for(band)).is_false()
	p.global_position = Vector3.ZERO
	tap.set_target(Vector3(5.0, 0.0, 0.0))
	p.step(1.0 / 60.0)
	assert_float(p.velocity.x).is_equal_approx(p.walk_speed, 0.001)
	assert_float(p.velocity.z).is_equal_approx(0.0, 0.001)
	# Arrival clears the target; keyboard/joystick input cancels it.
	assert_vector(tap.direction_from(Vector3(5.0, 3.0, 0.1))).is_equal(Vector3.ZERO)
	assert_bool(tap.has_target).is_false()
	tap.set_target(Vector3(5.0, 0.0, 0.0))
	p.set_move_input(Vector2(0.0, 1.0))
	assert_bool(tap.has_target).is_false()


func test_jump_only_from_the_floor() -> void:
	var p: PlayerController = await _player()
	p.request_jump()
	p.step(1.0 / 60.0)
	# No floor under the test player: gravity applies, the jump request is dropped.
	assert_float(p.velocity.y).is_less(0.0)
