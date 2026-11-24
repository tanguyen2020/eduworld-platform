class_name SkyBackdrop
extends Control
## Warm sky backdrop of the entry screens (STUDENT_GAME_UX_CORE S01, WORLD_DESIGN §11 art direction): a soft sky
## gradient, the sun, clouds, floating islands of the Knowledge Archipelago, Knowledge Sparks and rolling hills.
## Drawn with vector shapes in `UiTokens.COMPONENT_BACKDROP_*` colours — no image asset, crisp at any window size,
## static (no animation: low-end preset and reduced motion, DESIGN_SYSTEM §5.4). Shapes are laid out in fractions of
## the control size so the picture covers any aspect ratio.

## Points per hill curve; enough for a smooth silhouette at 4K, cheap to redraw on resize.
const HILL_SEGMENTS: int = 48


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	resized.connect(queue_redraw)


func _draw() -> void:
	var w: float = size.x
	var h: float = size.y
	if w <= 0.0 or h <= 0.0:
		return
	var unit: float = minf(w, h)
	_draw_sky(w, h)
	_draw_sun(Vector2(w * 0.86, h * 0.16), unit * 0.07)
	_draw_cloud(Vector2(w * 0.12, h * 0.14), unit * 0.07)
	_draw_cloud(Vector2(w * 0.58, h * 0.09), unit * 0.05)
	_draw_cloud(Vector2(w * 0.9, h * 0.5), unit * 0.055)
	_draw_island(Vector2(w * 0.83, h * 0.66), unit * 0.08)
	_draw_island(Vector2(w * 0.4, h * 0.11), unit * 0.035)
	_draw_island(Vector2(w * 0.95, h * 0.34), unit * 0.04)
	for spark: Vector3 in [Vector3(0.33, 0.12, 0.018), Vector3(0.7, 0.3, 0.014), Vector3(0.08, 0.55, 0.016),
			Vector3(0.95, 0.75, 0.013), Vector3(0.62, 0.5, 0.011)]:
		_draw_spark(Vector2(w * spark.x, h * spark.y), unit * spark.z)
	_draw_hill(w, h, h * 0.8, h * 0.05, 1.3, 0.4, UiTokens.COMPONENT_BACKDROP_HILL_FAR)
	_draw_hill(w, h, h * 0.88, h * 0.045, 2.1, 1.7, UiTokens.COMPONENT_BACKDROP_HILL_NEAR)


func _draw_sky(w: float, h: float) -> void:
	var top: Color = UiTokens.COMPONENT_BACKDROP_SKY_TOP
	var bottom: Color = UiTokens.COMPONENT_BACKDROP_SKY_BOTTOM
	draw_polygon(PackedVector2Array([Vector2.ZERO, Vector2(w, 0.0), Vector2(w, h), Vector2(0.0, h)]),
			PackedColorArray([top, top, bottom, bottom]))


func _draw_sun(center: Vector2, radius: float) -> void:
	draw_circle(center, radius * 1.6, UiTokens.COMPONENT_BACKDROP_SUN_GLOW)
	draw_circle(center, radius, UiTokens.COMPONENT_BACKDROP_SUN)


func _draw_cloud(center: Vector2, radius: float) -> void:
	var color: Color = UiTokens.COMPONENT_BACKDROP_CLOUD
	draw_circle(center + Vector2(-radius * 1.1, radius * 0.25), radius * 0.7, color)
	draw_circle(center + Vector2(radius * 1.1, radius * 0.25), radius * 0.75, color)
	draw_circle(center + Vector2(radius * 0.3, -radius * 0.2), radius, color)
	draw_circle(center + Vector2(-radius * 0.4, 0.0), radius * 0.85, color)
	draw_rect(Rect2(center + Vector2(-radius * 1.1, 0.0), Vector2(radius * 2.2, radius * 0.95)), color)


## A floating island: grassy top, a rock cone below with a lit and a shaded side.
func _draw_island(center: Vector2, radius: float) -> void:
	var tip: Vector2 = center + Vector2(radius * 0.1, radius * 1.5)
	var left: Vector2 = center + Vector2(-radius, 0.0)
	var right: Vector2 = center + Vector2(radius, 0.0)
	draw_colored_polygon(PackedVector2Array([left, center, tip]), UiTokens.COMPONENT_BACKDROP_ISLAND_ROCK_LIGHT)
	draw_colored_polygon(PackedVector2Array([center, right, tip]), UiTokens.COMPONENT_BACKDROP_ISLAND_ROCK)
	_draw_ellipse(center, Vector2(radius * 1.08, radius * 0.32), UiTokens.COMPONENT_BACKDROP_ISLAND_GRASS)


## A Knowledge Spark (Tia Sáng): a small four-point star.
func _draw_spark(center: Vector2, radius: float) -> void:
	draw_colored_polygon(SparkMark.star_points(center, radius, radius * 0.38), UiTokens.COMPONENT_BACKDROP_SPARK)


func _draw_hill(w: float, h: float, base_y: float, amplitude: float, waves: float, phase: float, color: Color) -> void:
	var points: PackedVector2Array = [Vector2(0.0, h)]
	for i: int in HILL_SEGMENTS + 1:
		var t: float = float(i) / float(HILL_SEGMENTS)
		points.append(Vector2(w * t, base_y - amplitude * sin(TAU * waves * t + phase)))
	points.append(Vector2(w, h))
	draw_colored_polygon(points, color)


func _draw_ellipse(center: Vector2, radii: Vector2, color: Color) -> void:
	var points: PackedVector2Array = []
	for i: int in HILL_SEGMENTS:
		var a: float = TAU * float(i) / float(HILL_SEGMENTS)
		points.append(center + Vector2(cos(a) * radii.x, sin(a) * radii.y))
	draw_colored_polygon(points, color)
