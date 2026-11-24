extends SceneTree
## Review capture of the Main Hub art (F067 T020, DESIGN_SYSTEM §9, WORLD_DESIGN §11): renders the hub zone from fixed
## camera shots to PNG so the look can be reviewed and kept as evidence. Needs a renderer (not --headless):
##   godot --path src/game -s res://tools/capture_hub.gd -- --out=<dir> [--tag=after] [--preset=medium] [--shots=a,b]
##   godot --path src/game --rendering-method gl_compatibility -s res://tools/capture_hub.gd -- --out=<dir> --tag=compat
## Display only: the zone runs non-interactive data (a sample avatar on the local player), no network, no server state.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const SETTLE_FRAMES: int = 12
## Camera shots: name → [position, look-at target, vertical FOV].
const SHOTS: Dictionary = {
	"overview": [Vector3(0, 21, 31), Vector3(0, 0, -5), 55.0],
	"player": [Vector3(0, 3.4, 15.5), Vector3(0, 1.4, 4), 60.0],
	"plaza_east": [Vector3(21, 7, 13), Vector3(0, 1.5, -6), 55.0],
	"characters": [Vector3(2.6, 1.5, 11.2), Vector3(0, 0.95, 7.6), 45.0],
	"portals": [Vector3(-14, 9, 4), Vector3(-6, 1.5, -20), 60.0],
}
## Sample look of the local player (catalog codes of the F070 fixture; display only).
const SAMPLE_AVATAR: Dictionary = {
	"body_type": "kid_a", "skin_tone": "tone_03", "hair": {"item_id": "hair_bob_01", "color": "hair_col_02"},
	"outfit": {"item_id": "outfit_school_01"}, "accessories": [{"slot": "back", "item_id": "acc_backpack_owl"}],
}


func _initialize() -> void:
	_run.call_deferred()


func _arg(name: String, fallback: String) -> String:
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--%s=" % name):
			return arg.trim_prefix("--%s=" % name)
	return fallback


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("capture_hub: needs a renderer (run without --headless)")
		quit(2)
		return
	var out_dir: String = _arg("out", ProjectSettings.globalize_path("user://capture_hub"))
	var tag: String = _arg("tag", "after")
	var preset: String = _arg("preset", "medium")
	var only: PackedStringArray = _arg("shots", "").split(",", false)
	DirAccess.make_dir_recursive_absolute(out_dir)
	root.size = Vector2i(1280, 720)
	var quality: Node = root.get_node("QualityManager")
	quality.call("apply_preset", preset, false)
	var zone: Node3D = (load(ZONE) as PackedScene).instantiate() as Node3D
	root.add_child(zone)
	for i: int in SETTLE_FRAMES:
		await process_frame
	_dress_player(zone)
	var camera: Camera3D = Camera3D.new()
	camera.name = "CaptureCamera"
	zone.add_child(camera)
	camera.make_current()
	var count: int = 0
	for shot: String in SHOTS:
		if not only.is_empty() and not shot in only:
			continue
		var spec: Array = SHOTS[shot] as Array
		camera.fov = float(spec[2])
		camera.global_position = spec[0] as Vector3
		camera.look_at(spec[1] as Vector3, Vector3.UP)
		for i: int in SETTLE_FRAMES:
			await process_frame
		await RenderingServer.frame_post_draw
		var image: Image = root.get_texture().get_image()
		var path: String = out_dir.path_join("hub_%s_%s.png" % [tag, shot])
		image.save_png(path)
		count += 1
	print("capture_hub: %d images (%s, preset %s, %s) in %s" % [
		count, tag, preset, RenderingServer.get_current_rendering_method(), out_dir])
	quit(0)


func _dress_player(zone: Node3D) -> void:
	var player: Node = zone.get("player") as Node
	if player == null:
		return
	var rig: AvatarBuilder = player.get_node_or_null("AvatarRig") as AvatarBuilder
	if rig != null:
		rig.apply(AvatarConfigModel.from_dict(SAMPLE_AVATAR))
		rig.rotation.y = PI
