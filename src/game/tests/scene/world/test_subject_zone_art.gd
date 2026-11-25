extends GdUnitTestSuite
## F093 art pass 3 — the five subject learning zones (WORLD_DESIGN §2.2, §4.2, §11; ADR-0102, ADR-0103;
## PERFORMANCE_BUDGET §7, §8; WORLD_RENDERING §2, §8.1):
## - F093-AC-01: each zone shows its set pieces — named nodes with baked first-party meshes in the colours WORLD_DESIGN
##   describes, each beside its anchor — plus its landmark, its terrain and the house-icon wind gate home;
## - F093-AC-02: every drawn mesh uses a generated toon preset (colours = palette slots only); solid art has the outline
##   pass, only decor scatter and the far scenery go without; the world meshes are smooth shaded;
## - F093-AC-04 (asset part): every baked zone asset stays in its PERFORMANCE_BUDGET §8 row;
## - F093-AC-05: the anchors equal the world seed layout (ids, kinds, positions) and the new static collision keeps the
##   spawn, every anchor and the two lanes reachable.

const ArtCatalog := preload("res://tools/art/art_catalog.gd")
const GENERATED: String = "res://resources/art/generated/"
const SEED: String = "../backend/modules/game/seed/world/areas.v1.json"
const ZONES: PackedStringArray = ["learning_zone.math", "learning_zone.vietnamese", "learning_zone.english",
	"learning_zone.science", "learning_zone.history"]
## Set piece node (under Static) → [anchor it stands by, max distance (m), palette slots it must show].
const SET_PIECES: Dictionary = {
	"learning_zone.math": {
		"NumberBridge": ["discovery.math.number_bridge", 5.0, ["wood", "wood_dark", "stone", "tile_blue"]],
		"OldWell": ["discovery.math.old_well", 4.0, ["roof", "stone", "water_deep", "tile_blue"]],
		"AbacusArch": ["collectible.math.shell_abacus", 0.5, ["subject_math", "cloth_red", "lantern_glow", "wood_dark"]],
		"FruitStall": ["quest.math.fruit_stall", 3.5, ["subject_math", "cloth_red", "basket", "wood"]],
	},
	"learning_zone.vietnamese": {
		"SteleTurtle": ["discovery.vietnamese.ancient_letter", 4.0, ["stone", "stone_dark", "bronze_patina", "roof"]],
		"WordTree": ["discovery.vietnamese.word_tree", 4.0, ["leaf_autumn", "wall_yellow", "trunk", "flower_white", "lantern"]],
		"BrushScroll": ["collectible.vietnamese.page_lost", 0.5, ["flower_white", "cloth_blue", "eye", "lantern", "bamboo"]],
	},
	"learning_zone.english": {
		"RobotWorkshop": ["discovery.english.robot_workshop", 5.0, ["subject_english", "bronze", "metal", "wall"]],
		"EchoCave": ["discovery.english.echo_cave", 5.5, ["stone", "subject_english", "blossom_violet", "eye", "wind"]],
		"GearArch": ["collectible.english.gear_word", 0.5, ["subject_english", "bronze", "metal", "metal_dark"]],
	},
	"learning_zone.science": {
		"WeatherStation": ["discovery.science.weather_station", 4.5, ["metal", "flower_white", "cloth_red", "cloth_blue"]],
		"TidePool": ["discovery.science.tide_pool", 4.5, ["water", "stone", "cloth_red", "flower_white"]],
		"FossilGate": ["collectible.science.leaf_fossil", 0.5, ["stone", "stone_dark", "leaf"]],
	},
	"learning_zone.history": {
		"BronzeDrum": ["discovery.history.bronze_drum", 4.0, ["bronze", "bronze_patina", "lantern_glow", "stone"]],
		"MapPavilion": ["discovery.history.old_map", 4.5, ["lantern", "wall_yellow", "water", "leaf_light"]],
		"CoinGate": ["collectible.history.ancient_coin", 0.5, ["bronze", "bronze_patina", "stone"]],
	},
}
## Landmark behind the wind gate (Static/Buildings) and terrain (Terrain/Ground) slots per zone.
const LANDMARK_SLOTS: Dictionary = {
	"learning_zone.math": ["tile_blue", "tile_blue_dark", "wall", "subject_math"],
	"learning_zone.vietnamese": ["roof", "roof_dark", "wall", "lantern_glow", "wood"],
	"learning_zone.english": ["subject_english", "cloth_violet", "flower_white", "bronze"],
	"learning_zone.science": ["wind", "subject_science", "flower_white", "metal"],
	"learning_zone.history": ["roof_dark", "wall_yellow", "lantern", "stone_dark"],
}
const TERRAIN_SLOTS: Dictionary = {
	"learning_zone.math": ["water", "water_deep", "tile_blue", "path"],
	"learning_zone.vietnamese": ["water", "roof", "leaf_autumn", "paving"],
	"learning_zone.english": ["cloth_violet", "blossom_violet", "path"],
	"learning_zone.science": ["path", "water", "subject_science"],
	"learning_zone.history": ["paving_dark", "roof_dark", "water", "stone"],
}
## Player capsule radius margin for the reachability check.
const PLAYER_RADIUS: float = 0.35

