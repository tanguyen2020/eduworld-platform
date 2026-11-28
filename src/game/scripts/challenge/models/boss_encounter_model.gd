class_name BossEncounterModel
extends RefCounted
## The boss encounter on screen (OpenAPI `BossEncounter`, AsyncAPI `boss.encounter.updated` / `boss.phase.*` /
## `boss.encounter.ended`; F110 data-model §1). Every number is the server's: the energy is replaced, never computed
## (AGENTS R5, ADR-0016); `my_correct_count` counts only this student's own `boss.answer.evaluated` (shown to them alone,
## FR-PTY-013). `members` hold nickname + ready / connected / `contribution_met` only (R16, R19).

const STATUS_LOBBY: String = "lobby"
const STATUS_ACTIVE: String = "phase_active"
const STATUS_TRANSITION: String = "phase_transition"
const STATUS_RETRY: String = "phase_retry"
const STATUS_DEFEATED: String = "defeated"
const STATUS_ENDED: String = "ended"
const STATUSES: PackedStringArray = [STATUS_LOBBY, STATUS_ACTIVE, STATUS_TRANSITION, STATUS_RETRY, STATUS_DEFEATED,
	STATUS_ENDED]
const OUTCOME_DEFEATED: String = "defeated"
const OUTCOME_TIME_UP: String = "time_up"
const OUTCOME_ABANDONED: String = "abandoned"
const OUTCOMES: PackedStringArray = [OUTCOME_DEFEATED, OUTCOME_TIME_UP, OUTCOME_ABANDONED]

var encounter_id: String = ""
var boss_id: String = ""
var boss_version: int = 1
var mode: String = BossSummary.MODE_SOLO
var party_id: String = ""
var status: String = STATUS_LOBBY
var state_version: int = 0
## 0-based index of the current phase (`current_phase` / `phase_index`).
var current_phase: int = 0
var phase_id: String = ""
var phase_count: int = 1
var phases_cleared: int = 0
var energy_remaining_pct: float = 100.0
var phase_progress: float = 0.0
var phase_retries_left: int = -1
## Server unix ms (0 = unknown).
var phase_deadline_ms: int = 0
var deadline_ms: int = 0
var outcome: String = ""
var members: Array[BossMemberModel] = []
var my_correct_count: int = 0


## REST `BossEncounter`; null without a valid id.
static func from_dict(data: Variant) -> BossEncounterModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not Uuid.is_valid(DataRead.str_of(d, "encounter_id")):
		return null
	var e: BossEncounterModel = BossEncounterModel.new()
	e.encounter_id = DataRead.str_of(d, "encounter_id")
	e.boss_id = DataRead.str_of(d, "boss_id")
	e.boss_version = maxi(1, DataRead.int_of(d, "boss_version", 1))
	e.mode = BossSummary.MODE_PARTY if DataRead.str_of(d, "mode") == BossSummary.MODE_PARTY else BossSummary.MODE_SOLO
	e.party_id = DataRead.str_of(d, "party_id")
	e.phase_count = clampi(DataRead.int_of(d, "phase_count", 1), 1, 8)
	e.phases_cleared = clampi(DataRead.int_of(d, "phases_cleared"), 0, 8)
	e.phase_retries_left = DataRead.int_of(d, "phase_retries_left", -1)
	e.deadline_ms = ExamCountdown.iso_to_ms(DataRead.str_of(d, "deadline_at"))
	var o: String = DataRead.str_of(d, "outcome")
	e.outcome = o if o in OUTCOMES else ""
	e.apply_update(d)
	return e


## Applies the fields of `boss.encounter.updated` (or the REST body): only the fields present change.
func apply_update(d: Dictionary) -> void:
	var s: String = DataRead.str_of(d, "status")
	if s in STATUSES:
		status = s
	if WsEnvelope.is_int(d.get("state_version")):
		state_version = maxi(0, DataRead.int_of(d, "state_version"))
	if d.has("current_phase"):
		current_phase = maxi(0, DataRead.int_of(d, "current_phase"))
	if d.has("phase_id"):
		phase_id = DataRead.str_of(d, "phase_id").left(64)
	if d.has("energy_remaining_pct"):
		energy_remaining_pct = clampf(DataRead.num_of(d, "energy_remaining_pct", energy_remaining_pct), 0.0, 100.0)
	if d.has("phase_progress"):
		phase_progress = clampf(DataRead.num_of(d, "phase_progress"), 0.0, 1.0)
	if d.has("phase_deadline_at"):
		phase_deadline_ms = ExamCountdown.iso_to_ms(DataRead.str_of(d, "phase_deadline_at"))
	if d.get("members") is Array:
		members = BossMemberModel.list_from(d["members"])


func member(student_id: String) -> BossMemberModel:
	for m: BossMemberModel in members:
		if m.student_id == student_id:
			return m
	return null


func is_party() -> bool:
	return mode == BossSummary.MODE_PARTY


func is_over() -> bool:
	return not outcome.is_empty() or status in [STATUS_DEFEATED, STATUS_ENDED]


## "Phase n/total" (1-based) for the HUD.
func phase_number() -> int:
	return clampi(current_phase + 1, 1, maxi(phase_count, 1))
