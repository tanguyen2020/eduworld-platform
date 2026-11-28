class_name ChatSearchPanel
extends PanelContainer
## "Tìm tin nhắn" of the HUD chat (F112-AC-15…AC-17; CHAT_SYSTEM §8.4; STUDENT_GAME_UX_SOCIAL §7): over the tabs of the
## ChatHub — "Quay lại", the query field with its counter "{n}/100" ("Tìm" grey outside 2–100 characters, the contract
## bounds), the scope "Tất cả" or "Kênh này" (the club / DM on screen when its `features.search` is on — never World /
## Area), then the hits: the sender's portrait and nickname, the channel (club name or friend's nickname), the day and
## time in the student's locale and the excerpt as PLAIN text with the keyword in bold and underlined (`RichTextLabel.add_text`, BBCode
## off — nothing from the server or a student is ever parsed, F112-AC-16, AC-30); "Xem thêm" pages on; no hit → Mây
## with a hint. "Xem" of a hit opens its channel at that message (ChatHub.jump_to). The query is never stored or logged
## (AGENTS R14). UI only (R1).

signal hit_chosen(hit: Dictionary)

const SCOPE_ALL: String = "all"
const SCOPE_CHANNEL: String = "channel"

var search: ChatSearchService = null
var hub_service: ChatHubService = null
var scope: String = SCOPE_ALL
var _channel_id: String = ""
var _input: LineEdit = null
var _counter: Label = null
var _go: Button = null
var _scope_all: Button = null
var _scope_channel: Button = null
var _results: VBoxContainer = null
var _more: Button = null
var _empty: Control = null
var _rows: Array[Control] = []


func _init() -> void:
	name = "SearchPanel"
	theme_type_variation = &"ChatPanel"
	_build()


func bind(service: ChatSearchService, hub: ChatHubService) -> void:
	unbind()
	search = service
	hub_service = hub
	if search != null:
		search.results_changed.connect(_render)
		search.busy_changed.connect(_on_busy)


func unbind() -> void:
	if search != null:
		if search.results_changed.is_connected(_render):
			search.results_changed.disconnect(_render)
		if search.busy_changed.is_connected(_on_busy):
			search.busy_changed.disconnect(_on_busy)
	search = null
	hub_service = null


## Opens the panel; `tab` = the tab on screen ("Kênh này" when its channel can be searched).
func open_for(tab: PersistentChannelTab) -> void:
	_channel_id = ""
	if tab != null and tab.kind in [PersistentChannelTab.KIND_CLUB, PersistentChannelTab.KIND_DM] and search != null:
		var chat: ChatService = AppState.services.chat if AppState.services != null else null
		var ch: ChatChannelModel = chat.store.channel(tab.channel_id) if chat != null else null
		if ch != null and ch.feature("search"):
			_channel_id = tab.channel_id
	_scope_channel.visible = not _channel_id.is_empty()
	set_scope(SCOPE_CHANNEL if not _channel_id.is_empty() else SCOPE_ALL)
	visible = true
	_input.grab_focus.call_deferred()
	_update_go()
	_render()


func set_query(text: String) -> void:
	_input.text = text
	_update_go()


func set_scope(next: String) -> void:
	scope = next if next == SCOPE_ALL or not _channel_id.is_empty() else SCOPE_ALL
	_scope_all.set_pressed_no_signal(scope == SCOPE_ALL)
	_scope_channel.set_pressed_no_signal(scope == SCOPE_CHANNEL)
	UiBuild.mark_chips([_scope_all, _scope_channel])


## Coroutine: "Tìm".
func submit() -> void:
	if search == null or not ChatSearchService.query_ok(_input.text):
		return
	await search.search(_input.text, _channel_id if scope == SCOPE_CHANNEL else "")


func go_button() -> Button:
	return _go


func more_button() -> Button:
	return _more


func counter_text() -> String:
	return _counter.text


func hit_rows() -> Array[Control]:
	return _rows.duplicate()


func empty_visible() -> bool:
	return _empty.visible


## The excerpt texts shown (tests: plain text, no markup).
func excerpts() -> PackedStringArray:
	var out: PackedStringArray = []
	for r: Control in _rows:
		var rt: RichTextLabel = r.find_child("Excerpt", true, false) as RichTextLabel
		if rt != null:
			out.append(rt.get_parsed_text())
	return out


## Every label text of a row (nickname, channel, time).
func row_texts(index: int) -> PackedStringArray:
	var out: PackedStringArray = []
	if index < 0 or index >= _rows.size():
		return out
	for node: Node in _rows[index].find_children("*", "Label", true, false):
		out.append(tr((node as Label).text))
	return out


func _render() -> void:
	for r: Control in _rows:
		r.queue_free()
	_rows.clear()
	UiBuild.clear(_results)
	if search == null:
		return
	for hit: Dictionary in search.hits:
		var row: Control = _row(hit)
		_results.add_child(row)
		_rows.append(row)
	_more.visible = search.has_more
	_empty.visible = search.loaded and search.hits.is_empty()
	UiBuild.size_buttons(_results)