var _zones: Array[Node] = []


func after_test() -> void:
	for zone: Node in _zones:
		if is_instance_valid(zone):
			zone.free()
	_zones.clear()


func _zone(area: String) -> ZoneRoot:
	var zone: ZoneRoot = (load(_path(area)) as PackedScene).instantiate() as ZoneRoot
	zone.interactive = false
	add_child(zone)
	_zones.append(zone)
	return zone


static func _path(area: String) -> String:
	return "res://scenes/world/zones/%s/Zone.tscn" % area.replace(".", "_")


static func _slots(mesh: Mesh) -> Dictionary:
	var out: Dictionary = {}
	if mesh == null or mesh.get_surface_count() == 0:
		return out
	for uv: Vector2 in mesh.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]:
		if uv.y < 1.0:
			out[Art3dTokens.ART_PALETTE[int(uv.x)]] = true
	return out


static func _interactables(zone: Node) -> Dictionary:
	var out: Dictionary = {}
	for node: Node in zone.find_children("*", "Interactable", true, false):
		out[(node as Interactable).interactable_id] = node
	return out


static func _seed() -> Dictionary:
	var path: String = ProjectSettings.globalize_path("res://").path_join(SEED).simplify_path()
	var out: Dictionary = {}
	for a: Variant in JSON.parse_string(FileAccess.get_file_as_string(path)) as Array:
		out[String((a as Dictionary)["area_id"])] = a
	return out


static func _vec(value: Variant) -> Vector3:
	var arr: Array = value as Array
	return Vector3(float(arr[0]), float(arr[1]), float(arr[2]))


## Share of the vertices whose shading normal differs from their face normal (0 = flat low-poly; F068).
static func _smooth_share(mesh: ArrayMesh) -> float:
	var arrays: Array = mesh.surface_get_arrays(0)
	var verts: PackedVector3Array = arrays[Mesh.ARRAY_VERTEX]
	var normals: PackedVector3Array = arrays[Mesh.ARRAY_NORMAL]
	var smooth: int = 0
	for i: int in range(0, verts.size(), 3):
		var face: Vector3 = (verts[i + 2] - verts[i]).cross(verts[i + 1] - verts[i]).normalized()
		for k: int in 3:
			if normals[i + k].dot(face) < 0.999:
				smooth += 1
	return float(smooth) / float(maxi(verts.size(), 1))


