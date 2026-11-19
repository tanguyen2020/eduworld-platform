class_name Uuid
extends RefCounted
## UUID helpers (F061): v4 for the install id `device_id` (ADR-0031), v7 for `Idempotency-Key` (ADR-0089 §4 —
## the server rejects a key that is not UUIDv7). Random bytes come from `Crypto` (CSPRNG).

const PATTERN: String = "^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
const V7_PATTERN: String = "^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"


static func v4() -> String:
	var b: PackedByteArray = Crypto.new().generate_random_bytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return _format(b)


## Time-ordered UUID: 48-bit unix milliseconds + random bits (RFC 9562 §5.7).
static func v7(unix_ms: int = -1) -> String:
	var ms: int = unix_ms if unix_ms >= 0 else int(Time.get_unix_time_from_system() * 1000.0)
	var b: PackedByteArray = Crypto.new().generate_random_bytes(16)
	for i: int in 6:
		b[i] = (ms >> (8 * (5 - i))) & 0xff
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return _format(b)


static func is_valid(value: String) -> bool:
	return RegEx.create_from_string(PATTERN).search(value.to_lower()) != null


static func is_v7(value: String) -> bool:
	return RegEx.create_from_string(V7_PATTERN).search(value.to_lower()) != null


static func _format(b: PackedByteArray) -> String:
	var h: String = b.hex_encode()
	return "%s-%s-%s-%s-%s" % [h.substr(0, 8), h.substr(8, 4), h.substr(12, 4), h.substr(16, 4), h.substr(20, 12)]
