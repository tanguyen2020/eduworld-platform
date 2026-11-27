extends GdUnitTestSuite
## F125-AC-01, F125-AC-02, F125-AC-03, F125-AC-05 (STUDENT_GAME_UX §2, DESIGN_SYSTEM §5.2): OrientationGuard switches
## compact mode on every window resize (no scene reload) and writes the content scale of the mode — once per change,
## so turning the phone between the two landscape sides does not flicker; the rotate prompt's factor wins while it
## shows and the mode's factor comes back when it hides. The compact Theme of each band floors the touch target and
## the smallest font and never lowers a band size; UiBuild targets follow the mode; a real Window with that factor
## lays the game out on a 540 px tall canvas.

const GuardScript := preload("res://scripts/core/orientation_guard.gd")
const COMPACT: float = 720.0 / 540.0

var guard: Node = null
var content_scale: float = 1.0
var scale_calls: Array[float] = []
var compact_calls: Array[bool] = []
var density: float = 1.0
var keyboard: float = 0.0
var paused: bool = false


func before_test() -> void:
	content_scale = 1.0
	scale_calls.clear()
	compact_calls.clear()
	density = 1.0
	keyboard = 0.0
	paused = false
	guard = GuardScript.new()
	guard.set("follow_window", false)
	guard.set("set_paused", func(value: bool) -> void: paused = value)
	guard.set("is_paused", func() -> bool: return paused)
	guard.set("lock_landscape", func() -> void: pass)
	guard.set("set_content_scale", func(value: float) -> void:
		content_scale = value
		scale_calls.append(value))
	guard.set("get_content_scale", func() -> float: return content_scale)
	guard.set("get_density", func() -> float: return density)
	guard.set("get_keyboard_height", func() -> float: return keyboard)
	guard.set("set_compact", func(value: bool) -> void: compact_calls.append(value))
	add_child(guard)


func after_test() -> void:
	guard.free()
	get_tree().paused = false
	AppState.set_compact(false)


func test_a_phone_window_is_compact_with_a_540_canvas_and_a_pc_window_is_not() -> void:
	guard.call("apply", Vector2(1280, 720))
	assert_bool(bool(guard.get("compact"))).is_false()
	assert_array(scale_calls).is_empty()
	assert_float(content_scale).is_equal(1.0)
	# Browser window dragged down to a phone-sized window, then the phone itself.
	guard.call("apply", Vector2(844, 390))
	assert_bool(bool(guard.get("compact"))).is_true()
	assert_array(compact_calls).is_equal([true])
	assert_float(content_scale).is_equal_approx(COMPACT, 0.0001)
	# Another phone size, the other landscape side: same factor, nothing written again (no flicker).
	guard.call("apply", Vector2(740, 360))
	guard.call("apply", Vector2(844, 390))
	assert_array(scale_calls).is_equal([COMPACT])
	assert_array(compact_calls).is_equal([true])
	# Back to a PC/tablet window: factor 1, regular mode.
	guard.call("apply", Vector2(1024, 600))
	assert_bool(bool(guard.get("compact"))).is_false()
	assert_array(compact_calls).is_equal([true, false])
	assert_float(content_scale).is_equal(1.0)


func test_density_independent_px_decide() -> void:
	density = 3.0
	guard.call("apply", Vector2(2532, 1170))
	assert_bool(bool(guard.get("compact"))).is_true()
	assert_float(content_scale).is_equal_approx(COMPACT, 0.0001)
	density = 2.0
	guard.call("apply", Vector2(2048, 1536))
	assert_bool(bool(guard.get("compact"))).is_false()
	assert_float(content_scale).is_equal(1.0)


func test_the_on_screen_keyboard_keeps_the_mode_and_the_factor() -> void:
	guard.call("apply", Vector2(1024, 600))
	keyboard = 300.0
	guard.call("apply", Vector2(1024, 300))
	assert_bool(bool(guard.get("compact"))).is_false()
	assert_float(content_scale).is_equal(1.0)
	keyboard = 0.0
	guard.call("apply", Vector2(844, 390))
	keyboard = 190.0
	guard.call("apply", Vector2(844, 200))
	assert_array(scale_calls).is_equal([COMPACT])


