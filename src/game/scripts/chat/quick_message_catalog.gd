class_name QuickMessageCatalog
extends RefCounted
## Bilingual quick messages of the server (`listQuickMessages`, CHAT_SYSTEM §8.5, ADR-0056; F119-AC-08). The sender
## sends only `quick_message_id` (+ params); every viewer reads the sentence in the viewer's own language, so a student
## playing in English sees "Good luck!" for a Vietnamese sender. Nothing is hard-coded: an unknown id (catalog not
## loaded yet, a newer catalog version) shows a neutral placeholder key. `{n}`-style parameters come from the server
## message (already moderated, F116-AC-28) and are inserted with `String.format` — no sentence is built by code.

const KEY_UNKNOWN: String = "ui.chat.quick_unknown"

class Item:
	extends RefCounted
	var id: String = ""
	var category: String = ""
	## `{vi, en}` (LocalizedName).
	var text: Dictionary = {}
	## Emoji code shown as an icon beside the words ("" = none).
	var emoji: String = ""
	var sort_order: int = 0


## Items in display order (`sort_order`, then id).
var items: Array[Item] = []
var _by_id: Dictionary = {}
## `channel_type` the list was loaded for ("" = none yet).
var channel_type: String = ""


## Replaces the catalog with the `data` of `listQuickMessages`. Entries without an id or a text are dropped.
func load_list(data: Variant, for_channel_type: String = "") -> void:
	items.clear()
	_by_id.clear()
	channel_type = for_channel_type
	if not data is Array:
		return
	for raw: Variant in data as Array:
		if not raw is Dictionary:
			continue
		var d: Dictionary = raw as Dictionary
		var it: Item = Item.new()
		it.id = DataRead.str_of(d, "quick_message_id").left(64)
		it.category = DataRead.str_of(d, "category")
		it.text = DataRead.text_of(d, "text")
		it.emoji = DataRead.str_of(d, "emoji").left(64)
		it.sort_order = maxi(0, DataRead.int_of(d, "sort_order"))
		if it.id.is_empty() or it.text.is_empty() or _by_id.has(it.id):
			continue
		items.append(it)
		_by_id[it.id] = it
	items.sort_custom(func(a: Item, b: Item) -> bool:
		return a.id < b.id if a.sort_order == b.sort_order else a.sort_order < b.sort_order)


func is_loaded() -> bool:
	return not items.is_empty()


func item(id: String) -> Item:
	return _by_id.get(id) as Item


## The sentence of `id` in `locale` (viewer's locale by default) with the message `params`; the placeholder key when
## the id is unknown.
func text_for(id: String, params: Dictionary = {}, locale: String = "") -> String:
	var it: Item = item(id)
	if it == null:
		return TranslationServer.translate(KEY_UNKNOWN)
	var sentence: String = L10n.pick(it.text, locale)
	return sentence.format(params) if not params.is_empty() else sentence


func emoji_for(id: String) -> String:
	var it: Item = item(id)
	return it.emoji if it != null else ""


func clear() -> void:
	items.clear()
	_by_id.clear()
	channel_type = ""
