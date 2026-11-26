class_name EligibleActivityModel
extends RefCounted
## One activity of `listPartyEligibleActivities` (F101-AC-05, F102-AC-10): the activity (`PartyActivityType` + ref,
## `{vi, en}` title), its member range and, per member, whether the server finds them eligible. A member who is not
## eligible carries only a reason code and a requirement of the activity (e.g. `grade_levels:4`), never the grade or
## school of the student (AGENTS R16). The client never computes eligibility (R1, R5): it only shows the list.

## `PartyIneligibleMember.reason_code` → sentence key (`{nickname}` is filled by the lobby).
const REASON_KEYS: Dictionary = {
	"grade_not_allowed": "ui.party.reason.grade_not_allowed",
	"grade_band_mismatch": "ui.party.reason.grade_band_mismatch",
	"prerequisite_missing": "ui.party.reason.prerequisite_missing",
	"area_locked": "ui.party.reason.area_locked",
	"attempt_limit": "ui.party.reason.attempt_limit",
	"sanctioned": "ui.party.reason.sanctioned",
	"not_online": "ui.party.reason.not_online",
}
const REASON_GENERIC: String = "ui.party.reason.generic"
## Grade reason without a readable requirement ("hoạt động của khối khác").
const REASON_GRADE_OTHER: String = "ui.party.reason.grade_other"
const KINDS: PackedStringArray = ["explore", "quest", "boss", "learning_challenge", "coop_challenge", "team_battle"]
## Icon of each activity kind on its card (design/icons/game).
const KIND_ICONS: Dictionary = {
	"explore": "map", "quest": "journal", "boss": "fire", "learning_challenge": "star", "coop_challenge": "star",
	"team_battle": "flag",
}

var activity_type: String = ""
var activity_ref: String = ""
var title: Dictionary = {}
var min_members: int = 1
var max_members: int = 8
var all_eligible: bool = false
## `[{student_id, eligible, reason_code, requirement}]` of the members the server checked.
var members: Array[Dictionary] = []


static func from_dict(data: Variant) -> EligibleActivityModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var kind: String = DataRead.str_of(d, "activity_type")
	var ref: String = DataRead.str_of(d, "activity_ref")
	if not kind in KINDS or ref.is_empty() or ref.length() > 64:
		return null
	var m: EligibleActivityModel = EligibleActivityModel.new()
	m.activity_type = kind
	m.activity_ref = ref
	m.title = DataRead.text_of(d, "title")
	m.min_members = clampi(DataRead.int_of(d, "min_members", 1), 1, 8)
	m.max_members = clampi(DataRead.int_of(d, "max_members", 8), m.min_members, 8)
	m.all_eligible = DataRead.bool_of(d, "all_eligible")
	for raw: Variant in DataRead.arr_of(d, "members"):
		if raw is Dictionary:
			var e: Dictionary = raw as Dictionary
			var id: String = DataRead.str_of(e, "student_id", DataRead.str_of(e, "user_id"))
			if Uuid.is_valid(id):
				m.members.append({"student_id": id, "eligible": DataRead.bool_of(e, "eligible", true),
					"reason_code": DataRead.str_of(e, "reason_code"), "requirement": DataRead.str_of(e, "requirement").left(255)})
	return m


static func list_from(data: Variant) -> Array[EligibleActivityModel]:
	var out: Array[EligibleActivityModel] = []
	if not data is Array:
		return out
	for raw: Variant in data as Array:
		var m: EligibleActivityModel = from_dict(raw)
		if m != null:
			out.append(m)
	return out


## Members the server found not eligible: `[{student_id, reason_code, requirement}]`.
func ineligible() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for e: Dictionary in members:
		if not bool(e["eligible"]):
			out.append(e)
	return out


func display_title() -> String:
	var text: String = L10n.pick(title)
	return text if not text.is_empty() else activity_ref


func icon_name() -> String:
	return String(KIND_ICONS.get(activity_type, "star"))


## "cần khối 4" style words for a reason code + activity requirement (`grade_levels:4,5` → "4, 5").
static func reason_text(reason_code: String, requirement: String) -> String:
	var key: String = String(REASON_KEYS.get(reason_code, REASON_GENERIC))
	var grades: String = ""
	if requirement.begins_with("grade_levels:"):
		grades = ", ".join(requirement.trim_prefix("grade_levels:").split(",", false))
	if reason_code == "grade_not_allowed" and grades.is_empty():
		key = REASON_GRADE_OTHER
	return TranslationServer.translate(key).format({"grades": grades})
