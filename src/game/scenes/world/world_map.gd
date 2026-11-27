extends Control
## World Map S04 (screen `world_map`, F071 T044): areas open/locked with their conditions in words and the exploration
## percentage, fast travel to an open area; grade band g1_3 gets a one-tap picture board without percentages
## (F071-AC-26, AC-27). The data comes from WorldService (server), the travel goes through SceneRouter.request_area.
## UI only (AGENTS R1); no coin purchase exists here.

signal closed()
signal travel_requested(area_id: String)

const ICON_PATH: String = "res://resources/ui/icons/area_%s.svg"
const ICON_FALLBACK: String = "central"
## g1_3 tile size in touch-target units (icon above a two-line name).
const TILE_WIDTH_FACTOR: float = 3.0
const TILE_HEIGHT_FACTOR: float = 2.0

var model: WorldMapModel = null

@onready var _list: VBoxContainer = %List
@onready var _board: GridContainer = %Board
@onready var _title: Label = %Title
@onready var _close: Button = %Close


func _ready() -> void:
	visible = false
	UiBuild.fit_icon(%TitleIcon as TextureRect)
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


## Icon of an area (`icon_key` of the world seed → design/icons/game `area_<icon_key>.svg`, F096-AC-12).
static func area_icon(icon_key: String) -> Texture2D:
	var path: String = ICON_PATH % icon_key
	return load(path if ResourceLoader.exists(path) else ICON_PATH % ICON_FALLBACK) as Texture2D


## g1_3: a big picture tile — the area icon above its short name — one tap travels (F071-AC-27, F096-AC-12). The area
## the student stands in is shown but cannot be chosen.
func _tile(e: Dictionary) -> Button:
	var b: Button = Button.new()
	b.name = "Tile_" + String(e["area_id"]).replace(".", "_")
	b.theme_type_variation = &"SecondaryButton"
	var side: float = float(UiTokens.DENSITY_G1_3_BUTTON_MIN_HEIGHT)
	b.custom_minimum_size = Vector2(side * TILE_WIDTH_FACTOR, side * TILE_HEIGHT_FACTOR)
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.icon = area_icon(String(e.get("icon_key", "")))
	b.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
	b.vertical_icon_alignment = VERTICAL_ALIGNMENT_TOP
	b.expand_icon = false
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.text = L10n.pick(e["name"] as Dictionary)
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.tooltip_text = b.text
	b.disabled = not bool(e["can_travel"])
	b.pressed.connect(_travel.bind(String(e["area_id"])))
	return b


## Older bands: icon, then the name over the progress or the unlock conditions (wrapping in their own column), then
## "Đến đây" with its label (F096-AC-12: the row never squeezes a column to one character).
func _row(e: Dictionary) -> HBoxContainer:
	var row: HBoxContainer = HBoxContainer.new()
	row.name = "Row_" + String(e["area_id"]).replace(".", "_")
	row.theme_type_variation = &"ToolbarBox"
	var icon: TextureRect = TextureRect.new()
	icon.texture = area_icon(String(e.get("icon_key", "")))
	icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	icon.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = VBoxContainer.new()
	texts.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	texts.theme_type_variation = &"FieldBox"
	row.add_child(texts)
	var name_label: Label = Label.new()
	name_label.name = "Name"
	name_label.theme_type_variation = &"HeadingLabel"
	name_label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	name_label.text = L10n.pick(e["name"] as Dictionary)
	name_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	texts.add_child(name_label)
	var state: Label = Label.new()
	state.name = "State"
	state.theme_type_variation = &"MutedLabel"
	state.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	if bool(e["accessible"]):
		state.text = tr("ui.worldmap.progress").format({"pct": int(round(float(e["pct"])))})
	else:
		var words: PackedStringArray = []
		for r: Dictionary in e["requirements"] as Array[Dictionary]:
			words.append(WorldMapModel.requirement_text(r))
		state.text = tr("ui.worldmap.locked") + (": " + ", ".join(words) if not words.is_empty() else "")
	state.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	texts.add_child(state)
	if bool(e["can_travel"]):
		var go: Button = Button.new()
		go.name = "Travel"
		go.size_flags_vertical = Control.SIZE_SHRINK_CENTER
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
