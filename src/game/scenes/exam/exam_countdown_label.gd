extends HBoxContainer
## Exam clock of the runner header (F064-AC-03, F064-AC-04; STUDENT_GAME_UX_LEARNING §9.2, §9.4). Draws what
## ExamCountdown computed from the server deadline: `mm:ss` left, or for `g1_3` a sand glass bar without seconds. In the
## last 5 and 1 minutes the time switches to the steady `ExamClockWarningLabel` colour (design/game-tokens) — no
## blinking, no animation. UI only.

const WARNING_VARIATION: StringName = &"ExamClockWarningLabel"

var warning: bool = false

@onready var _time: Label = %Time
@onready var _icon: Label = %Icon
@onready var _sand: ProgressBar = %Sand


## `left_ms` from ExamService.remaining_ms(); `total_ms` the student's duration (sand glass fill).
func render(left_ms: int, total_ms: int, grade_band: String) -> void:
	var digits: bool = ExamCountdown.shows_seconds(grade_band)
	_time.visible = digits
	_sand.visible = not digits
	if digits:
		_time.text = tr("ui.exam.time_left").format({"time": ExamCountdown.clock_text(left_ms)})
	else:
		_sand.value = ExamCountdown.sand_fraction(left_ms, total_ms)
	warning = ExamCountdown.warning_level(left_ms) > 0
	var variation: StringName = WARNING_VARIATION if warning else &""
	_time.theme_type_variation = variation
	_icon.theme_type_variation = variation


func time_text() -> String:
	return _time.text if _time.visible else ""
