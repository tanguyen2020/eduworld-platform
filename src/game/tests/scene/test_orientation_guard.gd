extends GdUnitTestSuite
## F113-AC-03, F113-AC-04, F113-AC-05, F113-AC-07 (STUDENT_GAME_UX §2): OrientationGuard shows the rotate prompt while
## the view is taller than wide and hides it in landscape (rotation or resize), pauses the game — never the network —
## and resumes it with the same scene state, and never resumes a tree paused by someone else.

const GuardScript := preload("res://scripts/core/orientation_guard.gd")

var guard: Node = null
var paused: bool = false
var pause_calls: Array[bool] = []
var lock_calls: int = 0
var content_scale: float = 1.0
var scale_calls: Array[float] = []


func before_test() -> void:
	paused = false
	pause_calls.clear()
	lock_calls = 0
	content_scale = 1.0
	scale_calls.clear()
	guard = GuardScript.new()
	guard.set("follow_window", false)
	guard.set("set_paused", func(value: bool) -> void:
		paused = value
		pause_calls.append(value))
	guard.set("is_paused", func() -> bool: return paused)
	guard.set("lock_landscape", func() -> void: lock_calls += 1)
	guard.set("set_content_scale", func(value: float) -> void:
		content_scale = value
		scale_calls.append(value))
	guard.set("get_content_scale", func() -> float: return content_scale)
	add_child(guard)


func after_test() -> void:
	guard.free()
	get_tree().paused = false


func _prompt() -> Control:
	return guard.call("prompt") as Control


func _layer() -> CanvasLayer:
	return guard.call("layer") as CanvasLayer


func test_portrait_shows_the_prompt_and_landscape_hides_it() -> void:
	var shown: Array[int] = [0]
	var hidden: Array[int] = [0]
	guard.connect("prompt_shown", func() -> void: shown[0] += 1)
	guard.connect("prompt_hidden", func() -> void: hidden[0] += 1)
	guard.call("apply", Vector2(1280, 720))
	assert_bool(bool(guard.get("prompt_visible"))).is_false()
	assert_object(_prompt()).is_null()
	# The phone is turned upright (390×844).
	guard.call("apply", Vector2(390, 844))
	assert_bool(bool(guard.get("prompt_visible"))).is_true()
	assert_bool(_layer().visible).is_true()
	assert_int(_layer().layer).is_greater(100)
	assert_array(pause_calls).is_equal([true])
	assert_int(lock_calls).is_equal(1)
	assert_int(shown[0]).is_equal(1)
	# Turned back: hidden, resumed, content scale restored.
	guard.call("apply", Vector2(844, 390))
	assert_bool(bool(guard.get("prompt_visible"))).is_false()
	assert_bool(_layer().visible).is_false()
	assert_array(pause_calls).is_equal([true, false])
	assert_int(hidden[0]).is_equal(1)
	assert_float(content_scale).is_equal(1.0)
	# A square view is landscape.
	guard.call("apply", Vector2(900, 900))
	assert_bool(bool(guard.get("prompt_visible"))).is_false()


func test_a_resize_while_portrait_only_relayouts_the_prompt() -> void:
	guard.call("apply", Vector2(720, 1280))
	guard.call("apply", Vector2(390, 844))
	assert_array(pause_calls).is_equal([true])
	assert_int(lock_calls).is_equal(1)
	# F113-AC-05: the window content scale lays the prompt on the short side of the base canvas (written once: the
	# second portrait size needs the same factor), and the prompt fills the visible rect (full-rect anchors).
	assert_array(scale_calls).is_equal([1280.0 / 720.0])
	assert_float(content_scale).is_equal_approx(1280.0 / 720.0, 0.0001)
	assert_vector(_prompt().size).is_equal_approx(_prompt().get_viewport_rect().size, Vector2(0.5, 0.5))
	assert_float(_layer().scale.x).is_equal(1.0)
	# An earlier content scale (not ours) comes back when the prompt hides.
	guard.call("apply", Vector2(1280, 720))
	assert_float(content_scale).is_equal(1.0)


