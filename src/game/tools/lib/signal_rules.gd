extends RefCounted
## Signal conventions (GODOT_ARCHITECTURE §7, AGENTS §7, F060-AC-06): snake_case, past tense, typed arguments;
## `event_bus.gd` declares only signals (its list is the reviewed, fixed list of global signals).

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const EVENT_BUS_PATH: String = "res://scripts/core/event_bus.gd"
## Irregular past participles / past forms accepted as the last word of a signal name.
const IRREGULAR_PAST: PackedStringArray = [
	"begun", "bought", "broken", "brought", "built", "caught", "chosen", "come", "cut", "done", "drawn", "fallen",
	"found", "gone", "got", "gotten", "given", "held", "hidden", "hit", "hurt", "kept", "known", "laid", "led", "left",
	"lost", "made", "met", "paid", "put", "quit", "read", "run", "seen", "sent", "set", "shown", "shut", "sold",
	"spent", "split", "spoken", "stood", "struck", "taken", "thrown", "understood", "woken", "won", "worn", "written",
]
const SIGNAL_PATTERN: String = "^\\s*signal\\s+([A-Za-z0-9_]+)\\s*(\\((.*)\\))?\\s*$"
const SNAKE_CASE_PATTERN: String = "^[a-z][a-z0-9]*(_[a-z0-9]+)*$"
const TYPED_ARG_PATTERN: String = "^[a-z_][a-z0-9_]*\\s*:\\s*[A-Za-z_][A-Za-z0-9_.\\[\\], ]*$"
## Lines allowed in event_bus.gd besides signal declarations.
const EVENT_BUS_ALLOWED: String = "^\\s*(extends\\s+Node|@warning_ignore\\(.*\\)|##.*|#.*)?\\s*$"


static func check(path: String, text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var signal_re: RegEx = RegEx.create_from_string(SIGNAL_PATTERN)
	var allowed_re: RegEx = RegEx.create_from_string(EVENT_BUS_ALLOWED)
	var is_event_bus: bool = path == EVENT_BUS_PATH
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var code: String = ToolFiles.strip_comment(lines[i]).strip_edges(false, true)
		var m: RegExMatch = signal_re.search(code)
		if m == null:
			if is_event_bus and allowed_re.search(lines[i]) == null:
				out.append("%s:%d event_bus.gd may only declare signals" % [path, i + 1])
			continue
		for problem: String in check_declaration(m.get_string(1), m.get_string(3)):
			out.append("%s:%d %s" % [path, i + 1, problem])
	return out


## Problems of one declaration `signal <name>(<args>)`.
static func check_declaration(signal_name: String, args: String) -> PackedStringArray:
	var out: PackedStringArray = []
	if RegEx.create_from_string(SNAKE_CASE_PATTERN).search(signal_name) == null:
		out.append("signal '%s' is not snake_case" % signal_name)
	if not is_past_tense(signal_name):
		out.append("signal '%s' must be past tense (e.g. quest_updated)" % signal_name)
	var typed_re: RegEx = RegEx.create_from_string(TYPED_ARG_PATTERN)
	for arg: String in _split_args(args):
		if typed_re.search(arg) == null:
			out.append("signal '%s' argument '%s' has no type" % [signal_name, arg])
	return out


static func is_past_tense(signal_name: String) -> bool:
	var words: PackedStringArray = signal_name.split("_", false)
	if words.is_empty():
		return false
	var last: String = words[words.size() - 1]
	return (last.length() > 2 and last.ends_with("ed")) or last in IRREGULAR_PAST


## Splits an argument list on top-level commas (ignores commas inside `[]`, e.g. Dictionary[String, int]).
static func _split_args(args: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var depth: int = 0
	var current: String = ""
	for c: String in args:
		if c == "[":
			depth += 1
		elif c == "]":
			depth -= 1
		if c == "," and depth == 0:
			out.append(current.strip_edges())
			current = ""
		else:
			current += c
	if not current.strip_edges().is_empty():
		out.append(current.strip_edges())
	return out
