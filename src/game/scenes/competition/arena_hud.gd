class_name ArenaHud
extends Control
## The arena layer over the world HUD (F109-AC-02, AC-03, AC-26): the floating QueueStatusBar (top centre, the student
## keeps walking), the ReadyCheckDialog when the server proposes a match, the tournament toast ("Trận giải của em
## bắt đầu sau 10 phút" — "Xem" opens the tournament tab) and the DuelIncomingPrompt when a friend challenges the
## student (`competition.duel.received`, top centre under the HUD top bar). Lets every click through except on its own controls. UI only:
## MatchmakingService / TournamentService do the work.

signal arena_requested(args: Dictionary)

## Seconds a tournament toast stays (≥ 3 s reading time, STUDENT_GAME_UX §4).
const TOAST_SEC: float = 6.0

var matchmaking: MatchmakingService = null
var tournaments: TournamentService = null
var toast_tournament: String = ""
var _bar: QueueStatusBar = null
var _ready_check: ReadyCheckDialog = null
var _duel_prompt: DuelIncomingPrompt = null
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
	_fit_bar(AppState.compact)
	AppState.compact_changed.connect(_fit_bar)
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
	_duel_prompt = DuelIncomingPrompt.new()
	_duel_prompt.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_duel_prompt.anchor_left = 0.5
	_duel_prompt.anchor_right = 0.5
	_duel_prompt.anchor_top = 0.14
	_duel_prompt.anchor_bottom = 0.14
	_duel_prompt.grow_horizontal = Control.GROW_DIRECTION_BOTH
	_duel_prompt.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_660)
	add_child(_duel_prompt)


## F125 polish: on the compact canvas the HUD's left column (next step card, joystick) and its bottom-right "Về Quảng
## trường" take a larger share of the width — the bar keeps to the band between them so it never covers either.
func _fit_bar(compact: bool) -> void:
	_bar.anchor_left = 0.3 if compact else 0.2
	_bar.anchor_right = 0.75 if compact else 0.8
	_bar.set_stacked(compact)


func bind(mm: MatchmakingService, tours: TournamentService) -> void:
	if matchmaking != null and matchmaking.duel_received.is_connected(show_duel):
		matchmaking.duel_received.disconnect(show_duel)
	matchmaking = mm
	if matchmaking != null:
		matchmaking.duel_received.connect(show_duel)
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


## A friend's duel (`competition.duel.received`): the prompt with "Nhận lời" / "Để lần sau".
func show_duel(duel: DuelModel) -> void:
	if matchmaking != null and duel != null:
		_duel_prompt.present(matchmaking, duel)


func advance(delta: float) -> void:
	_bar.advance(delta)
	_ready_check.advance(delta)
	_duel_prompt.advance(delta)
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


func duel_prompt() -> DuelIncomingPrompt:
	return _duel_prompt


func toast_text() -> String:
	return _toast_label.text if _toast.visible else ""


func _on_toast_open() -> void:
	_toast.visible = false
	arena_requested.emit({"tab": ArenaScreen.TAB_TOURNAMENTS, "tournament_id": toast_tournament})
