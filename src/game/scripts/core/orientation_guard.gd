extends Node
## Autoload #14 `OrientationGuard` (F113; STUDENT_GAME_UX §2 landscape on every platform; GODOT_ARCHITECTURE §4,
## §10.2). Android/iOS never rotate to portrait (project.godot sensor landscape); a Web window (phone held upright,
## narrow browser window) can. While the window is taller than wide this service covers the game with the rotate
## prompt (RotatePrompt, a view only — AGENTS R1), pauses the SceneTree so the game takes no input and keeps its exact
## state, and on the Web tries `screen.orientation.lock('landscape')` (silently ignored where the browser refuses).
## Back to landscape: the prompt hides and the tree resumes — only if this guard paused it. Network and session keep
## running behind the prompt: ApiClient and RealtimeClient process always, the session heartbeat uses an always
## timer (ClientClock). While shown, the window `content_scale_factor` lays the prompt out on a canvas whose short
## side is the base short side (720), so its text renders at the band size, sharp (font oversampling follows the content
## scale); the paused game behind relayouts and is restored with the factor. Rules: OrientationPolicy (pure).
## F125 (STUDENT_GAME_UX §2 compact breakpoint): the same window content scale also carries compact mode — a window whose
## short side is below 500 density-independent px (a phone) gets the factor that makes the logical canvas 540 tall
## (CompactPolicy) and AppState switches to the compact Theme; otherwise the factor is 1. Decided again on every window
## resize, no scene reload. While the rotate prompt shows its factor wins; when it hides the factor of the current mode
## comes back (F125-AC-02). The factor is written only when it changes (no flicker between the two landscape sides).

signal prompt_shown()
signal prompt_hidden()

const PROMPT_SCENE: String = "res://scenes/ui/rotate_prompt/RotatePrompt.tscn"
## Above every game layer (HUD, dialogs, toasts, the world snapshot of QualityManager).
const PROMPT_LAYER: int = 120
const LOCK_SCRIPT: String = "(function(){try{var o=screen.orientation;if(o&&o.lock){o.lock('landscape')" \
		+ ".catch(function(){});}}catch(e){}})()"

var prompt_visible: bool = false
## Test seams: how the tree is paused/read, the content scale written/read and the Web lock tried (defaults: SceneTree,
## root Window, JavaScriptBridge).
var set_paused: Callable = _set_tree_paused
var is_paused: Callable = _is_tree_paused
var set_content_scale: Callable = _set_root_content_scale
var get_content_scale: Callable = _get_root_content_scale
var lock_landscape: Callable = _lock_landscape
## F125 seams: physical px per density-independent px of the screen, height of an open on-screen keyboard (px) and the
## receiver of the compact mode (defaults: DisplayServer, AppState).
var get_density: Callable = _screen_density
var get_keyboard_height: Callable = _keyboard_height
var set_compact: Callable = _set_app_compact
## F125: compact mode decided by the last `apply`.
var compact: bool = false
## Reacts to the window size (off in tests that drive `apply` themselves).
var follow_window: bool = true

var _layer: CanvasLayer = null
var _prompt: Control = null
var _paused_by_guard: bool = false
var _theme_service: ThemeService = null


func _ready() -> void:
	process_mode = Node.PROCESS_MODE_ALWAYS
	if follow_window:
		get_tree().root.size_changed.connect(_on_window_resized)
		_on_window_resized.call_deferred()


func _exit_tree() -> void:
	if get_tree() != null and get_tree().root.size_changed.is_connected(_on_window_resized):
		get_tree().root.size_changed.disconnect(_on_window_resized)


func _on_window_resized() -> void:
	# A headless run (server, test runner) has no screen: its placeholder window is neither a phone nor portrait.
	if DisplayServer.get_name() == "headless":
		return
	if follow_window and is_inside_tree():
		apply(Vector2(get_tree().root.size))


