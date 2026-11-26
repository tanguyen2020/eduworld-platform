extends Node
## Autoload #2 `Log` (GODOT_ARCHITECTURE §4, F060-AC-04, AGENTS R14).
## Structured log into a ring buffer (size `eduworld/log/ring_buffer_size`, attached to crash reports by F061+).
## Every field is redacted before it is stored or printed: tokens, passwords, `Authorization`, real names.
## Never sends logs to the server by itself.

enum Level { DEBUG, INFO, WARN, ERROR }

const LEVEL_NAMES: PackedStringArray = ["debug", "info", "warn", "error"]
const REDACTED: String = "[REDACTED]"
## Field names (lower case, compared after removing `-`) whose value is always hidden.
const SENSITIVE_KEYS: PackedStringArray = [
	"password", "passwd", "pwd", "pin", "secret", "client_secret", "token", "access_token", "refresh_token",
	"id_token", "authorization", "cookie", "set_cookie", "login_ticket", "otp", "ticket", "resume_token",
	"full_name", "fullname", "first_name", "last_name", "given_name", "family_name", "real_name", "ho_ten",
	"display_name", "name",
	# F119 (AGENTS R14): chat content never reaches a log, whatever the event.
	"body", "chat_text", "draft", "excerpt", "reply_excerpt", "preview", "quote_excerpt",
]
## Key fragments that hide the value wherever they appear in a field name.
const SENSITIVE_FRAGMENTS: PackedStringArray = ["password", "token", "secret", "authorization", "cookie", "ticket"]
## Value patterns hidden even under a harmless key: bearer credentials, JWT-looking strings and a realtime ticket in a
## URL query (F092-AC-07).
const VALUE_PATTERNS: PackedStringArray = [
	"(?i)bearer\\s+[A-Za-z0-9._~+/=-]+",
	"eyJ[A-Za-z0-9_-]{6,}\\.[A-Za-z0-9_-]{6,}\\.[A-Za-z0-9_-]*",
	"(?i)ticket=[^&\\s\"]+",
]

var min_level: Level = Level.INFO
var _capacity: int = 500
var _buffer: Array[Dictionary] = []
var _value_regexes: Array[RegEx] = []


func _init() -> void:
	_capacity = maxi(1, int(ProjectSettings.get_setting("eduworld/log/ring_buffer_size", 500)))
	for pattern: String in VALUE_PATTERNS:
		_value_regexes.append(RegEx.create_from_string(pattern))


func _ready() -> void:
	var config: Node = get_node_or_null("/root/Config")
	if config != null:
		set_level_name(String(config.get("log_level")))


func set_level_name(level_name: String) -> void:
	var index: int = LEVEL_NAMES.find(level_name)
	if index >= 0:
		min_level = index as Level


func debug(event: String, fields: Dictionary = {}) -> void:
	write(Level.DEBUG, event, fields)


func info(event: String, fields: Dictionary = {}) -> void:
	write(Level.INFO, event, fields)


func warn(event: String, fields: Dictionary = {}) -> void:
	write(Level.WARN, event, fields)


func error(event: String, fields: Dictionary = {}) -> void:
	write(Level.ERROR, event, fields)


## Stores one redacted entry; entries below `min_level` are dropped (not stored, not printed).
func write(level: Level, event: String, fields: Dictionary = {}) -> void:
	if level < min_level:
		return
	var entry: Dictionary = {
		"ts": Time.get_datetime_string_from_system(true),
		"level": LEVEL_NAMES[level],
		"event": _redact_string(event),
		"fields": redact(fields),
	}
	_buffer.append(entry)
	while _buffer.size() > _capacity:
		_buffer.pop_front()
	var line: String = JSON.stringify(entry)
	if level >= Level.WARN:
		printerr(line)
	else:
		print(line)


## Copy of the ring buffer, oldest first.
func entries() -> Array[Dictionary]:
	return _buffer.duplicate(true)


func capacity() -> int:
	return _capacity


func clear() -> void:
	_buffer.clear()


## Recursively hides sensitive values. Keys are kept so support staff can see which field was present.
func redact(value: Variant, key: String = "") -> Variant:
	if not key.is_empty() and is_sensitive_key(key):
		return REDACTED
	if value is Dictionary:
		var out: Dictionary = {}
		var dict: Dictionary = value as Dictionary
		for k: Variant in dict.keys():
			out[k] = redact(dict[k], String(k))
		return out
	if value is Array:
		var list: Array = []
		for item: Variant in (value as Array):
			list.append(redact(item))
		return list
	if value is String or value is StringName:
		return _redact_string(String(value))
	if value is PackedByteArray:
		return REDACTED
	return value


static func is_sensitive_key(key: String) -> bool:
	var k: String = key.to_lower().replace("-", "_")
	if k in SENSITIVE_KEYS:
		return true
	for fragment: String in SENSITIVE_FRAGMENTS:
		if k.contains(fragment):
			return true
	return false


func _redact_string(text: String) -> String:
	var out: String = text
	for regex: RegEx in _value_regexes:
		out = regex.sub(out, REDACTED, true)
	return out
