class_name PartyMemberCard
extends PanelContainer
## PartyMemberCard (F102-AC-06, AC-13; STUDENT_GAME_UX_SOCIAL §5): one member of the party — a 2D portrait drawn from the
## avatar config (AvatarThumb, no 3D in the lobby), the nickname ("(em)" for the student), the crown + "Trưởng nhóm"
## for the leader, the level when the server sent it, the presence as an icon + words and the Ready state as a tag with
## words (never colour alone). A member who lost the connection gets the faded card `PartyMemberCardAway` with
## "Đang kết nối lại…". Only nickname, avatar and level of another student are shown (AGENTS R16). The leader sees a
## "…" button on the other cards to open the leader menu (also right click / long press). UI only: `bind()` with the
## store model, `refresh()` when the store says this member changed (the lobby redraws only this card, AC-UX-104).

signal menu_requested(student_id: String)

const PORTRAIT_SCALE: float = 1.25
const SMALL_ICON_SCALE: float = 0.75
## Seconds of a press that opens the leader menu on touch screens (long press, STUDENT_GAME_UX_SOCIAL §5).
const LONG_PRESS_SEC: float = 0.6
## Neutral look while a member has no avatar config (catalog codes of the F070 presets; display only).
const DEFAULT_AVATAR: Dictionary = {
	"body_type": "kid_a", "skin_tone": "tone_03", "hair": {"item_id": "hair_short_01", "color": "hair_col_02"},
	"outfit": {"item_id": "outfit_school_01"},
}
const PRESENCE_ICONS: Dictionary = {
	"online": "presence_online", "away": "moon", "in_activity": "play", "disconnected": "wifi_off", "offline": "wifi_off",
}
const PRESENCE_KEYS: Dictionary = {
	"online": "ui.party.presence.online", "away": "ui.party.presence.away", "in_activity": "ui.party.presence.in_activity",
	"disconnected": "ui.party.presence.disconnected", "offline": "ui.party.presence.offline",
}

var member: PartyMemberModel = null
var is_self: bool = false
## The viewer is the leader and this card is another member: the "…" leader menu is offered.
var leader_menu: bool = false
## Number of renders (tests: a change of another member never redraws this card).
var renders: int = 0
var _thumb: AvatarThumb = null
var _crown: TextureRect = null
var _name: Label = null
var _leader_tag: Label = null
var _presence_icon: TextureRect = null
var _presence: Label = null
var _level: Label = null
var _ready_tag: Label = null
var _menu: Button = null
var _press_left: float = -1.0


func _init() -> void:
	theme_type_variation = &"PartyMemberCard"
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func bind(model: PartyMemberModel, self_card: bool, show_leader_menu: bool) -> void:
	member = model
	is_self = self_card
	leader_menu = show_leader_menu
	name = "Member_%s" % model.student_id.right(12) if model != null else "Member"
	var look: AvatarConfigModel = model.avatar() if model != null else null
	_thumb.set_look(look if look != null else AvatarConfigModel.from_dict(DEFAULT_AVATAR))
	refresh()


## Redraws the texts and tags from `member` (the store changed it in place).
func refresh() -> void:
	if member == null:
		return
	renders += 1
	var faded: bool = member.is_disconnected()
	theme_type_variation = &"PartyMemberCardAway" if faded else &"PartyMemberCard"
	var text_variation: StringName = &"PartyAwayLabel" if faded else &""
	_name.text = TranslationServer.translate("ui.party.member_self").format({"nickname": member.nickname}) \
			if is_self else member.nickname
	_name.tooltip_text = _name.text
	_name.theme_type_variation = text_variation if faded else &"HeadingLabel"
	_crown.visible = member.is_leader()
	_leader_tag.visible = member.is_leader()
	_presence_icon.texture = UiBuild.icon(String(PRESENCE_ICONS.get(member.presence, "presence_online")))
	_presence.text = String(PRESENCE_KEYS.get(member.presence, "ui.party.presence.online"))
	_presence.theme_type_variation = text_variation if faded else &"MutedLabel"
	_level.visible = member.level > 0
	_level.text = TranslationServer.translate("ui.party.level").format({"level": member.level})
	_level.theme_type_variation = text_variation if faded else &"MutedLabel"
	_ready_tag.visible = not faded
	_ready_tag.text = "ui.party.ready_on" if member.ready else "ui.party.ready_off"
	_ready_tag.theme_type_variation = &"TileTagCorrect" if member.ready else &"TileTagPending"
	_menu.visible = leader_menu
	_menu.custom_minimum_size = Vector2(UiBuild.target(), UiBuild.target())


