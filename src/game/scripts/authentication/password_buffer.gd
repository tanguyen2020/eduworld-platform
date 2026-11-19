class_name PasswordBuffer
extends RefCounted
## Holds the password only while a `gameLogin` request is in flight, including the `429 LOGIN_QUEUE_BUSY` retry
## loop (research R1, ADR-0083). `wipe()` overwrites the bytes with 0 and clears them at every final response,
## on cancel and on error; takeover never needs the password. No API writes it anywhere.

var _bytes: PackedByteArray = PackedByteArray()


func set_text(value: String) -> void:
	wipe()
	_bytes = value.to_utf8_buffer()


## Password for building the request body (the only reader).
func text() -> String:
	return _bytes.get_string_from_utf8()


func is_empty() -> bool:
	return _bytes.is_empty()


func wipe() -> void:
	_bytes.fill(0)
	_bytes.clear()


func _to_string() -> String:
	return "PasswordBuffer([REDACTED])"
