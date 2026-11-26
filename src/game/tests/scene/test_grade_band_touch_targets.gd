extends GdUnitTestSuite
## F096-AC-18 (STUDENT_GAME_UX §4.3, AC-UX-0005, NFR-A11Y-001): on the screens F096 touched, every visible button is at
## least as tall as the touch target of the grade band (UiTokens density; 64 dp for g1_3) in all four bands — the
## HUD with Mây's guide and the "Việc tiếp theo" card, the World Map (picture board and rows), Cài đặt with the
## Licences page, and the result screen.

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const SETTINGS: String = "res://scenes/ui/settings/SettingsPanel.tscn"
const MAP: String = "res://scenes/world/WorldMap.tscn"
const RESULT: String = "res://scenes/learning/ActivityResult.tscn"

var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	AppState.start_ui(get_tree().root)


func after_test() -> void:
	AppState.set_grade_band(_band)


func _too_small(root: Node, band: String) -> PackedStringArray:
	var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
	var out: PackedStringArray = []
	for node: Node in root.find_children("*", "BaseButton", true, false):
		var button: Control = node as Control
		if not button.is_visible_in_tree():
			continue
		var height: float = maxf(button.get_combined_minimum_size().y, button.size.y)
		if height + 0.5 < minimum:
			out.append("%s %s: %.0f < %.0f" % [band, button.get_path(), height, minimum])
	return out


func test_every_button_meets_the_band_touch_target() -> void:
	var failures: PackedStringArray = []
	var hud: CanvasLayer = auto_free((load(HUD) as PackedScene).instantiate()) as CanvasLayer
	add_child(hud)
	var settings: Control = auto_free((load(SETTINGS) as PackedScene).instantiate()) as Control
	add_child(settings)
	var map: Control = auto_free((load(MAP) as PackedScene).instantiate()) as Control
	add_child(map)
	var result: VBoxContainer = auto_free((load(RESULT) as PackedScene).instantiate()) as VBoxContainer
	add_child(result)
	for band: String in UiTokens.DENSITY_BANDS:
		AppState.set_grade_band(band)
		hud.call("start_onboarding")
		settings.call("open")
		map.show()
		map.call("render", WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), band, "main_hub"))
		result.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), band,
				[{"kind": "xp", "amount": 10}])
		await await_idle_frame()
		await await_idle_frame()
		for root: Node in [hud, settings, map, result]:
			failures.append_array(_too_small(root, band))
		settings.call("show_licenses")
		await await_idle_frame()
		failures.append_array(_too_small(settings, band))
		settings.call("show_main")
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


## F096-AC-12: list rows of the World Map keep a readable column for the conditions and a labelled "Đến đây" button at
## the smallest reference window (1024×600 → logical 1280×750) — no one-character column, no empty button.
func test_world_map_rows_do_not_squeeze_columns() -> void:
	var map: Control = auto_free((load(MAP) as PackedScene).instantiate()) as Control
	add_child(map)
	map.size = Vector2(1280, 750)
	map.show()
	map.call("render", WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), "g4_5", "main_hub"))
	await await_idle_frame()
	await await_idle_frame()
	for row: Node in map.find_children("Row_*", "HBoxContainer", true, false):
		var state: Label = row.find_child("State", true, false) as Label
		assert_float(state.size.x).override_failure_message(String(row.name)).is_greater(float(UiTokens.SIZE_LAYOUT_200))
		var go: Button = row.find_child("Travel", true, false) as Button
		if go != null:
			var font: Font = go.get_theme_font(&"font")
			var needed: float = font.get_string_size(go.text, HORIZONTAL_ALIGNMENT_LEFT, -1.0,
					go.get_theme_font_size(&"font_size")).x
			assert_float(go.size.x).override_failure_message(String(row.name)).is_greater_equal(needed)
