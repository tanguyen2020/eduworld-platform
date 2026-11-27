class_name ReadyCheckDialog
extends Control
## "Đã tìm thấy trận!" (F109-AC-03; STUDENT_GAME_UX_SOCIAL §8 `ReadyCheckDialog`, MATCHMAKING §6): the players of the
## proposal (portrait + nickname only, R16), a bar that empties until the server `deadline_at` (server clock), the
## seconds in words for lớp 4–9 (lớp 1–3: the bar and "Bấm Vào trận nhé!", no number), "Vào trận" and "Để lần sau".
## After "Vào trận": "Đang chờ các bạn khác… (1/2)". A cancelled proposal with `requeued` goes back to the queue bar.
## UI only: MatchmakingService sends the answers (WS, REST fallback).

var service: MatchmakingService = null
var _window: PanelContainer = null
var _players: VBoxContainer = null
var _bar: ProgressBar = null
var _status: Label = null
var _accept: Button = null
var _decline: Button = null
var _total_ms: int = 0


func _init() -> void:
	name = "ReadyCheckDialog"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(center)
	_window = UiBuild.panel(&"FormCard")
	_window.name = "Window"
	center.add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_window.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"ConfirmTitleBar")
	box.add_child(bar)
	var title_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(title_row)
	title_row.add_child(UiBuild.icon_rect("arena"))
	var title: Label = UiBuild.label("ui.arena.ready.title", &"ConfirmTitleLabel")
	title.name = "Title"
	UiBuild.expand(title)
	title_row.add_child(title)
	_players = UiBuild.vbox(&"FieldBox")
	_players.name = "Players"
	box.add_child(_players)
	_bar = UiBuild.progress_bar(1.0)
	_bar.name = "TimeBar"
	UiBuild.expand(_bar)
	box.add_child(_bar)
	_status = UiBuild.data_label("", &"InstructionLabel")
	_status.name = "Status"
	_status.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(_status)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.alignment = FlowContainer.ALIGNMENT_CENTER
	box.add_child(buttons)
	_decline = UiBuild.button("ui.arena.ready.decline", "cancel", &"SecondaryButton")
	_decline.name = "Decline"
	_decline.pressed.connect(_on_decline)
	buttons.add_child(_decline)
	_accept = UiBuild.button("ui.arena.ready.accept", "play_light")
	_accept.name = "Accept"
	_accept.pressed.connect(_on_accept)
	buttons.add_child(_accept)


func bind(next: MatchmakingService) -> void:
	if service != null:
		for pair: Array in [[service.proposal_changed, _on_proposal], [service.proposal_closed, _on_closed],
				[service.match_assigned, _on_assigned]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	service = next
	if service == null:
		hide()
		return
	service.proposal_changed.connect(_on_proposal)
	service.proposal_closed.connect(_on_closed)
	service.match_assigned.connect(_on_assigned)


## Advances the bar on the server clock (called every frame by the HUD; tests call it after moving the clock).
func advance(_delta: float) -> void:
	if visible:
		_render_time()


func status_text() -> String:
	return _status.text if visible else ""


func accept_button() -> Button:
	return _accept


func decline_button() -> Button:
	return _decline


func bar_value() -> float:
	return _bar.value


func _on_proposal(p: MatchProposalModel) -> void:
	if p == null:
		return
	if not visible:
		_total_ms = maxi(1000, p.deadline_at - service.now_ms())
		UiBuild.clear(_players)
		for pl: PublicPlayerModel in p.participants:
			if pl.user_id != CompetitionPlayer.self_id(AppState):
				_players.add_child(ArenaUi.player_row(pl.nickname, pl.avatar_config))
		if _players.get_child_count() == 0:
			_players.add_child(UiBuild.label("ui.arena.ready.opponent_hidden", &"MutedLabel"))
		# A comfortable card: names never wrap letter by letter in a narrow window.
		_window.custom_minimum_size.x = UiBuild.target() * 8.0
		show()
		UiBuild.size_buttons(self)
		_accept.grab_focus.call_deferred()
	_accept.disabled = p.accepted
	_decline.visible = not p.accepted
	_render_time()


func _render_time() -> void:
	var p: MatchProposalModel = service.proposal if service != null else null
	if p == null:
		return
	var left_ms: int = maxi(0, p.deadline_at - service.now_ms())
	_bar.value = clampf(float(left_ms) / float(maxi(1, _total_ms)), 0.0, 1.0)
	if p.accepted:
		_status.text = TranslationServer.translate("ui.arena.ready.waiting").format({"accepted": maxi(1, p.accepted_count),
			"total": maxi(p.total, 1)})
	elif ArenaUi.band() == "g1_3":
		_status.text = TranslationServer.translate("ui.arena.ready.press")
	else:
		_status.text = TranslationServer.translate("ui.arena.ready.seconds").format({"seconds": p.seconds_left(service.now_ms())})


func _on_closed(_requeued: bool) -> void:
	hide()


func _on_assigned(_match_id: String, _mode: String) -> void:
	hide()


func _on_accept() -> void:
	_accept.disabled = true
	if not await service.accept_proposal():
		_accept.disabled = false


func _on_decline() -> void:
	_decline.disabled = true
	await service.decline_proposal()
	_decline.disabled = false
	hide()
