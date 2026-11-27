class_name ArenaModeModel
extends RefCounted
## One competition mode the server opened for the signed-in student (`listCompetitionModes` → `CompetitionMode`;
## F109-AC-01, COMPETITION_SYSTEM §2). The server already filtered the modes by the student's grade, school and the
## effective matchmaking config (F104-AC-22): the client shows exactly this list and never decides a mode by grade
## (R1, R5). `subjects` empty = the config does not restrict the subject.

const MODES: PackedStringArray = ["battle_1v1", "team_battle", "coop_challenge", "timed_quiz", "tournament"]
const MATCHMAKING_TYPES: PackedStringArray = ["random", "skill", "grade", "subject", "topic", "team"]

var mode: String = ""
var matchmaking_types: PackedStringArray = []
var ranked: bool = false
var ranked_matchmaking_types: PackedStringArray = []
var min_grade: int = 1
var subjects: PackedStringArray = []


## Parses a `CompetitionMode`; null when `mode` is unknown or a required field is missing.
static func from_dict(data: Variant) -> ArenaModeModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: ArenaModeModel = ArenaModeModel.new()
	m.mode = DataRead.str_of(d, "mode")
	if not m.mode in MODES or not d.get("matchmaking_types") is Array or not d.get("ranked") is bool:
		return null
	for t: String in DataRead.strings_of(d, "matchmaking_types", 8):
		if t in MATCHMAKING_TYPES:
			m.matchmaking_types.append(t)
	m.ranked = bool(d["ranked"])
	m.ranked_matchmaking_types = DataRead.strings_of(d, "ranked_matchmaking_types", 8)
	m.min_grade = clampi(DataRead.int_of(d, "min_grade", 1), 1, 9)
	for s: String in DataRead.strings_of(d, "subjects", 50):
		if Uuid.is_valid(s):
			m.subjects.append(s)
	return m


static func list_from(raw: Variant) -> Array[ArenaModeModel]:
	var out: Array[ArenaModeModel] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var m: ArenaModeModel = from_dict(item)
			if m != null:
				out.append(m)
	return out


## The queue may group by topic (`topic` matchmaking type): the picker then offers the topics of the subject.
func allows_topic() -> bool:
	return "topic" in matchmaking_types


## The matchmaking type sent with the ticket: `topic` when a topic is chosen and the mode allows it, otherwise the
## server default (none sent).
func matchmaking_type_for(topic_id: String) -> String:
	return "topic" if not topic_id.is_empty() and allows_topic() else ""
