class_name ClubPage
extends VBoxContainer
## The page of one club (F111-AC-20, AC-21, AC-24, AC-25; SOCIAL_SYSTEM §6, ADR-0050, AGENTS R16): the club icon, name,
## type, "{n}/{max} thành viên", the student's club role and the state in words ("Đang chờ nhà trường duyệt", view only
## when locked / archived); the actions "Chat club" (active, with a channel), "Mời bạn" (owner / officer of an active club),
## "Xin vào" (not a member yet), "Rời club" (a member who is not the owner — one confirmation); section chips "Thành
## viên" (nickname, portrait, role — nothing else of a member), "Mục tiêu" (the group bar + "Em đã góp {n}", never another
## member's share), "Lịch" and, for an owner / officer, "Yêu cầu" (Chấp nhận / Từ chối). The role only decides what is
## SHOWN — the server decides every action (F120). UI only (AGENTS R1).

signal back_requested()
signal action_requested(action: StringName, club_id: String)
signal request_answered(request_id: String, accept: bool)

const ACT_CHAT: StringName = &"chat"
const ACT_INVITE: StringName = &"invite"
const ACT_JOIN: StringName = &"join"
const ACT_LEAVE: StringName = &"leave"
## [action, label key, icon, variation].
const ACTIONS: Array = [
	[ACT_CHAT, "ui.club.chat", "chat", &"SecondaryButton"],
	[ACT_INVITE, "ui.club.invite", "invite_light", &""],
	[ACT_JOIN, "ui.club.join_request", "invite_light", &""],
	[ACT_LEAVE, "ui.club.leave", "exit", &"SecondaryButton"],
]
## F111-AC-32: members shown a page at a time ("Xem thêm thành viên" adds the next page) — a 50-member club stays light.
const MEMBERS_PAGE: int = 20
const SECTION_MEMBERS: String = "members"
const SECTION_GOALS: String = "goals"
const SECTION_EVENTS: String = "events"
const SECTION_REQUESTS: String = "requests"
const SECTIONS: Array = [
	[SECTION_MEMBERS, "ui.club.section.members", "group"],
	[SECTION_GOALS, "ui.club.section.goals", "flag"],
	[SECTION_EVENTS, "ui.club.section.events", "calendar"],
	[SECTION_REQUESTS, "ui.club.requests", "invite"],
]

var club_id: String = ""
var section: String = SECTION_MEMBERS
var _icon: TextureRect = null
var _name: Label = null
var _meta: Label = null
var _count: Label = null
var _role: Label = null
var _state: HBoxContainer = null
var _read_only: PanelContainer = null
var _buttons: Dictionary = {}
var _chips: Dictionary = {}
var _content: VBoxContainer = null
var _requests: Dictionary = {}
var _store: ClubStore = null
var _members_shown: int = MEMBERS_PAGE
var _chip_icons: Dictionary = {}


func _init() -> void:
	name = "ClubPage"
	theme_type_variation = &"FormBox"
	# A flow: on a narrow phone window or a long translation the actions move under the title instead of widening it.
	var head: HFlowContainer = HFlowContainer.new()
	head.name = "Head"
	add_child(head)
	var back: Button = UiBuild.button("ui.club.back", "arrow_left", &"SecondaryButton")
	back.name = "Back"
	back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	back.pressed.connect(func() -> void: back_requested.emit())
	head.add_child(back)
	_icon = UiBuild.icon_rect("flag", 1.3)
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(UiBuild.icon_badge(_icon))
	var titles: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(titles)
	titles.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(titles)
	_name = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_name.name = "ClubName"
	titles.add_child(_name)
	var line: HFlowContainer = HFlowContainer.new()
	titles.add_child(line)
	_meta = UiBuild.label("", &"MutedLabel", false)
	_meta.name = "ClubType"
	line.add_child(_meta)
	_count = UiBuild.data_label("", &"SectionTag", false)
	_count.name = "Members"
	_count.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	line.add_child(_count)
	_role = UiBuild.label("", &"SectionTag", false)
	_role.name = "MyRole"
	_role.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	line.add_child(_role)
	_state = UiBuild.hbox()
	_state.name = "State"
	titles.add_child(_state)
	_read_only = UiBuild.panel(&"WarningBannerPanel")
	_read_only.name = "ReadOnly"
	_read_only.add_child(UiBuild.keep_words(UiBuild.label("ui.club.read_only", &"WarningBannerLabel")))
	add_child(_read_only)
	# The actions sit at the right end of the header (a flow that wraps on a narrow window) so the sections and their
	# content start higher on a phone (F125 compact).
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	actions.alignment = FlowContainer.ALIGNMENT_END
	actions.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(actions)
	head.add_child(actions)
	for spec: Array in ACTIONS:
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), spec[3] as StringName)
		b.name = String(spec[0]).capitalize()
		var act: StringName = spec[0] as StringName
		b.pressed.connect(func() -> void: action_requested.emit(act, club_id))
		actions.add_child(b)
		_buttons[act] = b
	var chips: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	chips.name = "Sections"
	var group: ButtonGroup = ButtonGroup.new()
	for spec: Array in SECTIONS:
		var c: Button = UiBuild.chip(String(spec[1]), group)
		c.name = "Section_" + String(spec[0])
		c.icon = UiBuild.icon(String(spec[2]))
		_chip_icons[c] = c.icon
		c.pressed.connect(select_section.bind(String(spec[0])))
		chips.add_child(c)
		_chips[spec[0]] = c
	add_child(UiBuild.chip_scroller(chips))
	_content = UiBuild.vbox(&"FormBox")
	_content.name = "Content"
	add_child(_content)


