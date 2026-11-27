class_name ChatComposer
extends VBoxContainer
## The writing area of the chat panel (F119-AC-08, AC-09, AC-11; STUDENT_GAME_UX_SOCIAL §7 QuickMessageBar + Composer):
## - the quick-message row is always above the input — one tap sends `quick_message_id`; the words follow the viewer's
##   language; `g1_3` gets taller buttons and the smile board open by default (big pictures instead of the keyboard);
## - the smile board and the text row take turns ("bảng emoji thay vì bàn phím"): the smile button opens the board,
##   "Gõ chữ" on the board brings the text field back — the panel stays short enough for a phone in landscape;
## - the text field counts characters against the channel `max_text_length`, `Enter` sends and `Shift+Enter` breaks the
##   line on a PC; `@` lists the nicknames of the members (mentions are user ids, server-checked);
## - "Đang trả lời / Đang trích dẫn / Đang sửa tin" strip with a cancel button;
## - locked (sanction, channel closed, curfew) → the input and buttons are disabled with the reason in words; reading
##   goes on. A school preset quick + emote (server `allowed_kinds`) hides the free text row — no client flag (R15).
## UI only: it emits intents, the dock calls ChatService (AGENTS R1).

signal text_submitted(text: String, reply_to: String, quote: Dictionary, mention_ids: PackedStringArray)
signal edit_submitted(message: ChatMessageModel, text: String)
signal quick_chosen(quick_message_id: String)
signal emoji_chosen(code: String)
signal sticker_chosen(sticker_id: String)
signal gif_chosen(gif_id: String)
## Typing started (true) / stopped (false) — throttled by the service.
signal typing_changed(active: bool)
signal picker_tab_opened(tab: StringName)
signal gif_topic_chosen(tag: String)

## `g1_3` quick-message buttons are this much taller than the band touch target (STUDENT_GAME_UX_SOCIAL §7).
const G13_QUICK_SCALE: float = 1.2
## Mention suggestions shown at once.
const MENTION_SHOWN: int = 5
## Visible lines of the text field.
const INPUT_LINES: float = 1.6


## Multi-line field: Enter sends, Shift+Enter keeps the new line (PC).
class ChatInput:
	extends TextEdit
	signal submit_requested()

	func _gui_input(event: InputEvent) -> void:
		var key: InputEventKey = event as InputEventKey
		if key != null and key.pressed and not key.echo and (key.keycode == KEY_ENTER or key.keycode == KEY_KP_ENTER) \
				and not key.shift_pressed:
			accept_event()
			submit_requested.emit()


var channel: ChatChannelModel = null
var catalog: QuickMessageCatalog = null
## `{user_id, nickname}` of the members (mention list); never the student.
var members: Array[Dictionary] = []
var locked: bool = false
## Quick messages locked too (a mute; a `restrict_text` sanction keeps them).
var quick_locked: bool = false
var reply_to: ChatMessageModel = null
var quote_of: ChatMessageModel = null
var editing: ChatMessageModel = null
var _context_bar: PanelContainer = null
var _context_label: Label = null
var _quick_row: HBoxContainer = null
var _quick_scroller: Control = null
var _picker: ChatPicker = null
var _input_row: HBoxContainer = null
var _smile: Button = null
var _keyboard: Button = null
## The smile board is shown instead of the text row.
var board: bool = false
var _input: ChatInput = null
var _counter: Label = null
var _send: Button = null
var _mentions: VBoxContainer = null
var _lock: PanelContainer = null
var _lock_label: Label = null
var _mention_ids: Dictionary = {}
var _typing: bool = false


func _init() -> void:
	name = "ChatComposer"
	theme_type_variation = &"FieldBox"
	_build()


## Binds the channel facts and the quick-message catalog (re-run when they change).
func configure(next: ChatChannelModel, quick_catalog: QuickMessageCatalog) -> void:
	channel = next
	catalog = quick_catalog
	refresh_quick()
	var text_allowed: bool = channel == null or channel.allows(ChatChannelModel.KIND_TEXT)
	_picker.set_kinds(channel != null and channel.allows(ChatChannelModel.KIND_STICKER),
			channel != null and channel.allows(ChatChannelModel.KIND_GIF))
	(_picker.tab_button(ChatPicker.TAB_EMOJI) as Button).visible = channel == null or channel.allows(ChatChannelModel.KIND_EMOJI)
	if not text_allowed:
		board = true
	_apply_board()
	_update_counter()


