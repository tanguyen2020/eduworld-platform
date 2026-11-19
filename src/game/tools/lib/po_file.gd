extends RefCounted
## Minimal gettext .po/.pot reader/writer for the i18n tools (single msgid/msgstr entries, no plurals yet).

const BACKSLASH: String = "\\"
const QUOTE: String = "\""
const NEWLINE: String = "\n"


## Parses `text` into { msgid: msgstr } (the header entry with an empty msgid is excluded).
static func parse(text: String) -> Dictionary:
	var entries: Dictionary = {}
	var msgid: String = ""
	var msgstr: String = ""
	var field: String = ""
	var has_entry: bool = false
	for raw: String in text.split(NEWLINE):
		var line: String = raw.strip_edges()
		if line.begins_with("msgid "):
			if has_entry and not msgid.is_empty():
				entries[msgid] = msgstr
			msgid = unquote(line.trim_prefix("msgid "))
			msgstr = ""
			field = "msgid"
			has_entry = true
		elif line.begins_with("msgstr "):
			msgstr = unquote(line.trim_prefix("msgstr "))
			field = "msgstr"
		elif line.begins_with(QUOTE):
			if field == "msgid":
				msgid += unquote(line)
			elif field == "msgstr":
				msgstr += unquote(line)
	if has_entry and not msgid.is_empty():
		entries[msgid] = msgstr
	return entries


## Quotes a value as a gettext string literal (escapes backslash, quote and newline).
static func quote(value: String) -> String:
	var escaped: String = value.replace(BACKSLASH, BACKSLASH + BACKSLASH)
	escaped = escaped.replace(QUOTE, BACKSLASH + QUOTE).replace(NEWLINE, BACKSLASH + "n")
	return QUOTE + escaped + QUOTE


static func unquote(value: String) -> String:
	var v: String = value.strip_edges()
	if v.length() >= 2 and v.begins_with(QUOTE) and v.ends_with(QUOTE):
		v = v.substr(1, v.length() - 2)
	var out: String = ""
	var i: int = 0
	while i < v.length():
		var c: String = v[i]
		if c == BACKSLASH and i + 1 < v.length():
			var n: String = v[i + 1]
			out += NEWLINE if n == "n" else n
			i += 2
			continue
		out += c
		i += 1
	return out


## Serialises a header block plus entries sorted by key.
static func write(header_lines: PackedStringArray, entries: Dictionary, comment: String = "") -> String:
	var out: String = comment
	out += 'msgid ""' + NEWLINE + 'msgstr ""' + NEWLINE
	for h: String in header_lines:
		out += quote(h + NEWLINE) + NEWLINE
	var keys: Array = entries.keys()
	keys.sort()
	for key: Variant in keys:
		out += NEWLINE + "msgid " + quote(String(key)) + NEWLINE + "msgstr " + quote(String(entries[key])) + NEWLINE
	return out
