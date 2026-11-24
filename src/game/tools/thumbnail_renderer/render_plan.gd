class_name RenderPlan
extends RefCounted
## Pure plan of one thumbnail/turntable render (F078 US5, FR-CMS-017, DESIGN_SYSTEM §7): command-line parsing and
## validation, frame angles, sprite sheet geometry and camera framing. No node, no rendering, so GdUnit4 tests it
## headless (res://tests/tools/test_render_thumbnails.gd); render_thumbnails.gd executes the plan.
##
## Geometry matches the work-queue contract of the worker (src/backend/modules/game/internal/infrastructure/render):
## thumbnail = 1 frame of size×size; turntable = `frames` frames of size×size rotating 360°/frames around Y, packed
## left to right, top to bottom on a sheet of TURNTABLE_COLUMNS columns (24 × 512 → 6 × 4 → 3072×2048).

const KIND_THUMBNAIL: String = "render_thumbnail"
const KIND_TURNTABLE: String = "render_turntable"
const OUTPUT_FILES: Dictionary = {KIND_THUMBNAIL: "thumbnail.png", KIND_TURNTABLE: "turntable.png"}
## Default size and frame count of the contract (render.Size, render.TurntableFrames).
const DEFAULT_SIZE: int = 512
const DEFAULT_TURNTABLE_FRAMES: int = 24
const TURNTABLE_COLUMNS: int = 6
const MIN_SIZE: int = 16
const MAX_SIZE: int = 4096
const MAX_TURNTABLE_FRAMES: int = 120
## Background of the render: `transparent` keeps the alpha channel (the CMS draws the PNG on its own surface, no
## design colour is baked in); `environment` shows the sky of the game Environment.
const BACKGROUND_TRANSPARENT: String = "transparent"
const BACKGROUND_ENVIRONMENT: String = "environment"
const BACKGROUNDS: PackedStringArray = [BACKGROUND_TRANSPARENT, BACKGROUND_ENVIRONMENT]
## Zone Environment of the game used for ambient light (Art3dTokens.ENVIRONMENT_PATHS key, design/game-tokens).
const DEFAULT_ENVIRONMENT: String = "hub"
const MODEL_EXTENSIONS: PackedStringArray = ["glb", "gltf"]
## 3/4 view: camera azimuth 45° from the model front (+Z, glTF convention), elevation 25°, narrow FOV (little
## perspective distortion) and a margin around the bounding sphere so every turntable frame fits.
const CAMERA_AZIMUTH_DEGREES: float = 45.0
const CAMERA_ELEVATION_DEGREES: float = 25.0
const CAMERA_FOV_DEGREES: float = 30.0
const CAMERA_MARGIN: float = 1.1
const OPTIONS: PackedStringArray = ["input", "output", "kind", "size", "frames", "background", "environment"]

var input_path: String = ""
var output_dir: String = ""
var kind: String = ""
var size: int = DEFAULT_SIZE
var frames: int = 1
var background: String = BACKGROUND_TRANSPARENT
var environment: String = DEFAULT_ENVIRONMENT
## Empty when the plan is valid; otherwise the reason, printed on stderr by the script.
var error: String = ""


## Parses `-- --input <abs .glb> --output <abs dir> --kind render_thumbnail|render_turntable [--size 512]
## [--frames 1|24] [--background transparent|environment] [--environment hub]` (also `--key=value`).
static func parse(args: PackedStringArray) -> RenderPlan:
	var plan: RenderPlan = RenderPlan.new()
	return plan.read_args(args)


## Fills this plan from the command-line arguments; sets `error` on the first problem. Returns self.
func read_args(args: PackedStringArray) -> RenderPlan:
	var values: Dictionary = {}
	var i: int = 0
	while i < args.size():
		var arg: String = args[i]
		if not arg.begins_with("--"):
			return _fail("unexpected argument '%s'" % arg)
		var key: String = arg.substr(2)
		var value: String = ""
		if key.contains("="):
			value = key.substr(key.find("=") + 1)
			key = key.substr(0, key.find("="))
		elif i + 1 < args.size() and not args[i + 1].begins_with("--"):
			i += 1
			value = args[i]
		else:
			return _fail("option --%s needs a value" % key)
		if not key in OPTIONS:
			return _fail("unknown option --%s" % key)
		if values.has(key):
			return _fail("option --%s given twice" % key)
		values[key] = value
		i += 1
	return _apply(values)


func is_valid() -> bool:
	return error.is_empty()


func is_turntable() -> bool:
	return kind == KIND_TURNTABLE


