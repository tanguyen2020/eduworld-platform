extends RefCounted
## Catalog of the baked art (F067, F068, F093; ADR-0102): output path under res://assets/models/<category>/ → recipe. Used by
## the bake CLI (tools/art/bake_art.gd) and by the freshness test, which re-runs every recipe in memory and compares it
## with the committed resource (same vertex count, positions/UVs/bone weights within a small tolerance, MultiMesh
## transforms, animation keys).
## SPDX-License-Identifier: MIT — EduWorld first-party.

const HubArt := preload("res://tools/art/hub_art.gd")
const CharacterArt := preload("res://tools/art/character_art.gd")
const CharacterAnim := preload("res://tools/art/character_anim.gd")
const ZoneMathArt := preload("res://tools/art/zone_math_art.gd")
const ZoneVietnameseArt := preload("res://tools/art/zone_vietnamese_art.gd")
const ZoneEnglishArt := preload("res://tools/art/zone_english_art.gd")
const ZoneScienceArt := preload("res://tools/art/zone_science_art.gd")
const ZoneHistoryArt := preload("res://tools/art/zone_history_art.gd")

const MODELS_DIR: String = "res://assets/models"
const TOLERANCE: float = 0.002
## Hub portal targets with their own wind-gate icon (area ids of the world seed, F071); others use the default gate.
## "main_hub" is the gate home of every other zone (F093: house icon).
const GATE_TARGETS: PackedStringArray = [
	"main_hub", "learning_zone.math", "learning_zone.vietnamese", "learning_zone.english", "learning_zone.science",
	"learning_zone.history", "quest_zone.discovery_ocean", "challenge_mountain", "exam_zone", "arena", "practice_zone",
	"social_zone",
]
## Vegetation kinds scattered as MultiMesh in the hub → mesh file name.
const VEG_KINDS: Dictionary = {
	"tree": "veg_tree_round_a", "palm": "veg_palm_cau_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a",
	"tuft": "veg_tuft_a", "flower": "veg_flower_a", "rice": "veg_rice_a",
}
const ACCESSORIES: PackedStringArray = ["acc_cap_star", "acc_glasses_round", "acc_backpack_owl", "acc_flag_small"]
## Subject zones of art pass 3 (F093, WORLD_DESIGN §4.2): zone slug → recipe script (meshes(), SCATTER, placements()).
const ZONES: Dictionary = {"math": ZoneMathArt, "vietnamese": ZoneVietnameseArt, "english": ZoneEnglishArt,
	"science": ZoneScienceArt, "history": ZoneHistoryArt}
## Growth stages of the Spark Tree (F068, WORLD_DESIGN §11 mastery visualization): 0 grey … 4 in bloom.
const SPARK_TREE_STAGES: int = 5


static func spark_tree_path(stage: int) -> String:
	return "%s/veg/veg_banyan_spark_s%d_a.res" % [MODELS_DIR, stage]


static func face_expressions() -> PackedStringArray:
	var out: PackedStringArray = KidSkeleton.EXPRESSIONS.duplicate()
	out.append(KidSkeleton.BLINK)
	return out


## Animation clips: path → Callable returning an Animation.
static func animations() -> Dictionary:
	var out: Dictionary = {}
	for clip_name: String in CharacterAnim.names():
		out[KidSkeleton.clip_path(clip_name)] = CharacterAnim.clip.bind(clip_name)
	return out


static func gate_slug(target_area_id: String) -> String:
	return target_area_id.replace(".", "_")


static func gate_path(target_area_id: String) -> String:
	return "%s/prop/prop_wind_gate_%s.res" % [MODELS_DIR, gate_slug(target_area_id)]


static func veg_mesh_path(kind: String) -> String:
	return "%s/veg/%s.res" % [MODELS_DIR, String(VEG_KINDS[kind])]


static func veg_scatter_path(kind: String) -> String:
	return "%s/env/env_hub_scatter_%s_a.res" % [MODELS_DIR, kind]


## Scatter of one vegetation kind in a subject zone (F093).
static func zone_scatter_path(slug: String, kind: String) -> String:
	return "%s/env/env_%s_scatter_%s_a.res" % [MODELS_DIR, slug, kind]


