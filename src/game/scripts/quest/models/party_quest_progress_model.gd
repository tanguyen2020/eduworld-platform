class_name PartyQuestProgressModel
extends RefCounted
## Shared progress of one step of a party quest session (AsyncAPI `quest.progress.updated.party_progress`, OpenAPI
## `QuestProgress.party_progress[]` = `QuestPartyStepProgress`; F110 data-model §1). Only the group share and whether
## THIS student reached the minimum contribution — never a count or a result of another member (FR-PTY-013, R19).

var quest_id: String = ""
var step_id: String = ""
## 0…1 of the collective target (`collective_progress`; -1 when the step has none).
var collective_progress: float = -1.0
var my_contribution_met: bool = false
## A frame step one member did for the whole party (`shared_completed`).
var shared_completed: bool = false


static func from_dict(quest: String, data: Variant) -> PartyQuestProgressModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var step: String = DataRead.str_of(d, "step_id")
	if step.is_empty() or step.length() > 64 or quest.is_empty():
		return null
	var m: PartyQuestProgressModel = PartyQuestProgressModel.new()
	m.quest_id = quest
	m.step_id = step
	m.collective_progress = clampf(DataRead.num_of(d, "collective_progress"), 0.0, 1.0) \
			if d.has("collective_progress") else -1.0
	m.my_contribution_met = DataRead.bool_of(d, "my_contribution_met")
	m.shared_completed = DataRead.bool_of(d, "shared_completed")
	return m


func has_collective() -> bool:
	return collective_progress >= 0.0
