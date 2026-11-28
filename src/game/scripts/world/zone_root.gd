class_name ZoneRoot
extends Node3D
## Root of a zone scene (WORLD_RENDERING §2; F062 Main Hub greybox). Declares the zone metadata, places the local
## player at the spawn point chosen by the server (fallback: `default_spawn`, F062-AC-01), tracks which Interactable
## the player stands in and turns `interacted` into a UI intent (`EventBus.ui_requested`) — no learning logic here
## (AGENTS R1, G-4). Entering the zone and closing a panel refresh the school assignments through the service layer
## (F062-AC-07); the zone registers itself with QualityManager (dynamic resolution, suspend, shadows, art quality — F067).

signal interaction_focus_changed(interactable_id: String, prompt_key: String)
signal player_spawned(spawn_point_id: String)
## F071: intents of the world anchors, handled by WorldRoot through the services (the zone decides nothing).
signal portal_requested(portal_id: String, target_area_id: String)
signal npc_requested(npc_id: String, interactable_id: String)
signal discovery_requested(interactable_id: String)
## A quest object of the layout (F072): the quest service reports it for the active step it belongs to.
signal quest_object_requested(interactable_id: String)

## Screen opened by each Interactable kind (data, not logic: the screen decides what to show).
## `exam_board`: the Exam Zone greybox entry of the Main Hub (F064 T015; the full `exam_zone` scene is F071).
const QUEST_MARKER_SCENE: PackedScene = preload("res://scenes/quest/QuestMarker3D.tscn")
## F111: `event_board` — the event board of Quảng trường Tia Sáng opens the Event center on its "Sự kiện" tab.
const KIND_SCREENS: Dictionary = {"quest_board": &"quest_journal", "exam_board": &"exam_lobby", "event_board": &"event_center"}
## Arguments per interactable id (the school assignments board opens the "Nhà trường giao" tab).
const INTERACTABLE_ARGS: Dictionary = {"board.school_assignments": {"tab": "assigned"}, "board.events": {"tab": "events"}}
const SCREEN_QUEST_JOURNAL: StringName = &"quest_journal"
## Falling off the greybox below this height puts the player back on the default spawn.
const KILL_Y: float = -30.0

@export var area_id: String = "main_hub"
@export var default_spawn: String = "main_hub.default"
@export var music_id: String = ""
@export var player_scene: PackedScene = null
## Off for the benchmark/preview instances: no player, no service calls.
@export var interactive: bool = true

var player: CharacterBody3D = null
var focused: Interactable = null
var _inside: Array[Interactable] = []


func _ready() -> void:
	if interactive:
		AppState.current_zone = area_id
		if player_scene != null:
			player = player_scene.instantiate() as CharacterBody3D
			get_node("Players").add_child(player)
			place_player(AppState.spawn_point_id)
		for node: Node in find_children("*", "Area3D", true, false):
			if node is Interactable:
				_connect_interactable(node as Interactable)
		EventBus.ui_closed.connect(_on_ui_closed)
		EventBus.zone_changed.emit(area_id)
		refresh_assignments()
	# F097 (F096-4): markers near the camera are hidden so they never cover the player.
	add_child(MarkerFader.new())
	QualityManager.register_world(self)
	QualityManager.quality_changed.connect(_on_quality_changed)
	_on_quality_changed(QualityManager.preset)


func _exit_tree() -> void:
	QualityManager.unregister_world(self)
	if QualityManager.quality_changed.is_connected(_on_quality_changed):
		QualityManager.quality_changed.disconnect(_on_quality_changed)
	if EventBus.ui_closed.is_connected(_on_ui_closed):
		EventBus.ui_closed.disconnect(_on_ui_closed)


func spawn_points() -> Array[SpawnPoint]:
	var out: Array[SpawnPoint] = []
	for node: Node in get_node("SpawnPoints").get_children():
		if node is SpawnPoint:
			out.append(node as SpawnPoint)
	return out


## F062-AC-01: the spawn point with `id`, else `default_spawn` (warning logged when `id` is unknown).
func find_spawn(id: String) -> SpawnPoint:
	var fallback: SpawnPoint = null
	for point: SpawnPoint in spawn_points():
		if not id.is_empty() and point.spawn_point_id == id:
			return point
		if point.spawn_point_id == default_spawn:
			fallback = point
	if not id.is_empty():
		Log.warn("spawn_point_missing", {"area_id": area_id, "spawn_point_id": id, "fallback": default_spawn})
	return fallback


