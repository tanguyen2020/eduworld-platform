extends SceneTree
## Review capture of the chibi anime cartoon characters (F068 T030, ADR-0103, DESIGN_SYSTEM §9): renders avatars in
## the Main Hub light to PNG — every hair style / outfit / skin tone, the accessories, the body types, the facial
## expressions, one pose of every animation clip, and the 5 growth stages of the Spark Tree. Needs a renderer (not --headless):
##   godot --path src/game -s res://tools/capture_avatars.gd -- --out=<dir> [--tag=after] [--shots=lineup,poses]
##   godot --path src/game --rendering-method gl_compatibility -s res://tools/capture_avatars.gd -- --out=<dir> --tag=web
## Display only: sample avatar configs of the F070 fixture catalog codes, no network, no server state.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const RIG: String = "res://scenes/character/AvatarRig.tscn"
const SETTLE_FRAMES: int = 10
## Where the avatars stand (south part of the plaza, the Spark Tree behind them) and the spacing between them.
const ORIGIN: Vector3 = Vector3(0, 0, 14.5)
const SPACING: float = 0.95
const HAIRS: PackedStringArray = ["short", "bob", "ponytail", "braids", "curly", "spiky"]
const OUTFITS: PackedStringArray = ["school", "casual", "sport", "star", "rain", "explorer"]
const HAIR_COLORS: PackedStringArray = ["hair_col_01", "hair_col_02", "hair_col_03", "hair_col_04", "hair_col_05",
	"hair_col_06", "hair_col_07", "hair_col_08"]
const ACCESSORIES: Array = [["head", "acc_cap_star"], ["face", "acc_glasses_round"], ["back", "acc_backpack_owl"],
	["hand", "acc_flag_small"]]
const CharacterPoses := preload("res://tools/art/character_anim.gd")
## Clip → sampled time (s) of the pose shot.
const POSES: Dictionary = {
	"idle": 1.2, "walk": 0.2, "run": 0.12, "jump": 0.2, "wave": 0.55, "celebrate": 0.4, "clap": 0.44, "dance": 0.2,
	"thank": 0.6, "apologize": 0.8,
}

var _zone: Node3D = null
var _camera: Camera3D = null
var _rigs: Array[AvatarBuilder] = []
var _extras: Array[Node3D] = []


func _initialize() -> void:
	_run.call_deferred()


func _arg(arg_name: String, fallback: String) -> String:
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--%s=" % arg_name):
			return arg.trim_prefix("--%s=" % arg_name)
	return fallback


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("capture_avatars: needs a renderer (run without --headless)")
		quit(2)
		return
	var out_dir: String = _arg("out", ProjectSettings.globalize_path("user://capture_avatars"))
	var tag: String = _arg("tag", "after")
	var only: PackedStringArray = _arg("shots", "").split(",", false)
	DirAccess.make_dir_recursive_absolute(out_dir)
	root.size = Vector2i(1280, 720)
	(root.get_node("QualityManager")).call("apply_preset", _arg("preset", "medium"), false)
	_zone = (load(ZONE) as PackedScene).instantiate() as Node3D
	root.add_child(_zone)
	for i: int in SETTLE_FRAMES:
		await process_frame
	var player: Node3D = _zone.get("player") as Node3D
	if player != null:
		player.visible = false
		player.process_mode = Node.PROCESS_MODE_DISABLED
	_camera = Camera3D.new()
	_zone.add_child(_camera)
	_camera.make_current()
	var shots: Dictionary = {
		"lineup": _lineup, "accessories": _accessories, "body_types": _body_types, "expressions": _expressions,
		"poses_move": _poses.bind(CharacterPoses.LOCOMOTION), "poses_emotes": _poses.bind(CharacterPoses.EMOTES),
		"closeup": _closeup, "spark_stages": _spark_stages,
	}
	var count: int = 0
	for shot: String in shots:
		if not only.is_empty() and not shot in only:
			continue
		_clear()
		(shots[shot] as Callable).call()
		for i: int in SETTLE_FRAMES:
			await process_frame
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(out_dir.path_join("avatars_%s_%s.png" % [tag, shot]))
		count += 1
	print("capture_avatars: %d images (%s, %s) in %s" % [count, tag, RenderingServer.get_current_rendering_method(),
		out_dir])
	quit(0)


func _clear() -> void:
	for rig: AvatarBuilder in _rigs:
		rig.queue_free()
	_rigs.clear()
	for extra: Node3D in _extras:
		extra.queue_free()
	_extras.clear()
	for shown: String in ["Ambient", "Static", "Interactables"]:
		(_zone.get_node(shown) as Node3D).visible = true


