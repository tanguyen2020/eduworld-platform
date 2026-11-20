class_name RendererContext
extends RefCounted
## What a renderer needs besides the attempt (plan F063 `ActivityHostApi`): the content language fixed by the
## attempt (ADR-0056 — a question keeps its language when the UI locale changes, F063-AC-13), the grade band skin,
## the formula and media loaders and the clock. Renderers read; they never call the network themselves.

var content_language: String = L10n.FALLBACK_LANGUAGE
var grade_band: String = ""
var math: MathSvgCache = null
var media: MediaLoader = null
var clock: ClientClock = null
## g1_3 reads the stem aloud automatically (STUDENT_GAME_UX_LEARNING §7).
var auto_voice: bool = false


## Text of a content string (`i18nText`: a plain string in the content language, or `{vi, en}`) in the attempt's
## language — never the UI locale.
func text(value: Variant) -> String:
	if value is String:
		return value as String
	if value is Dictionary:
		return L10n.pick(value as Dictionary, content_language, content_language)
	return ""


func now_msec() -> int:
	return clock.now_msec() if clock != null else Time.get_ticks_msec()
