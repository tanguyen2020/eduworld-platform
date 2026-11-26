class_name InvitationToast
extends PanelContainer
## Lời mời vào nhóm — the small popup at the top right of the HUD (F102-AC-03; PARTY_SYSTEM §5, STUDENT_GAME_UX_SOCIAL
## §5): the inviter's portrait and nickname, how many friends are in the party and the time left before the invitation
## ends, counted from the server `expires_at` (never a client deadline — R5). "Vào nhóm" accepts, "Lần sau nhé"
## declines; the popup hides by itself after AUTO_HIDE_SEC and the invitation stays in the bell / the Party screen.
## Kind words, no pressure (R19): no blinking, no sound, the time left is shown calmly. It never pops during an exam,
## a challenge or the activity player, nor with popups off (the HUD asks PartyService first). Only the nickname of the
## inviter is shown (AGENTS R16). UI only: PartyService does the work (AGENTS R1).

## The student accepted (joined) or must confirm a party switch: the owner opens the Party screen with `args`.
signal open_requested(args: Dictionary)

## STUDENT_GAME_UX_SOCIAL §5: the invitation popup hides by itself after 10 seconds (owner of the value).
const AUTO_HIDE_SEC: float = 10.0
const PORTRAIT_SCALE: float = 1.1

var service: PartyService = null
var invitation: InvitationModel = null
## Seconds the popup has been shown (tests drive `advance()`).
var shown_sec: float = 0.0
var _thumb: AvatarThumb = null
var _title: Label = null
var _detail: Label = null
var _time: Label = null
var _accept: Button = null
var _decline: Button = null


func _init() -> void:
	name = "InvitationToast"
	theme_type_variation = &"FormCard"
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


## Shows `item` (the newest invitation replaces an older popup).
func present(item: InvitationModel, party_service: PartyService) -> void:
	invitation = item
	service = party_service
	shown_sec = 0.0
	var look: AvatarConfigModel = item.inviter_avatar_model()
	_thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	_accept.disabled = false
	_decline.disabled = false
	_render()
	UiBuild.size_buttons(self)
	show()


func dismiss() -> void:
	hide()
	invitation = null


## Advances the auto-hide timer and the countdown by `delta` seconds.
func advance(delta: float) -> void:
	if not visible or invitation == null:
		return
	shown_sec += delta
	if shown_sec >= AUTO_HIDE_SEC or (service != null and invitation.is_expired(service.now_ms())):
		dismiss()
		return
	_render_time()


func title_text() -> String:
	return _title.text


func time_text() -> String:
	return _time.text


func accept_button() -> Button:
	return _accept


func decline_button() -> Button:
	return _decline


## "4:59" (minutes:seconds) of the time left.
static func clock_text(seconds: int) -> String:
	return "%d:%02d" % [floori(seconds / 60.0), seconds % 60]


func _process(delta: float) -> void:
	advance(delta)


func _render() -> void:
	if invitation == null:
		return
	_title.text = tr("ui.party.invite.received").format({"nickname": invitation.inviter_nickname})
	var detail: String = invitation.party_name
	if invitation.member_count > 0:
		detail = tr("ui.party.invite.member_count").format({"count": invitation.member_count})
	_detail.text = detail
	_detail.visible = not detail.is_empty()
	_render_time()


func _render_time() -> void:
	var left: int = invitation.seconds_left(service.now_ms()) if service != null and invitation != null else 0
	_time.text = tr("ui.party.invite.time_left").format({"time": clock_text(left)})
	_time.visible = left > 0


func _on_accept_pressed() -> void:
	if service == null or invitation == null:
		return
	var item: InvitationModel = invitation
	_accept.disabled = true
	_decline.disabled = true
	var result: ApiResult = await service.accept(item)
	dismiss()
	if result.ok:
		open_requested.emit({})
	elif result.code == PartyService.CODE_ALREADY_IN_PARTY:
		open_requested.emit({"view": "invitations", "id": item.invitation_id, "switch": true})


func _on_decline_pressed() -> void:
	if service == null or invitation == null:
		return
	var item: InvitationModel = invitation
	dismiss()
	await service.reject(item)


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func _build() -> void:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	add_child(box)
	var top: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(top)
	_thumb = AvatarThumb.new()
	_thumb.standalone = true
	_thumb.name = "Portrait"
	var side: float = UiBuild.target() * PORTRAIT_SCALE
	_thumb.custom_minimum_size = Vector2(side, side)
	_thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	top.add_child(_thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	top.add_child(texts)
	_title = UiBuild.data_label("", &"HeadingLabel")
	_title.name = "Title"
	texts.add_child(_title)
	_detail = UiBuild.data_label("", &"MutedLabel")
	_detail.name = "Detail"
	texts.add_child(_detail)
	var time_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	texts.add_child(time_row)
	var clock: TextureRect = UiBuild.icon_rect("clock", 0.75)
	clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	time_row.add_child(clock)
	_time = UiBuild.data_label("", &"MutedLabel", false)
	_time.name = "TimeLeft"
	time_row.add_child(_time)
	var buttons: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	buttons.alignment = BoxContainer.ALIGNMENT_END
	box.add_child(buttons)
	_decline = UiBuild.button("ui.party.invite.decline", "cancel", &"SecondaryButton")
	_decline.name = "Decline"
	_decline.pressed.connect(_on_decline_pressed)
	buttons.add_child(_decline)
	_accept = UiBuild.button("ui.party.invite.accept", "check")
	_accept.name = "Accept"
	_accept.pressed.connect(_on_accept_pressed)
	buttons.add_child(_accept)
