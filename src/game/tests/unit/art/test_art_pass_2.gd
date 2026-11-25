extends GdUnitTestSuite
## F068 art pass 2 "chibi anime cartoon 3D" (ADR-0103; DESIGN_SYSTEM §5.3; CHARACTER_SYSTEM §5; WORLD_RENDERING §6.1):
## - AC-01: rounded world forms — smooth shading inside the crease angle, bevelled boxes, round primitives; the hub
##   meshes are smooth shaded;
## - AC-02: cel shading with clear token-driven bands and an ink outline (`toon.steps`, `toon.band_softness`,
##   `outline.color`), the same shader on Mobile and Compatibility;
## - AC-03: skinned characters use the `toon_skinned` preset (outline along the skinned normal);
## - AC-04: chibi proportions — head 1/3–1/2 of the height, short limbs;
## - AC-05: anime faces — eyes with white, iris, highlights, lash line, blush; one mesh per expression;
## - AC-06: the `kid_base` rig — body and outfits skinned to KidSkeleton; clips idle/walk/run/jump/wave/celebrate (and
##   the other emotes) key every bone; squash & stretch keeps the volume.

const ArtCatalog := preload("res://tools/art/art_catalog.gd")
const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const CharacterArt := preload("res://tools/art/character_art.gd")
const CharacterAnim := preload("res://tools/art/character_anim.gd")
const TOON_SHADER: String = "res://resources/art/shaders/toon.gdshader"


static func _slots(mesh: Mesh) -> Dictionary:
	var out: Dictionary = {}
	for uv: Vector2 in mesh.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]:
		if uv.y < 1.0:
			out[Art3dTokens.ART_PALETTE[int(uv.x)]] = true
	return out


## Share of the vertices whose shading normal differs from their face normal (0 = flat low-poly).
static func _smooth_share(mesh: ArrayMesh) -> float:
	var arrays: Array = mesh.surface_get_arrays(0)
	var verts: PackedVector3Array = arrays[Mesh.ARRAY_VERTEX]
	var normals: PackedVector3Array = arrays[Mesh.ARRAY_NORMAL]
	var indices: Variant = arrays[Mesh.ARRAY_INDEX]
	if indices != null:
		return 0.0
	var smooth: int = 0
	for i: int in range(0, verts.size(), 3):
		var face: Vector3 = (verts[i + 2] - verts[i]).cross(verts[i + 1] - verts[i]).normalized()
		for k: int in 3:
			if normals[i + k].dot(face) < 0.999:
				smooth += 1
	return float(smooth) / float(maxi(verts.size(), 1))


func test_builder_rounds_boxes_and_smooths_round_shapes() -> void:
	var m: LowPoly = LowPoly.new()
	m.box(Vector3.ZERO, Vector3.ONE, "wood")
	assert_int(m.triangle_count()).is_equal(42)  # 5 faces (no bottom) + 12 bevel strips + 8 corners
	var box: ArrayMesh = m.build()
	assert_float(_smooth_share(box)).is_greater(0.3)
	var flat: LowPoly = LowPoly.new()
	flat.set_flat()
	flat.box(Vector3.ZERO, Vector3.ONE, "wood")
	flat.blob(Vector3.ZERO, Vector3.ONE, "leaf", 1)
	assert_int(flat.triangle_count()).is_equal(10 + 80)  # sharp box without bottom + 80-face blob
	assert_float(_smooth_share(flat.build())).is_equal(0.0)
	var round: LowPoly = LowPoly.new()
	round.cylinder(Vector3.ZERO, 0.5, 0.5, 1.0, 4, "wood")
	# min_sides 8 sides, rounded rim: 8 × (side 2 + rim 2 + cap 1)
	assert_int(round.triangle_count()).is_equal(40)
	# A thin board stays sharp (no bevel below min_bevel_size).
	assert_float(m.bevel_for(Vector3(1, 0.02, 1))).is_equal(0.0)
	assert_float(m.bevel_for(Vector3.ONE)).is_equal(LowPoly.MAX_BEVEL)


