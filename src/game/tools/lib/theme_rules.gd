extends RefCounted
## Theme rule (DESIGN_SYSTEM §5, §9; AGENTS R32; F060-AC-07, F060-AC-11): every Control takes its style from the
## Theme generated from design/game-tokens. Scenes may not override colours, fonts, font sizes or styleboxes, nor
## point at another Theme; scripts under scenes/ may not add theme overrides at runtime. Exceptions are listed in
## res://tools/theme_override_allowlist.txt as `<res path>::<property>` (reviewed like code).

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const ALLOWLIST_PATH: String = "res://tools/theme_override_allowlist.txt"
const GENERATED_THEME_PREFIX: String = "res://resources/ui/theme/eduworld_theme"
const SCENE_OVERRIDE_PATTERN: String = "^(theme_override_(colors|fonts|font_sizes|styles|icons)/[A-Za-z0-9_]+)\\s*="
const SCRIPT_OVERRIDE_PATTERN: String = "add_theme_(color|font|font_size|stylebox|icon)_override\\s*\\("
const EXT_THEME_PATTERN: String = "^\\[ext_resource [^\\]]*type=\"Theme\"[^\\]]*path=\"([^\"]+)\""


static func load_allowlist(path: String = ALLOWLIST_PATH) -> PackedStringArray:
	var out: PackedStringArray = []
	if not FileAccess.file_exists(path):
		return out
	for line: String in FileAccess.get_file_as_string(path).split("\n"):
		var entry: String = line.strip_edges()
		if not entry.is_empty() and not entry.begins_with("#"):
			out.append(entry)
	return out


static func check_scene(path: String, text: String, allowlist: PackedStringArray) -> PackedStringArray:
	var out: PackedStringArray = []
	var override_re: RegEx = RegEx.create_from_string(SCENE_OVERRIDE_PATTERN)
	var theme_re: RegEx = RegEx.create_from_string(EXT_THEME_PATTERN)
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var line: String = lines[i]
		var m: RegExMatch = override_re.search(line)
		if m != null and not ("%s::%s" % [path, m.get_string(1)]) in allowlist:
			out.append("%s:%d theme override '%s' (use the generated Theme / a type variation, R32)" % [
				path, i + 1, m.get_string(1)])
		var t: RegExMatch = theme_re.search(line)
		if t != null and not t.get_string(1).begins_with(GENERATED_THEME_PREFIX):
			out.append("%s:%d Theme '%s' is not generated from design/game-tokens (R32)" % [path, i + 1, t.get_string(1)])
	return out


static func check_script(path: String, text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var re: RegEx = RegEx.create_from_string(SCRIPT_OVERRIDE_PATTERN)
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		if re.search(ToolFiles.strip_comment(lines[i])) != null:
			out.append("%s:%d runtime theme override in a scene script (R32)" % [path, i + 1])
	return out


## Theme resources other than the generated ones are not allowed in the game project.
static func check_theme_resource(path: String) -> PackedStringArray:
	if path.begins_with(GENERATED_THEME_PREFIX):
		return PackedStringArray()
	return PackedStringArray(["%s is a Theme not generated from design/game-tokens (R32)" % path])
