class_name ChallengeSummaryModel
extends RefCounted
## A challenge the server lists for this student (`GameChallengeSummary`, CHALLENGE_BOSS_SYSTEM §2): already filtered by
## grade, unlocks, time window and tenant; the difficulties are the server's (`available_difficulties` — `g1_3`
## easy–hard by default is a server rule, never a client rule, R1). F069 plays the seven solo types; a challenge that
## needs a party (or a type of F109/F110) has no solo button (F069-AC-14).

## Solo types of F076 played by this client (spec Clarifications 2026-11-16).
const SOLO_TYPES: PackedStringArray = ["time", "perfect", "survival", "puzzle", "knowledge", "weekly", "seasonal"]
const DIFFICULTIES: PackedStringArray = ["easy", "normal", "hard", "expert", "legendary"]
## Translation keys of the contract types and difficulties, listed literally for tools/extract_pot.gd.
const TYPE_KEYS: PackedStringArray = [
	"ui.challenge.type.time", "ui.challenge.type.perfect", "ui.challenge.type.survival", "ui.challenge.type.puzzle",
	"ui.challenge.type.knowledge", "ui.challenge.type.weekly", "ui.challenge.type.seasonal", "ui.challenge.type.boss",
	"ui.challenge.type.coop", "ui.challenge.type.party", "ui.challenge.type.tournament",
]
const DIFFICULTY_KEYS: PackedStringArray = [
	"ui.challenge.difficulty.easy", "ui.challenge.difficulty.normal", "ui.challenge.difficulty.hard",
	"ui.challenge.difficulty.expert", "ui.challenge.difficulty.legendary",
]

var challenge_id: String = ""
var challenge_type: String = ""
var title: Dictionary = {}
var area_id: String = ""
var available_difficulties: PackedStringArray = []
var party_required: bool = false
var available_to: String = ""
## -1 = not sent.
var attempts_left: int = -1
## Attempt in progress of this student (contract §6 delta `active_attempt_id`; "" when absent).
var active_attempt_id: String = ""


static func from_dict(d: Variant) -> ChallengeSummaryModel:
	var s: ChallengeSummaryModel = ChallengeSummaryModel.new()
	return s if s.parse(d) else null


static func list_from(v: Variant) -> Array[ChallengeSummaryModel]:
	var out: Array[ChallengeSummaryModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var s: ChallengeSummaryModel = from_dict(d)
			if s != null:
				out.append(s)
	return out


func parse(d: Variant) -> bool:
	if not d is Dictionary:
		return false
	var m: Dictionary = d as Dictionary
	challenge_id = DataRead.str_of(m, "challenge_id")
	if not Uuid.is_valid(challenge_id):
		return false
	challenge_type = DataRead.str_of(m, "challenge_type")
	title = DataRead.text_of(m, "title")
	area_id = DataRead.str_of(m, "area_id")
	available_difficulties = []
	for level: String in DataRead.strings_of(m, "available_difficulties", DIFFICULTIES.size()):
		if level in DIFFICULTIES and not level in available_difficulties:
			available_difficulties.append(level)
	party_required = DataRead.bool_of(m, "party_required")
	available_to = DataRead.str_of(m, "available_to")
	attempts_left = DataRead.nullable_int(m, "attempts_left")
	var active: String = DataRead.str_of(m, "active_attempt_id")
	active_attempt_id = active if Uuid.is_valid(active) else ""
	return true


func can_play_solo() -> bool:
	return not party_required and challenge_type in SOLO_TYPES


func display_title() -> String:
	return L10n.pick(title)


func type_key() -> String:
	return "ui.challenge.type." + challenge_type if challenge_type in type_codes() \
			else "ui.challenge.type.other"


## Contract challenge types with a translation key.
static func type_codes() -> PackedStringArray:
	var out: PackedStringArray = []
	for key: String in TYPE_KEYS:
		out.append(key.trim_prefix("ui.challenge.type."))
	return out
