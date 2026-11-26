extends GdUnitTestSuite
## F096-AC-15 (NFR-COMP-004, NFR-I18N-003, AGENTS R32): the Web build has no system fonts, so every character a student
## can see must be in the fonts the Theme ships (Baloo 2, Be Vietnam Pro): each msgstr of vi.po / en.po, and no emoji
## left in the string literals of scene/gameplay scripts. Symbols are SVG icons of design/icons/game instead.

const PoFile := preload("res://tools/lib/po_file.gd")
const ToolFiles := preload("res://tools/lib/tool_files.gd")
const FONT_DIR: String = "res://resources/ui/theme/fonts"
const LOCALES: PackedStringArray = ["res://locale/vi.po", "res://locale/en.po"]
## Characters drawn only by a heading font (Baloo 2, `HeadingLabel`) that covers them: "path U+XXXX".
const HEADING_ONLY: PackedStringArray = [
	"res://scenes/learning/simulation/templates/physics_lever/v1/LeverTemplate.tscn U+25B2",
]
const HEADING_FONT: String = "Baloo2-Bold.ttf"
## Ranges the shipped fonts never cover (emoji, pictographs, dingbats, misc symbols/arrows used as icons).
const EMOJI_RANGES: Array[Vector2i] = [
	Vector2i(0x1F000, 0x1FAFF), Vector2i(0x2600, 0x27BF), Vector2i(0x2B00, 0x2BFF), Vector2i(0xFE0F, 0xFE0F),
]


func _fonts() -> Array[FontFile]:
	var out: Array[FontFile] = []
	for file: String in DirAccess.get_files_at(FONT_DIR):
		if file.get_extension() == "ttf":
			out.append(load(FONT_DIR.path_join(file)) as FontFile)
	return out


func test_every_translated_character_is_in_the_shipped_fonts() -> void:
	var fonts: Array[FontFile] = _fonts()
	assert_int(fonts.size()).is_greater_equal(3)
	var missing: PackedStringArray = []
	for po: String in LOCALES:
		var entries: Dictionary = PoFile.parse(FileAccess.get_file_as_string(po))
		for key: String in entries:
			var text: String = String(entries[key])
			for i: int in text.length():
				var code: int = text.unicode_at(i)
				if code < 0x20:
					continue
				for font: FontFile in fonts:
					if not font.has_char(code):
						missing.append("%s %s: U+%04X '%s' not in %s" % [po.get_file(), key, code, text[i],
								font.resource_path.get_file()])
						break
	assert_array(missing).override_failure_message("\n".join(missing)).is_empty()


## String literals of scene/gameplay scripts and `text` of scenes (icons as glyphs, timers, marks) must be renderable too.
func test_no_glyph_outside_the_fonts_in_scene_and_gameplay_strings() -> void:
	var fonts: Array[FontFile] = _fonts()
	var literal: RegEx = RegEx.create_from_string("\"((?:[^\"\\\\]|\\\\.)*)\"")
	assert_bool(literal.is_valid()).is_true()
	# Non-ASCII character → first place it was seen (each character is checked against the fonts once).
	var seen: Dictionary = {}
	var literals: int = 0
	var files: PackedStringArray = ToolFiles.list_files("res://scenes", PackedStringArray(["gd", "tscn"]))
	files.append_array(ToolFiles.list_files("res://scripts", PackedStringArray(["gd"])))
	for path: String in files:
		var lines: PackedStringArray = FileAccess.get_file_as_string(path).split("\n")
		for n: int in lines.size():
			var line: String = lines[n]
			if path.ends_with(".tscn"):
				if not (line.begins_with("text = ") or line.begins_with("tooltip_text = ")):
					continue
			else:
				line = ToolFiles.strip_comment(line)
			if line.find("\"") < 0:
				continue
			for m: RegExMatch in literal.search_all(line):
				literals += 1
				var value: String = m.get_string(1)
				for i: int in value.length():
					var c: int = value.unicode_at(i)
					if c >= 0x80 and not seen.has(c):
						seen[c] = "%s:%d" % [path, n + 1]
	assert_int(literals).is_greater(1000)
	var found: PackedStringArray = []
	for c: int in seen:
		for r: Vector2i in EMOJI_RANGES:
			if c >= r.x and c <= r.y:
				found.append("%s U+%04X (emoji)" % [seen[c], c])
		var where: String = String(seen[c]).get_slice(":", 0) + ":" + String(seen[c]).get_slice(":", 1)
		var heading_only: bool = "%s U+%04X" % [where, c] in HEADING_ONLY
		for font: FontFile in fonts:
			if heading_only and font.resource_path.get_file() != HEADING_FONT:
				continue
			if not font.has_char(c):
				found.append("%s U+%04X not in %s" % [seen[c], c, font.resource_path.get_file()])
				break
	assert_array(found).override_failure_message("\n".join(found)).is_empty()
