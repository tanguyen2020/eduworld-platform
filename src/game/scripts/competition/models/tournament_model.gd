class_name TournamentModel
extends RefCounted
## A tournament in the student's scope (`Tournament` of `listTournaments` / `getTournament`; F109-AC-23, AC-24,
## COMPETITION_SYSTEM §10.3). The server already filtered the audience (grade ± students − exclusions, ADR-0095): the
## client never filters by grade. The register / check-in buttons follow the server status and windows only.

const NAME_MAX_LENGTH: int = 80

var tournament_id: String = ""
var name: String = ""
var status: String = ""
var format: String = ""
var mode: String = ""
var participant_type: String = "individual"
var subject_id: String = ""
var grade_level: int = 0
var registration_open_ms: int = 0
var registration_closes_ms: int = 0
var check_in_opens_ms: int = 0
var starts_ms: int = 0
var max_entries: int = 0
var entry_count: int = 0
var reward_summary: String = ""
var cancel_reason: String = ""
var my_entry: TournamentEntryModel = null


static func from_dict(data: Variant) -> TournamentModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var t: TournamentModel = TournamentModel.new()
	t.tournament_id = DataRead.str_of(d, "tournament_id")
	t.name = DataRead.str_of(d, "name").left(NAME_MAX_LENGTH)
	t.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(t.tournament_id) or t.name.is_empty() or t.status.is_empty():
		return null
	t.format = DataRead.str_of(d, "format")
	t.mode = DataRead.str_of(d, "mode")
	t.participant_type = DataRead.str_of(d, "participant_type", "individual")
	t.subject_id = DataRead.str_of(d, "subject_id")
	t.grade_level = DataRead.nullable_int(d, "grade_level")
	t.registration_open_ms = _ms(d, "registration_open_at")
	t.registration_closes_ms = _ms(d, "registration_closes_at")
	t.check_in_opens_ms = _ms(d, "check_in_opens_at")
	t.starts_ms = _ms(d, "starts_at")
	t.max_entries = maxi(0, DataRead.int_of(d, "max_entries"))
	t.entry_count = maxi(0, DataRead.int_of(d, "entry_count"))
	t.reward_summary = DataRead.str_of(d, "reward_summary").left(1000)
	t.cancel_reason = DataRead.str_of(d, "cancel_reason").left(1000)
	t.my_entry = TournamentEntryModel.from_dict(d.get("my_entry"))
	return t


static func list_from(raw: Variant) -> Array[TournamentModel]:
	var out: Array[TournamentModel] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var t: TournamentModel = from_dict(item)
			if t != null:
				out.append(t)
	return out


static func _ms(d: Dictionary, key: String) -> int:
	var text: String = DataRead.str_of(d, key)
	return ExamCountdown.iso_to_ms(text) if not text.is_empty() else 0


func registered() -> bool:
	return my_entry != null and my_entry.is_active()


## "Đăng ký" is offered while the server status is `registration_open` and the student has no active entry.
func can_register() -> bool:
	return status == "registration_open" and not registered()


## "Rút tên" only before the bracket is locked (server statuses up to `registration_closed`).
func can_withdraw() -> bool:
	return registered() and status in ["registration_open", "registration_closed"] and my_entry.status != "checked_in"


## Check-in is enabled only inside the server window (from `check_in_opens_at` to `starts_at`, server time `now_ms`).
func can_check_in(now_ms: int) -> bool:
	if not registered() or my_entry.status == "checked_in" or check_in_opens_ms <= 0:
		return false
	return now_ms >= check_in_opens_ms and (starts_ms <= 0 or now_ms < starts_ms) and status in ["seeding", "scheduled",
		"registration_closed", "registration_open"]


func checked_in() -> bool:
	return my_entry != null and my_entry.status == "checked_in"
