extends Control
## World Map S04 (screen `world_map`, F071 T044): areas open/locked with their conditions in words and the exploration
## percentage, fast travel to an open area; grade band g1_3 gets a one-tap picture board without percentages
## (F071-AC-26, AC-27). The data comes from WorldService (server), the travel goes through SceneRouter.request_area.
## UI only (AGENTS R1); no coin purchase exists here.

signal closed()
signal travel_requested(area_id: String)

var model: WorldMapModel = null

@onready var _list: VBoxContainer = %List
@onready var _board: GridContainer = %Board
@onready var _title: Label = %Title
@onready var _close: Button = %Close


func _ready() -> void:
	visible = false
	_close.pressed.connect(close)


func open(_args: Dictionary = {}) -> void:
	show()
	var world: WorldService = AppState.get_services().world
	await world.fetch_areas()
	await world.fetch_exploration()
	render(WorldMapModel.build(world.areas, world.exploration, AppState.grade_band, AppState.current_zone))


func close() -> void:
	if visible:
		hide()
		closed.emit()


## Draws `m` (also used by the tests with fixture data).
func render(m: WorldMapModel) -> void:
	model = m
	for child: Node in _list.get_children() + _board.get_children():
		child.queue_free()
	_list.visible = not m.picture_mode
	_board.visible = m.picture_mode
	_title.text = tr("ui.worldmap.title")
	for e: Dictionary in m.entries:
		if m.picture_mode:
			_board.add_child(_tile(e))
		else:
			_list.add_child(_row(e))


## g1_3: a picture tile without text (name only as tooltip for assistive tech); one tap travels.
func _tile(e: Dictionary) -> Button:
	var b: Button = Button.new()
	b.name = "Tile_" + String(e["area_id"]).replace(".", "_")
	b.custom_minimum_size = Vector2.ONE * UiTokens.DENSITY_G1_3_BUTTON_MIN_HEIGHT
	b.tooltip_text = L10n.pick(e["name"] as Dictionary)
	b.disabled = not bool(e["can_travel"])
	b.pressed.connect(_travel.bind(String(e["area_id"])))
	return b


func _row(e: Dictionary) -> HBoxContainer:
	var row: HBoxContainer = HBoxContainer.new()
	row.name = "Row_" + String(e["area_id"]).replace(".", "_")
	var name_label: Label = Label.new()
	name_label.text = L10n.pick(e["name"] as Dictionary)
	name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(name_label)
	var state: Label = Label.new()
	state.name = "State"
	if bool(e["accessible"]):
		state.text = tr("ui.worldmap.progress").format({"pct": int(round(float(e["pct"])))})
	else:
		var words: PackedStringArray = []
		for r: Dictionary in e["requirements"] as Array[Dictionary]:
			words.append(WorldMapModel.requirement_text(r))
		state.text = tr("ui.worldmap.locked") + (": " + ", ".join(words) if not words.is_empty() else "")
	state.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	row.add_child(state)
	if bool(e["can_travel"]):
		var go: Button = Button.new()
		go.name = "Travel"
		go.text = tr("ui.worldmap.travel")
		go.pressed.connect(_travel.bind(String(e["area_id"])))
		row.add_child(go)
	return row


func _travel(area_id: String) -> void:
	travel_requested.emit(area_id)
	close()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()
