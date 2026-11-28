class_name ClubInvitePicker
extends Control
## "Mời bạn vào club" for an owner / officer (F111-AC-26; SOCIAL_SYSTEM §6.2, FR-SOC-003, AGENTS R16): the student picks
## ONLY from their friends list (portrait + nickname, a tick chip — there is no field to type a name), at most 50 at a
## time; after `inviteToClub` each picked friend reads "Đã mời" or, when the server did not invite them (another school,
## a block…), the same general "Chưa mời được" — never the reason. UI only: `send_requested` (AGENTS R1).

signal send_requested(club_id: String, ids: PackedStringArray)
signal closed()

## Contract `invitee_ids` maxItems.
const MAX_PICK: int = 50

var club_id: String = ""
var _ui: Dictionary = {}
var _list: VBoxContainer = null
var _empty: Control = null
var _picked_label: Label = null
var _send: Button = null
var _picked: PackedStringArray = []
var _chips: Dictionary = {}
var _results: Dictionary = {}
var _status_labels: Dictionary = {}
var _busy: bool = false


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()


func open_for(id: String, friends: Array[FriendModel]) -> void:
	club_id = id
	_picked.clear()
	_results.clear()
	UiBuild.clear(_list)
	_chips.clear()
	_status_labels.clear()
	for f: FriendModel in friends:
		var ui: Dictionary = FriendsUi.person_card(f.nickname, f.avatar_config, "Friend")
		var actions: HFlowContainer = ui["actions"] as HFlowContainer
		var status: Label = UiBuild.label("", &"MutedLabel", false)
		status.name = "Result"
		status.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		status.visible = false
		actions.add_child(status)
		_status_labels[f.user_id] = status
		var chip: Button = UiBuild.chip("ui.club.invite", null)
		chip.name = "Pick"
		var uid: String = f.user_id
		chip.toggled.connect(func(_on: bool) -> void: toggle(uid))
		actions.add_child(chip)
		_chips[f.user_id] = chip
		_list.add_child(ui["card"] as Control)
	_empty.visible = friends.is_empty()
	show()
	_refresh()


func close() -> void:
	hide()
	closed.emit()


## Adds / removes friend `user_id` from the picked ones (never past MAX_PICK).
func toggle(user_id: String) -> void:
	if user_id in _picked:
		_picked.remove_at(_picked.find(user_id))
	elif _picked.size() < MAX_PICK and _chips.has(user_id):
		_picked.append(user_id)
	_refresh()


## The server answer: `{ok_ids, failed_ids}` → "Đã mời" / "Chưa mời được".
func show_result(result: Dictionary) -> void:
	for id: String in result.get("ok_ids", PackedStringArray()) as PackedStringArray:
		_results[id] = "ui.club.invite_done"
	for id: String in result.get("failed_ids", PackedStringArray()) as PackedStringArray:
		_results[id] = "ui.club.invite_failed"
	_picked.clear()
	_refresh()


func set_busy(busy: bool) -> void:
	_busy = busy
	_refresh()


func candidate_ids() -> PackedStringArray:
	return PackedStringArray(_chips.keys())


func send_button() -> Button:
	return _send


func result_text(user_id: String) -> String:
	return tr(String(_results.get(user_id, "")))


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).visible:
			out.append(tr((node as Label).text))
	return out


func _refresh() -> void:
	for id: String in _chips.keys():
		var chip: Button = _chips[id] as Button
		chip.set_pressed_no_signal(id in _picked)
		chip.icon = UiBuild.icon("checkbox_on" if id in _picked else "checkbox_off")
		chip.disabled = _busy or (not id in _picked and _picked.size() >= MAX_PICK)
		var status: Label = _status_labels[id] as Label
		status.text = String(_results.get(id, ""))
		status.visible = _results.has(id)
		chip.visible = not _results.has(id) or String(_results[id]) == "ui.club.invite_failed"
	_picked_label.text = tr("ui.club.invite_picked").format({"count": LocaleFormat.number(_picked.size()),
		"max": LocaleFormat.number(MAX_PICK)})
	_send.disabled = _busy or _picked.is_empty()


func _on_send() -> void:
	if not _picked.is_empty() and not _busy:
		send_requested.emit(club_id, _picked.duplicate())


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _build() -> void:
	_ui = UiBuild.window(self, "ui.club.invite_title", "invite")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box)
	scroll.add_child(box)
	_list = UiBuild.vbox(&"FormBox")
	_list.name = "Friends"
	box.add_child(_list)
	_empty = FriendsUi.empty_line("ui.club.invite_none")
	_empty.visible = false
	box.add_child(_empty)
	var foot: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	foot.name = "Footer"
	body.add_child(foot)
	_picked_label = UiBuild.data_label("", &"MutedLabel", false)
	_picked_label.name = "Picked"
	_picked_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_picked_label)
	foot.add_child(_picked_label)
	_send = UiBuild.button("ui.club.invite_send", "send")
	_send.name = "Send"
	_send.pressed.connect(_on_send)
	foot.add_child(_send)
	UiBuild.size_buttons(self)
