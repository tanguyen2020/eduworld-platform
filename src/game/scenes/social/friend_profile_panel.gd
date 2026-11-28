class_name FriendProfilePanel
extends PanelContainer
## The profile of one friend (F110-AC-24; STUDENT_GAME_UX_SOCIAL §6 `FriendProfilePanel`): portrait, nickname, "Cấp n",
## the level title when the server sent one (`PlayerCard.title`), the presence in words — never a real name, a school, a
## grade or a learning result (AGENTS R16, ADR-0095) — and the big action buttons: "Mời party", "Tắt tiếng", "Chặn",
## "Báo cáo" (hidden without the F119 report sheet), "Hủy kết bạn". Each safety action is one tap here + one confirmation
## (≤ 2 taps, STUDENT_GAME_UX §9). UI only: the friends screen acts on `action_requested` (AGENTS R1).
## F111-AC-02 (T015): "Tham gia bạn" first — grey with the reason in words under the buttons when the friend cannot be
## joined (`set_join_state`, JoinAvailability; never colour alone).

signal action_requested(action: StringName, user_id: String)

const ACT_JOIN: StringName = &"join"
const ACT_INVITE: StringName = &"invite"
const ACT_MUTE: StringName = &"mute"
const ACT_BLOCK: StringName = &"block"
const ACT_REPORT: StringName = &"report"
const ACT_REMOVE: StringName = &"remove"
## [action, label key, icon, variation].
const ACTIONS: Array = [
	[ACT_JOIN, "ui.friends.join_friend", "forward_light", &""],
	[ACT_INVITE, "ui.friends.invite_party", "party", &""],
	[ACT_MUTE, "ui.friends.mute", "sound_off", &"SecondaryButton"],
	[ACT_BLOCK, "ui.friends.block", "block", &"SecondaryButton"],
	[ACT_REPORT, "ui.friends.report", "flag", &"SecondaryButton"],
	[ACT_REMOVE, "ui.friends.remove", "cancel", &"SecondaryButton"],
]

var friend: FriendModel = null
var report_available: bool = true
var _thumb: AvatarThumb = null
var _name: Label = null
var _level: Label = null
var _title: Label = null
var _presence: Label = null
var _presence_icon: TextureRect = null
var _empty: Control = null
var _card: VBoxContainer = null
var _buttons: Dictionary = {}
var _join_reason: Label = null
var _join_state: StringName = &""
var _busy: bool = false


func _init() -> void:
	name = "FriendProfile"
	theme_type_variation = &"FormCard"
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	add_child(box)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	_empty = coach["row"] as Control
	_empty.name = "Empty"
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.keep_words(UiBuild.label("ui.friends.pick_friend",
			&"SpeechBubbleLabel")))
	box.add_child(_empty)
	_card = UiBuild.vbox(&"FormBox")
	_card.name = "Card"
	box.add_child(_card)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_card.add_child(head)
	_thumb = AvatarThumb.new()
	_thumb.standalone = true
	_thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	head.add_child(texts)
	# The nickname and "Cấp n" share a line (it wraps on a narrow column), like a row of the list.
	var top: HFlowContainer = HFlowContainer.new()
	top.name = "NameRow"
	texts.add_child(top)
	# One line (≤ 32 characters, never cut): in a flow a wrapping label would get no width and break per letter.
	_name = UiBuild.data_label("", &"HeadingLabel", false)
	_name.name = "Nickname"
	top.add_child(_name)
	_level = UiBuild.data_label("", &"MutedLabel", false)
	_level.name = "Level"
	_level.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	top.add_child(_level)
	_title = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_title.name = "Title"
	texts.add_child(_title)
	var presence: HBoxContainer = UiBuild.hbox()
	texts.add_child(presence)
	_presence_icon = UiBuild.icon_rect("presence_online", 0.8)
	_presence_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	presence.add_child(_presence_icon)
	_presence = UiBuild.keep_words(UiBuild.data_label("", &"InstructionLabel"))
	_presence.name = "Presence"
	UiBuild.expand(_presence)
	presence.add_child(_presence)
	_join_reason = UiBuild.keep_words(UiBuild.label("", &"MutedLabel"))
	_join_reason.name = "JoinReason"
	_join_reason.visible = false
	_card.add_child(_join_reason)
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	_card.add_child(actions)
	for spec: Array in ACTIONS:
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), spec[3] as StringName)
		b.name = String(spec[0]).capitalize()
		b.pressed.connect(_on_action.bind(spec[0] as StringName))
		actions.add_child(b)
		_buttons[spec[0]] = b
	show_friend(null, false)


func show_friend(f: FriendModel, presence_unknown: bool) -> void:
	friend = f
	_empty.visible = f == null
	_card.visible = f != null
	if f == null:
		return
	# Compact (F125 phone canvas): a smaller portrait so Tắt tiếng / Chặn / Báo cáo show without scrolling.
	var side: float = UiBuild.target() * (1.0 if AppState.compact else 1.6)
	_thumb.custom_minimum_size = Vector2(side, side)
	var look: AvatarConfigModel = f.avatar()
	_thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	_name.text = f.nickname
	_level.text = tr("ui.friends.level").format({"level": LocaleFormat.number(f.level)}) if f.level > 0 else ""
	_level.visible = f.level > 0
	_title.text = f.display_title()
	_title.visible = not _title.text.is_empty()
	_presence_icon.texture = UiBuild.icon("wifi_off" if presence_unknown else f.presence_icon())
	_presence.text = tr("ui.friends.presence.unknown") if presence_unknown else tr(f.presence_key())
	(_buttons[ACT_REPORT] as Button).visible = report_available
	UiBuild.size_buttons(self)
	for b: Button in _buttons.values():
		UiBuild.fit_wrapping(b, float(UiTokens.SIZE_LAYOUT_280))


func set_busy(busy: bool) -> void:
	_busy = busy
	for b: Button in _buttons.values():
		b.disabled = busy
	_apply_join()


## F111-AC-02: &"" = "Tham gia bạn" may be pressed; else the button is grey and the reason is said in words.
func set_join_state(reason: StringName) -> void:
	_join_state = reason
	_apply_join()


func join_reason_text() -> String:
	return tr(_join_reason.text) if _join_reason.visible else ""


func _apply_join() -> void:
	var b: Button = _buttons[ACT_JOIN] as Button
	b.disabled = _busy or _join_state != &""
	b.icon = UiBuild.icon("forward" if b.disabled else "forward_light")
	_join_reason.text = JoinAvailability.reason_key(_join_state) if _join_state != &"" else ""
	_join_reason.visible = _join_state != &""
	b.tooltip_text = _join_reason.text


func button(action: StringName) -> Button:
	return _buttons.get(action) as Button


## Texts of the card (tests: nickname / level / title / presence — nothing else about the friend).
func texts() -> PackedStringArray:
	return PackedStringArray([_name.text, _level.text, _title.text, _presence.text])


func _on_action(action: StringName) -> void:
	if friend != null:
		action_requested.emit(action, friend.user_id)
