class_name BossQuestionModel
extends RefCounted
## A question the server gave to THIS student for the running phase (AsyncAPI `boss.question.assigned` → one model per
## `questions[]` item; OpenAPI `BossEncounter.my_questions[]` after a reconnect; F110 data-model §1). The render payload
## has no answer key (ADR-0016, AGENTS R6) and is drawn by the F063 renderer of its type. `state`: `open` (may be
## answered), `sending` (waiting for the server ack — the same question cannot be sent twice), `evaluated` (the server
## answered `boss.answer.evaluated` / REST `answered`), `closed` (`BOSS_PHASE_CLOSED`, a new phase).

const STATE_OPEN: String = "open"
const STATE_SENDING: String = "sending"
const STATE_EVALUATED: String = "evaluated"
const STATE_CLOSED: String = "closed"

var encounter_id: String = ""
var phase_id: String = ""
var requirement_id: String = ""
var question_instance_id: String = ""
var question_type: String = ""
var render: QuestionRenderModel = null
## Server deadline of the question (unix ms; 0 = none).
var deadline_ms: int = 0
var state: String = STATE_OPEN
## Device time (ms) the question was shown — only for `client_elapsed_ms` (reference for anti-cheat, ADR-0016).
var shown_at_ms: int = 0


## Every question of a `boss.question.assigned` payload (an item without `question_instance_id` is dropped).
static func list_from_assigned(payload: Dictionary) -> Array[BossQuestionModel]:
	var out: Array[BossQuestionModel] = []
	var deadline: int = ExamCountdown.iso_to_ms(DataRead.str_of(payload, "deadline_at"))
	var index: int = 0
	for raw: Variant in DataRead.arr_of(payload, "questions"):
		if not raw is Dictionary:
			continue
		var q: BossQuestionModel = _make(raw as Dictionary, DataRead.str_of(payload, "encounter_id"),
				DataRead.str_of(payload, "phase_id"), DataRead.str_of(payload, "requirement_id"), index)
		if q != null:
			q.deadline_ms = deadline
			out.append(q)
			index += 1
	return out


## One `BossQuestion` of `getBossEncounter.my_questions` (render + its meta).
static func from_rest(data: Variant, encounter: String, phase: String, index: int = 0) -> BossQuestionModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var flat: Dictionary = DataRead.dict_of(d, "render").duplicate(true)
	flat["question_instance_id"] = DataRead.str_of(d, "question_instance_id")
	flat["question_type"] = DataRead.str_of(d, "question_type", DataRead.str_of(flat, "question_type"))
	var q: BossQuestionModel = _make(flat, encounter, phase, DataRead.str_of(d, "requirement_id"), index)
	if q != null and DataRead.bool_of(d, "answered"):
		q.state = STATE_EVALUATED
	return q


static func _make(q: Dictionary, encounter: String, phase: String, requirement: String, index: int) -> BossQuestionModel:
	var instance: String = DataRead.str_of(q, "question_instance_id")
	var kind: String = DataRead.str_of(q, "question_type")
	if instance.is_empty() or instance.length() > 64 or kind.is_empty():
		return null
	var m: BossQuestionModel = BossQuestionModel.new()
	m.encounter_id = encounter
	m.phase_id = phase.left(64)
	m.requirement_id = requirement.left(64)
	m.question_instance_id = instance
	m.question_type = kind
	m.render = BattleView.question_from(q, index)
	return m


func is_open() -> bool:
	return state == STATE_OPEN


## The command payload of `boss.answer.submit` (AsyncAPI): ids, the raw response, `client_elapsed_ms` for reference.
func submit_payload(response: Dictionary, now_ms: int) -> Dictionary:
	return {"encounter_id": encounter_id, "phase_id": phase_id, "requirement_id": requirement_id,
		"question_instance_id": question_instance_id, "response": response,
		"client_elapsed_ms": maxi(0, now_ms - shown_at_ms) if shown_at_ms > 0 else 0}
