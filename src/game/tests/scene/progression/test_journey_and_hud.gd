extends GdUnitTestSuite
## F069 "Hành trình của tôi" and the HUD progression (T012, T015; F069-AC-01, AC-02, AC-05, AC-27): the HUD shows the
## level, title, XP bar and Xu from `getProgression` / `getWallet`, a message updates it in the same frame without
## polling (moon at the daily cap, "Cấp tối đa" at the end), the HUD widgets have no `_process`; the journey lists the
## XP lines with friendly labels and no technical id, pages with "Xem thêm" (cursor) and shows the milestones.

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const JOURNEY: String = "res://scenes/progression/JourneyScreen.tscn"
const UUID_PATTERN: String = "[0-9a-f]{8}-[0-9a-f]{4}-"

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func test_hud_shows_progression_and_wallet_from_the_server() -> void:
	await s.services.progression.refresh()
	await s.services.inventory.load_wallet()
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(3)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var bar: XpBar = hud.call("xp_bar") as XpBar
	assert_bool(bar.visible).is_true()
	assert_bool((hud.find_child("XpPanel", true, false) as Control).visible).is_true()
	assert_str(bar.level_text()).is_equal("%s · %s" % [tr("ui.progression.level").format({"level": "3"}), "Nhà thám hiểm nhí"])
	assert_str(bar.numbers_text()).is_equal(tr("ui.progression.xp_numbers").format({"xp": "40", "total": "120"}))
	assert_float(bar.bar_value()).is_equal_approx(40.0 / 120.0, 0.001)
	var chip: CurrencyChip = hud.call("currency_chip") as CurrencyChip
	assert_bool(chip.visible).is_true()
	assert_str(chip.text).is_equal(tr("ui.reward.currency").format({"amount": "150"}))
	assert_float(chip.get_combined_minimum_size().y).is_greater_equal(64.0)


func test_message_updates_the_hud_in_the_same_frame_without_polling() -> void:
	await s.services.progression.refresh()
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var bar: XpBar = hud.call("xp_bar") as XpBar
	var gets: int = s.h.http.count(F069Fixtures.ROUTE_PROGRESSION)
	s.services.progression.handle_message(ProgressionService.MSG_XP_UPDATED, F069Fixtures.xp_update(25, 65, 55, true))
	assert_str(bar.numbers_text()).is_equal(tr("ui.progression.xp_numbers").format({"xp": "65", "total": "120"}))
	assert_bool(bar.moon_visible()).is_true()
	assert_str((bar.find_child("Fly", true, false) as Label).text).is_equal(tr("ui.progression.xp_gained").format({"xp": "25"}))
	await runner.simulate_frames(3)
	assert_int(s.h.http.count(F069Fixtures.ROUTE_PROGRESSION)).is_equal(gets)
	s.h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(50, 0, null))
	await s.services.progression.refresh()
	assert_str(bar.numbers_text()).is_equal(tr("ui.progression.max_level"))
	assert_float(bar.bar_value()).is_equal(1.0)


func test_hud_widgets_never_poll() -> void:
	for path: String in ["res://scenes/ui/hud/xp_bar.gd", "res://scenes/ui/hud/currency_chip.gd",
			"res://scenes/ui/hud/main_hud.gd"]:
		var text: String = FileAccess.get_file_as_string(path)
		assert_bool(text.contains("func _process(")).override_failure_message(path).is_false()


func test_journey_lists_friendly_lines_and_pages_by_cursor() -> void:
	var runner: GdUnitSceneRunner = scene_runner(JOURNEY)
	await runner.simulate_frames(1)
	var journey: Control = runner.scene() as Control
	journey.call("open", {})
	await runner.simulate_frames(8)
	var rows: PackedStringArray = journey.call("row_texts") as PackedStringArray
	assert_array(Array(rows)).contains([tr("ui.journey.source.activity.completed"), tr("ui.journey.source.quest.side_completed"),
		"Bài học vui"])
	var re: RegEx = RegEx.create_from_string(UUID_PATTERN)
	for text: String in rows:
		assert_object(re.search(text)).override_failure_message(text).is_null()
	for text: String in rows:
		assert_str(text.to_lower()).not_contains("điểm số")
	assert_bool((journey.find_child("More", true, false) as Button).visible).is_true()
	s.h.http.reply(F069Fixtures.ROUTE_HISTORY, 200, F069Fixtures.history(2))
	await journey.call("load_more")
	assert_str(s.h.http.last(F069Fixtures.ROUTE_HISTORY).query).contains("cursor=cursor-2")
	assert_int((journey.get("entries") as Array).size()).is_equal(6)
	assert_bool((journey.find_child("More", true, false) as Button).visible).is_false()
	assert_int((journey.find_child("Levels", true, false) as Node).get_child_count()).is_equal(5)
