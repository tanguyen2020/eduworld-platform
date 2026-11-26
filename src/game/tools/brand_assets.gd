extends SceneTree
## EduWorld brand images (F096, ADR-0102, DESIGN_SYSTEM §5.1, GODOT_ARCHITECTURE §5): renders the boot splash and the
## app icons from the S01 components — the Knowledge Spark (`SparkMark`), the "EduWorld" wordmark (`LogoLabel` of the
## generated Theme) and the guide Mây (`CloudyMascot`) on the sky colours of `component.backdrop.*` — and points the
## project and the export presets at them, so no Godot logo or icon is shown anywhere. First-party, MIT
## (`res://assets/LICENSES.md`). Rendering needs a renderer (not --headless):
##   godot --path src/game -s res://tools/brand_assets.gd
## `--check` (headless) fails when a file is missing or has the wrong size, a setting points elsewhere, the boot splash
## colour is not the token, or the licence row is missing:
##   godot --headless --path src/game -s res://tools/brand_assets.gd -- --check

const OUT_DIR: String = "res://resources/branding"
const PROJECT_FILE: String = "res://project.godot"
const PRESETS_FILE: String = "res://export_presets.cfg"
const LICENSES_FILE: String = "res://assets/LICENSES.md"
const LICENSE_GLOB: String = "`resources/branding/*.png`"
const SPLASH: String = OUT_DIR + "/boot_splash.png"
const ICON: String = OUT_DIR + "/icon_1024.png"
const ICON_IOS: String = OUT_DIR + "/icon_ios_1024.png"
const ICON_ANDROID: String = OUT_DIR + "/icon_android_192.png"
const ICON_FG: String = OUT_DIR + "/icon_adaptive_fg_432.png"
const ICON_BG: String = OUT_DIR + "/icon_adaptive_bg_432.png"
## File → pixel size.
const SIZES: Dictionary = {
	SPLASH: Vector2i(1280, 720), ICON: Vector2i(1024, 1024), ICON_IOS: Vector2i(1024, 1024),
	ICON_ANDROID: Vector2i(192, 192), ICON_FG: Vector2i(432, 432), ICON_BG: Vector2i(432, 432),
}
## Boot splash: the lockup drawn at this scale of its Theme size, centred in the 16:9 frame.
const SPLASH_SCALE: float = 2.0
## Rounded corner of the project icon (share of the side); iOS draws its own mask, so its icon is a full square.
const ICON_CORNER: float = 0.22
## Android adaptive icons keep the artwork inside the central 66 % safe zone.
const ADAPTIVE_SAFE: float = 0.66
const SETTLE_FRAMES: int = 6


## Icon artwork: the sky gradient (optionally rounded), a big Knowledge Spark and Mây peeking at the bottom right.
class IconArt:
	extends Control

	var rounded: bool = true
	var background: bool = true

	func _draw() -> void:
		var side: float = minf(size.x, size.y)
		if background:
			_draw_sky(side)

	func _draw_sky(side: float) -> void:
		var radius: float = side * ICON_CORNER if rounded else 0.0
		var points: PackedVector2Array = []
		var colors: PackedColorArray = []
		var steps: int = 12
		for corner: int in 4:
			var center: Vector2 = [Vector2(side - radius, radius), Vector2(side - radius, side - radius),
					Vector2(radius, side - radius), Vector2(radius, radius)][corner]
			for i: int in steps + 1:
				var angle: float = -PI / 2.0 + PI / 2.0 * float(corner) + PI / 2.0 * float(i) / float(steps)
				var p: Vector2 = center + Vector2(cos(angle), sin(angle)) * radius
				points.append(p)
				colors.append(UiTokens.COMPONENT_BACKDROP_SKY_TOP.lerp(UiTokens.COMPONENT_BACKDROP_SKY_BOTTOM, p.y / side))
		draw_polygon(points, colors)


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	if "--check" in OS.get_cmdline_user_args():
		var errors: PackedStringArray = check()
		for e: String in errors:
			printerr("brand_assets: " + e)
		print("brand_assets: %s (%d file(s))" % ["OK" if errors.is_empty() else "%d error(s)" % errors.size(), SIZES.size()])
		quit(0 if errors.is_empty() else 1)
		return
	if DisplayServer.get_name() == "headless":
		printerr("brand_assets: rendering needs a renderer (run without --headless, or pass --check)")
		quit(2)
		return
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(OUT_DIR))
	await _save(await _render_splash(), SPLASH)
	var icon: Image = await _render_icon(Vector2i(1024, 1024), true, true, true, 1.0)
	await _save(icon, ICON)
	var android: Image = icon.duplicate() as Image
	android.resize(192, 192, Image.INTERPOLATE_LANCZOS)
	await _save(android, ICON_ANDROID)
	await _save(await _render_icon(Vector2i(1024, 1024), false, true, true, 1.0), ICON_IOS)
	await _save(await _render_icon(Vector2i(432, 432), false, false, true, ADAPTIVE_SAFE), ICON_FG)
	await _save(await _render_icon(Vector2i(432, 432), false, true, false, 1.0), ICON_BG)
	_write_settings()
	print("brand_assets: %d image(s) written to %s; project.godot and export_presets.cfg updated" % [SIZES.size(), OUT_DIR])
	quit(0)


