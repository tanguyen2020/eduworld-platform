extends Control
## Quest journal — tabs "Nhà trường giao" (F062) and "Đang làm" / "Khả dụng" of the quests (F072, QuestService:
## accept, start the learning step, track; the list is the server's for the grade and tenant of the student). Opening it
## refreshes `listMyAssignments` through AssignmentService (F062-AC-07); rows show the student's own assignments
## (F062-AC-08/11) in a virtualised list (≤ 12 row nodes, S05 §13); "Bắt đầu" asks the service for the intent
## (F062-AC-09) and shows `ui.activity_player.update_required` when this client has no screen for it (F062-AC-10).
## Loading / empty / error (+ "Thử lại", previous list kept, F062-AC-12) states. UI only: no network, no rules.

signal closed()

const ROW_SCENE: PackedScene = preload("res://scenes/quest/AssignmentRow.tscn")
const QUEST_ROW_SCENE: PackedScene = preload("res://scenes/quest/QuestRow.tscn")
## S05 §13: at most 12 row nodes alive at once.
const MAX_ROW_NODES: int = 12
const TAB_ASSIGNED: String = "assigned"
const TAB_ACTIVE: String = "active"
const TAB_AVAILABLE: String = "available"

var tab: String = TAB_ASSIGNED
var rows: Array[Control] = []
var row_height: float = 0.0

@onready var _close: Button = %Close
@onready var _state: Label = %StateLabel
@onready var _retry: Button = %Retry
@onready var _scroll: ScrollContainer = %List
@onready var _content: Control = %Content
@onready var _notice: PanelContainer = %Notice
## F097: the chosen tab is a filled chip with a tick (not colour alone, NFR-A11Y-001).
const ICON_TAB_CHOSEN: Texture2D = preload("res://resources/ui/icons/check.svg")

@onready var _tab_assigned: Button = %TabAssigned
@onready var _tab_active: Button = %TabActive
@onready var _tab_available: Button = %TabAvailable
@onready var _quest_scroll: ScrollContainer = %QuestScroll
@onready var _quest_rows: VBoxContainer = %QuestRows


func _ready() -> void:
	UiBuild.fit_icon(%TitleIcon as TextureRect)
	_close.pressed.connect(close)
	_retry.pressed.connect(refresh)
	_scroll.get_v_scroll_bar().value_changed.connect(_on_scrolled)
	_scroll.resized.connect(_layout_rows)
	AppState.assignments_changed.connect(_on_assignments_changed)
	EventBus.locale_changed.connect(_on_locale_changed)
	var service: AssignmentService = _service()
	if service != null:
		service.state_changed.connect(_on_service_state_changed)
	_tab_assigned.pressed.connect(_select_tab.bind(TAB_ASSIGNED))
	_tab_active.pressed.connect(_select_tab.bind(TAB_ACTIVE))
	_tab_available.pressed.connect(_select_tab.bind(TAB_AVAILABLE))
	var quests: QuestService = _quests()
	if quests != null:
		quests.quests_updated.connect(_on_quests_updated)
		quests.progress_updated.connect(_on_quest_changed)
	_render_state()


func _exit_tree() -> void:
	var quests: QuestService = _quests()
	if quests != null and quests.quests_updated.is_connected(_on_quests_updated):
		quests.quests_updated.disconnect(_on_quests_updated)
		quests.progress_updated.disconnect(_on_quest_changed)
	if AppState.assignments_changed.is_connected(_on_assignments_changed):
		AppState.assignments_changed.disconnect(_on_assignments_changed)
	var service: AssignmentService = _service()
	if service != null and service.state_changed.is_connected(_on_service_state_changed):
		service.state_changed.disconnect(_on_service_state_changed)


## WorldRoot: `ui_requested("quest_journal", {"tab": "assigned"})`.
func open(args: Dictionary = {}) -> void:
	show()
	_select_tab(String(args.get("tab", TAB_ASSIGNED)))


func close() -> void:
	_notice.call("close")
	hide()
	closed.emit()


## F062-AC-07: opening the journal or "Thử lại" reloads the list (the service keeps the old one on failure).
func refresh() -> void:
	var service: AssignmentService = _service()
	if service != null:
		service.refresh()


func visible_row_count() -> int:
	var n: int = 0
	for row: Control in rows:
		if row.visible:
			n += 1
	return n


func _unhandled_input(event: InputEvent) -> void:
	if visible and (event.is_action_pressed("ui_cancel") or event.is_action_pressed("open_journal")):
		close()
		get_viewport().set_input_as_handled()


