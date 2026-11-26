extends GdUnitTestSuite
## F094 art pass 4 — the six remaining zones and the F093 open points (WORLD_DESIGN §2–§4, §11; ADR-0102, ADR-0103;
## PERFORMANCE_BUDGET §7, §8; WORLD_RENDERING §2, §8.1):
## - F094-AC-01: each zone shows its set pieces (named nodes with baked first-party meshes in the colours of the spec,
##   beside their anchor when they have one), its landmark, its terrain and the house-icon wind gate home;
## - F094-AC-02: every drawn mesh uses a generated toon preset; solid art has the outline pass; world meshes are smooth;
## - F094-AC-03: the Trial Tower stays calm (no bright red/orange slots on its landmark and set pieces), the arena is
##   festive (at least five festive colours);
## - F094-AC-05 (asset part): every baked zone asset stays in its PERFORMANCE_BUDGET §8 row;
## - F094-AC-07: anchors = world seed, ground collision kept, new collision keeps spawn, anchors and lanes reachable;
## - F094-AC-12: no outlined world mesh carries a zero-thickness two-sided plane (F093-3);
## - F094-AC-13: way-finding — a signpost with icon arrows near the spawn and glowing rings on the anchor pads.

const ArtCatalog := preload("res://tools/art/art_catalog.gd")
const GENERATED: String = "res://resources/art/generated/"
const SEED: String = "../backend/modules/game/seed/world/areas.v1.json"
const SLUGS: Dictionary = {
	"practice_zone": "practice", "social_zone": "social", "quest_zone.discovery_ocean": "ocean",
	"challenge_mountain": "mountain", "exam_zone": "exam", "arena": "arena",
}
## Set piece node (under Static) → [anchor it stands by ("" = free-standing), max distance (m), palette slots it shows].
const SET_PIECES: Dictionary = {
	"practice_zone": {
		"PracticeStation": ["station.practice.board", 3.5, ["wood", "leaf_dark", "flower_white", "cloth_blue"]],
		"StudyCorner": ["", 0.0, ["wood", "cloth_green", "flower_white"]],
		"StudyDesks": ["", 0.0, ["roof", "wood", "wall_yellow"]],
		"BookArch": ["", 0.0, ["cloth_red", "cloth_blue", "flower_white"]],
	},
	"social_zone": {
		"SeedPlanter": ["collectible.social.flower_seed", 4.5, ["roof", "lantern_glow", "leaf_dark", "cloth_blue", "flower_white"]],
		"HeartArch": ["", 0.0, ["flower_pink", "cloth_red", "leaf"]],
		"Playground": ["", 0.0, ["cloth_red", "cloth_blue", "wood"]],
		"PicnicSpot": ["", 0.0, ["cloth_red", "cloth_blue", "basket"]],
		"ClubCorner": ["", 0.0, ["cloth_blue", "cloth_green", "cloth_violet"]],
	},
	"quest_zone.discovery_ocean": {
		"Shipwreck": ["discovery.ocean.shipwreck", 5.0, ["wood", "wood_dark", "cloth_red", "flower_white", "lantern_glow"]],
		"CoralCave": ["discovery.ocean.coral_cave", 5.0, ["stone", "eye", "flower_pink", "cloth_violet", "spark"]],
		"ShellArch": ["collectible.ocean.rare_shell", 0.5, ["stone", "flower_pink", "flower_white", "spark"]],
		"OctopusRock": ["", 0.0, ["cloth_violet", "eye_white", "stone_dark"]],
	},
	"challenge_mountain": {
		"SummitLookout": ["discovery.mountain.summit_view", 5.0, ["wood", "bronze", "cloth_red"]],
		"CrystalCluster": ["collectible.mountain.crystal_shard", 4.0, ["blossom_violet", "wind", "spark", "stone_dark"]],
		"ChallengeCircle": ["", 0.0, ["stone", "blossom_violet", "lantern_glow"]],
	},
	"exam_zone": {
		"ExamBoardStand": ["board.exam_sessions", 3.5, ["tile_blue", "flower_white", "cloth_blue", "wall"]],
		"ReflectionPool": ["", 0.0, ["water", "stone", "wind"]],
		"GravelGarden": ["", 0.0, ["path", "stone_dark", "leaf_dark"]],
		"StudyPavilion": ["", 0.0, ["tile_blue", "wood"]],
	},
	"arena": {
		"MatchGates": ["npc.referee_thunder", 9.5, ["cloth_red", "cloth_blue", "lantern_glow", "cloth_green", "flower_white"]],
		"TournamentBoard": ["npc.madam_calendar", 6.5, ["wood", "flower_white", "cloth_green"]],
		"Leaderboard": ["", 0.0, ["lantern_glow", "metal", "bronze", "cloth_blue"]],
		"TrophyStatue": ["", 0.0, ["lantern_glow", "stone"]],
	},
}
const LANDMARK_SLOTS: Dictionary = {
	"practice_zone": ["wall_yellow", "roof", "wind", "flower_white"],
	"social_zone": ["wall_yellow", "rice_ripe", "flower_pink", "leaf"],
	"quest_zone.discovery_ocean": ["cloth_red", "flower_white", "wind", "lantern_glow"],
	"challenge_mountain": ["stone", "blossom_violet", "spark", "eye_light"],
	"exam_zone": ["wall", "tile_blue", "tile_blue_dark", "wind"],
	"arena": ["cloth_red", "cloth_blue", "lantern_glow", "cloth_green", "cloth_violet"],
}
const TERRAIN_SLOTS: Dictionary = {
	"practice_zone": ["paving", "wind", "wall_yellow"],
	"social_zone": ["flower_pink", "path"],
	"quest_zone.discovery_ocean": ["path", "wood", "water"],
	"challenge_mountain": ["stone", "stone_dark", "eye_light"],
	"exam_zone": ["path", "stone", "tile_blue"],
	"arena": ["paving", "lantern_glow", "cloth_violet"],
}
## Slots that would break the calm of the Trial Tower (F094-AC-03) and the festive colours of the arena.
const LOUD_SLOTS: PackedStringArray = ["lantern", "cloth_red", "subject_vietnamese"]
const FESTIVE_SLOTS: PackedStringArray = ["cloth_red", "cloth_blue", "cloth_green", "cloth_violet", "lantern_glow", "flower_pink"]
## Walking lines of each layout besides spawn → wind gate: [from, to] (cross lanes to the anchors).
const LANES: Dictionary = {
	"practice_zone": [[Vector3(-8.5, 0, -7), Vector3(7, 0, -7)]],
	"social_zone": [[Vector3(-11, 0, -4), Vector3(10, 0, -4)]],
	"quest_zone.discovery_ocean": [[Vector3(-11, 0, -8), Vector3(11, 0, -8)]],
	"challenge_mountain": [[Vector3(-12.5, 0, -10), Vector3(10, 0, -10)]],
	"exam_zone": [[Vector3(-8.5, 0, -8), Vector3(9, 0, -8)]],
	"arena": [[Vector3(-10, 0, -9), Vector3(10, 0, -9)]],
}
const PLAYER_RADIUS: float = 0.35
## Baked meshes drawn without the outline pass (flat decor, far scenery, faces) are exempt from the two-sided check.
const NO_OUTLINE: PackedStringArray = ["veg_tuft_a", "veg_flower_a", "veg_rice_a", "_far_a", "/chr/"]