func test_hub_world_meshes_are_smooth_shaded() -> void:
	for path: String in [ArtCatalog.MODELS_DIR + "/bld/bld_hub_set_a.res", ArtCatalog.MODELS_DIR + "/prop/prop_hub_set_a.res",
			ArtCatalog.spark_tree_path(3), ArtCatalog.veg_mesh_path("tree"), ArtCatalog.gate_path("arena")]:
		var mesh: ArrayMesh = load(path) as ArrayMesh
		assert_float(_smooth_share(mesh)).override_failure_message(path).is_greater(0.3)


func test_cel_bands_and_ink_outline_come_from_the_tokens() -> void:
	for key: String in ["toon", "toon_no_outline", "toon_skinned"]:
		var preset: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS[key])) as ShaderMaterial
		assert_str(preset.shader.resource_path).is_equal(TOON_SHADER)
		assert_float(float(preset.get_shader_parameter(&"band_softness"))).is_equal_approx(Art3dTokens.TOON_BAND_SOFTNESS, 0.0001)
		assert_float(float(preset.get_shader_parameter(&"toon_steps"))).is_equal(Art3dTokens.TOON_STEPS)
	# Crisp cel bands: a narrow band edge and at least shadow / lit steps.
	assert_float(Art3dTokens.TOON_BAND_SOFTNESS).is_less_equal(0.15)
	assert_float(Art3dTokens.TOON_STEPS).is_greater_equal(1.0)
	var code: String = FileAccess.get_file_as_string(TOON_SHADER)
	assert_bool(code.contains("smoothstep(0.5 - soft, 0.5 + soft, fract(scaled))")).is_true()
	# Ink outline: dark, from the token, on the solid and the skinned presets.
	var outline: Color = Art3dTokens.OUTLINE_COLOR
	assert_float(outline.get_luminance()).is_less(0.2)
	for key: String in ["toon", "toon_skinned"]:
		var pass_2: ShaderMaterial = (load(String(Art3dTokens.MATERIAL_PATHS[key])) as ShaderMaterial).next_pass as ShaderMaterial
		assert_object(pass_2.get_shader_parameter(&"outline_color")).is_equal(outline)


func test_skinned_preset_outlines_along_the_skinned_normal() -> void:
	var skinned: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon_skinned"])) as ShaderMaterial
	assert_bool(bool((skinned.next_pass as ShaderMaterial).get_shader_parameter(&"smooth_normals_in_color"))).is_false()
	var solid: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon"])) as ShaderMaterial
	assert_bool(bool((solid.next_pass as ShaderMaterial).get_shader_parameter(&"smooth_normals_in_color"))).is_true()


func test_chibi_proportions() -> void:
	var body: ArrayMesh = load(ArtCatalog.MODELS_DIR + "/chr/chr_body_kid_a.res") as ArrayMesh
	var height: float = body.get_aabb().end.y
	var head: float = KidSkeleton.HEAD_RADII.y * 2.0
	assert_float(head / height).is_between(1.0 / 3.0, 0.5)
	assert_float(height).is_between(1.1, 1.4)
	# Short limbs: legs below the hips and arms (shoulder → hand) each shorter than the head.
	assert_float(KidSkeleton.BONE_REST[KidSkeleton.LEG_L].y).is_less(head)
	var hand: Vector3 = KidSkeleton.ANCHORS["hand"][1] as Vector3
	assert_float(hand.length()).is_less(head)


func test_anime_faces_per_expression() -> void:
	for expression: String in ArtCatalog.face_expressions():
		var face: ArrayMesh = load(KidSkeleton.face_path(expression)) as ArrayMesh
		assert_object(face).override_failure_message(expression).is_not_null()
		var slots: Dictionary = _slots(face)
		for feature: String in ["eye", "mouth", "cheek"]:
			assert_bool(slots.has(feature)).override_failure_message("%s %s" % [expression, feature]).is_true()
	var neutral: Dictionary = _slots(load(KidSkeleton.face_path("neutral")) as ArrayMesh)
	for feature: String in ["eye_white", "eye_iris", "eye_light"]:
		assert_bool(neutral.has(feature)).override_failure_message(feature).is_true()
	assert_bool(_slots(load(KidSkeleton.face_path("surprised")) as ArrayMesh).has("eye_light")).is_true()
	# Faces sit on the front of the head (head-bone space), in front of the skin.
	var aabb: AABB = (load(KidSkeleton.face_path("neutral")) as Mesh).get_aabb()
	var head_local: Vector3 = KidSkeleton.HEAD_CENTER - KidSkeleton.BONE_REST[KidSkeleton.HEAD]
	assert_float(aabb.position.z).is_less(head_local.z - KidSkeleton.HEAD_RADII.z * 0.9)


