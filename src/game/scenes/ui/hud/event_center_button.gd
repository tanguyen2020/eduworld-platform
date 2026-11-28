class_name EventCenterButton
extends Button
## The calendar button of the HUD side bar (F111-AC-09, AC-29; STUDENT_GAME_UX_SOCIAL §9): opens the Event center
## (`ui_requested("event_center")`). A small STATIC dot (no animation, no number, no "chỉ còn…") tells that something new
## is there — on when LiveEventStore has an event the student has not seen, gone once the Event center was opened.
## Styled by the generated Theme only (AGENTS R32); UI only (AGENTS R1).

const SCREEN: StringName = &"event_center"

var _dot: Panel = null
var _store: LiveEventStore = null


func _init() -> void:
	name = "EventCenterButton"
	theme_type_variation = &"SecondaryButton"
	icon = UiBuild.icon("calendar")
	text = "ui.events.hud"
	tooltip_text = "ui.events.hud.tooltip"
	autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	size_flags_horizontal = Control.SIZE_SHRINK_END
	_dot = Panel.new()
	_dot.name = "NewDot"
	_dot.theme_type_variation = &"CountBadge"
	_dot.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_dot.visible = false
	_dot.tooltip_text = "ui.events.new_dot"
	add_child(_dot)
	pressed.connect(func() -> void: EventBus.ui_requested.emit(SCREEN, {}))
	resized.connect(_place_dot)


func bind_store(store: LiveEventStore) -> void:
	if _store != null and _store.new_content_changed.is_connected(_on_new):
		_store.new_content_changed.disconnect(_on_new)
	_store = store
	if _store != null:
		_store.new_content_changed.connect(_on_new)
	_on_new(_store != null and _store.has_new)


func dot_visible() -> bool:
	return _dot.visible


func _on_new(has_new: bool) -> void:
	_dot.visible = has_new
	accessibility_description = tr("ui.events.new_dot") if has_new else ""
	_place_dot()


## The dot sits on the top-right corner of the button (a third of the icon size).
func _place_dot() -> void:
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE) * 0.4
	_dot.size = Vector2(side, side)
	_dot.position = Vector2(size.x - side * 1.2, side * 0.2)
