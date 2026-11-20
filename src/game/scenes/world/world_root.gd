extends Node
## In-world screen after login (replaces the F061 WorldPlaceholder; `GameStateMachine.IN_WORLD_SCENE`). Composes the
## Main Hub zone, the HUD, the panel host and the dialogs, and wires them: Zone ↔ HUD signals, `EventBus.ui_requested`
## → registered screen or `ui.activity_player.update_required` (F062-AC-09/10), the first-run quality benchmark
## (F062-AC-14) and device profiles (F062-AC-15), the break reminder (F062-AC-21/22), the curfew warning banner
## (F061-AC-36) and the periodic telemetry flush (F062-AC-24). Glue only: no business rule, no network call
## (services do the work, AGENTS R1).

## Screens that hold the break reminder while open (F062-AC-22): a panel sets `blocks_break_reminder = true`.
const HOLD_PROPERTY: StringName = &"blocks_break_reminder"
const BENCHMARK_HOLD: StringName = &"quality_benchmark"
const UPDATE_REQUIRED_KEY: String = "ui.activity_player.update_required"

var zone: ZoneRoot = null
var panels: Dictionary = {}
var open_screen: StringName = &""

@onready var _hud: CanvasLayer = %MainHud
@onready var _panel_root: Control = %PanelRoot
@onready var _break_dialog: PanelContainer = %BreakReminderDialog
@onready var _notice: PanelContainer = %Notice
@onready var _benchmark: Control = %QualityBenchmark


func _ready() -> void:
	zone = %Zone as ZoneRoot
	_hud.connect("context_action_pressed", _on_context_action_pressed)
	_hud.connect("journal_pressed", _on_journal_pressed)
	_hud.connect("settings_pressed", _on_settings_pressed)
	_hud.connect("home_pressed", zone.return_home)
	_hud.connect("joystick_moved", _on_joystick_moved)
	zone.interaction_focus_changed.connect(_on_focus_changed)
	EventBus.ui_requested.connect(_on_ui_requested)
	AuthService.curfew_warning_raised.connect(_on_curfew_warning_raised)
	var services: ServiceLocator = AppState.get_services()
	services.play_time.break_reminder_shown.connect(_on_break_reminder_shown)
	_benchmark.connect("finished", _on_benchmark_finished)
	_start_quality()


func _exit_tree() -> void:
	if EventBus.ui_requested.is_connected(_on_ui_requested):
		EventBus.ui_requested.disconnect(_on_ui_requested)
	if AuthService.curfew_warning_raised.is_connected(_on_curfew_warning_raised):
		AuthService.curfew_warning_raised.disconnect(_on_curfew_warning_raised)
	var services: ServiceLocator = AppState.services
	if services != null and services.play_time.break_reminder_shown.is_connected(_on_break_reminder_shown):
		services.play_time.break_reminder_shown.disconnect(_on_break_reminder_shown)


func _process(delta: float) -> void:
	if AppState.player.is_empty():
		return
	var services: ServiceLocator = AppState.get_services()
	services.play_time.tick(delta)
	services.telemetry.drain_app_state(AppState)
	if services.telemetry.can_flush():
		services.telemetry.flush()


## Opens a registered screen or explains that this client cannot open it yet.
func _on_ui_requested(screen_id: StringName, args: Dictionary) -> void:
	var path: String = UiScreenRegistry.path_for(screen_id)
	if path.is_empty():
		Log.info("ui_screen_not_available", {"screen_id": String(screen_id)})
		_notice.call("open", UPDATE_REQUIRED_KEY)
		return
	if not open_screen.is_empty() and open_screen != screen_id:
		_close_panel(open_screen)
	var panel: Control = panels.get(screen_id) as Control
	if panel == null:
		panel = (load(path) as PackedScene).instantiate() as Control
		panel.connect("closed", _on_panel_closed.bind(screen_id))
		_panel_root.add_child(panel)
		panels[screen_id] = panel
	open_screen = screen_id
	_set_player_input(false)
	if panel.get(HOLD_PROPERTY) == true:
		AppState.get_services().play_time.hold(screen_id)
	panel.call("open", args)


func _close_panel(screen_id: StringName) -> void:
	var panel: Control = panels.get(screen_id) as Control
	if panel != null and panel.visible:
		panel.call("close")


func _on_panel_closed(screen_id: StringName) -> void:
	if open_screen == screen_id:
		open_screen = &""
		_set_player_input(true)
	AppState.get_services().play_time.release(screen_id)
	EventBus.ui_closed.emit(screen_id)


func _start_quality() -> void:
	var config: ClientConfig = AuthService.version_check.config if AuthService.version_check != null else null
	if config != null and not config.device_profiles.is_empty():
		QualityManager.apply_device_profiles(config.device_profiles)
	if not QualityManager.needs_benchmark() or zone.player == null:
		_benchmark.hide()
		return
	_set_player_input(false)
	_hud.visible = false
	AppState.get_services().play_time.hold(BENCHMARK_HOLD)
	_benchmark.call("start", zone.player.get("camera_rig"))


func _on_benchmark_finished(_preset: String) -> void:
	_hud.visible = true
	_set_player_input(true)
	AppState.get_services().play_time.release(BENCHMARK_HOLD)


func _set_player_input(enabled: bool) -> void:
	if zone != null and zone.player != null:
		zone.player.set("input_enabled", enabled)
		if not enabled:
			zone.player.call("set_move_input", Vector2.ZERO)


func _on_context_action_pressed() -> void:
	zone.interact_focused()


func _on_journal_pressed() -> void:
	EventBus.ui_requested.emit(&"quest_journal", {"tab": "assigned"})


func _on_settings_pressed() -> void:
	EventBus.ui_requested.emit(&"settings", {})


func _on_joystick_moved(vector: Vector2) -> void:
	if zone.player != null:
		zone.player.call("set_move_input", vector)


func _on_focus_changed(interactable_id: String, prompt_key: String) -> void:
	_hud.call("set_context_action", interactable_id, prompt_key)


func _on_break_reminder_shown(_reminder_no: int, minutes_played: int) -> void:
	_break_dialog.call("open", minutes_played)


func _on_curfew_warning_raised(minutes_left: int) -> void:
	_hud.call("show_banner", "ui.login.curfew_warning", {"minutes": minutes_left})
