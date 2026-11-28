class_name ChatBubble
extends Control
## One message of the chat list (F119-AC-18, AC-20; STUDENT_GAME_UX_SOCIAL §7 MessageBubble). A tier 2 bubble (thin ink
## outline, no bottom edge — F114): white for the others, blue for the student's own, yellow when it mentions the
## student (with the words "Nhắc tới em", never the colour alone). It shows only what the server sent: the masked text,
## the quick sentence in the viewer's language, an emoji / sticker / GIF picture, the reply or quote excerpt of the
## server ("Tin nhắn gốc không còn" when gone), "Đã chuyển tiếp từ {nickname}", "đã chỉnh sửa", the placeholder of a
## recalled / removed message, the reaction counts (the student's own in a yellow pill with a tick) and, on the
## student's messages, the state with an icon and a word: Đang gửi (clock) → Đã gửi (✓) → Đã nhận (✓✓) → Đã xem (✓✓
## blue). Long press (touch), right click, or the "⋯" button that appears on hover / focus open the message menu.
## Pooled and re-bound by ChatMessageList (only bubbles in view exist). UI only (AGENTS R1).

signal menu_requested(message: ChatMessageModel)

## Long press that opens the menu on a touch screen (STUDENT_GAME_UX_SOCIAL §7 "Nhấn giữ").
const LONG_PRESS_SEC: float = 0.45
## Share of the list width a bubble takes (own messages on the right, the others on the left).
const WIDTH_SHARE: float = 0.86
## Picture side of a sticker / GIF in touch targets.
const PICTURE_TARGETS: float = 2.2
const STATE_KEYS: Dictionary = {
	ChatMessageModel.LocalState.PENDING: "ui.chat.state.pending",
	ChatMessageModel.LocalState.SENT: "ui.chat.state.sent",
	ChatMessageModel.LocalState.DELIVERED: "ui.chat.state.delivered",
	ChatMessageModel.LocalState.READ: "ui.chat.state.read",
}
const STATE_ICONS: Dictionary = {
	ChatMessageModel.LocalState.PENDING: "clock",
	ChatMessageModel.LocalState.SENT: "tick_sent",
	ChatMessageModel.LocalState.DELIVERED: "tick_double",
	ChatMessageModel.LocalState.READ: "tick_double_read",
}

var message: ChatMessageModel = null
var self_id: String = ""
var catalog: QuickMessageCatalog = null
var assets: ChatAssetCache = null
var budget: ChatAnimationBudget = null
## F112-AC-17: the card reached from a search, lit for 2 s (ChatMessageList.flash).
var flashing: bool = false
var _panel: PanelContainer = null
var _box: VBoxContainer = null
var _head: HBoxContainer = null
var _name: Label = null
## The time beside the sender name (a message of another player: name and time on one line, F119 "≥ 3 tin").
var _head_time: Label = null
var _forwarded: Label = null
var _quote: PanelContainer = null
var _quote_label: Label = null
var _text: Label = null
var _picture: TextureRect = null
var _picture_tag: Label = null
var _held: HBoxContainer = null
var _reactions: HFlowContainer = null
var _meta: HBoxContainer = null
var _mention: Label = null
var _edited: Label = null
var _time: Label = null
var _state_icon: TextureRect = null
var _state: Label = null
var _menu: Button = null
var _press_left: float = -1.0
var _tween: Tween = null
var _playing: bool = false


func _init() -> void:
	name = "ChatBubble"
	mouse_filter = Control.MOUSE_FILTER_PASS
	_build()
	mouse_entered.connect(func() -> void: _menu.visible = message != null and not message.is_pending())
	mouse_exited.connect(func() -> void: _menu.visible = _menu.has_focus())


## Binds a message (pool re-use). `ctx`: {self_id, catalog, assets, budget}.
func bind(m: ChatMessageModel, ctx: Dictionary) -> void:
	message = m
	self_id = String(ctx.get("self_id", ""))
	catalog = ctx.get("catalog") as QuickMessageCatalog
	assets = ctx.get("assets") as ChatAssetCache
	budget = ctx.get("budget") as ChatAnimationBudget
	flashing = String(ctx.get("flash_key", "")) == m.key()
	_stop_animation()
	refresh()


