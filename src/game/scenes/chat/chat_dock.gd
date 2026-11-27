class_name ChatDock
extends Control
## The one embed point of the gameplay chat (F119 US1–US4; plan "ChatDock"; CHAT_SYSTEM §4.2, STUDENT_GAME_UX_SOCIAL
## §5 EmbeddedChat, §7): the party lobby (F102), the match / team battle (F109) and the boss screen (F110) mount it and
## give it their context — `show_party(party_id, chat_channel_id, members)` or `show_match(match_id, chat_channel_ids,
## members)` — without knowing anything else of the chat.
## - Always there in party / match (ADR-0020 §6): it can be collapsed (header + unread badge, "@" badge) or shrink to
##   a 2-line strip, never turned off — there is no such option anywhere (F119-AC-01, AC-23).
## - Team battle: two tabs "Đội" (own `team` channel) and "Cả trận" (`match`), each with its own unread badge (AC-02).
## - While a question is on screen (`EventBus.question_shown`) it shrinks to the 2-line strip and grows back after;
##   messages keep arriving and are kept (AC-03).
## - Banners: "Đang kết nối lại chat…" (pending messages with a clock, AC-05), "Đang đồng bộ tin nhắn…" (a gap sync
##   longer than 2 s), the server's moderation feedback in friendly words (AC-10; the care message stays until closed),
##   the composer lock with the time writing opens again (AC-11), the end of the match with the days the messages stay
##   readable from the server `purge_after` (AC-06, AC-07).
## - The message menu, the report sheet, the players list (Báo cáo người chơi / Chặn) go through ChatDialogs.
## - `external_toggle`: a host with its own chat button (phone lobby) hides the collapsed dock and shows its own badge
##   from `unread_changed`.
## Typing and receipt redraws are coalesced to ≤ 1 per second (AC-21). UI only: ChatService does the work (R1).

signal mode_changed(mode: int)
signal unread_changed(count: int, mentions: int)

enum Mode { EXPANDED, COLLAPSED, MINI }

const CONTEXT_PARTY: StringName = &"party"
const CONTEXT_MATCH: StringName = &"match"
## Seconds a notice stays (≥ 3 s reading time, STUDENT_GAME_UX §4); the care message stays until closed.
const NOTICE_SEC: float = 5.0
## Typing / receipt redraws at most once a second (STUDENT_GAME_UX_SOCIAL §7 "gộp cập nhật 1 lần/giây").
const COALESCE_SEC: float = 1.0
const MINI_LINES: int = 2
const NOTICE_PANELS: Dictionary = {
	ChatFeedbackMapper.TONE_INFO: [&"InfoBannerPanel", &"InfoBannerLabel", "info"],
	ChatFeedbackMapper.TONE_WARNING: [&"WarningBannerPanel", &"WarningBannerLabel", "alert"],
}

var service: ChatService = null
var social: SocialService = null
var reports: ReportService = null
var mode: Mode = Mode.EXPANDED
var external_toggle: bool = false
var auto_shrink: bool = true
var context_kind: StringName = &""
var context_id: String = ""
## Channels of the tabs (team first, then match; one channel = no tabs).
var channel_ids: PackedStringArray = []
var current_channel: String = ""
## `{user_id, nickname, avatar_config?}` of the other players (mentions, players list).
var members: Array[Dictionary] = []
var _mode_before_question: Mode = Mode.EXPANDED
var _question_on: bool = false
var _notice_left: float = 0.0
var _coalesce_left: float = 0.0
var _dirty_typing: bool = false
var _dirty_receipts: bool = false
var _kb_offset: float = 0.0
var _panel: PanelContainer = null
var _box: VBoxContainer = null
var _header: HBoxContainer = null
var _head_icon: TextureRect = null
var _spacer: Control = null
var _title: Label = null
var _badge: Label = null
var _mention_badge: Label = null
var _players: Button = null
var _collapse: Button = null
var _expand: Button = null
var _tabs: HBoxContainer = null
var _tab_bar: ChipScroller = null
var _tab_buttons: Dictionary = {}
var _tab_badges: Dictionary = {}
var _banner: PanelContainer = null
var _banner_label: Label = null
var _notice: PanelContainer = null
var _notice_icon: TextureRect = null
var _notice_label: Label = null
var _care: PanelContainer = null
var _care_label: Label = null
var _pins: VBoxContainer = null
var _list_box: Control = null
var _list: ChatMessageList = null
var _empty: Label = null
var _typing: Label = null
var _retention: HBoxContainer = null
var _retention_label: Label = null
var _composer: ChatComposer = null
var _mini: Label = null
var _sheet: ChatSheet = null
var _dialogs: ChatDialogs = null


