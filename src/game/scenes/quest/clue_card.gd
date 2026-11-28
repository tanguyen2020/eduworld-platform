class_name ClueCard
extends PanelContainer
## One clue card of an `interdependent` step (F110-AC-15, AC-16; QUEST_SYSTEM §3.2): the role name and its icon, the
## clue the server rendered (display blocks of the learning material, no answer — ADR-0016) and, once shared, who put it
## on the board (nickname only). The card of the student's own role has "Chia sẻ dữ kiện" until it is shared. The client
## never writes, combines or guesses a clue; nothing is stored on disk. UI only.

signal share_requested(role: String)

var role: String = ""
var _title: Label = null
var _icon: TextureRect = null
var _body: Label = null
var _by: Label = null
var _share: Button = null


func _init() -> void:
	name = "ClueCard"
	theme_type_variation = &"QuestCard"
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	add_child(box)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(head)
	_icon = UiBuild.icon_rect("star")
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_icon)
	_title = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_title.name = "RoleTitle"
	UiBuild.expand(_title)
	head.add_child(_title)
	_body = UiBuild.keep_words(UiBuild.data_label("", &"InstructionLabel"))
	_body.name = "ClueText"
	box.add_child(_body)
	_by = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_by.name = "SharedBy"
	box.add_child(_by)
	_share = UiBuild.button("ui.quest.clue.share", "emoji_handshake")
	_share.name = "Share"
	_share.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_share.pressed.connect(func() -> void:
		_share.disabled = true
		share_requested.emit(role))
	box.add_child(_share)


## `mine`: the card of a role this student holds (shows "Chia sẻ dữ kiện" until shared).
func show_card(c: ClueCardModel, mine: bool) -> void:
	role = c.role
	_title.text = c.display_title()
	_icon.texture = icon_for(c.icon_key)
	_body.text = c.body_text()
	_body.visible = not _body.text.is_empty()
	_by.text = tr("ui.quest.clue.shared_by").format({"nickname": c.shared_by_nickname}) if c.shared and not c.shared_by_nickname.is_empty() \
			else (tr("ui.quest.clue.shared") if c.shared else tr("ui.quest.clue.only_you"))
	_share.visible = mine and not c.shared
	_share.disabled = false
	UiBuild.size_buttons(self)


func share_button() -> Button:
	return _share


func texts() -> PackedStringArray:
	return PackedStringArray([_title.text, _body.text, _by.text])


## The icon of a role (`icon_key` of the content); a key without an icon in this build shows the star.
static func icon_for(icon_key: String) -> Texture2D:
	if not icon_key.is_empty() and icon_key.is_valid_filename() and ResourceLoader.exists(UiBuild.ICONS % icon_key):
		return UiBuild.icon(icon_key)
	return UiBuild.icon("star")