## Every zone scatter: path → [veg mesh path, placements Callable].
static func zone_scatters() -> Dictionary:
	var out: Dictionary = {}
	for slug: String in ZONES:
		var recipe: GDScript = ZONES[slug] as GDScript
		var kinds: Dictionary = recipe.get_script_constant_map()["SCATTER"] as Dictionary
		for kind: String in kinds:
			out[zone_scatter_path(slug, kind)] = ["%s/veg/%s.res" % [MODELS_DIR, String(kinds[kind])],
				Callable(recipe, "placements").bind(kind)]
	return out


## Mesh recipes: path → Callable returning an ArrayMesh.
static func meshes() -> Dictionary:
	var out: Dictionary = {
		MODELS_DIR + "/env/env_hub_terrain_a.res": HubArt.terrain,
		MODELS_DIR + "/bld/bld_hub_set_a.res": HubArt.buildings,
		MODELS_DIR + "/prop/prop_hub_set_a.res": HubArt.props,
		MODELS_DIR + "/prop/prop_notice_board_a.res": HubArt.notice_board,
		MODELS_DIR + "/prop/prop_spark_marker_a.res": HubArt.spark_marker,
		MODELS_DIR + "/prop/prop_wind_gate_default.res": HubArt.wind_gate.bind(""),
		MODELS_DIR + "/chr/chr_body_kid_a.res": CharacterArt.body,
	}
	for target: String in GATE_TARGETS:
		out[gate_path(target)] = HubArt.wind_gate.bind(target)
	for stage: int in SPARK_TREE_STAGES:
		out[spark_tree_path(stage)] = HubArt.spark_tree.bind(stage)
	for expression: String in face_expressions():
		out[KidSkeleton.face_path(expression)] = CharacterArt.face.bind(expression)
	var veg: Dictionary = {"tree": HubArt.tree_round, "palm": HubArt.palm, "bamboo": HubArt.bamboo, "bush": HubArt.bush,
		"tuft": HubArt.tuft, "flower": HubArt.flower, "rice": HubArt.rice}
	for kind: String in VEG_KINDS:
		out[veg_mesh_path(kind)] = veg[kind]
	for style: String in CharacterArt.HAIR_STYLES:
		out["%s/chr/chr_hair_%s_a.res" % [MODELS_DIR, style]] = CharacterArt.hair.bind(style)
	for style: String in CharacterArt.OUTFIT_STYLES:
		out["%s/chr/chr_outfit_%s_a.res" % [MODELS_DIR, style]] = CharacterArt.outfit.bind(style)
	for code: String in ACCESSORIES:
		out["%s/chr/chr_%s_a.res" % [MODELS_DIR, code]] = CharacterArt.accessory.bind(code)
	for look: String in CharacterArt.NPC_LOOKS:
		out["%s/chr/chr_npc_%s_a.res" % [MODELS_DIR, look]] = CharacterArt.npc.bind(look)
	for slug: String in ZONES:
		var zone: Dictionary = (ZONES[slug] as GDScript).call("meshes") as Dictionary
		for key: String in zone:
			out["%s/%s.res" % [MODELS_DIR, key]] = zone[key]
	return out


## Scatter of one vegetation kind referencing the saved mesh (`mesh` = the loaded veg mesh resource).
static func scatter(kind: String, mesh: Mesh) -> ScatterSet:
	var data: ScatterSet = ScatterSet.new()
	data.mesh = mesh
	data.transforms = HubArt.placements(kind)
	return data


## Scatter referencing a saved veg mesh with explicit placements (zones, F093).
static func scatter_of(mesh: Mesh, transforms: Array[Transform3D]) -> ScatterSet:
	var data: ScatterSet = ScatterSet.new()
	data.mesh = mesh
	data.transforms = transforms
	return data


