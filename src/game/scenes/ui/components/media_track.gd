class_name MediaTrack
extends Range
## Progress bar of the video and audio players (F099, STUDENT_GAME_UX_LEARNING §5.1, §6.1, DESIGN_SYSTEM §5.1): a
## rounded groove as tall as the band's touch target, the watched part (the student may seek inside it) in a light
## fill, the played part and the thumb in the accent colour, a notch at the start of every chapter / audio segment, and
## one disc per embedded question — a question mark while it waits, a tick once answered (shape, not colour alone).
## Drawn with `UiTokens.COMPONENT_MEDIA_TRACK_*`. Dragging or tapping moves the thumb; releasing emits
## `seek_released(value)` and the renderer applies its seek rules (watched_only, current play). Display only.

## The thumb was released at `ms` (drag end, tap, or arrow key).
signal seek_released(ms: float)

## A keyboard arrow moves the thumb by this many ms.
const KEY_STEP_MS: float = 5000.0

## End of the part already watched / heard (ms).
var watched_value: float = 0.0:
	set(v):
		watched_value = v
		queue_redraw()
## Start of each chapter / segment (ms); a notch is drawn at each one after 0.
var chapters: PackedFloat32Array = PackedFloat32Array():
	set(v):
		chapters = v
		queue_redraw()
## Embedded questions `{at: ms, done: bool}`.
var markers: Array[Dictionary] = []:
	set(v):
		markers = v
		queue_redraw()
## Whether the student may drag / tap the bar (an audio whose play ended is not seekable).
var editable: bool = true:
	set(v):
		editable = v
		queue_redraw()
var _dragging: bool = false
## One rounded box reused by every fill (StyleBox.draw bakes its colour into the canvas commands).
var _box: StyleBoxFlat = StyleBoxFlat.new()


func _init() -> void:
	focus_mode = Control.FOCUS_ALL
	mouse_filter = Control.MOUSE_FILTER_STOP
	step = 1.0
	value_changed.connect(func(_v: float) -> void: queue_redraw())
	changed.connect(queue_redraw)


## As tall as the band's touch target (density.<band>.button.min_height; F125 compact floor on a phone).
func _get_minimum_size() -> Vector2:
	var h: float = UiBuild.target()
	return Vector2(h * 2.0, h)


func _x_of(ms: float) -> float:
	var pad: float = size.y * 0.5
	var span: float = maxf(max_value - min_value, 1.0)
	return pad + (size.x - 2.0 * pad) * clampf((ms - min_value) / span, 0.0, 1.0)


func _ms_of(x: float) -> float:
	var pad: float = size.y * 0.5
	var t: float = clampf((x - pad) / maxf(size.x - 2.0 * pad, 1.0), 0.0, 1.0)
	return min_value + t * (max_value - min_value)


func _draw() -> void:
	var h: float = size.y
	var groove: float = maxf(h * 0.32, 6.0)
	var top: float = (h - groove) / 2.0
	var r: float = groove / 2.0
	var left: float = _x_of(min_value)
	var right: float = _x_of(max_value)
	_bar(left - r, right + r, top, groove, UiTokens.COMPONENT_MEDIA_TRACK_TRACK)
	if watched_value > min_value:
		_bar(left - r, _x_of(watched_value) + r, top, groove, UiTokens.COMPONENT_MEDIA_TRACK_WATCHED)
	if value > min_value:
		_bar(left - r, _x_of(value) + r, top, groove, UiTokens.COMPONENT_MEDIA_TRACK_PLAYED)
	var notch: float = maxf(h * 0.06, 2.0)
	for at: float in chapters:
		if at <= min_value or at >= max_value:
			continue
		var x: float = _x_of(at)
		draw_rect(Rect2(x - notch / 2.0, top - groove * 0.45, notch, groove * 1.9), UiTokens.COMPONENT_MEDIA_TRACK_CHAPTER)
	var thumb: float = h * 0.26
	draw_circle(Vector2(_x_of(value), h / 2.0), thumb, UiTokens.COMPONENT_MEDIA_TRACK_MARKER_DONE_TEXT)
	draw_circle(Vector2(_x_of(value), h / 2.0), thumb * 0.72, UiTokens.COMPONENT_MEDIA_TRACK_PLAYED)
	for m: Dictionary in markers:
		_marker(Vector2(_x_of(float(m.get("at", 0.0))), h / 2.0), h * 0.3, bool(m.get("done", false)))
	if has_focus():
		var ring: StyleBox = get_theme_stylebox(&"focus", &"Button")
		if ring != null:
			ring.draw(get_canvas_item(), Rect2(Vector2.ZERO, size))


func _bar(x0: float, x1: float, top: float, height: float, color: Color) -> void:
	_box.bg_color = color
	_box.set_corner_radius_all(int(height / 2.0))
	draw_style_box(_box, Rect2(x0, top, maxf(x1 - x0, height), height))


## A question disc: "?" while it waits, a tick once answered.
func _marker(c: Vector2, r: float, done: bool) -> void:
	draw_circle(c, r, UiTokens.COMPONENT_MEDIA_TRACK_MARKER_DONE if done else UiTokens.COMPONENT_MEDIA_TRACK_MARKER)
	var w: float = maxf(r * 0.24, 2.0)
	if done:
		draw_polyline(PackedVector2Array([c + Vector2(-0.45, 0.02) * r, c + Vector2(-0.12, 0.35) * r,
				c + Vector2(0.5, -0.32) * r]), UiTokens.COMPONENT_MEDIA_TRACK_MARKER_DONE_TEXT, w, true)
		return
	var font: Font = get_theme_font(&"font", &"Button")
	var font_size: int = int(r * 1.4)
	var text_size: Vector2 = font.get_string_size("?", HORIZONTAL_ALIGNMENT_CENTER, -1, font_size)
	var baseline: float = c.y + (font.get_ascent(font_size) - font.get_descent(font_size)) / 2.0
	draw_string(font, Vector2(c.x - text_size.x / 2.0, baseline), "?", HORIZONTAL_ALIGNMENT_LEFT, -1, font_size,
			UiTokens.COMPONENT_MEDIA_TRACK_MARKER_TEXT)


func _gui_input(event: InputEvent) -> void:
	if not editable:
		return
	if event is InputEventMouseButton and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed:
			_dragging = true
			set_value_no_signal(_ms_of(mb.position.x))
			queue_redraw()
		elif _dragging:
			_dragging = false
			release_at(_ms_of(mb.position.x))
		accept_event()
	elif event is InputEventMouseMotion and _dragging:
		set_value_no_signal(_ms_of((event as InputEventMouseMotion).position.x))
		queue_redraw()
		accept_event()
	elif event.is_action_pressed(&"ui_left") or event.is_action_pressed(&"ui_right"):
		var dir: float = -1.0 if event.is_action_pressed(&"ui_left") else 1.0
		release_at(clampf(value + dir * KEY_STEP_MS, min_value, max_value))
		accept_event()


## Moves the thumb to `ms` and reports it (the renderer may move it back when its rules refuse the seek).
func release_at(ms: float) -> void:
	set_value_no_signal(ms)
	queue_redraw()
	seek_released.emit(ms)
