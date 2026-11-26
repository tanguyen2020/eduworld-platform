class_name UiMotion
extends RefCounted
## F114-AC-07 (DESIGN_SYSTEM §5.1, STUDENT_GAME_UX_LEARNING §10.3): a modal window opens with a light zoom
## 0.96 → 1 in at most 0.2 s (`component.window.pop_*` tokens); no animation at all while "Giảm chuyển động" is on.

const POP_SCALE: float = UiTokens.COMPONENT_WINDOW_POP_SCALE
const POP_DURATION: float = UiTokens.COMPONENT_WINDOW_POP_DURATION


## Starts the opening zoom of `window` around its centre; returns the tween, or null when motion is reduced (the
## window is then shown at full size at once).
static func pop_in(window: Control, reduce_motion: bool) -> Tween:
	window.scale = Vector2.ONE
	if reduce_motion or not window.is_inside_tree():
		return null
	var tween: Tween = window.create_tween()
	tween.tween_method(_zoom.bind(window), POP_SCALE, 1.0, POP_DURATION) \
			.set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	return tween


## One zoom step: the pivot follows the window centre (a container may lay the window out after it opens).
static func _zoom(value: float, window: Control) -> void:
	if not is_instance_valid(window):
		return
	window.pivot_offset = window.size * 0.5
	window.scale = Vector2(value, value)
