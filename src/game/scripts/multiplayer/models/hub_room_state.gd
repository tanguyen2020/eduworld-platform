class_name HubRoomState
extends RefCounted
## The hub instance this client is in (AsyncAPI `hub.instance.assigned`, `hub.player.joined/left`,
## `hub.instance.migrated`; F092 data-model §1). Only the players the server sent for this room are known
## (F092-AC-19): there is no other source of players.

var room_id: String = ""
var area_id: String = ""
var self_eid: int = -1
var tick_rate_hz: int = 10
var max_speed_mps: float = 0.0
var spawn: Vector3 = Vector3.ZERO
## eid → PublicPlayerModel (never contains self_eid).
var players: Dictionary = {}


func is_joined() -> bool:
	return not room_id.is_empty()


func clear() -> void:
	room_id = ""
	area_id = ""
	self_eid = -1
	players.clear()


## Applies a validated `hub.instance.assigned`; returns the initial players as `[eid, PublicPlayerModel, state]`
## (state: SnapshotSample or null). Entries with a bad player or eid are skipped.
func apply_assigned(payload: Dictionary, at_ms: int) -> Array:
	room_id = String(payload["room_id"])
	area_id = String(payload["area_id"])
	self_eid = int(payload["self_eid"])
	tick_rate_hz = clampi(WsEnvelope.as_int(payload.get("tick_rate_hz"), 10), 1, 60)
	max_speed_mps = float(payload.get("max_speed_mps", 0.0)) if payload.get("max_speed_mps") is float \
			or payload.get("max_speed_mps") is int else 0.0
	spawn = WsMessageSchemas.to_vec3(payload["spawn"])
	players.clear()
	var out: Array = []
	for entry: Variant in payload.get("players", []) as Array:
		if not entry is Dictionary:
			continue
		var e: Dictionary = entry as Dictionary
		if not WsEnvelope.is_int(e.get("eid")) or int(e["eid"]) == self_eid:
			continue
		var player: PublicPlayerModel = PublicPlayerModel.from_dict(e.get("player"))
		if player == null:
			continue
		var eid: int = int(e["eid"])
		var state: SnapshotSample = null
		if e.get("state") is Dictionary:
			var ignored: Array = []
			state = SnapshotSample.from_entity(e["state"], 0, at_ms, ignored)
		players[eid] = player
		out.append([eid, player, state])
	return out