## Number of sheet columns: TURNTABLE_COLUMNS, or fewer when there are fewer frames (1 for a thumbnail).
func columns() -> int:
	return mini(frames, TURNTABLE_COLUMNS)


func rows() -> int:
	return ceili(float(frames) / float(columns()))


## Pixel size of the written PNG (thumbnail size×size; turntable columns·size × rows·size).
func sheet_size() -> Vector2i:
	return Vector2i(columns() * size, rows() * size)


## Top-left pixel of frame `index` on the sheet (row-major, left to right then top to bottom).
func cell_position(index: int) -> Vector2i:
	var column: int = index % columns()
	var row: int = floori(float(index) / float(columns()))
	return Vector2i(column * size, row * size)


## Y rotation of the model per frame, in degrees: 0, 360/frames, … (15° steps for 24 frames).
func frame_angles_degrees() -> PackedFloat32Array:
	var out: PackedFloat32Array = []
	for index: int in frames:
		out.append(360.0 * float(index) / float(frames))
	return out


func output_file_name() -> String:
	return String(OUTPUT_FILES.get(kind, ""))


func output_path() -> String:
	return output_dir.path_join(output_file_name())


## Camera distance from the centre of `aabb` so that its bounding sphere fits a square view of `fov_degrees` with
## `margin` (> 1 leaves a border); the sphere is used so every turntable angle fits the same framing.
static func camera_distance(aabb: AABB, fov_degrees: float = CAMERA_FOV_DEGREES, margin: float = CAMERA_MARGIN) -> float:
	var radius: float = maxf(aabb.size.length() * 0.5, 0.001)
	return radius * margin / sin(deg_to_rad(fov_degrees * 0.5))


## Camera position around the origin at `distance`, `azimuth_degrees` from +Z towards +X, `elevation_degrees` up.
static func camera_position(distance: float, azimuth_degrees: float = CAMERA_AZIMUTH_DEGREES,
		elevation_degrees: float = CAMERA_ELEVATION_DEGREES) -> Vector3:
	var azimuth: float = deg_to_rad(azimuth_degrees)
	var elevation: float = deg_to_rad(elevation_degrees)
	return Vector3(sin(azimuth) * cos(elevation), sin(elevation), cos(azimuth) * cos(elevation)) * distance


func _apply(values: Dictionary) -> RenderPlan:
	input_path = String(values.get("input", ""))
	output_dir = String(values.get("output", ""))
	kind = String(values.get("kind", ""))
	background = String(values.get("background", BACKGROUND_TRANSPARENT))
	environment = String(values.get("environment", DEFAULT_ENVIRONMENT))
	if input_path.is_empty() or not input_path.is_absolute_path() or input_path.begins_with("res://") \
			or input_path.begins_with("user://"):
		return _fail("--input must be the absolute file path of a .glb/.gltf model")
	if not input_path.get_extension().to_lower() in MODEL_EXTENSIONS:
		return _fail("--input must be a .glb or .gltf file")
	if output_dir.is_empty() or not output_dir.is_absolute_path() or output_dir.begins_with("res://") \
			or output_dir.begins_with("user://"):
		return _fail("--output must be an absolute directory path")
	if not OUTPUT_FILES.has(kind):
		return _fail("--kind must be %s or %s" % [KIND_THUMBNAIL, KIND_TURNTABLE])
	if not background in BACKGROUNDS:
		return _fail("--background must be one of %s" % ", ".join(BACKGROUNDS))
	if environment.is_empty() or not environment.is_valid_ascii_identifier():
		return _fail("--environment must be a zone name (Art3dTokens.ENVIRONMENT_PATHS)")
	size = _int_option(values, "size", DEFAULT_SIZE)
	if size < MIN_SIZE or size > MAX_SIZE:
		return _fail("--size must be an integer in [%d, %d]" % [MIN_SIZE, MAX_SIZE])
	frames = _int_option(values, "frames", DEFAULT_TURNTABLE_FRAMES if is_turntable() else 1)
	if not is_turntable() and frames != 1:
		return _fail("--frames must be 1 for %s" % KIND_THUMBNAIL)
	if is_turntable() and (frames < 2 or frames > MAX_TURNTABLE_FRAMES):
		return _fail("--frames must be an integer in [2, %d] for %s" % [MAX_TURNTABLE_FRAMES, KIND_TURNTABLE])
	return self


## Integer option or `fallback` when absent; -1 (rejected by the range checks) when not an integer.
static func _int_option(values: Dictionary, key: String, fallback: int) -> int:
	if not values.has(key):
		return fallback
	var text: String = String(values[key])
	return text.to_int() if text.is_valid_int() else -1


func _fail(message: String) -> RenderPlan:
	error = message
	return self
