class_name PartyDialog
extends Control
## Small dialog over the Party screen (F102-AC-04, AC-12; F114 confirmation style): the nested scrim, a tier 1 frame
## with the yellow `ConfirmTitleBar`, the question in words and buttons with an icon beside their label. The safe
## choice ("Hủy" / "Để sau", a neutral secondary button) has the focus; red `DangerButton` only for leaving / ending
## (leave, disband, remove a member — F114, never for "Hủy"). `Esc` / Back = the safe choice. Extra controls (the
## chips of the party settings) go into `content()`. UI only: the owner acts on `chosen(action)`.

signal chosen(action: StringName)

const CANCEL: StringName = &"cancel"

var _window: PanelContainer = null
var _title: Label = null
var _message: Label = null
var _detail: Label = null
var _content: VBoxContainer = null
var _buttons: HFlowContainer = null
var _safe: Button = null


func _init() -> void:
	name = "PartyDialog"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


## Shows the dialog: `title_key`, `message` (already formatted text), optional `detail`, then `buttons` =
## `[[action, label_key, icon, variation], …]`; the button whose action is CANCEL is the safe focused choice.
## `keep_content`: the caller filled `content()` just before (settings chips); otherwise it is cleared.
func present(title_key: String, message: String, buttons: Array, detail: String = "", keep_content: bool = false) -> void:
	if not keep_content:
		clear_content()
	_title.text = title_key
	_message.text = message
	_message.visible = not message.is_empty()
	_detail.text = detail
	_detail.visible = not detail.is_empty()
	UiBuild.clear(_buttons)
	_safe = null
	for spec: Array in buttons:
		var action: StringName = spec[0] as StringName
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), spec[3] as StringName)
		b.name = String(action).capitalize().replace(" ", "")
		b.pressed.connect(_on_pressed.bind(action))
		_buttons.add_child(b)
		if action == CANCEL:
			_safe = b
	UiBuild.size_buttons(self)
	show()
	UiMotion.pop_in(_window, Settings.reduce_motion)
	if _safe != null:
		_safe.grab_focus.call_deferred(true)


## Container for extra controls between the message and the buttons (cleared by `clear_content`).
func content() -> VBoxContainer:
	return _content


func clear_content() -> void:
	UiBuild.clear(_content)


func button(action: StringName) -> Button:
	for b: Node in _buttons.get_children():
		if b.name == String(action).capitalize().replace(" ", ""):
			return b as Button
	return null


func safe_button() -> Button:
	return _safe


func message_text() -> String:
	return _message.text


func dismiss() -> void:
	hide()
	clear_content()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_on_pressed(CANCEL)


## Hides first, then reports: the content (settings chips) stays readable for the handler, which may present again.
func _on_pressed(action: StringName) -> void:
	hide()
	chosen.emit(action)


func _build() -> void:
	add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(center)
	_window = UiBuild.panel(&"FormCard")
	_window.name = "Window"
	_window.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_660)
	center.add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_window.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"ConfirmTitleBar")
	bar.name = "TitleBar"
	box.add_child(bar)
	_title = UiBuild.label("", &"ConfirmTitleLabel", false)
	_title.name = "Title"
	_title.uppercase = true
	_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	bar.add_child(_title)
	_message = UiBuild.data_label("", &"")
	_message.name = "Message"
	_message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(_message)
	_detail = UiBuild.data_label("", &"MutedLabel")
	_detail.name = "Detail"
	_detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(_detail)
	_content = UiBuild.vbox(&"FormBox")
	_content.name = "Content"
	box.add_child(_content)
	_buttons = HFlowContainer.new()
	_buttons.name = "Buttons"
	_buttons.alignment = FlowContainer.ALIGNMENT_CENTER
	box.add_child(_buttons)