func test_set_pieces_stand_by_their_anchors_with_their_look() -> void:
	for area: String in ZONES:
		var zone: ZoneRoot = _zone(area)
		var anchors: Dictionary = _interactables(zone)
		var pieces: Dictionary = SET_PIECES[area]
		for piece: String in pieces:
			var spec: Array = pieces[piece] as Array
			var node: MeshInstance3D = zone.get_node_or_null("Static/" + piece) as MeshInstance3D
			assert_object(node).override_failure_message("%s %s" % [area, piece]).is_not_null()
			assert_str(node.mesh.resource_path).starts_with(ArtCatalog.MODELS_DIR)
			var anchor: Node3D = anchors.get(String(spec[0])) as Node3D
			assert_object(anchor).override_failure_message(String(spec[0])).is_not_null()
			var flat: Vector2 = Vector2(node.global_position.x - anchor.global_position.x, node.global_position.z - anchor.global_position.z)
			assert_float(flat.length()).override_failure_message("%s %s far from %s" % [area, piece, spec[0]]) \
					.is_less_equal(float(spec[1]))
			var slots: Dictionary = _slots(node.mesh)
			for slot: String in spec[2] as Array:
				assert_bool(slots.has(slot)).override_failure_message("%s %s %s" % [area, piece, slot]).is_true()
		var landmark: Dictionary = _slots((zone.get_node("Static/Buildings") as MeshInstance3D).mesh)
		for slot: String in LANDMARK_SLOTS[area] as Array:
			assert_bool(landmark.has(slot)).override_failure_message("%s landmark %s" % [area, slot]).is_true()
		var ground: Dictionary = _slots((zone.get_node("Terrain/Ground") as MeshInstance3D).mesh)
		for slot: String in TERRAIN_SLOTS[area] as Array:
			assert_bool(ground.has(slot)).override_failure_message("%s terrain %s" % [area, slot]).is_true()
		# the wind gate home shows the house icon (prop_wind_gate_main_hub)
		for portal: Node in zone.get_node("Portals").get_children():
			var gate: MeshInstance3D = portal.get_node("Gate") as MeshInstance3D
			assert_str(gate.mesh.resource_path).is_equal(ArtCatalog.gate_path("main_hub"))


func test_every_mesh_uses_a_generated_toon_preset() -> void:
	for area: String in ZONES:
		var zone: ZoneRoot = _zone(area)
		var checked: int = 0
		for node: Node in zone.find_children("*", "GeometryInstance3D", true, false):
			if node is Label3D or not (node as GeometryInstance3D).is_visible_in_tree() or _inside_avatar(node):
				continue
			var geo: GeometryInstance3D = node as GeometryInstance3D
			var material: Material = geo.material_override
			if node is BlobShadow:
				assert_object(material).is_same(BlobShadow.shared_material())
				continue
			var where: String = "%s %s" % [area, node.get_path()]
			assert_object(material).override_failure_message(where).is_not_null()
			assert_str(material.resource_path).override_failure_message(where).starts_with(GENERATED)
			var decor: bool = node.is_in_group(ZoneArtQuality.DECOR_GROUP) and material.next_pass == null
			var far: bool = node.name == &"Far" and geo.cast_shadow == GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
			if not (decor or far):
				assert_object(material.next_pass).override_failure_message(where).is_not_null()
			checked += 1
		assert_int(checked).is_greater(12)


func _inside_avatar(node: Node) -> bool:
	var parent: Node = node.get_parent()
	while parent != null:
		if parent is AvatarBuilder:
			return true
		parent = parent.get_parent()
	return false


func test_zone_world_meshes_are_smooth_shaded() -> void:
	for slug: String in ArtCatalog.ZONES:
		var recipes: Dictionary = (ArtCatalog.ZONES[slug] as GDScript).call("meshes") as Dictionary
		for key: String in recipes:
			if key.begins_with("env/"):
				continue  # flat ground and far scenery
			var path: String = "%s/%s.res" % [ArtCatalog.MODELS_DIR, key]
			assert_float(_smooth_share(load(path) as ArrayMesh)).override_failure_message(path).is_greater(0.3)


## PERFORMANCE_BUDGET §8: ground + far scenery ≤ 40k per zone, a building (or the landmark set) ≤ 10k, a large prop
## (set piece at an anchor) ≤ 3k, a repeated tree ≤ 2k.
func test_zone_assets_stay_in_the_asset_budgets() -> void:
	for slug: String in ArtCatalog.ZONES:
		var recipes: Dictionary = (ArtCatalog.ZONES[slug] as GDScript).call("meshes") as Dictionary
		var ground: int = 0
		for key: String in recipes:
			var tris: int = (load("%s/%s.res" % [ArtCatalog.MODELS_DIR, key]) as Mesh).get_faces().size() / 3
			if key.begins_with("env/"):
				ground += tris
			elif key.begins_with("bld/") or key.ends_with("_set_a"):
				assert_int(tris).override_failure_message(key).is_less_equal(10000)
			elif key.begins_with("veg/"):
				assert_int(tris).override_failure_message(key).is_less_equal(2000)
			else:
				assert_int(tris).override_failure_message(key).is_less_equal(3000)
		assert_int(ground).override_failure_message(slug).is_less_equal(40000)


