class_name HighlightText
extends RefCounted
## F112 (F112-AC-16; Clarifications 2026-11-28 F112-8, CHAT_SYSTEM §8.4): the excerpt of a search hit as PLAIN text —
## every markup-looking tag (`<em>`, `[b]`, `[url=…]`) is removed and nothing is ever parsed (the panel adds the text
## with `RichTextLabel.add_text`, never BBCode — AGENTS R15, F112-AC-30) — and where the keyword is: the server's
## `highlight_ranges` (code points of `highlight`) when they are valid, else a client match that ignores case and
## Vietnamese accents ("di boss" finds "đi boss"), each word of the query being the start of a word. Pure and static;
## never logs the query or the text (AGENTS R14).

const MAX_TEXT: int = 1000
## Vietnamese letters → their base letter (one character each, so positions stay the same after folding).
const FOLD_FROM: String = "àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ"
const FOLD_TO: String = "aaaaaaaaaaaaaaaaaeeeeeeeeeeeiiiiiooooooooooooooooouuuuuuuuuuuyyyyyd"

## Longest tag removed (`[url=` + a value).
const TAG_MAX: int = 160


## `text` without markup tags (`<em>`, `</em>`, `[b]`, `[/b]`, `[url=…]`), trimmed to MAX_TEXT characters. A plain scan,
## no pattern engine (the chat client holds no matching rule — F119 boundaries): a tag is `<` or `[`, an optional `/`, a
## letter, then up to TAG_MAX characters to the closing `>` / `]` (a `[` tag: letters, or `=` and a value).
static func plain(text: String) -> String:
	var out: PackedStringArray = []
	var i: int = 0
	while i < text.length():
		var end: int = _tag_end(text, i)
		if end > i:
			i = end + 1
			continue
		out.append(text[i])
		i += 1
	return "".join(out).left(MAX_TEXT)


## Index of the `>` / `]` closing a tag that starts at `at`, or -1.
static func _tag_end(text: String, at: int) -> int:
	var open: String = text[at]
	if open != "<" and open != "[":
		return -1
	var close: String = ">" if open == "<" else "]"
	var j: int = at + 1
	if j < text.length() and text[j] == "/":
		j += 1
	if j >= text.length() or not _is_ascii_letter(text[j]):
		return -1
	var value: bool = false
	while j < text.length() and j - at <= TAG_MAX:
		var ch: String = text[j]
		if ch == close:
			return j
		if ch == "<" or ch == "[" or (ch == "]" and open == "<") or (ch == ">" and open == "["):
			return -1
		if open == "[" and not value:
			if ch == "=":
				value = true
			elif not _is_ascii_letter(ch):
				return -1
		j += 1
	return -1


static func _is_ascii_letter(ch: String) -> bool:
	return (ch >= "a" and ch <= "z") or (ch >= "A" and ch <= "Z")


## Lower case without Vietnamese accents (same length as `text`).
static func fold(text: String) -> String:
	var lower: String = text.to_lower()
	var out: PackedStringArray = []
	for i: int in lower.length():
		var ch: String = lower[i]
		var at: int = FOLD_FROM.find(ch)
		out.append(FOLD_TO[at] if at >= 0 else ch)
	return "".join(out)


## [start, length] pairs of the words of `query` found at the start of words of `text` (accent / case blind), sorted,
## not overlapping.
static func match_ranges(text: String, query: String) -> Array[Vector2i]:
	var hay: String = fold(text)
	var found: Array[Vector2i] = []
	for word: String in fold(query).split(" ", false):
		var from: int = 0
		while true:
			var at: int = hay.find(word, from)
			if at < 0:
				break
			if at == 0 or not _is_letter(hay[at - 1]):
				found.append(Vector2i(at, word.length()))
			from = at + 1
	return _merge(found)


## The server's `highlight_ranges` (`[{start, length}]`) when every range lies inside `text`, else [].
static func server_ranges(text: String, ranges: Variant) -> Array[Vector2i]:
	var out: Array[Vector2i] = []
	if not ranges is Array:
		return out
	for r: Variant in ranges as Array:
		if not r is Dictionary:
			return [] as Array[Vector2i]
		var start: int = DataRead.int_of(r as Dictionary, "start", -1)
		var length: int = DataRead.int_of(r as Dictionary, "length", 0)
		if start < 0 or length <= 0 or start + length > text.length():
			return [] as Array[Vector2i]
		out.append(Vector2i(start, length))
	return _merge(out)


## The pieces of `text` in order: `[{text, hit}]` (hit = the keyword), for a plain-text label.
static func pieces(text: String, ranges: Array[Vector2i]) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	var at: int = 0
	for r: Vector2i in ranges:
		if r.x > at:
			out.append({"text": text.substr(at, r.x - at), "hit": false})
		out.append({"text": text.substr(r.x, r.y), "hit": true})
		at = r.x + r.y
	if at < text.length():
		out.append({"text": text.substr(at), "hit": false})
	return out


static func _is_letter(ch: String) -> bool:
	return ch.to_lower() != ch.to_upper() or (ch >= "0" and ch <= "9")


static func _merge(list: Array[Vector2i]) -> Array[Vector2i]:
	list.sort_custom(func(a: Vector2i, b: Vector2i) -> bool: return a.x < b.x)
	var out: Array[Vector2i] = []
	for r: Vector2i in list:
		if not out.is_empty() and r.x < out[out.size() - 1].x + out[out.size() - 1].y:
			var last: Vector2i = out[out.size() - 1]
			out[out.size() - 1] = Vector2i(last.x, maxi(last.x + last.y, r.x + r.y) - last.x)
		else:
			out.append(r)
	return out
