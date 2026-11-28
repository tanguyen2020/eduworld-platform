class_name ChatHub
extends Control
## The HUD chat of the persistent channels (F112 US1–US4; S12 of STUDENT_GAME_UX, STUDENT_GAME_UX_SOCIAL §7): opened by
## the HUD chat button (badge), `Enter` / `T` on PC, "Nhắn tin" of a friend's profile (`{dm: user_id}`) or a deep link
## (`{channel_id}`) — `ui_requested("chat", args)`. A window with the shared title bar (chat icon, "CHAT", the magnifier
## and the red "✕ Đóng" — F125-AC-06): PC / tablet a tall frame on the left (≤ 40% of the width); a phone in compact mode
## (F125) the whole height. A row of tabs (one line, arrows when it does not fit) — "Khu vực", "Cả trường", each club,
## "Bạn bè", the DMs opened — each with its unread count (clubs / DMs) or "@" badge (ChatHubState). A channel tab shows
## the F119 ChatDock in `show_channel` mode (every message part, the menu, Tắt tiếng · Chặn · Báo cáo with
## `context.channel_id`, the "≥ 3 tin" rule); only the tab on screen is joined, closing the window or changing tab leaves
## it (F112-AC-04). "Bạn bè" lists the friends with their DM unread count and "Nhắn tin" (ChatFriendsList). The magnifier
## opens ChatSearchPanel over the tabs. UI only: ChatHubService / ChatService / ChatSearchService do the work (R1).

signal closed()

## Window anchors [left, top, right, bottom] on PC / tablet (a tall frame on the left, ≤ 40% of the width).
const EDGES: Array = [0.02, 0.0, 0.42, 1.0]
## On a compact phone the HUD chat is an overlay over the whole screen (STUDENT_GAME_UX_SOCIAL §7, F112-AC-27).
const EDGES_COMPACT: Array = [0.0, 0.0, 1.0, 1.0]
const NOTICE_SEC: float = 4.0

var services: ServiceLocator = null
var hub_service: ChatHubService = null
var chat: ChatService = null
var search_service: ChatSearchService = null
var current_tab: String = ""
var _ui: Dictionary = {}
var _tabs_box: HBoxContainer = null
var _tab_row: Control = null
var _tab_buttons: Dictionary = {}
var _tab_badges: Dictionary = {}
var _dock: ChatDock = null
var _friends: ChatFriendsList = null
var _empty: Control = null
var _search_button: Button = null
var _search: ChatSearchPanel = null
var _notice: PanelContainer = null
var _notice_label: Label = null
var _notice_left: float = 0.0
## True while `open` reads the channels: the tab asked for is chosen at the end (no join of another tab meanwhile).
var _opening: bool = false


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)
	AppState.compact_changed.connect(_on_compact_changed)


func _exit_tree() -> void:
	_unbind()
	for pair: Array in [[EventBus.locale_changed, _on_locale_changed], [AppState.compact_changed, _on_compact_changed]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)


## WorldRoot: `ui_requested("chat", {tab | dm | channel_id})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_bind()
	if hub_service != null and not hub_service.available():
		closed.emit.call_deferred()
		return
	_fit()
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	(_ui["close"] as Button).grab_focus.call_deferred(true)
	_search.visible = false
	_opening = true
	if hub_service != null:
		await hub_service.refresh()
		if social_ready():
			await services.social.load_friends()
			hub_service.rebuild()
	_opening = false
	if not visible:
		return
	var dm: String = String(args.get("dm", ""))
	var channel_id: String = String(args.get("channel_id", ""))
	if Uuid.is_valid(dm) and hub_service != null:
		if await hub_service.open_dm(dm):
			return
	if Uuid.is_valid(channel_id) and hub_service != null:
		var t: PersistentChannelTab = hub_service.state.tab_of_channel(channel_id)
		if t != null:
			select_tab(t.tab_id)
			return
	var wanted: String = String(args.get("tab", current_tab))
	select_tab(wanted if hub_service != null and hub_service.state.tab(wanted) != null else _first_tab())


func close() -> void:
	_search.visible = false
	if _dock != null:
		_dock.clear_context()
	if hub_service != null:
		hub_service.leave_all()
		hub_service.state.open_tab = ""
	hide()
	_unbind()
	closed.emit()


func social_ready() -> bool:
	return services != null and services.social != null


