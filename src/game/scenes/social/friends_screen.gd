class_name FriendsScreen
extends Control
## Bạn bè (F110 US4; STUDENT_GAME_UX_SOCIAL §6, S11; SOCIAL_SYSTEM §4, §5): the window opened by F, the HUD "Thêm" menu,
## the NPC Chú Ong Kết Nối or a friend-request notification (`ui_requested("friends", {tab})`). Four tabs:
## - "Bạn bè": Online / Offline groups (virtualised FriendListView — never grouped by class, ADR-0095), the presence in
##   words, "—" without network, redraws at most once a second (F110-AC-19, AC-20); "Ẩn trạng thái" (AC-26); the
##   profile with "Mời party", "Tắt tiếng", "Chặn", "Báo cáo" (F119 report sheet), "Hủy kết bạn" — each safety action
##   one tap + one confirmation with a reason picked by icon (≤ 2 taps, AC-24);
## - "Lời mời": incoming with Đồng ý / Từ chối, sent ones always "Đang chờ" (AC-21);
## - "Tìm bạn": "Ở gần đây" with "Kết bạn" and the friend code — no search field (AC-22, AC-23);
## - "Đã chặn": blocks and mutes with undo (AC-25).
## Only nicknames, portraits, levels and level titles of other players (R16); nothing on disk. UI only: SocialService /
## PresenceService / PartyService do the work (AGENTS R1).

signal closed()

const TAB_FRIENDS: String = "friends"
const TAB_REQUESTS: String = "requests"
const TAB_FIND: String = "find"
const TAB_BLOCKED: String = "blocked"
## [id, label key, icon, icon on the selected (blue) tab].
const TABS: Array = [
	[TAB_FRIENDS, "ui.friends.tab.friends", "group", "group"],
	[TAB_REQUESTS, "ui.friends.tab.requests", "invite", "invite_light"],
	[TAB_FIND, "ui.friends.tab.find", "search", "search_light"],
	[TAB_BLOCKED, "ui.friends.tab.blocked", "block", "block"],
]
const NOTICE_SEC: float = 4.0
## Block / mute reasons chosen by icon (lớp 1–3); local only — the block API takes none (SOCIAL_SYSTEM §5).
const RELATION_REASONS: Array = [
	["rude_language", "ui.chat.report.reason.rude_language", "report_rude"],
	["spam", "ui.chat.report.reason.spam", "report_spam"],
	["other", "ui.chat.report.reason.other", "help"],
]
const ACT_CONFIRM: StringName = &"confirm"

var services: ServiceLocator = null
var social: SocialService = null
var tab: String = TAB_FRIENDS
var selected_id: String = ""
var _ui: Dictionary = {}
var _tabs: Dictionary = {}
var _pages: Dictionary = {}
var _list: FriendListView = null
var _profile: FriendProfilePanel = null
var _empty: Control = null
var _profile_scroll: ScrollContainer = null
var _hidden_button: Button = null
var _hidden_state: Label = null
var _requests: FriendRequestList = null
var _find: FindFriendsTab = null
var _blocks: BlockList = null
var _notice: PanelContainer = null
var _notice_label: Label = null
var _notice_left: float = 0.0
var _sheet: ChatSheet = null
var _confirm: ChatSheet = null
var _dialogs: ChatDialogs = null
var _pending: Dictionary = {}


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)
	EventBus.friend_request_received.connect(_on_friend_request)


func _exit_tree() -> void:
	_unbind()


## WorldRoot: `ui_requested("friends", {tab})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_bind(services.social if services != null else null)
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	select_tab(String(args.get("tab", TAB_FRIENDS)))
	(_ui["close"] as Button).grab_focus.call_deferred(true)
	refresh()


func close() -> void:
	_sheet.dismiss()
	_confirm.dismiss()
	hide()
	_unbind()
	closed.emit()


## Coroutine: reads the friends and the requests (and the block lists on their tab).
func refresh() -> void:
	if social == null:
		return
	await social.load_friends()
	if not visible or social == null:
		return
	await social.load_requests()
	if visible and social != null and tab == TAB_BLOCKED:
		await social.load_relations()


func select_tab(next: String) -> void:
	tab = next if _pages.has(next) else TAB_FRIENDS
	for spec: Array in TABS:
		var id: String = String(spec[0])
		var b: Button = _tabs[id] as Button
		b.theme_type_variation = &"TabButtonSelected" if id == tab else &"TabButton"
		b.icon = UiBuild.icon(String(spec[3] if id == tab else spec[2]))
		b.set_pressed_no_signal(id == tab)
	for id: String in _pages.keys():
		(_pages[id] as Control).visible = id == tab
	if tab == TAB_BLOCKED and social != null and visible:
		social.load_relations()
	render()