# ---------- rendering ----------

func _viewport(size: Vector2i) -> SubViewport:
	var vp: SubViewport = SubViewport.new()
	vp.size = size
	vp.transparent_bg = true
	vp.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(vp)
	return vp


func _grab(vp: SubViewport) -> Image:
	for i: int in SETTLE_FRAMES:
		await process_frame
	await RenderingServer.frame_post_draw
	var image: Image = vp.get_texture().get_image()
	image.convert(Image.FORMAT_RGBA8)
	vp.queue_free()
	return image


func _render_splash() -> Image:
	var size: Vector2i = SIZES[SPLASH] as Vector2i
	var vp: SubViewport = _viewport(size)
	# Opaque, in the boot splash colour: no premultiplied fringes, seamless with the letterbox of "keep".
	var sky: ColorRect = ColorRect.new()
	sky.color = UiTokens.COMPONENT_BACKDROP_SKY_TOP
	sky.size = Vector2(size)
	vp.add_child(sky)
	var lockup: BrandLockup = (load("res://scenes/ui/components/BrandLockup.tscn") as PackedScene).instantiate() as BrandLockup
	vp.add_child(lockup)
	await process_frame
	var natural: Vector2 = lockup.get_combined_minimum_size()
	lockup.size = natural
	lockup.scale = Vector2.ONE * SPLASH_SCALE
	lockup.position = (Vector2(size) - natural * SPLASH_SCALE) / 2.0
	return await _grab(vp)


## `artwork_share` < 1 keeps the spark and Mây inside the central share of the canvas (adaptive safe zone).
func _render_icon(size: Vector2i, rounded: bool, background: bool, artwork: bool, artwork_share: float) -> Image:
	var vp: SubViewport = _viewport(size)
	var art: IconArt = IconArt.new()
	art.rounded = rounded
	art.background = background
	art.size = Vector2(size)
	vp.add_child(art)
	if artwork:
		var side: float = float(size.x) * artwork_share
		var origin: Vector2 = (Vector2(size) - Vector2(side, side)) / 2.0
		var spark: SparkMark = SparkMark.new()
		art.add_child(spark)
		spark.custom_minimum_size = Vector2.ZERO
		spark.position = origin + Vector2(side * 0.1, side * 0.06)
		spark.size = Vector2(side * 0.66, side * 0.66)
		var mascot: CloudyMascot = CloudyMascot.new()
		mascot.bubble_tail = false
		art.add_child(mascot)
		mascot.custom_minimum_size = Vector2.ZERO
		mascot.position = origin + Vector2(side * 0.46, side * 0.5)
		mascot.size = Vector2(side * 0.5, side * 0.5)
	return await _grab(vp)


func _save(image: Image, path: String) -> void:
	var err: Error = image.save_png(ProjectSettings.globalize_path(path))
	if err != OK:
		printerr("brand_assets: cannot write %s (%s)" % [path, error_string(err)])
	await process_frame


# ---------- settings (line edits keep the comments of project.godot / export_presets.cfg) ----------