## Shows tab `tab_id` (a channel tab joins its channel; the previous one is left).
func select_tab(tab_id: String) -> void:
	if hub_service == null:
		return
	var t: PersistentChannelTab = hub_service.state.tab(tab_id)
	current_tab = t.tab_id if t != null else ""
	hub_service.state.open_tab = current_tab
	_render_tabs()
	_empty.visible = t == null
	_friends.visible = t != null and t.kind == PersistentChannelTab.KIND_FRIENDS
	_dock.visible = t != null and not t.channel_id.is_empty()
	if t == null or t.channel_id.is_empty():
		_dock.clear_context()
		if t != null:
			_friends.render()
		return
	_dock.show_channel(t.channel_id, t.channel_type, t.label())
	_dock.expand()


func tab_ids() -> PackedStringArray:
	return hub_service.state.tab_ids() if hub_service != null else PackedStringArray()


func tab_button(tab_id: String) -> Button:
	return _tab_buttons.get(tab_id) as Button


func tab_badge_text(tab_id: String) -> String:
	var b: Label = _tab_badges.get(tab_id) as Label
	return b.text if b != null and b.visible else ""


func dock() -> ChatDock:
	return _dock


func friends_list() -> ChatFriendsList:
	return _friends


func search_panel() -> ChatSearchPanel:
	return _search


func search_button() -> Button:
	return _search_button


func close_buttons() -> Array[Button]:
	var out: Array[Button] = []
	for b: Node in (_ui["window"] as Control).find_children("Close", "Button", true, false):
		out.append(b as Button)
	return out


func notice_text() -> String:
	return _notice_label.text if _notice.visible else ""


func show_notice(key: String, args: Dictionary = {}) -> void:
	_notice_label.text = TranslationServer.translate(key).format(args)
	_notice.visible = not key.is_empty()
	_notice_left = NOTICE_SEC


## Coroutine: a search hit was tapped — its tab opens, the history around it is read and the message is shown and lit
## for 2 s; `404` (not a member any more) → "Tin này không còn xem được" and the hit leaves the list (F112-AC-17).
func jump_to(hit: Dictionary) -> void:
	var m: ChatMessageModel = hit.get("message") as ChatMessageModel
	var channel_id: String = String(hit.get("channel_id", ""))
	if m == null or chat == null:
		return
	var t: PersistentChannelTab = hub_service.state.tab_of_channel(channel_id)
	if t == null:
		var user_id: String = hub_service.state.peers.user_of(channel_id)
		if not user_id.is_empty() and await hub_service.open_dm(user_id):
			t = hub_service.state.tab_of_channel(channel_id)
	if t == null:
		show_notice("ui.chat.search.gone")
		search_service.drop(m.message_id)
		return
	var result: ApiResult = await chat.load_around(channel_id, m.seq)
	if result.status == 404:
		show_notice("ui.chat.search.gone")
		search_service.drop(m.message_id)
		return
	_search.visible = false
	select_tab(t.tab_id)
	_dock.jump_to(m.key())


func _process(delta: float) -> void:
	if not visible:
		return
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed(&"ui_cancel"):
		if _search.visible:
			_search.visible = false
		else:
			close()
		get_viewport().set_input_as_handled()


# ---------- binding ----------

func _bind() -> void:
	if services == null or hub_service != null:
		return
	hub_service = services.chat_hub
	chat = services.chat
	search_service = services.chat_search
	if hub_service == null:
		return
	hub_service.tabs_changed.connect(_on_tabs_changed)
	hub_service.dm_opened.connect(_on_dm_opened)
	hub_service.channel_removed.connect(_on_channel_removed)
	hub_service.notice_requested.connect(show_notice)
	hub_service.availability_changed.connect(_on_availability)
	_friends.bind(hub_service, chat, services.social)
	_search.bind(search_service, hub_service)
	if _dock.service == null:
		_dock.bind_services(services.chat, services.social, services.reports)


