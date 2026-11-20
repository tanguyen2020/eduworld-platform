class_name FakeWebSocketPeer
extends WsPeer
## TEST-ONLY (AGENTS R17): scripted WebSocket peer for RealtimeClient tests (GAME_NETWORKING §11). Records the URL,
## subprotocols and every text frame the client sends; the test opens the connection, pushes server frames
## (`push`/`push_raw`), drops them (`loss` simulation is done by the test not pushing) and closes with any code.
## Lives under tests/ (excluded from every export preset).

var url: String = ""
var protocols: PackedStringArray = []
var tls: TLSOptions = null
var connect_error: Error = OK
## Frames sent by the client, decoded (JSON objects).
var sent: Array[Dictionary] = []
var sent_raw: PackedStringArray = []
var closed_by_client: bool = false
var client_close_code: int = -1
var _state: ReadyState = ReadyState.CLOSED
var _inbox: PackedStringArray = []
var _close_code: int = -1
var _selected_protocol: String = ""


func connect_to_url(target: String, offered: PackedStringArray, options: TLSOptions = null) -> Error:
	url = target
	protocols = offered
	tls = options
	if connect_error != OK:
		return connect_error
	_state = ReadyState.CONNECTING
	return OK


## Completes the handshake with the given subprotocol.
func open(protocol: String = "eduworld.v1") -> void:
	_selected_protocol = protocol
	_state = ReadyState.OPEN


func poll() -> void:
	pass


func get_ready_state() -> ReadyState:
	return _state


func get_selected_protocol() -> String:
	return _selected_protocol


func send_text(text: String) -> Error:
	if _state != ReadyState.OPEN:
		return ERR_CONNECTION_ERROR
	sent_raw.append(text)
	var parsed: Variant = JSON.parse_string(text)
	sent.append(parsed as Dictionary if parsed is Dictionary else {})
	return OK


func get_available_packet_count() -> int:
	return _inbox.size()


func get_packet_text() -> String:
	if _inbox.is_empty():
		return ""
	var text: String = _inbox[0]
	_inbox.remove_at(0)
	return text


func close(code: int = 1000, _reason: String = "") -> void:
	closed_by_client = true
	client_close_code = code
	_close_code = code
	_state = ReadyState.CLOSED


func get_close_code() -> int:
	return _close_code


# ---------- server side (test API) ----------

## Queues a server frame built from an envelope dictionary.
func push(envelope: Dictionary) -> void:
	_inbox.append(JSON.stringify(envelope))


func push_raw(text: String) -> void:
	_inbox.append(text)


## Server closes the connection with `code` (-1 = dropped without a close frame).
func server_close(code: int) -> void:
	_close_code = code
	_state = ReadyState.CLOSED


## Frames of one `type` sent by the client.
func sent_of(type: String) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for frame: Dictionary in sent:
		if String(frame.get("type", "")) == type:
			out.append(frame)
	return out


func last_sent() -> Dictionary:
	return sent[sent.size() - 1] if not sent.is_empty() else {}
