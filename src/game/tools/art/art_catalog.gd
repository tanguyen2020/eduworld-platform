extends RefCounted
## Catalog of the baked F067 art (ADR-0102): output path under res://assets/models/<category>/ → recipe. Used by the
## bake CLI (tools/art/bake_art.gd) and by the freshness test, which re-runs every recipe in memory and compares it
## with the committed resource (same vertex count, positions/UVs within a small tolerance, MultiMesh transforms).
## SPDX-License-Identifier: MIT — EduWorld first-party.

const HubArt := preload("res://tools/art/hub_art.gd")
const CharacterArt := preload("res://tools/art/character_art.gd")

const MODELS_DIR: String = "res://assets/models"
const TOLERANCE: float = 0.002
## Hub portal targets with their own wind-gate icon (area ids of the world seed, F071); others use the default gate.
const GATE_TARGETS: PackedStringArray = [
	"learning_zone.math", "learning_zone.vietnamese", "learning_zone.english", "learning_zone.science",
	"learning_zone.history", "quest_zone.discovery_ocean", "challenge_mountain", "exam_zone", "arena", "practice_zone",
	"social_zone",
]
## Vegetation kinds scattered as MultiMesh in the hub → mesh file name.
const VEG_KINDS: Dictionary = {
	"tree": "veg_tree_round_a", "palm": "veg_palm_cau_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a",
	"tuft": "veg_tuft_a", "flower": "veg_flower_a", "rice": "veg_rice_a",
}
const ACCESSORIES: PackedStringArray = ["acc_cap_star", "acc_glasses_round", "acc_backpack_owl", "acc_flag_small"]


static func gate_slug(target_area_id: String) -> String:
	return target_area_id.replace(".", "_")


static func gate_path(target_area_id: String) -> String:
	return "%s/prop/prop_wind_gate_%s.res" % [MODELS_DIR, gate_slug(target_area_id)]


static func veg_mesh_path(kind: String) -> String:
	return "%s/veg/%s.res" % [MODELS_DIR, String(VEG_KINDS[kind])]


static func veg_scatter_path(kind: String) -> String:
	return "%s/env/env_hub_scatter_%s_a.res" % [MODELS_DIR, kind]


## Mesh recipes: path → Callable returning an ArrayMesh.
static func meshes() -> Dictionary:
	var out: Dictionary = {
		MODELS_DIR + "/env/env_hub_terrain_a.res": HubArt.terrain,
		MODELS_DIR + "/bld/bld_hub_set_a.res": HubArt.buildings,
		MODELS_DIR + "/prop/prop_hub_set_a.res": HubArt.props,
		MODELS_DIR + "/veg/veg_banyan_spark_a.res": HubArt.banyan,
		MODELS_DIR + "/prop/prop_notice_board_a.res": HubArt.notice_board,
		MODELS_DIR + "/prop/prop_spark_marker_a.res": HubArt.spark_marker,
		MODELS_DIR + "/prop/prop_wind_gate_default.res": HubArt.wind_gate.bind(""),
		MODELS_DIR + "/chr/chr_body_kid_a.res": CharacterArt.body,
	}
	for target: String in GATE_TARGETS:
		out[gate_path(target)] = HubArt.wind_gate.bind(target)
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
	return out


## Scatter of one vegetation kind referencing the saved mesh (`mesh` = the loaded veg mesh resource).
static func scatter(kind: String, mesh: Mesh) -> ScatterSet:
	var data: ScatterSet = ScatterSet.new()
	data.mesh = mesh
	data.transforms = HubArt.placements(kind)
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
	for kind: String in VEG_KINDS:
		var path: String = veg_scatter_path(kind)
		var saved_set: ScatterSet = load(path) as ScatterSet if ResourceLoader.exists(path) else null
		var expected: Array[Transform3D] = HubArt.placements(kind)
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
	return ""