func test_body_and_outfits_are_skinned_to_kid_base() -> void:
	var meshes: Array[String] = [ArtCatalog.MODELS_DIR + "/chr/chr_body_kid_a.res"]
	for style: String in CharacterArt.OUTFIT_STYLES:
		meshes.append("%s/chr/chr_outfit_%s_a.res" % [ArtCatalog.MODELS_DIR, style])
	for path: String in meshes:
		var mesh: ArrayMesh = load(path) as ArrayMesh
		assert_bool(AvatarBuilder.is_skinned(mesh)).override_failure_message(path).is_true()
		var bones: PackedInt32Array = mesh.surface_get_arrays(0)[Mesh.ARRAY_BONES]
		var weights: PackedFloat32Array = mesh.surface_get_arrays(0)[Mesh.ARRAY_WEIGHTS]
		var used: Dictionary = {}
		for i: int in bones.size():
			assert_int(bones[i]).is_less(KidSkeleton.BONE_NAMES.size())
			if weights[i] > 0.0:
				used[bones[i]] = true
		for v: int in range(0, weights.size(), 4):
			assert_float(weights[v] + weights[v + 1] + weights[v + 2] + weights[v + 3]).is_equal_approx(1.0, 0.001)
		assert_int(used.size()).override_failure_message(path).is_greater(2)
	# Hair and accessories are rigid parts on bone attachments.
	assert_bool(AvatarBuilder.is_skinned(load(ArtCatalog.MODELS_DIR + "/chr/chr_hair_bob_a.res") as Mesh)).is_false()


func test_clips_key_every_bone_and_squash_keeps_volume() -> void:
	for clip_name: String in ["idle", "walk", "run", "jump", "wave", "celebrate", "clap", "dance", "thank", "apologize"]:
		var anim: Animation = load(KidSkeleton.clip_path(clip_name)) as Animation
		assert_object(anim).override_failure_message(clip_name).is_not_null()
		var paths: Dictionary = {}
		for t: int in anim.get_track_count():
			paths[String(anim.track_get_path(t))] = true
		for bone: int in KidSkeleton.BONE_NAMES.size():
			assert_bool(paths.has(String(KidSkeleton.track(bone)))).override_failure_message("%s %d" % [clip_name, bone]).is_true()
		assert_bool(anim.loop_mode == Animation.LOOP_LINEAR).is_equal(clip_name in ["idle", "walk", "run"])
	for amount: float in [-0.14, -0.05, 0.0, 0.07, 0.12]:
		var s: Vector3 = CharacterAnim.squash(amount)
		assert_float(s.x * s.y * s.z).is_equal_approx(1.0, 0.001)
	# The bouncy run squashes on contact and stretches in the air.
	var run: Animation = load(KidSkeleton.clip_path("run")) as Animation
	var squash_track: int = run.find_track(KidSkeleton.track(KidSkeleton.ROOT), Animation.TYPE_SCALE_3D)
	assert_float(run.scale_track_interpolate(squash_track, 0.0).y).is_less(0.95)
	assert_float(run.scale_track_interpolate(squash_track, run.length * 0.25).y).is_greater(1.04)


func test_emote_items_play_the_baked_clips() -> void:
	for emote: String in ["wave", "celebrate", "clap", "dance", "thank", "apologize"]:
		var visual: ItemVisual = ItemVisual.load_item("emote_" + emote)
		assert_object(visual.emote_clip).override_failure_message(emote).is_not_null()
		assert_bool(visual.placeholder).is_false()
		assert_str(visual.emote_clip.resource_path).is_equal(KidSkeleton.clip_path(emote))
		if not visual.expression.is_empty():
			assert_bool(visual.expression in KidSkeleton.EXPRESSIONS).is_true()
