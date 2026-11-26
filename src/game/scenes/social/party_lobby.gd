class_name PartyLobby
extends Control
## Màn Nhóm — the Party screen (F102 US1–US4; PARTY_SYSTEM §5, STUDENT_GAME_UX_SOCIAL §5). A window like S15 Cài đặt
## (F114 tier 1 frame over the scrim, blue title bar):
## - Alone: Mây says "Mời bạn bè để cùng chơi nhé!" over one big "Tạo nhóm và mời bạn" button (≤ 3 taps from the HUD to
##   an invitation, SC-001) and the pending invitations with "Vào nhóm" / "Lần sau nhé" (F102-AC-01, AC-03).
## - In a party: the member cards (PartyMemberCard: portrait, nickname, level, presence and Ready in words, crown),
##   "+ Mời bạn" (InviteFriendPicker), the activity the leader chose (ActivitySelector for the leader, "Trưởng nhóm đang
##   chọn" for the others), the Ready toggle and — for the leader only — "Bắt đầu", grey with the reason in words while
##   something is missing (F102-AC-06…AC-10). PC: two columns; a wide phone: the cards in one row, the big buttons at the
##   bottom (F102-AC-18). The party chat (F119 ChatDock, channel `party` of the snapshot `chat_channel_id`, always-on —
##   ADR-0020 §6) sits under the activity on a PC; on a phone it slides in from the right over 45% of the window from the
##   "Chat nhóm" button of the bottom bar (with the unread badge), the Ready / Start buttons stay visible.
## - Leader menu on a card ("Chuyển trưởng nhóm", "Mời rời nhóm"), "Rời nhóm" (red) and the settings ("Giải tán nhóm")
##   always through a confirmation (F102-AC-12); "Thu nhỏ" closes the window and keeps the party (PartyHud on the HUD).
## - Banners: yellow "Đang kết nối lại…" while offline (writes locked, the party stays readable, AC-15) and "Trưởng nhóm
##   đang kết nối lại…" during the leader grace (AC-14); short notices for the server answers (AC-05), the new leader, an
##   activity change. `party.session.started` → countdown 3‑2‑1 then the session room (AC-11); back from a session the
##   shared outcome only, never another student's score.
## Every text is a `ui.party.*` key; nicknames are the only data of other students (AGENTS R16). UI only: PartyService
## and PartyStore do the work (AGENTS R1, F102-AC-20).

signal closed()

const VIEW_INVITATIONS: String = "invitations"
## Seconds a notice stays on screen (≥ 3 s reading time, STUDENT_GAME_UX §4).
const NOTICE_SEC: float = 4.0
## Start countdown 3-2-1 (STUDENT_GAME_UX_SOCIAL §5 "Bắt đầu thành công"), one second per step.
const COUNTDOWN_FROM: int = 3
## Width / height above which the window uses the phone layout (cards in one row, big buttons at the bottom).
const PHONE_ASPECT: float = 1.9
## Width of the big "Tạo nhóm và mời bạn" button, in touch targets.
const CREATE_WIDTH: float = 6.0
const SUMMARY_KEYS: Dictionary = {
	"completed": "ui.party.session.completed", "abandoned": "ui.party.session.abandoned",
	"failed": "ui.party.session.failed", "aborted": "ui.party.session.aborted",
}
const TYPE_KEYS: Dictionary = {
	"friend": "ui.party.type.friend", "study_party": "ui.party.type.study_party", "quest": "ui.party.type.quest",
	"competition_team": "ui.party.type.competition_team", "custom": "ui.party.type.custom",
}

var service: PartyService = null
var open_tween: Tween = null
var view: String = ""
## Invitation the screen was opened for (notification "Xem"); "" = none.
var focus_invitation: String = ""
## Seconds left of the 3-2-1 countdown (< 0 = none).
var countdown_left: float = -1.0
var notice_key: String = ""
var _notice_left: float = 0.0
var _rows_left: float = 1.0
var _cards: Dictionary = {}
var _window: PanelContainer = null
var _title: Label = null
var _type_tag: Label = null
var _settings: Button = null
var _minimize: Button = null
var _leave: Button = null
var _close: Button = null
var _banner: PanelContainer = null
var _banner_label: Label = null
var _notice: PanelContainer = null
var _notice_icon: TextureRect = null
var _notice_label: Label = null
var _state: Label = null
var _retry: Button = null
var _alone: VBoxContainer = null
var _create: Button = null
var _invitations_box: VBoxContainer = null
var _invitations: VBoxContainer = null
var _lobby: BoxContainer = null
var _members_title: Label = null
var _members_column: VBoxContainer = null
var _activity_column: VBoxContainer = null
var _activity_row: HBoxContainer = null
var _members_scroll: ScrollContainer = null
var _members: BoxContainer = null
var _invite: Button = null
var _activity_card: PanelContainer = null
var _activity_icon: TextureRect = null
var _activity_title: Label = null
var _activity_hint: Label = null
var _choose: Button = null
var _ineligible: VBoxContainer = null
var _chat_slot: Control = null
var _chat_dock: ChatDock = null
var _chat_overlay: Control = null
var _chat_button: Button = null
var _chat_badge: Label = null
var _bottom: HBoxContainer = null
var _ready_toggle: Button = null
var _start_hint: Label = null
var _start: Button = null
var _countdown: Control = null
var _countdown_label: Label = null
var _dialog: PartyDialog = null
var _dialogs: PartyLobbyDialogs = null
var _picker: InviteFriendPicker = null
var _selector: ActivitySelector = null
var _phone: bool = false


