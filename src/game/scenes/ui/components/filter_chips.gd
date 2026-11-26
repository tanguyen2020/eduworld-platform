class_name FilterChips
extends HBoxContainer
## Single row of filter chips (S08 `FilterChips`, S09 categories, S06 types; a long row sits in a horizontal
## `ScrollContainer` via `UiBuild.chip_scroller` — one line, swipe to see more): one choice at a time, the chosen
## chip filled with a tick icon (shape, not only colour — F097, NFR-A11Y-001), every chip at the grade-band touch target. The codes are
## contract enum values; the labels are translation keys. Emits `filter_changed(code)` ("" = all).

signal filter_changed(code: String)

var selected: String = ""
var _group: ButtonGroup = ButtonGroup.new()
var _icons: Dictionary = {}


## `options`: `[{code, key, icon?}]` in display order.
func set_options(options: Array[Dictionary], chosen: String = "") -> void:
	UiBuild.clear(self)
	_icons.clear()
	_group = ButtonGroup.new()
	for o: Dictionary in options:
		var b: Button = UiBuild.chip(String(o["key"]), _group)
		var code: String = String(o["code"])
		b.name = "Chip_%s" % (code if not code.is_empty() else "all")
		b.set_meta(&"code", code)
		if o.has("icon") and not String(o["icon"]).is_empty():
			_icons[b] = UiBuild.icon(String(o["icon"]))
		b.pressed.connect(_on_pressed.bind(code))
		add_child(b)
	select(chosen, false)


func select(code: String, notify: bool = true) -> void:
	selected = code
	for node: Node in get_children():
		var b: Button = node as Button
		b.set_pressed_no_signal(String(b.get_meta(&"code")) == code)
	UiBuild.mark_chips(get_children(), _icons)
	if notify:
		filter_changed.emit(code)


func chip(code: String) -> Button:
	for node: Node in get_children():
		if String(node.get_meta(&"code")) == code:
			return node as Button
	return null


func _on_pressed(code: String) -> void:
	select(code)