## The quick-message row in the viewer's language (rebuilt on a locale change).
func refresh_quick() -> void:
	UiBuild.clear(_quick_row)
	# F125: on the compact phone canvas every button is already the compact touch target; the list keeps its 3 messages.
	var big: bool = UiBuild.band() == "g1_3" and not AppState.compact
	if catalog == null:
		return
	for it: QuickMessageCatalog.Item in catalog.items:
		var b: Button = UiBuild.button("", ChatText.emoji_icon(it.emoji) if not it.emoji.is_empty() else "emote", &"SecondaryButton")
		b.name = "Quick"
		b.text = catalog.text_for(it.id)
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.set_meta("quick_message_id", it.id)
		b.disabled = quick_locked
		if big:
			b.custom_minimum_size.y = UiBuild.target() * G13_QUICK_SCALE
		b.pressed.connect(func() -> void: quick_chosen.emit(it.id))
		_quick_row.add_child(b)
	_quick_row.visible = channel == null or channel.allows(ChatChannelModel.KIND_QUICK)


## Locks writing with the reason (`key` + args); "" unlocks. `allow_quick`: a `restrict_text` sanction still lets the
## student send quick messages (MODERATION §7).
func set_lock(key: String, args: Dictionary = {}, allow_quick: bool = false) -> void:
	locked = not key.is_empty()
	_lock.visible = locked
	_lock_label.text = TranslationServer.translate(key).format(args) if locked else ""
	_input.editable = not locked
	_send.disabled = locked
	_send.icon = UiBuild.icon("send_blue" if _send.disabled else "send")
	_smile.disabled = locked
	quick_locked = locked and not allow_quick
	for b: Node in _quick_row.get_children():
		(b as Button).disabled = quick_locked
	if locked:
		cancel_context()
	_apply_board()
	_update_counter()


## The smile board opens by default for g1_3 (AC-08), closed for the other bands until the smile is pressed. F125: on a
## phone (compact canvas 540 tall) it starts closed for every band so three messages stay readable (F119 "≥ 3 tin"); the
## big quick messages stay above the input and the smile button opens the board with one tap.
func apply_band(band: String) -> void:
	var text_off: bool = channel != null and not channel.allows(ChatChannelModel.KIND_TEXT)
	board = (band == "g1_3" and not AppState.compact) or text_off
	_apply_board()
	refresh_quick()


## Board ⇄ text row. Locked: neither (reading only, the lock notice explains).
func set_board(on: bool) -> void:
	board = on
	_apply_board()
	if not on:
		focus_input()


func _apply_board() -> void:
	var text_allowed: bool = channel == null or channel.allows(ChatChannelModel.KIND_TEXT)
	_picker.visible = board and not locked
	_input_row.visible = text_allowed and not (board and not locked)
	_keyboard.visible = text_allowed


func set_members(list: Array[Dictionary]) -> void:
	members = list


func start_reply(m: ChatMessageModel, excerpt: String) -> void:
	cancel_context()
	reply_to = m
	_show_context(TranslationServer.translate("ui.chat.replying_to").format({"nickname": m.sender_nickname,
		"text": excerpt.left(60)}))
	focus_input()


func start_quote(m: ChatMessageModel) -> void:
	cancel_context()
	quote_of = m
	_show_context(TranslationServer.translate("ui.chat.quoting").format({"nickname": m.sender_nickname,
		"text": m.body.left(60)}))
	focus_input()


func start_edit(m: ChatMessageModel) -> void:
	cancel_context()
	editing = m
	_show_context(TranslationServer.translate("ui.chat.editing"))
	_input.text = m.body
	_input.set_caret_column(_input.text.length())
	_update_counter()
	focus_input()


func cancel_context() -> void:
	reply_to = null
	quote_of = null
	editing = null
	_context_bar.visible = false


## A refused text comes back so the student can rewrite it.
func restore_draft(text: String) -> void:
	if _input.text.strip_edges().is_empty():
		_input.text = text
		_input.set_caret_column(_input.text.length())
		_update_counter()


func focus_input() -> void:
	if _input_row.visible and _input.editable and is_inside_tree() and is_visible_in_tree():
		_input.grab_focus.call_deferred()


func input() -> TextEdit:
	return _input


func send_button() -> Button:
	return _send


func smile_button() -> Button:
	return _smile


func picker() -> ChatPicker:
	return _picker


func quick_buttons() -> Array[Node]:
	return _quick_row.get_children()


func counter_text() -> String:
	return _counter.text


func context_text() -> String:
	return _context_label.text if _context_bar.visible else ""


func lock_text() -> String:
	return _lock_label.text if _lock.visible else ""


func mention_buttons() -> Array[Node]:
	return _mentions.get_children()


func text_row_visible() -> bool:
	return _input_row.visible


