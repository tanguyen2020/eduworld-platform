extends GdUnitTestSuite
## F069 S07 LevelUpModal (T011; F069-AC-03, AC-04; STUDENT_GAME_UX_CORE S07 AC-UX-0701…0704): the heading says the level
## or "Lên {count} cấp!", the rewards are the server's with no "Nhận" button, one tap skips the effect, it closes by
## itself after 10 s, the focus starts on "Tuyệt!", and the buttons follow the grade band (g1_3 only "Tuyệt!", g4_5 +
## "Trang bị ngay", g6_9 + XP left and "Hành trình của tôi"). Inside WorldRoot a level-up during a challenge waits for
## the end of the run.

const MODAL: String = "res://scenes/progression/LevelUpModal.tscn"
## S07 §9: the modal closes itself after 10 s.
const AUTO_CLOSE_SEC: float = 10.0

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func _modal() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(MODAL)
	await runner.simulate_frames(2)
	return [runner, runner.scene() as Control]


func _batch(from_level: int, to_level: int, item: bool = true) -> LevelUpModel:
	var list: Array[LevelUpModel] = []
	for level: int in range(from_level, to_level):
		list.append(LevelUpModel.from_dict(F069Fixtures.level_up(level, level + 1, item and level == from_level)))
	return LevelUpModel.merge(list)


func test_heading_rewards_and_no_claim_button() -> void:
	var r: Array = await _modal()
	var modal: Control = r[1] as Control
	modal.call("present", _batch(3, 5), ProgressionModel.from_dict(F069Fixtures.progression(5, 20, 140)), "g1_3")
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	assert_str(String(modal.call("heading_text"))).is_equal(tr("ui.levelup.multi").format({"count": "2"}))
	var row: RewardRow = modal.find_child("Rewards", true, false) as RewardRow
	assert_array(Array(row.chip_texts())).contains([tr("ui.reward.currency").format({"amount": "100"})])
	for b: Node in modal.find_children("*", "Button", true, false):
		var text: String = tr((b as Button).text).to_lower()
		assert_str(text).not_contains("nhận")
		assert_str(text).not_contains("claim")


func test_buttons_follow_the_grade_band() -> void:
	var r: Array = await _modal()
	var modal: Control = r[1] as Control
	var progression: ProgressionModel = ProgressionModel.from_dict(F069Fixtures.progression(4, 20, 140))
	modal.call("present", _batch(3, 4), progression, "g1_3")
	assert_array(Array(modal.call("buttons_shown"))).contains_exactly(["Ok"])
	modal.call("present", _batch(3, 4), progression, "g4_5")
	assert_array(Array(modal.call("buttons_shown"))).contains_exactly(["EquipNow", "Ok"])
	modal.call("present", _batch(3, 4, false), progression, "g4_5")
	assert_array(Array(modal.call("buttons_shown"))).contains_exactly(["Ok"])
	modal.call("present", _batch(3, 4), progression, "g6_7")
	assert_array(Array(modal.call("buttons_shown"))).contains_exactly(["Journey", "EquipNow", "Ok"])
	var xp: Label = modal.find_child("XpLeft", true, false) as Label
	assert_bool(xp.visible).is_true()
	assert_str(xp.text).is_equal(tr("ui.levelup.xp_to_next").format({"xp": "140", "next": "5"}))


func test_focus_skip_and_auto_close() -> void:
	var r: Array = await _modal()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var modal: Control = r[1] as Control
	var closed: Array[bool] = []
	modal.connect("closed", func() -> void: closed.append(true))
	Settings.reduce_motion = false
	modal.call("present", _batch(3, 4), null, "g1_3")
	await runner.simulate_frames(2)
	assert_object(modal.get_viewport().gui_get_focus_owner()).is_same(modal.find_child("Ok", true, false))
	assert_bool(bool(modal.get("effect_running"))).is_true()
	var tap: InputEventMouseButton = InputEventMouseButton.new()
	tap.button_index = MOUSE_BUTTON_LEFT
	tap.pressed = true
	modal.call("_gui_input", tap)
	assert_bool(bool(modal.get("effect_running"))).is_false()
	modal.call("tick", AUTO_CLOSE_SEC - 0.5)
	assert_bool(modal.visible).is_true()
	modal.call("tick", 1.0)
	assert_bool(modal.visible).is_false()
	assert_array(closed).has_size(1)


func test_reduced_motion_is_a_static_card() -> void:
	var r: Array = await _modal()
	var modal: Control = r[1] as Control
	Settings.reduce_motion = true
	modal.call("present", _batch(3, 4), null, "g1_3")
	assert_bool(bool(modal.get("effect_running"))).is_false()
	Settings.reduce_motion = false


func test_world_holds_the_modal_during_a_challenge_and_shows_it_after() -> void:
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/world/WorldRoot.tscn")
	await runner.simulate_frames(10)
	var world: Node = runner.scene()
	var modal: Control = world.find_child("LevelUpModal", true, false) as Control
	AppState.begin_blocking(&"challenge")
	s.services.progression.handle_message(ProgressionService.MSG_LEVEL_REACHED, F069Fixtures.level_up(3, 4))
	s.services.progression.handle_message(ProgressionService.MSG_LEVEL_REACHED, F069Fixtures.level_up(4, 5))
	await runner.simulate_frames(3)
	assert_bool(modal.visible).is_false()
	AppState.end_blocking(&"challenge")
	await runner.simulate_frames(3)
	assert_bool(modal.visible).is_true()
	assert_str(String(modal.call("heading_text"))).is_equal(tr("ui.levelup.multi").format({"count": "2"}))
	modal.call("close")
	assert_object(s.services.progression.queue.current).is_null()
