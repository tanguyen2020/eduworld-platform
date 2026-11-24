class_name BusySpinner
extends Control
## Small "working…" indicator (S01 "Đang vào…" + spinner): a three-quarter ring that turns while visible, in the
## colour of disabled button text (`UiTokens.COLOR_TEXT_SECONDARY`), `COMPONENT_ICON_SIZE` square. With reduced
## motion (Settings) the ring stays still — the button text already says what is happening.

## One turn per second.
const TURNS_PER_SECOND: float = 1.0

var _angle: float = 0.0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(UiTokens.COMPONENT_ICON_SIZE, UiTokens.COMPONENT_ICON_SIZE)
	visibility_changed.connect(_on_visibility_changed)
	_on_visibility_changed()


func _process(delta: float) -> void:
	if bool(Settings.reduce_motion):
		return
	_angle = fmod(_angle + TAU * TURNS_PER_SECOND * delta, TAU)
	queue_redraw()


func _draw() -> void:
	var width: float = float(UiTokens.COMPONENT_FOCUS_RING_WIDTH)
	var radius: float = minf(size.x, size.y) / 2.0 - width
	if radius <= 0.0:
		return
	draw_arc(size / 2.0, radius, _angle, _angle + TAU * 0.75, 32, UiTokens.COLOR_TEXT_SECONDARY, width, true)


func _on_visibility_changed() -> void:
	set_process(is_visible_in_tree())
