class_name WsEnvelope
extends RefCounted
## Envelope of the realtime protocol (AsyncAPI `ClientEnvelope`/`ServerEnvelope`, subprotocol `eduworld.v1`;
## F092-AC-06, data-model §3). Parses a server frame into a typed envelope or rejects it (null): JSON object,
## `v == 1`, `type` matching `^[a-z_]+\.[a-z_]+\.[a-z_]+$` (≤ 64), integer `seq` ≥ 1, object `payload`, frame within
## the inbound limit. Builds client frames (`v`, `type`, `id`, `ts`, `payload`) and refuses one larger than
## `max_frame_bytes` (welcome `limits`). Never logs a payload (R14).

const VERSION: int = 1
const TYPE_MAX_LENGTH: int = 64
const TYPE_PATTERN: String = "^[a-z_]+\\.[a-z_]+\\.[a-z_]+$"
## AsyncAPI `ServerEnvelope.ref` / `ClientEnvelope.id` limits.
const REF_MAX_LENGTH: int = 255
const ID_MIN_LENGTH: int = 10
const ID_MAX_LENGTH: int = 40
## Welcome `limits.max_frame_bytes` default (AsyncAPI).
const DEFAULT_MAX_FRAME_BYTES: int = 16384

var type: String = ""
var seq: int = 0
var ref: String = ""
var ts: int = 0
var payload: Dictionary = {}

static var _type_regex: RegEx = null


static func is_valid_type(value: String) -> bool:
	if value.is_empty() or value.length() > TYPE_MAX_LENGTH:
		return false
	if _type_regex == null:
		_type_regex = RegEx.create_from_string(TYPE_PATTERN)
	return _type_regex.search(value) != null


## JSON numbers arrive as float: true for a whole number (or an int).
static func is_int(value: Variant) -> bool:
	if value is int:
		return true
	return value is float and is_finite(value as float) and float(value) == floorf(value as float)


static func as_int(value: Variant, fallback: int = 0) -> int:
	return int(value) if is_int(value) else fallback


## Parses one server text frame; null when it breaks the envelope contract or is larger than `max_bytes`.
static func parse(text: String, max_bytes: int = WsPeer.BUFFER_BYTES) -> WsEnvelope:
	if text.is_empty() or text.to_utf8_buffer().size() > max_bytes:
		return null
	var data: Variant = JSON.parse_string(text)
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not is_int(d.get("v")) or int(d["v"]) != VERSION:
		return null
	if not d.get("type") is String or not is_valid_type(String(d["type"])):
		return null
	if not is_int(d.get("seq")) or int(d["seq"]) < 1:
		return null
	if not d.get("payload") is Dictionary:
		return null
	if d.has("ref") and (not d["ref"] is String or String(d["ref"]).length() > REF_MAX_LENGTH):
		return null
	var env: WsEnvelope = WsEnvelope.new()
	env.type = String(d["type"])
	env.seq = int(d["seq"])
	env.ref = String(d.get("ref", ""))
	env.ts = as_int(d.get("ts"))
	env.payload = d["payload"] as Dictionary
	return env


## Client frame text, or "" when the type/id is invalid or the frame exceeds `max_frame_bytes`.
static func encode(command_type: String, id: String, body: Dictionary, client_ms: int,
		max_frame_bytes: int = DEFAULT_MAX_FRAME_BYTES) -> String:
	if not is_valid_type(command_type) or id.length() < ID_MIN_LENGTH or id.length() > ID_MAX_LENGTH:
		return ""
	var text: String = JSON.stringify({"v": VERSION, "type": command_type, "id": id, "ts": client_ms, "payload": body})
	if text.to_utf8_buffer().size() > max_frame_bytes:
		return ""
	return text
