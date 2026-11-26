class_name OrientationPolicy
extends RefCounted
## F113 (STUDENT_GAME_UX §2: landscape on every platform, no portrait layout; GODOT_ARCHITECTURE §10.2): pure rules of
## the rotate prompt. Android/iOS are locked to sensor landscape in project.godot; on the Web (or any window) that is
## taller than wide the game is covered by the rotate prompt instead of a portrait layout. Unit-tested
## (tests/unit/core/test_orientation_policy.gd).

const BASE_HEIGHT_SETTING: String = "display/window/size/viewport_height"


## A view taller than wide is portrait; a square view counts as landscape (F113-AC-03).
static func is_portrait(view: Vector2) -> bool:
	return view.y > view.x


## Short side of the base canvas (1280×720 → 720): the prompt is laid out as if on the landscape canvas.
static func base_short() -> float:
	var height: float = float(ProjectSettings.get_setting(BASE_HEIGHT_SETTING, 720))
	var width: float = float(ProjectSettings.get_setting("display/window/size/viewport_width", 1280))
	return maxf(1.0, minf(width, height))


## Scale of the prompt layer on a logical view `view` (F113-AC-05): the short side of the view over the short side of
## the base canvas, never below 1. With `stretch canvas_items` + `expand`, a 390×844 window is a 1280×2770 logical view
## (factor 0.305), so unscaled text would show at a third of its size; scaled by 1280/720 it shows as on the
## landscape canvas of the same phone.
static func prompt_scale(view: Vector2, short_side: float) -> float:
	if short_side <= 0.0 or view.x <= 0.0 or view.y <= 0.0:
		return 1.0
	return maxf(1.0, minf(view.x, view.y) / short_side)
