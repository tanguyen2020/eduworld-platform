class_name ArenaHud
extends Control
## The arena layer over the world HUD (F109-AC-02, AC-03, AC-26): the floating QueueStatusBar (top centre, the student
## keeps walking), the ReadyCheckDialog when the server proposes a match, and the tournament toast ("Trận giải của em
## bắt đầu sau 10 phút" — "Xem" opens the tournament tab). Lets every click through except on its own controls. UI only:
## MatchmakingService / TournamentService do the work.

signal arena_requested(args: Dictionary)

## Seconds a tournament toast stays (≥ 3 s reading time, STUDENT_GAME_UX §4).
const TOAST_SEC: float = 6.0

var matchmaking: MatchmakingService = null
var tournaments: TournamentService = null
var toast_tournament: String = ""
var _bar: QueueStatusBar = null
var _ready_check: ReadyCheckDialog = null
var _toast: PanelContainer = null
var _toast_label: Label = null
var _toast_left: float = 0.0


func _init() -> void:
	name = "ArenaHud"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_bar = QueueStatusBar.new()
	# Bottom centre, between the joystick and the action buttons (the top centre is the toast stack of the HUD).
	_bar.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	_bar.anchor_left = 0.2
	_bar.anchor_right = 0.8
	_bar.anchor_top = 0.97
	_bar.anchor_bottom = 0.97
	_bar.grow_horizontal = Control.GROW_DIRECTION_BOTH
	_bar.grow_vertical = Control.GROW_DIRECTION_BEGIN
	_bar.arena_requested.connect(func() -> void: arena_requested.emit({}))
	add_child(_bar)
	_toast = UiBuild.panel(&"HudPanel")
	_toast.name = "TournamentToast"
	_toast.visible = false
	_toast.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_toast.anchor_left = 0.25
	_toast.anchor_right = 0.75
	_toast.anchor_top = 0.26
	_toast.anchor_bottom = 0.26
	_toast.grow_horizontal = Control.GROW_DIRECTION_BOTH
	add_child(_toast)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_toast.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("bracket")
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	_toast_label = UiBuild.data_label("", &"HudLabel")
	_toast_label.name = "ToastLabel"
	_toast_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_toast_label)
	row.add_child(_toast_label)
	var open: Button = UiBuild.button("ui.tournament.toast.open", "next_light")
	open.name = "ToastOpen"
	open.pressed.connect(_on_toast_open)
	row.add_child(open)
	_ready_check = ReadyCheckDialog.new()
	add_child(_ready_check)


func bind(mm: MatchmakingService, tours: TournamentService) -> void:
	matchmaking = mm
	_bar.bind(mm)
	_ready_check.bind(mm)
	if tournaments != null and tournaments.toast_requested.is_connected(show_toast):
		tournaments.toast_requested.disconnect(show_toast)
	tournaments = tours
	if tournaments != null:
		tournaments.toast_requested.connect(show_toast)


func show_toast(key: String, args: Dictionary, tournament_id: String) -> void:
	toast_tournament = tournament_id
	_toast_label.text = TranslationServer.translate(key).format(args)
	_toast.visible = true
	_toast_left = TOAST_SEC
	UiBuild.size_buttons(_toast)


func advance(delta: float) -> void:
	_bar.advance(delta)
	_ready_check.advance(delta)
	if _toast_left > 0.0:
		_toast_left -= delta
		if _toast_left <= 0.0:
			_toast.visible = false


func _process(delta: float) -> void:
	advance(delta)


func queue_bar() -> QueueStatusBar:
	return _bar


func ready_check() -> ReadyCheckDialog:
	return _ready_check


func toast_text() -> String:
	return _toast_label.text if _toast.visible else ""


func _on_toast_open() -> void:
	_toast.visible = false
	arena_requested.emit({"tab": ArenaScreen.TAB_TOURNAMENTS, "tournament_id": toast_tournament})
