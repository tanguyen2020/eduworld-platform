class_name ChatPlayerSafety
extends VBoxContainer
## The other players of a match or a party, each with "Báo cáo người chơi" and "Chặn" (F119-AC-14, AC-UX-110;
## CHILD_SAFETY §3.2b — the safety replacement for a chat that cannot be turned off). Shown from the chat panel
## ("Người chơi") and embedded by the match result screen (F109) and the party history while the messages are kept.
## A portrait, the nickname only (AGENTS R16) and the two buttons with an icon and words. UI only: the owner opens the
## report sheet / blocks through ChatDialogs.

signal report_requested(user_id: String, nickname: String)
signal block_requested(user_id: String, nickname: String)

var _rows: Dictionary = {}


func _init() -> void:
	name = "ChatPlayerSafety"
	theme_type_variation = &"FieldBox"


## `{user_id, nickname, avatar_config?}` of the other players (never the student).
func set_players(players: Array[Dictionary]) -> void:
	UiBuild.clear(self)
	_rows.clear()
	if players.is_empty():
		add_child(UiBuild.label("ui.chat.players.none", &"MutedLabel"))
		return
	for p: Dictionary in players:
		var id: String = String(p.get("user_id", ""))
		var nick: String = String(p.get("nickname", ""))
		if not Uuid.is_valid(id) or nick.is_empty():
			continue
		var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
		card.name = "Player"
		add_child(card)
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		card.add_child(row)
		var thumb: AvatarThumb = AvatarThumb.new()
		thumb.custom_minimum_size = Vector2.ONE * UiBuild.target()
		thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		if p.get("avatar_config") is Dictionary:
			thumb.set_look(AvatarConfigModel.from_dict(p["avatar_config"]))
		row.add_child(thumb)
		var name_label: Label = UiBuild.data_label(nick, &"HeadingLabel")
		name_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		UiBuild.expand(name_label)
		row.add_child(name_label)
		var buttons: HFlowContainer = HFlowContainer.new()
		buttons.alignment = FlowContainer.ALIGNMENT_END
		UiBuild.expand(buttons)
		row.add_child(buttons)
		var report: Button = UiBuild.button("ui.chat.players.report", "flag", &"SecondaryButton")
		report.name = "Report"
		report.pressed.connect(func() -> void: report_requested.emit(id, nick))
		buttons.add_child(report)
		var block: Button = UiBuild.button("ui.chat.menu.block", "block", &"SecondaryButton")
		block.name = "Block"
		block.pressed.connect(func() -> void: block_requested.emit(id, nick))
		buttons.add_child(block)
		_rows[id] = card
	UiBuild.size_buttons(self)


func player_count() -> int:
	return _rows.size()


func report_button(user_id: String) -> Button:
	var card: Control = _rows.get(user_id) as Control
	return card.find_child("Report", true, false) as Button if card != null else null


func block_button(user_id: String) -> Button:
	var card: Control = _rows.get(user_id) as Control
	return card.find_child("Block", true, false) as Button if card != null else null
