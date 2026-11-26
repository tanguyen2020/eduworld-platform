class_name ProgressTrail
extends Control
## Question trail of the activity player header (F097, STUDENT_GAME_UX_LEARNING §3.3, §3.7): one mark per question —
## a filled star for a question already passed, a large ring for the current one, a small hollow dot for the ones to
## come — joined by a line. Different shapes, not only colours (NFR-A11Y-001). More questions than fit as marks
## become a rounded bar with the current position. Drawn with `UiTokens.COMPONENT_PROGRESS_TRAIL_*`; display only:
## the position comes from the renderer (`progress()`), nothing is counted here.

## Above this many questions the trail is a bar (marks would be smaller than a finger).
const MAX_MARKS: int = 15
const STAR_POINTS: int = 5

var current: int = 0
var total: int = 0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(0.0, float(UiTokens.COMPONENT_ANSWER_TILE_BADGE_SIZE))
	resized.connect(queue_redraw)


## `position` is 1-based (the current question), `count` the number of questions.
func set_progress(position: int, count: int) -> void:
	current = clampi(position, 0, maxi(count, 0))
	total = maxi(count, 0)
	# One question needs no trail ("1/1" says it all).
	visible = total > 1
	queue_redraw()


## "done" / "current" / "todo" for question `index` (1-based) — what the mark shows.
func mark_of(index: int) -> String:
	if index < current:
		return "done"
	return "current" if index == current else "todo"


func uses_bar() -> bool:
	return total > MAX_MARKS


func _draw() -> void:
	if total <= 0 or size.x <= 0.0:
		return
	var h: float = size.y
	var outline: float = maxf(float(UiTokens.COMPONENT_FOCUS_RING_WIDTH) * 0.75, 2.0)
	if uses_bar():
		_draw_bar(h, outline)
		return
	var r: float = h * 0.42
	var step: float = minf((size.x - 2.0 * r) / maxf(float(total - 1), 1.0), r * 3.2)
	var width: float = step * float(total - 1)
	var start: Vector2 = Vector2((size.x - width) / 2.0, h / 2.0)
	draw_line(start, start + Vector2(width, 0.0), UiTokens.COMPONENT_PROGRESS_TRAIL_TRACK, outline * 1.5, true)
	for i: int in total:
		var c: Vector2 = start + Vector2(step * float(i), 0.0)
		match mark_of(i + 1):
			"done":
				_draw_star(c, r * 0.95, outline)
			"current":
				draw_circle(c, r, UiTokens.COMPONENT_PROGRESS_TRAIL_CURRENT)
				draw_circle(c, r * 0.5, UiTokens.COMPONENT_PROGRESS_TRAIL_TODO)
			_:
				draw_circle(c, r * 0.42 + outline, UiTokens.COMPONENT_PROGRESS_TRAIL_TRACK)
				draw_circle(c, r * 0.42, UiTokens.COMPONENT_PROGRESS_TRAIL_TODO)


func _draw_bar(h: float, outline: float) -> void:
	var track: Rect2 = Rect2(Vector2(0.0, h * 0.3), Vector2(size.x, h * 0.4))
	var box: StyleBoxFlat = StyleBoxFlat.new()
	box.set_corner_radius_all(int(track.size.y / 2.0))
	box.bg_color = UiTokens.COMPONENT_PROGRESS_TRAIL_TRACK
	draw_style_box(box, track)
	var fraction: float = float(current) / float(total)
	if fraction > 0.0:
		var fill: StyleBoxFlat = box.duplicate() as StyleBoxFlat
		fill.bg_color = UiTokens.COMPONENT_PROGRESS_TRAIL_CURRENT
		draw_style_box(fill, Rect2(track.position, Vector2(maxf(track.size.x * fraction, track.size.y), track.size.y)))
	_draw_star(Vector2(clampf(size.x * fraction, h * 0.4, size.x - h * 0.4), h / 2.0), h * 0.4, outline)


func _draw_star(c: Vector2, r: float, outline: float) -> void:
	var points: PackedVector2Array = PackedVector2Array()
	for i: int in STAR_POINTS * 2:
		var radius: float = r if i % 2 == 0 else r * 0.48
		var angle: float = -PI / 2.0 + PI * float(i) / float(STAR_POINTS)
		points.append(c + Vector2(cos(angle), sin(angle)) * radius)
	draw_colored_polygon(points, UiTokens.COMPONENT_PROGRESS_TRAIL_DONE)
	points.append(points[0])
	draw_polyline(points, UiTokens.COMPONENT_PROGRESS_TRAIL_OUTLINE, outline, true)