## Differences between the committed resources and a fresh run of the recipes (empty = up to date).
static func drift() -> PackedStringArray:
	var out: PackedStringArray = []
	var recipes: Dictionary = meshes()
	for path: String in recipes:
		var fresh: ArrayMesh = (recipes[path] as Callable).call() as ArrayMesh
		var saved: ArrayMesh = load(path) as ArrayMesh if ResourceLoader.exists(path) else null
		var why: String = mesh_difference(saved, fresh)
		if not why.is_empty():
			out.append("%s: %s" % [path, why])
	var clips: Dictionary = animations()
	for path: String in clips:
		var saved_clip: Animation = load(path) as Animation if ResourceLoader.exists(path) else null
		var why_clip: String = animation_difference(saved_clip, (clips[path] as Callable).call() as Animation)
		if not why_clip.is_empty():
			out.append("%s: %s" % [path, why_clip])
	var sets: Dictionary = {}
	for kind: String in VEG_KINDS:
		sets[veg_scatter_path(kind)] = HubArt.placements(kind)
	var zone_sets: Dictionary = zone_scatters()
	for path: String in zone_sets:
		sets[path] = ((zone_sets[path] as Array)[1] as Callable).call()
	for path: String in sets:
		var saved_set: ScatterSet = load(path) as ScatterSet if ResourceLoader.exists(path) else null
		var expected: Array[Transform3D] = sets[path]
		if saved_set == null:
			out.append("%s: missing" % path)
		elif saved_set.transforms.size() != expected.size():
			out.append("%s: %d instances, recipe has %d" % [path, saved_set.transforms.size(), expected.size()])
		else:
			for i: int in expected.size():
				if saved_set.transforms[i].origin.distance_to(expected[i].origin) > TOLERANCE:
					out.append("%s: instance %d moved" % [path, i])
					break
	return out


static func mesh_difference(saved: ArrayMesh, fresh: ArrayMesh) -> String:
	if saved == null:
		return "missing"
	if saved.get_surface_count() != fresh.get_surface_count():
		return "surface count %d, recipe has %d" % [saved.get_surface_count(), fresh.get_surface_count()]
	for s: int in fresh.get_surface_count():
		var a: Array = saved.surface_get_arrays(s)
		var b: Array = fresh.surface_get_arrays(s)
		var va: PackedVector3Array = a[Mesh.ARRAY_VERTEX]
		var vb: PackedVector3Array = b[Mesh.ARRAY_VERTEX]
		if va.size() != vb.size():
			return "%d vertices, recipe has %d" % [va.size(), vb.size()]
		var ua: PackedVector2Array = a[Mesh.ARRAY_TEX_UV]
		var ub: PackedVector2Array = b[Mesh.ARRAY_TEX_UV]
		for i: int in va.size():
			if va[i].distance_to(vb[i]) > TOLERANCE or ua[i].distance_to(ub[i]) > TOLERANCE:
				return "vertex %d differs (re-run tools/art/bake_art.gd)" % i
		var ba: Variant = a[Mesh.ARRAY_BONES]
		var bb: Variant = b[Mesh.ARRAY_BONES]
		if (ba == null) != (bb == null) or (bb != null and ba != bb):
			return "bone indices differ (re-run tools/art/bake_art.gd)"
		var wa: Variant = a[Mesh.ARRAY_WEIGHTS]
		var wb: Variant = b[Mesh.ARRAY_WEIGHTS]
		if bb != null:
			for i: int in (wb as PackedFloat32Array).size():
				if absf((wa as PackedFloat32Array)[i] - (wb as PackedFloat32Array)[i]) > TOLERANCE:
					return "bone weight %d differs (re-run tools/art/bake_art.gd)" % i
	return ""


static func animation_difference(saved: Animation, fresh: Animation) -> String:
	if saved == null:
		return "missing"
	if not is_equal_approx(saved.length, fresh.length) or saved.loop_mode != fresh.loop_mode:
		return "length/loop differ"
	if saved.get_track_count() != fresh.get_track_count():
		return "%d tracks, recipe has %d" % [saved.get_track_count(), fresh.get_track_count()]
	for t: int in fresh.get_track_count():
		if saved.track_get_path(t) != fresh.track_get_path(t) or saved.track_get_key_count(t) != fresh.track_get_key_count(t):
			return "track %d differs" % t
		for k: int in fresh.track_get_key_count(t):
			var va: Variant = saved.track_get_key_value(t, k)
			var vb: Variant = fresh.track_get_key_value(t, k)
			var same: bool = absf(saved.track_get_key_time(t, k) - fresh.track_get_key_time(t, k)) < TOLERANCE
			if vb is Quaternion:
				same = same and (va as Quaternion).is_equal_approx(vb as Quaternion)
			elif vb is Vector3:
				same = same and (va as Vector3).distance_to(vb as Vector3) < TOLERANCE
			if not same:
				return "track %d key %d differs (re-run tools/art/bake_art.gd)" % [t, k]
	return ""
