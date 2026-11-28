class_name EventCard
extends PanelContainer
## One live-ops event of the "Sự kiện" tab (F111-AC-09, AC-12; STUDENT_GAME_UX_SOCIAL §9, EVENT_SEASON_SYSTEM §9): the
## event icon on its card (no banner download — a themed thumbnail, NFR-COMP-001), the name in the UI language (the
## Vietnamese name when a school event has no English one), the tagline and the time left in WHOLE DAYS with a clock icon
## ("Còn 5 ngày", "Hôm nay là ngày cuối", "Sắp diễn ra", "Sự kiện tạm dừng", "Đã kết thúc") — never hours, minutes or
## seconds, nothing blinking (AGENTS R19). "Xem" (or a tap on the card) opens the detail. UI only (AGENTS R1).

signal opened(event_id: String)

var event_id: String = ""
var view_button: Button = null
var _icon: TextureRect = null
var _name: Label = null
var _tagline: Label = null
var _status_icon: TextureRect = null
var _status: Label = null
var _points: Label = null


func _init() -> void:
	name = "EventCard"
	theme_type_variation = &"QuestCard"
	mouse_filter = Control.MOUSE_FILTER_STOP
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.name = "Row"
	add_child(row)
	_icon = UiBuild.icon_rect("star", 1.5)
	_icon.name = "EventIcon"
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(UiBuild.icon_badge(_icon))
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	row.add_child(texts)
	_name = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_name.name = "EventName"
	texts.add_child(_name)
	_tagline = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_tagline.name = "Tagline"
	texts.add_child(_tagline)
	var status: HFlowContainer = HFlowContainer.new()
	status.name = "StatusRow"
	texts.add_child(status)
	var tag: HBoxContainer = UiBuild.hbox()
	status.add_child(tag)
	_status_icon = UiBuild.icon_rect("clock", 0.7)
	_status_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	tag.add_child(_status_icon)
	_status = UiBuild.data_label("", &"InstructionLabel", false)
	_status.name = "TimeLeft"
	tag.add_child(_status)
	_points = UiBuild.data_label("", &"SectionTag", false)
	_points.name = "Points"
	_points.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	status.add_child(_points)
	view_button = UiBuild.button("ui.events.view", "arrow_right", &"SecondaryButton")
	view_button.name = "View"
	view_button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	view_button.pressed.connect(func() -> void: opened.emit(event_id))
	row.add_child(view_button)
	gui_input.connect(_on_gui_input)


func show_event(e: LiveEventModel, now_unix: int) -> void:
	event_id = e.event_id
	_icon.texture = UiBuild.icon(e.icon_name())
	_name.text = e.display_name()
	_tagline.text = e.display_tagline()
	_tagline.visible = not _tagline.text.is_empty()
	_status.text = e.time_left_text(now_unix)
	_status_icon.texture = UiBuild.icon("pause" if e.is_suspended() else ("check" if e.is_ended() else "clock"))
	_points.text = tr("ui.events.points").format({"points": LocaleFormat.number(e.event_points)})
	_points.visible = e.joined and not e.is_scheduled()
	accessibility_name = "%s, %s" % [_name.text, _status.text]
	UiBuild.fit_words(_name)
	UiBuild.fit_words(_tagline)


## Texts on the card (tests: name, tagline, time left, points — no clock time).
func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Label in [_name, _tagline, _status, _points]:
		if l.visible:
			out.append(l.text)
	return out


func _on_gui_input(event: InputEvent) -> void:
	var tap: bool = (event is InputEventMouseButton and (event as InputEventMouseButton).pressed
			and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT) \
			or (event is InputEventScreenTouch and (event as InputEventScreenTouch).pressed)
	if tap and not event_id.is_empty():
		accept_event()
		opened.emit(event_id)
