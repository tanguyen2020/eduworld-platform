extends Node
## Autoload #4 `AppState` (GODOT_ARCHITECTURE §4, plan F060): session state store. Never calls the network.
## Holds the player bootstrap and game session id (filled by F061), policies, feature flags, connectivity, the telemetry queue
## (moved into TelemetryService and sent to /game/telemetry, F062) and the ThemeService. Shows the MOCK badge when
## `Config.is_mock`. F062: the school assignments of the session (RAM only), the current zone/spawn point and the
## ServiceLocator of the domain services (GODOT_ARCHITECTURE §4).

signal player_changed(player: Dictionary)
signal grade_band_changed(band: String)
signal assignments_changed(count: int)
## F069: a timed activity started or ended (`challenge`, `exam`, later `boss`); "" = none — holds the level-up modal.
signal blocking_activity_changed(activity: StringName)
## F125: compact mode (a phone held sideways, CompactPolicy) switched on or off by OrientationGuard.
signal compact_changed(compact: bool)

const MOCK_BADGE_SCENE: String = "res://scenes/ui/debug/MockBadge.tscn"
const TELEMETRY_QUEUE_MAX: int = 100

var player: Dictionary = {}
var grade_band: String = ""
## F125 (STUDENT_GAME_UX §2): the window is a phone (short side < 500 density-independent px); set by OrientationGuard.
var compact: bool = false
var school_policies: Dictionary = {}
var feature_flags: Dictionary = {}
var connectivity: int = 0
var pending_telemetry: Array[Dictionary] = []
## Game session of this device (F061, `GameSession.game_session_id`, ADR-0083); RAM only.
var game_session_id: String = ""
var theme_service: ThemeService = null
## `MyAssignment` list of the signed-in student (F062-AC-07); never written to disk (GODOT_ARCHITECTURE §8.2).
var assignments: Array[MyAssignmentModel] = []
## Zone currently loaded (`world_area.area_id`) and the spawn point the server chose for it (empty = zone default).
var current_zone: String = ""
var spawn_point_id: String = ""
## World summary of the session (F071, WorldService): area count and the last `getWorldExploration`; RAM only.
var world: Dictionary = {}
## Domain services (created on first use after bootstrap; tests assign their own).
var services: ServiceLocator = null
## The student's party (F102, PartyService/PartyStore); null = no party. RAM only, never written to disk.
var party: PartyModel = null
## Realtime network counters (RTT p50/p95, reconnects, resume results) for the batched telemetry of F080 (F092 T028).
var net_stats: NetStats = null
var mock_badge: CanvasLayer = null
## F069 (data-model §2): the timed activity running now (`challenge` in Starting…Result, `exam` in the runner, `boss`
## phase in F110); "" = none. LevelUpQueue keeps S07 waiting while it is set (F069-AC-03, AC-UX-0701).
var blocking_activity: StringName = &""
## F111: `policies.social` of the bootstrap, display only ("Tạo club" shown or not); absent → club creation disabled.
var social_policy: SocialPolicyModel = SocialPolicyModel.from_policies({})
var _blockers: Array[StringName] = []


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
	theme_service.compact = compact
	theme_service.attach(window)
	var config: Node = get_node_or_null("/root/Config")
	if config != null and bool(config.get("is_mock")):
		show_mock_badge()


## Stores the bootstrap from F061 and switches density to the student's grade band.
func set_player(value: Dictionary) -> void:
	player = value
	feature_flags = value.get("feature_flags", feature_flags) as Dictionary
	school_policies = value.get("policies", school_policies) as Dictionary
	social_policy = SocialPolicyModel.from_policies(school_policies)
	get_services().start_session(value)
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


## F125-AC-01: compact mode on/off (OrientationGuard); the Theme follows without reloading the scene.
func set_compact(value: bool) -> void:
	if value == compact:
		return
	compact = value
	if theme_service != null:
		theme_service.set_compact(value)
	compact_changed.emit(value)


## F070: the profile read after the game session (nickname chosen in S02, level) replaces the display fields of
## `player.game_profile` so the HUD shows the current nickname. No session restart, no network.
func set_game_profile(display: Dictionary) -> void:
	if player.is_empty():
		return
	var current: Dictionary = player.get("game_profile") as Dictionary if player.get("game_profile") is Dictionary else {}
	current.merge(display, true)
	player["game_profile"] = current
	player_changed.emit(player)


func clear_player() -> void:
	player = {}
	school_policies = {}
	social_policy = SocialPolicyModel.from_policies({})
	feature_flags = {}
	spawn_point_id = ""
	if services != null:
		services.end_session()
	var none: Array[MyAssignmentModel] = []
	set_assignments(none)
	player_changed.emit(player)
	set_grade_band(ThemeService.default_band())


## Replaces the assignment list (AssignmentService) and notifies the HUD chip / quest journal.
func set_assignments(list: Array[MyAssignmentModel]) -> void:
	assignments = list.duplicate()
	assignments_changed.emit(assignments.size())


## The session services; created on first use with the ApiClient autoload.
func get_services() -> ServiceLocator:
	if services == null:
		var rest: RestApiClient = get_node_or_null("/root/ApiClient") as RestApiClient
		services = ServiceLocator.new(rest, self, rest.clock if rest != null else null)
	return services


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


## WorldService publishes its state here (F071); never written to disk.
func set_world(state: Dictionary) -> void:
	world = state


## F069: a timed activity starts (several may overlap; the first still open is reported).
func begin_blocking(activity: StringName) -> void:
	if activity in _blockers:
		return
	_blockers.append(activity)
	_set_blocking()


func end_blocking(activity: StringName) -> void:
	_blockers.erase(activity)
	_set_blocking()


func _set_blocking() -> void:
	var next: StringName = _blockers[0] if not _blockers.is_empty() else &""
	if next == blocking_activity:
		return
	blocking_activity = next
	blocking_activity_changed.emit(blocking_activity)