## Expected `project.godot` values (also checked by --check and test_branding.gd).
static func project_values() -> Dictionary:
	return {
		"application/boot_splash/image": SPLASH,
		"application/boot_splash/bg_color": UiTokens.COMPONENT_BACKDROP_SKY_TOP,
		"application/boot_splash/use_filter": true,
		"application/boot_splash/stretch_mode": 1,
		"application/boot_splash/show_image": true,
		"application/config/icon": ICON,
	}


## Expected export preset options: section → key → value.
static func preset_values() -> Dictionary:
	return {
		"preset.0.options": {
			"launcher_icons/main_192x192": ICON_ANDROID,
			"launcher_icons/adaptive_foreground_432x432": ICON_FG,
			"launcher_icons/adaptive_background_432x432": ICON_BG,
		},
		"preset.1.options": {"icons/icon_1024x1024": ICON_IOS},
		"preset.2.options": {"html/export_icon": true},
	}


func _write_settings() -> void:
	var project: String = FileAccess.get_file_as_string(PROJECT_FILE)
	var values: Dictionary = project_values()
	for key: String in values:
		project = set_line(project, "application", key.trim_prefix("application/"), var_to_str(values[key]))
	_write(PROJECT_FILE, project)
	var presets: String = FileAccess.get_file_as_string(PRESETS_FILE)
	var options: Dictionary = preset_values()
	for section: String in options:
		var keys: Dictionary = options[section] as Dictionary
		for key: String in keys:
			presets = set_line(presets, section, key, var_to_str(keys[key]))
	_write(PRESETS_FILE, presets)


func _write(path: String, text: String) -> void:
	var f: FileAccess = FileAccess.open(path, FileAccess.WRITE)
	f.store_string(text)
	f.close()


## Sets `key=value` inside `[section]` (replaces the line, or appends it after the section's last key).
static func set_line(text: String, section: String, key: String, value: String) -> String:
	var lines: PackedStringArray = text.split("\n")
	var start: int = lines.find("[%s]" % section)
	if start < 0:
		return text
	var last_key: int = start
	for i: int in range(start + 1, lines.size()):
		var line: String = lines[i]
		if line.begins_with("["):
			break
		if line.begins_with(key + "="):
			lines[i] = "%s=%s" % [key, value]
			return "\n".join(lines)
		if not line.strip_edges().is_empty() and not line.begins_with(";"):
			last_key = i
	lines.insert(last_key + 1, "%s=%s" % [key, value])
	return "\n".join(lines)


# ---------- check ----------

## Colours written by var_to_str keep 4 decimals: equal within one 8-bit step.
static func _same_color(a: Color, b: Color) -> bool:
	var step: float = 1.0 / 255.0
	return absf(a.r - b.r) < step and absf(a.g - b.g) < step and absf(a.b - b.b) < step and absf(a.a - b.a) < step


static func check() -> PackedStringArray:
	var errors: PackedStringArray = []
	for path: String in SIZES:
		if not FileAccess.file_exists(path):
			errors.append("%s missing (run tools/brand_assets.gd with a renderer)" % path)
			continue
		var image: Image = Image.load_from_file(ProjectSettings.globalize_path(path))
		if image == null or image.get_size() != (SIZES[path] as Vector2i):
			errors.append("%s must be %s" % [path, SIZES[path]])
	var values: Dictionary = project_values()
	for key: String in values:
		var actual: Variant = ProjectSettings.get_setting(key)
		var expected: Variant = values[key]
		var same: bool = _same_color(actual as Color, expected as Color) if expected is Color and actual is Color \
				else actual == expected
		if not same:
			errors.append("project setting %s is %s, expected %s" % [key, actual, expected])
	var presets: ConfigFile = ConfigFile.new()
	if presets.load(PRESETS_FILE) != OK:
		errors.append("cannot read %s" % PRESETS_FILE)
	else:
		var options: Dictionary = preset_values()
		for section: String in options:
			var keys: Dictionary = options[section] as Dictionary
			for key: String in keys:
				if presets.get_value(section, key, null) != keys[key]:
					errors.append("%s [%s] %s is %s, expected %s" % [PRESETS_FILE, section, key,
							presets.get_value(section, key, null), keys[key]])
	if not FileAccess.get_file_as_string(LICENSES_FILE).contains(LICENSE_GLOB):
		errors.append("%s has no row for %s (ADR-0102)" % [LICENSES_FILE, LICENSE_GLOB])
	return errors
