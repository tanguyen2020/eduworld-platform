extends Node
## Autoload #14 `OrientationGuard` (F113; STUDENT_GAME_UX §2 landscape on every platform; GODOT_ARCHITECTURE §4,
## §10.2). Android/iOS never rotate to portrait (project.godot sensor landscape); a Web window (phone held upright,
## narrow browser window) can. While the window is taller than wide this service covers the game with the rotate
## prompt (RotatePrompt, a view only — AGENTS R1), pauses the SceneTree so the game takes no input and keeps its exact
## state, and on the Web tries `screen.orientation.lock('landscape')` (silently ignored where the browser refuses).
## Back to landscape: the prompt hides and the tree resumes — only if this guard paused it. Network and session keep
## running behind the prompt: ApiClient and RealtimeClient process always, the session heartbeat uses an always
## timer (ClientClock). Rules: OrientationPolicy (pure).

signal prompt_shown()
signal prompt_hidden()

const PROMPT_SCENE: String = "res://scenes/ui/rotate_prompt/RotatePrompt.tscn"
## Above every game layer (HUD, dialogs, toasts, the world snapshot of QualityManager).
const PROMPT_LAYER: int = 120
const LOCK_SCRIPT: String = "(function(){try{var o=screen.orientation;if(o&&o.lock){o.lock('landscape')" \
		+ ".catch(function(){});}}catch(e){}})()"

var prompt_visible: bool = false
## Test seams: how the tree is paused/read and how the Web lock is tried (defaults: SceneTree / JavaScriptBridge).
var set_paused: Callable = _set_tree_paused
var is_paused: Callable = _is_tree_paused
var lock_landscape: Callable = _lock_landscape
## Reacts to the window size (off in tests that drive `apply` themselves).
var follow_window: bool = true

var _layer: CanvasLayer = null
var _prompt: Control = null
var _paused_by_guard: bool = false


func _ready() -> void:
	process_mode = Node.PROCESS_MODE_ALWAYS
	if follow_window:
		get_tree().root.size_changed.connect(_on_window_resized)
		_on_window_resized.call_deferred()


func _exit_tree() -> void:
	if get_tree() != null and get_tree().root.size_changed.is_connected(_on_window_resized):
		get_tree().root.size_changed.disconnect(_on_window_resized)


func _on_window_resized() -> void:
	if follow_window and is_inside_tree():
		apply(get_tree().root.get_visible_rect().size)


## F113-AC-03/04: shows or hides the prompt for a logical view of `view` (the window after canvas stretch).
func apply(view: Vector2) -> void:
	if OrientationPolicy.is_portrait(view):
		_show(view)
	else:
		_hide()


## The prompt view (null until first shown).
func prompt() -> Control:
	return _prompt


func layer() -> CanvasLayer:
	return _layer


func _show(view: Vector2) -> void:
	_ensure_prompt()
	# The band Theme of the moment (density, font scale, contrast), set on the prompt itself so it also holds when
	# the prompt is not under a themed Window.
	if AppState.theme_service != null:
		_prompt.theme = AppState.theme_service.current_theme
	var scale: float = OrientationPolicy.prompt_scale(view, OrientationPolicy.base_short())
	_layer.scale = Vector2(scale, scale)
	_prompt.position = Vector2.ZERO
	_prompt.size = view / scale
	if prompt_visible:
		return
	prompt_visible = true
	_layer.visible = true
	if not bool(is_paused.call()):
		set_paused.call(true)
		_paused_by_guard = true
	lock_landscape.call()
	Log.info("orientation_prompt_shown", {"w": int(view.x), "h": int(view.y)})
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


func _set_tree_paused(paused: bool) -> void:
	get_tree().paused = paused


func _is_tree_paused() -> bool:
	return get_tree().paused


## F113-AC-07: a Web page may lock to landscape (Android Chrome in full screen after a gesture); anything else
## (no API, iOS Safari, desktop, no gesture) fails inside the page script and is ignored.
func _lock_landscape() -> void:
	if OS.has_feature("web"):
		JavaScriptBridge.eval(LOCK_SCRIPT, true)
