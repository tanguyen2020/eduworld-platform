class_name ChatSheet
extends Control
## Modal sheet of the chat panel (F119 US3/US4): the message menu, the report dialog, "Ai đã xem", the forward
## targets, the players of a match / party and the recall confirmation. F114 style like PartyDialog: nested scrim, a
## tier 1 `FormCard` frame, a blue `WindowTitleBar` (yellow `ConfirmTitleBar` for a confirmation), a body that scrolls
## when it does not fit a phone in landscape, and footer buttons with an icon beside their label. The safe choice
## ("Đóng" / "Hủy") is focused; `Esc` / Back = that choice. Red only for leaving / ending (never used here). UI only:
## the owner acts on `chosen(action)`.

signal chosen(action: StringName)

const CANCEL: StringName = &"cancel"
## Largest share of the view the sheet may take (it never covers the whole screen).
const MAX_HEIGHT_SHARE: float = 0.92
const MAX_WIDTH_SHARE: float = 0.9

var _window: PanelContainer = null
var _bar: PanelContainer = null
var _title: Label = null
var _scroll: ScrollContainer = null
var _body: VBoxContainer = null
var _footer: HFlowContainer = null
var _safe: Button = null


func _init() -> void:
	name = "ChatSheet"
	# Covers the whole view whatever the size of its host (the chat dock is a small part of a screen).
	top_level = true
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


## Clears and opens the sheet with `title` (already translated or a key); `confirm` picks the yellow title bar.
func open_sheet(title: String, confirm: bool = false) -> VBoxContainer:
	UiBuild.clear(_body)
	UiBuild.clear(_footer)
	_safe = null
	_title.text = title
	_bar.theme_type_variation = &"ConfirmTitleBar" if confirm else &"WindowTitleBar"
	_title.theme_type_variation = &"ConfirmTitleLabel" if confirm else &"WindowTitleLabel"
	show()
	_fit()
	UiMotion.pop_in(_window, Settings.reduce_motion)
	return _body


## A footer button; the one with CANCEL is the safe, focused choice.
func add_button(action: StringName, label_key: String, icon_name: String, variation: StringName = &"SecondaryButton") -> Button:
	var b: Button = UiBuild.button(label_key, icon_name, variation)
	b.name = String(action).capitalize().replace(" ", "")
	b.pressed.connect(_on_pressed.bind(action))
	_footer.add_child(b)
	if action == CANCEL:
		_safe = b
		_safe.grab_focus.call_deferred(true)
	UiBuild.size_buttons(_footer)
	return b


func body() -> VBoxContainer:
	return _body


func footer_button(action: StringName) -> Button:
	for b: Node in _footer.get_children():
		if b.name == String(action).capitalize().replace(" ", ""):
			return b as Button
	return null


func title_text() -> String:
	return _title.text


func dismiss() -> void:
	hide()
	UiBuild.clear(_body)


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_on_pressed(CANCEL)


func _on_pressed(action: StringName) -> void:
	hide()
	chosen.emit(action)


func _notification(what: int) -> void:
	if what == NOTIFICATION_RESIZED and visible:
		_fit()


## Width ≈ a dialog (660 px) but never wider than the view; the body scrolls past 92% of the view height.
func _fit() -> void:
	var view: Vector2 = get_viewport_rect().size if is_inside_tree() else size
	if is_inside_tree():
		global_position = Vector2.ZERO
		size = view
	_window.custom_minimum_size.x = minf(float(UiTokens.SIZE_LAYOUT_660), view.x * MAX_WIDTH_SHARE)
	var room: float = view.y * MAX_HEIGHT_SHARE - _bar.get_combined_minimum_size().y - _footer.get_combined_minimum_size().y \
			- float(UiTokens.SIZE_LAYOUT_120) * 0.5
	_scroll.custom_minimum_size.y = clampf(_body.get_combined_minimum_size().y, 0.0, maxf(room, UiBuild.target()))


func _build() -> void:
	add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(center)
	_window = UiBuild.panel(&"FormCard")
	_window.name = "Window"
	center.add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_window.add_child(box)
	_bar = UiBuild.panel(&"WindowTitleBar")
	_bar.name = "TitleBar"
	box.add_child(_bar)
	_title = UiBuild.data_label("", &"WindowTitleLabel")
	_title.name = "Title"
	_title.uppercase = true
	_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_bar.add_child(_title)
	_scroll = ScrollContainer.new()
	_scroll.name = "Scroll"
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	box.add_child(_scroll)
	_body = UiBuild.vbox(&"FormBox")
	_body.name = "Body"
	UiBuild.expand(_body)
	_body.resized.connect(func() -> void: if visible: _fit())
	_scroll.add_child(_body)
	_footer = HFlowContainer.new()
	_footer.name = "Footer"
	_footer.alignment = FlowContainer.ALIGNMENT_CENTER
	box.add_child(_footer)