func refresh() -> void:
	if message == null:
		return
	var m: ChatMessageModel = message
	var mine: bool = m.is_mine(self_id)
	var mentioned: bool = m.mentions.has(self_id) and not mine
	_panel.theme_type_variation = &"ChatBubbleMention" if mentioned or flashing else (&"ChatBubbleMine" if mine \
			else &"ChatBubbleOther")
	_name.text = ChatText.sender_text(m, self_id)
	_name.visible = not mine and not m.is_system()
	_head.visible = _name.visible
	_forwarded.visible = m.is_forwarded and not m.recalled
	_forwarded.text = ChatText.forwarded_text(m) if m.is_forwarded else ""
	var has_reply: bool = not m.reply_to_message_id.is_empty()
	var has_quote: bool = not m.quote_message_id.is_empty()
	_quote.visible = (has_reply or has_quote) and not m.recalled and not m.is_removed()
	if has_quote:
		_quote_label.text = TranslationServer.translate(ChatText.KEY_QUOTE).format({"text": ChatText.quote_text(m)})
	elif has_reply:
		_quote_label.text = TranslationServer.translate(ChatText.KEY_REPLY_TO).format({"text": ChatText.reply_text(m)})
	_bind_content(m)
	_held.visible = mine and m.is_held()
	_bind_reactions(m)
	_mention.visible = mentioned
	_edited.visible = m.edited_at_ms > 0 and not m.recalled and not m.is_removed()
	_time.text = ChatText.time_text(m.created_at_ms)
	# Another player: the time sits on the name line; the bottom line only shows "đã chỉnh sửa" / "Nhắc tới em" when
	# there is one (a typical message is two lines — STUDENT_GAME_UX_SOCIAL §7 "≥ 3 tin").
	_head_time.text = _time.text
	_head_time.visible = _name.visible and not _time.text.is_empty()
	_time.visible = not _time.text.is_empty() and not _head_time.visible
	_state_icon.visible = mine and not m.is_held()
	_state.visible = mine and not m.is_held()
	_meta.visible = _mention.visible or _edited.visible or _time.visible or _state.visible
	if mine:
		_state_icon.texture = UiBuild.icon(String(STATE_ICONS[m.local_state]))
		_state.text = String(STATE_KEYS[m.local_state])
	_menu.visible = false
	tooltip_text = ""
	update_minimum_size()


## The text a screen reader / the copy action would use (what the bubble shows).
func shown_text() -> String:
	return _text.text if _text.visible else ChatText.visible_text(message, catalog, assets)


func menu_button() -> Button:
	return _menu


func state_text() -> String:
	return _state.text if _state.visible else ""


func reaction_count() -> int:
	return _reactions.get_child_count()


func is_animating() -> bool:
	return _playing


func is_mention_style() -> bool:
	return _panel.theme_type_variation == &"ChatBubbleMention"


## Visible area check of the list: an animated sticker / GIF plays only while visible and a slot is free (F119-AC-17).
func update_animation(is_visible_now: bool) -> void:
	if message == null or not (message.kind in [ChatChannelModel.KIND_STICKER, ChatChannelModel.KIND_GIF]) or budget == null:
		return
	var play: bool = budget.request(message.key(), is_visible_now) and not Settings.reduce_motion
	if play and not _playing:
		_start_animation()
	elif not play and _playing:
		_stop_animation()


## Gives the animation slot back (recycled or scrolled away).
func release() -> void:
	_stop_animation()
	if budget != null and message != null:
		budget.release(message.key())
	message = null


func _get_minimum_size() -> Vector2:
	return _panel.get_combined_minimum_size() if _panel != null else Vector2.ZERO


func _gui_input(event: InputEvent) -> void:
	if message == null or message.is_pending():
		return
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.button_index == MOUSE_BUTTON_RIGHT and mb.pressed:
			accept_event()
			menu_requested.emit(message)
		elif mb.button_index == MOUSE_BUTTON_LEFT:
			_press_left = LONG_PRESS_SEC if mb.pressed else -1.0
	elif event is InputEventScreenTouch:
		_press_left = LONG_PRESS_SEC if (event as InputEventScreenTouch).pressed else -1.0
	elif event is InputEventScreenDrag or (event is InputEventMouseMotion and (event as InputEventMouseMotion).relative.length() > 8.0):
		_press_left = -1.0


func _process(delta: float) -> void:
	if _press_left < 0.0:
		return
	_press_left -= delta
	if _press_left <= 0.0:
		_press_left = -1.0
		if message != null:
			menu_requested.emit(message)


