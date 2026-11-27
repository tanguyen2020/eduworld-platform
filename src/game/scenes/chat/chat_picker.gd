class_name ChatPicker
extends VBoxContainer
## The smile board of the composer (F119-AC-08, AC-16, AC-17; CHAT_SYSTEM §7.5, STUDENT_GAME_UX_SOCIAL §7
## EmojiStickerGifPicker): three tabs Emoji / Sticker / GIF (icon + word on one line, never wrapping; the chosen tab
## yellow-blue like Cài đặt). One sideways-scrolling row per tab keeps it small on a phone in landscape. The board
## keeps the message list room of three messages (F119 "≥ 3 tin", STUDENT_GAME_UX_SOCIAL §7): in a wide panel the tabs
## sit on the content row ("Gõ chữ" · tabs · pictures — one row); in a narrow one the tabs are a row of their own above
## "Gõ chữ" · pictures (two rows).
## - Emoji: the platform emoji set as SVG pictures (no emoji characters — the Web build has no emoji font);
## - Sticker: the stickers of the unlocked packs of `listStickerPacks`; a locked pack shows a lock and the server's hint
##   to unlock it ("Hoàn thành nhiệm vụ … để mở bộ này");
## - GIF: topic chips built from the tags of the internal library (`listGifs?tag=`) — there is no search box, no URL
##   field and no Internet GIF service.
## Tabs the channel policy does not allow (no sticker / GIF, school preset quick + emote) are hidden (server data).
## UI only: the composer turns the picks into intents.

signal emoji_chosen(code: String)
signal sticker_chosen(sticker_id: String)
signal gif_chosen(gif_id: String)
## The student opened a tab (the dock loads stickers / GIFs on first use).
signal tab_opened(tab: StringName)
signal topic_chosen(tag: String)

const TAB_EMOJI: StringName = &"emoji"
const TAB_STICKER: StringName = &"sticker"
const TAB_GIF: StringName = &"gif"

## Touch targets of picture room the content keeps beside the tabs before they move to their own row (the arrows of
## the row and about three pictures).
const WIDE_CONTENT_TARGETS: float = 5.0

var current_tab: StringName = TAB_EMOJI
var current_topic: String = ""
## The tabs sit on the content row (wide panel) instead of their own row.
var wide: bool = false
var _tabs: HBoxContainer = null
var _tab_bar: ChipScroller = null
var _body: HBoxContainer = null
var _tab_buttons: Dictionary = {}
var _emoji_row: HBoxContainer = null
var _emoji_box: Control = null
## Left of the content: the composer's "Gõ chữ" (keeps the tab row on one line in a narrow panel).
var _side: VBoxContainer = null
var _stack: VBoxContainer = null
var _sticker_box: VBoxContainer = null
var _sticker_row: HBoxContainer = null
var _sticker_hints: VBoxContainer = null
var _gif_box: VBoxContainer = null
var _topic_row: HBoxContainer = null
var _gif_row: HBoxContainer = null
var _empty: Label = null
var _assets: ChatAssetCache = null


func _init() -> void:
	name = "ChatPicker"
	theme_type_variation = &"FieldBox"
	_build()
	resized.connect(_arrange)


func _notification(what: int) -> void:
	if what == NOTIFICATION_THEME_CHANGED or what == NOTIFICATION_TRANSLATION_CHANGED:
		_arrange.call_deferred()


## Wide panel: "Gõ chữ" · tabs · pictures on one row; narrow: the tabs on their own row above.
func _arrange() -> void:
	if _tab_bar == null or size.x <= 0.0:
		return
	var sep: float = float(_body.get_theme_constant(&"separation"))
	var tabs_w: float = _tabs.get_combined_minimum_size().x
	var room: float = _side.get_combined_minimum_size().x + tabs_w + UiBuild.target() * WIDE_CONTENT_TARGETS + sep * 2.0
	var next: bool = size.x >= room
	if next == wide and _tab_bar.get_parent() == (_body if wide else self):
		_tab_bar.custom_minimum_size.x = tabs_w if wide else 0.0
		return
	wide = next
	_tab_bar.get_parent().remove_child(_tab_bar)
	if wide:
		_body.add_child(_tab_bar)
		_body.move_child(_tab_bar, _side.get_index() + 1)
		_tab_bar.size_flags_horizontal = Control.SIZE_FILL
		_tab_bar.custom_minimum_size.x = tabs_w
	else:
		add_child(_tab_bar)
		move_child(_tab_bar, 0)
		_tab_bar.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_tab_bar.custom_minimum_size.x = 0.0


