class_name SparkMark
extends Control
## The EduWorld logo mark: a Knowledge Spark (Tia Sáng, WORLD_DESIGN §1) drawn as an outlined four-point star in
## `UiTokens.COMPONENT_LOGO_*` colours, sized to the wordmark (`COMPONENT_LOGO_FONT_SIZE`). Vector, no image asset.

## Inner radius of the star relative to its outer radius.
const INNER_RATIO: float = 0.42


## Points of a four-point star centred on `center` (shared with SkyBackdrop's floating sparks).
static func star_points(center: Vector2, outer: float, inner: float) -> PackedVector2Array:
	var points: PackedVector2Array = []
	for i: int in 8:
		var angle: float = -PI / 2.0 + PI / 4.0 * float(i)
		var radius: float = outer if i % 2 == 0 else inner
		points.append(center + Vector2(cos(angle), sin(angle)) * radius)
	return points


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(UiTokens.COMPONENT_LOGO_FONT_SIZE, UiTokens.COMPONENT_LOGO_FONT_SIZE)
	resized.connect(queue_redraw)


func _draw() -> void:
	# The outline grows with the mark (app icon) and keeps the token width at the wordmark size.
	var grow: float = maxf(1.0, minf(size.x, size.y) / float(UiTokens.COMPONENT_LOGO_FONT_SIZE))
	var outline: float = float(UiTokens.COMPONENT_MASCOT_OUTLINE_WIDTH) * grow
	var outer: float = minf(size.x, size.y) / 2.0 - outline
	if outer <= 0.0:
		return
	var center: Vector2 = size / 2.0
	var points: PackedVector2Array = star_points(center, outer, outer * INNER_RATIO)
	draw_colored_polygon(points, UiTokens.COMPONENT_LOGO_SPARK)
	var closed: PackedVector2Array = points.duplicate()
	closed.append(points[0])
	draw_polyline(closed, UiTokens.COMPONENT_LOGO_SPARK_OUTLINE, outline, true)
	draw_circle(center + Vector2(-outer, -outer) * 0.12, outer * 0.14, UiTokens.COMPONENT_LOGO_SPARK_CORE)
