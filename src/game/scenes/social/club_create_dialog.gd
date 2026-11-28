class_name ClubCreateDialog
extends Control
## "Tạo club mới" (F111-AC-22, AC-23; SOCIAL_SYSTEM §6.1, ADR-0041, AGENTS R15): the type (only "Nhóm sở thích" / "Đội thi
## đấu" for a student), the name (2–64 characters — typing past 64 is blocked), the description (≤ 500 with a counter —
## typing past it is blocked) and an icon of the library (picture chips). "Tạo club" stays off until the form is valid
## (ClubForm). A moderation refusal (`451`) shows a general sentence — never the word that matched — and keeps the form
## to fix. When the school asks to approve clubs the form says so. UI only: `submitted(form)` (AGENTS R1).

signal submitted(form: Dictionary)
signal closed()

var club_type: String = ClubModel.STUDENT_TYPES[0]
var icon_id: String = "club_icon_star"
var _ui: Dictionary = {}
var _types: Dictionary = {}
var _icons: Dictionary = {}
var _name: LineEdit = null
var _description: TextEdit = null
var _counter: Label = null
var _error: PanelContainer = null
var _error_label: Label = null
var _note: Label = null
var _submit: Button = null
var _busy: bool = false


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()


func open_form(needs_approval: bool) -> void:
	_name.text = ""
	_description.text = ""
	club_type = ClubModel.STUDENT_TYPES[0]
	icon_id = "club_icon_star"
	_note.visible = needs_approval
	_error.visible = false
	_busy = false
	show()
	_refresh()
	_name.grab_focus.call_deferred()


func close() -> void:
	hide()
	closed.emit()


func set_busy(busy: bool) -> void:
	_busy = busy
	_refresh()


## A friendly sentence under the form (the form stays).
func show_error(key: String) -> void:
	_error_label.text = key
	_error.visible = not key.is_empty()


# ---------- test hooks ----------

func type_ids() -> PackedStringArray:
	return PackedStringArray(_types.keys())


func icon_ids() -> PackedStringArray:
	return PackedStringArray(_icons.keys())


func choose_type(id: String) -> void:
	if _types.has(id):
		club_type = id
		_refresh()


func set_name_text(text: String) -> void:
	_name.text = text.left(ClubModel.NAME_MAX)
	_refresh()


func name_text() -> String:
	return _name.text


func set_description_text(text: String) -> void:
	_description.text = text
	_on_description_changed()


func description_text() -> String:
	return _description.text


func counter_text() -> String:
	return _counter.text


func error_text() -> String:
	return tr(_error_label.text) if _error.visible else ""


func submit_button() -> Button:
	return _submit


func form() -> Dictionary:
	return {"club_type": club_type, "name": _name.text, "description": _description.text, "icon_id": icon_id}


# ---------- internals ----------

func _refresh() -> void:
	for id: String in _types.keys():
		(_types[id] as Button).set_pressed_no_signal(id == club_type)
	for id: String in _icons.keys():
		(_icons[id] as Button).set_pressed_no_signal(id == icon_id)
	UiBuild.mark_chips(_types.values())
	var problem: String = ClubForm.validate(form())
	_submit.disabled = _busy or not problem.is_empty()
	# Only a name that was started and is too short is pointed out (an empty form says nothing yet).
	if problem == "ui.club.form.name_short" and not _name.text.strip_edges().is_empty():
		show_error(problem)
	elif _error_label.text == "ui.club.form.name_short":
		show_error("")
	_counter.text = tr("ui.club.form.counter").format({"n": LocaleFormat.number(_description.text.length()),
		"max": LocaleFormat.number(ClubModel.DESCRIPTION_MAX)})


func _on_description_changed() -> void:
	if _description.text.length() > ClubModel.DESCRIPTION_MAX:
		var caret: int = _description.get_caret_column()
		_description.text = _description.text.left(ClubModel.DESCRIPTION_MAX)
		_description.set_caret_column(mini(caret, ClubModel.DESCRIPTION_MAX))
	_refresh()


func _on_submit() -> void:
	if ClubForm.validate(form()).is_empty() and not _busy:
		submitted.emit(form())


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _build() -> void:
	_ui = UiBuild.window(self, "ui.club.form.title", "plus")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box)
	scroll.add_child(box)
	_note = UiBuild.keep_words(UiBuild.label("ui.club.form.approval_note", &"MutedLabel"))
	_note.name = "ApprovalNote"
	box.add_child(_note)
	box.add_child(UiBuild.label("ui.club.form.type", &"HeadingLabel"))
	var types: HFlowContainer = HFlowContainer.new()
	types.name = "Types"
	box.add_child(types)
	var tg: ButtonGroup = ButtonGroup.new()
	for t: String in ClubModel.STUDENT_TYPES:
		var b: Button = UiBuild.chip("ui.club.type.%s" % t, tg)
		b.name = "Type_" + t
		b.pressed.connect(func() -> void:
			club_type = t
			_refresh())
		types.add_child(b)
		_types[t] = b
	box.add_child(UiBuild.label("ui.club.form.name", &"HeadingLabel"))
	_name = LineEdit.new()
	_name.name = "Name"
	_name.max_length = ClubModel.NAME_MAX
	_name.placeholder_text = "ui.club.form.name_hint"
	_name.custom_minimum_size.y = UiBuild.target()
	_name.text_changed.connect(func(_t: String) -> void: _refresh())
	UiBuild.expand(_name)
	box.add_child(_name)
	var desc_head: HBoxContainer = UiBuild.hbox()
	box.add_child(desc_head)
	var dl: Label = UiBuild.label("ui.club.form.description", &"HeadingLabel")
	UiBuild.expand(dl)
	desc_head.add_child(dl)
	_counter = UiBuild.data_label("", &"MutedLabel", false)
	_counter.name = "Counter"
	desc_head.add_child(_counter)
	_description = TextEdit.new()
	_description.name = "Description"
	_description.wrap_mode = TextEdit.LINE_WRAPPING_BOUNDARY
	_description.custom_minimum_size.y = UiBuild.target() * 1.6
	_description.text_changed.connect(_on_description_changed)
	UiBuild.expand(_description)
	box.add_child(_description)
	box.add_child(UiBuild.label("ui.club.form.icon", &"HeadingLabel"))
	var icons: HFlowContainer = HFlowContainer.new()
	icons.name = "Icons"
	box.add_child(icons)
	var ig: ButtonGroup = ButtonGroup.new()
	for id: String in ClubModel.ICONS.keys():
		var ib: Button = UiBuild.button("", ClubModel.icon_for(id), &"ChoiceChip")
		ib.name = "Icon_" + id
		ib.toggle_mode = true
		ib.button_group = ig
		ib.tooltip_text = "ui.club.form.icon"
		ib.pressed.connect(func() -> void:
			icon_id = id
			_refresh())
		icons.add_child(ib)
		_icons[id] = ib
	_error = UiBuild.panel(&"InlineErrorPanel")
	_error.name = "Error"
	_error.visible = false
	_error_label = UiBuild.keep_words(UiBuild.label("", &"InlineErrorLabel"))
	_error.add_child(_error_label)
	body.add_child(_error)
	_submit = UiBuild.button("ui.club.form.submit", "check")
	_submit.name = "Submit"
	_submit.size_flags_horizontal = Control.SIZE_SHRINK_END
	_submit.pressed.connect(_on_submit)
	body.add_child(_submit)
	UiBuild.size_buttons(self)
