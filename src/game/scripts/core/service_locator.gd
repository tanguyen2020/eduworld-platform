class_name ServiceLocator
extends RefCounted
## Domain services of the signed-in session (GODOT_ARCHITECTURE §4: `RefCounted` services created by the
## ServiceLocator held in AppState after bootstrap, receiving ApiClient through their constructor so tests inject
## fakes). UI and gameplay scripts reach them through `AppState.services`, never ApiClient directly (G-4, R1).

var assignments: AssignmentService = null
var play_time: PlayTimeService = null
var telemetry: TelemetryService = null
## Activity player (F063): attempts, formula SVG cache and item media (RAM).
var learning: LearningService = null
var math_svg: MathSvgCache = null
var media: MediaLoader = null
var video_files: VideoFileCache = null
## Player profile, nickname, avatar and saved outfits (F070); character catalog cache + ownership view (F070/F075).
var profile: ProfileService = null
var inventory: InventoryService = null
## Hub multiplayer and own presence over the realtime connection (F092).
var realtime: GameRealtimeClient = null
var hub: HubSync = null
var presence: PresenceService = null
## `(code: String) -> void` for session-ending refusals; default `AuthService.handle_access_denied` (F062-AC-12/23).
var access_denied_handler: Callable = Callable()


func _init(rest: RestApiClient = null, app_state: Node = null, clock: ClientClock = null,
		realtime_client: GameRealtimeClient = null) -> void:
	telemetry = TelemetryService.new(rest, clock)
	assignments = AssignmentService.new(rest, app_state, telemetry)
	play_time = PlayTimeService.new()
	play_time.break_reminder_shown.connect(_on_break_reminder_shown)
	assignments.access_denied.connect(_on_access_denied)
	learning = LearningService.new(rest, telemetry, clock)
	learning.access_denied.connect(_on_access_denied)
	var transport: HttpTransport = rest.transport if rest != null else null
	math_svg = MathSvgCache.new(transport, _cdn_base_url())
	math_svg.failure_handler = func(_reason: String) -> void: learning.report_math_failed()
	media = MediaLoader.new(transport)
	video_files = VideoFileCache.new(rest)
	profile = ProfileService.new(rest, app_state)
	profile.access_denied.connect(_on_access_denied)
	inventory = InventoryService.new(rest)
	realtime = realtime_client if realtime_client != null else _realtime_autoload()
	hub = HubSync.new(realtime)
	presence = PresenceService.new(realtime, clock)
	if realtime != null:
		realtime.resync_required.connect(resync)
		if app_state != null:
			app_state.set("net_stats", realtime.stats)


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
	learning.clear()
	media.clear()
	profile.clear()
	play_time.configure({})
	telemetry.clear()
	hub.leave_area()


## F092-AC-05: the resume failed — read the profile again over REST, join the current area again, then Online.
func resync() -> void:
	await profile.fetch_profile()
	if not hub.desired_area.is_empty():
		await hub.join()
	if realtime != null:
		realtime.finish_resync()


static func _realtime_autoload() -> GameRealtimeClient:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null("RealtimeClient") as GameRealtimeClient if tree != null else null


static func _cdn_base_url() -> String:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var config: Node = tree.root.get_node_or_null("Config") if tree != null else null
	return String(config.get("cdn_base_url")) if config != null else ""


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
