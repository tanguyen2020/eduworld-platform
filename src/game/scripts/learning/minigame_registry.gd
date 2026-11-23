class_name MiniGameRegistry
extends RefCounted
## `minigame_type` → mini-game scene (MINIGAME_ENGINE §2, §4.2; F073-AC-21): the sub-registry the `minigame` activity
## renderer and MiniGameHost use. Each entry names its scene, the question types it accepts (the compatibility table
## of §2 — the server already refuses incompatible configurations at publish) and its visual variants:
## `presentation.scene_variant = "<type>.<variant>"` (e.g. `matching.market_cards`) only restyles the same scene with
## a Theme type variation of the generated Theme — never a new script (AGENTS R32). A type missing here makes the host
## show "Cần cập nhật game" and send telemetry, without crashing.

const SCENES_DIR: String = "res://scenes/minigames/types"
const TYPES: Dictionary = {
	"multiple_choice": {
		"scene": SCENES_DIR + "/multiple_choice/MultipleChoice.tscn",
		"question_types": ["single_choice", "multiple_choice", "image_based"],
		"variants": {"balloons": "QuestCard", "gates": "VoiceCaption", "cards": "QuestCard"},
	},
	"true_false": {
		"scene": SCENES_DIR + "/true_false/TrueFalse.tscn",
		"question_types": ["true_false"],
		"variants": {"signs": "QuestCard", "run_sides": "VoiceCaption"},
	},
	"fill_in_blank": {
		"scene": SCENES_DIR + "/fill_in_blank/FillInBlank.tscn",
		"question_types": ["fill_in_blank", "short_answer"],
		"variants": {"blocks": "QuestCard", "keyboard": "VoiceCaption"},
	},
	"drag_and_drop": {
		"scene": SCENES_DIR + "/drag_and_drop/DragAndDrop.tscn",
		"question_types": ["drag_drop", "image_based"],
		"variants": {"baskets": "QuestCard", "shelves": "VoiceCaption"},
	},
	"matching": {
		"scene": SCENES_DIR + "/matching/Matching.tscn",
		"question_types": ["matching"],
		"variants": {"card_flip": "QuestCard", "market_cards": "QuestCard", "fishing_pond": "VoiceCaption"},
	},
	"ordering": {
		"scene": SCENES_DIR + "/ordering/Ordering.tscn",
		"question_types": ["ordering"],
		"variants": {"train": "QuestCard", "stairs": "VoiceCaption"},
	},
	"calculation": {
		"scene": SCENES_DIR + "/calculation/Calculation.tscn",
		"question_types": ["numeric"],
		"variants": {"abacus": "QuestCard", "calculator": "VoiceCaption"},
	},
	"puzzle": {
		"scene": SCENES_DIR + "/puzzle/Puzzle.tscn",
		"question_types": [
			"single_choice", "multiple_choice", "true_false", "fill_in_blank", "short_answer", "numeric", "matching",
			"ordering", "drag_drop", "image_based",
		],
		"variants": {"picture": "QuestCard", "map": "VoiceCaption"},
	},
	"word_game": {
		"scene": SCENES_DIR + "/word_game/WordGame.tscn",
		"question_types": ["fill_in_blank", "short_answer", "ordering"],
		"variants": {"crossword": "QuestCard", "letter_blocks": "VoiceCaption"},
	},
	"science_simulation": {
		"scene": SCENES_DIR + "/science_simulation/ScienceSimulation.tscn",
		"question_types": ["interactive"],
		"variants": {"lab": "QuestCard", "garden": "VoiceCaption"},
	},
	"timed_challenge": {
		"scene": SCENES_DIR + "/timed_challenge/TimedChallenge.tscn",
		"question_types": ["single_choice", "true_false", "numeric"],
		"variants": {"race": "VoiceCaption", "stopwatch": "QuestCard"},
	},
}
## Spellings of the same type in older contracts (`drag_drop` in the first MiniGameSession enum).
const ALIASES: Dictionary = {"drag_drop": "drag_and_drop"}


static func canonical(minigame_type: String) -> String:
	return String(ALIASES.get(minigame_type, minigame_type))


static func has(minigame_type: String) -> bool:
	var path: String = scene_path(minigame_type)
	return not path.is_empty() and ResourceLoader.exists(path)


static func types() -> PackedStringArray:
	return PackedStringArray(TYPES.keys())


static func scene_path(minigame_type: String) -> String:
	var entry: Variant = TYPES.get(canonical(minigame_type))
	return String((entry as Dictionary).get("scene", "")) if entry is Dictionary else ""


static func question_types(minigame_type: String) -> PackedStringArray:
	var entry: Variant = TYPES.get(canonical(minigame_type))
	return PackedStringArray((entry as Dictionary).get("question_types", [])) if entry is Dictionary else PackedStringArray()


## Whether `minigame_type` draws items of `question_type`.
static func accepts(minigame_type: String, question_type: String) -> bool:
	return question_type in question_types(minigame_type)


## The variant name of `scene_variant` for this type (`"matching.market_cards"` → `"market_cards"`); "" when the
## variant is unknown, belongs to another type or is not set (the scene keeps its default look).
static func variant_of(minigame_type: String, scene_variant: String) -> String:
	var prefix: String = canonical(minigame_type) + "."
	if not scene_variant.begins_with(prefix):
		return ""
	var variant: String = scene_variant.trim_prefix(prefix)
	return variant if _variants(minigame_type).has(variant) else ""


## Theme type variation of the stage panel for a variant ("" = default panel).
static func variant_style(minigame_type: String, variant: String) -> String:
	return String(_variants(minigame_type).get(variant, ""))


## A new scene for `minigame_type`, null when this client has none.
static func instantiate(minigame_type: String) -> MiniGameBase:
	if not has(minigame_type):
		return null
	return (load(scene_path(minigame_type)) as PackedScene).instantiate() as MiniGameBase


static func _variants(minigame_type: String) -> Dictionary:
	var entry: Variant = TYPES.get(canonical(minigame_type))
	var v: Variant = (entry as Dictionary).get("variants", {}) if entry is Dictionary else {}
	return v as Dictionary if v is Dictionary else {}