func _ready() -> void:
	visible = false
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)
	resized.connect(_on_resized)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	_unbind()


## WorldRoot: `ui_requested("party", args)` — args `{view: "invitations", id, switch}`, `{countdown: true}`,
## `{summary: {outcome, reason}}`.
func open(args: Dictionary = {}) -> void:
	if service == null and AppState.services != null:
		service = AppState.get_services().party
	_bind(service)
	view = String(args.get("view", ""))
	focus_invitation = String(args.get("id", ""))
	show()
	_apply_layout()
	open_tween = UiMotion.pop_in(_window, Settings.reduce_motion)
	_render_all()
	if args.get("summary") is Dictionary:
		_show_summary(args["summary"] as Dictionary)
	if bool(args.get("countdown", false)) or (service != null and not service.store.session.is_empty()
			and SceneRouter.party_room_id != String(service.store.session.get("room_id", ""))):
		start_countdown()
	_focus_default.call_deferred()
	refresh(bool(args.get("switch", false)))


func close() -> void:
	if _dialog.visible:
		_dialog.dismiss()
	_chat_dock.clear_context()
	_picker.hide()
	_selector.hide()
	hide()
	countdown_left = -1.0
	_countdown.visible = false
	_unbind()
	closed.emit()


## Coroutine: reads the party and the invitations (the screen shows what the store holds while it loads).
func refresh(ask_switch: bool = false) -> void:
	if service == null:
		return
	_set_state("ui.party.loading", false)
	var result: ApiResult = await service.load_my_party()
	await service.load_invitations()
	if not visible:
		return
	_set_state("" if result.ok or result.status == 404 else "ui.party.load_failed", not result.ok and result.status != 404)
	_render_all()
	var p: PartyModel = _party()
	if p != null and p.has_activity() and p.activity_title.is_empty():
		service.load_eligible()
	if not focus_invitation.is_empty():
		var inv: InvitationModel = service.store.invitation(focus_invitation)
		if inv == null:
			show_notice("ui.party.error.PARTY_INVITATION_EXPIRED", {}, true)
		elif ask_switch and service.store.has_party():
			_dialogs.ask_switch(inv)


# ---------- test / owner hooks ----------

func is_alone() -> bool:
	return _alone.visible


func card(student_id: String) -> PartyMemberCard:
	return _cards.get(student_id) as PartyMemberCard


func card_count() -> int:
	return _cards.size()


func ready_button() -> Button:
	return _ready_toggle


func start_button() -> Button:
	return _start


func start_hint_text() -> String:
	return _start_hint.text if _start_hint.visible else ""


func create_button() -> Button:
	return _create


func invite_button() -> Button:
	return _invite


func choose_button() -> Button:
	return _choose


func leave_button() -> Button:
	return _leave


func minimize_button() -> Button:
	return _minimize


func settings_button() -> Button:
	return _settings


func dialog() -> PartyDialog:
	return _dialog


func picker() -> InviteFriendPicker:
	return _picker


func selector() -> ActivitySelector:
	return _selector


func banner_text() -> String:
	return tr(_banner_label.text) if _banner.visible else ""


func notice_text() -> String:
	return _notice_label.text if _notice.visible else ""


func countdown_text() -> String:
	return _countdown_label.text if _countdown.visible else ""


func is_phone_layout() -> bool:
	return _phone


func chat_slot() -> Control:
	return _chat_slot


func chat_dock() -> ChatDock:
	return _chat_dock


func chat_button() -> Button:
	return _chat_button


func invitation_rows() -> int:
	return _invitations.get_child_count() if _invitations_box.visible else 0


## Shows a short notice (`key` + format args); `error` uses the red-free error panel with the alert icon.
func show_notice(key: String, args: Dictionary = {}, error: bool = false) -> void:
	notice_key = key
	_notice_left = NOTICE_SEC
	_notice.theme_type_variation = &"InlineErrorPanel" if error else &"InfoBannerPanel"
	_notice_label.theme_type_variation = &"InlineErrorLabel" if error else &"InfoBannerLabel"
	_notice_icon.texture = UiBuild.icon("alert" if error else "info")
	_notice_label.text = tr(key).format(args)
	_notice.visible = true


## F102-AC-11: 3-2-1 over the lobby, then PartyService enters the session room.
func start_countdown() -> void:
	countdown_left = float(COUNTDOWN_FROM)
	_countdown.visible = true
	_render_countdown()


