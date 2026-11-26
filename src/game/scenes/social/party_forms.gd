class_name PartyForms
extends RefCounted
## Builders of the Party screen parts that are not the lobby itself (F102): a pending invitation row (portrait,
## "{nickname} mời em vào nhóm!", the time left from the server `expires_at`, "Lần sau nhé" / "Vào nhóm") and the leader
## settings chips of the older bands. Plain Controls styled by the generated Theme (R32), texts as `ui.party.*` keys,
## only the nickname of the inviter (R16). UI only (R1).


## One pending invitation; `on_accept` / `on_decline` receive the invitation.
static func invitation_row(inv: InvitationModel, service: PartyService, on_accept: Callable, on_decline: Callable) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Invite_%s" % inv.invitation_id.right(12)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var thumb: AvatarThumb = AvatarThumb.new()
	thumb.standalone = true
	var side: float = UiBuild.target()
	thumb.custom_minimum_size = Vector2(side, side)
	thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var look: AvatarConfigModel = inv.inviter_avatar_model()
	thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	row.add_child(thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	row.add_child(texts)
	var text: Label = UiBuild.data_label(TranslationServer.translate("ui.party.invite.received").format({
		"nickname": inv.inviter_nickname}))
	text.name = "Text"
	texts.add_child(text)
	var left: int = inv.seconds_left(service.now_ms()) if service != null else 0
	if left > 0:
		var time: Label = UiBuild.data_label(TranslationServer.translate("ui.party.invite.time_left").format({
			"time": InvitationToast.clock_text(left)}), &"MutedLabel")
		time.name = "TimeLeft"
		texts.add_child(time)
	var decline: Button = UiBuild.button("ui.party.invite.decline", "cancel", &"SecondaryButton")
	decline.name = "Decline"
	decline.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	decline.pressed.connect(on_decline.bind(inv))
	row.add_child(decline)
	var accept: Button = UiBuild.button("ui.party.invite.accept", "check")
	accept.name = "Accept"
	accept.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	accept.disabled = service == null or service.is_offline()
	accept.pressed.connect(on_accept.bind(inv))
	row.add_child(accept)
	return card


## Leader settings of the older bands (PARTY_SYSTEM §12.4 "lớp 6–9: thêm settings nâng cao"): who may join, who may
## invite, as chips (chosen = yellow + tick, F114). Values only as the server knows them (`private`/`friends`,
## `leader_only`/`members` — ADR-0095: no class).
static func settings_rows(box: VBoxContainer, p: PartyModel) -> void:
	for spec: Array in [["ui.party.settings.visibility", "visibility", [["private", "ui.party.settings.visibility_private"],
			["friends", "ui.party.settings.visibility_friends"]], p.visibility],
			["ui.party.settings.policy", "invitation_policy", [["leader_only", "ui.party.settings.policy_leader"],
			["members", "ui.party.settings.policy_members"]], p.invitation_policy]]:
		var tag: Label = UiBuild.label(String(spec[0]), &"SectionTag", false)
		tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		box.add_child(tag)
		var row: HFlowContainer = HFlowContainer.new()
		row.name = "Row_%s" % String(spec[1])
		box.add_child(row)
		var group: ButtonGroup = ButtonGroup.new()
		var chips: Array = []
		for option: Array in spec[2] as Array:
			var chip: Button = UiBuild.chip(String(option[1]), group)
			chip.name = "Chip_%s" % String(option[0])
			chip.set_meta("value", String(option[0]))
			chip.set_meta("field", String(spec[1]))
			chip.button_pressed = String(option[0]) == String(spec[3])
			chips.append(chip)
			row.add_child(chip)
		UiBuild.mark_chips(chips)
		for chip: Variant in chips:
			(chip as Button).toggled.connect(func(_on: bool) -> void: UiBuild.mark_chips(chips))


## The settings the leader changed in `box` (merge patch of `updatePartySettings`).
static func settings_changes(box: VBoxContainer, p: PartyModel) -> Dictionary:
	var changes: Dictionary = {}
	for node: Node in box.find_children("Chip_*", "Button", true, false):
		var chip: Button = node as Button
		if chip.button_pressed:
			var field: String = String(chip.get_meta("field"))
			var value: String = String(chip.get_meta("value"))
			if p != null and String(p.get(field)) != value:
				changes[field] = value
	return changes
