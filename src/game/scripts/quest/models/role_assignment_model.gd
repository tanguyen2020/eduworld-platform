class_name RoleAssignmentModel
extends RefCounted
## One role of an `interdependent` step (AsyncAPI `quest.roles.assigned.assignments[]`, OpenAPI `QuestStepRoles`;
## F110 data-model §1, QUEST_SYSTEM §3.2): the role code, who holds it (nickname only, R16) and whether it is this
## student's. The role name and icon come from the clue card of that role when the server sent it (the student's own
## roles and the shared ones) — another member's clue is never on this client (AC-QST-014).

var quest_id: String = ""
var step_id: String = ""
var role: String = ""
var student_id: String = ""
var nickname: String = ""
var is_me: bool = false
## `LocalizedText` name of the role (from its clue card), empty when unknown.
var title: Dictionary = {}
var icon_key: String = ""


static func from_dict(quest: String, step: String, data: Variant, self_id: String) -> RoleAssignmentModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var role_code: String = DataRead.str_of(d, "role")
	var student: String = DataRead.str_of(d, "student_id")
	if role_code.is_empty() or role_code.length() > 32 or not Uuid.is_valid(student):
		return null
	var m: RoleAssignmentModel = RoleAssignmentModel.new()
	m.quest_id = quest
	m.step_id = step
	m.role = role_code
	m.student_id = student
	m.nickname = DataRead.str_of(d, "nickname").left(32)
	m.is_me = not self_id.is_empty() and student == self_id
	return m
