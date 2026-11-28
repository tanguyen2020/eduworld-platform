class_name RoleAssignDialog
extends ChatSheet
## "Chọn vai" of the party leader (F110-AC-14; QUEST_SYSTEM §3.2 `role_assignment = leader_assigns`): one row per role
## of the step with the party members as chips (nickname only); "Gán vai" sends `assignQuestStepRoles` through the
## panel. Shown only to the leader while the server waits for the leader (`pending_leader`) — a convenience only: the
## server answers `403 PARTY_NOT_LEADER` to anyone else (friendly sentence). UI only.

signal assign_requested(assignments: Array[Dictionary])

const ACT_ASSIGN: StringName = &"assign"

var _choice: Dictionary = {}


## `roles`: the roles of the step (names from their clue cards when known); `members`: `{student_id, nickname}`.
func open_roles(roles: Array[RoleAssignmentModel], members: Array[Dictionary]) -> void:
	_choice.clear()
	var body: VBoxContainer = open_sheet(TranslationServer.translate("ui.quest.coop.assign_title"))
	if roles.is_empty():
		body.add_child(UiBuild.keep_words(UiBuild.label("ui.quest.coop.assign_none", &"MutedLabel")))
	for r: RoleAssignmentModel in roles:
		_choice[r.role] = r.student_id
		var title: String = L10n.pick(r.title) if not r.title.is_empty() else tr("ui.quest.coop.role_unnamed")
		body.add_child(UiBuild.keep_words(UiBuild.data_label(title, &"HeadingLabel")))
		var chips: HFlowContainer = HFlowContainer.new()
		chips.name = "Role_" + r.role
		body.add_child(chips)
		var group: ButtonGroup = ButtonGroup.new()
		for m: Dictionary in members:
			var chip: Button = UiBuild.chip("", group)
			chip.text = String(m.get("nickname", ""))
			chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			chip.name = "Member"
			var id: String = String(m.get("student_id", ""))
			chip.set_pressed_no_signal(id == r.student_id)
			var role: String = r.role
			chip.pressed.connect(func() -> void:
				_choice[role] = id
				UiBuild.mark_chips(chips.get_children()))
			chips.add_child(chip)
		UiBuild.mark_chips(chips.get_children())
	add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	var assign: Button = add_button(ACT_ASSIGN, "ui.quest.coop.assign", "check", &"")
	assign.disabled = roles.is_empty()
	if not chosen.is_connected(_on_chosen):
		chosen.connect(_on_chosen)
	UiBuild.size_buttons(body)


func choice() -> Dictionary:
	return _choice.duplicate()


func _on_chosen(action: StringName) -> void:
	if action != ACT_ASSIGN:
		return
	var out: Array[Dictionary] = []
	for role: String in _choice.keys():
		if Uuid.is_valid(String(_choice[role])):
			out.append({"role": role, "student_id": String(_choice[role])})
	assign_requested.emit(out)