func _bind_content(m: ChatMessageModel) -> void:
	_picture.visible = false
	_picture_tag.visible = false
	_text.visible = true
	_text.theme_type_variation = &"ChatTextLabel"
	if m.recalled or m.is_removed():
		_text.text = ChatText.visible_text(m, catalog, assets)
		_text.theme_type_variation = &"ChatMetaLabel"
		return
	match m.kind:
		ChatChannelModel.KIND_EMOJI:
			_text.visible = false
			_picture.visible = true
			_picture.texture = UiBuild.icon(ChatText.emoji_icon(m.emoji))
			_picture.custom_minimum_size = Vector2.ONE * UiBuild.target() * 1.2
			_picture.tooltip_text = ChatText.emoji_name(m.emoji)
		ChatChannelModel.KIND_QUICK:
			_text.text = ChatText.visible_text(m, catalog, assets)
			var code: String = catalog.emoji_for(m.quick_message_id) if catalog != null else ""
			if not code.is_empty():
				_picture.visible = true
				_picture.texture = UiBuild.icon(ChatText.emoji_icon(code))
				_picture.custom_minimum_size = Vector2.ONE * float(UiTokens.COMPONENT_ICON_SIZE)
		ChatChannelModel.KIND_STICKER, ChatChannelModel.KIND_GIF:
			_bind_picture(m)
		ChatMessageModel.KIND_SYSTEM:
			_text.text = ChatText.visible_text(m, catalog, assets)
			_text.theme_type_variation = &"ChatMetaLabel"
		_:
			_text.text = m.body


## A sticker / GIF from the internal library (cached); the alt text shows while loading or when it fails.
func _bind_picture(m: ChatMessageModel) -> void:
	var gif: bool = m.kind == ChatChannelModel.KIND_GIF
	var info: Dictionary = {}
	var url: String = ""
	if assets != null:
		info = assets.gif(m.gif_id) if gif else assets.sticker(m.sticker_id)
		url = String(info.get("preview_url" if gif else "url", ""))
	_text.text = ChatText.visible_text(m, catalog, assets)
	_text.theme_type_variation = &"ChatMetaLabel"
	_picture_tag.visible = gif
	var side: float = UiBuild.target() * PICTURE_TARGETS
	_picture.custom_minimum_size = Vector2(side, side)
	var tex: Texture2D = assets.cached_texture(url) if assets != null else null
	if tex != null:
		_show_picture(tex)
	elif assets != null and assets.is_allowed(url):
		_load_picture(m.key(), url)


func _load_picture(key: String, url: String) -> void:
	var tex: Texture2D = await assets.texture(url)
	if tex != null and message != null and message.key() == key:
		_show_picture(tex)


func _show_picture(tex: Texture2D) -> void:
	_picture.texture = tex
	_picture.visible = true
	_picture.tooltip_text = _text.text
	_text.visible = false
	update_minimum_size()


func _bind_reactions(m: ChatMessageModel) -> void:
	UiBuild.clear(_reactions)
	var codes: Array = m.reactions.keys()
	codes.sort()
	for code: Variant in codes:
		var mine: bool = String(code) in m.my_reactions
		var pill: PanelContainer = UiBuild.panel(&"ReactionPillMine" if mine else &"ReactionPill")
		pill.tooltip_text = ChatText.emoji_name(String(code))
		var row: HBoxContainer = UiBuild.hbox()
		pill.add_child(row)
		var icon: TextureRect = UiBuild.icon_rect(ChatText.emoji_icon(String(code)), 0.75)
		row.add_child(icon)
		row.add_child(UiBuild.data_label(str(int(m.reactions[code])), &"ReactionLabel", false))
		if mine:
			row.add_child(UiBuild.icon_rect("tick", 0.6))
		_reactions.add_child(pill)
	_ignore_mouse(_reactions)
	_reactions.visible = _reactions.get_child_count() > 0


func _start_animation() -> void:
	_playing = true
	_picture.pivot_offset = _picture.size / 2.0
	_tween = create_tween().set_loops()
	_tween.tween_property(_picture, "scale", Vector2(1.06, 1.06), 0.45).set_trans(Tween.TRANS_SINE)
	_tween.tween_property(_picture, "scale", Vector2.ONE, 0.45).set_trans(Tween.TRANS_SINE)


