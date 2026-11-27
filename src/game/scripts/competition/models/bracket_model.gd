class_name BracketModel
extends RefCounted
## A tournament bracket (`Bracket` of `getTournamentBracket`; F109-AC-25, COMPETITION_SYSTEM §10.8): the nodes the
## server scheduled and the entries' nicknames / team names only (FR-LDB-003). `student_id` (staff only) and seeds
## stay ids; nothing is computed — winners and statuses are the server's.

var tournament_id: String = ""
var format: String = ""
var status: String = ""
## entry_id → TournamentEntryModel.
var entries: Dictionary = {}
## `{node_id, bracket, round, position, entry_a, entry_b, winner, status, scheduled_ms}` sorted by bracket/round/position.
var nodes: Array[Dictionary] = []


static func from_dict(data: Variant) -> BracketModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var b: BracketModel = BracketModel.new()
	b.tournament_id = DataRead.str_of(d, "tournament_id")
	if not Uuid.is_valid(b.tournament_id) or not d.get("nodes") is Array:
		return null
	b.format = DataRead.str_of(d, "format")
	b.status = DataRead.str_of(d, "status")
	for raw: Variant in DataRead.arr_of(d, "entries"):
		var e: TournamentEntryModel = TournamentEntryModel.from_dict(raw)
		if e != null:
			b.entries[e.entry_id] = e
	for raw: Variant in d["nodes"] as Array:
		if not raw is Dictionary:
			continue
		var n: Dictionary = raw as Dictionary
		var id: String = DataRead.str_of(n, "node_id")
		if not Uuid.is_valid(id):
			continue
		var at: String = DataRead.str_of(n, "scheduled_at")
		b.nodes.append({"node_id": id, "bracket": DataRead.str_of(n, "bracket", "winners"),
			"round": maxi(1, DataRead.int_of(n, "round", 1)), "position": maxi(0, DataRead.int_of(n, "position")),
			"entry_a": DataRead.str_of(n, "entry_a_id"), "entry_b": DataRead.str_of(n, "entry_b_id"),
			"winner": DataRead.str_of(n, "winner_entry_id"), "status": DataRead.str_of(n, "status"),
			"scheduled_ms": ExamCountdown.iso_to_ms(at) if not at.is_empty() else 0})
	b.nodes.sort_custom(func(x: Dictionary, y: Dictionary) -> bool:
		if String(x["bracket"]) != String(y["bracket"]):
			return String(x["bracket"]) < String(y["bracket"])
		if int(x["round"]) != int(y["round"]):
			return int(x["round"]) < int(y["round"])
		return int(x["position"]) < int(y["position"]))
	return b


## Nickname / team name of `entry_id`; "" for an empty slot (a bye or a winner still to come).
func name_of(entry_id: String) -> String:
	var e: TournamentEntryModel = entries.get(entry_id) as TournamentEntryModel
	return e.display_name() if e != null else ""


func rounds() -> int:
	var top: int = 0
	for n: Dictionary in nodes:
		top = maxi(top, int(n["round"]))
	return top


func nodes_of_round(round_no: int) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for n: Dictionary in nodes:
		if int(n["round"]) == round_no and String(n["bracket"]) != "group":
			out.append(n)
	return out


## The path of the student's entry: every node it plays, in round order ("con đường" of lớp 1–5).
func path_of(entry_id: String) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for n: Dictionary in nodes:
		if String(n["entry_a"]) == entry_id or String(n["entry_b"]) == entry_id:
			out.append(n)
	return out


## The student's entry in this bracket (one of its players is `self_id`), or "".
func entry_of_player(self_id: String) -> String:
	for id: Variant in entries.keys():
		if self_id in (entries[id] as TournamentEntryModel).player_ids:
			return String(id)
	return ""
