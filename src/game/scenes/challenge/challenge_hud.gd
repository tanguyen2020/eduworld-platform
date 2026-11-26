class_name ChallengeHud
extends Control
## S06 (b) — HUD of a solo challenge run (F069-AC-18, AC-19; STUDENT_GAME_UX_CORE S06 §10): the time left computed from
## the server's `deadline_at` on the server clock (ChallengeStore: lag ≤ 1 s, a changed device clock changes nothing) —
## `g1_3` a coloured bar WITHOUT seconds, older grades the bar + mm:ss —, stamina hearts, score and activities done,
## updated only from `challenge.attempt.updated` / `getGameChallengeAttempt` (ChallengeClient). "Tiếp tục" re-opens
## the activity, "Dừng lại" asks for confirmation then gives up (`abandonGameChallengeAttempt`). The clock label ticks
## on a Timer (4 Hz), never `_process`. Shown above the activity player while a run is open. UI only (AGENTS R1).

const SECONDS_BANDS: PackedStringArray = ["g4_5", "g6_7", "g8_9"]
const TICK_SEC: float = 0.25

var client: ChallengeClient = null
var band: String = ""
var _panel: PanelContainer = null
var _title: Label = null
var _bar: ProgressBar = null
var _clock: Label = null
var _lives: HBoxContainer = null
var _score: Label = null
var _continue: Button = null
var _stop: Button = null
var _confirm: PanelContainer = null
var _timer: Timer = null
var _total_ms: int = 0


func _ready() -> void:
	set_anchors_preset(Control.PRESET_TOP_WIDE)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	var column: VBoxContainer = UiBuild.vbox()
	column.set_anchors_preset(Control.PRESET_TOP_WIDE)
	column.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(column)
	_panel = UiBuild.panel(&"HudPanel")
	_panel.name = "Panel"
	column.add_child(_panel)
	var row: HBoxContainer = UiBuild.hbox()
	_panel.add_child(row)
	row.add_child(UiBuild.icon_rect("area_challenge_mountain"))
	_title = UiBuild.data_label("", &"HudLabel", false)
	_title.name = "Title"
	_title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_title.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_120)
	UiBuild.expand(_title)
	row.add_child(_title)
	row.add_child(UiBuild.icon_rect("clock", 0.8))
	_bar = ProgressBar.new()
	_bar.name = "TimeBar"
	_bar.max_value = 1.0
	_bar.step = 0.0
	_bar.show_percentage = false
	_bar.custom_minimum_size = Vector2(float(UiTokens.SIZE_LAYOUT_120), float(UiTokens.COMPONENT_PROGRESS_HEIGHT) * 0.6)
	_bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_bar)
	_clock = UiBuild.data_label("", &"HudLabel", false)
	_clock.name = "Clock"
	row.add_child(_clock)
	_lives = UiBuild.hbox()
	_lives.name = "Lives"
	row.add_child(_lives)
	_score = UiBuild.data_label("", &"HudLabel", false)
	_score.name = "Score"
	row.add_child(_score)
	_continue = UiBuild.button("ui.challenge.continue", "next_light")
	_continue.name = "Continue"
	_continue.pressed.connect(func() -> void: client.continue_run())
	row.add_child(_continue)
	_stop = UiBuild.button("ui.challenge.stop", "pause", &"SecondaryButton")
	_stop.name = "Stop"
	_stop.pressed.connect(ask_abandon)
	row.add_child(_stop)
	_build_confirm(column)
	_timer = Timer.new()
	_timer.wait_time = TICK_SEC
	_timer.timeout.connect(render_clock)
	add_child(_timer)
	visible = false


func _exit_tree() -> void:
	bind(null)


func bind(value: ChallengeClient) -> void:
	if client != null and client.attempt_changed.is_connected(_on_attempt_changed):
		client.attempt_changed.disconnect(_on_attempt_changed)
	client = value
	if client != null:
		client.attempt_changed.connect(_on_attempt_changed)
		_on_attempt_changed(client.attempt())


## `activity_open`: the activity player covers the world (the HUD stays, "Tiếp tục" hides).
func set_activity_open(open: bool) -> void:
	if _continue != null:
		_continue.visible = not open


func ask_abandon() -> void:
	_confirm.show()


## Coroutine: confirmed give-up (F069-AC-19).
func confirm_abandon() -> void:
	_confirm.hide()
	await client.abandon()


## Height of the strip (the activity player of a run starts below it).
func strip_height() -> float:
	return _panel.get_combined_minimum_size().y if _panel != null and visible else 0.0


func confirm_visible() -> bool:
	return _confirm.visible


func clock_text() -> String:
	return _clock.text if _clock.visible else ""


func bar_value() -> float:
	return _bar.value


func render_clock() -> void:
	if client == null or not client.is_running():
		return
	var left: int = client.store.remaining_ms()
	if left < 0:
		_bar.visible = false
		_clock.visible = false
		return
	_bar.visible = true
	_total_ms = maxi(_total_ms, left)
	_bar.value = ExamCountdown.sand_fraction(left, _total_ms)
	_clock.visible = band in SECONDS_BANDS
	_clock.text = ExamCountdown.clock_text(left) if left > 0 else tr("ui.challenge.time_up")
	_bar.tooltip_text = tr("ui.challenge.time_left_tooltip")


func _on_attempt_changed(a: ChallengeAttemptModel) -> void:
	visible = a != null and not a.is_finished()
	if not visible:
		_timer.stop()
		_confirm.hide()
		_total_ms = 0
		return
	band = UiBuild.band()
	UiBuild.size_buttons(self)
	var d: ChallengeDetailModel = client.store.detail
	_title.text = d.display_title() if d != null else tr("ui.challenge.board_title")
	if d != null and d.time_limit_seconds > 0:
		_total_ms = d.time_limit_seconds * 1000
	UiBuild.clear(_lives)
	if a.has_lives():
		for i: int in a.lives_left:
			var heart: TextureRect = UiBuild.icon_rect("heart", 0.75)
			_lives.add_child(heart)
		_lives.tooltip_text = tr("ui.challenge.lives").format({"n": LocaleFormat.number(a.lives_left)})
	_score.text = tr("ui.challenge.progress").format({"n": LocaleFormat.number(a.activities_completed),
		"score": LocaleFormat.number(a.score)})
	render_clock()
	if _timer.is_stopped():
		_timer.start()


func _build_confirm(parent: Control) -> void:
	_confirm = UiBuild.panel(&"FormCard")
	_confirm.name = "AbandonConfirm"
	_confirm.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	parent.add_child(_confirm)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_confirm.add_child(box)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.ENCOURAGE)
	box.add_child(coach["row"] as Control)
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.label("ui.challenge.stop_confirm", &"SpeechBubbleLabel"))
	var actions: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	actions.alignment = BoxContainer.ALIGNMENT_END
	box.add_child(actions)
	var keep: Button = UiBuild.button("ui.challenge.keep_playing", "next_light")
	keep.name = "KeepPlaying"
	keep.pressed.connect(func() -> void: _confirm.hide())
	actions.add_child(keep)
	var stop: Button = UiBuild.button("ui.challenge.stop_now", "pause", &"SecondaryButton")
	stop.name = "StopNow"
	stop.pressed.connect(confirm_abandon)
	actions.add_child(stop)
	_confirm.hide()
