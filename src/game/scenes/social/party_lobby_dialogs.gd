class_name PartyLobbyDialogs
extends RefCounted
## The confirmations of the Party screen (F102-AC-04, AC-12) on one PartyDialog: leave, switch party, the leader menu of
## a member card (transfer the lead / remove from the party, each confirmed again), the leader settings (who may join /
## invite for the older bands, disband for every band, confirmed again). The safe choice has the focus; red only for
## leaving / ending (F114). It calls PartyService only after the student confirmed — nothing is done silently (R1).

## A confirmed action finished (`ok` = the server took it); the lobby renders again.
signal finished(action: StringName, ok: bool)

## Grade bands with the extra party settings (PARTY_SYSTEM §12.4 "lớp 6–9: thêm settings nâng cao").
const SETTINGS_BANDS: PackedStringArray = ["g6_7", "g8_9"]
const CANCEL_BUTTON: Array = [PartyDialog.CANCEL, "ui.common.cancel", "cancel", &"SecondaryButton"]

var dialog: PartyDialog = null
var service: PartyService = null
var pending_member: String = ""
var invitation: InvitationModel = null


func _init(party_dialog: PartyDialog) -> void:
	dialog = party_dialog
	dialog.chosen.connect(_on_chosen)


## "Rời nhóm hiện tại và tham gia?" (F102-AC-04): leave + accept only once the student agrees.
func ask_switch(inv: InvitationModel) -> void:
	invitation = inv
	pending_member = ""
	dialog.present("ui.party.switch.title", TranslationServer.translate("ui.party.switch_party_confirm"),
			[CANCEL_BUTTON, [&"switch", "ui.party.switch.confirm", "check", &""]])


func ask_leave() -> void:
	dialog.present("ui.party.leave.title", TranslationServer.translate("ui.party.leave_confirm"),
			[CANCEL_BUTTON, [&"leave", "ui.party.leave", "exit_light", &"DangerButton"]])


func ask_settings(party: PartyModel, band: String) -> void:
	dialog.clear_content()
	var buttons: Array = [CANCEL_BUTTON]
	if band in SETTINGS_BANDS:
		PartyForms.settings_rows(dialog.content(), party)
		buttons.append([&"save", "ui.party.settings.save", "check", &""])
	buttons.append([&"disband", "ui.party.disband", "exit_light", &"DangerButton"])
	dialog.present("ui.party.settings.title", TranslationServer.translate("ui.party.settings.hint"), buttons, "", true)


## The leader menu of another member's card.
func ask_member(member: PartyMemberModel) -> void:
	pending_member = member.student_id
	dialog.present("ui.party.member_menu", member.nickname, [
		[PartyDialog.CANCEL, "ui.common.close", "cancel", &"SecondaryButton"],
		[&"transfer", "ui.party.transfer", "crown", &"SecondaryButton"],
		[&"kick", "ui.party.kick", "exit_light", &"DangerButton"],
	])


func _on_chosen(action: StringName) -> void:
	var p: PartyModel = service.store.party if service != null else null
	var m: PartyMemberModel = p.member(pending_member) if p != null and not pending_member.is_empty() else null
	var ok: bool = false
	match action:
		&"leave":
			ok = (await service.leave()).ok
		&"switch":
			if invitation != null:
				ok = (await service.switch_and_accept(invitation)).ok
		&"disband":
			dialog.present("ui.party.disband.title", TranslationServer.translate("ui.party.disband_confirm"),
					[CANCEL_BUTTON, [&"disband_confirmed", "ui.party.disband", "exit_light", &"DangerButton"]])
		&"disband_confirmed":
			ok = (await service.disband()).ok
		&"save":
			var changes: Dictionary = PartyForms.settings_changes(dialog.content(), p)
			ok = changes.is_empty() or (await service.update_settings(changes)).ok
		&"transfer":
			if m != null:
				dialog.present("ui.party.transfer.title", TranslationServer.translate("ui.party.transfer_confirm").format({
					"nickname": m.nickname}), [CANCEL_BUTTON, [&"transfer_confirmed", "ui.party.transfer", "crown", &""]])
		&"transfer_confirmed":
			if m != null:
				ok = (await service.transfer_leader(m.student_id)).ok
		&"kick":
			if m != null:
				dialog.present("ui.party.kick.title", TranslationServer.translate("ui.party.kick_confirm").format({
					"nickname": m.nickname}), [CANCEL_BUTTON, [&"kick_confirmed", "ui.party.kick", "exit_light", &"DangerButton"]])
		&"kick_confirmed":
			if m != null:
				ok = (await service.kick(m.student_id)).ok
	finished.emit(action, ok)
