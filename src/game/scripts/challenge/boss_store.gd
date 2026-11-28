class_name BossStore
extends RefCounted
## RAM store of the boss encounter on screen (F110 data-model §2, §3; ADR-0064 — nothing written to disk). Fed by
## BossClient with the REST `BossEncounter` and the realtime `boss.*` messages of THIS encounter only:
## - `boss.encounter.updated` follows `state_version`: an older or equal version is dropped, a gap (> held + 1) asks for
##   a full read (`resync_needed` → `getBossEncounter`), F110-AC-03;
## - the energy only takes the server value (`boss.encounter.updated` / `boss.phase.completed`), never a client guess
##   (F110-AC-05, AGENTS R5);
## - the questions are the ones the server gave to this student; `boss.answer.evaluated` marks the answered one and
##   counts this student's own right answers (shown to them alone, FR-PTY-013);
## - while an encounter is held (Lobby … Result) `AppState.blocking_activity` = `boss`, so the level-up modal waits for
##   the result (F069 LevelUpQueue, spec edge case).

signal encounter_changed(encounter: BossEncounterModel)
## The question to answer now (null = none: waiting, between phases, closed).
signal question_changed(question: BossQuestionModel)
## `{question_instance_id, is_correct, counted, explanation}` of this student's own answer.
signal answer_evaluated(result: Dictionary)
signal phase_started(retry: bool, presentation_ref: String)
signal phase_completed(phase_index: int)
signal encounter_ended(outcome: String, rewards: Array)
signal resync_needed()

const BLOCKING_ACTIVITY: StringName = &"boss"
const MAX_QUESTIONS: int = 60

var encounter: BossEncounterModel = null
var questions: Array[BossQuestionModel] = []
## `boss.encounter.ended` of this encounter: `{outcome, phases_cleared, phase_count, eligible, rewards}`.
var result: Dictionary = {}
## `presentation_ref` of the last `boss.phase.started` (key of the pinned content pack; spec edge case).
var presentation_ref: String = ""
## AppState (or a test fake) with `begin_blocking(name)` / `end_blocking(name)`.
var app_state: Object = null
var _blocking: bool = false


func _init(store: Object = null) -> void:
	app_state = store


func encounter_id() -> String:
	return encounter.encounter_id if encounter != null else ""


## A full read (`createBossEncounter`, `getBossEncounter`): replaces the encounter unless it is older than the one held;
## `my_questions` (REST) restores the open questions after a reconnect (F110-AC-09).
func apply_snapshot(next: BossEncounterModel, my_questions: Array = []) -> bool:
	if next == null:
		return false
	if encounter != null and encounter.encounter_id == next.encounter_id and next.state_version < encounter.state_version:
		return false
	if encounter != null and encounter.encounter_id == next.encounter_id:
		next.my_correct_count = encounter.my_correct_count
	else:
		result = {}
	encounter = next
	_block(true)
	if not my_questions.is_empty() or next.status != BossEncounterModel.STATUS_ACTIVE:
		questions.clear()
		var i: int = 0
		for raw: Variant in my_questions:
			var q: BossQuestionModel = BossQuestionModel.from_rest(raw, next.encounter_id, next.phase_id, i)
			if q != null and questions.size() < MAX_QUESTIONS:
				questions.append(q)
				i += 1
	encounter_changed.emit(encounter)
	question_changed.emit(current_question())
	return true


