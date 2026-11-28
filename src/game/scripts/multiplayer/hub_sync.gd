class_name HubSync
extends RefCounted
## Hub multiplayer service (GAME_NETWORKING §5, REALTIME_ARCHITECTURE §5.1; F092 US2). Created by the ServiceLocator
## with the RealtimeClient; the world scene only calls its intents and renders its signals (AGENTS R1):
## - `enter_area(area_id, spawn_point)` → `hub.instance.join` now and again whenever a fresh connection comes up;
##   `hub.instance.assigned` → spawn, `self_eid`, the players the server listed (F092-AC-10, AC-19);
## - `update_local()` → `hub.player.move` at most `client_input_rate` (and the server `max_move_hz`), only after
##   Δposition ≥ 2 cm, Δyaw ≥ 2° or a new animation, 1 Hz keepalive when standing; position quantised to 1 cm, yaw to
##   1° (F092-AC-11). The local player moves at once; the server stays authoritative (`hub.player.corrected`, R5);
## - snapshots → SnapshotBuffer + Interpolator (F092-AC-12); joined/left/removed, emotes, appearance (AC-13/15/16);
## - `hub.instance.migrated` → new room id without reloading the scene; join errors → `join_failed(code)` (AC-17).
## Other players carry only nickname and avatar (PublicPlayerModel, R16).
## F111 "Tham gia bạn": `enter_area(area_id, spawn_point, follow_user_id)` (after a zone change) or `follow(user_id)` (same
## area) sends ONE `hub.instance.join` with `follow_user_id` (SOCIAL_SYSTEM §4.2, F091-AC-05) — the next join of the area
## is ordinary again; a refused follow (ROOM_FULL, PERMISSION_DENIED, NOT_FOUND) is `follow_failed(code)`, never the
## generic hub notice (JoinFriendService explains it without telling where the friend is, R16).

signal room_assigned(room: HubRoomState)
signal remote_player_joined(eid: int, player: PublicPlayerModel, state: SnapshotSample)
signal remote_player_left(eid: int)
signal remote_appearance_changed(eid: int, player: PublicPlayerModel)
signal remote_player_emoted(eid: int, emote_id: String)
signal position_corrected(position: Vector3)
signal room_migrated(room_id: String)
signal join_failed(code: String)
## F111: the server refused the join next to a friend (`hub.instance.join` with `follow_user_id`).
signal follow_failed(code: String)
signal emote_failed(code: String)

const PREFIX: String = "hub"
const TYPE_JOIN: String = "hub.instance.join"
const TYPE_MOVE: String = "hub.player.move"
const TYPE_EMOTE: String = "hub.player.emote"
## CONFIG_DEFAULTS `client_input_rate` (≤ 10 Hz).
const MAX_INPUT_HZ: int = 10
## GAME_NETWORKING §5.1: thresholds and keepalive.
const MIN_MOVE_M: float = 0.02
const MIN_TURN_DEG: float = 2.0
const KEEPALIVE_MS: int = 1000
const POSITION_STEP_M: float = 0.01
## CONFIG_DEFAULTS `emote_rate`: one emote per 2 s (client-side guard; the server enforces it too).
const EMOTE_INTERVAL_MS: int = 2000
const CODE_RATE_LIMITED: String = "RATE_LIMITED"
const ID_MAX_LENGTH: int = 64
## Friendly notice per error code (F092-AC-16, AC-17); other codes → ui.error.generic.
const JOIN_NOTICE_KEYS: Dictionary = {
	"ROOM_FULL": "ui.hub.room_full", "PERMISSION_DENIED": "ui.hub.join_denied", "NOT_FOUND": "ui.hub.area_not_found",
}
const EMOTE_NOTICE_KEYS: Dictionary = {
	"RATE_LIMITED": "ui.hub.emote_rate_limited", "PERMISSION_DENIED": "ui.hub.emote_not_owned",
}
const GENERIC_NOTICE_KEY: String = "ui.error.generic"

var realtime: GameRealtimeClient = null
var room: HubRoomState = HubRoomState.new()
var buffer: SnapshotBuffer = SnapshotBuffer.new()
var interpolator: Interpolator = Interpolator.new()
## Area the world is showing (joined again after a fresh connection or a resync).
var desired_area: String = ""
var desired_spawn: String = ""
## F111: friend to join next to on the next `hub.instance.join` only ("" = an ordinary join).
var desired_follow: String = ""
var client_seq: int = 0
## Moves sent (tests and bandwidth estimate).
var moves_sent: int = 0
var _last_sent_ms: int = -1
var _last_p: Vector3 = Vector3.INF
var _last_yaw_deg: float = 0.0
var _last_anim: String = ""
var _last_emote_ms: int = -EMOTE_INTERVAL_MS
var _joining: bool = false
var _fresh: bool = false


