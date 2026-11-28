class_name BlockList
extends VBoxContainer
## The "Đã chặn" tab (F110-AC-25; STUDENT_GAME_UX_SOCIAL §6 `BlockList`): the players this student blocked ("Bỏ chặn")
## and muted ("Bỏ tắt tiếng") from `listBlocks` / `listMutes` — nickname + portrait only. Nothing anywhere tells who
## blocked this student (there is no such list or message). UI only: the screen calls SocialService.

signal unblock_requested(user_id: String)
signal unmute_requested(user_id: String)

var _blocks: VBoxContainer = null
var _mutes: VBoxContainer = null


func _init() -> void:
	name = "BlockList"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.friends.blocked.blocks", "block"))
	_blocks = UiBuild.vbox(&"FormBox")
	_blocks.name = "Blocks"
	add_child(_blocks)
	add_child(FriendsUi.group_title("ui.friends.blocked.mutes", "sound_off"))
	_mutes = UiBuild.vbox(&"FormBox")
	_mutes.name = "Mutes"
	add_child(_mutes)


func set_relations(blocks: Array[RelationModel], mutes: Array[RelationModel]) -> void:
	_fill(_blocks, blocks, "ui.friends.unblock", "ui.friends.blocked.none_blocks", unblock_requested)
	_fill(_mutes, mutes, "ui.friends.unmute", "ui.friends.blocked.none_mutes", unmute_requested)
	UiBuild.size_buttons(self)


func button(kind: String, user_id: String) -> Button:
	var box: VBoxContainer = _mutes if kind == RelationModel.KIND_MUTE else _blocks
	for card: Node in box.get_children():
		if card.get_meta("user_id", "") == user_id:
			return card.find_child("Undo", true, false) as Button
	return null


func _fill(box: VBoxContainer, list: Array[RelationModel], undo_key: String, none_key: String, sig: Signal) -> void:
	UiBuild.clear(box)
	for r: RelationModel in list:
		var ui: Dictionary = FriendsUi.person_card(r.nickname, r.avatar_config, "Relation")
		var card: Control = ui["card"] as Control
		card.set_meta("user_id", r.user_id)
		var undo: Button = UiBuild.button(undo_key, "rotate_left", &"SecondaryButton")
		undo.name = "Undo"
		var id: String = r.user_id
		undo.pressed.connect(func() -> void:
			undo.disabled = true
			sig.emit(id))
		(ui["actions"] as HFlowContainer).add_child(undo)
		box.add_child(card)
	if list.is_empty():
		box.add_child(UiBuild.label(none_key, &"MutedLabel"))