## Sends the text (Enter / "Gửi"): an edit when editing, else a new message with the reply / quote / mentions.
func submit() -> void:
	if locked:
		return
	var text: String = _input.text.strip_edges()
	if text.is_empty() or _over_limit():
		return
	if editing != null:
		edit_submitted.emit(editing, text)
	else:
		var quote: Dictionary = {}
		if quote_of != null:
			quote = {"message_id": quote_of.message_id, "excerpt_start": 0, "excerpt_end": quote_of.body.length()}
		text_submitted.emit(text, reply_to.message_id if reply_to != null else "", quote, _mentioned(text))
	_input.text = ""
	_mention_ids.clear()
	cancel_context()
	_update_counter()
	_set_typing(false)
	_mentions.visible = false


func _over_limit() -> bool:
	return channel != null and _input.text.strip_edges().length() > channel.text_limit()


func _update_counter() -> void:
	var limit: int = channel.text_limit() if channel != null else ChatChannelModel.BODY_MAX
	var count: int = _input.text.strip_edges().length()
	var over: bool = count > limit
	_counter.text = TranslationServer.translate("ui.chat.counter_over" if over else "ui.chat.counter").format(
			{"count": count, "max": limit})
	_counter.theme_type_variation = &"InlineErrorLabel" if over else &"ChatMetaLabel"
	UiBuild.fit_words(_counter)
	# F125: on the compact phone canvas the counter shows once the student types (the list keeps its 3 messages).
	_counter.visible = count > 0 or not AppState.compact
	_send.disabled = locked or count == 0 or over
	# The white plane is for the blue face; the grey disabled button shows the blue one so it stays readable.
	_send.icon = UiBuild.icon("send_blue" if _send.disabled else "send")


func _on_text_changed() -> void:
	_update_counter()
	_set_typing(not _input.text.strip_edges().is_empty())
	_update_mentions()


func _set_typing(active: bool) -> void:
	if active != _typing or active:
		_typing = active
		typing_changed.emit(active)


## `@` + letters before the caret → the matching member nicknames.
func _update_mentions() -> void:
	UiBuild.clear(_mentions)
	var line: String = _input.get_line(_input.get_caret_line()).left(_input.get_caret_column())
	var at: int = line.rfind("@")
	var word: String = line.substr(at + 1) if at >= 0 else ""
	if at < 0 or word.contains(" ") or members.is_empty():
		_mentions.visible = false
		return
	for m: Dictionary in members:
		var nick: String = String(m["nickname"])
		if word.is_empty() or nick.to_lower().begins_with(word.to_lower()):
			var b: Button = UiBuild.button("", "", &"TextButton")
			b.text = "@" + nick
			b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			b.name = "Mention"
			b.pressed.connect(_insert_mention.bind(String(m["user_id"]), nick, word.length()))
			_mentions.add_child(b)
			if _mentions.get_child_count() >= MENTION_SHOWN:
				break
	_mentions.visible = _mentions.get_child_count() > 0
	UiBuild.size_buttons(_mentions)


func _insert_mention(user_id: String, nickname: String, typed: int) -> void:
	for i: int in typed:
		_input.backspace()
	_input.insert_text_at_caret(nickname + " ")
	_mention_ids[nickname] = user_id
	_mentions.visible = false
	focus_input()


