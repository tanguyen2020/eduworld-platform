class_name WsPeer
extends RefCounted
## Thin interface over Godot's `WebSocketPeer` used by RealtimeClient (F092 plan "ws_peer.gd", GODOT_ARCHITECTURE §12):
## the client only talks to this class, so unit tests swap in `FakeWebSocketPeer` (tests/fakes) with scripted close
## codes, delays and losses. Text frames only (the realtime protocol is JSON, AsyncAPI `eduworld.v1`).
## Buffers are 1 MB in and out (GAME_NETWORKING §4.1 [Confirmed]).

enum ReadyState { CONNECTING, OPEN, CLOSING, CLOSED }

## GAME_NETWORKING §4.1: inbound/outbound buffers of 1 MB.
const BUFFER_BYTES: int = 1 << 20

var _peer: WebSocketPeer = null


## Opens `url` offering `protocols` (subprotocol `eduworld.v1`); `tls` is the client TLS options (CA bundle of the
## release build, see RealtimeClient.tls_options) or null for the engine defaults.
func connect_to_url(url: String, protocols: PackedStringArray, tls: TLSOptions = null) -> Error:
	_peer = WebSocketPeer.new()
	_peer.supported_protocols = protocols
	_peer.inbound_buffer_size = BUFFER_BYTES
	_peer.outbound_buffer_size = BUFFER_BYTES
	return _peer.connect_to_url(url, tls)


func poll() -> void:
	if _peer != null:
		_peer.poll()


func get_ready_state() -> ReadyState:
	if _peer == null:
		return ReadyState.CLOSED
	match _peer.get_ready_state():
		WebSocketPeer.STATE_CONNECTING:
			return ReadyState.CONNECTING
		WebSocketPeer.STATE_OPEN:
			return ReadyState.OPEN
		WebSocketPeer.STATE_CLOSING:
			return ReadyState.CLOSING
	return ReadyState.CLOSED


func get_selected_protocol() -> String:
	return _peer.get_selected_protocol() if _peer != null else ""


func send_text(text: String) -> Error:
	if _peer == null:
		return ERR_UNCONFIGURED
	return _peer.send_text(text)


func get_available_packet_count() -> int:
	return _peer.get_available_packet_count() if _peer != null else 0


## Next text frame (UTF-8); empty when none is waiting.
func get_packet_text() -> String:
	if _peer == null or _peer.get_available_packet_count() == 0:
		return ""
	return _peer.get_packet().get_string_from_utf8()


func close(code: int = 1000, reason: String = "") -> void:
	if _peer != null:
		_peer.close(code, reason)


## Close code sent by the server (-1 while open or when the connection dropped without a close frame).
func get_close_code() -> int:
	return _peer.get_close_code() if _peer != null else -1


## Reason text of the server close frame (never shown to the student).
func get_close_reason() -> String:
	return _peer.get_close_reason() if _peer != null else ""
