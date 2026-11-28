class_name CoopQuestStore
extends RefCounted
## RAM store of the party quest session of this student (F110 US2/US3, data-model §3; ADR-0064 — nothing on disk):
## the shared progress of each step (group share + "I reached my part"), the roles of an `interdependent` step and the
## clue cards the server sent (own role + shared ones). Clues are keyed by `(quest_id, step_id, role)`: a shared card
## that is already on the board is never added twice (spec edge case). Cleared on logout / session replaced.

var _progress: Dictionary = {}
var _roles: Dictionary = {}
var _my_roles: Dictionary = {}
var _pending_leader: Dictionary = {}
var _clues: Dictionary = {}


static func key(quest_id: String, step_id: String) -> String:
	return "%s|%s" % [quest_id, step_id]


func progress(quest_id: String, step_id: String) -> PartyQuestProgressModel:
	return _progress.get(key(quest_id, step_id)) as PartyQuestProgressModel


## `party_progress` of `quest.progress.updated` (one step) or of the REST `QuestProgress` (a list).
func apply_progress(quest_id: String, data: Variant) -> bool:
	var changed: bool = false
	var rows: Array = data as Array if data is Array else [data]
	for raw: Variant in rows:
		var m: PartyQuestProgressModel = PartyQuestProgressModel.from_dict(quest_id, raw)
		if m != null:
			_progress[key(quest_id, m.step_id)] = m
			changed = true
	return changed


func roles(quest_id: String, step_id: String) -> Array[RoleAssignmentModel]:
	var out: Array[RoleAssignmentModel] = []
	for r: Variant in _roles.get(key(quest_id, step_id), []) as Array:
		out.append(r as RoleAssignmentModel)
	return out


func my_roles(quest_id: String, step_id: String) -> PackedStringArray:
	return _my_roles.get(key(quest_id, step_id), PackedStringArray()) as PackedStringArray


func has_roles(quest_id: String, step_id: String) -> bool:
	return _roles.has(key(quest_id, step_id))


func pending_leader(quest_id: String, step_id: String) -> bool:
	return bool(_pending_leader.get(key(quest_id, step_id), false))


## `quest.roles.assigned` / `QuestStepRoles`: replaces the roles of the step. Returns the roles this student holds now
## and did not hold before (a member past the grace left them: "Em giữ thêm vai…", F110-AC-18).
func apply_roles(payload: Dictionary, self_id: String) -> PackedStringArray:
	var quest_id: String = DataRead.str_of(payload, "quest_id")
	var step_id: String = DataRead.str_of(payload, "step_id")
	var k: String = key(quest_id, step_id)
	var before: PackedStringArray = my_roles(quest_id, step_id)
	var had: bool = _roles.has(k)
	var list: Array = []
	var mine: PackedStringArray = []
	for raw: Variant in DataRead.arr_of(payload, "assignments"):
		var r: RoleAssignmentModel = RoleAssignmentModel.from_dict(quest_id, step_id, raw, self_id)
		if r != null and list.size() < 4:
			_name_role(r, quest_id, step_id)
			list.append(r)
			if r.is_me and not r.role in mine:
				mine.append(r.role)
	if payload.get("my_roles") is Array:
		for role: String in DataRead.strings_of(payload, "my_roles", 4):
			if not role in mine:
				mine.append(role)
	_roles[k] = list
	_my_roles[k] = mine
	_pending_leader[k] = DataRead.bool_of(payload, "pending_leader")
	var added: PackedStringArray = []
	if had and not before.is_empty():
		for role: String in mine:
			if not role in before:
				added.append(role)
	return added


func clues(quest_id: String, step_id: String) -> Array[ClueCardModel]:
	var out: Array[ClueCardModel] = []
	for c: Variant in (_clues.get(key(quest_id, step_id), {}) as Dictionary).values():
		out.append(c as ClueCardModel)
	return out


func clue(quest_id: String, step_id: String, role: String) -> ClueCardModel:
	return (_clues.get(key(quest_id, step_id), {}) as Dictionary).get(role) as ClueCardModel


## `getQuestStepClues` / `shareQuestStepClue`: exactly the cards the server returned (F110-AC-15); the shared cards
## already on the board stay (a card never disappears while the step runs, F110-AC-18).
func set_clues(body: Dictionary) -> void:
	var quest_id: String = DataRead.str_of(body, "quest_id")
	var step_id: String = DataRead.str_of(body, "step_id")
	var k: String = key(quest_id, step_id)
	var old: Dictionary = _clues.get(k, {}) as Dictionary
	var next: Dictionary = {}
	for c: ClueCardModel in old.values():
		if c.shared:
			next[c.role] = c
	for raw: Variant in DataRead.arr_of(body, "clues"):
		var c: ClueCardModel = ClueCardModel.from_dict(raw)
		if c != null and next.size() < 4:
			next[c.role] = c
	_clues[k] = next
	if body.get("my_roles") is Array:
		_my_roles[k] = DataRead.strings_of(body, "my_roles", 4)
	for r: RoleAssignmentModel in roles(quest_id, step_id):
		_name_role(r, quest_id, step_id)


## `quest.clue.shared`: true when the card is new on the board (the client then reads the rendered card).
func is_new_share(payload: Dictionary) -> bool:
	var c: ClueCardModel = clue(DataRead.str_of(payload, "quest_id"), DataRead.str_of(payload, "step_id"),
			DataRead.str_of(payload, "role"))
	return c == null or not c.shared


func clear() -> void:
	_progress.clear()
	_roles.clear()
	_my_roles.clear()
	_pending_leader.clear()
	_clues.clear()


func is_empty() -> bool:
	return _progress.is_empty() and _roles.is_empty() and _clues.is_empty()


func _name_role(r: RoleAssignmentModel, quest_id: String, step_id: String) -> void:
	var c: ClueCardModel = clue(quest_id, step_id, r.role)
	if c != null:
		r.title = c.title
		r.icon_key = c.icon_key