## Whether the tabs share the content row (wide panel).
func tabs_on_content_row() -> bool:
	return _tab_bar.get_parent() == _body


## Which tabs the channel allows (`allowed_kinds` of the server).
func set_kinds(sticker: bool, gif: bool) -> void:
	(_tab_buttons[TAB_STICKER] as Button).visible = sticker
	(_tab_buttons[TAB_GIF] as Button).visible = gif
	if (current_tab == TAB_STICKER and not sticker) or (current_tab == TAB_GIF and not gif):
		show_tab(TAB_EMOJI)


func show_tab(tab: StringName) -> void:
	current_tab = tab
	for t: Variant in _tab_buttons.keys():
		var b: Button = _tab_buttons[t] as Button
		b.theme_type_variation = &"ChatTabButtonSelected" if t == tab else &"ChatTabButton"
	_emoji_box.visible = tab == TAB_EMOJI
	_sticker_box.visible = tab == TAB_STICKER
	_sticker_hints.visible = tab == TAB_STICKER
	_gif_box.visible = tab == TAB_GIF
	_refresh_empty()
	tab_opened.emit(tab)


## Sticker packs as loaded by the service.
func refresh_stickers(assets: ChatAssetCache) -> void:
	_assets = assets
	UiBuild.clear(_sticker_row)
	UiBuild.clear(_sticker_hints)
	for pack: Dictionary in assets.packs:
		if not bool(pack["unlocked"]):
			var hint: HBoxContainer = UiBuild.hbox()
			hint.name = "Locked"
			hint.add_child(UiBuild.icon_rect("lock", 0.75))
			var words: String = TranslationServer.translate("ui.chat.picker.locked_pack").format({"pack": String(pack["name"]),
				"hint": String(pack["unlock_hint"])})
			var l: Label = UiBuild.data_label(words, &"MutedLabel")
			UiBuild.expand(l)
			hint.add_child(l)
			_sticker_hints.add_child(hint)
			continue
		for s: Dictionary in pack["stickers"] as Array:
			var b: Button = _picture_button(String(s["url"]), String(s["alt_text"]))
			b.name = "Sticker"
			b.pressed.connect(func() -> void: sticker_chosen.emit(String(s["sticker_id"])))
			_sticker_row.add_child(b)
	UiBuild.size_buttons(self)
	_refresh_empty()


## Topic chips and the GIFs of `tag` ("" = the first page).
func refresh_gifs(assets: ChatAssetCache, tag: String) -> void:
	_assets = assets
	UiBuild.clear(_topic_row)
	var group: ButtonGroup = ButtonGroup.new()
	for t: String in assets.topics():
		var chip: Button = UiBuild.chip("", group)
		chip.text = t
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.name = "Topic"
		chip.button_pressed = t == tag
		chip.pressed.connect(func() -> void:
			current_topic = t
			topic_chosen.emit(t))
		_topic_row.add_child(chip)
	UiBuild.mark_chips(_topic_row.get_children())
	UiBuild.clear(_gif_row)
	for g: Dictionary in assets.gifs(tag):
		var b: Button = _picture_button(String(g["preview_url"]), TranslationServer.translate("ui.chat.gif_label"))
		b.name = "Gif"
		b.pressed.connect(func() -> void: gif_chosen.emit(String(g["gif_id"])))
		_gif_row.add_child(b)
	UiBuild.size_buttons(self)
	_refresh_empty()


## An extra button left of the board content (the composer's "Gõ chữ").
func add_tab_extra(b: Button) -> void:
	_side.add_child(b)


func tab_button(tab: StringName) -> Button:
	return _tab_buttons.get(tab) as Button


func emoji_buttons() -> Array[Node]:
	return _emoji_row.get_children()


func sticker_buttons() -> Array[Node]:
	return _sticker_row.get_children()


func gif_buttons() -> Array[Node]:
	return _gif_row.get_children()


func topic_chips() -> Array[Node]:
	return _topic_row.get_children()


func locked_hints() -> Array[Node]:
	return _sticker_hints.get_children()


## The board has no free text field at all (no Internet search, no URL — F119-AC-16).
func has_text_field() -> bool:
	return not find_children("*", "LineEdit", true, false).is_empty() or not find_children("*", "TextEdit", true, false).is_empty()