## Advances the countdown and the notice timer by `delta` seconds (tests call it directly).
func advance(delta: float) -> void:
	_rows_left -= delta
	if _rows_left <= 0.0:
		# The time left of the pending invitations follows the server expires_at (once a second).
		_rows_left = 1.0
		if service != null and _invitations_box.visible:
			_render_invitations()
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false
	if countdown_left < 0.0:
		return
	countdown_left -= delta
	if countdown_left > 0.0:
		_render_countdown()
		return
	countdown_left = -1.0
	_countdown.visible = false
	if service != null and service.enter_session() == OK:
		close()


func _process(delta: float) -> void:
	if visible:
		advance(delta)
		if _chat_overlay.visible:
			_place_chat_overlay()


func _unhandled_input(event: InputEvent) -> void:
	if not visible or not event.is_action_pressed("ui_cancel"):
		return
	if _dialog.visible or _picker.visible or _selector.visible or countdown_left >= 0.0:
		return
	get_viewport().set_input_as_handled()
	close()


# ---------- binding ----------

func _bind(next: PartyService) -> void:
	_unbind()
	service = next
	_dialogs.service = next
	if service == null:
		return
	var s: PartyStore = service.store
	s.party_updated.connect(_on_party_updated)
	s.member_updated.connect(_on_member_updated)
	s.invitations_changed.connect(_render_invitations)
	s.session_started.connect(_on_session_started)
	s.session_ended.connect(_show_summary)
	s.roster_changed.connect(_on_roster_changed)
	service.notice_requested.connect(_on_notice_requested)
	service.connection_changed.connect(_on_connection_changed)
	service.busy_changed.connect(_on_busy_changed)
	service.eligible_changed.connect(_on_eligible_changed)


func _unbind() -> void:
	if service == null:
		return
	var s: PartyStore = service.store
	for pair: Array in [[s.party_updated, _on_party_updated], [s.member_updated, _on_member_updated],
			[s.invitations_changed, _render_invitations], [s.session_started, _on_session_started],
			[s.session_ended, _show_summary], [s.roster_changed, _on_roster_changed],
			[service.notice_requested, _on_notice_requested], [service.connection_changed, _on_connection_changed],
			[service.busy_changed, _on_busy_changed], [service.eligible_changed, _on_eligible_changed]]:
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)


# ---------- rendering ----------

func _render_all() -> void:
	_render_frame()
	_sync_cards()
	_render_activity()
	_render_controls()
	_render_invitations()
	_render_banner()
	_render_chat()
	UiBuild.size_buttons(self)


func _party() -> PartyModel:
	return service.store.party if service != null else null


func _render_frame() -> void:
	var p: PartyModel = _party()
	var in_party: bool = p != null
	_title.text = tr("ui.party.title").format({"name": p.name}) if in_party else tr("ui.party.title_alone")
	_title.tooltip_text = _title.text
	_type_tag.visible = in_party
	_type_tag.text = String(TYPE_KEYS.get(p.party_type, "ui.party.type.friend")) if in_party else ""
	_settings.visible = in_party and service.store.is_leader()
	_minimize.visible = in_party
	_leave.visible = in_party
	_close.visible = not in_party
	_alone.visible = not in_party
	_lobby.visible = in_party
	_bottom.visible = in_party


## Keeps one card per member: new members get a card, gone members lose theirs; the others are refreshed in place.
func _sync_cards() -> void:
	var p: PartyModel = _party()
	var keep: Dictionary = {}
	if p != null:
		var leader: bool = service.store.is_leader()
		for m: PartyMemberModel in p.members:
			keep[m.student_id] = true
			var c: PartyMemberCard = _cards.get(m.student_id) as PartyMemberCard
			if c == null:
				c = PartyMemberCard.new()
				c.menu_requested.connect(_on_member_menu)
				_members.add_child(c)
				_cards[m.student_id] = c
			c.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			c.bind(m, m.student_id == service.store.self_id, leader and m.student_id != service.store.self_id)
	for id: Variant in _cards.keys():
		if not keep.has(id):
			var gone: PartyMemberCard = _cards[id] as PartyMemberCard
			_cards.erase(id)
			_members.remove_child(gone)
			gone.queue_free()
	if p != null:
		_members_title.text = tr("ui.party.members").format({"count": p.members.size(), "max": p.max_members})
		_invite.visible = service.can_invite()
		_members.move_child(_invite, _members.get_child_count() - 1)


