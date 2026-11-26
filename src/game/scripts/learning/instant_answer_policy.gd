class_name InstantAnswerPolicy
extends RefCounted
## Tap-to-answer (F098, STUDENT_GAME_UX_LEARNING §4.1b): whether a tap on an answer tile sends the answer at once
## (no "Kiểm tra" step) and whether the grade band asks for a second tap first. Reads only what the server sent in
## the `Attempt` — the timing rules are ACTIVITY_ENGINE §8.1's — plus one client setting; it decides nothing about
## grading (AGENTS R5, R6) and adds no backend policy. Kept out of the scenes so the UI only wires signals (R1).

## One-step question types: one tap is a whole answer. `true_false` only with a single statement.
const INSTANT_TYPES: PackedStringArray = ["single_choice", "true_false"]
## Modes that never show per-question feedback while answering (ACTIVITY_ENGINE §8.1; owner 2026-11-26).
const BLOCKED_MODES: PackedStringArray = ["exam", "diagnostic", "competition"]
const TIMING_IMMEDIATE: String = "immediate"
## Client setting: grade bands that confirm with a second tap (CONFIG_DEFAULTS `tap_confirm_bands`, default none).
const SETTING: String = "eduworld/learning/tap_confirm_bands"


## The attempt shows per-question feedback right after each answer: effective `feedback_timing = immediate`, a mode
## with feedback while answering, and results released at once (an assignment with a later release never qualifies).
static func mode_allows(attempt: AttemptModel) -> bool:
	if attempt == null or not attempt.is_in_progress():
		return false
	return attempt.feedback_timing == TIMING_IMMEDIATE and not attempt.mode in BLOCKED_MODES \
			and attempt.results_immediate()


## The item is answered by one tap: a single choice, or true/false with one statement (several statements are a
## multi-step answer that keeps "Kiểm tra").
static func item_allows(item: QuestionRenderModel) -> bool:
	if item == null or not item.type in INSTANT_TYPES:
		return false
	if item.type == "true_false":
		var statements: Variant = item.render_payload.get("statements")
		return not (statements is Array and not (statements as Array).is_empty())
	return true


static func applies(attempt: AttemptModel, item: QuestionRenderModel) -> bool:
	return mode_allows(attempt) and item_allows(item)


## Grade bands listed in the client setting confirm with a second tap on the same tile (default: none).
static func double_tap(band: String) -> bool:
	var bands: Variant = ProjectSettings.get_setting(SETTING, PackedStringArray())
	if bands is PackedStringArray:
		return band in (bands as PackedStringArray)
	if bands is Array:
		return band in (bands as Array)
	return false