func render() -> void:
	if social == null:
		return
	var s: FriendsStore = social.store
	(_tabs[TAB_FRIENDS] as Button).text = tr("ui.friends.tab.friends_count").format({"count": LocaleFormat.number(s.friends.size())})
	(_tabs[TAB_REQUESTS] as Button).text = tr("ui.friends.tab.requests_count").format({
		"count": LocaleFormat.number(s.incoming.size())}) if not s.incoming.is_empty() else tr("ui.friends.tab.requests")
	_render_friends()
	_requests.set_requests(s.incoming, s.outgoing)
	_render_find()
	_blocks.set_relations(s.blocks, s.mutes)


# ---------- test hooks ----------

func list_view() -> FriendListView:
	return _list


func profile() -> FriendProfilePanel:
	return _profile


func requests_view() -> FriendRequestList:
	return _requests


func find_view() -> FindFriendsTab:
	return _find


func block_view() -> BlockList:
	return _blocks


func confirm_sheet() -> ChatSheet:
	return _confirm


func report_sheet() -> ChatSheet:
	return _sheet


func hidden_button() -> Button:
	return _hidden_button


func tab_button(id: String) -> Button:
	return _tabs.get(id) as Button


func notice_text() -> String:
	return _notice_label.text if _notice.visible else ""


func select_friend(user_id: String) -> void:
	selected_id = user_id
	_render_friends()


## Every text the screen shows right now (tests: no real name, school, grade or learning result of another player).
func shown_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree():
			out.append(tr((node as Label).text))
	for node: Node in find_children("*", "Button", true, false):
		if (node as Button).is_visible_in_tree():
			out.append(tr((node as Button).text))
	return out


# ---------- per frame ----------

func _process(delta: float) -> void:
	if not visible or social == null:
		return
	social.store.flush(Time.get_ticks_msec())
	_find.code_widget.tick(social.now_ms())
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


# ---------- binding ----------

func _bind(next: SocialService) -> void:
	_unbind()
	social = next
	if social == null:
		return
	for pair: Array in _pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)
	_dialogs.bind(services.chat, social, services.reports)
	_profile.report_available = services.reports != null


func _unbind() -> void:
	if social == null:
		return
	for pair: Array in _pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	social = null


func _pairs() -> Array:
	var s: FriendsStore = social.store
	var out: Array = [[s.friends_changed, render], [s.requests_changed, render], [s.relations_changed, render],
		[s.code_changed, _on_code_changed], [social.notice_requested, show_notice], [social.busy_changed, _on_busy],
		[social.refreshed_after_gone, _on_gone]]
	if services != null and services.presence != null:
		out.append([services.presence.hidden_changed, _on_hidden_changed])
	return out


# ---------- rendering ----------

func _render_friends() -> void:
	var s: FriendsStore = social.store
	if s.friend(selected_id) == null:
		selected_id = ""
	_list.set_friends(s.online_friends(), s.offline_friends(), s.presence_unknown, selected_id)
	_list.visible = not s.friends.is_empty()
	_empty.visible = s.friends.is_empty() and s.friends_loaded
	_profile_scroll.visible = not s.friends.is_empty()
	# Compact: the profile gets half of the page, so two action buttons fit on a line (fewer lines to scroll).
	_profile_scroll.size_flags_stretch_ratio = 1.0 if AppState.compact else 0.8
	_profile.show_friend(s.friend(selected_id), s.presence_unknown)
	var hidden: bool = services != null and services.presence != null and services.presence.hidden
	_hidden_button.set_pressed_no_signal(hidden)
	_hidden_button.icon = UiBuild.icon("eye_off" if hidden else "eye")
	_hidden_state.text = "ui.friends.hidden_on" if hidden else "ui.friends.hidden_off"


func _render_find() -> void:
	var friends: Dictionary = {}
	for f: FriendModel in social.store.friends:
		friends[f.user_id] = true
	_pending.clear()
	for r: FriendRequestModel in social.store.outgoing:
		_pending[r.other.user_id] = true
	var players: Array[Dictionary] = []
	var hub: HubSync = services.hub if services != null else null
	if hub != null:
		for p: Variant in hub.room.players.values():
			var pp: PublicPlayerModel = p as PublicPlayerModel
			if pp != null and not friends.has(pp.user_id) and players.size() < 20:
				players.append({"user_id": pp.user_id, "nickname": pp.nickname, "avatar_config": pp.avatar_config})
	_find.set_nearby(players, _pending)


