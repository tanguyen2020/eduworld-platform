extends GdUnitTestSuite
## F062-AC-01 (FR-GAME-003, WORLD_RENDERING §2/§4; T007) and F062-AC-07 entry refresh (T021): the Main Hub greybox has
## the standard zone tree, places the local player at the server's spawn point or, when that id is not in the scene,
## at `main_hub.default` with a warning; entering the zone and closing a panel reload "Nhà trường giao".

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const ROUTE: String = "GET /learning/my-assignments"
const ZONE_NODES: PackedStringArray = [
	"Environment", "Terrain", "Static", "Navigation", "Occluders", "SpawnPoints", "Interactables", "Portals",
	"Players", "Ambient",
]

var _services: ServiceLocator = null


func before_test() -> void:
	_services = AppState.services
	AppState.spawn_point_id = ""
	Log.clear()


func after_test() -> void:
	AppState.services = _services
	AppState.spawn_point_id = ""
	AppState.player = {}


func _spawn(id: String) -> Array:
	AppState.spawn_point_id = id
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(2)
	var zone: ZoneRoot = runner.scene() as ZoneRoot
	return [runner, zone]


func _marker(zone: ZoneRoot, id: String) -> SpawnPoint:
	for point: SpawnPoint in zone.spawn_points():
		if point.spawn_point_id == id:
			return point
	return null


func test_zone_has_the_standard_tree_and_the_assignments_board() -> void:
	var r: Array = await _spawn("")
	var zone: ZoneRoot = r[1] as ZoneRoot
	for node_name: String in ZONE_NODES:
		assert_object(zone.get_node_or_null(node_name)).override_failure_message(node_name).is_not_null()
	assert_str(zone.area_id).is_equal("main_hub")
	assert_str(AppState.current_zone).is_equal("main_hub")
	var board: Interactable = zone.get_node("Interactables/SchoolAssignmentsBoard") as Interactable
	assert_str(board.interactable_id).is_equal("board.school_assignments")
	assert_str(board.kind).is_equal("quest_board")
	assert_str(board.prompt_key).starts_with("ui.hub.action.")
	assert_object(zone.player).is_not_null()
	assert_bool(zone.player.is_in_group(Interactable.PLAYER_GROUP)).is_true()


func test_player_spawns_at_the_default_point() -> void:
	var r: Array = await _spawn("")
	var zone: ZoneRoot = r[1] as ZoneRoot
	var point: SpawnPoint = _marker(zone, "main_hub.default")
	assert_float(zone.player.global_position.distance_to(point.global_position)).is_less_equal(point.radius + 0.5)


func test_player_spawns_at_the_server_spawn_point() -> void:
	var r: Array = await _spawn("main_hub.from_arena")
	var zone: ZoneRoot = r[1] as ZoneRoot
	var point: SpawnPoint = _marker(zone, "main_hub.from_arena")
	assert_float(zone.player.global_position.distance_to(point.global_position)).is_less_equal(point.radius + 0.5)


func test_unknown_spawn_point_falls_back_to_default_with_a_warning() -> void:
	var r: Array = await _spawn("main_hub.removed_in_v2")
	var zone: ZoneRoot = r[1] as ZoneRoot
	var point: SpawnPoint = _marker(zone, "main_hub.default")
	assert_float(zone.player.global_position.distance_to(point.global_position)).is_less_equal(point.radius + 0.5)
	var warned: bool = false
	for entry: Dictionary in Log.entries():
		if String(entry["event"]) == "spawn_point_missing" and String(entry["level"]) == "warn":
			warned = true
	assert_bool(warned).is_true()


func test_return_home_puts_the_player_back_on_the_default_spawn() -> void:
	var r: Array = await _spawn("main_hub.from_arena")
	var zone: ZoneRoot = r[1] as ZoneRoot
	zone.return_home()
	var point: SpawnPoint = _marker(zone, "main_hub.default")
	assert_float(zone.player.global_position.distance_to(point.global_position)).is_less_equal(point.radius + 0.5)


## F062-AC-07: entering the Hub and returning from a panel (EventBus.ui_closed) reload the assignments.
func test_entering_and_returning_to_the_hub_refreshes_assignments() -> void:
	var h: AuthHarness = AuthHarness.create()
	await h.sign_in()
	AppState.services = ServiceLocator.new(h.api, AppState, h.clock)
	h.http.reply(ROUTE, 200, AssignmentFixtures.two())
	var r: Array = await _spawn("")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	await runner.simulate_frames(5)
	assert_int(h.http.count(ROUTE)).is_equal(1)
	assert_int(AppState.assignments.size()).is_equal(2)
	EventBus.ui_closed.emit(&"quest_journal")
	await runner.simulate_frames(5)
	assert_int(h.http.count(ROUTE)).is_equal(2)
	h.dispose()
