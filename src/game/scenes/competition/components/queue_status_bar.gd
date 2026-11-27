class_name QueueStatusBar
extends PanelContainer
## The floating queue bar of the HUD (F109-AC-02, AC-04; STUDENT_GAME_UX_SOCIAL §8): "Đang tìm bạn đấu… 0:24" with
## the mode, "Hủy" — the student keeps walking in the world. The wait counts up from the server `wait_ms` (no
## countdown pressure). The server's `search_widened` / `bot_offer_available` add a calm line and "Luyện với Robot"
## (unranked, labelled). After a timeout: "Chưa tìm được bạn đấu" with "Chọn chế độ khác" and "Tìm lại".
## UI only: MatchmakingService does the work (R1).

signal arena_requested()

var service: MatchmakingService = null
var _status: Label = null
var _note: Label = null
var _cancel: Button = null
var _robot: Button = null
var _other_mode: Button = null
var _again: Button = null
var _base_wait_ms: int = 0
var _since_ms: int = 0
var _timed_out: bool = false


func _init() -> void:
	name = "QueueStatusBar"
	theme_type_variation = &"HudPanel"
	visible = false
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("search")
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	# The words keep room for one line of the wait (the buttons flow beside them).
	texts.custom_minimum_size.x = UiBuild.target() * 5.0
	UiBuild.expand(texts)
	row.add_child(texts)
	_status = UiBuild.data_label("", &"HudLabel")
	_status.name = "Status"
	texts.add_child(_status)
	_note = UiBuild.data_label("", &"HudLabel")
	_note.name = "Note"
	_note.visible = false
	texts.add_child(_note)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.alignment = FlowContainer.ALIGNMENT_END
	buttons.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(buttons)
	_robot = UiBuild.button("ui.arena.queue.robot", "robot", &"SecondaryButton")
	_robot.name = "Robot"
	_robot.pressed.connect(_on_robot)
	buttons.add_child(_robot)
	_other_mode = UiBuild.button("ui.arena.queue.other_mode", "arena", &"SecondaryButton")
	_other_mode.name = "OtherMode"
	_other_mode.pressed.connect(_on_other_mode)
	buttons.add_child(_other_mode)
	_again = UiBuild.button("ui.arena.queue.again", "search", &"SecondaryButton")
	_again.name = "Again"
	_again.pressed.connect(_on_again)
	buttons.add_child(_again)
	_cancel = UiBuild.button("ui.arena.queue.cancel", "cancel", &"SecondaryButton")
	_cancel.name = "Cancel"
	_cancel.pressed.connect(_on_cancel)
	buttons.add_child(_cancel)


func bind(next: MatchmakingService) -> void:
	if service != null:
		for pair: Array in [[service.ticket_changed, _on_ticket], [service.queue_ended, _on_queue_ended],
				[service.match_assigned, _on_match_assigned], [service.proposal_changed, _on_proposal]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	service = next
	if service == null:
		hide()
		return
	service.ticket_changed.connect(_on_ticket)
	service.queue_ended.connect(_on_queue_ended)
	service.match_assigned.connect(_on_match_assigned)
	service.proposal_changed.connect(_on_proposal)
	_on_ticket(service.ticket)


## Advances the wait clock (the HUD calls it every frame; tests call it directly).
func advance(_delta: float) -> void:
	if visible and not _timed_out and service != null and service.ticket != null:
		_render()


func status_text() -> String:
	return _status.text if visible else ""


func note_text() -> String:
	return _note.text if visible and _note.visible else ""


func robot_button() -> Button:
	return _robot


func cancel_button() -> Button:
	return _cancel


func other_mode_button() -> Button:
	return _other_mode


func again_button() -> Button:
	return _again


func _on_ticket(ticket: MatchmakingTicketModel) -> void:
	if ticket == null:
		if not _timed_out:
			hide()
		return
	_timed_out = false
	_base_wait_ms = ticket.wait_ms
	_since_ms = Time.get_ticks_msec()
	show()
	_render()


func _on_queue_ended(reason: String) -> void:
	_timed_out = reason == "timeout"
	visible = _timed_out
	if _timed_out:
		_render()


func _on_match_assigned(_match_id: String, _mode: String) -> void:
	_timed_out = false
	hide()


func _on_proposal(_p: MatchProposalModel) -> void:
	_render()


func _render() -> void:
	_robot.visible = false
	_other_mode.visible = _timed_out
	_again.visible = _timed_out and service != null and not service.last_search.is_empty()
	_cancel.visible = not _timed_out
	if _timed_out:
		_status.text = TranslationServer.translate("ui.arena.queue.timeout")
		_note.text = TranslationServer.translate("ui.arena.queue.timeout_hint")
		_note.visible = true
		return
	var t: MatchmakingTicketModel = service.ticket if service != null else null
	if t == null:
		return
	var waited: int = _base_wait_ms + (Time.get_ticks_msec() - _since_ms)
	_status.text = TranslationServer.translate("ui.arena.queue.searching").format({
		"time": CompetitionText.clock(waited), "mode": TranslationServer.translate(CompetitionText.mode_key(t.mode))})
	var note: String = ""
	if t.status == "proposed":
		note = "ui.arena.queue.found"
	elif t.bot_offer_available:
		note = "ui.arena.queue.robot_hint"
	elif t.search_widened:
		note = "ui.arena.queue.widened"
	elif t.requeued:
		note = "ui.arena.queue.requeued"
	_note.text = TranslationServer.translate(note) if not note.is_empty() else ""
	_note.visible = not note.is_empty()
	_robot.visible = t.bot_offer_available and t.status != "proposed"
	_cancel.disabled = service.is_busy()


func _on_cancel() -> void:
	_cancel.disabled = true
	await service.cancel()
	_cancel.disabled = false


func _on_robot() -> void:
	_robot.disabled = true
	await service.start_bot_match()
	_robot.disabled = false


func _on_other_mode() -> void:
	_timed_out = false
	hide()
	arena_requested.emit()


func _on_again() -> void:
	_timed_out = false
	hide()
	var s: Dictionary = service.last_search
	await service.find_match(String(s.get("mode", "")), String(s.get("subject_id", "")), String(s.get("topic_id", "")))
