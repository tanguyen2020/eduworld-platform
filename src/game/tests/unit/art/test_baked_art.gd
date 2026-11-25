extends GdUnitTestSuite
## F067 baked art (ADR-0102, DESIGN_SYSTEM §5.3, PERFORMANCE_BUDGET §8):
## - AC-01: every baked face addresses a slot of the art palette tokens (or the per-avatar ALBEDO slot) — no colour is
##   stored in a mesh, and the toon presets carry exactly the `Art3dTokens.ART_PALETTE_COLORS`;
## - AC-05: the shared toon preset = toon shader (`toon_steps`, `shadow_tint`, `rim_strength` from the tokens) + the
##   inverted-hull outline pass (`outline.width`, `outline.color`); the decor preset has no outline;
## - AC-02: per-asset triangle budgets; AC-09: the committed meshes match their recipes (re-bake on change).

const ArtCatalog := preload("res://tools/art/art_catalog.gd")
const CharacterArt := preload("res://tools/art/character_art.gd")
const TOON_SHADER: String = "res://resources/art/shaders/toon.gdshader"
const OUTLINE_SHADER: String = "res://resources/art/shaders/toon_outline.gdshader"
## PERFORMANCE_BUDGET §8 (LOD0): player character total, NPC, large prop, building, tree, ground per zone.
const BUDGET_CHARACTER: int = 8000
const BUDGET_NPC: int = 10000
const BUDGET_LARGE_PROP: int = 3000
const BUDGET_BUILDING_SET_PER_BUILDING: int = 10000
const BUDGET_TREE: int = 2000
const BUDGET_GROUND: int = 40000


func _triangles(path: String) -> int:
	var mesh: Mesh = load(path) as Mesh
	return mesh.get_faces().size() / 3 if mesh != null else 0


func test_committed_art_matches_the_recipes() -> void:
	assert_array(ArtCatalog.drift()).is_empty()


func test_every_baked_face_uses_a_palette_slot_or_albedo() -> void:
	var size: int = Art3dTokens.ART_PALETTE.size()
	for path: String in ArtCatalog.meshes():
		var mesh: ArrayMesh = load(path) as ArrayMesh
		assert_object(mesh).override_failure_message(path).is_not_null()
		var uvs: PackedVector2Array = mesh.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]
		for uv: Vector2 in uvs:
			var albedo: bool = is_equal_approx(uv.y, 1.5)
			var slot_ok: bool = is_equal_approx(uv.y, 0.5) and uv.x > 0.0 and int(uv.x) < size \
					and is_equal_approx(uv.x - floorf(uv.x), 0.5)
			if not (albedo or slot_ok):
				fail("%s: UV %s is not an art palette slot" % [path, uv])
				return


func test_toon_preset_is_built_from_the_tokens() -> void:
	var toon: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon"])) as ShaderMaterial
	assert_str(toon.shader.resource_path).is_equal(TOON_SHADER)
	assert_float(float(toon.get_shader_parameter(&"toon_steps"))).is_equal(Art3dTokens.TOON_STEPS)
	assert_float(float(toon.get_shader_parameter(&"rim_strength"))).is_equal_approx(Art3dTokens.TOON_RIM_STRENGTH, 0.0001)
	assert_object(toon.get_shader_parameter(&"shadow_tint")).is_equal(Art3dTokens.TOON_SHADOW_TINT)
	var palette: PackedColorArray = toon.get_shader_parameter(&"palette") as PackedColorArray
	assert_int(int(toon.get_shader_parameter(&"palette_size"))).is_equal(Art3dTokens.ART_PALETTE.size())
	for i: int in Art3dTokens.ART_PALETTE.size():
		var expected: Color = Art3dTokens.ART_PALETTE_COLORS[Art3dTokens.ART_PALETTE[i]] as Color
		assert_bool(palette[i].is_equal_approx(expected)).override_failure_message(Art3dTokens.ART_PALETTE[i]).is_true()
	var outline: ShaderMaterial = toon.next_pass as ShaderMaterial
	assert_object(outline).is_not_null()
	assert_str(outline.shader.resource_path).is_equal(OUTLINE_SHADER)
	assert_float(float(outline.get_shader_parameter(&"outline_width"))).is_equal_approx(Art3dTokens.OUTLINE_WIDTH, 0.0001)
	assert_object(outline.get_shader_parameter(&"outline_color")).is_equal(Art3dTokens.OUTLINE_COLOR)
	var flat: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon_no_outline"])) as ShaderMaterial
	assert_object(flat.next_pass).is_null()


