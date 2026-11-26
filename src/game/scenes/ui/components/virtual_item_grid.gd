class_name VirtualItemGrid
extends ScrollContainer
## Virtualised grid of item cells (S08 `VirtualItemGrid`, F069-AC-07, AC-27; STUDENT_GAME_UX_SOCIAL §3 "chỉ tạo Control
## cho ô hiển thị + 1 hàng đệm"): the pool holds the cells of the visible rows plus ONE buffer row whatever the list
## size (500 items → a few dozen Controls), re-bound while scrolling. A cell is a `PictureTile` with the 2D item picture
## (ItemThumb: vector glyph, no 3D model), the rarity mark (colour + shape, F069-AC-26), the name and a second line (price,
## "Đã sở hữu", "Chưa có"…), a tick when worn. The cell size follows the grade band (g1_3 bigger, ≥ 64 dp targets).
## Entries are display dictionaries built by the screen; the grid knows no rule (AGENTS R1).

signal entry_selected(index: int)

## Rows created beyond the visible ones (scrolling never shows an empty row).
const BUFFER_ROWS: int = 1
## Text lines under the picture (name + second line) at least / at most (F098: the cell grows with the longest
## wrapped text of the list so a long name never spills over the next row; past the cap the text is clipped).
const TEXT_LINES_MIN: int = 2
const TEXT_LINES_MAX: int = 5

## `{item_id, kind, rarity, name, sub, greyed, equipped, word_rarity}` per cell.
var entries: Array[Dictionary] = []
var cells: Array[Button] = []
var selected_index: int = -1
var columns: int = 1
var cell_size: Vector2 = Vector2.ZERO
var _content: Control = null
var _group: ButtonGroup = ButtonGroup.new()
## F098: tallest cell the bound texts really needed (the estimate can be a line short); never shrinks for a list.
var _measured_height: float = 0.0


func _ready() -> void:
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	size_flags_vertical = Control.SIZE_EXPAND_FILL
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_content = Control.new()
	_content.name = "Content"
	_content.mouse_filter = Control.MOUSE_FILTER_PASS
	add_child(_content)
	get_v_scroll_bar().value_changed.connect(func(_v: float) -> void: _layout())
	resized.connect(relayout)


func set_entries(list: Array[Dictionary], keep_selection: bool = false) -> void:
	entries = list
	_measured_height = 0.0
	if not keep_selection or selected_index >= entries.size():
		selected_index = -1
	relayout()


## Cells alive right now (pool size): visible rows + BUFFER_ROWS, times the columns, never more than the entries.
func pool_size() -> int:
	return cells.size()


func select(index: int) -> void:
	selected_index = index if index >= 0 and index < entries.size() else -1
	_layout()
	if selected_index >= 0:
		entry_selected.emit(selected_index)


## Recomputes the cell size (grade band), the columns (width) and the pool (height).
func relayout() -> void:
	if _content == null:
		return
	var target: float = UiBuild.target()
	var box: StyleBox = get_theme_stylebox(&"normal", &"PictureTile")
	var top: float = box.content_margin_top if box != null else target * 1.6
	var font_px: float = float(get_theme_font_size(&"font_size", &"Button"))
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	# Wide enough for one long word ("Celebrate", "Xu") on a line: never a break inside a word.
	var min_width: float = float(UiTokens.SIZE_LAYOUT_120) + float(UiTokens.SIZE_SPACE_12) * 2.0
	var width_cell: float = maxf(maxf(top + target * 0.6, target * 2.0), min_width)
	cell_size = Vector2(width_cell, maxf(top + _text_height(box, width_cell, font_px), _measured_height))
	var width: float = maxf(size.x - float(UiTokens.SIZE_SPACE_12), cell_size.x)
	columns = maxi(1, int((width + gap) / (cell_size.x + gap)))
	var rows: int = ceili(float(entries.size()) / float(columns))
	_content.custom_minimum_size = Vector2(width, float(rows) * (cell_size.y + gap))
	var visible_rows: int = ceili(maxf(size.y, cell_size.y) / (cell_size.y + gap)) + BUFFER_ROWS
	var wanted: int = mini(entries.size(), visible_rows * columns)
	while cells.size() < wanted:
		cells.append(_make_cell(cells.size()))
	while cells.size() > wanted:
		var c: Button = cells.pop_back() as Button
		c.queue_free()
	_layout()