## Applies a realtime `boss.*` message of the held encounter; false when ignored (another encounter, stale version).
func apply_message(type: String, payload: Dictionary) -> bool:
	if encounter == null or DataRead.str_of(payload, "encounter_id") != encounter.encounter_id:
		return false
	match type:
		"boss.encounter.updated":
			var version: int = DataRead.int_of(payload, "state_version", -1)
			if version <= encounter.state_version and version >= 0:
				return false
			if version > encounter.state_version + 1:
				resync_needed.emit()
				return false
			encounter.apply_update(payload)
			encounter_changed.emit(encounter)
		"boss.phase.started":
			_close_all()
			encounter.current_phase = maxi(0, DataRead.int_of(payload, "phase_index"))
			encounter.phase_id = DataRead.str_of(payload, "phase_id").left(64)
			encounter.phase_deadline_ms = ExamCountdown.iso_to_ms(DataRead.str_of(payload, "deadline_at"))
			encounter.status = BossEncounterModel.STATUS_ACTIVE
			presentation_ref = DataRead.str_of(payload, "presentation_ref").left(128)
			phase_started.emit(DataRead.bool_of(payload, "retry"), presentation_ref)
			encounter_changed.emit(encounter)
			question_changed.emit(null)
		"boss.phase.completed":
			_close_all()
			var index: int = maxi(0, DataRead.int_of(payload, "phase_index"))
			encounter.energy_remaining_pct = clampf(DataRead.num_of(payload, "energy_remaining_pct",
					encounter.energy_remaining_pct), 0.0, 100.0)
			encounter.phases_cleared = maxi(encounter.phases_cleared, index + 1)
			encounter.status = BossEncounterModel.STATUS_TRANSITION
			phase_completed.emit(index)
			encounter_changed.emit(encounter)
			question_changed.emit(null)
		"boss.question.assigned":
			for q: BossQuestionModel in BossQuestionModel.list_from_assigned(payload):
				if question(q.question_instance_id) == null and questions.size() < MAX_QUESTIONS:
					questions.append(q)
			question_changed.emit(current_question())
		"boss.answer.evaluated":
			var id: String = DataRead.str_of(payload, "question_instance_id")
			var q: BossQuestionModel = question(id)
			if q != null:
				q.state = BossQuestionModel.STATE_EVALUATED
			var correct: bool = DataRead.bool_of(payload, "is_correct")
			var counted: bool = DataRead.bool_of(payload, "counted", true)
			if correct and counted:
				encounter.my_correct_count += 1
			answer_evaluated.emit({"question_instance_id": id, "is_correct": correct, "counted": counted,
				"explanation": DataRead.text_of(payload, "explanation")})
		"boss.encounter.ended":
			var o: String = DataRead.str_of(payload, "outcome")
			if not o in BossEncounterModel.OUTCOMES:
				return false
			_close_all()
			encounter.outcome = o
			encounter.status = BossEncounterModel.STATUS_DEFEATED if o == BossEncounterModel.OUTCOME_DEFEATED \
					else BossEncounterModel.STATUS_ENDED
			if payload.has("phases_cleared"):
				encounter.phases_cleared = clampi(DataRead.int_of(payload, "phases_cleared"), 0, 8)
			if payload.has("phase_count"):
				encounter.phase_count = clampi(DataRead.int_of(payload, "phase_count", 1), 1, 8)
			var rewards: Array = DataRead.arr_of(payload, "rewards").duplicate(true)
			result = {"outcome": o, "phases_cleared": encounter.phases_cleared, "phase_count": encounter.phase_count,
				"eligible": DataRead.bool_of(payload, "eligible"), "rewards": rewards}
			encounter_changed.emit(encounter)
			question_changed.emit(null)
			encounter_ended.emit(o, rewards)
		_:
			return false
	return true


func question(instance_id: String) -> BossQuestionModel:
	for q: BossQuestionModel in questions:
		if q.question_instance_id == instance_id:
			return q
	return null


## The question on screen: the first one still open or on its way to the server.
func current_question() -> BossQuestionModel:
	for q: BossQuestionModel in questions:
		if q.state == BossQuestionModel.STATE_OPEN or q.state == BossQuestionModel.STATE_SENDING:
			return q
	return null


## `BOSS_PHASE_CLOSED`: the question is closed; the next phase brings new ones (F110-AC-05).
func close_question(instance_id: String) -> void:
	var q: BossQuestionModel = question(instance_id)
	if q != null:
		q.state = BossQuestionModel.STATE_CLOSED
	question_changed.emit(current_question())


## The result was closed (or the student left): nothing of the encounter stays; the level-up modal may show.
func clear() -> void:
	encounter = null
	questions.clear()
	result = {}
	presentation_ref = ""
	_block(false)


func _close_all() -> void:
	for q: BossQuestionModel in questions:
		if q.state != BossQuestionModel.STATE_EVALUATED:
			q.state = BossQuestionModel.STATE_CLOSED
	questions.clear()


func _block(on: bool) -> void:
	if on == _blocking:
		return
	_blocking = on
	if app_state != null and app_state.has_method("begin_blocking"):
		app_state.call("begin_blocking" if on else "end_blocking", BLOCKING_ACTIVITY)