func _mentioned(text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	for nick: Variant in _mention_ids.keys():
		if text.contains("@" + String(nick)) and not String(_mention_ids[nick]) in out:
			out.append(String(_mention_ids[nick]))
	return out


func _show_context(text: String) -> void:
	_context_label.text = text
	_context_bar.visible = true


func _build() -> void:
	_context_bar = UiBuild.panel(&"InfoBannerPanel")
	_context_bar.name = "ContextBar"
	_context_bar.visible = false
	add_child(_context_bar)
	var ctx_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_context_bar.add_child(ctx_row)
	ctx_row.add_child(UiBuild.icon_rect("reply", 0.75))
	_context_label = UiBuild.data_label("", &"InfoBannerLabel")
	_context_label.name = "ContextText"
	_context_label.max_lines_visible = 2
	_context_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	UiBuild.expand(_context_label)
	ctx_row.add_child(_context_label)
	var cancel: Button = UiBuild.button("", "cross", &"IconButton")
	cancel.name = "CancelContext"
	cancel.tooltip_text = "ui.chat.cancel_context"
	cancel.pressed.connect(func() -> void:
		var was_edit: bool = editing != null
		cancel_context()
		if was_edit:
			_input.text = ""
			_update_counter())
	ctx_row.add_child(cancel)
	_quick_row = UiBuild.hbox(&"ToolbarBox")
	_quick_row.name = "QuickRow"
	_quick_scroller = UiBuild.chip_scroller(_quick_row)
	_quick_scroller.name = "QuickBar"
	add_child(_quick_scroller)
	_picker = ChatPicker.new()
	_picker.visible = false
	_picker.emoji_chosen.connect(func(code: String) -> void: emoji_chosen.emit(code))
	_picker.sticker_chosen.connect(func(id: String) -> void: sticker_chosen.emit(id))
	_picker.gif_chosen.connect(func(id: String) -> void: gif_chosen.emit(id))
	_picker.tab_opened.connect(func(tab: StringName) -> void: picker_tab_opened.emit(tab))
	_picker.topic_chosen.connect(func(tag: String) -> void: gif_topic_chosen.emit(tag))
	add_child(_picker)
	_keyboard = UiBuild.button("ui.chat.keyboard", "edit", &"SecondaryButton")
	_keyboard.name = "Keyboard"
	_keyboard.pressed.connect(func() -> void: set_board(false))
	_picker.add_tab_extra(_keyboard)
	_mentions = UiBuild.vbox()
	_mentions.name = "Mentions"
	_mentions.visible = false
	add_child(_mentions)
	_input_row = UiBuild.hbox(&"ToolbarBox")
	_input_row.name = "InputRow"
	add_child(_input_row)
	_smile = UiBuild.button("", "emoji_smile", &"IconButton")
	_smile.name = "Smile"
	_smile.tooltip_text = "ui.chat.picker.open"
	_smile.pressed.connect(func() -> void: set_board(true))
	_input_row.add_child(_smile)
	var field: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(field)
	_input_row.add_child(field)
	_input = ChatInput.new()
	_input.name = "Input"
	_input.placeholder_text = "ui.chat.input_placeholder"
	_input.wrap_mode = TextEdit.LINE_WRAPPING_BOUNDARY
	_input.scroll_fit_content_height = false
	_input.custom_minimum_size.y = UiBuild.target()
	UiBuild.expand(_input)
	_input.text_changed.connect(_on_text_changed)
	_input.submit_requested.connect(submit)
	field.add_child(_input)
	# F125: the counter wraps between its words (long translations) so the input row fits a narrow phone column.
	_counter = UiBuild.keep_words(UiBuild.data_label("", &"ChatMetaLabel"))
	_counter.name = "Counter"
	_counter.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	field.add_child(_counter)
	_send = UiBuild.button("ui.chat.send", "send")
	_send.name = "Send"
	_send.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
	_send.pressed.connect(submit)
	_input_row.add_child(_send)
	_input.resized.connect(_fit_send)
	_lock = UiBuild.panel(&"WarningBannerPanel")
	_lock.name = "Lock"
	_lock.visible = false
	add_child(_lock)
	var lock_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_lock.add_child(lock_row)
	lock_row.add_child(UiBuild.icon_rect("lock", 0.8))
	_lock_label = UiBuild.data_label("", &"WarningBannerLabel")
	_lock_label.name = "LockText"
	UiBuild.expand(_lock_label)
	lock_row.add_child(_lock_label)
	_update_counter()


## F125 polish: in a narrow chat column (the match column on a phone, a long translation) the input would cut its hint
## ("Nhập tin nhắn…" → "Nhập tin"): the send button then shows its paper plane alone — its word stays as tooltip and
## accessible name — and the input gets that room. Decided from the widths with the word shown (no flicker).
func _fit_send() -> void:
	if _input == null or _send == null:
		return
	var font: Font = _input.get_theme_font(&"font")
	var send_font: Font = _send.get_theme_font(&"font")
	if font == null or send_font == null:
		return
	var word: String = tr("ui.chat.send")
	var box: StyleBox = _input.get_theme_stylebox(&"normal")
	var pad: float = box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT) if box != null else 0.0
	# The input keeps room for its scroll bar and the caret.
	pad += _input.get_v_scroll_bar().get_combined_minimum_size().x + float(UiTokens.SIZE_SPACE_8)
	var hint: float = font.get_string_size(tr(_input.placeholder_text), HORIZONTAL_ALIGNMENT_LEFT, -1.0,
			_input.get_theme_font_size(&"font_size")).x + pad
	var label: float = send_font.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1.0,
			_send.get_theme_font_size(&"font_size")).x + float(_send.get_theme_constant(&"h_separation"))
	var with_word: float = _input.size.x - (label if _send.text.is_empty() else 0.0)
	var show_word: bool = with_word >= hint
	var shown: bool = not _send.text.is_empty()
	if show_word == shown:
		return
	_send.text = "ui.chat.send" if show_word else ""
	_send.tooltip_text = "" if show_word else "ui.chat.send"
	_send.accessibility_name = "" if show_word else word
