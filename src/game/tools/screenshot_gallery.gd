extends SceneTree
## Visual regression of UiGallery (F060-AC-10, TC-UI-001, DESIGN_SYSTEM §9): 4 densities × vi / en / xx-PS plus
## high contrast, captured with a renderer (not --headless; CI uses xvfb + gl_compatibility) and compared with
## res://tests/scene/baselines/<platform>/. Candidates go to res://tests/scene/screenshots/<platform>/ (ignored).
##   godot --path src/game -s res://tools/screenshot_gallery.gd [-- --update-baseline]
## A platform without baselines writes candidates and reports "baseline missing" (exit 0) so a reviewer can
## approve them; any differing image exits 1.

const GALLERY: String = "res://scenes/ui/debug/UiGallery.tscn"
const BASELINE_DIR: String = "res://tests/scene/baselines"
const OUTPUT_DIR: String = "res://tests/scene/screenshots"
const LOCALES: PackedStringArray = ["vi-VN", "en-US", "xx-PS"]
## A pixel differs when a channel moves more than this (0–1); an image fails above MAX_DIFF_RATIO differing pixels.
const CHANNEL_TOLERANCE: float = 0.08
const MAX_DIFF_RATIO: float = 0.005
const SETTLE_FRAMES: int = 4


## Every captured state: [band, locale, high_contrast].
static func combinations() -> Array:
	var out: Array = []
	for band: String in UiTokens.DENSITY_BANDS:
		for locale: String in LOCALES:
			out.append([band, locale, false])
	out.append([UiTokens.DENSITY_BANDS[0], LOCALES[0], true])
	return out


static func file_name(combo: Array) -> String:
	return "%s_%s%s.png" % [combo[0], String(combo[1]).replace("-", "_"), "_hc" if bool(combo[2]) else ""]


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("screenshot_gallery: needs a renderer (run without --headless; CI: xvfb-run)")
		quit(2)
		return
	var update: bool = "--update-baseline" in OS.get_cmdline_user_args()
	var platform: String = OS.get_name().to_lower()
	var baseline_dir: String = BASELINE_DIR.path_join(platform)
	var output_dir: String = OUTPUT_DIR.path_join(platform)
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(output_dir))
	# Captures are review artefacts, not game resources: keep the editor from importing them.
	var marker: FileAccess = FileAccess.open(OUTPUT_DIR.path_join(".gdignore"), FileAccess.WRITE)
	if marker != null:
		marker.close()
	if update:
		DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(baseline_dir))
	var app_state: Node = root.get_node("AppState")
	var settings: Node = root.get_node("Settings")
	var gallery: Node = (load(GALLERY) as PackedScene).instantiate()
	root.add_child(gallery)
	var failures: PackedStringArray = []
	var missing: int = 0
	for combo: Array in combinations():
		app_state.call("set_grade_band", String(combo[0]))
		settings.call("set_locale", String(combo[1]), false)
		(app_state.get("theme_service") as ThemeService).set_high_contrast(bool(combo[2]))
		for i: int in SETTLE_FRAMES:
			await process_frame
		await RenderingServer.frame_post_draw
		var image: Image = root.get_texture().get_image()
		var image_name: String = file_name(combo)
		image.save_png(ProjectSettings.globalize_path(output_dir.path_join(image_name)))
		var baseline_path: String = baseline_dir.path_join(image_name)
		if update:
			image.save_png(ProjectSettings.globalize_path(baseline_path))
		elif not FileAccess.file_exists(baseline_path):
			missing += 1
		else:
			var ratio: float = diff_ratio(Image.load_from_file(ProjectSettings.globalize_path(baseline_path)), image)
			if ratio > MAX_DIFF_RATIO:
				failures.append("%s differs (%.2f%% pixels)" % [image_name, ratio * 100.0])
	gallery.queue_free()
	for f: String in failures:
		printerr("screenshot_gallery: " + f)
	if missing > 0:
		print("screenshot_gallery: baseline missing for %d image(s) on %s — review %s and run with --update-baseline" % [
			missing, platform, output_dir])
	print("screenshot_gallery: %d images, %d differing" % [combinations().size(), failures.size()])
	quit(1 if not failures.is_empty() else 0)


## Share of pixels whose channels differ by more than CHANNEL_TOLERANCE (1.0 when sizes differ).
static func diff_ratio(expected: Image, actual: Image) -> float:
	if expected == null or actual == null or expected.get_size() != actual.get_size():
		return 1.0
	var differing: int = 0
	var size: Vector2i = expected.get_size()
	for y: int in size.y:
		for x: int in size.x:
			var a: Color = expected.get_pixel(x, y)
			var b: Color = actual.get_pixel(x, y)
			if absf(a.r - b.r) > CHANNEL_TOLERANCE or absf(a.g - b.g) > CHANNEL_TOLERANCE or absf(a.b - b.b) > CHANNEL_TOLERANCE:
				differing += 1
	return float(differing) / float(size.x * size.y)
