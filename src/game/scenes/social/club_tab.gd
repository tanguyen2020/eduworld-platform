class_name ClubTab
extends VBoxContainer
## The "Club" tab of Bạn bè (F111 US5, US6; SOCIAL_SYSTEM §6, STUDENT_GAME_UX_SOCIAL §6): "Tạo club" (only when the school
## policy allows it and the student is eligible — the server still decides), "Lời mời vào club" ("{nickname} mời em vào
## {club}" with Đồng ý / Từ chối), "Club của em" and — not for lớp 1–3 — "Club đề xuất" (ClubCard), and the ClubPage of
## the club opened (it takes the place of the lists; "Quay lại" comes back). UI only: the friends screen calls ClubService
## on the signals (AGENTS R1).

signal open_requested(club_id: String)
signal join_requested(club_id: String)
signal invitation_answered(invitation_id: String, accept: bool)
signal create_requested()

var page: ClubPage = null
var _lists: VBoxContainer = null
var _create: Button = null
var _invites_box: VBoxContainer = null
var _invites: VBoxContainer = null
var _mine_box: VBoxContainer = null
var _mine: VBoxContainer = null
var _discover_box: VBoxContainer = null
var _discover: VBoxContainer = null
var _cards: Dictionary = {}
var _invite_rows: Dictionary = {}


func _init() -> void:
	name = "ClubTab"
	theme_type_variation = &"FormBox"
	_lists = UiBuild.vbox(&"FormBox")
	_lists.name = "Lists"
	add_child(_lists)
	_create = UiBuild.button("ui.club.create", "plus")
	_create.name = "Create"
	_create.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_create.pressed.connect(func() -> void: create_requested.emit())
	_lists.add_child(_create)
	_invites_box = _section("Invitations", "ui.club.invitations", "invite")
	_invites = _invites_box.get_node("Rows") as VBoxContainer
	_mine_box = _section("Mine", "ui.club.mine", "flag")
	_mine = _mine_box.get_node("Rows") as VBoxContainer
	_discover_box = _section("Discover", "ui.club.discover", "search")
	_discover = _discover_box.get_node("Rows") as VBoxContainer
	page = ClubPage.new()
	page.visible = false
	add_child(page)


## `can_create`: policy allows it and the school did not refuse it this session.
func show_lists(store: ClubStore, band: String, can_create: bool) -> void:
	_create.visible = can_create and band != "g1_3"
	UiBuild.clear(_invites)
	_invite_rows.clear()
	for inv: ClubInvitationModel in store.invitations:
		_invites.add_child(_invitation_row(inv))
	_invites_box.visible = not store.invitations.is_empty()
	UiBuild.clear(_mine)
	_cards.clear()
	for c: ClubModel in store.mine:
		_mine.add_child(_card(c, store))
	if store.mine.is_empty() and store.loaded:
		_mine.add_child(FriendsUi.empty_line("ui.club.mine_none"))
	UiBuild.clear(_discover)
	for c: ClubModel in store.discoverable:
		_discover.add_child(_card(c, store))
	if store.discoverable.is_empty() and store.loaded:
		_discover.add_child(FriendsUi.empty_line("ui.club.discover_none"))
	_discover_box.visible = band != "g1_3"
	UiBuild.size_buttons(_lists)


## Shows the page (`open`) or the lists.
func show_page_mode(open: bool) -> void:
	page.visible = open
	_lists.visible = not open


func page_view() -> ClubPage:
	return page


func create_button() -> Button:
	return _create


func card(club_id: String) -> ClubCard:
	return _cards.get(club_id) as ClubCard


func invitation_button(invitation_id: String, accept: bool) -> Button:
	var pair: Array = _invite_rows.get(invitation_id, []) as Array
	return (pair[0] if accept else pair[1]) as Button if pair.size() == 2 else null


func invitation_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _invites.find_children("Nickname", "Label", true, false):
		out.append((node as Label).text)
	return out


## Title keys of the sections shown, in order.
func section_keys() -> PackedStringArray:
	var out: PackedStringArray = []
	for box: VBoxContainer in [_invites_box, _mine_box, _discover_box]:
		if box.visible:
			out.append((box.find_child("GroupTitle", true, false) as Label).text)
	return out


func _section(node_name: String, title_key: String, icon_name: String) -> VBoxContainer:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = node_name
	_lists.add_child(box)
	box.add_child(FriendsUi.group_title(title_key, icon_name))
	var rows: VBoxContainer = UiBuild.vbox(&"FormBox")
	rows.name = "Rows"
	box.add_child(rows)
	return box


func _card(c: ClubModel, store: ClubStore) -> ClubCard:
	var cc: ClubCard = ClubCard.new()
	cc.show_club(c, store.is_requested(c.club_id))
	cc.opened.connect(func(id: String) -> void: open_requested.emit(id))
	cc.join_requested.connect(func(id: String) -> void: join_requested.emit(id))
	_cards[c.club_id] = cc
	return cc


func _invitation_row(inv: ClubInvitationModel) -> Control:
	var ui: Dictionary = FriendsUi.person_card("", {}, "Invitation")
	var nick: Label = (ui["card"] as Control).find_child("Nickname", true, false) as Label
	nick.text = tr("ui.club.invitation_line").format({"nickname": inv.inviter_nickname, "club": inv.club_name})
	var actions: HFlowContainer = ui["actions"] as HFlowContainer
	var accept: Button = UiBuild.button("ui.club.accept", "check")
	accept.name = "Accept"
	accept.pressed.connect(func() -> void: invitation_answered.emit(inv.invitation_id, true))
	actions.add_child(accept)
	var decline: Button = UiBuild.button("ui.club.decline", "cancel", &"SecondaryButton")
	decline.name = "Decline"
	decline.pressed.connect(func() -> void: invitation_answered.emit(inv.invitation_id, false))
	actions.add_child(decline)
	_invite_rows[inv.invitation_id] = [accept, decline]
	return ui["card"] as Control
