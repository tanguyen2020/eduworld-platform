class_name NextStepHint
extends RefCounted
## "Việc tiếp theo" card of the HUD (F096-AC-11, STUDENT_GAME_UX_CORE S03 "Đi đâu tiếp?"): one always-visible
## suggestion so a student never wonders what to do — school work first (it opens the "Nhà trường giao" tab), else
## exploring (it opens the World Map). Pure model over data the client already has (the assignment count of
## AppState); it decides no business rule and calls no service (AGENTS R1).

const KIND_ASSIGNED: String = "assigned"
const KIND_EXPLORE: String = "explore"


## `{kind, label_key, args, icon, screen_id, screen_args}` for `assignment_count` open school assignments.
static func for_assignments(assignment_count: int) -> Dictionary:
	if assignment_count > 0:
		return {"kind": KIND_ASSIGNED, "label_key": "ui.next_step.assignments", "args": {"count": assignment_count},
			"icon": "res://resources/ui/icons/assignments.svg", "screen_id": &"quest_journal",
			"screen_args": {"tab": "assigned"}}
	return {"kind": KIND_EXPLORE, "label_key": "ui.next_step.explore", "args": {},
		"icon": "res://resources/ui/icons/map.svg", "screen_id": &"world_map", "screen_args": {}}
