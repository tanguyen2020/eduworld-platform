class_name ColorDot
extends Control
## Round colour swatch inside an S02 / wardrobe option button (F097): a disc of the option's colour with an ink
## outline; when its parent button is selected a white ring and a tick are drawn on it (shape, not only colour —
## NFR-A11Y-001). The colour is a design token (skin tone / hair colour, `ItemVisual.appearance_color`). Display only.

var color: Color = Color.WHITE


func _init(fill: Color = Color.WHITE) -> void:
	color = fill
	name = "Swatch"
	mouse_filter = Control.MOUSE_FILTER_IGNORE


func _ready() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	var inset: float = float(UiTokens.SIZE_SPACE_8)
	offset_left = inset
	offset_top = inset
	offset_right = -inset
	offset_bottom = -inset
	var parent: Control = get_parent() as Control
	if parent != null:
		parent.draw.connect(queue_redraw)


func is_selected() -> bool:
	var parent: BaseButton = get_parent() as BaseButton
	return parent != null and parent.toggle_mode and parent.button_pressed


func _draw() -> void:
	var r: float = minf(size.x, size.y) / 2.0
	if r <= 0.0:
		return
	var c: Vector2 = size / 2.0
	var line: float = maxf(float(UiTokens.COMPONENT_BUTTON_BORDER_WIDTH), r * 0.08)
	draw_circle(c, r, UiTokens.COMPONENT_MASCOT_OUTLINE)
	draw_circle(c, r - line, color)
	if not is_selected():
		return
	draw_arc(c, r - line * 2.0, 0.0, TAU, 32, UiTokens.COMPONENT_ANSWER_TILE_TEXT_SELECTED, line, true)
	var tick: PackedVector2Array = PackedVector2Array([c + Vector2(-0.4, 0.0) * r, c + Vector2(-0.1, 0.3) * r,
			c + Vector2(0.42, -0.3) * r])
	draw_polyline(tick, UiTokens.COMPONENT_MASCOT_OUTLINE, line * 2.4, true)
	draw_polyline(tick, UiTokens.COMPONENT_ANSWER_TILE_TEXT_SELECTED, line * 1.2, true)
