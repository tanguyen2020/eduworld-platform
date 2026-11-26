extends Node
## In-world screen after login (replaces the F061 WorldPlaceholder; `GameStateMachine.IN_WORLD_SCENE`). Composes the
## Main Hub zone, the HUD, the panel host and the dialogs, and wires them: Zone ↔ HUD signals, `EventBus.ui_requested`
## → registered screen or `ui.activity_player.update_required` (F062-AC-09/10), the first-run quality benchmark
## (F062-AC-14) and device profiles (F062-AC-15), the break reminder (F062-AC-21/22), the curfew warning banner
## (F061-AC-36) and the periodic telemetry flush (F062-AC-24); F070: the local avatar drawn from the profile, the
## emote wheel with the equipped emotes and the wardrobe (HUD button / I); F092: the hub instance through HubSync —
## join on entering, the local state sent by the service, the other players (RemotePlayerSync), server corrections
## blended in 200 ms, emotes to the others, hub notices, presence (activity screens, idle). F071: zone changes through
## wind gates, the World Map and "Về Quảng trường" (SceneRouter.request_area → ZoneLoader swap, area_transition_max),
## NPC dialogues, discoveries, the area unlocked notice and the return next to the NPC after a learning activity. Glue
## only: no business rule, no network call (services do the work, AGENTS R1).

## Screens that hold the break reminder while open (F062-AC-22): a panel sets `blocks_break_reminder = true`.
const HOLD_PROPERTY: StringName = &"blocks_break_reminder"
const BENCHMARK_HOLD: StringName = &"quality_benchmark"
const UPDATE_REQUIRED_KEY: String = "ui.activity_player.update_required"
## Stable area_id of Quảng trường Tia Sáng (WORLD_DESIGN §3, the spawn of a first entry).
const HUB_AREA_ID: String = "main_hub"
const QUEST_TRACKER_SCENE: PackedScene = preload("res://scenes/quest/QuestTracker.tscn")

var zone: ZoneRoot = null
var panels: Dictionary = {}
var open_screen: StringName = &""
## F092: the other players of the instance and the correction of the local position.
var remote_players: RemotePlayerSync = null
var correction: PositionCorrection = PositionCorrection.new()
var hub: HubSync = null
## F071: return point of a learning activity opened from an NPC/station and the start of a zone change.
var activity_return: ActivityReturn = ActivityReturn.new()
var _area_started_ms: int = 0

@onready var _hud: CanvasLayer = %MainHud
@onready var _panel_root: Control = %PanelRoot
@onready var _break_dialog: PanelContainer = %BreakReminderDialog
@onready var _notice: PanelContainer = %Notice
@onready var _benchmark: Control = %QualityBenchmark
@onready var _emotes: EmoteWheel = %EmoteWheel


func _ready() -> void:
	zone = %Zone as ZoneRoot
	_hud.connect("context_action_pressed", _on_context_action_pressed)
	_hud.connect("journal_pressed", _on_journal_pressed)
	_hud.connect("settings_pressed", _on_settings_pressed)
	_hud.connect("wardrobe_pressed", _on_wardrobe_pressed)
	_hud.connect("home_pressed", _on_home_pressed)
	_hud.connect("map_pressed", _on_map_pressed)
	_hud.connect("joystick_moved", _on_joystick_moved)
	_hud.connect("notifications_pressed", _on_notifications_pressed)
	_hud.connect("next_step_pressed", _on_next_step_pressed)
	_bind_zone(zone)
	SceneRouter.area_entered.connect(_on_area_entered)
	SceneRouter.area_change_failed.connect(_on_area_change_failed)
	EventBus.ui_requested.connect(_on_ui_requested)
	EventBus.onboarding_requested.connect(_on_onboarding_requested)
	AuthService.curfew_warning_raised.connect(_on_curfew_warning_raised)
	var services: ServiceLocator = AppState.get_services()
	services.play_time.break_reminder_shown.connect(_on_break_reminder_shown)
	services.profile.avatar_saved.connect(_on_avatar_saved)
	if services.world != null:
		services.world.area_unlocked.connect(_on_area_unlocked)
	var local: PlayerController = zone.player as PlayerController
	# Emotes play locally at once; HubSync sends `hub.player.emote` to the others (F092-AC-16).
	_emotes.anim_driver = local.anim_driver if local != null else null
	_apply_avatar(services.profile.avatar_config())
	_emotes.emote_selected.connect(_on_emote_selected)
	_start_hub(services)
	_benchmark.connect("finished", _on_benchmark_finished)
	_start_quality()


