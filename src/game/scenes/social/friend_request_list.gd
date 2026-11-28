class_name FriendRequestList
extends VBoxContainer
## The "Lời mời" tab (F110-AC-21; STUDENT_GAME_UX_SOCIAL §6 `RequestList`): requests to this student with "Đồng ý" /
## "Từ chối", and the requests this student sent with the clock + "Đang chờ" — ALWAYS that, never "bị từ chối", until
## the request disappears (silent refusal, SC-005). Nickname + portrait only (R16). UI only: the screen calls
## SocialService on `accept_requested` / `reject_requested`.

signal accept_requested(request: FriendRequestModel)
signal reject_requested(request: FriendRequestModel)

var _incoming: VBoxContainer = null
var _outgoing: VBoxContainer = null
var _rows: Dictionary = {}


func _init() -> void:
	name = "FriendRequestList"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.friends.requests.incoming", "invite"))
	_incoming = UiBuild.vbox(&"FormBox")
	_incoming.name = "Incoming"
	add_child(_incoming)
	add_child(FriendsUi.group_title("ui.friends.requests.outgoing", "clock"))
	_outgoing = UiBuild.vbox(&"FormBox")
	_outgoing.name = "Outgoing"
	add_child(_outgoing)


func set_requests(incoming: Array[FriendRequestModel], outgoing: Array[FriendRequestModel]) -> void:
	UiBuild.clear(_incoming)
	UiBuild.clear(_outgoing)
	_rows.clear()
	for r: FriendRequestModel in incoming:
		var ui: Dictionary = FriendsUi.person_card(r.other.nickname, r.other.avatar_config, "Request")
		var actions: HFlowContainer = ui["actions"] as HFlowContainer
		var yes: Button = UiBuild.button("ui.friends.accept", "check")
		yes.name = "Accept"
		yes.pressed.connect(func() -> void: accept_requested.emit(r))
		actions.add_child(yes)
		var no: Button = UiBuild.button("ui.friends.reject", "cancel", &"SecondaryButton")
		no.name = "Reject"
		no.pressed.connect(func() -> void: reject_requested.emit(r))
		actions.add_child(no)
		_incoming.add_child(ui["card"] as Control)
		_rows[r.request_id] = ui["card"]
	if incoming.is_empty():
		_incoming.add_child(UiBuild.label("ui.friends.requests.none_incoming", &"MutedLabel"))
	for r: FriendRequestModel in outgoing:
		var ui: Dictionary = FriendsUi.person_card(r.other.nickname, r.other.avatar_config, "Sent")
		(ui["actions"] as HFlowContainer).add_child(FriendsUi.status(r.status_key(), "clock"))
		_outgoing.add_child(ui["card"] as Control)
		_rows[r.request_id] = ui["card"]
	if outgoing.is_empty():
		_outgoing.add_child(UiBuild.label("ui.friends.requests.none_outgoing", &"MutedLabel"))
	UiBuild.size_buttons(self)


func row(request_id: String) -> Control:
	return _rows.get(request_id) as Control


## Status words of the sent requests (tests: every one "Đang chờ").
func outgoing_status_keys() -> PackedStringArray:
	var out: PackedStringArray = []
	for card: Node in _outgoing.get_children():
		var l: Label = card.find_child("StatusLabel", true, false) as Label
		if l != null:
			out.append(l.text)
	return out