var _zones: Array[Node] = []


func after_test() -> void:
	for zone: Node in _zones:
		if is_instance_valid(zone):
			zone.free()
	_zones.clear()


func _zone(area: String) -> ZoneRoot:
	var zone: ZoneRoot = (load("res://scenes/world/zones/%s/Zone.tscn" % area.replace(".", "_")) as PackedScene).instantiate() as ZoneRoot
	zone.interactive = false
	add_child(zone)
	_zones.append(zone)
	return zone


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


static func _vec(value: Variant) -> Vector3:
	var arr: Array = value as Array
	return Vector3(float(arr[0]), float(arr[1]), float(arr[2]))


static func _seed() -> Dictionary:
	var path: String = ProjectSettings.globalize_path("res://").path_join(SEED).simplify_path()
	var out: Dictionary = {}
	for a: Variant in JSON.parse_string(FileAccess.get_file_as_string(path)) as Array:
		out[String((a as Dictionary)["area_id"])] = a
	return out


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


## Number of zero-thickness two-sided planes: triangles with the same three corners as another one facing the other way.
static func two_sided_faces(mesh: ArrayMesh) -> int:
	var seen: Dictionary = {}
	var count: int = 0
	for s: int in mesh.get_surface_count():
		var verts: PackedVector3Array = mesh.surface_get_arrays(s)[Mesh.ARRAY_VERTEX]
		for i: int in range(0, verts.size(), 3):
			var keys: Array[String] = []
			for k: int in 3:
				var v: Vector3 = verts[i + k]
				keys.append("%d,%d,%d" % [roundi(v.x * 1000.0), roundi(v.y * 1000.0), roundi(v.z * 1000.0)])
			keys.sort()
			var key: String = "|".join(keys)
			var n: Vector3 = (verts[i + 2] - verts[i]).cross(verts[i + 1] - verts[i])
			if n.length_squared() < 1e-12:
				continue
			n = n.normalized()
			if seen.has(key) and (seen[key] as Vector3).dot(n) < -0.9:
				count += 1
			else:
				seen[key] = n
	return count


