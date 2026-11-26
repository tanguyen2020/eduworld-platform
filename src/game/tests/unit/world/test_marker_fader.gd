extends GdUnitTestSuite
## F097-AC-11 (F096-4, owner order "chốt hết"; WORLD_RENDERING §5.1, CONFIG_DEFAULTS `marker_near_fade_m`): a world
## marker nearer to the camera than `eduworld/world/marker_near_fade_m` and nearer than the player is hidden (it would
## stand between the camera and the player); it shows again past the distance plus the hysteresis; a marker beside or
## beyond the player stays visible; the seed object never moves. Plus the Hub spawn case (WorldAnchor of
## `quest.hub.welcome_sign` in front of the camera).


func _marker(at: Vector3) -> Node3D:
	var m: Node3D = auto_free(Node3D.new()) as Node3D
	m.add_to_group(MarkerFader.GROUP)
	add_child(m)
	m.global_position = at
	return m


func _fader() -> MarkerFader:
	var f: MarkerFader = auto_free(MarkerFader.new()) as MarkerFader
	add_child(f)
	return f


func test_distance_is_a_project_setting_with_the_documented_default() -> void:
	assert_float(float(ProjectSettings.get_setting(MarkerFader.SETTING, -1.0))).is_equal(MarkerFader.DEFAULT_NEAR_M)
	assert_float(_fader().near_m).is_equal(5.0)


func test_marker_between_camera_and_player_is_hidden_and_comes_back() -> void:
	var fader: MarkerFader = _fader()
	var marker: Node3D = _marker(Vector3(0.0, 0.0, -3.0))
	var start: Vector3 = marker.global_position
	fader.refresh(Vector3.ZERO, 6.0)
	assert_bool(marker.visible).is_false()
	# Within the hysteresis band it stays hidden (no flicker).
	fader.refresh(Vector3(0.0, 0.0, 2.2), 8.0)
	assert_bool(marker.visible).is_false()
	fader.refresh(Vector3(0.0, 0.0, 3.0), 9.0)
	assert_bool(marker.visible).is_true()
	assert_vector(marker.global_position).is_equal(start)


func test_marker_beside_or_beyond_the_player_stays_visible() -> void:
	var fader: MarkerFader = _fader()
	var beyond: Node3D = _marker(Vector3(0.0, 0.0, -4.0))
	fader.refresh(Vector3.ZERO, 3.0)
	assert_bool(beyond.visible).is_true()
	var far: Node3D = _marker(Vector3(0.0, 0.0, -8.0))
	fader.refresh(Vector3.ZERO, 12.0)
	assert_bool(far.visible).is_true()


func test_rule_is_near_and_in_front_of_the_player() -> void:
	var fader: MarkerFader = _fader()
	assert_bool(fader.should_hide(2.0, 6.0)).is_true()
	assert_bool(fader.should_hide(4.9, 6.0)).is_true()
	assert_bool(fader.should_hide(5.1, 6.0)).is_false()
	assert_bool(fader.should_hide(4.0, 3.0)).is_false()


func test_world_anchor_marker_and_quest_marker_are_in_the_fade_group() -> void:
	var anchor: Node = auto_free((load("res://scenes/world/WorldAnchor.tscn") as PackedScene).instantiate())
	assert_bool(anchor.get_node("Marker").is_in_group(MarkerFader.GROUP)).is_true()
	var quest: Node = auto_free((load("res://scenes/quest/QuestMarker3D.tscn") as PackedScene).instantiate())
	assert_bool(quest.is_in_group(MarkerFader.GROUP)).is_true()


func test_hub_spawn_welcome_sign_is_hidden_from_the_spawn_camera() -> void:
	var zone: Node3D = auto_free((load("res://scenes/world/zones/main_hub/Zone.tscn") as PackedScene).instantiate()) as Node3D
	zone.set("interactive", false)
	add_child(zone)
	var sign: Node3D = zone.find_child("QuestHubWelcomeSign", true, false) as Node3D
	var spawn: Node3D = zone.find_child("Default", true, false) as Node3D
	var marker: Node3D = sign.get_node("Marker") as Node3D
	# Camera of Player.tscn at spawn: behind the player (+z), raised, about one spring arm away (F096-AC-07).
	var player_pos: Vector3 = spawn.global_position + Vector3(0.0, 1.0, 0.0)
	var camera_pos: Vector3 = player_pos + Vector3(0.0, 2.0, 5.6)
	var fader: MarkerFader = zone.find_child("MarkerFader", true, false) as MarkerFader
	if fader == null:
		fader = _fader()
	fader.refresh(camera_pos, player_pos.distance_to(camera_pos))
	assert_bool(marker.visible).is_false()
	assert_vector(sign.global_position).is_equal(Vector3(0.0, 0.0, 12.0))
