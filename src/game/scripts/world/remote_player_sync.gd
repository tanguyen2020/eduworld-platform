class_name RemotePlayerSync
extends Node3D
## The other players of the hub instance (F092-AC-12, AC-13, AC-15, AC-16, AC-21, AC-23): renders HubSync signals —
## one pooled RemotePlayer per entity (at most `hub_instance_capacity`, reused after the 0.3 s fade-out so a long
## session with players coming and going allocates nothing new), the interpolated state every frame (frozen while
## the connection is not Online, F092-AC-08), emotes and appearance changes. Level of detail by `quality_presets`
## (PERFORMANCE_BUDGET §5): the N closest players are drawn in full, the rest as impostors; players beyond
## ANIM_LOD_DISTANCE animate at a lower rate. Glue only: no rule about who may be shown — only players the server
## sent for this room exist (F092-AC-19).

## CONFIG_DEFAULTS `hub_instance_capacity` (WORLD_DESIGN, ADR-0012).
const CAPACITY: int = 40
## PERFORMANCE_BUDGET §13 / GAME_NETWORKING §5.2: animation LOD beyond 30 m.
const ANIM_LOD_DISTANCE: float = 30.0
const REMOTE_SCENE: String = "res://scenes/character/RemotePlayer.tscn"

var hub: HubSync = null
## `() -> CharacterCatalogModel` (avatar fallbacks of AvatarBuilder); null → builder defaults.
var catalog_source: Callable = Callable()
## Reference point of the LOD (the local player); null → this node's origin.
var viewer: Node3D = null
var quality_preset: String = QualityPolicy.MEDIUM
var is_pc: bool = true
## eid → RemotePlayer currently shown (fading ones included until they are back in the pool).
var active: Dictionary = {}
var pool: Array[RemotePlayer] = []
## RemotePlayer nodes ever created (AC-23: stays ≤ CAPACITY however many players come and go).
var created: int = 0
var _scene: PackedScene = null


func _ready() -> void:
	_scene = load(REMOTE_SCENE) as PackedScene
	QualityManager.quality_changed.connect(_on_quality_changed)
	quality_preset = QualityManager.preset
	is_pc = not (OS.has_feature("mobile") or OS.has_feature("web"))


func _exit_tree() -> void:
	if QualityManager.quality_changed.is_connected(_on_quality_changed):
		QualityManager.quality_changed.disconnect(_on_quality_changed)
	unbind()


func bind(service: HubSync) -> void:
	unbind()
	hub = service
	if hub == null:
		return
	hub.remote_player_joined.connect(_on_joined)
	hub.remote_player_left.connect(_on_left)
	hub.remote_player_emoted.connect(_on_emoted)
	hub.remote_appearance_changed.connect(_on_appearance_changed)
	for eid: Variant in hub.room.players.keys():
		_on_joined(int(eid), hub.room.players[eid] as PublicPlayerModel, null)


func unbind() -> void:
	if hub == null:
		return
	for pair: Array in [[hub.remote_player_joined, _on_joined], [hub.remote_player_left, _on_left],
			[hub.remote_player_emoted, _on_emoted], [hub.remote_appearance_changed, _on_appearance_changed]]:
		var sig: Signal = pair[0] as Signal
		if sig.is_connected(pair[1] as Callable):
			sig.disconnect(pair[1] as Callable)
	hub = null


func player_for(eid: int) -> RemotePlayer:
	return active.get(eid) as RemotePlayer


func shown_count() -> int:
	var n: int = 0
	for rp: Variant in active.values():
		if not (rp as RemotePlayer).fading:
			n += 1
	return n


func _process(_delta: float) -> void:
	if hub == null or active.is_empty():
		return
	if not hub.is_frozen():
		var now: int = hub.realtime.server_now_ms()
		for eid: Variant in active.keys():
			var rp: RemotePlayer = active[eid] as RemotePlayer
			if not rp.fading:
				rp.apply_state(hub.remote_state(int(eid), now))
	update_lod()


## Assigns FULL / FAR / IMPOSTOR: the closest `full_characters` players are full (FAR beyond 30 m), the rest impostors.
func update_lod() -> void:
	var origin: Vector3 = viewer.global_position if viewer != null and viewer.is_inside_tree() else global_position
	var list: Array[RemotePlayer] = []
	for rp: Variant in active.values():
		if not (rp as RemotePlayer).fading:
			list.append(rp as RemotePlayer)
	list.sort_custom(func(a: RemotePlayer, b: RemotePlayer) -> bool:
		return a.position.distance_squared_to(origin) < b.position.distance_squared_to(origin))
	var full: int = QualityPolicy.full_characters(quality_preset, is_pc)
	for i: int in list.size():
		var rp: RemotePlayer = list[i]
		if i >= full:
			rp.set_lod(RemotePlayer.Lod.IMPOSTOR)
		elif rp.position.distance_to(origin) > ANIM_LOD_DISTANCE:
			rp.set_lod(RemotePlayer.Lod.FAR)
		else:
			rp.set_lod(RemotePlayer.Lod.FULL)


func _on_joined(eid: int, player: PublicPlayerModel, state: SnapshotSample) -> void:
	var rp: RemotePlayer = active.get(eid) as RemotePlayer
	if rp == null:
		rp = _take()
		if rp == null:
			Log.warn("remote_player_capacity", {"capacity": CAPACITY})
			return
		active[eid] = rp
	rp.setup(eid, player, _catalog())
	if state != null:
		rp.position = state.p
		rp.avatar.rotation.y = state.yaw


func _on_left(eid: int) -> void:
	var rp: RemotePlayer = active.get(eid) as RemotePlayer
	if rp != null and not rp.fading:
		rp.fade_out()


func _on_emoted(eid: int, emote_id: String) -> void:
	var rp: RemotePlayer = active.get(eid) as RemotePlayer
	if rp != null and not rp.fading:
		rp.play_emote(emote_id)


func _on_appearance_changed(eid: int, player: PublicPlayerModel) -> void:
	var rp: RemotePlayer = active.get(eid) as RemotePlayer
	if rp != null:
		rp.player = player
		rp.apply_avatar(_catalog())


func _on_fade_completed(eid: int) -> void:
	var rp: RemotePlayer = active.get(eid) as RemotePlayer
	if rp == null or rp.visible:
		return
	active.erase(eid)
	rp.eid = -1
	rp.player = null
	pool.append(rp)


## A pooled RemotePlayer, or a new one while fewer than CAPACITY exist.
func _take() -> RemotePlayer:
	if not pool.is_empty():
		return pool.pop_back()
	if created >= CAPACITY or _scene == null:
		return null
	var rp: RemotePlayer = _scene.instantiate() as RemotePlayer
	add_child(rp)
	rp.fade_completed.connect(_on_fade_completed)
	created += 1
	return rp


func _catalog() -> CharacterCatalogModel:
	return catalog_source.call() as CharacterCatalogModel if catalog_source.is_valid() else null


func _on_quality_changed(preset: String) -> void:
	quality_preset = preset
