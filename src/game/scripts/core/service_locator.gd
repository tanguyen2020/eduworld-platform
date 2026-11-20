class_name ServiceLocator
extends RefCounted
## Domain services of the signed-in session (GODOT_ARCHITECTURE §4: `RefCounted` services created by the
## ServiceLocator held in AppState after bootstrap, receiving ApiClient through their constructor so tests inject
## fakes). UI and gameplay scripts reach them through `AppState.services`, never ApiClient directly (G-4, R1).

var assignments: AssignmentService = null
var play_time: PlayTimeService = null
var telemetry: TelemetryService = null
## `(code: String) -> void` for session-ending refusals; default `AuthService.handle_access_denied` (F062-AC-12/23).
var access_denied_handler: Callable = Callable()


func _init(rest: RestApiClient = null, app_state: Node = null, clock: ClientClock = null) -> void:
	telemetry = TelemetryService.new(rest, clock)
	assignments = AssignmentService.new(rest, app_state, telemetry)
	play_time = PlayTimeService.new()
	play_time.break_reminder_shown.connect(_on_break_reminder_shown)
	assignments.access_denied.connect(_on_access_denied)


## Applies the bootstrap of a new session (policies, product telemetry switch) and drops the previous data.
func start_session(bootstrap: Dictionary) -> void:
	assignments.clear()
	play_time.configure(bootstrap.get("policies", {}) as Dictionary)
	var flags: Variant = bootstrap.get("feature_flags", {})
	# CONFIG_DEFAULTS `analytics_product_telemetry_enabled` (default on) as published in the bootstrap feature flags.
	telemetry.product_telemetry_enabled = not (flags is Dictionary
			and (flags as Dictionary).get("product_telemetry_enabled") is bool
			and not bool((flags as Dictionary)["product_telemetry_enabled"]))


## Session ended (logout, expiry, tenant change): drops the list and the unsent telemetry so nothing of this student
## is ever sent with the next student's token or tenant.
func end_session() -> void:
	assignments.clear()
	play_time.configure({})
	telemetry.clear()


func _on_break_reminder_shown(reminder_no: int, minutes_played: int) -> void:
	telemetry.queue_break_reminder(reminder_no, minutes_played)


func _on_access_denied(code: String) -> void:
	if access_denied_handler.is_valid():
		access_denied_handler.call(code)
		return
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var auth: Node = tree.root.get_node_or_null("AuthService") if tree != null else null
	if auth != null:
		auth.call("handle_access_denied", code)