func _init(client: GameRealtimeClient) -> void:
	realtime = client
	if realtime != null:
		realtime.register_handler(PREFIX, _on_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


func dispose() -> void:
	if realtime != null:
		realtime.unregister_handler(PREFIX, _on_message)
		if realtime.connection_state_changed.is_connected(_on_connection_state_changed):
			realtime.connection_state_changed.disconnect(_on_connection_state_changed)


## Remote players stop moving while the connection is not Online (F092-AC-08).
func is_frozen() -> bool:
	return realtime == null or not realtime.is_online()


## The world entered `area_id`: joins it (now when Online, else once the connection is up).
## F111: `follow_user_id` (a friend) makes that first join a "Tham gia bạn" one.
func enter_area(area_id: String, spawn_point: String = "", follow_user_id: String = "") -> void:
	if area_id.is_empty() or area_id.length() > ID_MAX_LENGTH or spawn_point.length() > ID_MAX_LENGTH:
		return
	desired_area = area_id
	desired_spawn = spawn_point
	desired_follow = follow_user_id if Uuid.is_valid(follow_user_id) else ""
	if realtime != null and realtime.is_online():
		join()


## F111: joins the instance of friend `user_id` in the area already shown ("Tham gia bạn", same area). False when
## nothing was sent (bad id, no area, offline, a join already on its way); the outcome comes as `room_assigned` or
## `follow_failed`.
func follow(user_id: String) -> bool:
	if not Uuid.is_valid(user_id) or desired_area.is_empty() or _joining or realtime == null or not realtime.is_online():
		return false
	desired_follow = user_id
	join()
	return true


## Leaves the hub view (logout, scene change): forgets the room and the other players.
func leave_area() -> void:
	desired_area = ""
	desired_follow = ""
	_reset_room()


## Coroutine: `hub.instance.join` for the desired area. The room arrives as `hub.instance.assigned`.
func join() -> CommandResult:
	if desired_area.is_empty() or _joining:
		return CommandResult.failure(CommandResult.CODE_INVALID)
	_joining = true
	var payload: Dictionary = {"area_id": desired_area}
	if not desired_spawn.is_empty():
		payload["spawn_point"] = desired_spawn
	# F111: the follow id is spent by this join (a fresh connection / resync later joins the area normally).
	var follow_id: String = desired_follow
	desired_follow = ""
	if not follow_id.is_empty():
		payload["follow_user_id"] = follow_id
	var result: CommandResult = await realtime.send_command(TYPE_JOIN, payload)
	_joining = false
	if not result.ok and result.code != CommandResult.CODE_OFFLINE:
		Log.info("hub_join_failed", {"code": result.code, "area_id": desired_area, "follow": not follow_id.is_empty()})
		if follow_id.is_empty():
			join_failed.emit(result.code)
		else:
			follow_failed.emit(result.code)
	return result


## Local player state for this frame; sends `hub.player.move` when the thresholds and the rate allow. Returns true
## when a frame was sent.
func update_local(position: Vector3, yaw: float, anim: String, now_ms: int) -> bool:
	if not room.is_joined() or realtime == null or not realtime.can_write():
		return false
	var hz: int = mini(MAX_INPUT_HZ, realtime.session.max_move_hz)
	var min_interval: int = int(ceil(1000.0 / float(maxi(hz, 1))))
	if _last_sent_ms >= 0 and now_ms - _last_sent_ms < min_interval:
		return false
	var p: Vector3 = quantize(position)
	var yaw_deg: float = roundf(rad_to_deg(wrapf(yaw, -PI, PI)))
	var moved: bool = _last_p == Vector3.INF or p.distance_to(_last_p) >= MIN_MOVE_M - 0.0001
	var turned: bool = absf(angle_difference(deg_to_rad(yaw_deg), deg_to_rad(_last_yaw_deg))) >= deg_to_rad(MIN_TURN_DEG) - 0.0001
	var changed: bool = moved or turned or anim != _last_anim
	var keepalive: bool = _last_sent_ms < 0 or now_ms - _last_sent_ms >= KEEPALIVE_MS
	if not changed and not keepalive:
		return false
	client_seq += 1
	var payload: Dictionary = {"room_id": room.room_id, "client_seq": client_seq, "p": [p.x, p.y, p.z],
			"r": deg_to_rad(yaw_deg), "a": anim.left(ID_MAX_LENGTH)}
	if not realtime.send_fire_and_forget(TYPE_MOVE, payload):
		return false
	moves_sent += 1
	_last_sent_ms = now_ms
	_last_p = p
	_last_yaw_deg = yaw_deg
	_last_anim = anim
	return true


static func join_notice_key(code: String) -> String:
	return String(JOIN_NOTICE_KEYS.get(code, GENERIC_NOTICE_KEY))


static func emote_notice_key(code: String) -> String:
	return String(EMOTE_NOTICE_KEYS.get(code, GENERIC_NOTICE_KEY))


static func quantize(v: Vector3) -> Vector3:
	return Vector3(snappedf(v.x, POSITION_STEP_M), snappedf(v.y, POSITION_STEP_M), snappedf(v.z, POSITION_STEP_M))


## Coroutine: `hub.player.emote` for an owned emote; the client keeps `emote_rate`. Errors → `emote_failed(code)`.
func emote(emote_id: String, now_ms: int) -> CommandResult:
	if not room.is_joined() or emote_id.is_empty() or emote_id.length() > ID_MAX_LENGTH:
		return CommandResult.failure(CommandResult.CODE_INVALID)
	if now_ms - _last_emote_ms < EMOTE_INTERVAL_MS:
		emote_failed.emit(CODE_RATE_LIMITED)
		return CommandResult.failure(CODE_RATE_LIMITED)
	_last_emote_ms = now_ms
	var result: CommandResult = await realtime.send_command(TYPE_EMOTE, {"room_id": room.room_id, "emote_id": emote_id})
	if not result.ok and result.code != CommandResult.CODE_OFFLINE:
		emote_failed.emit(result.code)
	return result


## Interpolated state of `eid` for this frame (empty when unknown).
func remote_state(eid: int, server_now_ms: int) -> Dictionary:
	return Interpolator.sample_at(buffer.samples(eid), interpolator.render_time(server_now_ms))


# ---------- messages ----------

func _on_message(type: String, payload: Dictionary, env: WsEnvelope) -> void:
	var at_ms: int = env.ts if env.ts > 0 else realtime.server_now_ms()
	match type:
		"hub.instance.assigned":
			_on_assigned(payload, at_ms)
		"hub.instance.migrated":
			if String(payload["old_room_id"]) == room.room_id:
				room.room_id = String(payload["room_id"])
				room_migrated.emit(room.room_id)
		_:
			if String(payload.get("room_id", "")) != room.room_id:
				return
			_on_room_message(type, payload, at_ms)


func _on_room_message(type: String, payload: Dictionary, at_ms: int) -> void:
	match type:
		"hub.snapshot.updated":
			_on_snapshot(payload, at_ms)
		"hub.player.joined":
			var eid: int = int(payload["eid"])
			var player: PublicPlayerModel = PublicPlayerModel.from_dict(payload["player"])
			if player == null or eid == room.self_eid:
				return
			var ignored: Array = []
			var state: SnapshotSample = SnapshotSample.from_entity(payload.get("state"), 0, at_ms, ignored) \
					if payload.get("state") is Dictionary else null
			room.players[eid] = player
			buffer.remove(eid)
			if state != null:
				buffer.push(eid, state)
			remote_player_joined.emit(eid, player, state)
		"hub.player.left":
			_remove(int(payload["eid"]))
		"hub.player.emoted":
			var who: int = int(payload["eid"])
			if who != room.self_eid and room.players.has(who):
				remote_player_emoted.emit(who, String(payload["emote_id"]))
		"hub.player.appearance_changed":
			var target: int = int(payload["eid"])
			var known: PublicPlayerModel = room.players.get(target) as PublicPlayerModel
			if known != null:
				known.apply_appearance(payload["appearance"] as Dictionary)
				remote_appearance_changed.emit(target, known)
		"hub.player.corrected":
			position_corrected.emit(WsMessageSchemas.to_vec3(payload["p"]))


func _on_assigned(payload: Dictionary, at_ms: int) -> void:
	_reset_room()
	var initial: Array = room.apply_assigned(payload, at_ms)
	desired_area = room.area_id
	room_assigned.emit(room)
	for entry: Array in initial:
		var state: SnapshotSample = entry[2] as SnapshotSample
		if state != null:
			buffer.push(int(entry[0]), state)
		remote_player_joined.emit(int(entry[0]), entry[1] as PublicPlayerModel, state)


func _on_snapshot(payload: Dictionary, at_ms: int) -> void:
	interpolator.observe(realtime.server_now_ms(), at_ms)
	var tick_no: int = int(payload["tick"])
	var eid_out: Array = []
	for entity: Variant in payload["entities"] as Array:
		var sample: SnapshotSample = SnapshotSample.from_entity(entity, tick_no, at_ms, eid_out)
		if sample == null:
			continue
		var eid: int = int(eid_out[0])
		if eid == room.self_eid or not room.players.has(eid):
			continue
		buffer.push(eid, sample)
	for removed: Variant in payload.get("removed", []) as Array:
		if WsEnvelope.is_int(removed):
			_remove(int(removed))


func _remove(eid: int) -> void:
	if not room.players.has(eid):
		return
	room.players.erase(eid)
	buffer.remove(eid)
	remote_player_left.emit(eid)


func _reset_room() -> void:
	for eid: Variant in room.players.keys():
		remote_player_left.emit(int(eid))
	room.clear()
	buffer.clear()
	_last_sent_ms = -1
	_last_p = Vector3.INF


## A fresh logical connection (welcome without resume) has no room: join the desired area again once Online. After a
## resume the server restored the room subscriptions itself (`rejoined_rooms`); after a failed resume the resync of
## the ServiceLocator joins (F092-AC-05).
func _on_connection_state_changed(next: int) -> void:
	if next == GameRealtimeClient.State.WELCOMED or next == GameRealtimeClient.State.RESYNC:
		_reset_room()
		_fresh = next == GameRealtimeClient.State.WELCOMED
	elif next == GameRealtimeClient.State.ONLINE and _fresh:
		_fresh = false
		if not desired_area.is_empty():
			join()
