class_name ChipScroller
extends HBoxContainer
## A long row of chips on one line (F099, STUDENT_GAME_UX_CORE S08 filter chips): the row scrolls sideways inside a
## ScrollContainer and, only when it does not fit, an arrow button on each side shows that more chips are hidden and
## scrolls by most of the visible width (the arrow at an end of the row is disabled). Swiping still works. Built by
## `UiBuild.chip_scroller`. Display only.

## Share of the visible width scrolled by one arrow press.
const PAGE_SHARE: float = 0.7

var scroll: ScrollContainer = null
var prev: Button = null
var next: Button = null


func _init(row: Control = null) -> void:
	name = "ChipScroller"
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	prev = UiBuild.button("", "arrow_left", &"SecondaryButton")
	prev.name = "ScrollPrev"
	prev.tooltip_text = "ui.common.scroll_back"
	prev.pressed.connect(func() -> void: _page(-1.0))
	add_child(prev)
	scroll = ScrollContainer.new()
	scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_NEVER
	scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	add_child(scroll)
	next = UiBuild.button("", "arrow_right", &"SecondaryButton")
	next.name = "ScrollNext"
	next.tooltip_text = "ui.common.scroll_more"
	next.pressed.connect(func() -> void: _page(1.0))
	add_child(next)
	if row != null:
		scroll.add_child(row)
		row.resized.connect(_refresh)
		# Chips that get narrower (a chosen tab, a translation) may fit again without any resize of the row.
		row.minimum_size_changed.connect(func() -> void: _refresh.call_deferred())
	scroll.resized.connect(_refresh)
	scroll.get_h_scroll_bar().value_changed.connect(func(_v: float) -> void: _refresh())
	# The bar learns its range after the row is laid out.
	scroll.get_h_scroll_bar().changed.connect(_refresh)
	prev.visible = false
	next.visible = false


## Whether the chips are wider than the room of the row (the arrows are shown).
func overflows() -> bool:
	var row: Control = scroll.get_child(0) as Control if scroll.get_child_count() > 0 else null
	if row == null:
		return false
	# The room of the whole row: the arrows shown now give their width back when the chips fit without them (a row
	# laid out narrow first must not keep its arrows once it is wide enough).
	var room: float = scroll.size.x
	var sep: float = float(get_theme_constant(&"separation"))
	for arrow: Button in [prev, next]:
		if arrow.visible:
			room += arrow.size.x + sep
	return row.get_combined_minimum_size().x > room + 1.0


func _refresh() -> void:
	var more: bool = overflows()
	if prev.visible != more:
		prev.visible = more
		next.visible = more
		# Shown / hidden while this row was being laid out (a resize signal): lay it out again afterwards, else the
		# scroll box keeps the room of the arrows.
		queue_sort.call_deferred()
	if not more:
		return
	var bar: HScrollBar = scroll.get_h_scroll_bar()
	prev.disabled = bar.value <= bar.min_value + 0.5
	next.disabled = bar.value + bar.page >= bar.max_value - 0.5


func _page(direction: float) -> void:
	scroll.scroll_horizontal += int(direction * scroll.size.x * PAGE_SHARE)
	_refresh()