func show_notice(key: String, args: Dictionary = {}) -> void:
	_notice_label.text = TranslationServer.translate(key).format(args)
	_notice.visible = not key.is_empty()
	_notice_left = NOTICE_SEC


# ---------- actions ----------

func _on_profile_action(action: StringName, user_id: String) -> void:
	var f: FriendModel = social.store.friend(user_id) if social != null else null
	if f == null:
		return
	match action:
		FriendProfilePanel.ACT_INVITE:
			await social.invite_to_party(user_id)
		FriendProfilePanel.ACT_REPORT:
			_dialogs.context = {}
			_dialogs.open_report(ReportService.TARGET_USER, user_id, f.nickname, user_id)
		FriendProfilePanel.ACT_MUTE, FriendProfilePanel.ACT_BLOCK:
			_ask_relation(action, f)
		FriendProfilePanel.ACT_REMOVE:
			_ask_remove(f)


## One confirmation with the reason chosen by an icon chip: tapping a reason IS the confirmation (2nd tap).
func _ask_relation(action: StringName, f: FriendModel) -> void:
	var block: bool = action == FriendProfilePanel.ACT_BLOCK
	var title_key: String = "ui.friends.block_confirm" if block else "ui.friends.mute_confirm"
	var body: VBoxContainer = _confirm.open_sheet(TranslationServer.translate(title_key).format({"nickname": f.nickname}), true)
	body.add_child(UiBuild.keep_words(UiBuild.label("ui.friends.block_hint" if block else "ui.friends.mute_hint")))
	body.add_child(UiBuild.label("ui.friends.reason_question", &"MutedLabel"))
	var reasons: HFlowContainer = HFlowContainer.new()
	reasons.name = "Reasons"
	body.add_child(reasons)
	var id: String = f.user_id
	var nick: String = f.nickname
	for spec: Array in RELATION_REASONS:
		var chip: Button = UiBuild.button(String(spec[1]), String(spec[2]), &"ChoiceChip")
		chip.name = "Reason_" + String(spec[0])
		chip.pressed.connect(func() -> void:
			_confirm.dismiss()
			_relation(block, id, nick))
		reasons.add_child(chip)
	_confirm.add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	UiBuild.size_buttons(body)


func _relation(block: bool, user_id: String, nickname: String) -> void:
	var result: ApiResult = null
	if block:
		result = await social.block_user(user_id)
	else:
		result = await social.mute_user(user_id)
	if result.ok:
		if block and selected_id == user_id:
			selected_id = ""
		show_notice("ui.friends.blocked_done" if block else "ui.friends.muted_done", {"nickname": nickname})
		render()


func _ask_remove(f: FriendModel) -> void:
	var body: VBoxContainer = _confirm.open_sheet(TranslationServer.translate("ui.friends.remove_confirm").format(
			{"nickname": f.nickname}), true)
	body.add_child(UiBuild.keep_words(UiBuild.label("ui.friends.remove_hint")))
	_confirm.add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	_confirm.add_button(ACT_CONFIRM, "ui.friends.remove", "check", &"")
	_confirm.set_meta("remove_id", f.user_id)


func _on_confirm_chosen(action: StringName) -> void:
	if action != ACT_CONFIRM or social == null:
		return
	var id: String = String(_confirm.get_meta("remove_id", ""))
	var result: ApiResult = await social.remove_friend(id)
	if result.ok:
		selected_id = ""
		show_notice("ui.friends.removed_done")


func _on_hidden_toggled(on: bool) -> void:
	if services == null or services.presence == null:
		return
	if not await services.presence.set_hidden(on):
		show_notice("ui.friends.offline")
	_render_friends()


func _on_hidden_changed(_hidden: bool) -> void:
	if visible:
		_render_friends()


func _on_friend_selected(user_id: String) -> void:
	select_friend(user_id)


func _on_code_changed() -> void:
	if social != null:
		_find.code_widget.show_code(social.store.code, social.now_ms())


func _on_busy(busy: bool) -> void:
	_profile.set_busy(busy)


func _on_gone() -> void:
	selected_id = ""
	show_notice("ui.friends.gone")
	render()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		render()


func _on_add_requested(user_id: String) -> void:
	if social != null:
		await social.send_friend_request(user_id)
		render()


func _on_create_code() -> void:
	if social != null:
		await social.create_code()


func _on_redeem_code(code: String) -> void:
	if social != null:
		await social.redeem_code(code)


func _on_unblock(user_id: String) -> void:
	if social != null:
		await social.unblock_user(user_id)


func _on_unmute(user_id: String) -> void:
	if social != null:
		await social.unmute_user(user_id)


func _on_accept(request: FriendRequestModel) -> void:
	if social != null:
		await social.accept(request)


