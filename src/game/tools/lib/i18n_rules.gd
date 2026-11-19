extends RefCounted
## i18n rules (GODOT_ARCHITECTURE §10.1, NFR-I18N-001, F060-AC-12, F060-AC-13, F060-AC-17):
## collects translation keys `ui.<screen>.<element>[.<state>]` from scripts and scenes, and finds hard-coded
## user-visible text in scenes (text/tooltip/placeholder must be a key; numbers and symbols are allowed).

const KEY_BODY: String = "ui\\.[a-z0-9_]+(?:\\.[A-Za-z0-9_]+)+"
const SCENE_TEXT_PATTERN: String = "^(text|tooltip_text|placeholder_text|title)\\s*=\\s*\"(.*)\"\\s*$"
## Visible text that needs no translation has no letter at all (digits, punctuation, symbols).
const LETTER_PATTERN: String = "[A-Za-z\\x{00C0}-\\x{024F}\\x{1E00}-\\x{1EFF}]"
const SOURCE_EXTENSIONS: PackedStringArray = ["gd", "tscn"]
## Folders scanned for keys (tests and tools are excluded: they are not shipped).
const SOURCE_ROOTS: PackedStringArray = ["res://scenes", "res://scripts"]


static func keys_in(text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var re: RegEx = RegEx.create_from_string('"(' + KEY_BODY + ')"')
	for m: RegExMatch in re.search_all(text):
		var key: String = m.get_string(1)
		if not key in out:
			out.append(key)
	return out


static func is_key(value: String) -> bool:
	return RegEx.create_from_string("^" + KEY_BODY + "$").search(value) != null


static func hardcoded_text(path: String, text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var re: RegEx = RegEx.create_from_string(SCENE_TEXT_PATTERN)
	var letter_re: RegEx = RegEx.create_from_string(LETTER_PATTERN)
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var m: RegExMatch = re.search(lines[i])
		if m == null:
			continue
		var value: String = m.get_string(2)
		if not is_key(value) and letter_re.search(value) != null:
			out.append('%s:%d hard-coded %s "%s" (use a key ui.<screen>.<element>)' % [path, i + 1, m.get_string(1), value])
	return out


## Keys of `pot_keys` missing (or empty) in a catalog.
static func missing(pot_keys: PackedStringArray, catalog: Dictionary) -> PackedStringArray:
	var out: PackedStringArray = []
	for key: String in pot_keys:
		if not catalog.has(key) or String(catalog[key]).is_empty():
			out.append(key)
	return out
