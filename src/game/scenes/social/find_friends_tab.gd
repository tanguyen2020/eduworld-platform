class_name FindFriendsTab
extends VBoxContainer
## The "Tìm bạn" tab (F110-AC-22, AC-23; STUDENT_GAME_UX_SOCIAL §6): the group "Ở gần đây" — the players of the same
## hub instance (F092 HubSync, nickname + portrait only) with "Kết bạn" — and the friend code (FriendCodeWidget). There
## is NO field to look a player up by name or id (FR-SOC-002); no "bạn cùng lớp" group (ADR-0095). Whether the friend
## request is allowed is the server's (`SOCIAL_SCOPE_VIOLATION` → "Em chỉ kết bạn được với bạn cùng trường"). UI only.
## F111 (US2): between them the group "Vừa chơi cùng" (SuggestionGroup, `listFriendSuggestions`) — three groups in all.

signal add_requested(user_id: String)

var code_widget: FriendCodeWidget = null
var _suggestions: SuggestionGroup = null
var _nearby: VBoxContainer = null
var _buttons: Dictionary = {}


func _init() -> void:
	name = "FindFriendsTab"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.friends.find.nearby", "area_social"))
	_nearby = UiBuild.vbox(&"FormBox")
	_nearby.name = "Nearby"
	add_child(_nearby)
	_suggestions = SuggestionGroup.new()
	_suggestions.add_requested.connect(func(id: String) -> void: add_requested.emit(id))
	add_child(_suggestions)
	code_widget = FriendCodeWidget.new()
	add_child(code_widget)


## `players`: `{user_id, nickname, avatar_config}` of the hub instance (never the student, never a friend already);
## `pending`: ids with a request this student sent ("Đang chờ").
func set_nearby(players: Array[Dictionary], pending: Dictionary) -> void:
	UiBuild.clear(_nearby)
	_buttons.clear()
	for p: Dictionary in players:
		var id: String = String(p.get("user_id", ""))
		var ui: Dictionary = FriendsUi.person_card(String(p.get("nickname", "")), p.get("avatar_config", {}) as Dictionary,
				"Player")
		var actions: HFlowContainer = ui["actions"] as HFlowContainer
		if pending.has(id):
			actions.add_child(FriendsUi.status("ui.friends.request_pending", "clock"))
		else:
			var add: Button = UiBuild.button("ui.friends.add", "invite_light")
			add.name = "Add"
			add.pressed.connect(func() -> void:
				add.disabled = true
				add_requested.emit(id))
			actions.add_child(add)
			_buttons[id] = add
		_nearby.add_child(ui["card"] as Control)
	if players.is_empty():
		_nearby.add_child(UiBuild.keep_words(UiBuild.label("ui.friends.find.nearby_none", &"MutedLabel")))
	UiBuild.size_buttons(self)


## F111: "Vừa chơi cùng" (see SuggestionGroup.set_suggestions).
func set_suggestions(list: Array[FriendSuggestionModel], pending: Dictionary, failed: bool, big: bool) -> void:
	_suggestions.set_suggestions(list, pending, failed, big)


func suggestion_group() -> SuggestionGroup:
	return _suggestions


## Title keys of the visible groups in order (tests: "Ở gần đây", "Vừa chơi cùng", the friend code — nothing else).
func group_keys() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("GroupTitle", "Label", true, false):
		if (node as Label).is_visible_in_tree() and not out.has((node as Label).text):
			out.append((node as Label).text)
	var code_keys: PackedStringArray = []
	for node: Node in code_widget.find_children("GroupTitle", "Label", true, false):
		code_keys.append((node as Label).text)
	var groups: PackedStringArray = []
	for k: String in out:
		if not k in code_keys:
			groups.append(k)
	if code_widget.is_visible_in_tree() and not code_keys.is_empty():
		groups.append(code_keys[0])
	return groups


func add_button(user_id: String) -> Button:
	return _buttons.get(user_id) as Button


func nearby_count() -> int:
	return _nearby.get_child_count()


## True when the tab has a way to type a name to look a player up (it must not — only the code field takes text).
func has_search_field() -> bool:
	for node: Node in find_children("*", "LineEdit", true, false):
		if node != code_widget.field():
			return true
	return not find_children("*", "TextEdit", true, false).is_empty()