func _unbind() -> void:
	if hub_service == null:
		return
	for pair: Array in [[hub_service.tabs_changed, _on_tabs_changed], [hub_service.dm_opened, _on_dm_opened],
			[hub_service.channel_removed, _on_channel_removed], [hub_service.notice_requested, show_notice],
			[hub_service.availability_changed, _on_availability]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	_friends.unbind()
	_search.unbind()
	hub_service = null


func _on_tabs_changed() -> void:
	if not visible or _opening:
		return
	if not current_tab.is_empty() and hub_service.state.tab(current_tab) == null:
		select_tab(_first_tab())
		return
	if current_tab.is_empty() and not hub_service.state.tabs.is_empty():
		select_tab(_first_tab())
		return
	_render_tabs()
	if _friends.visible:
		_friends.render()
	# A club locked / a DM frozen while open: the dock reads the channel status again.
	var t: PersistentChannelTab = hub_service.state.tab(current_tab)
	if t != null and not t.channel_id.is_empty():
		_dock.refresh_lock()


func _on_dm_opened(user_id: String, _channel_id: String) -> void:
	if visible:
		select_tab("dm:%s" % user_id)


func _on_channel_removed(channel_id: String) -> void:
	if visible and _dock.current_channel == channel_id:
		_dock.clear_context()


func _on_availability(available: bool) -> void:
	if not available and visible:
		close()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render_tabs()


func _on_compact_changed(_compact: bool) -> void:
	_fit()


# ---------- rendering ----------

func _first_tab() -> String:
	return hub_service.state.tabs[0].tab_id if hub_service != null and not hub_service.state.tabs.is_empty() else ""


func _fit() -> void:
	var win: Control = _ui["window"] as Control
	# On a phone the title word gives its room to the tabs (the chat icon keeps it as tooltip and accessible name).
	var title: Label = _ui["title"] as Label
	title.visible = not AppState.compact
	var icon: TextureRect = _ui["icon"] as TextureRect
	icon.tooltip_text = "ui.chat.hub.title" if AppState.compact else ""
	icon.mouse_filter = Control.MOUSE_FILTER_PASS if AppState.compact else Control.MOUSE_FILTER_IGNORE
	# F112 "≥ 3 tin" on a phone: the tabs sit on the title bar line (between the chat icon and the magnifier), like the
	# team tabs of the match chat, so the messages keep the height of a whole row; PC / tablet: a row under the title.
	var header: HBoxContainer = _ui["header"] as HBoxContainer
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var parent: Node = header if AppState.compact else body
	if _tab_row.get_parent() != parent:
		_tab_row.reparent(parent, false)
	if AppState.compact:
		# Order on the title bar: icon, (title), tabs, magnifier, ✕ Đóng.
		header.move_child(_tab_row, header.get_child_count() - 1)
		header.move_child(_search_button, header.get_child_count() - 1)
		header.move_child(_ui["close"] as Button, header.get_child_count() - 1)
	else:
		body.move_child(_tab_row, header.get_parent().get_index() + 1)
	var edges: Array = EDGES_COMPACT if AppState.compact else EDGES
	win.anchor_left = float(edges[0])
	win.anchor_top = float(edges[1])
	win.anchor_right = float(edges[2])
	win.anchor_bottom = float(edges[3])


func _render_tabs() -> void:
	UiBuild.clear(_tabs_box)
	_tab_buttons.clear()
	_tab_badges.clear()
	if hub_service == null:
		return
	for t: PersistentChannelTab in hub_service.state.tabs:
		var wrap: HBoxContainer = UiBuild.hbox()
		wrap.name = "Tab_" + t.tab_id.replace(":", "_")
		var on_bar: bool = _tab_row.get_parent() == _ui["header"]
		var selected: bool = t.tab_id == current_tab
		# On the blue title bar (phone): light tabs, the chosen one yellow with a tick (F114 chips); in the body: chat tabs.
		var variation: StringName = &"ChatTabButtonSelected" if selected else &"ChatTabButton"
		if on_bar:
			variation = &"ChoiceChip" if selected else &"SecondaryButton"
		# The chosen tab shows a tick (a white mark that reads on the blue tab; its line icon would vanish there).
		var b: Button = UiBuild.button("", "check" if selected else _tab_icon(t), variation)
		b.text = t.label()
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.autowrap_mode = TextServer.AUTOWRAP_OFF
		b.toggle_mode = true
		b.set_pressed_no_signal(t.tab_id == current_tab)
		var id: String = t.tab_id
		b.pressed.connect(func() -> void: select_tab(id))
		wrap.add_child(b)
		var badge: Label = UiBuild.data_label("", &"CountBadge", false)
		badge.name = "Badge"
		badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		if t.unread > 0 and t.tab_id != current_tab:
			badge.text = ChatHubState.badge_text(t.unread)
		elif t.mentions > 0 and t.tab_id != current_tab:
			badge.text = TranslationServer.translate("ui.chat.mention_badge")
		badge.visible = not badge.text.is_empty()
		wrap.add_child(badge)
		if t.muted:
			var muted: TextureRect = UiBuild.icon_rect("sound_off", 0.7)
			muted.name = "Muted"
			muted.tooltip_text = "ui.chat.hub.muted"
			muted.mouse_filter = Control.MOUSE_FILTER_PASS
			muted.size_flags_vertical = Control.SIZE_SHRINK_CENTER
			wrap.add_child(muted)
		_tabs_box.add_child(wrap)
		_tab_buttons[t.tab_id] = b
		_tab_badges[t.tab_id] = badge
	UiBuild.size_buttons(_tabs_box)
	_search_button.visible = _searchable()
	# The tab on screen is scrolled into view (a DM opened at the end of the row).
	_scroll_to_tab.call_deferred(current_tab)


## The tab on screen is brought into the visible part of the row once the row has its size (a frame later).
func _scroll_to_tab(tab_id: String) -> void:
	if not is_inside_tree():
		return
	await get_tree().process_frame
	var shown: Button = _tab_buttons.get(tab_id) as Button
	if shown != null and is_instance_valid(shown) and shown.is_inside_tree():
		(_tab_row as ChipScroller).scroll.ensure_control_visible(shown.get_parent() as Control)


static func _tab_icon(t: PersistentChannelTab) -> String:
	match t.kind:
		PersistentChannelTab.KIND_AREA:
			return "map"
		PersistentChannelTab.KIND_WORLD:
			return "globe"
		PersistentChannelTab.KIND_CLUB:
			return "flag"
		PersistentChannelTab.KIND_FRIENDS:
			return "group"
	return "chat"


## The magnifier shows when a club / DM channel with `features.search` exists (never World / Area).
func _searchable() -> bool:
	if chat == null or hub_service == null:
		return false
	for t: PersistentChannelTab in hub_service.state.tabs:
		if t.kind in [PersistentChannelTab.KIND_CLUB, PersistentChannelTab.KIND_DM]:
			var ch: ChatChannelModel = chat.store.channel(t.channel_id)
			if ch != null and ch.feature("search"):
				return true
	for id: String in chat.store.channel_ids():
		var c: ChatChannelModel = chat.store.channel(id)
		if c != null and c.channel_type == ChatChannelModel.TYPE_DIRECT and c.feature("search"):
			return true
	return false


func _build() -> void:
	_ui = UiBuild.window(self, "ui.chat.hub.title", "chat")
	# One short word: never broken over two lines (a long close label widens the frame instead).
	(_ui["title"] as Label).autowrap_mode = TextServer.AUTOWRAP_OFF
	(_ui["close"] as Button).pressed.connect(close)
	_search_button = UiBuild.button("", "search_light", &"")
	_search_button.name = "Search"
	_search_button.tooltip_text = "ui.chat.search.open"
	_search_button.accessibility_name = TranslationServer.translate("ui.chat.search.open")
	_search_button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_search_button.visible = false
	_search_button.pressed.connect(func() -> void:
		_search.open_for(hub_service.state.tab(current_tab) if hub_service != null else null))
	var header: HBoxContainer = _ui["header"] as HBoxContainer
	header.add_child(_search_button)
	header.move_child(_search_button, (_ui["close"] as Button).get_index())
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_tabs_box = UiBuild.hbox(&"ToolbarBox")
	_tabs_box.name = "Tabs"
	_tab_row = UiBuild.chip_scroller(_tabs_box)
	_tab_row.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	body.add_child(_tab_row)
	_notice = UiBuild.panel(&"InfoBannerPanel")
	_notice.name = "Notice"
	_notice.visible = false
	_notice_label = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_notice.add_child(_notice_label)
	body.add_child(_notice)
	var pages: Control = Control.new()
	pages.name = "Pages"
	pages.mouse_filter = Control.MOUSE_FILTER_PASS
	UiBuild.expand(pages, true, true)
	body.add_child(pages)
	_dock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	_dock.name = "Dock"
	_dock.hub_mode = true
	_dock.set_anchors_preset(Control.PRESET_FULL_RECT)
	_dock.visible = false
	pages.add_child(_dock)
	_friends = ChatFriendsList.new()
	_friends.set_anchors_preset(Control.PRESET_FULL_RECT)
	_friends.visible = false
	_friends.message_requested.connect(func(user_id: String) -> void:
		if hub_service != null:
			hub_service.open_dm(user_id))
	pages.add_child(_friends)
	_empty = FriendsUi.empty_line("ui.chat.hub.empty", "Empty")
	_empty.set_anchors_preset(Control.PRESET_TOP_WIDE)
	_empty.visible = false
	pages.add_child(_empty)
	_search = ChatSearchPanel.new()
	_search.set_anchors_preset(Control.PRESET_FULL_RECT)
	_search.visible = false
	_search.hit_chosen.connect(jump_to)
	pages.add_child(_search)
	_fit()
	UiBuild.size_buttons(self)
