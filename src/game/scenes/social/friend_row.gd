class_name FriendRow
extends PanelContainer
## One friend of the friends list (F110-AC-19, AC-20; STUDENT_GAME_UX_SOCIAL §6 `FriendList`): a card with the 2D
## portrait, the nickname, "Cấp n" and the presence as an icon AND words (never colour alone, F110-AC-28); "—" while the
## network is gone; a big "Xem" chip (✓ when this friend's profile is open). Tapping the card or the chip opens the
## profile. Nickname / avatar / level only — no real name, school, grade or learning result (AGENTS R16). A pooled row
## of FriendListView: `bind` re-fills it while scrolling. UI only (AGENTS R1).

signal chosen(user_id: String)

var user_id: String = ""
var view_button: Button = null
var _thumb: AvatarThumb = null
var _name: Label = null
var _level: Label = null
var _presence_icon: TextureRect = null
var _presence: Label = null


func _init() -> void:
	theme_type_variation = &"PartyMemberCard"
	mouse_filter = Control.MOUSE_FILTER_STOP
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.name = "Row"
	add_child(row)
	_thumb = AvatarThumb.new()
	_thumb.standalone = true
	_thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	row.add_child(texts)
	var top: HFlowContainer = HFlowContainer.new()
	texts.add_child(top)
	_name = UiBuild.data_label("", &"HeadingLabel", false)
	_name.name = "Nickname"
	top.add_child(_name)
	_level = UiBuild.data_label("", &"MutedLabel", false)
	_level.name = "Level"
	_level.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	top.add_child(_level)
	var bottom: HBoxContainer = UiBuild.hbox()
	texts.add_child(bottom)
	_presence_icon = UiBuild.icon_rect("presence_online", 0.7)
	_presence_icon.name = "PresenceIcon"
	_presence_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	bottom.add_child(_presence_icon)
	_presence = UiBuild.data_label("", &"InstructionLabel", false)
	_presence.name = "Presence"
	UiBuild.expand(_presence)
	bottom.add_child(_presence)
	view_button = UiBuild.chip("ui.friends.view")
	view_button.name = "View"
	view_button.icon = UiBuild.icon("arrow_right")
	view_button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	view_button.pressed.connect(func() -> void: chosen.emit(user_id))
	row.add_child(view_button)
	gui_input.connect(_on_gui_input)


func bind(f: FriendModel, presence_unknown: bool, selected: bool) -> void:
	user_id = f.user_id
	var side: float = UiBuild.target()
	_thumb.custom_minimum_size = Vector2(side, side)
	var look: AvatarConfigModel = f.avatar()
	_thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	_name.text = f.nickname
	_level.text = tr("ui.friends.level").format({"level": LocaleFormat.number(f.level)}) if f.level > 0 else ""
	_level.visible = f.level > 0
	if presence_unknown:
		_presence_icon.texture = UiBuild.icon("wifi_off")
		_presence.text = tr("ui.friends.presence.unknown")
	else:
		_presence_icon.texture = UiBuild.icon(f.presence_icon())
		_presence.text = tr(f.presence_key())
	view_button.set_pressed_no_signal(selected)
	view_button.icon = UiBuild.icon("check" if selected else "arrow_right")
	view_button.custom_minimum_size = Vector2(UiBuild.target(), UiBuild.target())
	accessibility_name = "%s, %s" % [f.nickname, _presence.text]
	tooltip_text = accessibility_name


## Height the row content needs (the list grows its rows to it).
func content_height() -> float:
	return get_combined_minimum_size().y


func texts() -> PackedStringArray:
	return PackedStringArray([_name.text, _level.text, _presence.text])


func _on_gui_input(event: InputEvent) -> void:
	var tap: bool = (event is InputEventMouseButton and (event as InputEventMouseButton).pressed
			and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT) \
			or (event is InputEventScreenTouch and (event as InputEventScreenTouch).pressed)
	if tap:
		accept_event()
		chosen.emit(user_id)
