extends SceneTree
## Thumbnail/turntable renderer of 3D game assets (F078 US5, FR-CMS-017, DESIGN_SYSTEM §7, research R2).
##   godot --path src/game --script res://tools/thumbnail_renderer/render_thumbnails.gd -- \
##     --input /abs/model.glb --output /abs/out --kind render_thumbnail|render_turntable [--size 512] [--frames 1|24] \
##     [--background transparent|environment] [--environment hub]
## Loads the model at run time (GLTFDocument, absolute file path, no import), centres it, frames it with a camera fitted
## to its bounding sphere (3/4 view), lights it with the game sun colour and the game zone Environment (both generated
## from design/game-tokens, no colour is hard-coded here) and renders through a size×size SubViewport:
##   render_thumbnail → <output>/thumbnail.png (1 frame); render_turntable → <output>/turntable.png (frames rotating
##   360°/frames around Y, sprite sheet of RenderPlan.TURNTABLE_COLUMNS columns: 24 × 512 → 3072×2048).
## The background is transparent by default (alpha PNG: the CMS draws it on its own surface).
##
## Needs a real renderer: `--headless` uses the dummy rendering server and every captured image is empty, so the
## script refuses to run headless (exit 3). The game-renderer container (deploy/docker/game-renderer.Dockerfile) runs
## Godot under Xvfb with `--rendering-driver opengl3` (Compatibility renderer, Mesa llvmpipe); `make render-thumbnails`
## runs it locally with the desktop renderer of the project.
## Exit codes: 0 ok (one JSON line on stdout), 2 invalid arguments, 3 no renderer, 4 model missing or not loadable,
## 5 empty scene (no mesh), 6 render or write failure. Errors go to stderr as `render_thumbnails: <reason>`.

## Frames to wait after a camera/model change before reading the viewport (render target update + MSAA resolve).
const SETTLE_FRAMES: int = 3
const SUN_ROTATION_DEGREES: Vector3 = Vector3(-50.0, -35.0, 0.0)

var _plan: RenderPlan


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	_plan = RenderPlan.parse(OS.get_cmdline_user_args())
	if not _plan.is_valid():
		_fail(2, _plan.error)
		return
	if DisplayServer.get_name() == "headless":
		_fail(3, "needs a renderer (run without --headless; container: xvfb-run + --rendering-driver opengl3)")
		return
	if not FileAccess.file_exists(_plan.input_path):
		_fail(4, "model not found: %s" % _plan.input_path.get_file())
		return
	var model: Node3D = _load_model(_plan.input_path)
	if model == null:
		return
	var viewport: SubViewport = _build_stage()
	var pivot: Node3D = Node3D.new()
	pivot.name = "Pivot"
	viewport.add_child(pivot)
	pivot.add_child(model)
	var bounds: AABB = _visual_bounds(model)
	if bounds.size == Vector3.ZERO:
		_fail(5, "empty scene: no visible mesh in %s" % _plan.input_path.get_file())
		return
	# Centre the model on the pivot so the turntable rotates it around its own centre.
	model.position -= bounds.get_center()
	_add_camera(viewport, RenderPlan.camera_distance(bounds), bounds)
	var sheet: Image = await _capture(viewport, pivot)
	if sheet == null:
		return
	var dir_error: Error = DirAccess.make_dir_recursive_absolute(_plan.output_dir)
	if dir_error != OK:
		_fail(6, "cannot create the output directory (%s)" % error_string(dir_error))
		return
	var save_error: Error = sheet.save_png(_plan.output_path())
	if save_error != OK:
		_fail(6, "cannot write %s (%s)" % [_plan.output_file_name(), error_string(save_error)])
		return
	print(JSON.stringify({
		"output": _plan.output_path(), "kind": _plan.kind, "width": sheet.get_width(), "height": sheet.get_height(),
		"frames": _plan.frames,
	}))
	quit(0)


## Loads a .glb/.gltf from an absolute path at run time. Returns null after reporting the failure.
func _load_model(path: String) -> Node3D:
	var document: GLTFDocument = GLTFDocument.new()
	var state: GLTFState = GLTFState.new()
	var load_error: Error = document.append_from_file(path, state)
	if load_error != OK:
		_fail(4, "cannot load %s (%s)" % [path.get_file(), error_string(load_error)])
		return null
	var scene: Node = document.generate_scene(state)
	if scene == null:
		_fail(4, "cannot build a scene from %s" % path.get_file())
		return null
	if scene is Node3D:
		return scene as Node3D
	var holder: Node3D = Node3D.new()
	holder.add_child(scene)
	return holder


