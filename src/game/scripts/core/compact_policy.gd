class_name CompactPolicy
extends RefCounted
## F125 (STUDENT_GAME_UX §2 compact breakpoint, DESIGN_SYSTEM §5.2): pure rules of compact mode — a phone held
## sideways. The base canvas 1280×720 (`stretch canvas_items` + `aspect expand`) shrunk as a whole into a 844×390
## window shows a 18 px text at ≈ 10 px and a 44 px button at ≈ 24 px; in compact mode the window
## `content_scale_factor` makes the logical canvas `component.compact.logical_height` (540) tall, so everything is
## ≈ 1.33× larger, and the compact density floors (`density.compact.*`) raise the touch target and the smallest font.
## Applied by OrientationGuard (with the rotate prompt of F113, which takes priority while it shows). Unit-tested
## (tests/unit/core/test_compact_policy.gd).

## Window short side (density-independent px: CSS px on the Web, dp on Android, pt on iOS) below which the game is in
## compact mode (STUDENT_GAME_UX §2 owns the value; project setting, never in code paths).
const SHORT_SIDE_SETTING: String = "eduworld/ui/compact_short_side"
const DEFAULT_SHORT_SIDE: float = 500.0


static func short_side_limit() -> float:
	return float(ProjectSettings.get_setting(SHORT_SIDE_SETTING, DEFAULT_SHORT_SIDE))


## Logical canvas height of compact mode (`component.compact.logical_height`).
static func logical_height() -> float:
	return float(UiTokens.COMPONENT_COMPACT_LOGICAL_HEIGHT)


## F125-AC-01: a window of `window` physical px on a screen of `density` px per density-independent px is compact when
## its short side is below the limit. `keyboard` = height of an open on-screen keyboard (px): it is added back, so a
## keyboard never switches the mode (edge case "bàn phím ảo").
static func is_compact(window: Vector2, density: float = 1.0, keyboard: float = 0.0) -> bool:
	if window.x <= 0.0 or window.y <= 0.0:
		return false
	var full: Vector2 = window + Vector2(0.0, maxf(keyboard, 0.0))
	return minf(full.x, full.y) / maxf(density, 0.01) < short_side_limit()


## F125-AC-01: `content_scale_factor` of the window: 1 outside compact mode; in compact mode the factor that makes the
## logical canvas `logical_height()` tall, capped so the canvas keeps at least the aspect of the base canvas (a window
## narrower than 16:9 stays `logical_height × 16/9` wide), never below 1.
static func content_scale(window: Vector2, base: Vector2, compact: bool) -> float:
	if not compact or window.x <= 0.0 or window.y <= 0.0 or base.x <= 0.0 or base.y <= 0.0:
		return 1.0
	var view: Vector2 = logical_size(window, base, 1.0)
	var height: float = logical_height()
	var by_height: float = view.y / height
	var by_width: float = view.x / (height * base.x / base.y)
	return maxf(1.0, minf(by_height, by_width))


## The logical canvas of a window (stretch `canvas_items` + `expand` on `base`) under a content scale `factor`.
static func logical_size(window: Vector2, base: Vector2, factor: float) -> Vector2:
	var px: float = physical_scale(window, base, factor)
	return window / px if px > 0.0 else base


## Window px per logical px (stretch × content scale): logical sizes × this = sizes on screen.
static func physical_scale(window: Vector2, base: Vector2, factor: float) -> float:
	if window.x <= 0.0 or window.y <= 0.0 or base.x <= 0.0 or base.y <= 0.0:
		return 1.0
	return minf(window.x / base.x, window.y / base.y) * maxf(factor, 0.01)


## F125-AC-03: minimum height of every pressable control of `band` (`density.<band>.button.min_height`; in compact mode
## at least `density.compact.button.min_height`).
static func button_target(band: String, compact: bool) -> float:
	var value: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT.get(band, UiTokens.DENSITY_BUTTON_MIN_HEIGHT[
			UiTokens.DENSITY_BANDS[0]]))
	return maxf(value, float(UiTokens.DENSITY_COMPACT_BUTTON_MIN_HEIGHT)) if compact else value


## F125-AC-03, AC-05: smallest font of `band` (`density.<band>.font.small`; in compact mode at least
## `density.compact.font.small` — a band font is never lowered).
static func font_floor(band: String, compact: bool) -> int:
	var value: int = int(UiTokens.DENSITY_FONT_SMALL.get(band, UiTokens.DENSITY_FONT_SMALL[UiTokens.DENSITY_BANDS[0]]))
	return maxi(value, UiTokens.DENSITY_COMPACT_FONT_SMALL) if compact else value