## AC-07 (renderer parity): the toon shaders use no screen/depth texture, SSAO/SSR or other Forward+-only feature.
func test_toon_shaders_only_use_features_shared_by_mobile_and_compatibility() -> void:
	for path: String in [TOON_SHADER, OUTLINE_SHADER]:
		var code: String = FileAccess.get_file_as_string(path)
		for forbidden: String in ["hint_screen_texture", "hint_depth_texture", "hint_normal_roughness_texture", "SCREEN_TEXTURE",
				"DEPTH_TEXTURE", "sdfgi", "BACKLIGHT", "SSS_STRENGTH", "texture(palette"]:
			assert_bool(code.contains(forbidden)).override_failure_message("%s uses %s" % [path, forbidden]).is_false()
		assert_bool(code.contains("source_color =")).override_failure_message("%s has a default colour" % path).is_false()


func test_assets_are_within_the_asset_budgets() -> void:
	var models: String = ArtCatalog.MODELS_DIR
	var heaviest_outfit: int = 0
	var heaviest_hair: int = 0
	for style: String in CharacterArt.OUTFIT_STYLES:
		heaviest_outfit = maxi(heaviest_outfit, _triangles("%s/chr/chr_outfit_%s_a.res" % [models, style]))
	for style: String in CharacterArt.HAIR_STYLES:
		heaviest_hair = maxi(heaviest_hair, _triangles("%s/chr/chr_hair_%s_a.res" % [models, style]))
	var accessories: int = 0
	for code: String in ArtCatalog.ACCESSORIES:
		accessories += _triangles("%s/chr/chr_%s_a.res" % [models, code])
	var heaviest_face: int = 0
	for expression: String in ArtCatalog.face_expressions():
		heaviest_face = maxi(heaviest_face, _triangles(KidSkeleton.face_path(expression)))
	# F068: the character total also counts the expression face mesh.
	assert_int(_triangles(models + "/chr/chr_body_kid_a.res") + heaviest_face + heaviest_hair + heaviest_outfit
			+ accessories).is_less_equal(BUDGET_CHARACTER)
	for look: String in CharacterArt.NPC_LOOKS:
		assert_int(_triangles("%s/chr/chr_npc_%s_a.res" % [models, look])).is_less_equal(BUDGET_NPC)
	for target: String in ArtCatalog.GATE_TARGETS:
		assert_int(_triangles(ArtCatalog.gate_path(target))).is_less_equal(BUDGET_LARGE_PROP)
	for stage: int in ArtCatalog.SPARK_TREE_STAGES:
		assert_int(_triangles(ArtCatalog.spark_tree_path(stage))).override_failure_message("spark tree %d" % stage) \
				.is_less_equal(BUDGET_LARGE_PROP)
	assert_int(_triangles(models + "/bld/bld_hub_set_a.res")).is_less_equal(BUDGET_BUILDING_SET_PER_BUILDING)
	assert_int(_triangles(models + "/env/env_hub_terrain_a.res")).is_less_equal(BUDGET_GROUND)
	for kind: String in ["tree", "palm", "bamboo"]:
		assert_int(_triangles(ArtCatalog.veg_mesh_path(kind))).is_less_equal(BUDGET_TREE)


## AC-08 (F067, proportions superseded by F068-AC-04 / ADR-0103: head 1/3–1/2 of the height): the chibi body and its
## neutral face mesh (eyes, highlights, mouth, blush).
func test_chibi_body_has_a_big_head_and_a_face() -> void:
	var mesh: ArrayMesh = load(ArtCatalog.MODELS_DIR + "/chr/chr_body_kid_a.res") as ArrayMesh
	var height: float = mesh.get_aabb().size.y
	var head: float = CharacterArt.HEAD_RADII.y * 2.0
	assert_float(head / height).is_between(0.33, 0.5)
	assert_float(height).is_between(1.1, 1.4)
	var face: ArrayMesh = load(KidSkeleton.face_path("neutral")) as ArrayMesh
	var slots: Dictionary = {}
	for uv: Vector2 in face.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]:
		if uv.y < 1.0:
			slots[Art3dTokens.ART_PALETTE[int(uv.x)]] = true
	for feature: String in ["eye", "eye_light", "mouth", "cheek"]:
		assert_bool(slots.has(feature)).override_failure_message(feature).is_true()