## SubViewport of size×size with its own 3D world, the game Environment (ambient light of the zone sky, no fog: it
## would wash out an object framed at close range) and the game sun.
func _build_stage() -> SubViewport:
	var viewport: SubViewport = SubViewport.new()
	viewport.size = Vector2i(_plan.size, _plan.size)
	viewport.own_world_3d = true
	viewport.transparent_bg = _plan.background == RenderPlan.BACKGROUND_TRANSPARENT
	viewport.msaa_3d = Viewport.MSAA_4X
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	var world_environment: WorldEnvironment = WorldEnvironment.new()
	world_environment.environment = _environment()
	viewport.add_child(world_environment)
	var sun: DirectionalLight3D = DirectionalLight3D.new()
	sun.light_color = Art3dTokens.LIGHT_SUN_COLOR
	sun.rotation_degrees = SUN_ROTATION_DEGREES
	sun.shadow_enabled = true
	viewport.add_child(sun)
	return viewport


func _environment() -> Environment:
	var path: String = String(Art3dTokens.ENVIRONMENT_PATHS.get(_plan.environment,
			Art3dTokens.ENVIRONMENT_PATHS[RenderPlan.DEFAULT_ENVIRONMENT]))
	var source: Environment = load(path) as Environment
	var environment: Environment = Environment.new()
	if source != null:
		environment = source.duplicate() as Environment
	environment.fog_enabled = false
	if _plan.background == RenderPlan.BACKGROUND_TRANSPARENT:
		# The sky still feeds the ambient light; only the background is cleared (alpha 0 with transparent_bg).
		environment.background_mode = Environment.BG_CLEAR_COLOR
	return environment


func _add_camera(viewport: SubViewport, distance: float, bounds: AABB) -> void:
	var camera: Camera3D = Camera3D.new()
	camera.fov = RenderPlan.CAMERA_FOV_DEGREES
	var radius: float = bounds.size.length() * 0.5
	camera.near = maxf(0.01, distance - radius * 2.0)
	camera.far = distance + radius * 2.0
	viewport.add_child(camera)
	camera.look_at_from_position(RenderPlan.camera_position(distance), Vector3.ZERO, Vector3.UP)
	camera.current = true


## Renders every frame and packs it on the sheet. Returns null after reporting a failure.
func _capture(viewport: SubViewport, pivot: Node3D) -> Image:
	var sheet_size: Vector2i = _plan.sheet_size()
	var sheet: Image = Image.create_empty(sheet_size.x, sheet_size.y, false, Image.FORMAT_RGBA8)
	var angles: PackedFloat32Array = _plan.frame_angles_degrees()
	for index: int in angles.size():
		pivot.rotation = Vector3(0.0, deg_to_rad(angles[index]), 0.0)
		for _frame: int in SETTLE_FRAMES:
			await process_frame
		await RenderingServer.frame_post_draw
		var frame: Image = viewport.get_texture().get_image()
		if frame == null or frame.is_empty():
			_fail(6, "the renderer returned an empty frame %d" % index)
			return null
		if frame.get_format() != Image.FORMAT_RGBA8:
			frame.convert(Image.FORMAT_RGBA8)
		if frame.get_size() != Vector2i(_plan.size, _plan.size):
			_fail(6, "frame %d is %dx%d, expected %dx%d" % [index, frame.get_width(), frame.get_height(), _plan.size,
					_plan.size])
			return null
		sheet.blit_rect(frame, Rect2i(Vector2i.ZERO, frame.get_size()), _plan.cell_position(index))
	return sheet


## Bounds of every visual instance under `node`, in the space of the pivot (the model is not centred yet).
func _visual_bounds(node: Node3D) -> AABB:
	var bounds: AABB = AABB()
	var found: bool = false
	for child: Node in node.find_children("*", "VisualInstance3D", true, false):
		var visual: VisualInstance3D = child as VisualInstance3D
		var local: AABB = visual.get_aabb()
		if local.size == Vector3.ZERO:
			continue
		var world: AABB = visual.global_transform * local
		bounds = world if not found else bounds.merge(world)
		found = true
	if node is VisualInstance3D:
		var own: AABB = node.global_transform * (node as VisualInstance3D).get_aabb()
		if own.size != Vector3.ZERO:
			bounds = own if not found else bounds.merge(own)
	return bounds


func _fail(code: int, message: String) -> void:
	printerr("render_thumbnails: %s" % message)
	quit(code)