func _exit_tree() -> void:
	_stop_hub()
	for pair: Array in [[SceneRouter.area_entered, _on_area_entered], [SceneRouter.area_change_failed, _on_area_change_failed]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	var world: WorldService = AppState.services.world if AppState.services != null else null
	if world != null and world.area_unlocked.is_connected(_on_area_unlocked):
		world.area_unlocked.disconnect(_on_area_unlocked)
	var locator: ServiceLocator = AppState.services
	if locator != null and locator.profile.avatar_saved.is_connected(_on_avatar_saved):
		locator.profile.avatar_saved.disconnect(_on_avatar_saved)
	if EventBus.ui_requested.is_connected(_on_ui_requested):
		EventBus.ui_requested.disconnect(_on_ui_requested)
	if EventBus.onboarding_requested.is_connected(_on_onboarding_requested):
		EventBus.onboarding_requested.disconnect(_on_onboarding_requested)
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
	services.presence.tick()
	_sync_local(delta)
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
	AppState.get_services().presence.screen_opened(screen_id)
	var panel: Control = panels.get(screen_id) as Control
	if panel == null:
		panel = (load(path) as PackedScene).instantiate() as Control
		panel.connect("closed", _on_panel_closed.bind(screen_id))
		if panel.has_signal("travel_requested"):
			panel.connect("travel_requested", _on_travel_requested)
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
	AppState.get_services().presence.screen_closed(screen_id)
	EventBus.ui_closed.emit(screen_id)
	if activity_return.on_closed(screen_id, zone.area_id) and zone.player != null:
		zone.player.global_position = activity_return.position # F071-AC-20: back next to the NPC, nothing chained


func _start_quality() -> void:
	var config: ClientConfig = AuthService.version_check.config if AuthService.version_check != null else null
	if config != null and not config.device_profiles.is_empty():
		QualityManager.apply_device_profiles(config.device_profiles)
	if not QualityManager.needs_benchmark() or zone.player == null:
		_benchmark.hide()
		_maybe_start_onboarding()
		return
	_set_player_input(false)
	_hud.visible = false
	AppState.get_services().play_time.hold(BENCHMARK_HOLD)
	_benchmark.call("start", zone.player.get("camera_rig"))


func _on_benchmark_finished(_preset: String) -> void:
	_hud.visible = true
	_set_player_input(true)
	_maybe_start_onboarding()


## F096-AC-10: Mây's guide on the first entry on this device, or right after S02 (character creation) in this session.
func _maybe_start_onboarding() -> void:
	if OnboardingGuide.should_show(Settings.onboarding_hub_seen, AuthService.just_created_character):
		AuthService.just_created_character = false
		_hud.call("start_onboarding")


## F096-AC-10: "Xem lại hướng dẫn của Mây" (Cài đặt).
func _on_onboarding_requested() -> void:
	_hud.call("start_onboarding")


## F096-AC-11: the "Việc tiếp theo" card opens the screen it suggests.
func _on_next_step_pressed(screen_id: StringName, args: Dictionary) -> void:
	EventBus.ui_requested.emit(screen_id, args)
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


## F064-AC-22: 🔔 or a toast → badge cleared, "Nhà trường giao" tab (which refreshes the list).
func _on_notifications_pressed() -> void:
	AppState.get_services().notifications.open_assigned()


## F070 Tủ đồ: HUD button or `open_wardrobe` (I).
func _on_wardrobe_pressed() -> void:
	EventBus.ui_requested.emit(&"wardrobe", {})


func _input(event: InputEvent) -> void:
	var player_input: bool = event is InputEventKey or event is InputEventMouseButton \
			or event is InputEventScreenTouch or event is InputEventJoypadButton
	if player_input:
		AppState.get_services().presence.touch()


func _unhandled_input(event: InputEvent) -> void:
	if open_screen.is_empty() and event.is_action_pressed("open_wardrobe"):
		_on_wardrobe_pressed()
		get_viewport().set_input_as_handled()
	elif open_screen.is_empty() and event.is_action_pressed("open_world_map"):
		_on_map_pressed()
		get_viewport().set_input_as_handled()


## Draws the local avatar from the profile (fallback visuals for missing items, F070-AC-18) and fills the emote
## wheel with the equipped emotes (F070-AC-20).
func _apply_avatar(config: AvatarConfigModel) -> void:
	var local: PlayerController = zone.player as PlayerController if zone != null else null
	var rig: AvatarBuilder = local.avatar as AvatarBuilder if local != null else null
	if rig != null and config != null:
		rig.apply(config, AppState.get_services().inventory.catalog)
	_emotes.catalog = AppState.get_services().inventory.catalog
	_emotes.set_emotes(config.emote_slots if config != null else PackedStringArray())


func _on_avatar_saved(config: AvatarConfigModel) -> void:
	_apply_avatar(config)


# ---------- F092 hub multiplayer ----------

func _start_hub(services: ServiceLocator) -> void:
	hub = services.hub
	remote_players = RemotePlayerSync.new()
	remote_players.name = "RemotePlayers"
	zone.add_child(remote_players)
	remote_players.viewer = zone.player
	remote_players.catalog_source = func() -> CharacterCatalogModel: return services.inventory.catalog
	if hub == null:
		return
	remote_players.bind(hub)
	hub.room_assigned.connect(_on_room_assigned)
	hub.position_corrected.connect(_on_position_corrected)
	hub.join_failed.connect(_on_join_failed)
	hub.emote_failed.connect(_on_emote_failed)
	hub.enter_area(zone.area_id, AppState.spawn_point_id)


func _stop_hub() -> void:
	if hub == null:
		return
	for pair: Array in [[hub.room_assigned, _on_room_assigned], [hub.position_corrected, _on_position_corrected],
			[hub.join_failed, _on_join_failed], [hub.emote_failed, _on_emote_failed]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	if remote_players != null:
		remote_players.unbind()
	hub.leave_area()
	hub = null


## Local state to the service (it throttles and quantises) and the server correction blend (F092-AC-11, AC-14).
func _sync_local(delta: float) -> void:
	var local: PlayerController = zone.player as PlayerController if zone != null else null
	if local == null or hub == null:
		return
	if correction.is_active():
		local.global_position += correction.step(delta)
	var yaw: float = local.avatar.rotation.y if local.avatar != null else 0.0
	var anim: String = ""
	if local.anim_driver != null:
		anim = String(local.anim_driver.state)
		if local.anim_driver.state == AnimDriver.EMOTE:
			anim = "emote:%s" % local.anim_driver.emote_id
	hub.update_local(local.global_position, yaw, anim, Time.get_ticks_msec())


## F092-AC-10: the server placed this player in the instance (spawn of the room).
func _on_room_assigned(room: HubRoomState) -> void:
	var local: PlayerController = zone.player as PlayerController if zone != null else null
	if local != null:
		local.global_position = room.spawn
		local.velocity = Vector3.ZERO


func _on_position_corrected(position: Vector3) -> void:
	var local: PlayerController = zone.player as PlayerController if zone != null else null
	if local != null:
		correction.start(local.global_position, position)


func _on_join_failed(code: String) -> void:
	_hud.call("show_hub_notice", HubSync.join_notice_key(code))


func _on_emote_failed(code: String) -> void:
	_hud.call("show_hub_notice", HubSync.emote_notice_key(code))


## F092-AC-16: the emote already played locally (EmoteWheel); the others see it through `hub.player.emote`.
func _on_emote_selected(emote_id: String) -> void:
	if hub != null and hub.room.is_joined():
		hub.emote(emote_id, Time.get_ticks_msec())



func _on_joystick_moved(vector: Vector2) -> void:
	if zone.player != null:
		zone.player.call("set_move_input", vector)


func _on_focus_changed(interactable_id: String, prompt_key: String) -> void:
	_hud.call("set_context_action", interactable_id, prompt_key)


func _on_break_reminder_shown(_reminder_no: int, minutes_played: int) -> void:
	_break_dialog.call("open", minutes_played)


func _on_curfew_warning_raised(minutes_left: int) -> void:
	_hud.call("show_banner", "ui.login.curfew_warning", {"minutes": minutes_left})


# ---------- F071 world: zones, portals, NPCs, discoveries ----------

func _bind_zone(z: ZoneRoot) -> void:
	z.interaction_focus_changed.connect(_on_focus_changed)
	z.portal_requested.connect(_on_portal_requested)
	z.npc_requested.connect(_on_npc_requested)
	z.discovery_requested.connect(_on_discovery_requested)
	z.quest_object_requested.connect(_on_quest_object_requested)
	_bind_quests()


## "Về Quảng trường" (≤ 1 action from any area, WORLD_DESIGN §6): the hub spawn, or a zone change back to main_hub.
func _on_home_pressed() -> void:
	if zone.area_id == HUB_AREA_ID:
		zone.return_home()
	else:
		_request_area(HUB_AREA_ID, "")


func _on_map_pressed() -> void:
	EventBus.ui_requested.emit(&"world_map", {})


func _on_portal_requested(portal_id: String, target_area_id: String) -> void:
	_request_area(target_area_id, portal_id)


func _on_travel_requested(area_id: String) -> void:
	_request_area(area_id, "")


func _request_area(area_id: String, portal_id: String) -> void:
	if area_id.is_empty() or SceneRouter.is_changing_area():
		return
	_area_started_ms = Time.get_ticks_msec()
	_set_player_input(false)
	await SceneRouter.request_area(area_id, portal_id)


## The server accepted the area: swap the zone scene, place the player, join the hub of the new area (F071-AC-12).
func _on_area_entered(area_id: String, scene_ref: String, _spawn_point_id: String, _enter: Dictionary) -> void:
	var next: ZoneRoot = ZoneLoader.instantiate(scene_ref)
	if next == null:
		_on_area_change_failed(area_id, SceneRouter.CODE_BUNDLE_MISSING, [])
		return
	_stop_hub()
	var parent: Node = zone.get_parent()
	var index: int = zone.get_index()
	parent.remove_child(zone)
	zone.queue_free()
	next.name = "Zone"
	parent.add_child(next)
	parent.move_child(next, index)
	zone = next
	_bind_zone(zone)
	_set_player_input(true)
	var services: ServiceLocator = AppState.get_services()
	_apply_avatar(services.profile.avatar_config())
	_emotes.anim_driver = (zone.player as PlayerController).anim_driver if zone.player is PlayerController else null
	_start_hub(services)
	ZoneLoader.check_budget(area_id, Time.get_ticks_msec() - _area_started_ms)


func _on_area_change_failed(_area_id: String, code: String, missing: Array) -> void:
	_set_player_input(true)
	if code == WorldService.CODE_AREA_LOCKED:
		var words: PackedStringArray = []
		for r: Variant in missing:
			if r is Dictionary:
				words.append(WorldMapModel.requirement_text(r as Dictionary))
		_hud.call("show_banner", "ui.world.area_locked", {"requirements": ", ".join(words)})
	else:
		_notice.call("open", "ui.world.area_unavailable")


## F072: the Quest Tracker on the HUD and the markers of the targets of the tracked quest follow QuestService.
func _bind_quests() -> void:
	var quests: QuestService = AppState.get_services().quest
	if quests == null:
		return
	if _hud.get_node_or_null(^"QuestTracker") == null:
		var tracker: Control = QUEST_TRACKER_SCENE.instantiate() as Control
		tracker.name = "QuestTracker"
		_hud.add_child(tracker)
	if not quests.progress_updated.is_connected(_on_quest_progress):
		quests.progress_updated.connect(_on_quest_progress)
		quests.tracked_changed.connect(_on_quest_progress)
		quests.notice_requested.connect(_on_quest_notice)
	_on_quest_progress(quests.tracked_quest_id)
	quests.refresh()


func _on_quest_progress(_quest_id: String) -> void:
	var quests: QuestService = AppState.get_services().quest
	var q: QuestModel = quests.quest(quests.tracked_quest_id) if quests != null else null
	var ids: PackedStringArray = []
	if q != null and q.is_active():
		for s: Dictionary in q.active_steps():
			for id: Variant in (s.get("interactable_ids", []) as Array if s.get("interactable_ids") is Array else []):
				ids.append(String(id))
	if zone != null:
		zone.mark_quest_targets(ids)


func _on_quest_notice(key: String) -> void:
	_hud.call("show_banner", key, {})


## F072: a quest object reports the active quest step it belongs to (the server checks presence, area and ids).
func _on_quest_object_requested(interactable_id: String) -> void:
	var quests: QuestService = AppState.get_services().quest
	if quests != null:
		await quests.on_interaction(zone.area_id, interactable_id)


func _on_npc_requested(npc_id: String, _interactable_id: String) -> void:
	var quests: QuestService = AppState.get_services().quest
	if quests != null:
		await quests.on_interaction(zone.area_id, "", npc_id) # talk_npc of an active quest; the dialogue opens anyway
	if zone.player != null:
		activity_return.remember(zone.area_id, zone.player.global_position)
	EventBus.ui_requested.emit(&"npc_dialogue", {"npc_id": npc_id})


func _on_discovery_requested(interactable_id: String) -> void:
	var world: WorldService = AppState.get_services().world
	var result: ApiResult = await world.report_discovery(zone.area_id, interactable_id)
	if result.ok:
		_hud.call("show_banner", "ui.world.discovery_found", {"pct": int(round(float(result.body().get("area_completion_pct", 0.0))))})
	elif result.code == WorldService.CODE_ALREADY_FOUND:
		_hud.call("show_banner", "ui.world.discovery_already_found", {})


func _on_area_unlocked(_area_id: String, area_name: Dictionary) -> void:
	_hud.call("show_banner", "ui.world.area_unlocked", {"name": L10n.pick(area_name)})
