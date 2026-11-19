class_name TraceContext
extends RefCounted
## W3C trace context (GAME_NETWORKING §2.1): a fresh `traceparent` for every request so client logs (which keep
## only `code` + `trace_id`) can be joined with server logs.

const VERSION: String = "00"
const FLAGS_SAMPLED: String = "01"


static func new_traceparent() -> String:
	var crypto: Crypto = Crypto.new()
	var trace_id: String = crypto.generate_random_bytes(16).hex_encode()
	var span_id: String = crypto.generate_random_bytes(8).hex_encode()
	return "%s-%s-%s-%s" % [VERSION, trace_id, span_id, FLAGS_SAMPLED]


## Trace id (32 hex) of a `traceparent` header value; empty when malformed.
static func trace_id_of(traceparent: String) -> String:
	var parts: PackedStringArray = traceparent.split("-")
	return parts[1] if parts.size() == 4 and parts[1].length() == 32 else ""
