class_name VttParser
extends RefCounted
## WebVTT subtitles parsed by the client (F065-AC-16, GODOT_ARCHITECTURE §6.2: "phụ đề WebVTT tự parse và vẽ theo
## Theme"). Supports the cue timing line `hh:mm:ss.mmm --> hh:mm:ss.mmm` (hours optional), optional cue identifiers,
## multi-line cue text and NOTE/STYLE blocks (skipped). Tags such as `<b>` or `<v Speaker>` are stripped; F099 keeps the
## speaker of `<v Speaker>` so the caption card writes it as words ("Cô: …"), never as a colour.

const MAX_CUES: int = 5000
const TAG_PATTERN: String = "<[^>]*>"
## F099: the voice span `<v Name>` / `<v.class Name>` (speaker name in group 1).
const VOICE_PATTERN: String = "<v(?:\\.[^ >]*)?\\s+([^>]+)>"


class Cue:
	extends RefCounted
	var start_ms: int = 0
	var end_ms: int = 0
	var text: String = ""
	## Speaker of the cue (`<v Name>`), "" when none (F099).
	var voice: String = ""


var cues: Array[Cue] = []


## Parses `source`; returns false when it is not WebVTT (no `WEBVTT` header).
func parse(source: String) -> bool:
	cues.clear()
	var text: String = source.replace("\r\n", "\n").replace("\r", "\n")
	if not text.is_empty() and text.unicode_at(0) == 0xFEFF:
		text = text.substr(1)
	if not text.begins_with("WEBVTT"):
		return false
	var tags: RegEx = RegEx.create_from_string(TAG_PATTERN)
	var voices: RegEx = RegEx.create_from_string(VOICE_PATTERN)
	for block: String in text.split("\n\n", false):
		var lines: PackedStringArray = block.strip_edges().split("\n", false)
		var timing: int = -1
		for i: int in lines.size():
			if lines[i].contains("-->"):
				timing = i
				break
		if timing < 0:
			continue
		var parts: PackedStringArray = lines[timing].split("-->")
		var start: int = parse_time(parts[0])
		var end: int = parse_time(parts[1].strip_edges().split(" ")[0])
		if start < 0 or end <= start:
			continue
		var body: PackedStringArray = []
		var voice: String = ""
		for j: int in range(timing + 1, lines.size()):
			var found: RegExMatch = voices.search(lines[j])
			if found != null and voice.is_empty():
				voice = found.get_string(1).strip_edges()
			body.append(tags.sub(lines[j], "", true).strip_edges())
		var cue: Cue = Cue.new()
		cue.voice = voice
		cue.start_ms = start
		cue.end_ms = end
		cue.text = "\n".join(body).strip_edges()
		cues.append(cue)
		if cues.size() >= MAX_CUES:
			break
	cues.sort_custom(func(a: Cue, b: Cue) -> bool: return a.start_ms < b.start_ms)
	return true


## Milliseconds of `hh:mm:ss.mmm` or `mm:ss.mmm`; -1 when malformed.
static func parse_time(value: String) -> int:
	var v: String = value.strip_edges()
	var fields: PackedStringArray = v.split(":")
	if fields.size() < 2 or fields.size() > 3:
		return -1
	var seconds_part: PackedStringArray = fields[fields.size() - 1].split(".")
	if seconds_part.size() != 2 or not seconds_part[0].is_valid_int() or not seconds_part[1].is_valid_int():
		return -1
	var total: int = int(seconds_part[0]) * 1000 + int(seconds_part[1].left(3).rpad(3, "0"))
	var minutes: String = fields[fields.size() - 2]
	if not minutes.is_valid_int():
		return -1
	total += int(minutes) * 60_000
	if fields.size() == 3:
		if not fields[0].is_valid_int():
			return -1
		total += int(fields[0]) * 3_600_000
	return total


## Text shown at `position_ms` (cues overlapping are joined), empty when none.
func text_at(position_ms: int) -> String:
	var out: PackedStringArray = []
	for cue: Cue in cues:
		if cue.start_ms > position_ms:
			break
		if position_ms < cue.end_ms:
			out.append(cue.text)
	return "\n".join(out)


## F099: the caption card text at `position_ms`: each cue prefixed by its speaker in words ("Cô: …"), "" when none.
func caption_at(position_ms: int) -> String:
	var out: PackedStringArray = []
	for cue: Cue in cues:
		if cue.start_ms > position_ms:
			break
		if position_ms < cue.end_ms:
			out.append(cue.text if cue.voice.is_empty() else "%s: %s" % [cue.voice, cue.text])
	return "\n".join(out)


## Picks the subtitle language: the UI language when available, else the `is_default` track, else the first.
static func pick_language(tracks: Array[Dictionary], ui_language: String) -> String:
	var fallback: String = ""
	for t: Dictionary in tracks:
		var lang: String = String(t.get("language", ""))
		if lang.left(2) == ui_language.left(2):
			return lang
		if bool(t.get("is_default", false)) and fallback.is_empty():
			fallback = lang
	if fallback.is_empty() and not tracks.is_empty():
		fallback = String(tracks[0].get("language", ""))
	return fallback
