extends GdUnitTestSuite
## F071-AC-12 client part (T022): `SceneRouter.request_area` asks the server (`enterWorldArea`), checks the zone bundle
## (`ensure_bundle`, greybox scenes in the build — PLACEHOLDER R17) and announces the swap with the server spawn; a
## refusal goes to `area_change_failed` with the missing requirements; an unknown spawn point falls back to `default`
## with a warning (ZoneRoot). The transition budget check logs over-budget transitions.

var h: AuthHarness = null
var store: WorldFixtures.FakeStore = null
var world: WorldService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	store = auto_free(WorldFixtures.FakeStore.new())
	world = WorldService.new(h.api, store)
	h.http.reply(WorldFixtures.ROUTE_AREAS, 200, WorldFixtures.areas())
	await world.fetch_areas()


func after_test() -> void:
	h.dispose()
	AppState.spawn_point_id = ""
	AppState.current_zone = ""


func test_accepted_area_is_announced_with_the_server_spawn() -> void:
	h.http.reply(WorldFixtures.ROUTE_ENTER, 200, WorldFixtures.enter_math())
	var ready: Array = []
	var on_ready: Callable = func(id: String, scene: String, spawn: String, _e: Dictionary) -> void: ready.append([id, scene, spawn])
	SceneRouter.area_entered.connect(on_ready)
	var err: Error = await SceneRouter.request_area("learning_zone.math", "portal.hub.to_math", world)
	SceneRouter.area_entered.disconnect(on_ready)
	assert_int(err).is_equal(OK)
	assert_array(ready).has_size(1)
	assert_str(String(ready[0][1])).is_equal("res://scenes/world/zones/learning_zone_math/Zone.tscn")
	assert_str(String(ready[0][2])).is_equal("learning_zone.math.default")
	assert_str(AppState.current_zone).is_equal("learning_zone.math")
	assert_bool(SceneRouter.is_changing_area()).is_false()


func test_locked_area_reports_the_missing_requirements() -> void:
	h.http.reply(WorldFixtures.ROUTE_ENTER, 403, WorldFixtures.area_locked())
	var failed: Array = []
	var on_failed: Callable = func(id: String, code: String, missing: Array) -> void: failed.append([id, code, missing.size()])
	SceneRouter.area_change_failed.connect(on_failed)
	var err: Error = await SceneRouter.request_area("challenge_mountain", "", world)
	SceneRouter.area_change_failed.disconnect(on_failed)
	assert_int(err).is_equal(FAILED)
	assert_array(failed).has_size(1)
	assert_str(String(failed[0][1])).is_equal("AREA_LOCKED")
	assert_int(int(failed[0][2])).is_equal(1)


func test_every_area_of_the_seed_has_a_loadable_zone() -> void:
	for a: WorldAreaModel in world.areas:
		assert_bool(SceneRouter.ensure_bundle(a.scene_ref)).override_failure_message(a.scene_ref).is_true()
		var zone: ZoneRoot = ZoneLoader.instantiate(a.scene_ref)
		assert_object(zone).is_not_null()
		assert_str(zone.area_id).is_equal(a.area_id)
		zone.free()
	assert_bool(SceneRouter.ensure_bundle("res://scenes/world/zones/nowhere/Zone.tscn")).is_false()


func test_unknown_spawn_falls_back_to_default() -> void:
	var zone: ZoneRoot = ZoneLoader.instantiate("res://scenes/world/zones/learning_zone_math/Zone.tscn")
	zone.interactive = false
	add_child(zone)
	var point: SpawnPoint = zone.find_spawn("learning_zone.math.nowhere")
	assert_str(point.spawn_point_id).is_equal("learning_zone.math.default")
	zone.queue_free()


func test_transition_budget() -> void:
	assert_bool(ZoneLoader.check_budget("learning_zone.math", 1200)).is_true()
	assert_bool(ZoneLoader.check_budget("learning_zone.math", int(SceneRouter.AREA_TRANSITION_MAX_SEC * 1000.0) + 1)).is_false()