func test_set_pieces_landmark_terrain_and_gate_home() -> void:
	for area: String in SLUGS:
		var zone: ZoneRoot = _zone(area)
		var anchors: Dictionary = _interactables(zone)
		var pieces: Dictionary = SET_PIECES[area]
		for piece: String in pieces:
			var spec: Array = pieces[piece] as Array
			var node: MeshInstance3D = zone.get_node_or_null("Static/" + piece) as MeshInstance3D
			assert_object(node).override_failure_message("%s %s" % [area, piece]).is_not_null()
			assert_str(node.mesh.resource_path).starts_with(ArtCatalog.MODELS_DIR)
			if not String(spec[0]).is_empty():
				var anchor: Node3D = anchors.get(String(spec[0])) as Node3D
				assert_object(anchor).override_failure_message(String(spec[0])).is_not_null()
				var flat: Vector2 = Vector2(node.global_position.x - anchor.global_position.x, node.global_position.z - anchor.global_position.z)
				assert_float(flat.length()).override_failure_message("%s %s far from %s" % [area, piece, spec[0]]).is_less_equal(float(spec[1]))
			var slots: Dictionary = _slots(node.mesh)
			for slot: String in spec[2] as Array:
				assert_bool(slots.has(slot)).override_failure_message("%s %s %s" % [area, piece, slot]).is_true()
		var landmark: Dictionary = _slots((zone.get_node("Static/Buildings") as MeshInstance3D).mesh)
		for slot: String in LANDMARK_SLOTS[area] as Array:
			assert_bool(landmark.has(slot)).override_failure_message("%s landmark %s" % [area, slot]).is_true()
		var ground: Dictionary = _slots((zone.get_node("Terrain/Ground") as MeshInstance3D).mesh)
		for slot: String in TERRAIN_SLOTS[area] as Array:
			assert_bool(ground.has(slot)).override_failure_message("%s terrain %s" % [area, slot]).is_true()
		for portal: Node in zone.get_node("Portals").get_children():
			assert_str((portal.get_node("Gate") as MeshInstance3D).mesh.resource_path).is_equal(ArtCatalog.gate_path("main_hub"))


func test_exam_tower_is_calm_and_arena_is_festive() -> void:
	var exam: ZoneRoot = _zone("exam_zone")
	var calm: Dictionary = _slots((exam.get_node("Static/Buildings") as MeshInstance3D).mesh)
	for piece: String in SET_PIECES["exam_zone"] as Dictionary:
		calm.merge(_slots((exam.get_node("Static/" + piece) as MeshInstance3D).mesh))
	for slot: String in LOUD_SLOTS:
		assert_bool(calm.has(slot)).override_failure_message("exam_zone shows %s" % slot).is_false()
	var arena: ZoneRoot = _zone("arena")
	var festive: Dictionary = _slots((arena.get_node("Static/Buildings") as MeshInstance3D).mesh)
	for piece: String in SET_PIECES["arena"] as Dictionary:
		festive.merge(_slots((arena.get_node("Static/" + piece) as MeshInstance3D).mesh))
	var count: int = 0
	for slot: String in FESTIVE_SLOTS:
		count += 1 if festive.has(slot) else 0
	assert_int(count).is_greater_equal(5)


