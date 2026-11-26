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
	# F123: the student's own notification (NotificationItemCreated; nested params/action checked by NotificationModel).
	"notification.item.created": {
		"notification_id": "s64", "kind": "s64", "message_code": "s128", "created_at": "int", "?params": "obj",
		"?title": "obj", "?body": "obj", "?ref_type": "s64", "?ref_id": "s64", "?action": "obj",
	},
	# F102: party lobby (members / activity / invitation payloads checked again by the party models).
	"party.state.snapshot": {
		"party_id": "s64", "state_version": "int", "party_type": "s32", "leader_id": "s64", "max_members": "int",
		"members": "arr", "status": "s32", "?name": "s255", "?visibility": "s32", "?current_activity": "obj",
		"?leader_reconnecting": "bool", "?room_id": "s64", "?chat_channel_id": "s64",
	},
	"party.invitation.received": {
		"invitation_id": "s64", "party_id": "s64", "inviter": "obj", "expires_at": "int", "?party_type": "s64",
		"?member_count": "int",
	},
	"party.invitation.updated": {"invitation_id": "s64", "status": "s32", "?party_id": "s64"},
	"party.member.joined": {"party_id": "s64", "state_version": "int", "member": "obj"},
	"party.member.left": {"party_id": "s64", "state_version": "int", "user_id": "s64", "reason": "s32"},
	"party.member.status_changed": {"party_id": "s64", "user_id": "s64", "status": "s32"},
	"party.member.ready_changed": {"party_id": "s64", "user_id": "s64", "ready": "bool", "?all_ready": "bool"},
	"party.leader.transferred": {"party_id": "s64", "from_user_id": "s64", "to_user_id": "s64", "reason": "s32"},
	"party.activity.selected": {
		"party_id": "s64", "activity": "obj", "?game_mode": "s64", "?state_version": "int", "?ineligible_members": "arr",
	},
	"party.session.started": {
		"party_id": "s64", "room_id": "s64", "activity": "obj", "?room_type": "s32", "?party_activity_id": "s64",
		"?scene_ref": "s255", "?member_ids": "arr",
	},
	"party.session.ended": {
		"party_id": "s64", "outcome": "s32", "?party_activity_id": "s64", "?reason": "s32", "?summary": "obj",
	},
	"party.party.disbanded": {"party_id": "s64", "reason": "s32"},
	# F119: gameplay chat (ChatMessage and nested players checked again by ChatMessageModel / ChatStore).
	"chat.message.created": {"message_id": "s64", "channel_id": "s64", "seq": "int", "sender": "obj", "kind": "s16",
		"created_at": "int", "?body": "s500", "?quick_message_id": "s64", "?emoji": "s64", "?params": "obj",
		"?reactions": "obj", "?revision": "int", "?recalled": "bool", "?moderation_state": "s16"},
	"chat.message.acked": {"client_message_id": "s64", "message_id": "s64", "channel_id": "s64", "seq": "int",
		"?moderation_state": "s16"},
	"chat.message.held": {"message_id": "s64", "channel_id": "s64", "?notice": "s1000"},
	"chat.message.edited": {"message_id": "s64", "channel_id": "s64", "seq": "int", "body": "s500", "revision": "int",
		"edited_at": "int", "?mentions": "arr"},
	"chat.message.recalled": {"message_id": "s64", "channel_id": "s64", "seq": "int", "?recalled_at": "int"},
	"chat.message.removed": {"message_id": "s64", "channel_id": "s64", "?reason": "s32"},
	"chat.reaction.updated": {"message_id": "s64", "channel_id": "s64", "reactions": "obj"},
	"chat.pin.updated": {"channel_id": "s64", "pinned_message_ids": "arr", "?actor": "obj"},
	"chat.typing.updated": {"channel_id": "s64", "typing": "arr"},
	"chat.receipt.updated": {"channel_id": "s64", "?member": "obj", "?delivered_seq": "int", "?read_seq": "int"},
	"chat.unread.updated": {"counts": "obj", "?mentions": "obj"},
	"chat.mention.created": {"channel_id": "s64", "message_id": "s64", "by": "obj", "?seq": "int", "?preview": "s80"},
	"chat.sanction.applied": {"sanction_type": "s32", "?reason_code": "s64", "?until": "int", "?notice": "s1000"},
	"chat.history.loaded": {"channel_id": "s64", "messages": "arr", "has_more": "bool"},
	"chat.channel.membership_changed": {"channel_id": "s64", "channel_type": "s16", "op": "s16", "status": "s16"},
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