## F113-AC-03/04/05: shows or hides the prompt for a window of `window` pixels (portrait = taller than wide).
## F125-AC-01/02: decides compact mode for the window and sets the content scale of the mode (the prompt factor while
## the prompt shows).
func apply(window: Vector2) -> void:
	var keyboard: float = maxf(float(get_keyboard_height.call()), 0.0)
	var next: bool = CompactPolicy.is_compact(window, float(get_density.call()), keyboard)
	if next != compact:
		compact = next
		set_compact.call(compact)
		Log.info("compact_mode_changed", {"compact": compact, "w": int(window.x), "h": int(window.y)})
	if OrientationPolicy.is_portrait(window):
		_show(window)
	else:
		_hide()
		_write_scale(mode_scale(window + Vector2(0.0, keyboard)))


## F125-AC-01: content scale of the current mode for a window of `window` pixels (1 outside compact mode).
func mode_scale(window: Vector2) -> float:
	return CompactPolicy.content_scale(window, OrientationPolicy.base_size(), compact)


## The prompt view (null until first shown).
func prompt() -> Control:
	return _prompt


func layer() -> CanvasLayer:
	return _layer


func _show(window: Vector2) -> void:
	_ensure_prompt()
	_prompt.theme = _prompt_theme()
	var was_visible: bool = prompt_visible
	prompt_visible = true
	_write_scale(OrientationPolicy.prompt_content_scale(window, OrientationPolicy.base_size()))
	if was_visible:
		return
	_layer.visible = true
	if not bool(is_paused.call()):
		set_paused.call(true)
		_paused_by_guard = true
	lock_landscape.call()
	Log.info("orientation_prompt_shown", {"w": int(window.x), "h": int(window.y)})
	prompt_shown.emit()


func _hide() -> void:
	if not prompt_visible:
		return
	prompt_visible = false
	if _layer != null:
		_layer.visible = false
	if _paused_by_guard:
		_paused_by_guard = false
		set_paused.call(false)
	Log.info("orientation_prompt_hidden", {})
	prompt_hidden.emit()


## Setting the factor resizes the visible rect (size_changed again): written only when it changes.
func _write_scale(factor: float) -> void:
	if not is_equal_approx(float(get_content_scale.call()), factor):
		set_content_scale.call(factor)


func _ensure_prompt() -> void:
	if _layer != null:
		return
	_layer = CanvasLayer.new()
	_layer.name = "RotatePromptLayer"
	_layer.layer = PROMPT_LAYER
	_layer.process_mode = Node.PROCESS_MODE_ALWAYS
	_layer.visible = false
	add_child(_layer)
	_prompt = (load(PROMPT_SCENE) as PackedScene).instantiate() as Control
	_prompt.process_mode = Node.PROCESS_MODE_ALWAYS
	_layer.add_child(_prompt)


## One short message read by every grade: the largest density (`UiTokens.DENSITY_BANDS[0]`, g1_3 — never smaller
## than the student's band) with the student's font scale and contrast settings, set on the prompt itself so it also
## holds where the prompt is not under the themed Window.
func _prompt_theme() -> Theme:
	if _theme_service == null:
		_theme_service = ThemeService.new(UiTokens.DENSITY_BANDS[0])
	var source: ThemeService = AppState.theme_service
	if source != null:
		_theme_service.font_scale_level = source.font_scale_level
		_theme_service.high_contrast = source.high_contrast
	return _theme_service.build_theme()


func _set_root_content_scale(factor: float) -> void:
	get_tree().root.content_scale_factor = factor


func _get_root_content_scale() -> float:
	return get_tree().root.content_scale_factor


func _screen_density() -> float:
	var density: float = DisplayServer.screen_get_scale()
	return density if density > 0.0 else 1.0


func _keyboard_height() -> float:
	return float(DisplayServer.virtual_keyboard_get_height()) if DisplayServer.has_feature(
			DisplayServer.FEATURE_VIRTUAL_KEYBOARD) else 0.0


func _set_app_compact(value: bool) -> void:
	AppState.set_compact(value)


func _set_tree_paused(paused: bool) -> void:
	get_tree().paused = paused


func _is_tree_paused() -> bool:
	return get_tree().paused


## F113-AC-07: a Web page may lock to landscape (Android Chrome in full screen after a gesture); anything else
## (no API, iOS Safari, desktop, no gesture) fails inside the page script and is ignored.
func _lock_landscape() -> void:
	if OS.has_feature("web"):
		JavaScriptBridge.eval(LOCK_SCRIPT, true)
