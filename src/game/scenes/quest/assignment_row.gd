extends PanelContainer
## One row of "Nhà trường giao" (STUDENT_GAME_UX_CORE S05, F062-AC-08/11): status icon + text, the title exactly as
## the school typed it (one line, ellipsis + tooltip), the due date in the UI locale (LocaleFormat, not the OS) and
## "Bắt đầu". The subject name (`subject_name`, OQ F062-1) is server data picked in the UI locale, hidden when absent.
## Only the student's own data: no score, no other student, no teacher comment (AGENTS R16).

signal start_pressed(assignment: MyAssignmentModel)

## Icon per status (shown with the status text, never colour alone; neutral style for `overdue`). F096-AC-15: icons of
## design/icons/game, not glyphs the shipped fonts lack (✓ was an empty box on the Web build).
const STATUS_ICONS: Dictionary = {
	"not_started": preload("res://resources/ui/icons/assignments.svg"),
	"in_progress": preload("res://resources/ui/icons/next.svg"),
	"submitted": preload("res://resources/ui/icons/tick.svg"), "graded": preload("res://resources/ui/icons/tick.svg"),
	"overdue": preload("res://resources/ui/icons/alert.svg"), "excused": preload("res://resources/ui/icons/info.svg"),
}
const STATUS_ICON_UNKNOWN: Texture2D = preload("res://resources/ui/icons/help.svg")
const STATUS_KEYS: Dictionary = {
	"not_started": "ui.quest.status.not_started", "in_progress": "ui.quest.status.in_progress",
	"submitted": "ui.quest.status.submitted", "graded": "ui.quest.status.graded",
	"overdue": "ui.quest.overdue", "excused": "ui.quest.status.excused",
}
const UNKNOWN_STATUS_KEY: String = "ui.quest.status.unsupported"
## Statuses without a "Bắt đầu" button (the school excused the student).
const NO_START_STATUSES: PackedStringArray = ["excused"]

var assignment: MyAssignmentModel = null

@onready var _icon: TextureRect = %StatusIcon
@onready var _title: Label = %Title
@onready var _subject: Label = %Subject
@onready var _due: Label = %Due
@onready var _status: Label = %Status
@onready var _start: Button = %Start


func _ready() -> void:
	_icon.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE
	_start.pressed.connect(_on_start_pressed)


func bind(item: MyAssignmentModel) -> void:
	assignment = item
	render()


func render() -> void:
	if assignment == null or _title == null:
		return
	_title.text = assignment.title
	_title.tooltip_text = assignment.title
	_subject.text = assignment.subject_text()
	_subject.visible = not _subject.text.is_empty()
	_icon.texture = STATUS_ICONS.get(assignment.status, STATUS_ICON_UNKNOWN) as Texture2D
	var status_text: String = tr(String(STATUS_KEYS.get(assignment.status, UNKNOWN_STATUS_KEY)))
	if assignment.status == MyAssignmentModel.STATUS_OVERDUE and assignment.allow_late:
		status_text += " · " + tr("ui.quest.late_allowed")
	_status.text = status_text
	_due.text = due_text(assignment)
	_start.visible = not assignment.status in NO_START_STATUSES


## Due date/time in the current UI locale ("" when the assignment has no deadline).
static func due_text(item: MyAssignmentModel, locale: String = "") -> String:
	var unix: int = item.due_unix()
	if unix <= 0:
		return ""
	var dt: Dictionary = LocaleFormat.from_unix(unix)
	return TranslationServer.translate("ui.quest.due").format({
		"date": LocaleFormat.date(dt, locale), "time": LocaleFormat.time(dt, locale),
	})


func _on_start_pressed() -> void:
	if assignment != null:
		start_pressed.emit(assignment)