func _render_activity() -> void:
	var p: PartyModel = _party()
	if p == null:
		return
	var leader: bool = service.store.is_leader()
	if p.has_activity():
		var title: String = L10n.pick(p.activity_title)
		var kind: String = tr(String({"explore": "ui.party.activity.kind.explore", "quest": "ui.party.activity.kind.quest",
			"boss": "ui.party.activity.kind.boss"}.get(p.activity_kind, "ui.party.activity.kind.other")))
		# The title comes with the eligible list (the REST party has only the ref): the kind words until then, never the
		# technical ref.
		_activity_title.text = title if not title.is_empty() else kind
		_activity_icon.texture = UiBuild.icon(String(EligibleActivityModel.KIND_ICONS.get(p.activity_kind, "star")))
		_activity_hint.text = kind if not title.is_empty() else ""
	else:
		_activity_title.text = tr("ui.party.activity.none")
		_activity_icon.texture = UiBuild.icon("help")
		_activity_hint.text = tr("ui.party.activity.leader_choosing") if not leader else tr("ui.party.activity.choose_hint")
	_choose.visible = leader
	_choose.text = "ui.party.activity.change" if p.has_activity() else "ui.party.activity.choose"
	UiBuild.clear(_ineligible)
	for e: Dictionary in service.store.ineligible:
		var m: PartyMemberModel = p.member(String(e["student_id"]))
		var panel: PanelContainer = UiBuild.panel(&"InlineErrorPanel")
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		panel.add_child(row)
		row.add_child(UiBuild.icon_rect("alert"))
		var text: Label = UiBuild.data_label(tr("ui.party.not_eligible").format({
			"nickname": m.nickname if m != null else tr("ui.party.someone"),
			"reason": EligibleActivityModel.reason_text(String(e["reason_code"]), String(e["requirement"]))}),
			&"InlineErrorLabel")
		UiBuild.expand(text)
		row.add_child(text)
		_ineligible.add_child(panel)


func _render_controls() -> void:
	var p: PartyModel = _party()
	if p == null:
		return
	var me: PartyMemberModel = service.store.me()
	var locked: bool = service.is_offline() or service.is_busy() or p.is_in_activity()
	var ready_now: bool = me != null and me.ready
	_ready_toggle.theme_type_variation = &"ChoiceChip" if ready_now else &""
	_ready_toggle.toggle_mode = true
	_ready_toggle.set_pressed_no_signal(ready_now)
	_ready_toggle.text = "ui.party.ready_done" if ready_now else "ui.party.ready"
	_ready_toggle.icon = UiBuild.icon("check")
	_ready_toggle.tooltip_text = "ui.party.ready_undo" if ready_now else "ui.party.ready_tooltip"
	_ready_toggle.disabled = locked or me == null
	var leader: bool = service.store.is_leader()
	_start.visible = leader
	_choose.disabled = locked or p.leader_reconnecting
	_invite.disabled = locked
	_settings.disabled = service.is_offline()
	_leave.disabled = service.is_offline()
	var block: Dictionary = service.start_block_key()
	_start.disabled = locked or not block.is_empty()
	# The white play sign is for the blue face; a disabled (light) button shows the blue one so it stays readable.
	_start.icon = UiBuild.icon("play" if _start.disabled else "play_light")
	_start_hint.visible = not block.is_empty()
	if not block.is_empty():
		_start_hint.text = tr(String(block["key"])).format(block["args"] as Dictionary)
	if not leader and (block.is_empty() or p.leader_reconnecting):
		# A member: what the leader will do (the leader grace already has its banner).
		_start_hint.visible = true
		_start_hint.text = tr("ui.party.wait_leader")


func _render_invitations() -> void:
	if service == null:
		return
	UiBuild.clear(_invitations)
	var list: Array[InvitationModel] = service.store.invitations
	for inv: InvitationModel in list:
		_invitations.add_child(PartyForms.invitation_row(inv, service, _on_accept_pressed, _on_decline_pressed))
	var alone: bool = not service.store.has_party()
	_invitations_box.visible = not list.is_empty() and (alone or view == VIEW_INVITATIONS)
	UiBuild.size_buttons(_invitations_box)


func _render_banner() -> void:
	var p: PartyModel = _party()
	var key: String = ""
	if service != null and service.is_offline():
		key = "ui.party.reconnecting"
	elif p != null and p.leader_reconnecting:
		key = "ui.party.leader_reconnecting"
	_banner_label.text = key
	_banner.visible = not key.is_empty()


func _render_countdown() -> void:
	_countdown_label.text = str(maxi(1, ceili(countdown_left)))


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _focus_default() -> void:
	if not visible:
		return
	if _alone.visible:
		_create.grab_focus(true)
	elif _ready_toggle.visible and not _ready_toggle.disabled:
		_ready_toggle.grab_focus(true)
	else:
		_minimize.grab_focus(true)


func _show_summary(info: Dictionary) -> void:
	var key: String = String(SUMMARY_KEYS.get(String(info.get("outcome", "")), "ui.party.session.ended"))
	show_notice(key)


# ---------- signals of the store / service ----------

func _on_party_updated(_party_model: PartyModel) -> void:
	if visible:
		_render_all()


## Only the card of `student_id` is redrawn (F102-AC-06, AC-UX-104); the Start hint and the banners follow.
func _on_member_updated(student_id: String) -> void:
	if not visible:
		return
	var c: PartyMemberCard = _cards.get(student_id) as PartyMemberCard
	if c != null:
		c.refresh()
	_render_controls()
	_render_banner()