## F125-AC-02: the rotate prompt's factor wins while it shows; the compact factor (not 1, not an older value) comes back.
func test_the_rotate_prompt_factor_wins_and_the_mode_factor_returns() -> void:
	guard.call("apply", Vector2(844, 390))
	assert_float(content_scale).is_equal_approx(COMPACT, 0.0001)
	guard.call("apply", Vector2(390, 844))
	assert_bool(bool(guard.get("prompt_visible"))).is_true()
	assert_float(content_scale).is_equal_approx(OrientationPolicy.prompt_content_scale(Vector2(390, 844),
			OrientationPolicy.base_size()), 0.0001)
	guard.call("apply", Vector2(844, 390))
	assert_bool(bool(guard.get("prompt_visible"))).is_false()
	assert_float(content_scale).is_equal_approx(COMPACT, 0.0001)
	# A PC window turned upright and back: 1 again.
	guard.call("apply", Vector2(1280, 720))
	guard.call("apply", Vector2(720, 1280))
	guard.call("apply", Vector2(1280, 720))
	assert_float(content_scale).is_equal(1.0)


## F125-AC-01: a real Window (stretch canvas_items + expand on 1280×720) with the compact factor has a 540 tall canvas.
func test_a_real_window_with_the_compact_factor_is_540_logical_px_tall() -> void:
	var win: Window = auto_free(Window.new()) as Window
	win.content_scale_size = Vector2i(1280, 720)
	win.content_scale_mode = Window.CONTENT_SCALE_MODE_CANVAS_ITEMS
	win.content_scale_aspect = Window.CONTENT_SCALE_ASPECT_EXPAND
	win.size = Vector2i(844, 390)
	add_child(win)
	win.content_scale_factor = CompactPolicy.content_scale(Vector2(844, 390), OrientationPolicy.base_size(), true)
	await await_idle_frame()
	var view: Vector2 = win.get_visible_rect().size
	assert_float(view.y).is_equal_approx(540.0, 1.0)
	assert_float(view.x).is_equal_approx(1168.6, 1.5)
	win.content_scale_factor = 1.0
	await await_idle_frame()
	assert_float(win.get_visible_rect().size.y).is_equal_approx(720.0, 1.0)


## F125-AC-03, AC-05: the compact Theme of every band floors the touch target and the smallest font, never lowers a size.
func test_compact_themes_floor_and_never_lower() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var regular: Theme = load(String(UiTokens.THEME_PATHS[band])) as Theme
		var compact: Theme = load(String(UiTokens.COMPACT_THEME_PATHS[band])) as Theme
		for type_name: String in regular.get_type_list():
			for size_name: String in regular.get_font_size_list(type_name):
				var before: int = regular.get_font_size(size_name, type_name)
				var after: int = compact.get_font_size(size_name, type_name)
				assert_int(after).override_failure_message("%s %s.%s %d < %d" % [band, type_name, size_name, after,
						before]).is_greater_equal(before)
				assert_int(after).override_failure_message("%s %s.%s %d px" % [band, type_name, size_name, after]) \
						.is_greater_equal(CompactPolicy.font_floor(band, true))
		assert_int(compact.default_font_size).is_greater_equal(regular.default_font_size)
		var button: Button = auto_free(Button.new()) as Button
		button.text = "Đóng"
		# Chips and tabs carry an icon in the game (their content line is the icon line).
		button.icon = UiBuild.icon("check")
		button.theme = compact
		add_child(button)
		for variation: StringName in [&"", &"SecondaryButton", &"DangerButton", &"ChoiceChip", &"TabButton"]:
			button.theme_type_variation = variation
			assert_float(button.get_combined_minimum_size().y + 0.5).override_failure_message("%s %s" % [band,
					variation]).is_greater_equal(CompactPolicy.button_target(band, true))


func test_theme_service_and_ui_build_follow_compact_mode_without_reload() -> void:
	if AppState.theme_service == null:
		AppState.start_ui(get_tree().root)
	AppState.set_grade_band("g8_9")
	var service: ThemeService = AppState.theme_service
	var changed: Array[String] = []
	var on_density: Callable = func(band: String) -> void: changed.append(band)
	service.density_changed.connect(on_density)
	var regular: Theme = service.current_theme
	assert_float(UiBuild.target()).is_equal(44.0)
	AppState.set_compact(true)
	assert_bool(service.compact).is_true()
	assert_object(service.current_theme).is_not_same(regular)
	assert_array(changed).is_equal(["g8_9"])
	assert_float(UiBuild.target()).is_equal(float(UiTokens.DENSITY_COMPACT_BUTTON_MIN_HEIGHT))
	assert_float(UiBuild.target_for("g1_3")).is_equal(64.0)
	var label: Label = auto_free(Label.new()) as Label
	label.text = "12:05"
	label.theme_type_variation = &"ChatMetaLabel"
	add_child(label)
	assert_int(label.get_theme_font_size(&"font_size")).is_greater_equal(UiTokens.DENSITY_COMPACT_FONT_SMALL)
	AppState.set_compact(false)
	assert_object(service.current_theme).is_same(regular)
	assert_float(UiBuild.target()).is_equal(44.0)
	service.density_changed.disconnect(on_density)
	AppState.set_grade_band("g1_3")
