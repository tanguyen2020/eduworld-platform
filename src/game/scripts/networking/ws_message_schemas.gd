class_name WsMessageSchemas
extends RefCounted
## Payload checks of the server messages this client consumes (AsyncAPI specs/asyncapi/eduworld-realtime.v1.yaml;
## F092-AC-06, GAME_NETWORKING §9 "JSON → model typed; thiếu trường → bỏ message"). RealtimeClient drops a message
## whose payload breaks its entry before any service sees it. Field kinds: `s<N>` string ≤ N, `int`, `num`, `bool`,
## `obj`, `arr`, `vec3` (array of 3 numbers); a leading `?` marks an optional field. Types not listed here pass
## (forward compatible) and go only to a registered handler; nested entries are checked by the models.

const SCHEMAS: Dictionary = {
	"system.session.welcome": {
		"conn_id": "s64", "server_time": "int", "heartbeat_s": "int", "timeout_s": "int", "resume_token": "s512",
		"protocol_version": "int", "?resume_window_s": "int", "?limits": "obj", "?user_id": "s64",
		"?game_session_id": "s64",
	},
	"system.heartbeat.pong": {"?client_time": "int", "?server_time": "int"},
	"system.command.acked": {"?result": "obj"},
	"system.command.failed": {"code": "s64", "message": "s1000", "?retry_after_ms": "int", "?details": "obj"},
	"system.session.resumed": {"?replayed": "int", "?rejoined_rooms": "arr"},
	"system.session.resume_failed": {"?reason": "s32"},
	"system.service.degraded": {"?features": "arr", "?poll_interval_s": "int"},
	"system.server.draining": {"reconnect_after_ms": "int", "?reason": "s32"},
	"system.notice.broadcasted": {"level": "s32", "?message_code": "s128", "?message": "obj"},
	"session.session.replaced": {"game_session_id": "s64", "replaced_at": "int", "?new_device_platform": "s32"},
	"session.session.ended": {"game_session_id": "s64", "reason": "s32", "ended_at": "int", "?reopen_at": "int"},
	"hub.instance.assigned": {
		"room_id": "s64", "area_id": "s64", "self_eid": "int", "spawn": "vec3", "tick_rate_hz": "int",
		"?max_speed_mps": "num", "?players": "arr", "?scene_ref": "s255",
	},
	"hub.snapshot.updated": {"room_id": "s64", "tick": "int", "entities": "arr", "?removed": "arr", "?ack_client_seq": "int"},
	"hub.player.joined": {"room_id": "s64", "eid": "int", "player": "obj", "?state": "obj"},
	"hub.player.left": {"room_id": "s64", "eid": "int"},
	"hub.player.emoted": {"room_id": "s64", "eid": "int", "emote_id": "s64"},
	"hub.player.appearance_changed": {"room_id": "s64", "eid": "int", "appearance": "obj"},
	"hub.player.corrected": {"room_id": "s64", "client_seq": "int", "p": "vec3", "?reason": "s32"},
	"hub.instance.migrated": {"old_room_id": "s64", "room_id": "s64", "?reason": "s32"},
}


static func is_known(type: String) -> bool:
	return SCHEMAS.has(type)


## True when `payload` satisfies the entry of `type` (or the type has no entry).
static func validate(type: String, payload: Dictionary) -> bool:
	var schema: Dictionary = SCHEMAS.get(type, {}) as Dictionary
	for raw_key: Variant in schema.keys():
		var key: String = String(raw_key)
		var optional: bool = key.begins_with("?")
		var field: String = key.trim_prefix("?")
		if not payload.has(field):
			if optional:
				continue
			return false
		if not check_value(payload[field], String(schema[raw_key])):
			return false
	return true


static func check_value(value: Variant, kind: String) -> bool:
	if kind.begins_with("s"):
		return value is String and (value as String).length() <= int(kind.substr(1))
	match kind:
		"int":
			return WsEnvelope.is_int(value)
		"num":
			return (value is float or value is int) and is_finite(float(value))
		"bool":
			return value is bool
		"obj":
			return value is Dictionary
		"arr":
			return value is Array
		"vec3":
			return is_vec3(value)
	return false


static func is_vec3(value: Variant) -> bool:
	if not value is Array or (value as Array).size() != 3:
		return false
	for n: Variant in value as Array:
		if not (n is float or n is int) or not is_finite(float(n)):
			return false
	return true


static func to_vec3(value: Variant) -> Vector3:
	var a: Array = value as Array
	return Vector3(float(a[0]), float(a[1]), float(a[2]))
