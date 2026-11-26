extends GdUnitTestSuite
## F071-AC-26, AC-27 (T042): World Map S04 from the server data — open/locked areas with the conditions in words, the
## exploration % from `getWorldExploration`, fast travel to open areas only, no option to open an area with coins;
## grade band g1_3: a picture board of the open areas without percentages, one tap travels.

const SCENE: String = "res://scenes/world/WorldMap.tscn"


func _model(band: String) -> WorldMapModel:
	return WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), band, "main_hub")


func _entry(m: WorldMapModel, id: String) -> Dictionary:
	for e: Dictionary in m.entries:
		if e["area_id"] == id:
			return e
	return {}


func test_list_mode_shows_percentages_and_conditions() -> void:
	var m: WorldMapModel = _model("g4_5")
	assert_bool(m.picture_mode).is_false()
	assert_int(m.entries.size()).is_equal(12)
	assert_float(float(_entry(m, "learning_zone.math")["pct"])).is_equal_approx(33.33, 0.001)
	var ocean: Dictionary = _entry(m, "quest_zone.discovery_ocean")
	assert_bool(ocean["accessible"]).is_false()
	assert_bool(ocean["can_travel"]).is_false()
	assert_str(WorldMapModel.requirement_text((ocean["requirements"] as Array)[0] as Dictionary, "en-US")).is_equal("Complete 1 subject chapter")
	assert_bool(_entry(m, "main_hub")["can_travel"]).is_false()
	assert_bool(_entry(m, "learning_zone.math")["can_travel"]).is_true()


func test_picture_board_for_grades_1_to_3() -> void:
	var m: WorldMapModel = _model("g1_3")
	assert_bool(m.picture_mode).is_true()
	for e: Dictionary in m.entries:
		assert_bool(e["accessible"]).is_true()
		assert_float(float(e["pct"])).is_equal(-1.0)
	assert_bool(_entry(m, "quest_zone.discovery_ocean").is_empty()).is_true()


func test_scene_renders_rows_and_travels() -> void:
	var map: Control = auto_free((load(SCENE) as PackedScene).instantiate()) as Control
	add_child(map)
	map.call("render", _model("g4_5"))
	var rows: Array[Node] = map.find_children("Row_*", "HBoxContainer", true, false)
	assert_int(rows.size()).is_equal(12)
	var travelled: Array[String] = []
	map.connect("travel_requested", func(id: String) -> void: travelled.append(id))
	var go: Button = map.find_child("Row_learning_zone_math", true, false).find_child("Travel", true, false) as Button
	go.pressed.emit()
	assert_array(travelled).contains_exactly(["learning_zone.math"])
	for b: Node in map.find_children("*", "Button", true, false):
		assert_str((b as Button).text.to_lower()).not_contains("xu")
	assert_object(map.find_child("Row_quest_zone_discovery_ocean", true, false).find_child("Travel", true, false)).is_null()


func test_scene_picture_board_one_tap() -> void:
	var map: Control = auto_free((load(SCENE) as PackedScene).instantiate()) as Control
	add_child(map)
	map.call("render", _model("g1_3"))
	var tiles: Array[Node] = map.find_children("Tile_*", "Button", true, false)
	assert_int(tiles.size()).is_equal(_model("g1_3").entries.size())
	# F096-AC-12: each picture tile shows the area icon and its short name (a blank square was unusable for g1_3).
	for t: Node in tiles:
		assert_str((t as Button).text).is_not_empty()
		assert_object((t as Button).icon).is_not_null()
		assert_float((t as Button).get_combined_minimum_size().y).is_greater_equal(float(UiTokens.DENSITY_G1_3_BUTTON_MIN_HEIGHT))
	assert_str((map.find_child("Tile_learning_zone_math", true, false) as Button).icon.resource_path).ends_with(
			"area_math_kingdom.svg")
	var travelled: Array[String] = []
	map.connect("travel_requested", func(id: String) -> void: travelled.append(id))
	(map.find_child("Tile_learning_zone_math", true, false) as Button).pressed.emit()
	assert_array(travelled).contains_exactly(["learning_zone.math"])
