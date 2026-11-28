class_name ClubCard
extends PanelContainer
## One club of the club tab (F111-AC-19; SOCIAL_SYSTEM §6, STUDENT_GAME_UX_SOCIAL §6): the club icon, name, type in words,
## "{n}/{max} thành viên" (server numbers) and the state in words with an icon (`pending_approval` → "Đang chờ nhà
## trường duyệt", locked / archived → view only) — never colour alone; "Xem" for a club of the student, "Xin vào" (then
## "Đang chờ duyệt") for a suggested one. UI only (AGENTS R1).

signal opened(club_id: String)
signal join_requested(club_id: String)

var club_id: String = ""
var _icon: TextureRect = null
var _name: Label = null
var _type: Label = null
var _count: Label = null
var _status: HBoxContainer = null
var _actions: HFlowContainer = null
var _view: Button = null
var _join: Button = null


func _init() -> void:
	name = "ClubCard"
	theme_type_variation = &"PartyMemberCard"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(row)
	_icon = UiBuild.icon_rect("flag", 1.3)
	_icon.name = "ClubIcon"
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(UiBuild.icon_badge(_icon))
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(texts)
	_name = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_name.name = "ClubName"
	texts.add_child(_name)
	var line: HFlowContainer = HFlowContainer.new()
	texts.add_child(line)
	_type = UiBuild.label("", &"MutedLabel", false)
	_type.name = "ClubType"
	line.add_child(_type)
	_count = UiBuild.data_label("", &"SectionTag", false)
	_count.name = "Members"
	_count.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	line.add_child(_count)
	_status = UiBuild.hbox()
	_status.name = "StatusRow"
	texts.add_child(_status)
	_actions = HFlowContainer.new()
	_actions.name = "Actions"
	_actions.alignment = FlowContainer.ALIGNMENT_END
	_actions.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_actions)
	_view = UiBuild.button("ui.club.open", "arrow_right", &"SecondaryButton")
	_view.name = "View"
	_view.pressed.connect(func() -> void: opened.emit(club_id))
	_actions.add_child(_view)
	_join = UiBuild.button("ui.club.join_request", "invite_light")
	_join.name = "Join"
	_join.pressed.connect(func() -> void:
		_join.disabled = true
		join_requested.emit(club_id))
	_actions.add_child(_join)


func show_club(c: ClubModel, requested: bool) -> void:
	club_id = c.club_id
	_icon.texture = UiBuild.icon(c.icon_name())
	_name.text = c.name
	_type.text = c.type_key()
	_count.text = tr("ui.club.members_count").format({"count": LocaleFormat.number(c.member_count),
		"max": LocaleFormat.number(c.max_members)})
	UiBuild.clear(_status)
	var state_key: String = ""
	var state_icon: String = ""
	if c.is_pending():
		state_key = "ui.club.status.pending_approval"
		state_icon = "clock"
	elif c.is_read_only():
		state_key = c.status_key()
		state_icon = "lock"
	elif not c.is_member() and requested:
		state_key = "ui.club.request_pending"
		state_icon = "clock"
	if not state_key.is_empty():
		_status.add_child(FriendsUi.status(state_key, state_icon))
	_status.visible = not state_key.is_empty()
	_view.visible = c.is_member()
	_join.visible = not c.is_member() and c.is_active() and not requested and not c.is_full()
	_join.disabled = false
	accessibility_name = c.name
	UiBuild.size_buttons(self)


func join_button() -> Button:
	return _join


func view_button() -> Button:
	return _view


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).visible:
			out.append(tr((node as Label).text))
	return out
