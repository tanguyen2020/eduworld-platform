class_name ChatFriendsList
extends ScrollContainer
## The "Bạn bè" tab of the HUD chat (F112-AC-07, AC-10; STUDENT_GAME_UX_SOCIAL §7): the friends (`listFriends`, F110 —
## portrait, nickname, "Cấp n" only, AGENTS R16) with the unread count of their DM when the client knows which `direct`
## channel is theirs, and "Nhắn tin" (only for a current friend, when the school allows DMs and the server did not refuse
## this friend this session — ChatHubService.can_message). A DM channel whose friend is unknown is never named here: it
## only adds to the badge of the tab. No `openDirectChannel` for a friend the student did not pick. UI only (R1).

signal message_requested(user_id: String)

var hub_service: ChatHubService = null
var chat: ChatService = null
var social: SocialService = null
var _rows: VBoxContainer = null
var _buttons: Dictionary = {}
var _badges: Dictionary = {}


func _init() -> void:
	name = "FriendsList"
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	_rows = UiBuild.vbox(&"FormBox")
	_rows.name = "Rows"
	UiBuild.expand(_rows)
	add_child(_rows)


func bind(hub: ChatHubService, chat_service: ChatService, social_service: SocialService) -> void:
	hub_service = hub
	chat = chat_service
	social = social_service


func unbind() -> void:
	hub_service = null


func render() -> void:
	UiBuild.clear(_rows)
	_buttons.clear()
	_badges.clear()
	if social == null or hub_service == null:
		return
	var friends: Array[FriendModel] = social.store.friends
	if friends.is_empty():
		_rows.add_child(FriendsUi.empty_line("ui.chat.dm.no_friends"))
		return
	for f: FriendModel in friends:
		var ui: Dictionary = FriendsUi.person_card(f.nickname, f.avatar_config, "Friend")
		var texts: Node = (ui["card"] as Control).find_child("Nickname", true, false).get_parent()
		if f.level > 0:
			var level: Label = UiBuild.data_label(tr("ui.friends.level").format({"level": LocaleFormat.number(f.level)}),
					&"MutedLabel", false)
			level.name = "Level"
			texts.add_child(level)
		var actions: HFlowContainer = ui["actions"] as HFlowContainer
		var n: int = hub_service.state.friend_unread(f.user_id, chat.store if chat != null else null)
		var badge: Label = UiBuild.data_label(ChatHubState.badge_text(n), &"CountBadge", false)
		badge.name = "Unread"
		badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		badge.visible = n > 0
		actions.add_child(badge)
		_badges[f.user_id] = badge
		if hub_service.can_message(f.user_id):
			var b: Button = UiBuild.button("ui.chat.dm.message", "chat", &"SecondaryButton")
			b.name = "Message"
			var id: String = f.user_id
			b.pressed.connect(func() -> void: message_requested.emit(id))
			actions.add_child(b)
			_buttons[f.user_id] = b
		_rows.add_child(ui["card"] as Control)
	UiBuild.size_buttons(_rows)


func message_button(user_id: String) -> Button:
	return _buttons.get(user_id) as Button


func unread_text(user_id: String) -> String:
	var b: Label = _badges.get(user_id) as Label
	return b.text if b != null and b.visible else ""


func row_count() -> int:
	return _rows.get_child_count()


## Every text of the list (tests: nickname, level, counts — nothing else of a friend).
func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _rows.find_children("*", "Label", true, false):
		if (node as Label).visible:
			out.append(tr((node as Label).text))
	return out