func _on_roster_changed(student_id: String, joined: bool) -> void:
	var p: PartyModel = _party()
	var m: PartyMemberModel = p.member(student_id) if p != null else null
	if joined and m != null:
		# Screen readers read the notice (NFR-A11Y-001: thông báo vào/ra được đọc).
		show_notice("ui.party.member_joined", {"nickname": m.nickname})
	elif not joined:
		show_notice("ui.party.member_left_notice")


func _on_session_started(_info: Dictionary) -> void:
	if visible:
		start_countdown()


func _on_notice_requested(key: String, args: Dictionary) -> void:
	if visible:
		show_notice(key, args, key.begins_with("ui.party.error."))


func _on_connection_changed(_offline: bool) -> void:
	if visible:
		_render_banner()
		_render_controls()
		_render_invitations()


func _on_eligible_changed() -> void:
	if visible:
		_render_activity()


func _on_busy_changed(_busy: bool) -> void:
	if visible:
		_render_controls()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render_all()


func _on_resized() -> void:
	if visible:
		_apply_layout()


# ---------- intents ----------

func _on_create_pressed() -> void:
	_create.disabled = true
	var result: ApiResult = await service.create_party()
	_create.disabled = false
	if result.ok and visible:
		_open_picker()


func _open_picker() -> void:
	_picker.open(service)


func _on_choose_pressed() -> void:
	_selector.open(service, UiBuild.band())


func _on_ready_pressed() -> void:
	var me: PartyMemberModel = service.store.me()
	if me == null:
		return
	_ready_toggle.disabled = true
	await service.set_ready(not me.ready)
	if visible:
		_render_controls()


func _on_start_pressed() -> void:
	_start.disabled = true
	await service.start_session()
	if visible:
		_render_controls()


func _on_accept_pressed(inv: InvitationModel) -> void:
	var result: ApiResult = await service.accept(inv)
	if result.code == PartyService.CODE_ALREADY_IN_PARTY:
		_dialogs.ask_switch(inv)
	elif result.ok:
		view = ""
		_render_all()


func _on_decline_pressed(inv: InvitationModel) -> void:
	await service.reject(inv)


func _on_leave_pressed() -> void:
	_dialogs.ask_leave()


func _on_settings_pressed() -> void:
	if _party() != null:
		_dialogs.ask_settings(_party(), UiBuild.band())


func _on_member_menu(student_id: String) -> void:
	var p: PartyModel = _party()
	var m: PartyMemberModel = p.member(student_id) if p != null else null
	if m != null and service.store.is_leader():
		_dialogs.ask_member(m)


func _on_dialogs_finished(action: StringName, ok: bool) -> void:
	if action == &"switch" and ok:
		view = ""
	if visible:
		_render_all()


# ---------- party chat (F119) ----------

## The dock follows the party: the `party` channel of the snapshot and the other members (mentions, players list).
func _render_chat() -> void:
	var p: PartyModel = _party()
	var has_chat: bool = p != null and Uuid.is_valid(p.chat_channel_id)
	if not has_chat:
		_chat_dock.clear_context()
	else:
		var others: Array[Dictionary] = []
		for m: PartyMemberModel in p.members:
			if m.student_id != service.store.self_id:
				others.append({"user_id": m.student_id, "nickname": m.nickname, "avatar_config": m.avatar_config})
		_chat_dock.show_party(p.party_id, p.chat_channel_id, others)
	_arrange_columns(has_chat and not _phone)
	_chat_slot.visible = has_chat and not _phone
	_chat_button.visible = has_chat and _phone
	_chat_overlay.visible = has_chat and _phone and _chat_dock.mode != ChatDock.Mode.COLLAPSED


## PC with a party chat: members + activity on the left, the chat on the whole right column (STUDENT_GAME_UX_SOCIAL
## §5 wireframe); without chat (or on a phone) the F102 two columns members | activity.
func _arrange_columns(chat_column: bool) -> void:
	var target: Control = _members_column if chat_column else _lobby
	if _activity_column.get_parent() != target:
		_activity_column.get_parent().remove_child(_activity_column)
		target.add_child(_activity_column)
		if not chat_column:
			_lobby.move_child(_activity_column, _members_column.get_index() + 1)
	_activity_column.size_flags_vertical = Control.SIZE_FILL if chat_column else Control.SIZE_EXPAND_FILL


## PC: the dock in the right column, open. Phone: the dock in the right overlay, closed until "Chat nhóm".
func _place_chat() -> void:
	var target: Control = _chat_overlay if _phone else _chat_slot
	if _chat_dock.get_parent() != target:
		_chat_dock.get_parent().remove_child(_chat_dock)
		target.add_child(_chat_dock)
	_chat_dock.external_toggle = _phone
	_chat_dock.set_mode(ChatDock.Mode.COLLAPSED if _phone else ChatDock.Mode.EXPANDED)
	_chat_dock._apply_mode()
	if _party() != null:
		_render_chat()