func _rebuild() -> void:
	var count: int = AppState.assignments.size()
	var wanted: int = mini(count, MAX_ROW_NODES)
	while rows.size() < wanted:
		var row: Control = ROW_SCENE.instantiate() as Control
		row.connect("start_pressed", _on_start_pressed)
		_content.add_child(row)
		rows.append(row)
	if row_height <= 0.0 and not rows.is_empty():
		row_height = maxf(rows[0].get_combined_minimum_size().y, float(UiTokens.SIZE_CONTROL_64)) \
				+ float(UiTokens.SIZE_SPACE_12)
	_content.custom_minimum_size = Vector2(0.0, row_height * float(count))
	_layout_rows()
	_render_state()


## Virtualisation: the pooled rows show the assignments under the scroll position.
func _layout_rows() -> void:
	var list: Array[MyAssignmentModel] = AppState.assignments
	var first: int = int(_scroll.scroll_vertical / row_height) if row_height > 0.0 else 0
	for i: int in rows.size():
		var index: int = first + i
		var row: Control = rows[i]
		if index >= list.size():
			row.visible = false
			continue
		row.visible = true
		row.position = Vector2(0.0, float(index) * row_height)
		row.size = Vector2(_content.size.x, row_height - float(UiTokens.SIZE_SPACE_12))
		row.call("bind", list[index])


func _render_state() -> void:
	if _state == null or tab != TAB_ASSIGNED:
		return
	var service: AssignmentService = _service()
	var state: int = service.state if service != null else AssignmentService.State.IDLE
	var empty: bool = AppState.assignments.is_empty()
	_retry.visible = state == AssignmentService.State.ERROR
	if state == AssignmentService.State.ERROR:
		_state.text = tr("ui.quest.load_failed")
	elif state == AssignmentService.State.LOADING and empty:
		_state.text = tr("ui.quest.loading")
	elif empty:
		_state.text = tr("ui.quest.empty")
	else:
		_state.text = ""
	_state.visible = not _state.text.is_empty()


func _on_start_pressed(assignment: MyAssignmentModel) -> void:
	var service: AssignmentService = _service()
	var screen: StringName = service.start(assignment) if service != null else &""
	if screen.is_empty():
		_notice.call("open", "ui.activity_player.update_required")


func _on_assignments_changed(_count: int) -> void:
	if visible:
		_rebuild()


func _on_service_state_changed(_state_value: int) -> void:
	_render_state()


func _on_scrolled(_value: float) -> void:
	_layout_rows()


func _on_locale_changed(_locale: String) -> void:
	for row: Control in rows:
		row.call("render")
	_render_state()


func _service() -> AssignmentService:
	var services: ServiceLocator = AppState.get_services()
	return services.assignments if services != null else null


func _select_tab(next: String) -> void:
	tab = next
	_tab_assigned.button_pressed = tab == TAB_ASSIGNED
	_tab_active.button_pressed = tab == TAB_ACTIVE
	_tab_available.button_pressed = tab == TAB_AVAILABLE
	for b: Button in [_tab_assigned, _tab_active, _tab_available]:
		b.icon = ICON_TAB_CHOSEN if b.button_pressed else null
	var quest_tab: bool = tab != TAB_ASSIGNED
	_scroll.visible = not quest_tab
	_quest_scroll.visible = quest_tab
	if quest_tab:
		_render_quests()
		var quests: QuestService = _quests()
		if quests != null:
			quests.refresh()
	else:
		_rebuild()
		refresh()


## Rows of the quests of the tab: "Đang làm" = active, "Khả dụng" = available (accept or the reason it waits).
func _render_quests() -> void:
	for c: Node in _quest_rows.get_children():
		c.queue_free()
	var quests: QuestService = _quests()
	if quests == null:
		return
	var n: int = 0
	for q: QuestModel in quests.quests:
		var wanted: bool = q.is_active() if tab == TAB_ACTIVE else q.status == QuestModel.STATUS_AVAILABLE
		if not wanted:
			continue
		var row: Control = QUEST_ROW_SCENE.instantiate() as Control
		row.connect("action_pressed", _on_quest_action)
		_quest_rows.add_child(row)
		row.call("bind", q)
		n += 1
	_state.text = tr("ui.quest.none") if n == 0 else ""
	_state.visible = n == 0


func _on_quest_action(quest_id: String, action: String) -> void:
	var quests: QuestService = _quests()
	if quests == null:
		return
	match action:
		"accept":
			await quests.accept(quest_id)
		"start":
			var q: QuestModel = quests.quest(quest_id)
			if q != null:
				await quests.start_activity(quest_id, String(q.activity_step().get("step_id", "")))
		"track":
			quests.track(quest_id)
	if tab != TAB_ASSIGNED:
		_render_quests()


func _on_quests_updated(_count: int) -> void:
	if visible and tab != TAB_ASSIGNED:
		_render_quests()


func _on_quest_changed(_quest_id: String) -> void:
	if visible and tab != TAB_ASSIGNED:
		_render_quests()


func _quests() -> QuestService:
	var services: ServiceLocator = AppState.get_services()
	return services.quest if services != null else null
