extends PanelContainer
## One quest of the journal (F072, STUDENT_GAME_UX_CORE S05): the name in the UI language, the current step or the
## reason it cannot be taken yet, and one action — "Nhận" (accept), "Bắt đầu" (learning step) or "Theo dõi" (track).
## UI only: what is possible comes from the server list (ADR-0016, AGENTS R1).

signal action_pressed(quest_id: String, action: String)

const ACTION_ACCEPT: String = "accept"
const ACTION_START: String = "start"
const ACTION_TRACK: String = "track"
## F110: an active `interdependent` step of a party quest — "Cùng nhóm giải" opens its roles + clue cards.
const ACTION_COOP: String = "coop_step"
const KEY_COOP: String = "ui.quest.coop_step"
const LOCKED_KEYS: Dictionary = {
	"prerequisite_not_met": "ui.quest.locked.prerequisite", "cooldown": "ui.quest.locked.cooldown",
	"already_completed": "ui.quest.status.completed",
}

var model: QuestModel = null
var action: String = ""

@onready var _title: Label = %Title
@onready var _step: Label = %Step
@onready var _action: Button = %Action


func _ready() -> void:
	_action.pressed.connect(_on_action)


func bind(q: QuestModel) -> void:
	model = q
	render()


func render() -> void:
	if model == null or _title == null:
		return
	_title.text = model.display_name()
	_title.tooltip_text = _title.text
	action = ""
	_step.text = ""
	if model.is_active():
		var steps: Array[Dictionary] = model.active_steps()
		if not steps.is_empty():
			_step.text = L10n.pick(steps[0].get("title", {}) as Dictionary)
		action = ACTION_START if not model.activity_step().is_empty() else ACTION_TRACK
		if not model.interdependent_step().is_empty():
			action = ACTION_COOP
	elif model.status == QuestModel.STATUS_AVAILABLE and model.can_accept:
		action = ACTION_ACCEPT
	else:
		_step.text = tr(String(LOCKED_KEYS.get(model.locked_reason, "ui.quest.status." + model.status)))
	_action.visible = not action.is_empty()
	_action.text = (tr(KEY_COOP) if action == ACTION_COOP else tr("ui.quest." + action)) if not action.is_empty() else ""


func _on_action() -> void:
	if model != null and not action.is_empty():
		action_pressed.emit(model.quest_id, action)
