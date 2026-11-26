class_name CloudyMascot
extends Control
## Mây (Cloudy, `npc.cloudy`, WORLD_DESIGN §3), the guide, waving hello beside a form (S01 `NpcHint`). Drawn with
## vector shapes in `UiTokens.COMPONENT_MASCOT_*` colours inside a square of `COMPONENT_MASCOT_SIZE`; with
## `bubble_tail` it also draws the tail of the SpeechBubble placed right above it. Static (no animation).
## F097: `compact` makes it the coach beside the activity feedback (`COMPONENT_COACH_SIZE`) with `side_tail` pointing
## at the bubble on its right, and `mood` changes the face — happy after a right answer, encouraging after a wrong
## one (a kind smile, never sad: NFR-SAFE-003).

enum Mood { NEUTRAL, HAPPY, ENCOURAGE }

## Draws the speech-bubble tail pointing from the bubble above down to Mây.
@export var bubble_tail: bool = true
## Coach size (F097) instead of the large illustration size.
@export var compact: bool = false
## Draws the tail of a SpeechBubble placed right of Mây (coach layout).
@export var side_tail: bool = false
@export var mood: Mood = Mood.NEUTRAL:
	set(value):
		mood = value
		queue_redraw()


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	var side: float = float(UiTokens.COMPONENT_COACH_SIZE if compact else UiTokens.COMPONENT_MASCOT_SIZE)
	custom_minimum_size = Vector2(side, side)
	resized.connect(queue_redraw)


func _draw() -> void:
	var side: float = minf(size.x, size.y)
	if side <= 0.0:
		return
	var origin: Vector2 = Vector2((size.x - side) / 2.0, size.y - side)
	var outline: float = float(UiTokens.COMPONENT_MASCOT_OUTLINE_WIDTH) * side / float(UiTokens.COMPONENT_MASCOT_SIZE)
	if bubble_tail:
		_draw_tail(origin, side)
	if side_tail:
		_draw_side_tail(origin, side)
	# Body: overlapping puffs, first enlarged in the outline colour, then filled.
	var puffs: Array[Vector3] = [
		Vector3(0.5, 0.56, 0.25), Vector3(0.27, 0.64, 0.17), Vector3(0.73, 0.64, 0.17),
		Vector3(0.38, 0.43, 0.16), Vector3(0.62, 0.4, 0.19), Vector3(0.36, 0.74, 0.12), Vector3(0.64, 0.74, 0.12),
		Vector3(0.87, 0.42, 0.075),
	]
	for puff: Vector3 in puffs:
		draw_circle(origin + Vector2(puff.x, puff.y) * side, puff.z * side + outline, UiTokens.COMPONENT_MASCOT_OUTLINE)
	for puff: Vector3 in puffs:
		draw_circle(origin + Vector2(puff.x, puff.y) * side, puff.z * side, UiTokens.COMPONENT_MASCOT_BODY)
	# Face.
	for x: float in [0.42, 0.58]:
		if mood == Mood.HAPPY:
			# Smiling eyes (^ ^).
			draw_arc(origin + Vector2(x, 0.555) * side, 0.032 * side, PI * 1.1, PI * 1.9, 10,
					UiTokens.COMPONENT_MASCOT_EYE, outline * 1.2, true)
		else:
			draw_circle(origin + Vector2(x, 0.54) * side, 0.036 * side, UiTokens.COMPONENT_MASCOT_EYE)
			draw_circle(origin + Vector2(x + 0.012, 0.528) * side, 0.012 * side, UiTokens.COMPONENT_MASCOT_BODY)
	for x: float in [0.34, 0.66]:
		draw_circle(origin + Vector2(x, 0.62) * side, 0.042 * side, UiTokens.COMPONENT_MASCOT_CHEEK)
	var smile: float = 0.08 if mood == Mood.HAPPY else (0.045 if mood == Mood.ENCOURAGE else 0.06)
	draw_arc(origin + Vector2(0.5, 0.6) * side, smile * side, PI * 0.15, PI * 0.85, 16,
			UiTokens.COMPONENT_MASCOT_EYE, outline, true)
	if mood == Mood.ENCOURAGE:
		# Raised, friendly eyebrows: "let's look again together".
		for x: float in [0.42, 0.58]:
			draw_line(origin + Vector2(x - 0.03, 0.47) * side, origin + Vector2(x + 0.03, 0.465) * side,
					UiTokens.COMPONENT_MASCOT_EYE, outline * 0.8, true)


## Tail of the bubble right above: a triangle in the bubble colours that overlaps the bubble's bottom border.
func _draw_tail(origin: Vector2, side: float) -> void:
	var border: float = float(UiTokens.COMPONENT_BUTTON_BORDER_WIDTH)
	var top: float = origin.y - float(UiTokens.SIZE_SPACE_4) - border * 2.0
	var tip: Vector2 = Vector2(origin.x + side * 0.46, origin.y + side * 0.2)
	var left: Vector2 = Vector2(origin.x + side * 0.38, top)
	var right: Vector2 = Vector2(origin.x + side * 0.56, top)
	draw_colored_polygon(PackedVector2Array([left, right, tip]), UiTokens.COMPONENT_SPEECH_BUBBLE_BG)
	draw_line(left + Vector2(0.0, border), tip, UiTokens.COMPONENT_SPEECH_BUBBLE_BORDER, border, true)
	draw_line(right + Vector2(0.0, border), tip, UiTokens.COMPONENT_SPEECH_BUBBLE_BORDER, border, true)


## Tail of a bubble placed on the right (coach layout): it starts on the bubble border and points at Mây.
func _draw_side_tail(origin: Vector2, side: float) -> void:
	var border: float = float(UiTokens.COMPONENT_BUTTON_BORDER_WIDTH)
	var base_x: float = size.x + float(UiTokens.SIZE_SPACE_4) + border * 2.0
	var tip: Vector2 = Vector2(origin.x + side * 0.86, origin.y + side * 0.5)
	var top: Vector2 = Vector2(base_x, origin.y + side * 0.38)
	var bottom: Vector2 = Vector2(base_x, origin.y + side * 0.6)
	draw_colored_polygon(PackedVector2Array([top, bottom, tip]), UiTokens.COMPONENT_SPEECH_BUBBLE_BG)
	draw_line(top - Vector2(border, 0.0), tip, UiTokens.COMPONENT_SPEECH_BUBBLE_BORDER, border, true)
	draw_line(bottom - Vector2(border, 0.0), tip, UiTokens.COMPONENT_SPEECH_BUBBLE_BORDER, border, true)