func test_way_finding_signpost_and_glowing_pads() -> void:
	for area: String in SLUGS:
		var zone: ZoneRoot = _zone(area)
		var props: Dictionary = _slots((zone.get_node("Static/Props") as MeshInstance3D).mesh)
		# signpost: wooden arrow boards and the house icon of the way home (roof colour)
		for slot: String in ["wood", "wood_dark", "roof", "roof_dark"]:
			assert_bool(props.has(slot)).override_failure_message("%s signpost %s" % [area, slot]).is_true()
		var ground: Dictionary = _slots((zone.get_node("Terrain/Ground") as MeshInstance3D).mesh)
		assert_bool(ground.has("spark") and ground.has("lantern_glow")).override_failure_message(area).is_true()
	# the shared anchor marker is a big glowing gem with a halo (F094)
	var marker: ArrayMesh = load(ArtCatalog.MODELS_DIR + "/prop/prop_spark_marker_a.res") as ArrayMesh
	assert_float(marker.get_aabb().size.y).is_greater(1.5)
	assert_bool(_slots(marker).has("spark")).is_true()


func test_every_mesh_uses_a_generated_toon_preset() -> void:
	for area: String in SLUGS:
		var zone: ZoneRoot = _zone(area)
		var checked: int = 0
		for node: Node in zone.find_children("*", "GeometryInstance3D", true, false):
			if node is Label3D or not (node as GeometryInstance3D).is_visible_in_tree() or _inside_avatar(node):
				continue
			var geo: GeometryInstance3D = node as GeometryInstance3D
			var material: Material = geo.material_override
			if node is BlobShadow:
				continue
			var where: String = "%s %s" % [area, node.get_path()]
			assert_object(material).override_failure_message(where).is_not_null()
			assert_str(material.resource_path).override_failure_message(where).starts_with(GENERATED)
			var decor: bool = node.is_in_group(ZoneArtQuality.DECOR_GROUP) and material.next_pass == null
			var far: bool = node.name == &"Far" and geo.cast_shadow == GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
			if not (decor or far):
				assert_object(material.next_pass).override_failure_message(where).is_not_null()
			checked += 1
		assert_int(checked).is_greater(10)


func _inside_avatar(node: Node) -> bool:
	var parent: Node = node.get_parent()
	while parent != null:
		if parent is AvatarBuilder:
			return true
		parent = parent.get_parent()
	return false


func test_world_meshes_are_smooth_and_in_budget() -> void:
	for slug: String in SLUGS.values():
		var recipes: Dictionary = (ArtCatalog.ZONES[slug] as GDScript).call("meshes") as Dictionary
		var ground: int = 0
		for key: String in recipes:
			var path: String = "%s/%s.res" % [ArtCatalog.MODELS_DIR, key]
			var mesh: ArrayMesh = load(path) as ArrayMesh
			var tris: int = mesh.get_faces().size() / 3
			if key.begins_with("env/"):
				ground += tris
				continue
			assert_float(_smooth_share(mesh)).override_failure_message(path).is_greater(0.3)
			if key.begins_with("bld/") or key.ends_with("_set_a"):
				assert_int(tris).override_failure_message(key).is_less_equal(10000)
			elif key.begins_with("veg/"):
				assert_int(tris).override_failure_message(key).is_less_equal(2000)
			else:
				assert_int(tris).override_failure_message(key).is_less_equal(3000)
		assert_int(ground).override_failure_message(slug).is_less_equal(40000)


