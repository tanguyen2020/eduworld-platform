extends GdUnitTestSuite
## F092 wiring in WorldRoot (F092-AC-10, AC-11, AC-14, AC-15, AC-16, AC-17): entering the Main Hub joins its instance,
## `hub.instance.assigned` puts the local player on the server spawn and draws the others (RemotePlayers under the
## zone), the local movement goes out as `hub.player.move`, a server correction is blended back, the emote wheel
## sends `hub.player.emote` and a refused join shows a friendly HUD notice. Glue only — the rules are in HubSync.

const WORLD: String = "res://scenes/world/WorldRoot.tscn"
const SettingsScript := preload("res://scripts/core/settings.gd")
const SETTINGS_PATH: String = "user://test_f092_world_settings.cfg"
const OVERRIDE_PATH: String = "user://test_f092_world_renderer.cfg"

var auth: AuthHarness = null
var rt: RealtimeHarness = null
var _services: ServiceLocator = null
var _settings: Node = null
var _saved_settings: Node = null
var _saved_override: String = ""
var _locator: ServiceLocator = null


func before_test() -> void:
	DirAccess.remove_absolute(SETTINGS_PATH)
	_settings = SettingsScript.new() as Node
	_settings.call("load_from", SETTINGS_PATH)
	_settings.set("quality_preset", QualityPolicy.MEDIUM)
	_saved_settings = QualityManager.settings
	QualityManager.settings = _settings
	_saved_override = QualityManager.override_path
	QualityManager.override_path = OVERRIDE_PATH
	_services = AppState.services
	auth = AuthHarness.create()
	await auth.sign_in()
	rt = RealtimeHarness.create()
	_locator = ServiceLocator.new(auth.api, AppState, auth.clock, rt.client)
	await _locator.profile.fetch_profile()
	auth.http.reply(ProfileFixtures.ROUTE_CATALOG, 200, ProfileFixtures.catalog(), {"ETag": ProfileFixtures.CATALOG_ETAG})
	await _locator.inventory.load_catalog()
	AppState.services = _locator


func after_test() -> void:
	_locator.hub.dispose()
	rt.dispose()
	auth.dispose()
	AppState.services = _services
	QualityManager.settings = _saved_settings
	QualityManager.override_path = _saved_override
	DirAccess.remove_absolute(OVERRIDE_PATH)
	QualityManager.apply_preset(QualityPolicy.MEDIUM, false)
	_settings.free()
	DirAccess.remove_absolute(SETTINGS_PATH)


func _world() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(WORLD)
	await runner.simulate_frames(2)
	return [runner, runner.scene()]


func _player(world: Node) -> PlayerController:
	return (world.get("zone") as ZoneRoot).player as PlayerController


func _join(world_runner: GdUnitSceneRunner) -> void:
	await rt.connect_online()
	await world_runner.simulate_frames(1)
	rt.ack_last("hub.instance.join")
	rt.push_fixture("hub.instance.assigned.main_hub")
	await world_runner.simulate_frames(1)


func test_entering_the_hub_joins_spawns_and_draws_the_others() -> void:
	var r: Array = await _world()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var world: Node = r[1] as Node
	await _join(runner)
	var join: Dictionary = rt.peer().sent_of("hub.instance.join")[0]
	assert_str(String((join["payload"] as Dictionary)["area_id"])).is_equal("main_hub")
	var player: PlayerController = _player(world)
	player.set_physics_process(false)
	assert_float(player.global_position.distance_to(Vector3(0, 0, 4))).is_less(0.01)
	var remotes: RemotePlayerSync = world.get("remote_players") as RemotePlayerSync
	assert_object(remotes.player_for(3)).is_not_null()
	assert_str(remotes.player_for(3).nickname_label.text).is_equal("CuMeoChamChi12")
	# The local state goes out as hub.player.move.
	player.global_position = Vector3(1, 0, 4)
	await runner.simulate_frames(2)
	assert_int(rt.peer().sent_of("hub.player.move").size()).is_greater(0)


func test_correction_is_blended_and_emotes_reach_the_others() -> void:
	var r: Array = await _world()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var world: Node = r[1] as Node
	await _join(runner)
	var player: PlayerController = _player(world)
	player.set_physics_process(false)
	player.global_position = Vector3(10, 0, 4)
	rt.push_fixture("hub.player.corrected.speed")
	world.call("_sync_local", 1.0 / 60.0)
	# One frame later the player has moved part of the way, not teleported.
	assert_float(player.global_position.distance_to(Vector3(1.2, 0, 3.4))).is_greater(1.0)
	for i: int in 20:
		world.call("_sync_local", 1.0 / 60.0)
	assert_float(player.global_position.distance_to(Vector3(1.2, 0, 3.4))).is_less(0.01)
	var wheel: EmoteWheel = world.find_child("EmoteWheel", true, false) as EmoteWheel
	wheel.choose("emote_wave")
	assert_int(rt.peer().sent_of("hub.player.emote").size()).is_equal(1)


func test_refused_join_shows_a_notice_and_stays() -> void:
	var r: Array = await _world()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var world: Node = r[1] as Node
	await rt.connect_online()
	await runner.simulate_frames(1)
	rt.ack_last("hub.instance.join", false, "ROOM_FULL")
	await runner.simulate_frames(1)
	var notice: Label = world.find_child("HubNotice", true, false) as Label
	assert_bool(notice.visible).is_true()
	assert_str(notice.text).is_equal("ui.hub.room_full")
	assert_str(String(AppState.current_zone)).is_equal("main_hub")
