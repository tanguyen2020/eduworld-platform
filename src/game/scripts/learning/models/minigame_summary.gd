class_name MiniGameSummaryModel
extends RefCounted
## End of a mini-game as the server reports it after submit (F073; MINIGAME_ENGINE §5–§6): the learning score of the
## attempt, the game score (fun points, never used for mastery) and whether the completion rule was met. Every value is
## the server's (ADR-0016); the client only displays it. `results` are the graded answers the host received, used to
## show deferred right/wrong effects at the end (`feedback_display = end_of_round | end_of_game`).

var status: String = ""
var has_game_score: bool = false
var game_score: int = 0
var has_completion: bool = false
var completion_met: bool = false
var score: float = -1.0
var max_score: float = -1.0
var results: Array[ResponseResultModel] = []


## Reads `Attempt.evaluation.{score, max_score, game_score, completion_met}` (top-level `game_score` /
## `completion_met` are accepted too).
static func from_attempt(a: AttemptModel, graded: Array[ResponseResultModel] = []) -> MiniGameSummaryModel:
	var s: MiniGameSummaryModel = MiniGameSummaryModel.new()
	s.results = graded
	if a == null:
		return s
	s.status = a.status
	var ev: Dictionary = a.evaluation
	var points: Variant = ev.get("game_score")
	s.has_game_score = points is int or points is float
	s.game_score = maxi(0, int(points)) if s.has_game_score else 0
	var met: Variant = ev.get("completion_met")
	s.has_completion = met is bool
	s.completion_met = met == true
	var sc: Variant = ev.get("score")
	s.score = float(sc) if sc is int or sc is float else -1.0
	var mx: Variant = ev.get("max_score")
	s.max_score = float(mx) if mx is int or mx is float else -1.0
	return s


## Graded answers with a verdict, in item order.
func verdicts() -> Array[ResponseResultModel]:
	var out: Array[ResponseResultModel] = []
	for r: ResponseResultModel in results:
		if r.has_verdict:
			out.append(r)
	out.sort_custom(func(x: ResponseResultModel, y: ResponseResultModel) -> bool: return x.position < y.position)
	return out