func _stop_animation() -> void:
	_playing = false
	if _tween != null:
		_tween.kill()
		_tween = null
	if _picture != null:
		_picture.scale = Vector2.ONE


func _build() -> void:
	_panel = UiBuild.panel(&"ChatBubbleOther")
	_panel.name = "Bubble"
	_panel.set_anchors_preset(Control.PRESET_FULL_RECT)
	_panel.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_panel)
	_box = UiBuild.vbox()
	_box.name = "Box"
	_box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_panel.add_child(_box)
	_head = UiBuild.hbox()
	_head.name = "Head"
	_head.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_box.add_child(_head)
	_name = UiBuild.data_label("", &"ChatNameLabel", false)
	_name.name = "Sender"
	_name.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	UiBuild.expand(_name)
	_head.add_child(_name)
	_head_time = UiBuild.data_label("", &"ChatMetaLabel", false)
	_head_time.name = "HeadTime"
	_head_time.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_head.add_child(_head_time)
	_forwarded = UiBuild.data_label("", &"ChatMetaLabel")
	_forwarded.name = "Forwarded"
	_box.add_child(_forwarded)
	_quote = UiBuild.panel(&"ChatQuoteBox")
	_quote.name = "Quote"
	_quote.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_box.add_child(_quote)
	_quote_label = UiBuild.data_label("", &"ChatMetaLabel")
	_quote_label.name = "QuoteText"
	_quote_label.max_lines_visible = 2
	_quote_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_quote.add_child(_quote_label)
	var content: HBoxContainer = UiBuild.hbox()
	content.name = "Content"
	content.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_box.add_child(content)
	_picture = TextureRect.new()
	_picture.name = "Picture"
	_picture.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_picture.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	_picture.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_picture.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	content.add_child(_picture)
	_text = UiBuild.data_label("", &"ChatTextLabel")
	_text.name = "Text"
	UiBuild.expand(_text)
	content.add_child(_text)
	_picture_tag = UiBuild.label("ui.chat.gif_tag", &"SectionTag", false)
	_picture_tag.name = "GifTag"
	_picture_tag.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
	content.add_child(_picture_tag)
	_held = UiBuild.hbox()
	_held.name = "Held"
	_held.add_child(UiBuild.icon_rect("clock", 0.7))
	_held.add_child(UiBuild.label(ChatText.KEY_HELD_OWN, &"ChatMetaLabel"))
	_box.add_child(_held)
	_reactions = HFlowContainer.new()
	_reactions.name = "Reactions"
	_reactions.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_box.add_child(_reactions)
	_meta = UiBuild.hbox()
	_meta.name = "Meta"
	_meta.alignment = BoxContainer.ALIGNMENT_END
	_meta.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_box.add_child(_meta)
	_mention = UiBuild.label("ui.chat.mentioned_you", &"ChatMetaLabel", false)
	_mention.name = "Mention"
	_meta.add_child(_mention)
	_edited = UiBuild.label(ChatText.KEY_EDITED, &"ChatMetaLabel", false)
	_edited.name = "Edited"
	_meta.add_child(_edited)
	_time = UiBuild.data_label("", &"ChatMetaLabel", false)
	_time.name = "Time"
	_meta.add_child(_time)
	_state_icon = UiBuild.icon_rect("tick_sent", 0.6)
	_state_icon.name = "StateIcon"
	_state_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_meta.add_child(_state_icon)
	_state = UiBuild.label("", &"ChatMetaLabel", false)
	_state.name = "State"
	_meta.add_child(_state)
	_menu = UiBuild.button("", "menu", &"IconButton")
	_menu.name = "MenuButton"
	_menu.tooltip_text = "ui.chat.menu.open"
	_menu.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	_menu.grow_horizontal = Control.GROW_DIRECTION_BEGIN
	_menu.visible = false
	_menu.pressed.connect(func() -> void: if message != null: menu_requested.emit(message))
	_menu.focus_entered.connect(func() -> void: _menu.visible = true)
	add_child(_menu)
	_ignore_mouse(_panel)
	_panel.minimum_size_changed.connect(update_minimum_size)


## Only the bubble itself takes the pointer (long press, right click); its parts never swallow it.
static func _ignore_mouse(root: Control) -> void:
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	for c: Node in root.find_children("*", "Control", true, false):
		(c as Control).mouse_filter = Control.MOUSE_FILTER_IGNORE