func _on_reject(request: FriendRequestModel) -> void:
	if social != null:
		await social.reject(request)


func _on_friend_request(_request_id: String) -> void:
	if visible:
		render()


# ---------- build ----------

func _build() -> void:
	_ui = UiBuild.window(self, "ui.friends.title", "group")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var tabs: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	tabs.name = "Tabs"
	var group: ButtonGroup = ButtonGroup.new()
	for spec: Array in TABS:
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), &"TabButton")
		b.name = "Tab_" + String(spec[0])
		b.toggle_mode = true
		b.button_group = group
		b.pressed.connect(select_tab.bind(String(spec[0])))
		tabs.add_child(b)
		_tabs[spec[0]] = b
	body.add_child(UiBuild.chip_scroller(tabs))
	_notice = UiBuild.panel(&"InfoBannerPanel")
	_notice.name = "Notice"
	_notice.visible = false
	_notice_label = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_notice.add_child(_notice_label)
	body.add_child(_notice)
	var pages: Control = UiBuild.vbox()
	pages.name = "Pages"
	UiBuild.expand(pages, true, true)
	body.add_child(pages)
	_pages[TAB_FRIENDS] = _build_friends(pages)
	_requests = FriendRequestList.new()
	_pages[TAB_REQUESTS] = _scroll_page(pages, _requests, "RequestsPage")
	_find = FindFriendsTab.new()
	_find.add_requested.connect(_on_add_requested)
	_find.code_widget.create_requested.connect(_on_create_code)
	_find.code_widget.redeem_requested.connect(_on_redeem_code)
	_pages[TAB_FIND] = _scroll_page(pages, _find, "FindPage")
	_blocks = BlockList.new()
	_blocks.unblock_requested.connect(_on_unblock)
	_blocks.unmute_requested.connect(_on_unmute)
	_pages[TAB_BLOCKED] = _scroll_page(pages, _blocks, "BlockedPage")
	_requests.accept_requested.connect(_on_accept)
	_requests.reject_requested.connect(_on_reject)
	_sheet = ChatSheet.new()
	add_child(_sheet)
	_dialogs = ChatDialogs.new(_sheet)
	_confirm = ChatSheet.new()
	_confirm.name = "ConfirmSheet"
	_confirm.chosen.connect(_on_confirm_chosen)
	add_child(_confirm)
	UiBuild.size_buttons(self)


func _build_friends(parent: Control) -> Control:
	var page: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	page.name = "FriendsPage"
	UiBuild.expand(page, true, true)
	parent.add_child(page)
	var left: VBoxContainer = UiBuild.vbox(&"FormBox")
	left.name = "Left"
	UiBuild.expand(left, true, true)
	page.add_child(left)
	var me: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	me.name = "Me"
	left.add_child(me)
	_hidden_button = UiBuild.button("ui.friends.hide_status", "eye", &"SecondaryButton")
	_hidden_button.name = "HideStatus"
	_hidden_button.toggle_mode = true
	_hidden_button.toggled.connect(_on_hidden_toggled)
	me.add_child(_hidden_button)
	_hidden_state = UiBuild.keep_words(UiBuild.label("ui.friends.hidden_off", &"MutedLabel"))
	_hidden_state.name = "HiddenState"
	_hidden_state.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_hidden_state)
	me.add_child(_hidden_state)
	_list = FriendListView.new()
	_list.name = "FriendList"
	_list.friend_selected.connect(_on_friend_selected)
	left.add_child(_list)
	var empty_box: VBoxContainer = UiBuild.vbox(&"FormBox")
	empty_box.name = "EmptyBox"
	empty_box.add_child(FriendsUi.empty_line("ui.friends.empty"))
	var find: Button = UiBuild.button("ui.friends.tab.find", "search_light")
	find.name = "GoFind"
	find.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	find.pressed.connect(select_tab.bind(TAB_FIND))
	empty_box.add_child(find)
	_empty = empty_box
	_empty.visible = false
	left.add_child(_empty)
	var right: ScrollContainer = ScrollContainer.new()
	right.name = "ProfileScroll"
	_profile_scroll = right
	right.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	right.size_flags_vertical = Control.SIZE_EXPAND_FILL
	right.size_flags_stretch_ratio = 0.8
	UiBuild.expand(right, true, true)
	page.add_child(right)
	_profile = FriendProfilePanel.new()
	UiBuild.expand(_profile)
	_profile.action_requested.connect(_on_profile_action)
	right.add_child(_profile)
	return page


func _scroll_page(parent: Control, content: Control, page_name: String) -> Control:
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = page_name
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	UiBuild.expand(content)
	scroll.add_child(content)
	parent.add_child(scroll)
	return scroll
