extends GdUnitTestSuite
## F078-AC-19 (T029): the thumbnail renderer plan — 512×512 thumbnail, 24-frame turntable in 15° steps packed 6 × 4 on
## a 3072×2048 sheet, the model read from the path given on the command line, invalid arguments rejected. Rendering
## itself needs a GPU/Xvfb and is exercised by the game-renderer container (render_thumbnails.gd).

const SCRIPT_PATH: String = "res://tools/thumbnail_renderer/render_thumbnails.gd"
const INPUT: String = "/tmp/render-job/source.glb"
const OUTPUT: String = "/tmp/render-job/out"


func _args(kind: String, extra: PackedStringArray = []) -> PackedStringArray:
	var args: PackedStringArray = ["--input", INPUT, "--output", OUTPUT, "--kind", kind]
	args.append_array(extra)
	return args


func test_thumbnail_is_one_frame_of_512() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_THUMBNAIL))
	assert_str(plan.error).is_empty()
	assert_bool(plan.is_valid()).is_true()
	assert_int(plan.size).is_equal(512)
	assert_int(plan.frames).is_equal(1)
	assert_that(plan.sheet_size()).is_equal(Vector2i(512, 512))
	assert_that(plan.cell_position(0)).is_equal(Vector2i(0, 0))
	assert_array(Array(plan.frame_angles_degrees())).is_equal([0.0])
	assert_str(plan.output_path()).is_equal(OUTPUT.path_join("thumbnail.png"))


func test_thumbnail_accepts_explicit_size_and_one_frame() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_THUMBNAIL, ["--size", "512", "--frames", "1"]))
	assert_bool(plan.is_valid()).is_true()
	assert_that(plan.sheet_size()).is_equal(Vector2i(512, 512))


func test_thumbnail_rejects_more_than_one_frame() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_THUMBNAIL, ["--frames", "24"]))
	assert_bool(plan.is_valid()).is_false()
	assert_str(plan.error).contains("--frames")


func test_turntable_is_24_frames_in_15_degree_steps() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_TURNTABLE, ["--size", "512", "--frames", "24"]))
	assert_bool(plan.is_valid()).is_true()
	assert_int(plan.frames).is_equal(24)
	var angles: PackedFloat32Array = plan.frame_angles_degrees()
	assert_int(angles.size()).is_equal(24)
	for index: int in angles.size():
		assert_float(angles[index]).is_equal_approx(15.0 * index, 0.0001)
	assert_float(angles[angles.size() - 1]).is_equal_approx(345.0, 0.0001)
	assert_str(plan.output_path()).is_equal(OUTPUT.path_join("turntable.png"))


func test_turntable_defaults_to_24_frames() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_TURNTABLE))
	assert_bool(plan.is_valid()).is_true()
	assert_int(plan.frames).is_equal(24)
	assert_int(plan.size).is_equal(512)


func test_turntable_sheet_is_6_by_4_cells_of_512() -> void:
	var plan: RenderPlan = RenderPlan.parse(_args(RenderPlan.KIND_TURNTABLE, ["--size=512", "--frames=24"]))
	assert_int(plan.columns()).is_equal(6)
	assert_int(plan.rows()).is_equal(4)
	assert_that(plan.sheet_size()).is_equal(Vector2i(3072, 2048))
	assert_that(plan.cell_position(0)).is_equal(Vector2i(0, 0))
	assert_that(plan.cell_position(1)).is_equal(Vector2i(512, 0))
	assert_that(plan.cell_position(5)).is_equal(Vector2i(2560, 0))
	assert_that(plan.cell_position(6)).is_equal(Vector2i(0, 512))
	assert_that(plan.cell_position(13)).is_equal(Vector2i(512, 1024))
	assert_that(plan.cell_position(23)).is_equal(Vector2i(2560, 1536))
	# Every cell lies inside the sheet and no two frames share a cell.
	var seen: Dictionary = {}
	for index: int in plan.frames:
		var cell: Vector2i = plan.cell_position(index)
		assert_bool(cell.x + plan.size <= plan.sheet_size().x and cell.y + plan.size <= plan.sheet_size().y).is_true()
		assert_bool(seen.has(cell)).is_false()
		seen[cell] = true


