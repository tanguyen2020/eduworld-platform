class_name FriendsUi
extends RefCounted
## Small builders shared by the tabs of the friends screen (F110 US4): a person card (2D portrait + nickname — nothing
## else of another player, AGENTS R16) with a flow of action buttons / status words at its right end that wraps on a
## narrow phone column, a group title with its icon, and an empty-state line with Mây. Styled only by the generated Theme (R32).


## `{card, actions}`: a `PartyMemberCard` row with the portrait and the nickname; buttons go into `actions`.
static func person_card(nickname: String, avatar_config: Dictionary, card_name: String) -> Dictionary:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = card_name
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var thumb: AvatarThumb = AvatarThumb.new()
	thumb.standalone = true
	thumb.custom_minimum_size = Vector2.ONE * UiBuild.target()
	thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var look: AvatarConfigModel = AvatarConfigModel.from_dict(avatar_config) if not avatar_config.is_empty() else null
	thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	row.add_child(thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	row.add_child(texts)
	var nick: Label = UiBuild.keep_words(UiBuild.data_label(nickname, &"HeadingLabel"))
	nick.name = "Nickname"
	texts.add_child(nick)
	# F110 compact polish: the buttons / status sit at the right end of the row (like "Xem" of a friend row), so a card
	# is one line and the next person shows on a short phone screen; on a narrow column the flow still wraps them.
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	actions.alignment = FlowContainer.ALIGNMENT_END
	actions.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(actions)
	row.add_child(actions)
	return {"card": card, "actions": actions}


## A status word with its icon (e.g. the clock + "Đang chờ" of a request this student sent).
static func status(key: String, icon_name: String) -> HBoxContainer:
	var h: HBoxContainer = UiBuild.hbox()
	h.name = "Status"
	var icon: TextureRect = UiBuild.icon_rect(icon_name, 0.8)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	h.add_child(icon)
	var l: Label = UiBuild.label(key, &"MutedLabel", false)
	l.name = "StatusLabel"
	l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	h.add_child(l)
	return h


static func group_title(key: String, icon_name: String) -> HBoxContainer:
	var h: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	var icon: TextureRect = UiBuild.icon_rect(icon_name, 0.9)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	h.add_child(icon)
	var l: Label = UiBuild.keep_words(UiBuild.label(key, &"HeadingLabel"))
	l.name = "GroupTitle"
	UiBuild.expand(l)
	h.add_child(l)
	return h


## Mây with a short line (empty lists): happy, never sad (NFR-SAFE-003).
static func empty_line(key: String, line_name: String = "Empty") -> Control:
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	var row: Control = coach["row"] as Control
	row.name = line_name
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.keep_words(UiBuild.label(key, &"SpeechBubbleLabel")))
	(row.get_node("Bubble") as Control).size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return row
