extends PanelContainer
## Quest Tracker of the HUD (F072, STUDENT_GAME_UX_CORE S05): the tracked quest and its current step with the server
## count ("2/3") of a collect/discover step. Updated from QuestService (REST answers and `quest.progress.updated`),
## never polled; hidden without a tracked active quest. UI only (AGENTS R1).
## F110-AC-12: in a party quest session the PartyProgressStrip under the step shows the group share and whether the
## student reached their own part (CoopQuestService; never another member's numbers).

var service: QuestService = null
var coop: CoopQuestService = null
var strip: PartyProgressStrip = null

@onready var _title: Label = %Title
@onready var _step: Label = %Step


func _ready() -> void:
	if service == null:
		var services: ServiceLocator = AppState.get_services()
		service = services.quest if services != null else null
		if coop == null and services != null:
			coop = services.coop_quest
	strip = PartyProgressStrip.new()
	(get_node("Box") as Control).add_child(strip)
	if coop != null:
		coop.progress_changed.connect(_on_coop_changed)
	if service != null:
		service.tracked_changed.connect(_on_changed)
		service.progress_updated.connect(_on_changed)
		service.quests_updated.connect(_on_count)
	EventBus.locale_changed.connect(_on_locale_changed)
	get_viewport().size_changed.connect(func() -> void: _place.call_deferred())
	render()


func _exit_tree() -> void:
	if service != null and service.progress_updated.is_connected(_on_changed):
		service.tracked_changed.disconnect(_on_changed)
		service.progress_updated.disconnect(_on_changed)
		service.quests_updated.disconnect(_on_count)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	if coop != null and coop.progress_changed.is_connected(_on_coop_changed):
		coop.progress_changed.disconnect(_on_coop_changed)


## F110: on the HUD the tracker sits under the top bar and left of the right column (it grew with the party strip and
## must not cover the HUD buttons).
func _place() -> void:
	var hud: Node = get_parent()
	if hud == null or not is_inside_tree():
		return
	var top: Control = hud.find_child("TopBar", true, false) as Control
	var side: Control = hud.find_child("SideBar", true, false) as Control
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	if top != null and top.is_visible_in_tree():
		offset_top = top.get_global_rect().end.y + gap
	if side != null and side.is_visible_in_tree():
		offset_right = side.get_global_rect().position.x - get_viewport_rect().size.x - gap


func render() -> void:
	var q: QuestModel = service.quest(service.tracked_quest_id) if service != null else null
	visible = q != null and q.is_active()
	_place.call_deferred()
	if not visible or _title == null:
		return
	_title.text = q.display_name()
	var steps: Array[Dictionary] = q.active_steps()
	if steps.is_empty():
		_step.text = ""
		if strip != null:
			strip.show_progress(null)
		return
	var s: Dictionary = steps[0]
	if strip != null:
		strip.show_progress(coop.store.progress(q.quest_id, String(s.get("step_id", ""))) if coop != null else null)
	var text: String = L10n.pick(s.get("title", {}) as Dictionary)
	var st: Dictionary = q.step_states.get(String(s.get("step_id", "")), {}) as Dictionary
	if st.has("target"):
		text = tr("ui.quest.tracker.count").format({"step": text, "count": int(st.get("count", 0)), "target": int(st["target"])})
	_step.text = text


func _on_changed(_quest_id: String) -> void:
	render()


func _on_coop_changed(_quest_id: String, _step_id: String) -> void:
	render()


func _on_count(_count: int) -> void:
	render()


func _on_locale_changed(_locale: String) -> void:
	render()