func presence_text() -> String:
	return tr(_presence.text)


func ready_text() -> String:
	return tr(_ready_tag.text) if _ready_tag.visible else ""


func is_faded() -> bool:
	return theme_type_variation == &"PartyMemberCardAway"


func shows_crown() -> bool:
	return _crown.visible


## Texts on the card (tests: only nickname, level, presence, Ready — no real name, school or grade).
func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Label in [_name, _leader_tag, _presence, _level, _ready_tag]:
		if l.visible:
			out.append(tr(l.text) if l.auto_translate_mode != Node.AUTO_TRANSLATE_MODE_DISABLED else l.text)
	return out


func menu_button() -> Button:
	return _menu


func _gui_input(event: InputEvent) -> void:
	if not leader_menu or member == null:
		return
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.button_index == MOUSE_BUTTON_RIGHT and mb.pressed:
			menu_requested.emit(member.student_id)
			accept_event()
	elif event is InputEventScreenTouch:
		var touch: InputEventScreenTouch = event as InputEventScreenTouch
		_press_left = LONG_PRESS_SEC if touch.pressed else -1.0


func _process(delta: float) -> void:
	if _press_left < 0.0:
		return
	_press_left -= delta
	if _press_left <= 0.0:
		_press_left = -1.0
		if leader_menu and member != null:
			menu_requested.emit(member.student_id)


func _on_menu_pressed() -> void:
	if member != null:
		menu_requested.emit(member.student_id)


func _on_locale_changed(_locale: String) -> void:
	refresh()


func _build() -> void:
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.name = "Row"
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(row)
	_thumb = AvatarThumb.new()
	_thumb.standalone = true
	_thumb.name = "Portrait"
	var side: float = UiBuild.target() * PORTRAIT_SCALE
	_thumb.custom_minimum_size = Vector2(side, side)
	_thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_thumb)
	var texts_box: VBoxContainer = UiBuild.vbox()
	texts_box.name = "Texts"
	texts_box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	texts_box.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts_box)
	row.add_child(texts_box)
	var name_line: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	name_line.mouse_filter = Control.MOUSE_FILTER_IGNORE
	texts_box.add_child(name_line)
	_crown = UiBuild.icon_rect("crown")
	_crown.name = "Crown"
	_crown.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	name_line.add_child(_crown)
	_name = UiBuild.data_label("", &"HeadingLabel", false)
	_name.name = "Nickname"
	# Wraps between words on a narrow card (a long translation of "(em)"), never cut.
	_name.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_name.mouse_filter = Control.MOUSE_FILTER_PASS
	UiBuild.expand(_name)
	name_line.add_child(_name)
	# One flowing line under the nickname: leader tag, presence (icon + words), level, Ready tag — it wraps on a narrow
	# card (phone) instead of cutting the nickname, and keeps 3–4 cards on screen without scrolling on PC.
	var info: HFlowContainer = HFlowContainer.new()
	info.name = "Info"
	info.mouse_filter = Control.MOUSE_FILTER_IGNORE
	texts_box.add_child(info)
	_leader_tag = UiBuild.label("ui.party.leader", &"SectionTag", false)
	_leader_tag.name = "LeaderTag"
	_leader_tag.uppercase = true
	_leader_tag.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	info.add_child(_leader_tag)
	var presence_box: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	presence_box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	presence_box.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	info.add_child(presence_box)
	_presence_icon = UiBuild.icon_rect("presence_online", SMALL_ICON_SCALE)
	_presence_icon.name = "PresenceIcon"
	_presence_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	presence_box.add_child(_presence_icon)
	_presence = UiBuild.label("", &"MutedLabel", false)
	_presence.name = "Presence"
	presence_box.add_child(_presence)
	_level = UiBuild.data_label("", &"MutedLabel", false)
	_level.name = "Level"
	_level.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	info.add_child(_level)
	_ready_tag = UiBuild.label("", &"TileTagPending", false)
	_ready_tag.name = "Ready"
	_ready_tag.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	info.add_child(_ready_tag)
	_menu = UiBuild.button("", "menu", &"IconButton")
	_menu.name = "Menu"
	_menu.tooltip_text = "ui.party.member_menu"
	_menu.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_menu.pressed.connect(_on_menu_pressed)
	_menu.visible = false
	row.add_child(_menu)