## The phone overlay: right 45% of the window, between the title bar and the Ready / Start bar.
func _place_chat_overlay() -> void:
	var win: Rect2 = _window.get_global_rect()
	var top: float = (_window.find_child("TitleBar", true, false) as Control).get_global_rect().end.y
	var bottom: float = _bottom.get_global_rect().position.y if _bottom.visible else win.end.y
	var pad: float = float(UiTokens.SIZE_SPACE_12)
	var w: float = win.size.x * 0.45
	var origin: Vector2 = get_global_rect().position
	_chat_overlay.position = Vector2(win.end.x - w - pad * 2.0, top + pad) - origin
	_chat_overlay.size = Vector2(w, maxf(bottom - top - pad * 2.0, UiBuild.target() * 2.0))


func _on_chat_mode_changed(_mode: int) -> void:
	if _chat_button == null:
		return
	_chat_overlay.visible = _phone and _chat_dock.mode != ChatDock.Mode.COLLAPSED and _chat_button.visible
	if _chat_overlay.visible:
		_place_chat_overlay()


func _on_chat_unread(count: int, mentions: int) -> void:
	if _chat_badge == null:
		return
	_chat_badge.text = ("@ " if mentions > 0 else "") + (str(count) if count <= 99 else tr("ui.chat.unread_many"))
	_chat_badge.visible = count > 0 or mentions > 0


# ---------- layout ----------

## PC / tablet: two columns. Wide phone (F113 landscape, aspect > 1.9): one row of cards, big buttons at the bottom.
func _apply_layout() -> void:
	var view_size: Vector2 = get_viewport_rect().size if is_inside_tree() else size
	var phone: bool = view_size.y > 0.0 and view_size.x / view_size.y > PHONE_ASPECT
	if phone == _phone and _lobby != null and _lobby.get_meta("built", false):
		return
	_phone = phone
	_lobby.set_meta("built", true)
	_place_chat()
	_lobby.vertical = phone
	_members.vertical = not phone
	_members_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO if phone else ScrollContainer.SCROLL_MODE_DISABLED
	_members_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED if phone else ScrollContainer.SCROLL_MODE_AUTO
	_members_scroll.custom_minimum_size.y = UiBuild.target() * 1.9 if phone else 0.0
	_members_scroll.size_flags_vertical = Control.SIZE_FILL if phone else Control.SIZE_EXPAND_FILL
	# Phone: the card row keeps its height, the activity takes the rest (no empty band between them).
	_members_column.size_flags_vertical = Control.SIZE_FILL if phone else Control.SIZE_EXPAND_FILL
	# Phone: "Đổi hoạt động" sits in the activity row (no scrolling to reach it); PC: under the activity card.
	var target: Control = _activity_row if phone else _activity_column
	if _choose.get_parent() != target:
		_choose.get_parent().remove_child(_choose)
		target.add_child(_choose)
		if not phone:
			_activity_column.move_child(_choose, _activity_card.get_index() + 1)


func _build() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(UiBuild.scrim())
	_window = UiBuild.panel()
	_window.name = "Window"
	_window.set_anchors_preset(Control.PRESET_FULL_RECT)
	_window.anchor_left = 0.04
	_window.anchor_top = 0.04
	_window.anchor_right = 0.96
	_window.anchor_bottom = 0.96
	_window.offset_left = 0.0
	_window.offset_top = 0.0
	_window.offset_right = 0.0
	_window.offset_bottom = 0.0
	add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	_window.add_child(box)
	_build_title(box)
	_banner = UiBuild.panel(&"WarningBannerPanel")
	_banner.name = "Banner"
	_banner.visible = false
	box.add_child(_banner)
	var banner_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_banner.add_child(banner_row)
	var banner_icon: TextureRect = UiBuild.icon_rect("wifi_off")
	banner_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	banner_row.add_child(banner_icon)
	_banner_label = UiBuild.label("", &"WarningBannerLabel")
	_banner_label.name = "BannerLabel"
	UiBuild.expand(_banner_label)
	banner_row.add_child(_banner_label)
	_notice = UiBuild.panel(&"InfoBannerPanel")
	_notice.name = "Notice"
	_notice.visible = false
	box.add_child(_notice)
	var notice_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_notice.add_child(notice_row)
	_notice_icon = UiBuild.icon_rect("info")
	_notice_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	notice_row.add_child(_notice_icon)
	_notice_label = UiBuild.data_label("", &"InfoBannerLabel")
	_notice_label.name = "NoticeLabel"
	UiBuild.expand(_notice_label)
	notice_row.add_child(_notice_label)
	var tools: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(tools)
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	_state.visible = false
	UiBuild.expand(_state)
	tools.add_child(_state)
	_retry = UiBuild.button("ui.common.retry", "rotate_right", &"SecondaryButton")
	_retry.name = "Retry"
	_retry.visible = false
	_retry.pressed.connect(refresh)
	tools.add_child(_retry)
	var body: VBoxContainer = UiBuild.vbox(&"FormBox")
	body.name = "Body"
	UiBuild.expand(body, true, true)
	# The body scrolls when it cannot fit (large text on a phone); the Ready / Start bar stays visible below it.
	var body_scroll: ScrollContainer = ScrollContainer.new()
	body_scroll.name = "BodyScroll"
	body_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(body_scroll, true, true)
	box.add_child(body_scroll)
	body_scroll.add_child(body)
	_build_invitations(body)
	_build_alone(body)
	_build_lobby(body)
	_build_bottom(box)
	_chat_overlay = Control.new()
	_chat_overlay.name = "ChatOverlay"
	_chat_overlay.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_chat_overlay.visible = false
	add_child(_chat_overlay)
	_build_countdown()
	_dialog = PartyDialog.new()
	add_child(_dialog)
	_dialogs = PartyLobbyDialogs.new(_dialog)
	_dialogs.finished.connect(_on_dialogs_finished)
	_picker = InviteFriendPicker.new()
	add_child(_picker)
	_selector = ActivitySelector.new()
	add_child(_selector)


