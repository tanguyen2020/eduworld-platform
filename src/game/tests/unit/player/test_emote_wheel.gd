extends GdUnitTestSuite
## F070-AC-20 (T031, CHARACTER_SYSTEM §4, §7): the emote wheel shows only the equipped emotes
## (`avatar_config.emote_slots`, at most `emote_wheel_slots`); choosing one plays a one-shot clip on the character
## through `anim_driver.gd`, which then returns to Locomotion (also when the player moves). Opened by the input action
## `emote_wheel` or the touch button. Broadcasting to other players is F091/F092.

const PLAYER: String = "res://scenes/character/Player.tscn"
const WHEEL: String = "res://scenes/character/EmoteWheel.tscn"

var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band


func after_test() -> void:
	AppState.set_grade_band(_band)


func _setup() -> Array:
	var player: PlayerController = auto_free((load(PLAYER) as PackedScene).instantiate()) as PlayerController
	add_child(player)
	# No floor in this test: keep the body still so only the emote drives the animation state.
	player.set_physics_process(false)
	var wheel: EmoteWheel = auto_free((load(WHEEL) as PackedScene).instantiate()) as EmoteWheel
	add_child(wheel)
	wheel.anim_driver = player.anim_driver
	await get_tree().process_frame
	return [player, wheel]


func _equipped() -> PackedStringArray:
	return AvatarConfigModel.from_dict(ProfileFixtures.profile_ready()["avatar_config"]).emote_slots


func test_wheel_shows_only_the_equipped_emotes() -> void:
	var r: Array = await _setup()
	var wheel: EmoteWheel = r[1] as EmoteWheel
	wheel.set_emotes(_equipped())
	wheel.open()
	assert_bool(wheel.is_open()).is_true()
	assert_array(Array(wheel.shown_emotes())).contains_exactly(["emote_wave", "emote_clap", "emote_dance"])
	assert_int(wheel.slot_buttons().size()).is_equal(3)
	for b: Button in wheel.slot_buttons():
		assert_bool(b.visible).is_true()
		assert_str(b.text).is_not_empty()
		assert_bool(b.text.begins_with("ui.")).is_false()


func test_more_than_the_wheel_size_is_cut_and_blank_codes_are_ignored() -> void:
	var r: Array = await _setup()
	var wheel: EmoteWheel = r[1] as EmoteWheel
	wheel.set_emotes(PackedStringArray(["emote_wave", "", "emote_clap", "emote_dance", "emote_celebrate", "emote_thank",
			"emote_apologize", "emote_extra_07"]))
	assert_int(wheel.shown_emotes().size()).is_equal(AvatarConfigModel.MAX_EMOTE_SLOTS)
	assert_bool("" in wheel.shown_emotes()).is_false()


func test_choosing_an_emote_plays_a_one_shot_clip_then_returns_to_locomotion() -> void:
	var r: Array = await _setup()
	var player: PlayerController = r[0] as PlayerController
	var wheel: EmoteWheel = r[1] as EmoteWheel
	var driver: AnimDriver = player.anim_driver
	var started: Array[String] = []
	var finished: Array[String] = []
	driver.emote_started.connect(func(id: String) -> void: started.append(id))
	driver.emote_finished.connect(func(id: String) -> void: finished.append(id))
	wheel.set_emotes(_equipped())
	wheel.open()
	wheel.slot_buttons()[1].pressed.emit()
	assert_bool(wheel.is_open()).is_false()
	assert_str(String(driver.state)).is_equal(String(AnimDriver.EMOTE))
	assert_str(driver.emote_id).is_equal("emote_clap")
	assert_array(started).contains_exactly(["emote_clap"])
	var clip: Animation = player.find_child("AnimationPlayer", true, false).call("get_animation",
			"%s/%s" % [AnimDriver.LIBRARY, AnimDriver.EMOTE]) as Animation
	assert_object(clip).is_not_null()
	assert_int(clip.loop_mode).is_equal(Animation.LOOP_NONE)
	await get_tree().process_frame
	assert_str(String(driver.playback().get_current_node())).is_equal(String(AnimDriver.EMOTE))
	driver.advance_emote(driver.emote_time_left + 0.01)
	assert_str(String(driver.state)).is_equal(String(AnimDriver.IDLE))
	assert_array(finished).contains_exactly(["emote_clap"])
	assert_str(driver.emote_id).is_empty()


func test_moving_interrupts_the_emote() -> void:
	var r: Array = await _setup()
	var driver: AnimDriver = (r[0] as PlayerController).anim_driver
	assert_bool(driver.play_emote("emote_wave")).is_true()
	assert_str(String(driver.update(Vector3.ZERO, true))).is_equal(String(AnimDriver.EMOTE))
	assert_str(String(driver.update(Vector3(2.0, 0.0, 0.0), true))).is_equal(String(AnimDriver.WALK))
	assert_str(driver.emote_id).is_empty()
	assert_bool(driver.play_emote("")).is_false()


func test_input_action_and_touch_button_open_the_wheel() -> void:
	assert_bool(InputMap.has_action("emote_wheel")).is_true()
	var r: Array = await _setup()
	var wheel: EmoteWheel = r[1] as EmoteWheel
	wheel.set_emotes(_equipped())
	var press: InputEventAction = InputEventAction.new()
	press.action = "emote_wheel"
	press.pressed = true
	wheel._unhandled_input(press)
	assert_bool(wheel.is_open()).is_true()
	wheel._unhandled_input(press)
	assert_bool(wheel.is_open()).is_false()
	wheel.touch_button().pressed.emit()
	assert_bool(wheel.is_open()).is_true()


func test_touch_targets_follow_the_grade_band() -> void:
	AppState.set_grade_band("g1_3")
	var r: Array = await _setup()
	var wheel: EmoteWheel = r[1] as EmoteWheel
	wheel.set_emotes(_equipped())
	wheel.open()
	var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT["g1_3"])
	assert_float(wheel.touch_button().get_combined_minimum_size().y).is_greater_equal(minimum)
	for b: Button in wheel.slot_buttons():
		assert_float(b.get_combined_minimum_size().y).is_greater_equal(minimum)


func test_without_equipped_emotes_nothing_opens() -> void:
	var r: Array = await _setup()
	var wheel: EmoteWheel = r[1] as EmoteWheel
	wheel.set_emotes(PackedStringArray())
	wheel.open()
	assert_bool(wheel.is_open()).is_false()
	assert_bool(wheel.touch_button().disabled).is_true()
