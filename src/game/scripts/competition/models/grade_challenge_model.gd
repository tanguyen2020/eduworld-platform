class_name GradeChallengeModel
extends RefCounted
## A grade challenge of the school (`CompetitionChallenge` of `listMyChallenges`; F107-AC-27, ADR-0095): the server
## lists only the challenges the student belongs to. A Timed Quiz is joined with `joinChallenge` inside the server join
## window; a school tournament points to its tournament.

const NAME_MAX_LENGTH: int = 80

var challenge_id: String = ""
var name: String = ""
var kind: String = ""
var format: String = ""
var status: String = ""
var starts_ms: int = 0
var ends_ms: int = 0
var join_opens_ms: int = 0
var tournament_id: String = ""


static func from_dict(data: Variant) -> GradeChallengeModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var c: GradeChallengeModel = GradeChallengeModel.new()
	c.challenge_id = DataRead.str_of(d, "challenge_id")
	c.name = DataRead.str_of(d, "name").left(NAME_MAX_LENGTH)
	if not Uuid.is_valid(c.challenge_id) or c.name.is_empty():
		return null
	c.kind = DataRead.str_of(d, "kind")
	c.format = DataRead.str_of(d, "format")
	c.status = DataRead.str_of(d, "status")
	c.starts_ms = TournamentModel._ms(d, "starts_at")
	c.ends_ms = TournamentModel._ms(d, "ends_at")
	c.join_opens_ms = TournamentModel._ms(d, "join_window_opens_at")
	c.tournament_id = DataRead.str_of(d, "tournament_id")
	return c


static func list_from(raw: Variant) -> Array[GradeChallengeModel]:
	var out: Array[GradeChallengeModel] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var c: GradeChallengeModel = from_dict(item)
			if c != null:
				out.append(c)
	return out


func is_timed_quiz() -> bool:
	return format == "timed_quiz"


## "Vào phòng" inside the server join window (server time `now_ms`); the server checks again (422 outside).
func can_join(now_ms: int) -> bool:
	return is_timed_quiz() and status in ["scheduled", "running"] and now_ms >= join_opens_ms \
			and (ends_ms <= 0 or now_ms < ends_ms)
