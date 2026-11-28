class_name QuestModel
extends RefCounted
## One quest of the student as the server lists it (`QuestSummary`, `QuestDetail`, `QuestProgress` of
## specs/openapi, F072): texts, type, status, the steps of the delivery (no grading data, ADR-0016) and the step states
## of the progress. Pure data; every decision (availability, step completion) comes from the server.

const STATUS_AVAILABLE: String = "available"
const STATUS_ACTIVE: String = "active"
const STATUS_COMPLETED: String = "completed"
const STEP_ACTIVE: String = "active"
const STEP_COMPLETED: String = "completed"
const STEP_TYPE_INTERDEPENDENT: String = "interdependent"

var quest_id: String = ""
var quest_version_id: String = ""
var code: String = ""
var name: Dictionary = {}
var description: Dictionary = {}
var quest_type: String = ""
var status: String = ""
var can_accept: bool = false
var locked_reason: String = ""
var area_id: String = ""
var giver_npc_id: String = ""
var progress_id: String = ""
## Delivery steps (`QuestStepView`) once the detail was read; empty for a summary.
var steps: Array[Dictionary] = []
## step_id → `QuestStepState` (`{status, count?, target?}`).
var step_states: Dictionary = {}
## F110: `QuestProgress.party_progress` of a party quest session (`QuestPartyStepProgress` rows: group share + own part).
var party_progress: Array = []


static func from_dict(d: Variant) -> QuestModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var q: QuestModel = QuestModel.new()
	q.quest_id = String(m.get("quest_id", ""))
	if q.quest_id.is_empty():
		return null
	q.apply(m)
	return q


## Merges a summary, a detail or a progress body.
func apply(m: Dictionary) -> void:
	quest_version_id = String(m.get("quest_version_id", quest_version_id))
	code = String(m.get("code", code))
	if m.get("name") is Dictionary:
		name = m["name"] as Dictionary
	if m.get("description") is Dictionary:
		description = m["description"] as Dictionary
	quest_type = String(m.get("quest_type", quest_type))
	status = String(m.get("status", status))
	can_accept = bool(m.get("can_accept", can_accept))
	locked_reason = String(m.get("locked_reason", ""))
	area_id = String(m.get("area_id", area_id))
	giver_npc_id = String(m.get("giver_npc_id", giver_npc_id))
	if m.get("steps") is Array:
		steps.clear()
		for s: Variant in m["steps"] as Array:
			if s is Dictionary:
				steps.append(s as Dictionary)
	var progress: Variant = m.get("progress")
	if progress is Dictionary:
		apply_progress(progress as Dictionary)
	elif m.has("step_states"):
		apply_progress(m)


## Applies a `QuestProgress` body.
func apply_progress(p: Dictionary) -> void:
	progress_id = String(p.get("quest_progress_id", progress_id))
	if p.get("step_states") is Dictionary:
		step_states = (p["step_states"] as Dictionary).duplicate(true)
	if p.has("status") and p.has("quest_progress_id"):
		status = String(p["status"])
	if p.get("party_progress") is Array:
		party_progress = (p["party_progress"] as Array).duplicate(true)


## Applies a `quest.progress.updated` message (status and the state of one step).
func apply_update(payload: Dictionary) -> void:
	status = String(payload.get("status", status))
	progress_id = String(payload.get("quest_progress_id", progress_id))
	var step_id: String = String(payload.get("step_id", ""))
	if not step_id.is_empty():
		var st: Dictionary = (step_states.get(step_id, {}) as Dictionary).duplicate()
		st["status"] = String(payload.get("step_status", st.get("status", "")))
		if payload.has("count"):
			st["count"] = int(payload["count"])
		step_states[step_id] = st
	var nexts: Variant = payload.get("next_step_ids")
	if nexts is Array:
		for n: Variant in nexts as Array:
			var st: Dictionary = (step_states.get(String(n), {}) as Dictionary).duplicate()
			st["status"] = STEP_ACTIVE
			step_states[String(n)] = st


func is_active() -> bool:
	return status == STATUS_ACTIVE


func step_status(step_id: String) -> String:
	return String((step_states.get(step_id, {}) as Dictionary).get("status", ""))


## The steps currently active (delivery order).
func active_steps() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for s: Dictionary in steps:
		if step_status(String(s.get("step_id", ""))) == STEP_ACTIVE:
			out.append(s)
	return out


## The active interaction step a world interaction may report (talk_npc by npc, go_to/collect by interactable), or
## empty. The server still checks the presence, the area and the ids (ADR-0016).
func step_for(target_area: String, interactable_id: String, npc_id: String) -> Dictionary:
	for s: Dictionary in active_steps():
		var kind: String = String(s.get("step_type", ""))
		var step_area: String = String(s.get("area_id", ""))
		if not step_area.is_empty() and step_area != target_area:
			continue
		var targets: Array = s.get("interactable_ids", []) as Array if s.get("interactable_ids") is Array else []
		match kind:
			"talk_npc":
				if not npc_id.is_empty() and String(s.get("npc_id", "")) == npc_id:
					return s
			"go_to", "collect":
				if not interactable_id.is_empty() and interactable_id in targets:
					return s
	return {}


## F110: the first active `interdependent` step (roles + clue cards of a party quest), or empty.
func interdependent_step() -> Dictionary:
	for s: Dictionary in active_steps():
		if String(s.get("step_type", "")) == STEP_TYPE_INTERDEPENDENT:
			return s
	return {}


## The first active step that needs a learning activity (`activity`), or empty.
func activity_step() -> Dictionary:
	for s: Dictionary in active_steps():
		if String(s.get("step_type", "")) == "activity":
			return s
	return {}


func display_name(locale: String = "") -> String:
	return L10n.pick(name, locale)
