extends SceneTree
## Review capture of a subject zone (F093 T004, DESIGN_SYSTEM §9, WORLD_DESIGN §4.2, §11): renders the zone of
## `--area` from fixed camera shots of the shared subject-zone layout (tools/art/zone_kit.gd) to JPEG/PNG, before and
## after the art pass, on the renderer of the run. Needs a renderer (not --headless):
##   godot --path src/game --rendering-method mobile -s res://tools/capture_zone.gd -- --area=learning_zone.math \
##         --out=<dir> [--tag=after] [--preset=medium] [--shots=a,b] [--format=jpg]
## Display only: the zone runs non-interactive data (a sample avatar on the local player), no network, no server state.

const ZONES_DIR: String = "res://scenes/world/zones"
const SETTLE_FRAMES: int = 12
## Camera shots of the subject-zone layout: name → [position, look-at target, vertical FOV].
const SHOTS: Dictionary = {
	"overview": [Vector3(0, 22, 30), Vector3(0, 0, -8), 55.0],
	"player": [Vector3(0, 3.4, 15.5), Vector3(0, 1.4, 2), 60.0],
	"east": [Vector3(4, 4.5, 2), Vector3(14, 1.0, -7), 60.0],
	"west": [Vector3(-4, 4.5, 2), Vector3(-14, 1.0, -7), 60.0],
	"north": [Vector3(5, 4, -3), Vector3(0, 3.0, -26), 60.0],
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
		printerr("capture_zone: needs a renderer (run without --headless)")
		quit(2)
		return
	var area: String = _arg("area", "learning_zone.math")
	var scene_path: String = "%s/%s/Zone.tscn" % [ZONES_DIR, area.replace(".", "_")]
	if RegEx.create_from_string("^[a-z0-9_.]{1,64}$").search(area) == null or not ResourceLoader.exists(scene_path):
		printerr("capture_zone: unknown area %s" % area)
		quit(2)
		return
	var out_dir: String = _arg("out", ProjectSettings.globalize_path("user://capture_zone"))
	var tag: String = _arg("tag", "after")
	var preset: String = _arg("preset", "medium")
	var format: String = _arg("format", "jpg")
	var only: PackedStringArray = _arg("shots", "").split(",", false)
	DirAccess.make_dir_recursive_absolute(out_dir)
	root.size = Vector2i(1280, 720)
	var quality: Node = root.get_node("QualityManager")
	quality.call("apply_preset", preset, false)
	var zone: Node3D = (load(scene_path) as PackedScene).instantiate() as Node3D
	root.add_child(zone)
	for i: int in SETTLE_FRAMES:
		await process_frame
	_dress_player(zone)
	var camera: Camera3D = Camera3D.new()
	camera.name = "CaptureCamera"
	zone.add_child(camera)
	camera.make_current()
	var slug: String = area.replace("learning_zone.", "").replace(".", "_")
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
		var path: String = out_dir.path_join("%s_%s_%s.%s" % [slug, tag, shot, format])
		if format == "png":
			image.save_png(path)
		else:
			image.save_jpg(path, 0.88)
		count += 1
	print("capture_zone: %d images (%s %s, preset %s, %s) in %s" % [
		count, area, tag, preset, RenderingServer.get_current_rendering_method(), out_dir])
	quit(0)


func _dress_player(zone: Node3D) -> void:
	var player: Node = zone.get("player") as Node
	if player == null:
		return
	var rig: AvatarBuilder = player.get_node_or_null("AvatarRig") as AvatarBuilder
	if rig != null:
		rig.apply(AvatarConfigModel.from_dict(SAMPLE_AVATAR))
		rig.rotation.y = PI
