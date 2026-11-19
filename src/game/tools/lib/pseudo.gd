extends RefCounted
## Pseudo-localisation `xx-PS` (GODOT_ARCHITECTURE §10.1, F060-AC-17): accented letters, +40 % length, `[!! … !!]`
## markers, so untranslated or clipped strings are visible. Placeholders `{name}` / `%s` stay intact and the
## `ui.format.*` entries are copied unchanged (they are patterns, formatted like en-US).

const EXPANSION: float = 0.4
const PAD_CHAR: String = "~"
const PREFIX: String = "[!! "
const SUFFIX: String = " !!]"
const FORMAT_PREFIX: String = "ui.format."
const ACCENTS: Dictionary = {
	"a": "á", "b": "ƀ", "c": "ç", "d": "đ", "e": "ë", "f": "ƒ", "g": "ğ", "h": "ĥ", "i": "ï", "j": "ĵ", "k": "ķ",
	"l": "ĺ", "m": "ɱ", "n": "ñ", "o": "ö", "p": "þ", "r": "ŕ", "s": "š", "t": "ţ", "u": "ü", "w": "ŵ", "y": "ý",
	"z": "ž", "A": "Å", "B": "Ɓ", "C": "Ç", "D": "Đ", "E": "Ẹ", "G": "Ğ", "H": "Ĥ", "I": "Ï", "J": "Ĵ", "K": "Ķ",
	"L": "Ĺ", "N": "Ñ", "O": "Ö", "R": "Ŕ", "S": "Š", "T": "Ţ", "U": "Ü", "W": "Ŵ", "Y": "Ý", "Z": "Ž",
}


static func transform(text: String) -> String:
	var out: String = ""
	var in_placeholder: bool = false
	var i: int = 0
	while i < text.length():
		var c: String = text[i]
		if c == "{":
			in_placeholder = true
		elif c == "}":
			in_placeholder = false
		if c == "%" and i + 1 < text.length():
			out += c + text[i + 1]
			i += 2
			continue
		out += c if in_placeholder else String(ACCENTS.get(c, c))
		i += 1
	var pad: int = ceili(text.length() * EXPANSION)
	return PREFIX + out + " " + PAD_CHAR.repeat(pad) + SUFFIX


static func build(source: Dictionary) -> Dictionary:
	var out: Dictionary = {}
	for key: Variant in source.keys():
		var k: String = String(key)
		out[k] = String(source[k]) if k.begins_with(FORMAT_PREFIX) else transform(String(source[k]))
	return out
