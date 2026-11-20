class_name RealtimeSession
extends RefCounted
## Logical realtime connection of this device (F092 data-model §1, AsyncAPI `system.session.welcome`): limits and
## timers from the welcome, the resume token and the last processed `seq`. RAM only (never stored, never logged —
## AGENTS R14, ADR-0086).

## CONFIG_DEFAULTS `ws_heartbeat` / `ws_timeout` (used when the welcome omits them).
const DEFAULT_HEARTBEAT_S: int = 15
const DEFAULT_TIMEOUT_S: int = 45
const DEFAULT_MAX_MOVE_HZ: int = 10

var conn_id: String = ""
var resume_token: String = ""
var last_seq: int = 0
var heartbeat_s: int = DEFAULT_HEARTBEAT_S
var timeout_s: int = DEFAULT_TIMEOUT_S
var max_frame_bytes: int = WsEnvelope.DEFAULT_MAX_FRAME_BYTES
var max_move_hz: int = DEFAULT_MAX_MOVE_HZ
## Server clock of the welcome (epoch ms) and the local monotonic ms when it arrived.
var server_time_ms: int = 0


## Applies a validated welcome payload; the resume token is replaced by the caller (resume keeps the old one).
func apply_welcome(payload: Dictionary) -> void:
	conn_id = String(payload.get("conn_id", ""))
	heartbeat_s = clampi(WsEnvelope.as_int(payload.get("heartbeat_s"), DEFAULT_HEARTBEAT_S), 1, 300)
	timeout_s = clampi(WsEnvelope.as_int(payload.get("timeout_s"), DEFAULT_TIMEOUT_S), heartbeat_s, 900)
	server_time_ms = WsEnvelope.as_int(payload.get("server_time"))
	var limits: Dictionary = payload.get("limits", {}) as Dictionary if payload.get("limits") is Dictionary else {}
	max_frame_bytes = clampi(WsEnvelope.as_int(limits.get("max_frame_bytes"), WsEnvelope.DEFAULT_MAX_FRAME_BYTES), 512,
			WsPeer.BUFFER_BYTES)
	max_move_hz = clampi(WsEnvelope.as_int(limits.get("max_move_hz"), DEFAULT_MAX_MOVE_HZ), 1, DEFAULT_MAX_MOVE_HZ)


func clear() -> void:
	conn_id = ""
	resume_token = ""
	last_seq = 0