func _build_title(box: VBoxContainer) -> void:
	var bar: PanelContainer = UiBuild.panel(&"WindowTitleBar")
	bar.name = "TitleBar"
	box.add_child(bar)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("group")
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	_title = UiBuild.data_label("", &"WindowTitleLabel", false)
	_title.name = "Title"
	_title.uppercase = true
	# A long party name / translation wraps onto a second line (never cut).
	_title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_title)
	row.add_child(_title)
	# The buttons flow onto a second line when a translation is long; the title keeps half of the bar.
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.name = "Buttons"
	buttons.alignment = FlowContainer.ALIGNMENT_END
	buttons.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(buttons)
	row.add_child(buttons)
	_settings = UiBuild.button("", "settings", &"IconButton")
	_settings.name = "Settings"
	_settings.tooltip_text = "ui.party.settings.title"
	_settings.pressed.connect(_on_settings_pressed)
	buttons.add_child(_settings)
	_minimize = UiBuild.button("ui.party.minimize", "arrow_down", &"SecondaryButton")
	_minimize.name = "Minimize"
	_minimize.tooltip_text = "ui.party.minimize_tooltip"
	_minimize.pressed.connect(close)
	buttons.add_child(_minimize)
	_leave = UiBuild.button("ui.party.leave", "exit_light", &"DangerButton")
	_leave.name = "Leave"
	_leave.pressed.connect(_on_leave_pressed)
	buttons.add_child(_leave)
	_close = UiBuild.button("ui.common.close", "cross", &"DangerButton")
	_close.name = "Close"
	_close.pressed.connect(close)
	buttons.add_child(_close)


func _build_invitations(body: VBoxContainer) -> void:
	_invitations_box = UiBuild.vbox(&"FieldBox")
	_invitations_box.name = "InvitationsBox"
	_invitations_box.visible = false
	body.add_child(_invitations_box)
	var tag: Label = UiBuild.label("ui.party.invitations", &"SectionTag", false)
	tag.uppercase = true
	tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_invitations_box.add_child(tag)
	_invitations = UiBuild.vbox(&"FieldBox")
	_invitations.name = "Invitations"
	_invitations_box.add_child(_invitations)


func _build_alone(body: VBoxContainer) -> void:
	_alone = UiBuild.vbox(&"FormBox")
	_alone.name = "Alone"
	_alone.alignment = BoxContainer.ALIGNMENT_CENTER
	UiBuild.expand(_alone, true, true)
	body.add_child(_alone)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	var coach_row: Control = coach["row"] as Control
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.label("ui.party.alone", &"SpeechBubbleLabel"))
	(coach_row.get_node("Bubble") as Control).size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_alone.add_child(coach_row)
	_create = UiBuild.button("ui.party.create_and_invite", "invite_light")
	_create.name = "Create"
	_create.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	# "nút Mời lớn" (STUDENT_GAME_UX_SOCIAL §5): wider than a normal button, the band touch target high.
	_create.custom_minimum_size.x = UiBuild.target() * CREATE_WIDTH
	_create.pressed.connect(_on_create_pressed)
	_alone.add_child(_create)


