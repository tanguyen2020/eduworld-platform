class_name PhaseTimer
extends HBoxContainer
## S06 `PhaseTimer` (F110-AC-03; STUDENT_GAME_UX_CORE S06 §10): the time left in the phase from the server
## `phase_deadline_at` (or the encounter `deadline_at`) on the server clock (BossClient.now_ms — display lag ≤ 1 s,
## AC-UX-0602): lớp 1–3 a coloured sand bar WITHOUT seconds, older grades the bar + mm:ss. Hidden without a deadline.
## UI only.

const SECONDS_BANDS: PackedStringArray = ["g4_5", "g6_7", "g8_9"]

var _bar: ProgressBar = null
var _clock: Label = null
var _icon: TextureRect = null
var _total_ms: int = 0
var _phase_key: String = ""


func _init() -> void:
	name = "PhaseTimer"
	theme_type_variation = &"ToolbarBox"
	_icon = UiBuild.icon_rect("clock", 0.8)
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	add_child(_icon)
	_bar = UiBuild.progress_bar(1.0)
	_bar.name = "TimeBar"
	add_child(_bar)
	_clock = UiBuild.data_label("", &"HudLabel", false)
	_clock.name = "Clock"
	_clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	add_child(_clock)


## `left_ms` < 0 hides the timer; `phase_key` resets the bar's full length on a new phase / retry.
func render(left_ms: int, band: String, phase_key: String) -> void:
	visible = left_ms >= 0
	if not visible:
		return
	if phase_key != _phase_key:
		_phase_key = phase_key
		_total_ms = 0
	_total_ms = maxi(_total_ms, left_ms)
	_bar.value = ExamCountdown.sand_fraction(left_ms, _total_ms)
	_clock.visible = band in SECONDS_BANDS
	_clock.text = ExamCountdown.clock_text(left_ms)
	_bar.tooltip_text = tr("ui.boss.time_left")


func clock_text() -> String:
	return _clock.text if _clock.visible and visible else ""


func bar_value() -> float:
	return _bar.value
