extends GdUnitTestSuite
## F070 wiring in WorldRoot (F070-AC-18, F070-AC-20, T030, T032): the local Player is drawn from the profile's
## AvatarConfig (fallback visuals included), the emote wheel lists the equipped emotes and plays them on the Player,
## the HUD "Tủ đồ" button opens the `wardrobe` screen, and a saved avatar redraws the Player and refills the wheel.

const WORLD: String = "res://scenes/world/WorldRoot.tscn"
const SettingsScript := preload("res://scripts/core/settings.gd")
const SETTINGS_PATH: String = "user://test_f070_world_settings.cfg"
const OVERRIDE_PATH: String = "user://test_f070_world_renderer.cfg"

var h: AuthHarness = null
var _services: ServiceLocator = null
var _settings: Node = null
var _saved_settings: Node = null
var _saved_override: String = ""


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
	h = AuthHarness.create()
	await h.sign_in()
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	await services.profile.fetch_profile()
	h.http.reply(ProfileFixtures.ROUTE_CATALOG, 200, ProfileFixtures.catalog(), {"ETag": ProfileFixtures.CATALOG_ETAG})
	await services.inventory.load_catalog()
	AppState.services = services


func after_test() -> void:
	h.dispose()
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


func test_player_wears_the_profile_avatar_and_the_wheel_lists_equipped_emotes() -> void:
	var r: Array = await _world()
	var world: Node = r[1] as Node
	var rig: AvatarBuilder = _player(world).avatar as AvatarBuilder
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_casual_01")
	assert_str(String(rig.shown["face"])).is_equal("acc_glasses_round")
	var wheel: EmoteWheel = world.find_child("EmoteWheel", true, false) as EmoteWheel
	assert_array(Array(wheel.shown_emotes())).contains_exactly(["emote_wave", "emote_clap", "emote_dance"])
	assert_object(wheel.anim_driver).is_same(_player(world).anim_driver)
	wheel.open()
	wheel.slot_buttons()[2].pressed.emit()
	assert_str(_player(world).anim_driver.emote_id).is_equal("emote_dance")


func test_hud_button_opens_the_wardrobe_and_a_saved_avatar_redraws_the_player() -> void:
	var one: Array[int] = [1]
	h.http.reply(ProfileFixtures.ROUTE_PRESETS, 200, ProfileFixtures.preset_list(one))
	var r: Array = await _world()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var world: Node = r[1] as Node
	# F069: the HUD Tủ đồ button opens S08 (tab Tủ đồ); its "Phối đồ" opens the F070 CustomizationStudio.
	(world.find_child("WardrobeButton", true, false) as Button).pressed.emit()
	await runner.simulate_frames(6)
	assert_str(String(world.get("open_screen"))).is_equal("inventory")
	(world.find_child("Customize", true, false) as Button).pressed.emit()
	await runner.simulate_frames(6)
	assert_str(String(world.get("open_screen"))).is_equal("wardrobe")
	var studio: Control = world.find_child("CustomizationStudio", true, false) as Control
	assert_bool(studio.visible).is_true()
	var saved: AvatarConfigModel = AppState.get_services().profile.avatar_config().copy()
	saved.outfit_item_id = "outfit_star_01"
	saved.emote_slots = PackedStringArray(["emote_thank"])
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, saved.to_dict())
	await AppState.get_services().profile.save_avatar(saved)
	var rig: AvatarBuilder = _player(world).avatar as AvatarBuilder
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_star_01")
	var wheel: EmoteWheel = world.find_child("EmoteWheel", true, false) as EmoteWheel
	assert_array(Array(wheel.shown_emotes())).contains_exactly(["emote_thank"])