func show_page(store: ClubStore, requests_denied: bool, chat_available: bool) -> void:
	_store = store
	var c: ClubModel = store.page
	if c == null:
		return
	if club_id != c.club_id:
		_members_shown = MEMBERS_PAGE
	club_id = c.club_id
	_icon.texture = UiBuild.icon(c.icon_name())
	_name.text = c.name
	_meta.text = c.type_key()
	_count.text = tr("ui.club.members_count").format({"count": LocaleFormat.number(c.member_count),
		"max": LocaleFormat.number(c.max_members)})
	_role.text = "ui.club.role.%s" % c.my_role if c.is_member() else ""
	_role.visible = c.is_member()
	UiBuild.clear(_state)
	if c.is_pending():
		_state.add_child(FriendsUi.status("ui.club.status.pending_approval", "clock"))
	elif not c.is_member() and store.is_requested(c.club_id):
		_state.add_child(FriendsUi.status("ui.club.request_pending", "clock"))
	_state.visible = _state.get_child_count() > 0
	_read_only.visible = c.is_read_only()
	var open: bool = c.is_active()
	(_buttons[ACT_CHAT] as Button).visible = open and c.is_member() and not c.chat_channel_id.is_empty() and chat_available
	(_buttons[ACT_INVITE] as Button).visible = open and c.is_officer()
	(_buttons[ACT_JOIN] as Button).visible = open and not c.is_member() and not store.is_requested(c.club_id) and not c.is_full()
	(_buttons[ACT_LEAVE] as Button).visible = c.is_member() and not c.is_owner()
	var can_requests: bool = c.is_officer() and not requests_denied and not c.is_read_only()
	(_chips[SECTION_REQUESTS] as Button).visible = can_requests
	(_chips[SECTION_REQUESTS] as Button).text = tr("ui.club.requests") if store.requests.is_empty() else \
			"%s (%s)" % [tr("ui.club.requests"), LocaleFormat.number(store.requests.size())]
	if section == SECTION_REQUESTS and not can_requests:
		section = SECTION_MEMBERS
	for id: String in _chips.keys():
		(_chips[id] as Button).set_pressed_no_signal(id == section)
	UiBuild.mark_chips(_chips.values(), _chip_icons)
	_render_section()
	UiBuild.size_buttons(self)


func select_section(id: String) -> void:
	section = id
	for k: String in _chips.keys():
		(_chips[k] as Button).set_pressed_no_signal(k == section)
	UiBuild.mark_chips(_chips.values(), _chip_icons)
	_render_section()


func set_busy(busy: bool) -> void:
	for b: Button in _buttons.values():
		b.disabled = busy
	for pair: Variant in _requests.values():
		for b: Button in pair as Array:
			b.disabled = busy


func action_button(action: StringName) -> Button:
	return _buttons.get(action) as Button


func section_button(id: String) -> Button:
	return _chips.get(id) as Button


func read_only_visible() -> bool:
	return _read_only.visible


func request_button(request_id: String, accept: bool) -> Button:
	var pair: Array = _requests.get(request_id, []) as Array
	return (pair[0] if accept else pair[1]) as Button if pair.size() == 2 else null


func section_texts() -> PackedStringArray:
	return _texts_of(_content)


func all_texts() -> PackedStringArray:
	return _texts_of(self)


func _texts_of(root: Node) -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in root.find_children("*", "Label", true, false):
		if (node as Label).visible and not node.is_queued_for_deletion():
			out.append(tr((node as Label).text))
	return out


