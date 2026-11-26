extends GdUnitTestSuite
## F067 Main Hub art pass (WORLD_DESIGN §4.1, §11; DESIGN_SYSTEM §5.3; ADR-0102):
## - AC-10: the square shows the Vietnamese-inspired set pieces of the hub — banyan Spark Tree, tiled-roof (mái ngói)
##   buildings, rice terraces, a basket boat (thuyền thúng) on the lotus pond, lanterns, wind gates with a shape icon
##   per destination and notice boards — as baked first-party meshes;
## - AC-05: every drawn mesh of the zone uses a generated toon preset; solid art has the outline pass;
## - AC-11: the gameplay contracts of F062/F071 stay: spawn points, interactables, portals and the static collision of
##   the plaza features are where the code expects them.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const GENERATED: String = "res://resources/art/generated/"


func _zone() -> Node:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(1)
	return runner.scene()


static func _slots(mesh: Mesh) -> Dictionary:
	var out: Dictionary = {}
	if mesh == null or mesh.get_surface_count() == 0:
		return out
	for uv: Vector2 in mesh.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]:
		if uv.y < 1.0:
			out[Art3dTokens.ART_PALETTE[int(uv.x)]] = true
	return out


func test_hub_has_the_vietnamese_set_pieces() -> void:
	var zone: Node = await _zone()
	var tree: Dictionary = _slots((zone.get_node("Static/SparkTree") as MeshInstance3D).mesh)
	for slot: String in ["trunk", "leaf_dark", "spark"]:
		assert_bool(tree.has(slot)).override_failure_message("banyan " + slot).is_true()
	var buildings: Dictionary = _slots((zone.get_node("Static/Buildings") as MeshInstance3D).mesh)
	for slot: String in ["roof", "roof_dark", "wall", "wall_yellow", "wood"]:
		assert_bool(buildings.has(slot)).override_failure_message("buildings " + slot).is_true()
	var terrain: Dictionary = _slots((zone.get_node("Terrain/Ground") as MeshInstance3D).mesh)
	for slot: String in ["rice_young", "rice_ripe", "water", "soil", "paving", "grass"]:
		assert_bool(terrain.has(slot)).override_failure_message("terrain " + slot).is_true()
	var props: Dictionary = _slots((zone.get_node("Static/Props") as MeshInstance3D).mesh)
	for slot: String in ["basket", "flower_pink", "lantern", "water"]:
		assert_bool(props.has(slot)).override_failure_message("props " + slot).is_true()
	var rice: ScatterInstance = zone.get_node("Ambient/Rice") as ScatterInstance
	assert_int(rice.multimesh.instance_count).is_greater(100)
	for board: String in ["SchoolAssignmentsBoard", "ExamBoard"]:
		var mesh: MeshInstance3D = zone.get_node("Interactables/%s/Board/Mesh" % board) as MeshInstance3D
		assert_str(mesh.mesh.resource_path).is_equal("res://assets/models/prop/prop_notice_board_a.res")


func test_each_wind_gate_shows_its_destination_icon() -> void:
	var zone: Node = await _zone()
	var meshes: Dictionary = {}
	for portal: Node in zone.get_node("Portals").get_children():
		var gate: MeshInstance3D = portal.get_node("Gate") as MeshInstance3D
		var target: String = String(portal.get("target_area_id"))
		assert_str(gate.mesh.resource_path).is_equal("res://assets/models/prop/prop_wind_gate_%s.res" % target.replace(".", "_"))
		meshes[gate.mesh.resource_path] = true
	assert_int(meshes.size()).is_equal(zone.get_node("Portals").get_child_count())
	assert_object(AreaPortal.gate_mesh("../../etc/passwd")).is_null()
	assert_object(AreaPortal.gate_mesh("unknown_area")).is_null()


func test_every_mesh_uses_a_generated_toon_preset() -> void:
	var zone: Node = await _zone()
	var checked: int = 0
	for node: Node in zone.find_children("*", "GeometryInstance3D", true, false):
		if node is Label3D or not (node as GeometryInstance3D).is_visible_in_tree():
			continue
		var geo: GeometryInstance3D = node as GeometryInstance3D
		var material: Material = geo.material_override
		if node.get_parent() is AvatarBuilder or _inside_avatar(node):
			continue
		if node is BlobShadow:
			# F068: the blob shadow has its own token material (test_blob_shadow.gd), not a toon preset.
			assert_object(material).is_same(BlobShadow.shared_material())
			continue
		assert_object(material).override_failure_message(str(node.get_path())).is_not_null()
		assert_str(material.resource_path).override_failure_message(str(node.get_path())).starts_with(GENERATED)
		var decor: bool = node.is_in_group(ZoneArtQuality.DECOR_GROUP) and material.next_pass == null
		# far scenery (F094, as in the F093 zones): no outline, no shadow
		var far: bool = node.name == &"Far" and geo.cast_shadow == GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		if not (decor or far):
			assert_object(material.next_pass).override_failure_message(str(node.get_path())).is_not_null()
		checked += 1
	assert_int(checked).is_greater(15)


func _inside_avatar(node: Node) -> bool:
	var parent: Node = node.get_parent()
	while parent != null:
		if parent is AvatarBuilder:
			return true
		parent = parent.get_parent()
	return false


func test_gameplay_anchors_and_plaza_collision_stay() -> void:
	var zone: Node = await _zone()
	var ids: PackedStringArray = []
	for node: Node in zone.find_children("*", "Interactable", true, false):
		ids.append(String((node as Interactable).interactable_id))
	for id: String in ["board.school_assignments", "board.exam_sessions", "npc.cloudy", "npc.postie", "npc.cotton",
			"discovery.hub.spark_tree", "portal.hub.to_math", "portal.hub.to_social"]:
		assert_bool(id in ids).override_failure_message(id).is_true()
	var spawns: PackedStringArray = []
	for point: SpawnPoint in (zone as ZoneRoot).spawn_points():
		spawns.append(point.spawn_point_id)
	assert_bool("main_hub.default" in spawns and "main_hub.from_arena" in spawns).is_true()
	for shape: String in ["FountainCollision", "SparkTreeCollision", "HallCollision", "LibraryCollision", "GateLeftCollision",
			"GateRightCollision", "HedgeNorthCollision", "HedgeSouthCollision", "HedgeEastCollision", "HedgeWestCollision"]:
		assert_object(zone.get_node_or_null("Static/" + shape)).override_failure_message(shape).is_not_null()
	# The đình hall sits behind the wind-gate row: the portal triggers (radius 1.6 at z = -20) stay reachable.
	var hall: CollisionShape3D = zone.get_node("Static/HallCollision") as CollisionShape3D
	var hall_front: float = hall.position.z + (hall.shape as BoxShape3D).size.z * 0.5
	assert_float(hall_front).is_less(-21.6)
	# The default spawn and the discovery trigger are outside every static collider.
	var default_spawn: Vector3 = Vector3(0, 0.1, 9)
	for shape: String in ["FountainCollision", "SparkTreeCollision"]:
		var c: CollisionShape3D = zone.get_node("Static/" + shape) as CollisionShape3D
		var r: float = (c.shape as CylinderShape3D).radius
		assert_float(Vector2(default_spawn.x - c.position.x, default_spawn.z - c.position.z).length()).is_greater(r + 0.5)
