class_name DuelInviteDialog
extends Control
## "Thách đấu bạn" (F109-AC-27; MATCHMAKING §8): two faces of one dialog (FormCard over the nested scrim).
## - send: the student's friends (`listFriends`, nickname + portrait, the server order) and the subjects, then
##   "Gửi lời thách đấu" (`createDuel`, unranked, with an Idempotency-Key) → "Đã gửi! Bạn có 60 giây để trả lời".
## - receive: "{nickname} thách đấu em môn Toán" with "Nhận lời" (`acceptDuel` → the match opens) / "Để lần sau".
## `410 DUEL_EXPIRED` → "Lời thách đấu đã hết hạn"; `SOCIAL_SCOPE_VIOLATION` → one general sentence without the reason.
## UI only: MatchmakingService / PartyService (friends) do the work.

signal closed()

var matchmaking: MatchmakingService = null
var party: PartyService = null
var duel: DuelModel = null
var friend_id: String = ""
var subject_id: String = ""
var _title: Label = null
var _body: VBoxContainer = null
var _status: Label = null
var _send: Button = null
var _accept: Button = null
var _decline: Button = null
var _close: Button = null
var _friend_group: ButtonGroup = ButtonGroup.new()
var _subject_group: ButtonGroup = ButtonGroup.new()
var _friends_row: HBoxContainer = null
var _subjects_row: HBoxContainer = null


func _init() -> void:
	name = "DuelInviteDialog"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(center)
	var win: PanelContainer = UiBuild.panel(&"FormCard")
	win.name = "Window"
	win.custom_minimum_size.x = UiBuild.target() * 9.0
	center.add_child(win)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	win.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"WindowTitleBar")
	box.add_child(bar)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(head)
	head.add_child(UiBuild.icon_rect("arena"))
	_title = UiBuild.label("ui.arena.duel.title", &"WindowTitleLabel")
	_title.name = "Title"
	UiBuild.expand(_title)
	head.add_child(_title)
	_body = UiBuild.vbox(&"FieldBox")
	_body.name = "Body"
	box.add_child(_body)
	_status = UiBuild.data_label("", &"InstructionLabel")
	_status.name = "Status"
	_status.visible = false
	box.add_child(_status)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.alignment = FlowContainer.ALIGNMENT_CENTER
	box.add_child(buttons)
	_close = UiBuild.button("ui.arena.duel.later", "cancel", &"SecondaryButton")
	_close.name = "Later"
	_close.pressed.connect(_on_later)
	buttons.add_child(_close)
	_decline = UiBuild.button("ui.arena.duel.decline", "cancel", &"SecondaryButton")
	_decline.name = "Decline"
	_decline.pressed.connect(_on_decline)
	buttons.add_child(_decline)
	_send = UiBuild.button("ui.arena.duel.send", "send")
	_send.name = "Send"
	_send.pressed.connect(_on_send)
	buttons.add_child(_send)
	_accept = UiBuild.button("ui.arena.duel.accept", "play_light")
	_accept.name = "Accept"
	_accept.pressed.connect(_on_accept)
	buttons.add_child(_accept)


## The send face: friends + subjects (`subject_ids` of the 1v1 mode the server opened).
func open_send(mm: MatchmakingService, party_service: PartyService, subject_ids: PackedStringArray) -> void:
	matchmaking = mm
	party = party_service
	duel = null
	_title.text = "ui.arena.duel.title"
	UiBuild.clear(_body)
	_status.visible = false
	_body.add_child(UiBuild.label("ui.arena.duel.pick_friend", &"HeadingLabel"))
	_friends_row = UiBuild.hbox(&"ToolbarBox")
	_friends_row.name = "Friends"
	_body.add_child(UiBuild.chip_scroller(_friends_row))
	_body.add_child(UiBuild.label("ui.arena.subject.title", &"HeadingLabel"))
	_subjects_row = UiBuild.hbox(&"ToolbarBox")
	_subjects_row.name = "Subjects"
	_body.add_child(UiBuild.chip_scroller(_subjects_row))
	subject_id = subject_ids[0] if not subject_ids.is_empty() else ""
	for id: String in subject_ids:
		var chip: Button = UiBuild.chip("", _subject_group)
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.text = mm.subject_name(id)
		chip.set_pressed_no_signal(id == subject_id)
		chip.pressed.connect(func() -> void:
			subject_id = id
			UiBuild.mark_chips(_subjects_row.get_children()))
		_subjects_row.add_child(chip)
	UiBuild.mark_chips(_subjects_row.get_children())
	_set_mode(true)
	show()
	if party != null and not party.friends_loaded:
		await party.load_friends()
	_render_friends()


## The receive face for `incoming` (nickname filled by the caller).
func open_receive(mm: MatchmakingService, incoming: DuelModel) -> void:
	matchmaking = mm
	duel = incoming
	UiBuild.clear(_body)
	_title.text = "ui.arena.duel.incoming_title"
	_body.add_child(ArenaUi.coach_line("ui.arena.duel.incoming", {"nickname": incoming.other_nickname,
		"subject": mm.subject_name(incoming.subject_id)}, CloudyMascot.Mood.HAPPY))
	_status.visible = false
	_set_mode(false)
	show()


func close() -> void:
	hide()
	closed.emit()


func status_text() -> String:
	return _status.text if _status.visible else ""


func send_button() -> Button:
	return _send


func accept_button() -> Button:
	return _accept


func friend_chips() -> Array[Node]:
	return _friends_row.get_children() if _friends_row != null else []


func _set_mode(sending: bool) -> void:
	_send.visible = sending
	_close.visible = sending
	_accept.visible = not sending
	_decline.visible = not sending
	_send.disabled = friend_id.is_empty() or subject_id.is_empty()
	UiBuild.size_buttons(self)


func _render_friends() -> void:
	UiBuild.clear(_friends_row)
	if party == null or party.friends.is_empty():
		_friends_row.add_child(UiBuild.label("ui.arena.duel.no_friends", &"MutedLabel"))
		return
	for f: PartyMemberModel in party.friends:
		var chip: Button = UiBuild.chip("", _friend_group)
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.name = "Friend_%s" % f.student_id.right(4)
		chip.text = f.nickname
		chip.pressed.connect(func() -> void:
			friend_id = f.student_id
			UiBuild.mark_chips(_friends_row.get_children())
			_send.disabled = subject_id.is_empty())
		_friends_row.add_child(chip)
	UiBuild.size_buttons(self)


func _say(key: String, args: Dictionary = {}) -> void:
	_status.text = TranslationServer.translate(key).format(args)
	_status.visible = true


func _on_send() -> void:
	_send.disabled = true
	var result: ApiResult = await matchmaking.challenge_friend(friend_id, subject_id)
	if result.ok:
		var sent: DuelModel = DuelModel.from_dict(result.body())
		_say("ui.arena.duel.sent", {"seconds": sent.seconds_left(matchmaking.now_ms()) if sent != null else 60})
		_send.visible = false
		return
	_send.disabled = false
	_say(CompetitionText.error_key(result.code))


func _on_accept() -> void:
	_accept.disabled = true
	var result: ApiResult = await matchmaking.accept_duel(duel)
	_accept.disabled = false
	if result.ok:
		close()
		return
	_say(CompetitionText.error_key(result.code))
	_accept.visible = false


func _on_decline() -> void:
	if duel != null:
		await matchmaking.decline_duel(duel)
	close()


func _on_later() -> void:
	close()