func _render_section() -> void:
	UiBuild.clear(_content)
	_requests.clear()
	if _store == null:
		return
	match section:
		SECTION_MEMBERS:
			for i: int in mini(_members_shown, _store.members.size()):
				_content.add_child(_member_row(_store.members[i]))
			if _store.members.size() > _members_shown:
				var more: Button = UiBuild.button("ui.club.members_more", "arrow_down", &"SecondaryButton")
				more.name = "MoreMembers"
				more.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
				more.pressed.connect(func() -> void:
					_members_shown += MEMBERS_PAGE
					_render_section())
				_content.add_child(more)
		SECTION_GOALS:
			for g: ClubGoalModel in _store.goals:
				_content.add_child(_goal_card(g))
			if _store.goals.is_empty():
				_content.add_child(FriendsUi.empty_line("ui.club.goals_none"))
		SECTION_EVENTS:
			for e: ClubEventModel in _store.calendar:
				_content.add_child(_event_row(e))
			if _store.calendar.is_empty():
				_content.add_child(FriendsUi.empty_line("ui.club.events_none"))
		SECTION_REQUESTS:
			for r: ClubJoinRequestModel in _store.requests:
				_content.add_child(_request_row(r))
			if _store.requests.is_empty():
				_content.add_child(FriendsUi.empty_line("ui.club.requests_none"))
	UiBuild.size_buttons(_content)


func _member_row(m: ClubMemberModel) -> Control:
	var ui: Dictionary = FriendsUi.person_card(m.nickname, {}, "Member")
	(ui["actions"] as HFlowContainer).add_child(FriendsUi.status(m.role_key(), m.role_icon()))
	return ui["card"] as Control


func _goal_card(g: ClubGoalModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"FormCard")
	card.name = "Goal"
	var box: VBoxContainer = UiBuild.vbox()
	card.add_child(box)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(head)
	var title: Label = UiBuild.keep_words(UiBuild.data_label(g.display_title(), &"HeadingLabel"))
	title.name = "GoalTitle"
	UiBuild.expand(title)
	head.add_child(title)
	var value: Label = UiBuild.data_label(tr("ui.club.goal.progress").format({"progress": LocaleFormat.number(g.progress),
		"target": LocaleFormat.number(g.target)}), &"SectionTag", false)
	value.name = "GoalValue"
	value.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(value)
	var bar: ProgressBar = UiBuild.progress_bar(g.ratio())
	UiBuild.expand(bar)
	box.add_child(bar)
	var mine: HBoxContainer = FriendsUi.status("ui.club.goal.mine", "heart")
	(mine.get_node("StatusLabel") as Label).text = tr("ui.club.goal.mine").format({"n": LocaleFormat.number(g.my_contribution)})
	box.add_child(mine)
	return card


func _event_row(e: ClubEventModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "ClubEvent"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("calendar", 1.0)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	var title: Label = UiBuild.keep_words(UiBuild.data_label(e.title, &"HeadingLabel"))
	title.name = "EventTitle"
	texts.add_child(title)
	var at: int = LiveOpsTime.unix_of(e.starts_at)
	if at > 0:
		texts.add_child(UiBuild.data_label(tr("ui.club.event_when").format({"date": LiveOpsTime.ict_date(at),
			"time": LiveOpsTime.ict_time(at)}), &"MutedLabel", false))
	if not e.description.is_empty():
		texts.add_child(UiBuild.keep_words(UiBuild.data_label(e.description, &"InstructionLabel")))
	return card


func _request_row(r: ClubJoinRequestModel) -> Control:
	var ui: Dictionary = FriendsUi.person_card(r.requester_nickname, {}, "Request")
	var nick: Label = (ui["card"] as Control).find_child("Nickname", true, false) as Label
	nick.text = tr("ui.club.request_line").format({"nickname": r.requester_nickname})
	var actions: HFlowContainer = ui["actions"] as HFlowContainer
	var accept: Button = UiBuild.button("ui.club.request_accept", "check")
	accept.name = "Accept"
	accept.pressed.connect(func() -> void: request_answered.emit(r.request_id, true))
	actions.add_child(accept)
	var reject: Button = UiBuild.button("ui.club.request_reject", "cancel", &"SecondaryButton")
	reject.name = "Reject"
	reject.pressed.connect(func() -> void: request_answered.emit(r.request_id, false))
	actions.add_child(reject)
	_requests[r.request_id] = [accept, reject]
	return ui["card"] as Control