func _picture_button(url: String, alt: String) -> Button:
	var b: Button = UiBuild.button("", "", &"IconButton")
	b.tooltip_text = alt
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.custom_minimum_size = Vector2.ONE * UiBuild.target() * 1.25
	b.expand_icon = true
	b.icon = UiBuild.icon("emote")
	if _assets != null:
		var tex: Texture2D = _assets.cached_texture(url)
		if tex != null:
			b.icon = tex
		elif _assets.is_allowed(url):
			_load_icon(b, url)
	return b


func _load_icon(b: Button, url: String) -> void:
	var tex: Texture2D = await _assets.texture(url)
	if tex != null and is_instance_valid(b):
		b.icon = tex


func _refresh_empty() -> void:
	var empty: bool = (current_tab == TAB_STICKER and _sticker_row.get_child_count() == 0 and _sticker_hints.get_child_count() == 0) \
			or (current_tab == TAB_GIF and _gif_row.get_child_count() == 0)
	_empty.visible = empty


func _build() -> void:
	# Tab chips on ONE line, never wrapping: icon + short word, as wide as their words; a much longer translation
	# scrolls sideways with the arrows instead of making the board taller.
	_tabs = UiBuild.hbox(&"ToolbarBox")
	_tabs.name = "Tabs"
	# Equal tabs across the row when they fit (the scroll box stretches an expanding row).
	UiBuild.expand(_tabs)
	for spec: Array in [[TAB_EMOJI, "ui.chat.picker.emoji", "tab_emoji"], [TAB_STICKER, "ui.chat.picker.sticker", "tab_sticker"],
			[TAB_GIF, "ui.chat.picker.gif", "tab_gif"]]:
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), &"ChatTabButton")
		b.name = "Tab" + String(spec[0]).capitalize()
		b.autowrap_mode = TextServer.AUTOWRAP_OFF
		UiBuild.expand(b)
		var tab: StringName = spec[0] as StringName
		b.pressed.connect(func() -> void: show_tab(tab))
		_tabs.add_child(b)
		_tab_buttons[tab] = b
	_tab_bar = UiBuild.chip_scroller(_tabs) as ChipScroller
	_tab_bar.name = "TabBar"
	add_child(_tab_bar)
	_body = UiBuild.hbox(&"ToolbarBox")
	_body.name = "Body"
	add_child(_body)
	_side = UiBuild.vbox()
	_side.name = "Side"
	_side.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_body.add_child(_side)
	_stack = UiBuild.vbox(&"FieldBox")
	_stack.name = "Stack"
	UiBuild.expand(_stack)
	_body.add_child(_stack)
	_emoji_row = UiBuild.hbox(&"ToolbarBox")
	_emoji_row.name = "Emoji"
	for code: String in ChatText.EMOJI_SET:
		var e: Button = UiBuild.button("", ChatText.emoji_icon(code), &"IconButton")
		e.name = "Emoji"
		e.tooltip_text = String(ChatText.EMOJI_KEYS[code])
		e.expand_icon = true
		e.pressed.connect(func() -> void: emoji_chosen.emit(code))
		_emoji_row.add_child(e)
	_emoji_box = UiBuild.chip_scroller(_emoji_row)
	_stack.add_child(_emoji_box)
	_sticker_box = UiBuild.vbox(&"FieldBox")
	_sticker_box.name = "Stickers"
	_stack.add_child(_sticker_box)
	_sticker_row = UiBuild.hbox(&"ToolbarBox")
	_sticker_box.add_child(UiBuild.chip_scroller(_sticker_row))
	# The hints of the locked packs run under the whole board ("Gõ chữ" included): fewer lines than beside it.
	_sticker_hints = UiBuild.vbox()
	_sticker_hints.name = "LockedHints"
	add_child(_sticker_hints)
	_gif_box = UiBuild.vbox(&"FieldBox")
	_gif_box.name = "Gifs"
	_stack.add_child(_gif_box)
	_topic_row = UiBuild.hbox(&"ToolbarBox")
	_gif_box.add_child(UiBuild.chip_scroller(_topic_row))
	_gif_row = UiBuild.hbox(&"ToolbarBox")
	_gif_box.add_child(UiBuild.chip_scroller(_gif_row))
	_empty = UiBuild.label("ui.chat.picker.empty", &"MutedLabel")
	_empty.name = "Empty"
	_stack.add_child(_empty)
	show_tab(TAB_EMOJI)
	# A hidden tab (channel policy), a translation or the "Gõ chữ" button changing width may move the tabs.
	_tabs.minimum_size_changed.connect(func() -> void: _arrange.call_deferred())
	_side.minimum_size_changed.connect(func() -> void: _arrange.call_deferred())