func test_outlined_meshes_have_no_two_sided_planes() -> void:
	var recipes: Dictionary = ArtCatalog.meshes()
	var checked: int = 0
	for path: String in recipes:
		var skip: bool = false
		for part: String in NO_OUTLINE:
			skip = skip or path.contains(part)
		if skip:
			continue
		var mesh: ArrayMesh = load(path) as ArrayMesh
		assert_int(two_sided_faces(mesh)).override_failure_message(path).is_equal(0)
		checked += 1
	assert_int(checked).is_greater(60)
	# the Hub areca palm (F093-3) is closed
	assert_int(two_sided_faces(load(ArtCatalog.veg_mesh_path("palm")) as ArrayMesh)).is_equal(0)


func test_anchors_equal_the_world_seed_layout() -> void:
	var seed: Dictionary = _seed()
	for area: String in SLUGS:
		var zone: ZoneRoot = _zone(area)
		var layout: Dictionary = (seed[area] as Dictionary)["layout"] as Dictionary
		var anchors: Dictionary = _interactables(zone)
		var expected: int = 0
		for it: Variant in layout.get("interactables", []) as Array:
			var item: Dictionary = it as Dictionary
			var node: Interactable = anchors.get(String(item["interactable_id"])) as Interactable
			assert_object(node).override_failure_message("%s %s" % [area, item["interactable_id"]]).is_not_null()
			assert_float(node.global_position.distance_to(_vec(item["position"]))).is_less(0.01)
			expected += 1
		for p: Variant in layout.get("portals", []) as Array:
			var portal: AreaPortal = anchors.get(String((p as Dictionary)["portal_id"])) as AreaPortal
			assert_object(portal).is_not_null()
			assert_str(portal.target_area_id).is_equal(String((p as Dictionary)["target_area_id"]))
			expected += 1
		assert_int(anchors.size()).override_failure_message(area).is_equal(expected)
		for s: Variant in layout["spawn_points"] as Array:
			var id: String = String((s as Dictionary)["spawn_point_id"])
			var found: bool = false
			for point: SpawnPoint in zone.spawn_points():
				found = found or (point.spawn_point_id == id and point.global_position.distance_to(_vec((s as Dictionary)["position"])) < 0.01)
			assert_bool(found).override_failure_message(id).is_true()
		var ground: CollisionShape3D = zone.get_node("Terrain/Collision") as CollisionShape3D
		assert_object((ground.shape as BoxShape3D).size).is_equal(Vector3(50, 1, 50))


func test_static_collision_keeps_spawn_anchors_and_lanes_reachable() -> void:
	for area: String in SLUGS:
		var zone: ZoneRoot = _zone(area)
		var shapes: Array[CollisionShape3D] = []
		for node: Node in zone.get_node("Static").get_children():
			if node is CollisionShape3D:
				shapes.append(node as CollisionShape3D)
		assert_int(shapes.size()).is_greater(4)
		var points: Array[Vector3] = []
		for point: SpawnPoint in zone.spawn_points():
			points.append(point.global_position)
		for node: Node in _interactables(zone).values():
			points.append((node as Node3D).global_position)
		for i: int in 61:
			points.append(Vector3(0, 0, 10.0 - i * 0.5))
		for lane: Array in LANES[area] as Array:
			var a: Vector3 = lane[0] as Vector3
			var b: Vector3 = lane[1] as Vector3
			for i: int in 41:
				points.append(a.lerp(b, float(i) / 40.0))
		for p: Vector3 in points:
			for shape: CollisionShape3D in shapes:
				assert_bool(_blocks(shape, p + Vector3.UP * 0.5)).override_failure_message("%s %s blocks %s" % [area, shape.name, p]).is_false()


static func _blocks(c: CollisionShape3D, p: Vector3) -> bool:
	var local: Vector3 = c.global_transform.affine_inverse() * p
	if c.shape is BoxShape3D:
		var h: Vector3 = (c.shape as BoxShape3D).size * 0.5
		return absf(local.x) < h.x + PLAYER_RADIUS and absf(local.z) < h.z + PLAYER_RADIUS and absf(local.y) < h.y + 0.5
	if c.shape is CylinderShape3D:
		var cyl: CylinderShape3D = c.shape as CylinderShape3D
		return Vector2(local.x, local.z).length() < cyl.radius + PLAYER_RADIUS and absf(local.y) < cyl.height * 0.5 + 0.5
	return false