func test_the_previous_content_scale_is_restored() -> void:
	content_scale = 1.25
	guard.call("apply", Vector2(390, 844))
	assert_float(content_scale).is_equal_approx(1280.0 / 720.0, 0.0001)
	guard.call("apply", Vector2(1280, 720))
	assert_float(content_scale).is_equal(1.25)


func test_a_tree_paused_by_someone_else_stays_paused() -> void:
	paused = true
	guard.call("apply", Vector2(390, 844))
	assert_bool(bool(guard.get("prompt_visible"))).is_true()
	guard.call("apply", Vector2(1280, 720))
	assert_array(pause_calls).is_empty()
	assert_bool(paused).is_true()


## The real SceneTree pause: the game stops (no process, no input) while the prompt, the network clients and the
## session idle watcher keep running; back in landscape the same scene resumes with its state. No frame is awaited
## while the tree is paused (the test runner itself is pausable).
func test_real_pause_stops_the_game_not_the_network_and_keeps_state() -> void:
	guard.set("set_paused", func(value: bool) -> void: get_tree().paused = value)
	guard.set("is_paused", func() -> bool: return get_tree().paused)
	var game: Control = auto_free(Control.new()) as Control
	add_child(game)
	var field: LineEdit = LineEdit.new()
	field.text = "18 quả"
	game.add_child(field)
	guard.call("apply", Vector2(390, 844))
	assert_bool(get_tree().paused).is_true()
	assert_bool(game.can_process()).is_false()
	assert_bool(_prompt().can_process()).is_true()
	assert_bool(ApiClient.can_process()).is_true()
	assert_bool(RealtimeClient.can_process()).is_true()
	assert_int(_prompt().mouse_filter).is_equal(Control.MOUSE_FILTER_STOP)
	guard.call("apply", Vector2(1280, 720))
	assert_bool(get_tree().paused).is_false()
	assert_bool(game.can_process()).is_true()
	assert_bool(is_instance_valid(field)).is_true()
	assert_str(field.text).is_equal("18 quả")


func test_prompt_shows_mascot_icon_and_translated_texts() -> void:
	guard.call("apply", Vector2(390, 844))
	var prompt: Control = _prompt()
	assert_object(prompt.find_child("Mascot", true, false)).is_instanceof(CloudyMascot)
	var phone: TextureRect = prompt.find_child("Phone", true, false) as TextureRect
	assert_object(phone.texture).is_not_null()
	assert_str(phone.texture.resource_path).ends_with("rotate_phone.svg")
	assert_str((prompt.find_child("Title", true, false) as Label).text).is_equal("ui.rotate.title")
	assert_str((prompt.find_child("Hint", true, false) as Label).text).is_equal("ui.rotate.hint")
	assert_str((prompt.find_child("Desktop", true, false) as Label).text).is_equal("ui.rotate.desktop")
	assert_str(tr("ui.rotate.title")).is_not_equal("ui.rotate.title")


func test_autoload_guard_follows_the_window_and_always_processes() -> void:
	var autoload: Node = get_tree().root.get_node("OrientationGuard")
	assert_int(autoload.process_mode).is_equal(Node.PROCESS_MODE_ALWAYS)
	assert_bool(bool(autoload.get("follow_window"))).is_true()
	# The test window is landscape: no prompt, tree running.
	assert_bool(bool(autoload.get("prompt_visible"))).is_false()
	assert_bool(get_tree().paused).is_false()


## One message for every grade: the prompt uses the largest density (g1_3) even for an older student's band.
func test_prompt_text_uses_the_largest_density() -> void:
	var band: String = AppState.grade_band
	AppState.set_grade_band("g8_9")
	guard.call("apply", Vector2(390, 844))
	var title: Label = _prompt().find_child("Title", true, false) as Label
	var largest: Theme = load(String(UiTokens.THEME_PATHS[UiTokens.DENSITY_BANDS[0]])) as Theme
	assert_int(title.get_theme_font_size(&"font_size")).is_equal(largest.get_font_size(&"font_size", &"HeadingLabel"))
	var hint: Label = _prompt().find_child("Hint", true, false) as Label
	assert_int(hint.get_theme_font_size(&"font_size")).is_greater_equal(int(UiTokens.DENSITY_FONT_SMALL["g8_9"]))
	AppState.set_grade_band(band)