## One avatar at slot `i` of `n` in the row, facing the camera (+Z).
func _rig(i: int, n: int, config: Dictionary, spacing: float = SPACING) -> AvatarBuilder:
	var rig: AvatarBuilder = (load(RIG) as PackedScene).instantiate() as AvatarBuilder
	_zone.add_child(rig)
	rig.position = ORIGIN + Vector3((float(i) - float(n - 1) * 0.5) * spacing, 0, 0)
	rig.rotation.y = PI
	rig.apply(AvatarConfigModel.from_dict(config))
	rig.set_process(false)
	_rigs.append(rig)
	return rig


func _config(i: int, extra: Array = []) -> Dictionary:
	var hair: String = HAIRS[i % HAIRS.size()]
	return {
		"body_type": "kid_a", "skin_tone": "tone_0%d" % (i % 6 + 1),
		"hair": {"item_id": "hair_%s_01" % hair, "color": HAIR_COLORS[i % HAIR_COLORS.size()]},
		"outfit": {"item_id": "outfit_%s_01" % OUTFITS[i % OUTFITS.size()]}, "accessories": extra,
	}


func _frame(width: float, height: float = 1.3, fov: float = 28.0) -> void:
	_camera.fov = fov
	var aspect: float = float(root.size.x) / float(root.size.y)
	var dist: float = maxf(width / aspect, height * 1.5) * 0.5 / tan(deg_to_rad(fov) * 0.5) + 0.8
	_camera.global_position = ORIGIN + Vector3(0, height * 0.75, dist)
	_camera.look_at(ORIGIN + Vector3(0, height * 0.5, 0), Vector3.UP)


func _lineup() -> void:
	for i: int in 6:
		_rig(i, 6, _config(i))
	_frame(6.0 * SPACING)


func _accessories() -> void:
	for i: int in ACCESSORIES.size():
		var acc: Array = ACCESSORIES[i] as Array
		var rig: AvatarBuilder = _rig(i, 4, _config(i + 1, [{"slot": acc[0], "item_id": acc[1]}]), 1.1)
		if acc[0] == "back":
			rig.rotation.y = PI * 0.25
	_frame(4.4)


func _body_types() -> void:
	for i: int in 4:
		var config: Dictionary = _config(i + 2)
		config["body_type"] = "kid_%s" % "abcd"[i]
		_rig(i, 4, config, 1.05)
	_frame(4.2)


func _expressions() -> void:
	for i: int in KidSkeleton.EXPRESSIONS.size():
		var rig: AvatarBuilder = _rig(i, 4, _config(i), 0.8)
		rig.set_expression(KidSkeleton.EXPRESSIONS[i])
	_camera.fov = 30.0
	_camera.global_position = ORIGIN + Vector3(0, 1.0, 3.4)
	_camera.look_at(ORIGIN + Vector3(0, 0.85, 0), Vector3.UP)


func _closeup() -> void:
	_rig(0, 1, _config(1, [{"slot": "back", "item_id": "acc_backpack_owl"}, {"slot": "hand", "item_id": "acc_flag_small"}]))
	_camera.fov = 30.0
	_camera.global_position = ORIGIN + Vector3(0.5, 0.95, 2.3)
	_camera.look_at(ORIGIN + Vector3(0, 0.7, 0), Vector3.UP)


func _poses(clips: PackedStringArray) -> void:
	for i: int in clips.size():
		var clip_name: String = clips[i]
		var rig: AvatarBuilder = _rig(i, clips.size(), _config(i + (4 if clips.size() > 4 else 0)), 0.95)
		var player: AnimationPlayer = rig.get_node("AnimationPlayer") as AnimationPlayer
		var library: AnimationLibrary = AnimationLibrary.new()
		library.add_animation(&"pose", load(KidSkeleton.clip_path(clip_name)) as Animation)
		player.add_animation_library(&"capture", library)
		player.play(&"capture/pose")
		player.seek(float(POSES[clip_name]), true)
		player.pause()
		var expression: String = {"run": "trying", "jump": "surprised", "apologize": "trying"}.get(clip_name, "happy")
		rig.set_expression("neutral" if clip_name in ["idle", "walk"] else expression)
	_frame(float(clips.size()) * 0.95 + 0.4, 1.6)


## The five Spark Tree stages side by side on the south lawn (meshes are authored around the hub tree centre).
func _spark_stages() -> void:
	var tree: SparkTree = _zone.get_node("Static/SparkTree") as SparkTree
	for hide: String in ["Ambient", "Static", "Interactables"]:
		(_zone.get_node(hide) as Node3D).visible = false  # only the row of stages on the plaza ground
	for i: int in tree.stage_meshes.size():
		var mi: MeshInstance3D = MeshInstance3D.new()
		mi.mesh = tree.stage_meshes[i]
		mi.material_override = tree.material_override
		_zone.add_child(mi)
		mi.position = Vector3((float(i) - 2.0) * 8.5, 0, 4.0 + 7.4)
		_extras.append(mi)
	_camera.fov = 50.0
	_camera.global_position = Vector3(0, 6.0, 37)
	_camera.look_at(Vector3(0, 3.6, 4), Vector3.UP)
