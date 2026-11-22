extends PanelContainer
## Quest Tracker of the HUD (F072, STUDENT_GAME_UX_CORE S05): the tracked quest and its current step with the server
## count ("2/3") of a collect/discover step. Updated from QuestService (REST answers and `quest.progress.updated`),
## never polled; hidden without a tracked active quest. UI only (AGENTS R1).

var service: QuestService = null

@onready var _title: Label = %Title
@onready var _step: Label = %Step


func _ready() -> void:
	if service == null:
		var services: ServiceLocator = AppState.get_services()
		service = services.quest if services != null else null
	if service != null:
		service.tracked_changed.connect(_on_changed)
		service.progress_updated.connect(_on_changed)
		service.quests_updated.connect(_on_count)
	EventBus.locale_changed.connect(_on_locale_changed)
	render()


func _exit_tree() -> void:
	if service != null and service.progress_updated.is_connected(_on_changed):
		service.tracked_changed.disconnect(_on_changed)
		service.progress_updated.disconnect(_on_changed)
		service.quests_updated.disconnect(_on_count)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func render() -> void:
	var q: QuestModel = service.quest(service.tracked_quest_id) if service != null else null
	visible = q != null and q.is_active()
	if not visible or _title == null:
		return
	_title.text = q.display_name()
	var steps: Array[Dictionary] = q.active_steps()
	if steps.is_empty():
		_step.text = ""
		return
	var s: Dictionary = steps[0]
	var text: String = L10n.pick(s.get("title", {}) as Dictionary)
	var st: Dictionary = q.step_states.get(String(s.get("step_id", "")), {}) as Dictionary
	if st.has("target"):
		text = tr("ui.quest.tracker.count").format({"step": text, "count": int(st.get("count", 0)), "target": int(st["target"])})
	_step.text = text


func _on_changed(_quest_id: String) -> void:
	render()


func _on_count(_count: int) -> void:
	render()


func _on_locale_changed(_locale: String) -> void:
	render()
