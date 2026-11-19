extends Node
## Autoload #4 `AppState` (GODOT_ARCHITECTURE §4, plan F060): session state store. Never calls the network.
## Holds the player bootstrap and game session id (filled by F061), policies, feature flags, connectivity, the telemetry queue
## (flushed to /game/telemetry by a later feature) and the ThemeService. Shows the MOCK badge when `Config.is_mock`.

signal player_changed(player: Dictionary)
signal grade_band_changed(band: String)

const MOCK_BADGE_SCENE: String = "res://scenes/ui/debug/MockBadge.tscn"
const TELEMETRY_QUEUE_MAX: int = 100

var player: Dictionary = {}
var grade_band: String = ""
var school_policies: Dictionary = {}
var feature_flags: Dictionary = {}
var connectivity: int = 0
var pending_telemetry: Array[Dictionary] = []
## Game session of this device (F061, `GameSession.game_session_id`, ADR-0083); RAM only.
var game_session_id: String = ""
var theme_service: ThemeService = null
var mock_badge: CanvasLayer = null


func _init() -> void:
	grade_band = ThemeService.default_band()


## Called once by Boot after Config is valid: applies the Theme (pre-bootstrap density), wires Settings and
## shows the MOCK badge for the mock environment (R17).
func start_ui(window: Window) -> void:
	if theme_service == null:
		theme_service = ThemeService.new(grade_band)
		theme_service.density_changed.connect(_on_density_changed)
	var settings: Node = get_node_or_null("/root/Settings")
	if settings != null:
		theme_service.font_scale_level = int(settings.get("font_scale"))
		theme_service.high_contrast = bool(settings.get("high_contrast"))
		_connect_once(settings, "font_scale_changed", theme_service.set_font_scale)
		_connect_once(settings, "high_contrast_changed", theme_service.set_high_contrast)
	theme_service.attach(window)
	var config: Node = get_node_or_null("/root/Config")
	if config != null and bool(config.get("is_mock")):
		show_mock_badge()


## Stores the bootstrap from F061 and switches density to the student's grade band.
func set_player(value: Dictionary) -> void:
	player = value
	feature_flags = value.get("feature_flags", feature_flags) as Dictionary
	school_policies = value.get("policies", school_policies) as Dictionary
	player_changed.emit(player)
	set_grade_band(String(value.get("grade_band", "")))


func set_grade_band(band: String) -> void:
	var next: String = band if ThemeService.is_valid_band(band) else ThemeService.default_band()
	if next == grade_band:
		return
	grade_band = next
	grade_band_changed.emit(grade_band)
	if theme_service != null:
		theme_service.set_band(grade_band)


func clear_player() -> void:
	player = {}
	school_policies = {}
	feature_flags = {}
	player_changed.emit(player)
	set_grade_band(ThemeService.default_band())


## Queues a telemetry event (fields go through Log redaction first, R14); oldest dropped beyond the cap.
func queue_telemetry(event_name: String, fields: Dictionary = {}) -> void:
	var log_node: Node = get_node_or_null("/root/Log")
	var safe: Dictionary = fields
	if log_node != null:
		safe = log_node.call("redact", fields) as Dictionary
	pending_telemetry.append({"event": event_name, "fields": safe, "ts": Time.get_unix_time_from_system()})
	while pending_telemetry.size() > TELEMETRY_QUEUE_MAX:
		pending_telemetry.pop_front()


func show_mock_badge() -> void:
	if mock_badge != null:
		return
	var scene: PackedScene = load(MOCK_BADGE_SCENE) as PackedScene
	if scene == null:
		return
	mock_badge = scene.instantiate() as CanvasLayer
	add_child(mock_badge)


func _on_density_changed(band: String) -> void:
	var bus: Node = get_node_or_null("/root/EventBus")
	if bus != null:
		bus.emit_signal("density_changed", band)


static func _connect_once(source: Object, signal_name: StringName, target: Callable) -> void:
	if not source.is_connected(signal_name, target):
		source.connect(signal_name, target)