func _row(hit: Dictionary) -> Control:
	var m: ChatMessageModel = hit["message"] as ChatMessageModel
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Hit"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var thumb: AvatarThumb = AvatarThumb.new()
	thumb.standalone = true
	thumb.custom_minimum_size = Vector2.ONE * UiBuild.target()
	thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var look: AvatarConfigModel = AvatarConfigModel.from_dict(m.sender_avatar) if not m.sender_avatar.is_empty() else null
	thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	row.add_child(thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(texts)
	var head: HFlowContainer = HFlowContainer.new()
	head.name = "Head"
	texts.add_child(head)
	var nick: Label = UiBuild.data_label(m.sender_nickname, &"HeadingLabel", false)
	nick.name = "Nickname"
	head.add_child(nick)
	var where: Label = UiBuild.data_label(_channel_name(String(hit["channel_id"]), String(hit["channel_type"])),
			&"SectionTag", false)
	where.name = "Channel"
	where.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(where)
	var at: int = int(m.created_at_ms / 1000)
	if at > 0:
		var when: Label = UiBuild.data_label(tr("ui.chat.search.when").format({"date": LiveOpsTime.ict_date(at),
			"time": LiveOpsTime.ict_time(at)}), &"MutedLabel", false)
		when.name = "When"
		when.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		head.add_child(when)
	var excerpt: RichTextLabel = RichTextLabel.new()
	excerpt.name = "Excerpt"
	excerpt.bbcode_enabled = false
	excerpt.fit_content = true
	excerpt.scroll_active = false
	excerpt.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	excerpt.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	excerpt.theme_type_variation = &"ChatTextLabel"
	excerpt.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiBuild.expand(excerpt)
	for piece: Dictionary in HighlightText.pieces(String(hit["excerpt"]), hit["ranges"] as Array[Vector2i]):
		if bool(piece["hit"]):
			# The keyword: bold and underlined (a mark that does not depend on a colour alone).
			excerpt.push_bold()
			excerpt.push_underline()
			excerpt.add_text(String(piece["text"]))
			excerpt.pop()
			excerpt.pop()
		else:
			excerpt.add_text(String(piece["text"]))
	texts.add_child(excerpt)
	var open: Button = UiBuild.button("ui.chat.search.open_hit", "arrow_right", &"SecondaryButton")
	open.name = "Open"
	open.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	open.pressed.connect(func() -> void: hit_chosen.emit(hit))
	row.add_child(open)
	return card


func _channel_name(channel_id: String, channel_type: String) -> String:
	if hub_service != null:
		var t: PersistentChannelTab = hub_service.state.tab_of_channel(channel_id)
		if t != null:
			return t.label()
		var peer: Dictionary = hub_service.state.peers.peer(channel_id)
		if not DataRead.str_of(peer, "nickname").is_empty():
			return DataRead.str_of(peer, "nickname")
	return tr("ui.chat.search.channel_dm") if channel_type == ChatChannelModel.TYPE_DIRECT else tr("ui.chat.search.channel_club")


func _update_go() -> void:
	var n: int = _input.text.strip_edges().length()
	_counter.text = tr("ui.chat.search.counter").format({"count": LocaleFormat.number(n),
		"max": LocaleFormat.number(ChatSearchService.Q_MAX)})
	_go.disabled = not ChatSearchService.query_ok(_input.text) or (search != null and search.busy)


func _on_busy(_busy: bool) -> void:
	_update_go()
	_more.disabled = search != null and search.busy


func _build() -> void:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	add_child(box)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	head.name = "Head"
	box.add_child(head)
	var back: Button = UiBuild.button("", "arrow_left", &"SecondaryButton")
	back.name = "Back"
	back.tooltip_text = "ui.chat.search.back"
	back.accessibility_name = TranslationServer.translate("ui.chat.search.back")
	back.pressed.connect(func() -> void: visible = false)
	head.add_child(back)
	_input = LineEdit.new()
	_input.name = "Query"
	_input.placeholder_text = "ui.chat.search.placeholder"
	_input.max_length = ChatSearchService.Q_MAX
	UiBuild.expand(_input)
	_input.text_changed.connect(func(_t: String) -> void: _update_go())
	_input.text_submitted.connect(func(_t: String) -> void: submit())
	head.add_child(_input)
	_counter = UiBuild.data_label("", &"MutedLabel", false)
	_counter.name = "Counter"
	_counter.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_counter)
	_go = UiBuild.button("ui.chat.search.go", "search_light")
	_go.name = "Go"
	_go.pressed.connect(submit)
	head.add_child(_go)
	var scopes: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	scopes.name = "Scopes"
	box.add_child(scopes)
	var group: ButtonGroup = ButtonGroup.new()
	_scope_all = UiBuild.chip("ui.chat.search.scope_all", group)
	_scope_all.name = "ScopeAll"
	_scope_all.pressed.connect(set_scope.bind(SCOPE_ALL))
	scopes.add_child(_scope_all)
	_scope_channel = UiBuild.chip("ui.chat.search.scope_channel", group)
	_scope_channel.name = "ScopeChannel"
	_scope_channel.pressed.connect(set_scope.bind(SCOPE_CHANNEL))
	scopes.add_child(_scope_channel)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	box.add_child(scroll)
	var list: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(list)
	scroll.add_child(list)
	_results = UiBuild.vbox(&"FormBox")
	_results.name = "Results"
	list.add_child(_results)
	_empty = FriendsUi.empty_line("ui.chat.search.empty", "Empty")
	_empty.visible = false
	list.add_child(_empty)
	_more = UiBuild.button("ui.chat.search.more", "arrow_down", &"SecondaryButton")
	_more.name = "More"
	_more.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_more.visible = false
	_more.pressed.connect(func() -> void: if search != null: search.more())
	list.add_child(_more)
	_update_go()
	UiBuild.size_buttons(self)
