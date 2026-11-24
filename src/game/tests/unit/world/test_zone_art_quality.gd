extends GdUnitTestSuite
## F067-AC-06 (PERFORMANCE_BUDGET §5, CONFIG_DEFAULTS `quality_presets`): the art follows the quality preset — toon
## outline fades at 12 m (Low) / 30 m (Medium) / never (High), grass and decor are drawn at 25 % / 60 % / 100 % up to
## the preset view distance; the Compatibility renderer (Web) draws the toon look without real-time shadows.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"


func after_test() -> void:
	RenderingServer.global_shader_parameter_set(ZoneArtQuality.OUTLINE_PARAMETER, ZoneArtQuality.outline_distance(QualityManager.preset))


func test_preset_values() -> void:
	assert_float(ZoneArtQuality.outline_distance("low")).is_equal(12.0)
	assert_float(ZoneArtQuality.outline_distance("medium")).is_equal(30.0)
	assert_float(ZoneArtQuality.outline_distance("high")).is_equal(0.0)
	assert_float(ZoneArtQuality.decor_density("low")).is_equal(0.25)
	assert_float(ZoneArtQuality.decor_density("medium")).is_equal(0.6)
	assert_float(ZoneArtQuality.decor_density("high")).is_equal(1.0)
	assert_float(ZoneArtQuality.decor_density("unknown")).is_equal(0.6)


func test_visible_count() -> void:
	assert_int(ZoneArtQuality.visible_count(200, 0.25)).is_equal(50)
	assert_int(ZoneArtQuality.visible_count(3, 0.1)).is_equal(1)
	assert_int(ZoneArtQuality.visible_count(0, 1.0)).is_equal(0)
	assert_int(ZoneArtQuality.visible_count(10, 1.0)).is_equal(10)


func test_realtime_shadows_are_off_on_the_compatibility_renderer() -> void:
	assert_bool(QualityPolicy.shadows_supported(QualityPolicy.RENDERER_COMPATIBILITY)).is_false()
	assert_bool(QualityPolicy.shadows_supported(QualityPolicy.RENDERER_MOBILE)).is_true()
	assert_bool(QualityPolicy.shadows_supported(QualityPolicy.RENDERER_FORWARD_PLUS)).is_true()


func test_hub_decor_follows_each_preset() -> void:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(1)
	var zone: Node = runner.scene()
	var decor: Array[MultiMeshInstance3D] = []
	for node: Node in zone.find_children("*", "MultiMeshInstance3D", true, false):
		if node.is_in_group(ZoneArtQuality.DECOR_GROUP):
			decor.append(node as MultiMeshInstance3D)
	assert_int(decor.size()).is_greater_equal(3)
	for preset: String in ["low", "medium", "high"]:
		ZoneArtQuality.apply(zone, preset)
		for d: MultiMeshInstance3D in decor:
			var expected: int = ZoneArtQuality.visible_count(d.multimesh.instance_count, ZoneArtQuality.decor_density(preset))
			assert_int(d.multimesh.visible_instance_count).override_failure_message("%s %s" % [preset, d.name]).is_equal(expected)
			assert_float(d.visibility_range_end).is_equal(float(QualityPolicy.preset_value(preset, "view_distance")))
	var trees: MultiMeshInstance3D = zone.get_node("Ambient/Trees") as MultiMeshInstance3D
	assert_bool(trees.is_in_group(ZoneArtQuality.DECOR_GROUP)).is_false()
	assert_int(trees.multimesh.visible_instance_count).is_equal(-1)
