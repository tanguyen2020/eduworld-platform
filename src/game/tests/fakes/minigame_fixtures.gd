class_name MiniGameFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): F073 mini-game bodies. `tests/fixtures/minigames/<type>.json` is one `MiniGameSession` per
## `minigame_type` (render payloads copied from the F063 contract `Attempt` example — no answer key, no `correct`, no
## scoring rule, no option feedback); graded results and submit bodies are built here like LearningFixtures does.

const DIR: String = "res://tests/fixtures/minigames"
const CONFIG_ID: String = "0f730000-0000-4000-8000-0000000c0f01"
const QUEST_STEP_ID: String = "0f730000-0000-4000-8000-0000000d0001"
const ROUTE_START: String = "POST /game/minigames/*:start"
const ROUTE_RESPOND: String = "POST /learning/activity-attempts/*/responses"
const ROUTE_SUBMIT: String = "POST /learning/activity-attempts/*:submit"
const ROUTE_GET: String = "GET /learning/activity-attempts/*"
const CORE_TYPES: PackedStringArray = ["multiple_choice", "true_false", "fill_in_blank", "drag_and_drop", "matching", "ordering"]
const EXTENDED_TYPES: PackedStringArray = ["calculation", "puzzle", "word_game", "science_simulation", "timed_challenge"]
## Answer-side keys that must never be in a render payload or a display config.
const FORBIDDEN: PackedStringArray = [
	"answer_key", "correct", "is_correct", "scoring_rule", "accepted_answers", "correct_option_id", "correct_option_ids",
	"option_feedback", "feedback", "explanation",
]


static func session(type: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [DIR, type])) as Dictionary


## The attempt alone (quest step / assignment: ActivityHost path).
static func attempt(type: String) -> Dictionary:
	return session(type)["attempt"] as Dictionary


static func model(type: String, now_unix: float = 0.0) -> MiniGameSessionModel:
	return MiniGameSessionModel.from_dict(session(type), now_unix)


static func attempt_id(type: String) -> String:
	return String(attempt(type)["attempt_id"])


## `ActivityResponseResult` of a mini-game answer (verdict, server game points, streak).
static func result(position: int, correct: bool, points: int = 10, streak: int = 1) -> Dictionary:
	return {
		"accepted": true, "position": position, "is_correct": correct, "score": 1 if correct else 0,
		"hints_used": 0, "game_points": points if correct else 0, "streak": streak if correct else 0,
	}


## Grading hidden by the feedback timing (`after_submit`): no verdict, no points.
static func result_hidden(position: int) -> Dictionary:
	return {"accepted": true, "position": position, "is_correct": null, "score": null, "hints_used": 0}


## The submitted attempt with the server's evaluation (learning score, game score, completion).
static func submitted(type: String, game_score: int = 30, completion_met: bool = true) -> Dictionary:
	var a: Dictionary = attempt(type)
	a["status"] = "graded"
	a["submitted_at"] = "2026-11-20T02:07:00Z"
	a["evaluation"] = {
		"evaluation_id": "0f730000-0000-4000-8000-0000000e0001", "attempt_id": a["attempt_id"], "source": "system",
		"status": "final", "effective": true, "created_at": "2026-11-20T02:07:00Z", "score": 2, "max_score": 3,
		"passed": completion_met, "game_score": game_score, "completion_met": completion_met,
	}
	return a


## JSON paths of every FORBIDDEN key in `value`.
static func forbidden_paths(value: Variant, path: String = "$") -> PackedStringArray:
	var found: PackedStringArray = []
	if value is Dictionary:
		for key: Variant in (value as Dictionary).keys():
			if String(key) in FORBIDDEN:
				found.append("%s.%s" % [path, key])
			found.append_array(forbidden_paths((value as Dictionary)[key], "%s.%s" % [path, key]))
	elif value is Array:
		var i: int = 0
		for entry: Variant in (value as Array):
			found.append_array(forbidden_paths(entry, "%s[%d]" % [path, i]))
			i += 1
	return found


## A complete answer for the question scene currently shown in `scene` (drives the F063 answer model like a tap would).
static func answer(scene: MiniGameQuestionScene) -> void:
	var q: QuestionRendererBase = scene.question
	match q.item.type:
		"single_choice":
			(q.answer as SingleChoiceAnswer).select("o_b")
		"multiple_choice":
			(q.answer as MultipleChoiceAnswer).toggle("o_a")
		"true_false":
			(q.answer as TrueFalseAnswer).set_value(false)
		"fill_in_blank":
			(q.answer as FillInBlankAnswer).set_blank("b_1", "4")
		"short_answer":
			(q.answer as ShortAnswerAnswer).set_text("phép nhân")
		"numeric":
			(q.answer as NumericAnswer).set_text("6")
		"matching":
			for pair: Array in [["l_1", "r_2"], ["l_2", "r_1"], ["l_3", "r_3"]]:
				(q.answer as MatchingAnswer).pair(String(pair[0]), String(pair[1]))
		"ordering":
			(q.answer as OrderingAnswer).move("i_2", -1)
		"drag_drop":
			for move: Array in [["d_1", "z_1"], ["d_2", "z_2"], ["d_3", "z_1"]]:
				(q.answer as DragDropAnswer).place(String(move[0]), String(move[1]))
		"interactive":
			(q.get("slider") as HSlider).value = 18.0
