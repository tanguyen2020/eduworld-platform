extends SceneTree
## TEST-ONLY visual capture of the S01 login form (STUDENT_GAME_UX_CORE S01, DESIGN_SYSTEM §5.5, §9): every state
## the form can show — empty, filled on a personal device, invalid credentials (+ wait hint), session expired banner,
## keyboard focus on the sign-in button, signing in, forgot-password hint — in vi-VN / en-US / xx-PS, at the window size given by `--resolution`. No
## network: the states are put into `AuthService.form` directly. Needs a renderer (not --headless):
##   godot --path src/game --rendering-driver opengl3 --resolution 1280x720 -s res://tests/visual/login_screens.gd \
##     -- [--out=<absolute dir>] [--locales=vi-VN,en-US] [--states=empty,error] [--size=720x1280]
## `--size` renders into a SubViewport of that size with the project stretch (canvas_items + expand) emulated, for
## window sizes the monitor cannot open (portrait phone, 4K).
## Images: <out>/login_<state>_<locale>_<W>x<H>.png (default out: res://tests/scene/screenshots/<platform>/, ignored
## by git).

const LOGIN: String = "res://scenes/auth/Login.tscn"
const OUTPUT_DIR: String = "res://tests/scene/screenshots"
const LOCALES: PackedStringArray = ["vi-VN", "en-US", "xx-PS"]
const STATES: PackedStringArray = ["empty", "filled", "focus", "error", "banner", "busy", "forgot"]
const SETTLE_FRAMES: int = 6


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("login_screens: needs a renderer (run without --headless)")
		quit(2)
		return
	var args: Dictionary = _user_args()
	var output_dir: String = String(args.get("out", ProjectSettings.globalize_path(
			OUTPUT_DIR.path_join(OS.get_name().to_lower()))))
	DirAccess.make_dir_recursive_absolute(output_dir)
	var locales: PackedStringArray = String(args["locales"]).split(",") if args.has("locales") else LOCALES
	var states: PackedStringArray = String(args["states"]).split(",") if args.has("states") else STATES
	var auth: Node = root.get_node("AuthService")
	var settings: Node = root.get_node("Settings")
	auth.set("autostart", false)
	root.get_node("AppState").call("start_ui", root)
	var form: LoginFormModel = auth.get("form") as LoginFormModel
	await _settle() # let the autoloads finish their deferred start (device profile) before the states are set
	var size: Vector2i = DisplayServer.window_get_size()
	var viewport: Viewport = root
	if args.has("size"):
		var parts: PackedStringArray = String(args["size"]).split("x")
		size = Vector2i(int(parts[0]), int(parts[1]))
		viewport = _stretched_viewport(size)
	var count: int = 0
	for locale: String in locales:
		settings.call("set_locale", locale, false)
		for state: String in states:
			_prepare(form, state)
			var scene: Control = (load(LOGIN) as PackedScene).instantiate() as Control
			if viewport != root:
				scene.theme = root.theme
			viewport.add_child(scene)
			await _settle()
			_after_ready(scene, form, state)
			await _settle()
			var image: Image = viewport.get_texture().get_image()
			image.save_png(output_dir.path_join("login_%s_%s_%dx%d.png" % [
					state, locale.replace("-", "_"), size.x, size.y]))
			count += 1
			scene.queue_free()
			await process_frame
	print("login_screens: %d images in %s" % [count, output_dir])
	quit(0)


## A SubViewport of `target` pixels showing the logical size the project stretch would give (expand aspect).
func _stretched_viewport(target: Vector2i) -> SubViewport:
	var base: Vector2 = Vector2(float(ProjectSettings.get_setting("display/window/size/viewport_width")),
			float(ProjectSettings.get_setting("display/window/size/viewport_height")))
	var scale: float = minf(float(target.x) / base.x, float(target.y) / base.y)
	var viewport: SubViewport = SubViewport.new()
	viewport.size = target
	viewport.size_2d_override = Vector2i(roundi(float(target.x) / scale), roundi(float(target.y) / scale))
	viewport.size_2d_override_stretch = true
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	return viewport


## Form state before the scene is built (what AuthService holds when the form opens).
func _prepare(form: LoginFormModel, state: String) -> void:
	var personal: bool = state != "empty"
	form.load_memory(true, "hs.thdemo.000123" if personal else "", not personal)
	form.failed_attempts = 0
	form.end_submit("")
	match state:
		"banner":
			form.show_banner("REFRESH_TOKEN_INVALID")
		"error":
			form.failed_attempts = LoginFormModel.WAIT_HINT_AFTER - 1
			form.end_submit("INVALID_CREDENTIALS")


## Interactions after the scene is shown (typing, pressing).
func _after_ready(scene: Control, form: LoginFormModel, state: String) -> void:
	var password: LineEdit = scene.find_child("Password", true, false) as LineEdit
	match state:
		"filled", "focus":
			password.text = "matkhau1"
			password.text_changed.emit(password.text)
			if state == "focus":
				(scene.find_child("SignIn", true, false) as Button).grab_focus()
		"busy":
			password.text = "matkhau1"
			form.begin_submit("hs.thdemo.000123")
		"forgot":
			(scene.find_child("Forgot", true, false) as Button).pressed.emit()


func _settle() -> void:
	for i: int in SETTLE_FRAMES:
		await process_frame
	await RenderingServer.frame_post_draw


static func _user_args() -> Dictionary:
	var out: Dictionary = {}
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--") and arg.contains("="):
			out[arg.substr(2, arg.find("=") - 2)] = arg.substr(arg.find("=") + 1)
	return out