func _ready() -> void:
	_build()
	EventBus.question_shown.connect(_on_question_shown)
	EventBus.question_hidden.connect(_on_question_hidden)
	EventBus.locale_changed.connect(_on_locale_changed)
	EventBus.density_changed.connect(_on_density_changed)
	if service == null and AppState.services != null:
		var s: ServiceLocator = AppState.get_services()
		bind_services(s.chat, s.social, s.reports)
	_apply_mode()


## Moving the dock (the phone lobby puts it in its overlay) keeps everything; only freeing it leaves the channels.
func _cleanup() -> void:
	for pair: Array in [[EventBus.question_shown, _on_question_shown], [EventBus.question_hidden, _on_question_hidden],
			[EventBus.locale_changed, _on_locale_changed], [EventBus.density_changed, _on_density_changed]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	_unbind()
	clear_context()


func bind_services(chat: ChatService, social_service: SocialService, report_service: ReportService) -> void:
	_unbind()
	service = chat
	social = social_service
	reports = report_service
	if _dialogs != null:
		_dialogs.bind(chat, social_service, report_service)
	if service == null:
		return
	var st: ChatStore = service.store
	st.messages_changed.connect(_on_messages_changed)
	st.message_updated.connect(_on_message_updated)
	st.channel_changed.connect(_on_channel_changed)
	st.typing_changed.connect(_on_typing_changed)
	st.receipts_changed.connect(_on_receipts_changed)
	st.unread_changed.connect(_render_badges)
	st.sanction_changed.connect(_refresh_lock)
	service.feedback_requested.connect(show_notice)
	service.connection_changed.connect(_on_connection_changed)
	service.draft_returned.connect(_on_draft_returned)
	service.quick_messages_loaded.connect(_on_quick_loaded)
	service.stickers_loaded.connect(func() -> void: _composer.picker().refresh_stickers(service.assets))
	service.gifs_loaded.connect(func(tag: String) -> void: _composer.picker().refresh_gifs(service.assets, tag))
	_list.context = {"self_id": st.self_id, "catalog": service.quick, "assets": service.assets, "budget": service.budget}


func _unbind() -> void:
	if service == null:
		return
	var st: ChatStore = service.store
	for pair: Array in [[st.messages_changed, _on_messages_changed], [st.message_updated, _on_message_updated],
			[st.channel_changed, _on_channel_changed], [st.typing_changed, _on_typing_changed],
			[st.receipts_changed, _on_receipts_changed], [st.unread_changed, _render_badges],
			[st.sanction_changed, _refresh_lock], [service.feedback_requested, show_notice],
			[service.connection_changed, _on_connection_changed], [service.draft_returned, _on_draft_returned],
			[service.quick_messages_loaded, _on_quick_loaded]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)


# ---------- context ----------

## Party lobby: the always-on `party` channel of the party (`chat_channel_id` of the party snapshot).
func show_party(party_id: String, channel_id: String, others: Array[Dictionary] = []) -> void:
	_open(CONTEXT_PARTY, party_id, [[channel_id, ChatChannelModel.TYPE_PARTY]], others)


## Match: `match_channel_id` ("Cả trận") and, in a team battle, the student's `team_channel_id` ("Đội").
func show_match(match_id: String, match_channel_id: String, team_channel_id: String = "",
		others: Array[Dictionary] = []) -> void:
	var list: Array = []
	if Uuid.is_valid(team_channel_id):
		list.append([team_channel_id, ChatChannelModel.TYPE_TEAM])
	list.append([match_channel_id, ChatChannelModel.TYPE_MATCH])
	_open(CONTEXT_MATCH, match_id, list, others)


func set_members(others: Array[Dictionary]) -> void:
	members = others
	if _composer != null:
		_composer.set_members(others)
	_players.visible = not others.is_empty()


## The host leaves the party / match screen.
func clear_context() -> void:
	if service != null:
		for id: String in channel_ids:
			service.leave_channel(id)
	channel_ids.clear()
	current_channel = ""
	context_kind = &""
	context_id = ""


func _open(kind: StringName, id: String, list: Array, others: Array[Dictionary]) -> void:
	var ids: PackedStringArray = []
	for pair: Array in list:
		if Uuid.is_valid(String(pair[0])):
			ids.append(String(pair[0]))
	if ids == channel_ids and kind == context_kind and id == context_id:
		set_members(others)
		return
	clear_context()
	context_kind = kind
	context_id = id
	channel_ids = ids
	set_members(others)
	_build_tabs()
	_title.text = "ui.chat.title.match" if kind == CONTEXT_MATCH else "ui.chat.title.party"
	if service == null or ids.is_empty():
		return
	for pair: Array in list:
		if Uuid.is_valid(String(pair[0])):
			service.open_channel(String(pair[0]), String(pair[1]))
	_list.context["self_id"] = service.store.self_id
	select_channel(ids[0])
	_dialogs.context = _report_context()


func select_channel(id: String) -> void:
	if not id in channel_ids:
		return
	if not current_channel.is_empty() and service != null:
		service.set_viewing(current_channel, false)
	current_channel = id
	_dialogs.context = _report_context()
	for t: Variant in _tab_buttons.keys():
		(_tab_buttons[t] as Button).theme_type_variation = &"ChatTabButtonSelected" if String(t) == id else &"ChatTabButton"
	_render_channel()
	_update_viewing()


# ---------- modes ----------

func set_mode(next: Mode) -> void:
	if next == mode:
		return
	mode = next
	_apply_mode()
	_update_viewing()
	mode_changed.emit(int(mode))


func expand() -> void:
	set_mode(Mode.EXPANDED)


func collapse() -> void:
	set_mode(Mode.COLLAPSED)


func _apply_mode() -> void:
	if _panel == null:
		return
	var expanded: bool = mode == Mode.EXPANDED
	var mini: bool = mode == Mode.MINI
	visible = not (external_toggle and mode == Mode.COLLAPSED)
	# Team battle, open: the tabs "Đội" / "Cả trận" take the place of the chat icon and title on the header line
	# (F119 "≥ 3 tin"); collapsed / 2-line strip / a single channel: the chat icon and the title.
	_tab_bar.visible = expanded and channel_ids.size() > 1
	var tabs_in_header: bool = _tab_bar.visible
	_head_icon.visible = not tabs_in_header
	_title.visible = not tabs_in_header
	_spacer.visible = not tabs_in_header
	_pins.visible = expanded and _pins.get_child_count() > 0
	_list_box.visible = expanded
	_composer.visible = expanded and not _care.visible
	_pins.visible = _pins.visible and not _care.visible
	_typing.visible = expanded and not _typing.text.is_empty()
	_retention.visible = expanded and not _retention_label.text.is_empty()
	_collapse.visible = expanded
	_expand.visible = not expanded
	_players.visible = expanded and not members.is_empty()
	_mini.visible = mini
	_badge.visible = not expanded and _badge.text != ""
	_mention_badge.visible = not expanded and _mention_badge.visible
	_render_mini()
	_render_badges()


# ---------- notices ----------

## A friendly notice (`key` + args, ChatFeedbackMapper tone). The care message gets Mây and stays until closed.
func show_notice(key: String, args: Dictionary = {}, tone: String = ChatFeedbackMapper.TONE_INFO) -> void:
	var text: String = TranslationServer.translate(key).format(args)
	if tone == ChatFeedbackMapper.TONE_CARE:
		_care_label.text = text
		_care.visible = true
		if mode != Mode.EXPANDED:
			expand()
		# The care message gets the room: writing waits until the student closed it (reading goes on).
		_apply_mode()
		return
	# The composer lock already says it (mute / sanction): no second banner with the same words.
	if _composer != null and _composer.locked and text == _composer.lock_text():
		return
	var style: Array = NOTICE_PANELS.get(tone, NOTICE_PANELS[ChatFeedbackMapper.TONE_INFO]) as Array
	_notice.theme_type_variation = style[0] as StringName
	_notice_label.theme_type_variation = style[1] as StringName
	_notice_icon.texture = UiBuild.icon(String(style[2]))
	_notice_label.text = text
	_notice.visible = true
	_notice_left = NOTICE_SEC


func notice_text() -> String:
	return _notice_label.text if _notice.visible else ""


func care_text() -> String:
	return _care_label.text if _care.visible else ""


func banner_text() -> String:
	return _banner_label.text if _banner.visible else ""


func retention_text() -> String:
	return _retention_label.text if _retention.visible else ""


func typing_text() -> String:
	return _typing.text if _typing.visible else ""


func mini_text() -> String:
	return _mini.text if _mini.visible else ""


func unread_badge_text() -> String:
	return _badge.text if _badge.visible else ""


func title_text() -> String:
	return _title.text


func tab_button(id: String) -> Button:
	return _tab_buttons.get(id) as Button


func tab_badge_text(id: String) -> String:
	var b: Label = _tab_badges.get(id) as Label
	return b.text if b != null and b.visible else ""


func list() -> ChatMessageList:
	return _list


func composer() -> ChatComposer:
	return _composer


func dialogs() -> ChatDialogs:
	return _dialogs


func sheet() -> ChatSheet:
	return _sheet


func collapse_button() -> Button:
	return _collapse


func expand_button() -> Button:
	return _expand


func players_button() -> Button:
	return _players


func pinned_count() -> int:
	return _pins.get_child_count()


func empty_visible() -> bool:
	return _empty.visible and _list_box.visible


## Every button / toggle of the dock — none turns the gameplay chat off (F119-AC-01, AC-23).
func control_names() -> PackedStringArray:
	var out: PackedStringArray = []
	for n: Node in find_children("*", "BaseButton", true, false):
		out.append(n.name)
	return out


func total_unread() -> int:
	var n: int = 0
	if service != null:
		for id: String in channel_ids:
			n += service.store.unread(id)
	return n


func total_mentions() -> int:
	var n: int = 0
	if service != null:
		for id: String in channel_ids:
			n += service.store.mentions(id)
	return n


# ---------- rendering ----------

func _render_channel() -> void:
	if service == null or current_channel.is_empty():
		return
	_composer.configure(service.store.channel(current_channel), service.quick)
	_composer.apply_band(UiBuild.band())
	_on_messages_changed(current_channel)
	_render_pins()
	_render_typing()
	_refresh_lock()
	_render_badges()
	_apply_mode()


func _on_messages_changed(channel_id: String) -> void:
	if channel_id != current_channel or service == null:
		if channel_id in channel_ids:
			_render_badges()
		return
	var entries: Array[ChatMessageModel] = service.store.visible(current_channel)
	_list.context["self_id"] = service.store.self_id
	_list.set_entries(entries)
	_empty.visible = entries.is_empty()
	var ch: ChatChannelModel = service.store.channel(current_channel)
	_empty.text = "ui.chat.empty_closed" if ch != null and ch.is_closed() else "ui.chat.empty"
	_render_mini()
	_render_pins()


func _on_message_updated(channel_id: String, key: String) -> void:
	if channel_id == current_channel:
		_dirty_receipts = true
		_list.refresh_key(key)
		_render_mini()


func _on_channel_changed(channel_id: String) -> void:
	if channel_id == current_channel:
		_composer.configure(service.store.channel(current_channel), service.quick)
		_refresh_lock()
		_render_pins()
	_render_badges()


func _on_typing_changed(channel_id: String) -> void:
	if channel_id == current_channel:
		_dirty_typing = true


func _on_receipts_changed(channel_id: String) -> void:
	if channel_id == current_channel:
		_dirty_receipts = true


func _render_typing() -> void:
	var ch: ChatChannelModel = service.store.channel(current_channel) if service != null else null
	_typing.text = ChatText.typing_sentence(service.store.typing_nicknames(current_channel)) if ch != null and ch.feature("typing") else ""
	_typing.visible = mode == Mode.EXPANDED and not _typing.text.is_empty()


func _render_pins() -> void:
	UiBuild.clear(_pins)
	if service == null or current_channel.is_empty():
		return
	for id: String in service.store.pins(current_channel):
		var m: ChatMessageModel = service.store.find(current_channel, id)
		if m == null:
			continue
		var b: Button = UiBuild.button("", "pin", &"TextButton")
		b.name = "Pin"
		b.text = TranslationServer.translate("ui.chat.pinned_item").format({"nickname": m.sender_nickname,
			"text": ChatText.visible_text(m, service.quick, service.assets).left(60)})
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.alignment = HORIZONTAL_ALIGNMENT_LEFT
		b.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
		b.pressed.connect(func() -> void: _list.scroll_to_key(m.key()))
		_pins.add_child(b)
	_pins.visible = mode == Mode.EXPANDED and _pins.get_child_count() > 0


func _render_mini() -> void:
	if _mini == null or service == null or current_channel.is_empty():
		return
	var entries: Array[ChatMessageModel] = service.store.visible(current_channel)
	var lines: PackedStringArray = []
	for i: int in range(maxi(0, entries.size() - MINI_LINES), entries.size()):
		var m: ChatMessageModel = entries[i]
		lines.append(TranslationServer.translate("ui.chat.mini_line").format({"nickname": ChatText.sender_text(m, service.store.self_id),
			"text": ChatText.visible_text(m, service.quick, service.assets)}))
	_mini.text = "\n".join(lines) if not lines.is_empty() else TranslationServer.translate("ui.chat.empty")


func _render_badges() -> void:
	if service == null:
		return
	for id: Variant in _tab_badges.keys():
		var n: int = service.store.unread(String(id))
		var badge: Label = _tab_badges[id] as Label
		badge.text = _count_text(n)
		badge.visible = n > 0 and String(id) != current_channel
	var total: int = total_unread()
	var mentions: int = total_mentions()
	_badge.text = _count_text(total) if total > 0 else ""
	_badge.visible = mode != Mode.EXPANDED and total > 0
	_mention_badge.visible = mode != Mode.EXPANDED and mentions > 0
	unread_changed.emit(total, mentions)


static func _count_text(n: int) -> String:
	return TranslationServer.translate("ui.chat.unread_many") if n > 99 else str(n)


## Composer lock: closed channel, sanction (with the time it ends), unknown channel.
func _refresh_lock() -> void:
	if service == null or current_channel.is_empty():
		return
	var ch: ChatChannelModel = service.store.channel(current_channel)
	_retention_label.text = ""
	if ch != null and ch.is_closed():
		var key: String = "ui.chat.closed_party" if ch.channel_type == ChatChannelModel.TYPE_PARTY else "ui.chat.closed_match"
		_composer.set_lock(key)
		var days: int = ch.retention_days(service.now_ms())
		if days > 0:
			_retention_label.text = TranslationServer.translate("ui.chat.retention_notice").format({"days": days})
	else:
		var lock: String = service.store.sanction_lock(service.now_ms())
		if lock.is_empty():
			_composer.set_lock("")
		else:
			var until: int = int(service.store.sanction.get("until_ms", 0))
			var fb: Dictionary = ChatFeedbackMapper.for_sanction(lock, String(service.store.sanction.get("reason_code", "")),
					ChatText.time_text(until))
			var key2: String = String(fb["key"]) if String(fb["tone"]) != ChatFeedbackMapper.TONE_CARE else ChatFeedbackMapper.KEY_MUTED
			_composer.set_lock(key2, fb["args"] as Dictionary, lock == ChatStore.SANCTION_RESTRICT_TEXT)
	_retention.visible = mode == Mode.EXPANDED and not _retention_label.text.is_empty()


func _on_connection_changed(_reconnecting: bool) -> void:
	_render_banner()


func _render_banner() -> void:
	var key: String = ""
	if service != null and service.reconnecting:
		key = "ui.chat.reconnecting"
	elif service != null and not current_channel.is_empty() and service.sync_slow(current_channel):
		key = "ui.chat.syncing"
	_banner_label.text = TranslationServer.translate(key) if not key.is_empty() else ""
	_banner.visible = not key.is_empty() and mode == Mode.EXPANDED


func _on_draft_returned(channel_id: String, text: String) -> void:
	if channel_id == current_channel:
		_composer.restore_draft(text)


func _on_quick_loaded() -> void:
	_composer.refresh_quick()
	_list.invalidate()
	_render_mini()


func _update_viewing() -> void:
	if service == null or current_channel.is_empty():
		return
	service.set_viewing(current_channel, mode == Mode.EXPANDED and is_visible_in_tree())
	_render_badges()


func _report_context() -> Dictionary:
	var ctx: Dictionary = {"channel_id": current_channel}
	if context_kind == CONTEXT_PARTY:
		ctx["party_id"] = context_id
	elif context_kind == CONTEXT_MATCH:
		ctx["match_id"] = context_id
	return ctx


# ---------- input from the parts ----------

func _on_menu_requested(m: ChatMessageModel) -> void:
	_dialogs.open_menu(m)


func _on_composer_action(action: StringName, m: ChatMessageModel) -> void:
	match action:
		ChatDialogs.ACT_REPLY:
			_composer.start_reply(m, ChatText.visible_text(m, service.quick, service.assets))
		ChatDialogs.ACT_QUOTE:
			_composer.start_quote(m)
		ChatDialogs.ACT_EDIT:
			_composer.start_edit(m)


func _on_text_submitted(text: String, reply_to: String, quote: Dictionary, mention_ids: PackedStringArray) -> void:
	if service != null:
		service.send_text(current_channel, text, reply_to, quote, mention_ids)
		service.typing_stopped(current_channel)
		_list.scroll_to_bottom()


func _on_picker_tab(tab: StringName) -> void:
	if service == null:
		return
	if tab == ChatPicker.TAB_STICKER and service.assets.packs.is_empty():
		service.load_stickers()
	elif tab == ChatPicker.TAB_GIF and service.assets.gifs_by_tag.is_empty():
		service.load_gifs("")


# ---------- events ----------

func _on_question_shown() -> void:
	if not auto_shrink or _question_on:
		return
	_question_on = true
	_mode_before_question = mode
	set_mode(Mode.MINI)


func _on_question_hidden() -> void:
	if not _question_on:
		return
	_question_on = false
	set_mode(_mode_before_question)


func _on_locale_changed(_locale: String) -> void:
	_composer.refresh_quick()
	_list.invalidate()
	_render_pins()
	_render_mini()
	_render_typing()
	_refresh_lock()


func _on_density_changed(band: String) -> void:
	_composer.apply_band(band)
	_list.invalidate()


func _notification(what: int) -> void:
	if what == NOTIFICATION_VISIBILITY_CHANGED and is_inside_tree():
		_update_viewing()
	elif what == NOTIFICATION_PREDELETE:
		_cleanup()


func _process(delta: float) -> void:
	if service != null:
		service.tick()
	if _notice.visible:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false
	_coalesce_left -= delta
	if _coalesce_left <= 0.0:
		_coalesce_left = COALESCE_SEC
		if _dirty_typing:
			_dirty_typing = false
			_render_typing()
		if _dirty_receipts:
			_dirty_receipts = false
		_render_banner()
	_follow_keyboard()


## The virtual keyboard of a phone pushes the dock up so the quick-message row stays visible (spec edge case).
func _follow_keyboard() -> void:
	var kb: float = float(DisplayServer.virtual_keyboard_get_height()) if _composer.input().has_focus() else 0.0
	var offset: float = 0.0
	if kb > 0.0:
		var scale: float = maxf(get_viewport().get_final_transform().get_scale().y, 0.01)
		var bottom_free: float = get_viewport_rect().size.y - kb / scale
		offset = minf(0.0, bottom_free - get_global_rect().end.y)
	if not is_equal_approx(offset, _kb_offset):
		_kb_offset = offset
		_panel.position.y = offset


# ---------- building ----------

func _build_tabs() -> void:
	UiBuild.clear(_tabs)
	_tab_buttons.clear()
	_tab_badges.clear()
	if channel_ids.size() < 2:
		_tab_bar.visible = false
		return
	for id: String in channel_ids:
		var team: bool = id == channel_ids[0]
		var wrap: HBoxContainer = UiBuild.hbox()
		UiBuild.expand(wrap)
		var b: Button = UiBuild.button("ui.chat.tab.team" if team else "ui.chat.tab.match", "tab_team" if team else "tab_match",
				&"ChatTabButton")
		b.name = "TabTeam" if team else "TabMatch"
		b.autowrap_mode = TextServer.AUTOWRAP_OFF
		UiBuild.expand(b)
		b.pressed.connect(func() -> void: select_channel(id))
		wrap.add_child(b)
		var badge: Label = UiBuild.data_label("", &"CountBadge", false)
		badge.name = "Badge"
		badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		badge.visible = false
		wrap.add_child(badge)
		_tabs.add_child(wrap)
		_tab_buttons[id] = b
		_tab_badges[id] = badge
	UiBuild.size_buttons(_tabs)


func _build() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_panel = UiBuild.panel(&"ChatPanel")
	_panel.name = "Panel"
	_panel.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(_panel)
	_box = UiBuild.vbox(&"FieldBox")
	_box.name = "Box"
	_panel.add_child(_box)
	_header = UiBuild.hbox(&"ToolbarBox")
	_header.name = "Header"
	_box.add_child(_header)
	_head_icon = UiBuild.icon_rect("chat")
	_head_icon.name = "ChatIcon"
	_head_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_header.add_child(_head_icon)
	_title = UiBuild.label("ui.chat.title.party", &"SectionTag", false)
	_title.name = "Title"
	_title.uppercase = true
	_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_header.add_child(_title)
	_badge = UiBuild.data_label("", &"CountBadge", false)
	_badge.name = "Badge"
	_badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_badge.visible = false
	_header.add_child(_badge)
	_mention_badge = UiBuild.label("ui.chat.mention_badge", &"CountBadge", false)
	_mention_badge.name = "MentionBadge"
	_mention_badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_mention_badge.visible = false
	_header.add_child(_mention_badge)
	_spacer = Control.new()
	_spacer.name = "Spacer"
	_spacer.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiBuild.expand(_spacer)
	_header.add_child(_spacer)
	_players = UiBuild.button("", "group", &"IconButton")
	_players.name = "Players"
	_players.tooltip_text = "ui.chat.players.open"
	_players.visible = false
	_players.pressed.connect(func() -> void: _dialogs.open_players(members))
	_header.add_child(_players)
	_collapse = UiBuild.button("", "arrow_down", &"IconButton")
	_collapse.name = "Collapse"
	_collapse.tooltip_text = "ui.chat.collapse"
	_collapse.pressed.connect(collapse)
	_header.add_child(_collapse)
	_expand = UiBuild.button("ui.chat.expand", "arrow_up", &"SecondaryButton")
	_expand.name = "Expand"
	_expand.pressed.connect(expand)
	_header.add_child(_expand)
	_mini = UiBuild.data_label("", &"ChatMetaLabel")
	_mini.name = "Mini"
	_mini.max_lines_visible = MINI_LINES
	_mini.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_mini.visible = false
	_box.add_child(_mini)
	# Team battle: the tabs "Đội" / "Cả trận" on the header line in place of the chat icon and title (F119 "≥ 3 tin"); a
	# much longer translation scrolls sideways with the arrows instead of taking a line of its own.
	_tabs = UiBuild.hbox(&"ToolbarBox")
	_tabs.name = "Tabs"
	_tab_bar = UiBuild.chip_scroller(_tabs) as ChipScroller
	_tab_bar.name = "TabBar"
	_tab_bar.visible = false
	_header.add_child(_tab_bar)
	_header.move_child(_tab_bar, _spacer.get_index())
	_banner = UiBuild.panel(&"WarningBannerPanel")
	_banner.name = "Banner"
	_banner.visible = false
	_box.add_child(_banner)
	var banner_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_banner.add_child(banner_row)
	banner_row.add_child(UiBuild.icon_rect("wifi_off", 0.8))
	_banner_label = UiBuild.data_label("", &"WarningBannerLabel")
	_banner_label.name = "BannerText"
	UiBuild.expand(_banner_label)
	banner_row.add_child(_banner_label)
	_notice = UiBuild.panel(&"InfoBannerPanel")
	_notice.name = "Notice"
	_notice.visible = false
	_box.add_child(_notice)
	var notice_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_notice.add_child(notice_row)
	_notice_icon = UiBuild.icon_rect("info", 0.8)
	_notice_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	notice_row.add_child(_notice_icon)
	_notice_label = UiBuild.data_label("", &"InfoBannerLabel")
	_notice_label.name = "NoticeText"
	UiBuild.expand(_notice_label)
	notice_row.add_child(_notice_label)
	_care = UiBuild.panel(&"SpeechBubble")
	_care.name = "Care"
	_care.visible = false
	_box.add_child(_care)
	var care_box: VBoxContainer = UiBuild.vbox(&"FieldBox")
	_care.add_child(care_box)
	var care_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	care_box.add_child(care_row)
	var mascot: CloudyMascot = CloudyMascot.new()
	mascot.compact = true
	mascot.mood = CloudyMascot.Mood.ENCOURAGE
	mascot.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	care_row.add_child(mascot)
	_care_label = UiBuild.data_label("", &"SpeechBubbleLabel")
	_care_label.name = "CareText"
	UiBuild.expand(_care_label)
	care_row.add_child(_care_label)
	var care_ok: Button = UiBuild.button("ui.chat.care_ok", "tick", &"SecondaryButton")
	care_ok.name = "CareOk"
	care_ok.size_flags_horizontal = Control.SIZE_SHRINK_END
	care_ok.pressed.connect(func() -> void:
		_care.visible = false
		_apply_mode())
	care_box.add_child(care_ok)
	_pins = UiBuild.vbox()
	_pins.name = "Pins"
	_pins.visible = false
	_box.add_child(_pins)
	_list_box = Control.new()
	_list_box.name = "ListBox"
	_list_box.mouse_filter = Control.MOUSE_FILTER_PASS
	UiBuild.expand(_list_box, true, true)
	_list_box.custom_minimum_size.y = UiBuild.target()
	_box.add_child(_list_box)
	_list = ChatMessageList.new()
	_list.name = "Messages"
	_list.set_anchors_preset(Control.PRESET_FULL_RECT)
	_list.menu_requested.connect(_on_menu_requested)
	_list.top_reached.connect(func() -> void:
		if service != null and service.store.has_older(current_channel):
			service.load_history(current_channel, service.store.oldest_seq(current_channel)))
	_list_box.add_child(_list)
	_empty = UiBuild.label("ui.chat.empty", &"MutedLabel")
	_empty.name = "Empty"
	_empty.set_anchors_preset(Control.PRESET_CENTER)
	_empty.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_empty.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_empty.set_anchors_preset(Control.PRESET_FULL_RECT)
	_empty.visible = false
	_list_box.add_child(_empty)
	_typing = UiBuild.data_label("", &"ChatMetaLabel")
	_typing.name = "Typing"
	_typing.visible = false
	_box.add_child(_typing)
	_retention = UiBuild.hbox()
	_retention.name = "Retention"
	_retention.visible = false
	_box.add_child(_retention)
	_retention.add_child(UiBuild.icon_rect("clock", 0.75))
	_retention_label = UiBuild.data_label("", &"ChatMetaLabel")
	_retention_label.name = "RetentionText"
	UiBuild.expand(_retention_label)
	_retention.add_child(_retention_label)
	_composer = ChatComposer.new()
	_composer.text_submitted.connect(_on_text_submitted)
	_composer.edit_submitted.connect(func(m: ChatMessageModel, text: String) -> void: if service != null: service.edit(m, text))
	_composer.quick_chosen.connect(func(id: String) -> void:
		if service != null:
			service.send_quick(current_channel, id)
			_list.scroll_to_bottom())
	_composer.emoji_chosen.connect(func(code: String) -> void: if service != null: service.send_emoji(current_channel, code))
	_composer.sticker_chosen.connect(func(id: String) -> void: if service != null: service.send_sticker(current_channel, id))
	_composer.gif_chosen.connect(func(id: String) -> void: if service != null: service.send_gif(current_channel, id))
	_composer.typing_changed.connect(func(active: bool) -> void:
		if service == null:
			return
		if active:
			service.typing_started(current_channel)
		else:
			service.typing_stopped(current_channel))
	_composer.picker_tab_opened.connect(_on_picker_tab)
	_composer.gif_topic_chosen.connect(func(tag: String) -> void:
		if service != null:
			if service.assets.gifs_by_tag.has(tag):
				_composer.picker().refresh_gifs(service.assets, tag)
			else:
				service.load_gifs(tag))
	_box.add_child(_composer)
	_sheet = ChatSheet.new()
	add_child(_sheet)
	_dialogs = ChatDialogs.new(_sheet)
	_dialogs.composer_action_chosen.connect(_on_composer_action)
	_dialogs.notice_requested.connect(show_notice)
	UiBuild.size_buttons(self)