func _build_lobby(body: VBoxContainer) -> void:
	_lobby = BoxContainer.new()
	_lobby.name = "Lobby"
	_lobby.theme_type_variation = &"ToolbarBox"
	UiBuild.expand(_lobby, true, true)
	body.add_child(_lobby)
	var left: VBoxContainer = UiBuild.vbox(&"FieldBox")
	_members_column = left
	left.name = "MembersColumn"
	UiBuild.expand(left, true, true)
	left.size_flags_stretch_ratio = 1.3
	_lobby.add_child(left)
	var tags: HFlowContainer = HFlowContainer.new()
	tags.name = "MembersTags"
	left.add_child(tags)
	_members_title = UiBuild.data_label("", &"SectionTag", false)
	_members_title.name = "MembersTitle"
	_members_title.uppercase = true
	tags.add_child(_members_title)
	_type_tag = UiBuild.label("", &"SectionTag", false)
	_type_tag.name = "TypeTag"
	_type_tag.uppercase = true
	tags.add_child(_type_tag)
	_members_scroll = ScrollContainer.new()
	_members_scroll.name = "MembersScroll"
	UiBuild.expand(_members_scroll, true, true)
	left.add_child(_members_scroll)
	_members = BoxContainer.new()
	_members.name = "Members"
	_members.vertical = true
	_members.theme_type_variation = &"ToolbarBox"
	UiBuild.expand(_members)
	_members_scroll.add_child(_members)
	_invite = UiBuild.button("ui.party.invite", "invite", &"SecondaryButton")
	_invite.name = "Invite"
	_invite.pressed.connect(_open_picker)
	_members.add_child(_invite)
	var right: VBoxContainer = UiBuild.vbox(&"FieldBox")
	right.name = "ActivityColumn"
	_activity_column = right
	UiBuild.expand(right, true, true)
	_lobby.add_child(right)
	var tag: Label = UiBuild.label("ui.party.activity.section", &"SectionTag", false)
	tag.uppercase = true
	tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	right.add_child(tag)
	_activity_card = UiBuild.panel(&"PartyMemberCard")
	_activity_card.name = "ActivityCard"
	right.add_child(_activity_card)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_activity_row = row
	_activity_card.add_child(row)
	_activity_icon = UiBuild.icon_rect("help", 1.4)
	_activity_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_activity_icon)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	row.add_child(texts)
	_activity_title = UiBuild.data_label("", &"HeadingLabel")
	_activity_title.name = "ActivityTitle"
	texts.add_child(_activity_title)
	_activity_hint = UiBuild.data_label("", &"MutedLabel")
	_activity_hint.name = "ActivityHint"
	texts.add_child(_activity_hint)
	_choose = UiBuild.button("ui.party.activity.choose", "map", &"SecondaryButton")
	_choose.name = "Choose"
	_choose.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_choose.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_choose.pressed.connect(_on_choose_pressed)
	right.add_child(_choose)
	_ineligible = UiBuild.vbox(&"FieldBox")
	_ineligible.name = "Ineligible"
	right.add_child(_ineligible)
	# F119: the party chat panel (always-on channel `party`, ADR-0020 §6) under the activity on a PC.
	_chat_slot = Control.new()
	_chat_slot.name = "EmbeddedChat"
	_chat_slot.visible = false
	_chat_slot.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_chat_slot.custom_minimum_size.y = UiBuild.target() * 4.0
	UiBuild.expand(_chat_slot, true, true)
	# The chat column is as wide as the members column (the smile board keeps its tabs on one line).
	_chat_slot.size_flags_stretch_ratio = 1.3
	_lobby.add_child(_chat_slot)
	_chat_dock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	_chat_dock.mode_changed.connect(_on_chat_mode_changed)
	_chat_dock.unread_changed.connect(_on_chat_unread)
	_chat_slot.add_child(_chat_dock)


func _build_bottom(box: VBoxContainer) -> void:
	_bottom = UiBuild.hbox(&"ToolbarBox")
	_bottom.name = "BottomBar"
	box.add_child(_bottom)
	_ready_toggle = UiBuild.button("ui.party.ready", "check")
	_ready_toggle.name = "ReadyToggle"
	_ready_toggle.pressed.connect(_on_ready_pressed)
	_bottom.add_child(_ready_toggle)
	# F119 phone layout: "Chat nhóm" opens the chat overlay (the badge counts what arrived meanwhile).
	_chat_button = UiBuild.button("ui.party.chat", "chat", &"SecondaryButton")
	_chat_button.name = "ChatButton"
	_chat_button.visible = false
	_chat_button.pressed.connect(func() -> void:
		_chat_dock.expand()
		_place_chat_overlay())
	_bottom.add_child(_chat_button)
	_chat_badge = UiBuild.data_label("", &"CountBadge", false)
	_chat_badge.name = "ChatBadge"
	_chat_badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_chat_badge.visible = false
	_bottom.add_child(_chat_badge)
	_start_hint = UiBuild.data_label("", &"MutedLabel")
	_start_hint.name = "StartHint"
	_start_hint.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	_start_hint.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_start_hint)
	_bottom.add_child(_start_hint)
	_start = UiBuild.button("ui.party.start", "play_light")
	_start.name = "StartButton"
	_start.pressed.connect(_on_start_pressed)
	_bottom.add_child(_start)


func _build_countdown() -> void:
	_countdown = Control.new()
	_countdown.name = "Countdown"
	_countdown.set_anchors_preset(Control.PRESET_FULL_RECT)
	_countdown.mouse_filter = Control.MOUSE_FILTER_STOP
	_countdown.visible = false
	add_child(_countdown)
	_countdown.add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_countdown.add_child(center)
	var card: PanelContainer = UiBuild.panel(&"FormCard")
	center.add_child(card)
	var col: VBoxContainer = UiBuild.vbox(&"FormBox")
	col.alignment = BoxContainer.ALIGNMENT_CENTER
	card.add_child(col)
	_countdown_label = UiBuild.data_label("3", &"CountdownLabel", false)
	_countdown_label.name = "Number"
	_countdown_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	col.add_child(_countdown_label)
	var words: Label = UiBuild.label("ui.party.countdown", &"HeadingLabel", false)
	words.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	col.add_child(words)
