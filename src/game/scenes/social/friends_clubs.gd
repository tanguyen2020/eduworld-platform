class_name FriendsClubs
extends RefCounted
## The "Club" tab of the friends screen (F111 US5, US6): wires ClubTab / ClubPage / ClubCreateDialog / ClubInvitePicker and
## the club chat window (ChatDock on the club's `team` channel) to ClubService — the friends screen owns the window, the
## notice banner and the one confirmation sheet ("Rời club?"). Glue only: every request goes through ClubService
## (AGENTS R1); the role and the school policy only decide what is SHOWN.

var screen: FriendsScreen = null
var tab: ClubTab = null
var form: ClubCreateDialog = null
var picker: ClubInvitePicker = null
var chat_window: Control = null
var chat_dock: ChatDock = null
var clubs: ClubService = null
var _pending_open: String = ""


func _init(owner: FriendsScreen) -> void:
	screen = owner
	tab = ClubTab.new()
	tab.open_requested.connect(open_club)
	tab.join_requested.connect(_on_join)
	tab.invitation_answered.connect(_on_invitation)
	tab.create_requested.connect(_on_create)
	tab.page.back_requested.connect(_on_back)
	tab.page.action_requested.connect(_on_action)
	tab.page.request_answered.connect(_on_request)
	form = ClubCreateDialog.new()
	form.name = "ClubCreateDialog"
	form.submitted.connect(_on_submit)
	picker = ClubInvitePicker.new()
	picker.name = "ClubInvitePicker"
	picker.send_requested.connect(_on_invite)


## Adds the dialogs over the screen (after its own children so they cover it).
func attach() -> void:
	screen.add_child(form)
	screen.add_child(picker)


func bind(service: ClubService) -> void:
	unbind()
	clubs = service
	if clubs == null:
		return
	for pair: Array in _pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)


func unbind() -> void:
	if clubs == null:
		return
	for pair: Array in _pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	clubs = null


## Coroutine: the lists of the tab, then the club of a deep link (`{tab: "clubs", id}`).
func load_tab(open_id: String = "") -> void:
	if clubs == null:
		return
	if Uuid.is_valid(open_id):
		_pending_open = open_id
	await clubs.load_lists(AppState.grade_band)
	if not _pending_open.is_empty() and clubs != null:
		var id: String = _pending_open
		_pending_open = ""
		await open_club(id)


func render() -> void:
	if clubs == null:
		return
	var store: ClubStore = clubs.store
	var open: bool = store.page != null
	tab.show_page_mode(open)
	if open:
		tab.page.show_page(store, clubs.requests_denied, screen.services != null and screen.services.chat != null)
	else:
		tab.show_lists(store, UiBuild.band(), AppState.social_policy.can_show_create() and not clubs.creation_closed)


## Coroutine: the page of `club_id`.
func open_club(club_id: String) -> void:
	if clubs != null:
		await clubs.open_club(club_id)


func close() -> void:
	form.hide()
	picker.hide()
	_close_chat()
	if clubs != null:
		clubs.store.close_page()


## The leave confirmation was accepted (the screen's sheet).
func confirm_leave(club_id: String) -> void:
	if clubs != null:
		await clubs.leave(club_id)


func _pairs() -> Array:
	return [[clubs.store.clubs_changed, render], [clubs.store.page_changed, render], [clubs.notice_requested, screen.show_notice],
		[clubs.busy_changed, _on_busy], [clubs.club_created, _on_created], [clubs.page_closed, _on_page_closed]]


func _on_busy(busy: bool) -> void:
	form.set_busy(busy)
	picker.set_busy(busy)
	tab.page.set_busy(busy)


func _on_join(club_id: String) -> void:
	if clubs != null:
		await clubs.request_join(club_id)


func _on_invitation(invitation_id: String, accept: bool) -> void:
	if clubs == null:
		return
	for inv: ClubInvitationModel in clubs.store.invitations:
		if inv.invitation_id == invitation_id:
			if accept:
				await clubs.accept_invitation(inv)
			else:
				await clubs.decline_invitation(inv)
			return


func _on_create() -> void:
	form.open_form(AppState.social_policy.needs_approval())


func _on_submit(values: Dictionary) -> void:
	if clubs == null:
		return
	var result: ApiResult = await clubs.create(values)
	if result.ok or result.code == ClubService.CODE_DENIED:
		form.close()
	elif result.code == ClubService.CODE_BLOCKED:
		# AGENTS R15: a general sentence, never the matched word; the form stays to be fixed.
		form.show_error("ui.club.form.blocked")
	elif not result.network_error:
		form.show_error(KeyedWriteService.error_key(result.code))


func _on_created(club: ClubModel) -> void:
	await open_club(club.club_id)


func _on_back() -> void:
	if clubs != null:
		clubs.store.close_page()


func _on_page_closed(_club_id: String) -> void:
	render()


func _on_action(action: StringName, club_id: String) -> void:
	if clubs == null or clubs.store.page == null:
		return
	var club: ClubModel = clubs.store.page
	match action:
		ClubPage.ACT_JOIN:
			await clubs.request_join(club_id)
		ClubPage.ACT_LEAVE:
			screen.ask_leave_club(club)
		ClubPage.ACT_INVITE:
			var friends: Array[FriendModel] = screen.social.store.friends if screen.social != null else [] as Array[FriendModel]
			picker.open_for(club_id, friends)
		ClubPage.ACT_CHAT:
			_open_chat(club)


func _on_request(request_id: String, accept: bool) -> void:
	if clubs == null:
		return
	for r: ClubJoinRequestModel in clubs.store.requests:
		if r.request_id == request_id:
			if accept:
				await clubs.accept_request(r)
			else:
				await clubs.reject_request(r)
			return


func _on_invite(club_id: String, ids: PackedStringArray) -> void:
	if clubs == null:
		return
	var result: Dictionary = await clubs.invite(club_id, ids)
	picker.show_result(result)
	var sent: int = (result["ok_ids"] as PackedStringArray).size()
	if sent > 0:
		screen.show_notice("ui.club.invite_sent_count", {"count": LocaleFormat.number(sent)})


## F111-AC-24: the club's `team` channel in a window over the friends screen (the F119 ChatDock, expanded).
func _open_chat(club: ClubModel) -> void:
	if club.chat_channel_id.is_empty() or screen.services == null or screen.services.chat == null:
		return
	if chat_window == null:
		chat_window = Control.new()
		chat_window.name = "ClubChat"
		chat_window.set_anchors_preset(Control.PRESET_FULL_RECT)
		chat_window.mouse_filter = Control.MOUSE_FILTER_STOP
		screen.add_child(chat_window)
		var ui: Dictionary = UiBuild.window(chat_window, "ui.club.chat", "chat")
		(ui["close"] as Button).pressed.connect(_close_chat)
		chat_dock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
		UiBuild.expand(chat_dock, true, true)
		(ui["body"] as VBoxContainer).add_child(chat_dock)
		chat_dock.bind_services(screen.services.chat, screen.services.social, screen.services.reports)
	chat_window.visible = true
	chat_window.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	chat_dock.show_club(club.club_id, club.chat_channel_id)
	chat_dock.expand()


func _close_chat() -> void:
	if chat_window != null:
		if chat_dock != null:
			chat_dock.clear_context()
		chat_window.visible = false
