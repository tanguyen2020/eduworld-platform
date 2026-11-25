extends GdUnitTestSuite
## F068-AC-08 (open decision F067-3; PERFORMANCE_BUDGET §4, §5): characters get a soft blob shadow where the sun casts
## no real-time shadow — the Compatibility renderer (Web) or a preset without shadows — and none where it does; its
## colour/opacity are tokens; on the local player it stays on the floor during a jump and shrinks with height.

const RIG: String = "res://scenes/character/AvatarRig.tscn"
const NPC: String = "res://scenes/npc/NPC.tscn"
const PLAYER: String = "res://scenes/character/Player.tscn"


func test_visible_only_without_realtime_shadows() -> void:
	for preset: String in QualityPolicy.PRESET_NAMES:
		var sun: bool = bool(QualityPolicy.preset_value(preset, "shadows"))
		assert_bool(QualityPolicy.blob_shadow_visible(QualityPolicy.RENDERER_COMPATIBILITY, sun)).is_true()
		# Mobile: only where the preset (Low) has no sun shadows.
		assert_bool(QualityPolicy.blob_shadow_visible(QualityPolicy.RENDERER_MOBILE, sun)).is_equal(preset == QualityPolicy.LOW)
	# The student turning sun shadows off brings the blob back.
	assert_bool(QualityPolicy.blob_shadow_visible(QualityPolicy.RENDERER_MOBILE, false)).is_true()


func test_material_comes_from_the_tokens() -> void:
	var material: ShaderMaterial = BlobShadow.shared_material()
	assert_object(material.get_shader_parameter(&"shadow_color")).is_equal(Art3dTokens.LIGHT_BLOB_SHADOW_COLOR)
	assert_float(float(material.get_shader_parameter(&"opacity"))).is_equal_approx(Art3dTokens.LIGHT_BLOB_SHADOW_OPACITY, 0.0001)
	var code: String = FileAccess.get_file_as_string(BlobShadow.SHADER.resource_path)
	assert_bool(code.contains("source_color =")).is_false()
	assert_bool(code.contains("SCREEN_TEXTURE") or code.contains("hint_depth_texture")).is_false()


func test_shrinks_with_height() -> void:
	assert_float(BlobShadow.scale_for_height(0.0, 2.0)).is_equal(1.0)
	assert_float(BlobShadow.scale_for_height(2.0, 2.0)).is_equal_approx(0.5, 0.001)
	assert_float(BlobShadow.scale_for_height(100.0, 2.0)).is_equal(0.3)
	assert_float(BlobShadow.scale_for_height(-1.0, 2.0)).is_equal(1.0)


func test_every_character_carries_one() -> void:
	for path: String in [RIG, NPC]:
		var node: Node = auto_free((load(path) as PackedScene).instantiate()) as Node
		add_child(node)
		var shadows: Array[Node] = node.find_children("*", "MeshInstance3D", true, false).filter(
				func(n: Node) -> bool: return n is BlobShadow)
		assert_int(shadows.size()).override_failure_message(path).is_equal(1)
		var blob: BlobShadow = shadows[0] as BlobShadow
		assert_bool(blob.is_in_group(BlobShadow.GROUP)).is_true()
		assert_bool(blob.visible).is_equal(QualityPolicy.blob_shadow_visible(RenderingServer.get_current_rendering_method(),
				QualityManager.shadows_enabled()))
	var runner: GdUnitSceneRunner = scene_runner(PLAYER)
	await runner.simulate_frames(1)
	assert_bool((runner.scene().get_node("AvatarRig/BlobShadow") as BlobShadow).snap_to_ground).is_true()


func test_zone_quality_toggles_the_shadows() -> void:
	var rig: Node3D = auto_free((load(RIG) as PackedScene).instantiate()) as Node3D
	add_child(rig)
	var blob: BlobShadow = rig.get_node("BlobShadow") as BlobShadow
	blob.visible = false
	ZoneArtQuality.apply(rig, QualityManager.preset)
	assert_bool(blob.visible).is_equal(QualityPolicy.blob_shadow_visible(RenderingServer.get_current_rendering_method(),
			QualityManager.shadows_enabled()))