func place_player(id: String) -> SpawnPoint:
	var point: SpawnPoint = find_spawn(id)
	if player == null or point == null:
		return point
	player.global_position = point.pick_position()
	player.velocity = Vector3.ZERO
	player.call("face", deg_to_rad(point.facing))
	player_spawned.emit(point.spawn_point_id)
	return point


## "Về Quảng trường" (S03 🏠): back to the default spawn of the Hub.
func return_home() -> void:
	place_player(default_spawn)


## The `interact` action / ContextActionButton: activates the Interactable the player stands in.
func interact_focused() -> bool:
	if focused == null:
		return false
	focused.interact()
	return true


func request_screen(screen_id: StringName, args: Dictionary = {}) -> void:
	EventBus.ui_requested.emit(screen_id, args)


## F062-AC-07: entering/returning to the Hub reloads "Nhà trường giao" (signed-in sessions only).
func refresh_assignments() -> void:
	if AppState.player.is_empty():
		return
	AppState.get_services().assignments.refresh()


func _physics_process(_delta: float) -> void:
	if player != null and player.global_position.y < KILL_Y:
		return_home()


func _unhandled_input(event: InputEvent) -> void:
	if not interactive:
		return
	if event.is_action_pressed("interact") and interact_focused():
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("open_journal"):
		request_screen(SCREEN_QUEST_JOURNAL, {"tab": "assigned"})
		get_viewport().set_input_as_handled()


func _connect_interactable(item: Interactable) -> void:
	item.interacted.connect(_on_interacted.bind(item))
	item.player_entered.connect(_on_player_entered.bind(item))
	item.player_exited.connect(_on_player_exited.bind(item))


func _on_player_entered(_id: String, item: Interactable) -> void:
	if not item in _inside:
		_inside.append(item)
	_set_focus(item)


func _on_player_exited(_id: String, item: Interactable) -> void:
	_inside.erase(item)
	_set_focus(_inside.back() if not _inside.is_empty() else null)


func _set_focus(item: Interactable) -> void:
	focused = item
	interaction_focus_changed.emit(item.interactable_id if item != null else "", item.prompt_key if item != null else "")


func _on_interacted(interactable_id: String, item: Interactable) -> void:
	match item.kind:
		"portal":
			portal_requested.emit(interactable_id, String(item.get("target_area_id")) if item is AreaPortal else "")
			return
		"npc":
			var npc_id: String = (item as NpcAnchor).npc_id if item is NpcAnchor else interactable_id
			npc_requested.emit(npc_id, interactable_id)
			return
		"discovery", "collectible":
			discovery_requested.emit(interactable_id)
			return
		"quest_object":
			quest_object_requested.emit(interactable_id)
			return
	var screen: StringName = KIND_SCREENS.get(item.kind, &"") as StringName
	if screen.is_empty():
		Log.debug("interactable_without_screen", {"interactable_id": interactable_id})
		return
	request_screen(screen, (INTERACTABLE_ARGS.get(interactable_id, {}) as Dictionary).duplicate())


func _on_ui_closed(_screen_id: StringName) -> void:
	refresh_assignments()


func _on_quality_changed(_preset: String) -> void:
	var light: DirectionalLight3D = get_node_or_null("Environment/Sun") as DirectionalLight3D
	if light != null:
		light.light_color = Art3dTokens.LIGHT_SUN_COLOR
		light.light_energy = Art3dTokens.LIGHT_SUN_ENERGY
		light.shadow_enabled = QualityManager.shadows_enabled() \
				and QualityPolicy.shadows_supported(RenderingServer.get_current_rendering_method())
	ZoneArtQuality.apply(self, QualityManager.preset)


## F072: puts a QuestMarker3D above every interactable whose id is in ids (targets of the active steps of the tracked
## quest, from the server delivery) and removes the others.
func mark_quest_targets(ids: PackedStringArray) -> void:
	for node: Node in find_children("*", "Interactable", true, false):
		var item: Interactable = node as Interactable
		var marker: Node = item.get_node_or_null(^"QuestMarker3D")
		var wanted: bool = item.interactable_id in ids
		if wanted and marker == null:
			var m: Node3D = QUEST_MARKER_SCENE.instantiate() as Node3D
			m.name = "QuestMarker3D"
			item.add_child(m)
		elif not wanted and marker != null:
			marker.queue_free()
