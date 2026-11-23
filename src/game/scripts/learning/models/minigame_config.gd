class_name MiniGameConfigModel
extends RefCounted
## Display part of a pinned mini-game configuration (`MiniGameDisplayConfig`, F073; MINIGAME_ENGINE §3). A closed
## shape for showing the game only: name, instructions, grades, counts, timing, feedback timing/display and the scoring
## *display* switches. It never carries an answer key, a scoring rule or option feedback (ADR-0016, AGENTS R6); any
## answer-side key is dropped like in QuestionRenderModel. The client never scores with these values — game points and
## streaks always come from the server (`ActivityResponseResult.game_points/streak`).

const FEEDBACK_IMMEDIATE: String = "immediate"
const DISPLAY_END_OF_ROUND: String = "end_of_round"
const DISPLAY_END_OF_GAME: String = "end_of_game"
const FEEDBACK_TIMINGS: PackedStringArray = ["immediate", "after_submit", "after_close", "never"]
const FEEDBACK_DISPLAYS: PackedStringArray = ["end_of_round", "end_of_game"]
## `presentation.*` string ids maxLength (specs/schemas/minigame-config.schema.json).
const ID_MAX_LENGTH: int = 64
const NAME_MAX_LENGTH: int = 255
const INSTRUCTIONS_MAX_LENGTH: int = 2000
## Grades of primary school (grades 1–5 get the large touch targets, NFR-A11Y-001).
const PRIMARY_MAX_GRADE: int = 5

var name: Dictionary = {}
var instructions: Dictionary = {}
var grade_levels: Array[int] = []
var question_count: int = 0
var time_limit_seconds: int = 0
var per_item_time_seconds: int = 0
var feedback_timing: String = FEEDBACK_IMMEDIATE
## Empty = effects right after each graded answer.
var feedback_display: String = ""
var show_explanation: bool = false
var show_correct_answer: bool = false
var points_per_correct: int = 0
var time_bonus_enabled: bool = false
var streak_bonus_enabled: bool = false
var scene_variant: String = ""
var theme: String = ""
var background_music_id: String = ""
var mascot_npc_id: String = ""
var resumable: bool = false
## `timed_challenge`: every item arrives up front, the client does not wait for the network between items.
var prefetch_items: bool = false


static func from_dict(data: Dictionary) -> MiniGameConfigModel:
	var clean: Dictionary = QuestionRenderModel.strip_answer_keys(data) as Dictionary
	var c: MiniGameConfigModel = MiniGameConfigModel.new()
	c.name = _i18n(clean.get("name"), NAME_MAX_LENGTH)
	c.instructions = _i18n(clean.get("instructions"), INSTRUCTIONS_MAX_LENGTH)
	var grades: Variant = clean.get("grade_levels", [])
	if grades is Array:
		for g: Variant in (grades as Array):
			if (g is int or g is float) and int(g) >= 1 and int(g) <= 12:
				c.grade_levels.append(int(g))
	c.question_count = _int(clean, "question_count")
	c.time_limit_seconds = _int(clean, "time_limit_seconds")
	c.per_item_time_seconds = _int(clean, "per_item_time_seconds")
	var timing: String = _text(clean, "feedback_timing")
	c.feedback_timing = timing if timing in FEEDBACK_TIMINGS else FEEDBACK_IMMEDIATE
	var display: String = _text(clean, "feedback_display")
	c.feedback_display = display if display in FEEDBACK_DISPLAYS else ""
	c.show_explanation = clean.get("show_explanation") == true
	c.show_correct_answer = clean.get("show_correct_answer") == true
	var scoring: Variant = clean.get("scoring", {})
	if scoring is Dictionary:
		c.points_per_correct = _int(scoring as Dictionary, "points_per_correct")
		c.time_bonus_enabled = (scoring as Dictionary).get("time_bonus_enabled") == true
		c.streak_bonus_enabled = (scoring as Dictionary).get("streak_bonus_enabled") == true
	var presentation: Variant = clean.get("presentation", {})
	if presentation is Dictionary:
		var p: Dictionary = presentation as Dictionary
		c.scene_variant = _text(p, "scene_variant").left(ID_MAX_LENGTH)
		c.theme = _text(p, "theme").left(ID_MAX_LENGTH)
		c.background_music_id = _text(p, "background_music_id").left(ID_MAX_LENGTH)
		c.mascot_npc_id = _text(p, "mascot_npc_id").left(ID_MAX_LENGTH)
	c.resumable = clean.get("resumable") == true
	c.prefetch_items = clean.get("prefetch_items") == true
	return c


## Name in the attempt's content language.
func display_name(language: String) -> String:
	return L10n.pick(name, language, language)


## Right/wrong effects wait for the end of the round or of the game (`feedback_display`).
func defers_effects() -> bool:
	return not feedback_display.is_empty()


## All grades of the configuration are primary grades (1–5).
func is_primary() -> bool:
	if grade_levels.is_empty():
		return false
	for g: int in grade_levels:
		if g > PRIMARY_MAX_GRADE:
			return false
	return true


static func _i18n(value: Variant, limit: int) -> Dictionary:
	var out: Dictionary = {}
	if value is String:
		out["vi"] = (value as String).left(limit)
	elif value is Dictionary:
		for key: String in ["vi", "en"]:
			if (value as Dictionary).get(key) is String:
				out[key] = ((value as Dictionary)[key] as String).left(limit)
	return out


static func _int(data: Dictionary, key: String) -> int:
	var v: Variant = data.get(key)
	return maxi(0, int(v)) if v is int or v is float else 0


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