## Height of the text block of a cell (F098): the tallest wrapped name + second line of the entries at the cell's
## text width, between TEXT_LINES_MIN and TEXT_LINES_MAX lines, plus the bottom padding — measured with the cell font.
func _text_height(box: StyleBox, cell_width: float, font_px: float) -> float:
	var font: Font = get_theme_font(&"font", &"PictureTile")
	var line: float = font.get_height(int(font_px)) if font != null else font_px * 1.4
	var bottom: float = box.content_margin_bottom if box != null else 0.0
	var text_width: float = cell_width - (box.content_margin_left + box.content_margin_right if box != null else 0.0)
	var tallest: float = line * float(TEXT_LINES_MIN)
	if font != null and text_width > 0.0:
		for e: Dictionary in entries:
			var name_text: String = String(e.get("name", ""))
			var sub: String = String(e.get("sub", ""))
			var text: String = name_text if sub.is_empty() else "%s\n%s" % [name_text, sub]
			var h: float = font.get_multiline_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, text_width, int(font_px), -1,
					TextServer.BREAK_MANDATORY | TextServer.BREAK_WORD_BOUND).y
			tallest = maxf(tallest, h)
	return minf(tallest, line * float(TEXT_LINES_MAX)) + bottom


func _layout() -> void:
	if _content == null or cells.is_empty():
		return
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	var first_row: int = int(float(scroll_vertical) / (cell_size.y + gap))
	for i: int in cells.size():
		var index: int = first_row * columns + i
		var cell: Button = cells[i]
		if index >= entries.size():
			cell.visible = false
			continue
		cell.visible = true
		cell.position = Vector2(float(index % columns) * (cell_size.x + gap), float(index / columns) * (cell_size.y + gap))
		cell.size = cell_size
		cell.custom_minimum_size = cell_size
		_bind(cell, index)
	# F098: a bound text taller than the estimate grows every cell (rows never overlap), then lays out again.
	var tallest: float = cell_size.y
	for cell: Button in cells:
		if cell.visible:
			cell.custom_minimum_size = Vector2.ZERO
			tallest = maxf(tallest, cell.get_combined_minimum_size().y)
			cell.custom_minimum_size = cell_size
	if tallest > cell_size.y + 0.5:
		_measured_height = ceilf(tallest)
		relayout.call_deferred()


func _make_cell(slot: int) -> Button:
	var b: Button = Button.new()
	b.name = "Cell%d" % slot
	b.theme_type_variation = &"PictureTile"
	b.toggle_mode = true
	b.button_group = _group
	b.clip_text = true
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.vertical_icon_alignment = VERTICAL_ALIGNMENT_TOP
	var thumb: ItemThumb = ItemThumb.new()
	thumb.name = "Thumb"
	b.add_child(thumb)
	var mark: RarityMark = RarityMark.new()
	mark.name = "Rarity"
	mark.position = Vector2(float(UiTokens.SIZE_SPACE_8), float(UiTokens.SIZE_SPACE_8))
	b.add_child(mark)
	var tick: TextureRect = UiBuild.icon_rect("verdict_correct")
	tick.name = "Worn"
	b.add_child(tick)
	b.pressed.connect(func() -> void: select(int(b.get_meta(&"index", -1))))
	_content.add_child(b)
	return b


func _bind(cell: Button, index: int) -> void:
	var e: Dictionary = entries[index]
	cell.set_meta(&"index", index)
	var name_text: String = String(e.get("name", ""))
	var sub: String = String(e.get("sub", ""))
	cell.text = name_text if sub.is_empty() else "%s\n%s" % [name_text, sub]
	cell.tooltip_text = cell.text
	cell.set_pressed_no_signal(index == selected_index)
	var box: StyleBox = cell.get_theme_stylebox(&"normal")
	var pad: float = box.content_margin_left if box != null else 0.0
	var top: float = box.content_margin_top if box != null else cell_size.y * 0.5
	var thumb: ItemThumb = cell.get_node("Thumb") as ItemThumb
	thumb.position = Vector2(pad, pad)
	thumb.size = Vector2(cell_size.x - pad * 2.0, top - pad)
	thumb.set_item(String(e.get("kind", "")), String(e.get("item_id", "")), String(e.get("rarity", "common")),
			bool(e.get("greyed", false)))
	var mark: RarityMark = cell.get_node("Rarity") as RarityMark
	mark.set_rarity(String(e.get("rarity", "common")), bool(e.get("word_rarity", false)))
	var tick: TextureRect = cell.get_node("Worn") as TextureRect
	tick.visible = bool(e.get("equipped", false))
	tick.position = Vector2(cell_size.x - tick.custom_minimum_size.x - pad * 2.0, pad * 2.0)
