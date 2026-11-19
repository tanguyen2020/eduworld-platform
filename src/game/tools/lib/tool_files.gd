extends RefCounted
## Shared helpers of the src/game/tools lint scripts: project file walking and comment stripping.

const SKIP_DIRS: PackedStringArray = [".godot", "addons", "build", "reports", "android"]


## Lists `res://` files under `root` with one of `extensions`, skipping SKIP_DIRS and `extra_skip` prefixes.
static func list_files(root: String, extensions: PackedStringArray, extra_skip: PackedStringArray = []) -> PackedStringArray:
	var out: PackedStringArray = []
	_walk(root, extensions, extra_skip, out)
	out.sort()
	return out


static func _walk(dir_path: String, extensions: PackedStringArray, extra_skip: PackedStringArray,
		out: PackedStringArray) -> void:
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return
	for sub: String in dir.get_directories():
		if sub in SKIP_DIRS or sub.begins_with("."):
			continue
		var sub_path: String = dir_path.path_join(sub)
		var skipped: bool = false
		for prefix: String in extra_skip:
			if sub_path.begins_with(prefix):
				skipped = true
		if not skipped:
			_walk(sub_path, extensions, extra_skip, out)
	for file_name: String in dir.get_files():
		if file_name.get_extension() in extensions:
			out.append(dir_path.path_join(file_name))


static func read_text(path: String) -> String:
	return FileAccess.get_file_as_string(path)


## Removes a `#` comment that is not inside a string literal.
static func strip_comment(line: String) -> String:
	var quote: String = ""
	var i: int = 0
	while i < line.length():
		var c: String = line[i]
		if quote.is_empty():
			if c == "#":
				return line.substr(0, i)
			if c == "\"" or c == "'":
				quote = c
		elif c == "\\":
			i += 1
		elif c == quote:
			quote = ""
		i += 1
	return line


## Removes string literal contents (keeps the quotes) so identifiers inside strings are not matched.
static func strip_strings(line: String) -> String:
	var out: String = ""
	var quote: String = ""
	var i: int = 0
	while i < line.length():
		var c: String = line[i]
		if quote.is_empty():
			out += c
			if c == "\"" or c == "'":
				quote = c
		elif c == "\\":
			i += 1
		elif c == quote:
			out += c
			quote = ""
		i += 1
	return out


## Prints violations and returns the process exit code (0 = clean).
static func report(tool_name: String, violations: PackedStringArray, checked: int) -> int:
	for v: String in violations:
		printerr("%s: %s" % [tool_name, v])
	if violations.is_empty():
		print("%s: OK (%d files)" % [tool_name, checked])
		return 0
	printerr("%s: %d violation(s) in %d files" % [tool_name, violations.size(), checked])
	return 1
