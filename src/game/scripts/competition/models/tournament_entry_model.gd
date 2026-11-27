class_name TournamentEntryModel
extends RefCounted
## The student's (or a team's) place in a tournament (`TournamentEntry`; F109-AC-24, COMPETITION_SYSTEM §10.4).
## Players are nicknames only (R16; `student_id` is an id, never shown).

var entry_id: String = ""
var tournament_id: String = ""
var status: String = ""
var seed: int = -1
var nicknames: PackedStringArray = []
var player_ids: PackedStringArray = []
var final_rank: int = -1


static func from_dict(data: Variant) -> TournamentEntryModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var e: TournamentEntryModel = TournamentEntryModel.new()
	e.entry_id = DataRead.str_of(d, "entry_id")
	e.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(e.entry_id) or e.status.is_empty():
		return null
	e.tournament_id = DataRead.str_of(d, "tournament_id")
	e.seed = DataRead.nullable_int(d, "seed")
	e.final_rank = DataRead.nullable_int(d, "final_rank")
	for raw: Variant in DataRead.arr_of(d, "players"):
		if raw is Dictionary:
			var nick: String = DataRead.str_of(raw as Dictionary, "nickname").left(32)
			e.nicknames.append(nick)
			e.player_ids.append(DataRead.str_of(raw as Dictionary, "student_id"))
	return e


## Words of the entry on a bracket: the nickname(s) joined (a team shows its members' nicknames).
func display_name() -> String:
	var shown: PackedStringArray = []
	for n: String in nicknames:
		if not n.is_empty():
			shown.append(n)
	return ", ".join(shown)


func is_active() -> bool:
	return status in ["registered", "confirmed", "checked_in"]