func test_anchors_equal_the_world_seed_layout() -> void:
	var seed: Dictionary = _seed()
	for area: String in ZONES:
		var zone: ZoneRoot = _zone(area)
		var layout: Dictionary = (seed[area] as Dictionary)["layout"] as Dictionary
		var anchors: Dictionary = _interactables(zone)
		var expected: int = 0
		for it: Variant in layout.get("interactables", []) as Array:
			var item: Dictionary = it as Dictionary
			var node: Interactable = anchors.get(String(item["interactable_id"])) as Interactable
			assert_object(node).override_failure_message("%s %s" % [area, item["interactable_id"]]).is_not_null()
			assert_float(node.global_position.distance_to(_vec(item["position"]))).is_less(0.01)
			assert_str(node.kind).is_equal("npc" if String(item["kind"]) == "npc" else String(item["kind"]))
			expected += 1
		for p: Variant in layout.get("portals", []) as Array:
			var portal: AreaPortal = anchors.get(String((p as Dictionary)["portal_id"])) as AreaPortal
			assert_object(portal).is_not_null()
			assert_str(portal.target_area_id).is_equal(String((p as Dictionary)["target_area_id"]))
			expected += 1
		assert_int(anchors.size()).override_failure_message(area).is_equal(expected)
		var spawns: Dictionary = {}
		for point: SpawnPoint in zone.spawn_points():
			spawns[point.spawn_point_id] = point.global_position
		for s: Variant in layout["spawn_points"] as Array:
			var id: String = String((s as Dictionary)["spawn_point_id"])
			assert_bool(spawns.has(id)).override_failure_message(id).is_true()
			assert_float((spawns[id] as Vector3).distance_to(_vec((s as Dictionary)["position"]))).is_less(0.01)
		# the ground collision of F071 stays
		var ground: CollisionShape3D = zone.get_node("Terrain/Collision") as CollisionShape3D
		assert_object((ground.shape as BoxShape3D).size).is_equal(Vector3(50, 1, 50))


func test_static_collision_keeps_spawn_anchors_and_lanes_reachable() -> void:
	for area: String in ZONES:
		var zone: ZoneRoot = _zone(area)
		var shapes: Array[CollisionShape3D] = []
		for node: Node in zone.get_node("Static").get_children():
			if node is CollisionShape3D and not ((node as CollisionShape3D).shape is ConvexPolygonShape3D):
				shapes.append(node as CollisionShape3D)  # a convex shape is a walkable deck (the Number Bridge)
		assert_int(shapes.size()).is_greater(4)
		var points: Array[Vector3] = []
		for point: SpawnPoint in zone.spawn_points():
			points.append(point.global_position)
		for node: Node in _interactables(zone).values():
			points.append((node as Node3D).global_position)
		for i: int in 61:
			points.append(Vector3(0, 0, 10.0 - i * 0.5))  # main lane: spawn → collectible → wind gate
		for i: int in 45:
			points.append(Vector3(-11.0 + i * 0.5, 0, -6.0))  # cross path between the discoveries
		for p: Vector3 in points:
			for shape: CollisionShape3D in shapes:
				assert_bool(_blocks(shape, p + Vector3.UP * 0.5)).override_failure_message("%s %s blocks %s" % [area, shape.name, p]) \
						.is_false()


## True when the player capsule centred at `p` would overlap the box / cylinder `c` (axis-aligned shapes of the zones).
static func _blocks(c: CollisionShape3D, p: Vector3) -> bool:
	var local: Vector3 = c.global_transform.affine_inverse() * p
	if c.shape is BoxShape3D:
		var h: Vector3 = (c.shape as BoxShape3D).size * 0.5
		return absf(local.x) < h.x + PLAYER_RADIUS and absf(local.z) < h.z + PLAYER_RADIUS and absf(local.y) < h.y + 0.5
	if c.shape is CylinderShape3D:
		var cyl: CylinderShape3D = c.shape as CylinderShape3D
		return Vector2(local.x, local.z).length() < cyl.radius + PLAYER_RADIUS and absf(local.y) < cyl.height * 0.5 + 0.5
	return false