func test_input_path_is_taken_from_the_parameter() -> void:
	var path: String = "/var/tmp/render-42/media-asset.glb"
	var plan: RenderPlan = RenderPlan.parse(PackedStringArray([
		"--kind", RenderPlan.KIND_THUMBNAIL, "--output", OUTPUT, "--input", path,
	]))
	assert_bool(plan.is_valid()).is_true()
	assert_str(plan.input_path).is_equal(path)
	var gltf: RenderPlan = RenderPlan.parse(PackedStringArray([
		"--input=/srv/assets/boss.gltf", "--output=" + OUTPUT, "--kind=" + RenderPlan.KIND_TURNTABLE,
	]))
	assert_bool(gltf.is_valid()).is_true()
	assert_str(gltf.input_path).is_equal("/srv/assets/boss.gltf")


func test_missing_or_invalid_arguments_are_rejected() -> void:
	var cases: Array = [
		PackedStringArray([]),
		PackedStringArray(["--output", OUTPUT, "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", INPUT, "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", INPUT, "--output", OUTPUT]),
		PackedStringArray(["--input", "models/item.glb", "--output", OUTPUT, "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", "res://item.glb", "--output", OUTPUT, "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", "/tmp/item.png", "--output", OUTPUT, "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", INPUT, "--output", "out", "--kind", RenderPlan.KIND_THUMBNAIL]),
		PackedStringArray(["--input", INPUT, "--output", OUTPUT, "--kind", "render_preview"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--size", "abc"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--size", "0"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--frames", "0"]),
		_args(RenderPlan.KIND_TURNTABLE, ["--frames", "1"]),
		_args(RenderPlan.KIND_TURNTABLE, ["--frames", "-24"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--background", "pink"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--environment", "../env"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--color", "red"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--size"]),
		_args(RenderPlan.KIND_THUMBNAIL, ["--kind", RenderPlan.KIND_TURNTABLE]),
		_args(RenderPlan.KIND_THUMBNAIL, ["stray"]),
	]
	for args: PackedStringArray in cases:
		var plan: RenderPlan = RenderPlan.parse(args)
		assert_bool(plan.is_valid()).override_failure_message("accepted: %s" % " ".join(args)).is_false()
		assert_str(plan.error).is_not_empty()


func test_camera_fits_the_bounding_sphere() -> void:
	var cube: AABB = AABB(Vector3(-1.0, -1.0, -1.0), Vector3(2.0, 2.0, 2.0))
	# FOV 60° → half angle 30°, sin = 0.5: distance = radius / 0.5 = 2 · radius = |size|.
	assert_float(RenderPlan.camera_distance(cube, 60.0, 1.0)).is_equal_approx(sqrt(12.0), 0.0001)
	# The distance depends on the size only, not on where the model sits.
	var moved: AABB = AABB(Vector3(10.0, 4.0, -7.0), Vector3(2.0, 2.0, 2.0))
	assert_float(RenderPlan.camera_distance(moved)).is_equal_approx(RenderPlan.camera_distance(cube), 0.0001)
	# The default margin leaves a border.
	var tight: float = RenderPlan.camera_distance(cube, RenderPlan.CAMERA_FOV_DEGREES, 1.0)
	assert_float(RenderPlan.camera_distance(cube)).is_greater(tight)
	# A flat or empty model still gets a positive distance.
	assert_float(RenderPlan.camera_distance(AABB())).is_greater(0.0)


func test_camera_position_is_a_three_quarter_view_at_the_distance() -> void:
	assert_vector(RenderPlan.camera_position(10.0, 0.0, 0.0)).is_equal_approx(Vector3(0.0, 0.0, 10.0), Vector3.ONE * 0.0001)
	var view: Vector3 = RenderPlan.camera_position(8.0)
	assert_float(view.length()).is_equal_approx(8.0, 0.0001)
	assert_float(view.x).is_equal_approx(view.z, 0.0001)
	assert_float(view.y).is_greater(0.0)


func test_render_script_compiles() -> void:
	var renderer_script: GDScript = load(SCRIPT_PATH) as GDScript
	assert_object(renderer_script).is_not_null()
	assert_bool(renderer_script.can_instantiate()).is_true()
