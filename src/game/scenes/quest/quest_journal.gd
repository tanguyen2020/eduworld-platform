extends Control
## Quest journal — tab "Nhà trường giao" only in F062 (STUDENT_GAME_UX_CORE S05; other tabs: F072). Opening it
## refreshes `listMyAssignments` through AssignmentService (F062-AC-07); rows show the student's own assignments
## (F062-AC-08/11) in a virtualised list (≤ 12 row nodes, S05 §13); "Bắt đầu" asks the service for the intent
## (F062-AC-09) and shows `ui.activity_player.update_required` when this client has no screen for it (F062-AC-10).
## Loading / empty / error (+ "Thử lại", previous list kept, F062-AC-12) states. UI only: no network, no rules.

signal closed()

const ROW_SCENE: PackedScene = preload("res://scenes/quest/AssignmentRow.tscn")
## S05 §13: at most 12 row nodes alive at once.
const MAX_ROW_NODES: int = 12
const TAB_ASSIGNED: String = "assigned"

var tab: String = TAB_ASSIGNED
var rows: Array[Control] = []
var row_height: float = 0.0

@onready var _close: Button = %Close
@onready var _state: Label = %StateLabel
@onready var _retry: Button = %Retry
@onready var _scroll: ScrollContainer = %List
@onready var _content: Control = %Content
@onready var _notice: PanelContainer = %Notice
@onready var _tab_assigned: Button = %TabAssigned


func _ready() -> void:
	_close.pressed.connect(close)
	_retry.pressed.connect(refresh)
	_scroll.get_v_scroll_bar().value_changed.connect(_on_scrolled)
	_scroll.resized.connect(_layout_rows)
	AppState.assignments_changed.connect(_on_assignments_changed)
	EventBus.locale_changed.connect(_on_locale_changed)
	var service: AssignmentService = _service()
	if service != null:
		service.state_changed.connect(_on_service_state_changed)
	_render_state()


func _exit_tree() -> void:
	if AppState.assignments_changed.is_connected(_on_assignments_changed):
		AppState.assignments_changed.disconnect(_on_assignments_changed)
	var service: AssignmentService = _service()
	if service != null and service.state_changed.is_connected(_on_service_state_changed):
		service.state_changed.disconnect(_on_service_state_changed)


## WorldRoot: `ui_requested("quest_journal", {"tab": "assigned"})`.
func open(args: Dictionary = {}) -> void:
	tab = String(args.get("tab", TAB_ASSIGNED))
	_tab_assigned.button_pressed = true
	show()
	_rebuild()
	refresh()


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
	if _state == null:
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
