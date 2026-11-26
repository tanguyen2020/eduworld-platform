extends GdUnitTestSuite
## T014, T016, T021, T024 (F102-AC-01, AC-03, AC-04, AC-06, AC-07, AC-09, AC-11, AC-12, AC-13, AC-14, AC-15, AC-16,
## AC-18; TC-PTY-001, -004, -005, -006, -008, -009, -014): the Party screen on PartySceneHarness (FakeHttp + contract
## fixtures, TEST-ONLY):
## - alone: Mây + one big "Tạo nhóm và mời bạn" button; creating opens the invite picker; HUD → create → friend tile
##   is three taps (SC-001); pending invitations show "Vào nhóm" / "Lần sau nhé"; an accept refused with
##   PARTY_ALREADY_IN_PARTY asks "Rời nhóm hiện tại và tham gia?" first;
## - in a party: one 2D card per member (no 3D node in the lobby), only the changed card is redrawn on a Ready change;
##   leader controls only for the leader, "Bắt đầu" grey with the reason in words, enabled when everyone is ready;
## - leave / kick / transfer / disband always through the confirmation (safe "Hủy" focused); "Thu nhỏ" keeps the party;
## - a member who drops fades with words; leader grace banner, then the new leader and the leader controls;
## - offline: the yellow banner, writes locked, the party still shown;
## - `party.session.started` → 3-2-1 → session room; `party.session.ended` → the shared outcome only;
## - a locale switch re-renders at once; a wide phone gets the one-row card layout.

const LOBBY: String = "res://scenes/social/PartyLobby.tscn"

var s: PartySceneHarness = null
var _locale: String = ""


func before_test() -> void:
	s = PartySceneHarness.new()
	_locale = String(Settings.locale)
	Settings.set_locale("vi-VN", false)


func after_test() -> void:
	Settings.set_locale(_locale, false)
	s.dispose()


func _frames(n: int = 6) -> void:
	for i: int in n:
		await get_tree().process_frame


func _lobby(args: Dictionary = {}) -> PartyLobby:
	var lobby: PartyLobby = auto_free((load(LOBBY) as PackedScene).instantiate()) as PartyLobby
	add_child(lobby)
	lobby.service = s.party
	lobby.open(args)
	await _frames()
	return lobby


# ---------- F102-AC-01, SC-001 ----------

func test_alone_shows_one_big_invite_button_and_creates_a_party() -> void:
	await s.start(true)
	var lobby: PartyLobby = await _lobby()
	assert_bool(lobby.is_alone()).is_true()
	var create: Button = lobby.create_button()
	assert_bool(create.is_visible_in_tree()).is_true()
	assert_object(create.icon).is_not_null()
	assert_str(String(create.theme_type_variation)).is_empty()
	assert_float(create.size.y).is_greater_equal(UiBuild.target())
	assert_bool(lobby.leave_button().visible).is_false()
	s.h.http.reply(PartyFixtures.ROUTE_CREATE, 201, PartyFixtures.party_alone(s.self_id))
	create.pressed.emit()
	await _frames(8)
	assert_bool(lobby.is_alone()).is_false()
	assert_int(lobby.card_count()).is_equal(1)
	assert_bool(lobby.picker().visible).is_true()
	# Third tap: a friend tile sends the invitation.
	await _frames(6)
	s.h.http.reply(PartyFixtures.ROUTE_INVITE, 201, PartyFixtures.openapi("PartyInvitation"))
	lobby.picker().tile(PartyFixtures.MEMBER).pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_INVITE)).is_equal(1)
	assert_str(lobby.picker().tile(PartyFixtures.MEMBER).text).contains(tr("ui.party.invite.invited"))


# ---------- F102-AC-06, AC-07 ----------

func test_cards_are_2d_and_only_the_changed_card_is_redrawn() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	assert_int(lobby.card_count()).is_equal(3)
	assert_array(lobby.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(lobby.find_children("*", "SubViewport", true, false)).is_empty()
	var waiting: PartyMemberCard = lobby.card(PartyFixtures.WAITING)
	var other: PartyMemberCard = lobby.card(PartyFixtures.MEMBER)
	assert_str(waiting.ready_text()).is_equal(tr("ui.party.ready_off"))
	assert_str(waiting.presence_text()).is_equal(tr("ui.party.presence.online"))
	assert_bool(lobby.card(s.self_id).shows_crown()).is_true()
	assert_bool(other.shows_crown()).is_false()
	var waiting_renders: int = waiting.renders
	var other_renders: int = other.renders
	var payload: Dictionary = PartyFixtures.ws("member.ready_changed.ready")
	s.push("party.member.ready_changed", payload)
	await _frames(1)
	assert_str(waiting.ready_text()).is_equal(tr("ui.party.ready_on"))
	assert_int(waiting.renders).is_equal(waiting_renders + 1)
	assert_int(other.renders).is_equal(other_renders)
	assert_str(String(waiting.find_child("Ready", true, false).get("theme_type_variation"))).is_equal("TileTagCorrect")


# ---------- F102-AC-09, AC-08 ----------

func test_leader_controls_and_the_start_reason() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	var start: Button = lobby.start_button()
	assert_bool(start.visible).is_true()
	assert_bool(start.disabled).is_true()
	assert_str(lobby.start_hint_text()).is_equal(tr("ui.party.start_block.one_not_ready"))
	assert_bool(lobby.choose_button().visible).is_true()
	assert_bool(lobby.card(PartyFixtures.MEMBER).menu_button().visible).is_true()
	assert_bool(lobby.card(s.self_id).menu_button().visible).is_false()
	assert_bool(lobby.settings_button().visible).is_true()
	# Everyone ready → Start enabled; pressing it asks the server (no local start).
	s.push("party.member.ready_changed", PartyFixtures.ws("member.ready_changed.ready"))
	await _frames(1)
	assert_bool(start.disabled).is_false()
	assert_str(lobby.start_hint_text()).is_empty()
	s.h.http.reply(PartyFixtures.ROUTE_START, 201, PartyFixtures.session_start())
	start.pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_START)).is_equal(1)


func test_members_see_no_leader_control() -> void:
	await s.start()
	s.as_member()
	var lobby: PartyLobby = await _lobby()
	assert_bool(lobby.start_button().visible).is_false()
	assert_bool(lobby.choose_button().visible).is_false()
	assert_bool(lobby.settings_button().visible).is_false()
	for id: String in [PartyFixtures.LEADER, PartyFixtures.WAITING]:
		assert_bool(lobby.card(id).menu_button().visible).is_false()
	assert_str(lobby.start_hint_text()).is_not_empty()
	# Ready goes to the server; the card changes once it confirmed (REST 204 while the socket is offline).
	s.h.http.reply(PartyFixtures.ROUTE_READY, 204)
	var me: PartyMemberCard = lobby.card(s.self_id)
	assert_str(me.ready_text()).is_equal(tr("ui.party.ready_on"))
	lobby.ready_button().pressed.emit()
	await _frames(6)
	assert_bool(bool(s.h.http.last(PartyFixtures.ROUTE_READY).body["ready"])).is_false()
	assert_str(me.ready_text()).is_equal(tr("ui.party.ready_off"))
	assert_str(tr(lobby.ready_button().text)).is_equal(tr("ui.party.ready"))


# ---------- F102-AC-12 ----------

func test_leave_kick_and_transfer_go_through_a_confirmation() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	var dlg: PartyDialog = lobby.dialog()
	lobby.leave_button().pressed.emit()
	await _frames(2)
	assert_bool(dlg.visible).is_true()
	assert_str(dlg.message_text()).is_equal(tr("ui.party.leave_confirm"))
	assert_str(String(dlg.safe_button().text)).is_equal("ui.common.cancel")
	assert_str(String(dlg.button(&"leave").theme_type_variation)).is_equal("DangerButton")
	assert_bool(dlg.safe_button().has_focus()).is_true()
	dlg.safe_button().pressed.emit()
	await _frames(2)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_LEAVE)).is_equal(0)
	# Kick: the leader menu, then the confirmation.
	lobby.card(PartyFixtures.WAITING).menu_button().pressed.emit()
	await _frames(2)
	dlg.button(&"kick").pressed.emit()
	await _frames(2)
	assert_str(dlg.message_text()).contains("GauNau15")
	s.h.http.reply(PartyFixtures.ROUTE_KICK, 204)
	dlg.button(&"kick_confirmed").pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_KICK)).is_equal(1)
	assert_object(lobby.card(PartyFixtures.WAITING)).is_null()
	# Transfer.
	lobby.card(PartyFixtures.MEMBER).menu_button().pressed.emit()
	await _frames(2)
	dlg.button(&"transfer").pressed.emit()
	await _frames(2)
	var moved: Dictionary = PartyFixtures.party(s.self_id)
	moved["leader_id"] = PartyFixtures.MEMBER
	moved["state_version"] = 9
	s.h.http.reply(PartyFixtures.ROUTE_TRANSFER, 200, moved)
	dlg.button(&"transfer_confirmed").pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_TRANSFER)).is_equal(1)
	assert_bool(lobby.start_button().visible).is_false()
	# Leave, confirmed.
	s.h.http.reply(PartyFixtures.ROUTE_LEAVE, 204)
	lobby.leave_button().pressed.emit()
	await _frames(2)
	dlg.button(&"leave").pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_LEAVE)).is_equal(1)
	assert_bool(lobby.is_alone()).is_true()


func test_disband_from_the_settings_and_minimise_keeps_the_party() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	var closed: Array[bool] = []
	lobby.closed.connect(func() -> void: closed.append(true))
	lobby.minimize_button().pressed.emit()
	await _frames(1)
	assert_array(closed).is_equal([true])
	assert_bool(lobby.visible).is_false()
	assert_bool(s.party.store.has_party()).is_true()
	lobby.open({})
	await _frames()
	lobby.settings_button().pressed.emit()
	await _frames(2)
	var dlg: PartyDialog = lobby.dialog()
	dlg.button(&"disband").pressed.emit()
	await _frames(2)
	assert_str(dlg.message_text()).is_equal(tr("ui.party.disband_confirm"))
	s.h.http.reply(PartyFixtures.ROUTE_DISBAND, 204)
	dlg.button(&"disband_confirmed").pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_DISBAND)).is_equal(1)
	assert_bool(lobby.is_alone()).is_true()


# ---------- F102-AC-13, AC-14 ----------

func test_a_dropped_member_fades_with_words() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	s.push("party.member.status_changed", PartyFixtures.ws("member.status_changed.disconnected"))
	await _frames(1)
	var card: PartyMemberCard = lobby.card(PartyFixtures.MEMBER)
	assert_bool(card.is_faded()).is_true()
	assert_str(card.presence_text()).is_equal(tr("ui.party.presence.disconnected"))
	assert_str(card.ready_text()).is_empty()
	s.push("party.member.status_changed", PartyFixtures.ws("member.status_changed.online"))
	await _frames(1)
	assert_bool(card.is_faded()).is_false()


func test_leader_grace_banner_then_the_new_leader_gets_the_controls() -> void:
	await s.start()
	s.as_member()
	var lobby: PartyLobby = await _lobby()
	s.push("party.member.status_changed", {"party_id": PartyFixtures.PARTY_ID, "user_id": PartyFixtures.LEADER,
		"status": "disconnected"})
	await _frames(1)
	assert_str(lobby.banner_text()).is_equal(tr("ui.party.leader_reconnecting"))
	var moved: Dictionary = PartyFixtures.ws("leader.transferred.leader_disconnected")
	moved["to_user_id"] = s.self_id
	s.push("party.leader.transferred", moved)
	await _frames(2)
	assert_str(lobby.banner_text()).is_empty()
	assert_str(lobby.notice_text()).contains(tr("ui.party.new_leader").format({"nickname": ""}).strip_edges())
	assert_bool(lobby.start_button().visible).is_true()
	assert_bool(lobby.choose_button().visible).is_true()


# ---------- F102-AC-15 (screen) ----------

func test_offline_shows_the_yellow_banner_and_locks_writes() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	s.party.offline = true
	s.party.connection_changed.emit(true)
	await _frames(1)
	assert_str(lobby.banner_text()).is_equal(tr("ui.party.reconnecting"))
	assert_bool(lobby.ready_button().disabled).is_true()
	assert_bool(lobby.choose_button().disabled).is_true()
	assert_bool(lobby.invite_button().disabled).is_true()
	assert_int(lobby.card_count()).is_equal(3)


# ---------- F102-AC-11 ----------

func test_session_start_counts_down_then_enters_the_room_and_comes_back_with_the_outcome() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	s.push("party.session.started", PartyFixtures.ws("session.started.quest"))
	await _frames(1)
	assert_str(lobby.countdown_text()).is_equal("3")
	lobby.advance(1.0)
	assert_str(lobby.countdown_text()).is_equal("2")
	lobby.advance(1.0)
	assert_str(lobby.countdown_text()).is_equal("1")
	assert_array(Array(s.entered)).is_empty()
	lobby.advance(1.05)
	assert_array(Array(s.entered)).is_equal(["res://scenes/world/zones/learning_zone_math/Zone.tscn|0193b0a0-0000-7000-8000-000000000201"])
	assert_bool(lobby.visible).is_false()
	s.push("party.session.ended", PartyFixtures.ws("session.ended.completed"))
	lobby.open({"summary": {"outcome": "completed", "reason": "completed"}})
	await _frames(2)
	assert_str(lobby.notice_text()).is_equal(tr("ui.party.session.completed"))
	# Only the shared outcome: no number at all in the notice (never another student's score).
	assert_bool(RegEx.create_from_string("[0-9]").search(lobby.notice_text()) == null).is_true()


# ---------- F102-AC-03, AC-04 (screen) ----------

func test_pending_invitations_and_the_switch_question() -> void:
	await s.start(true)
	s.h.http.clear(PartyFixtures.ROUTE_INVITATIONS).reply(PartyFixtures.ROUTE_INVITATIONS, 200,
			PartyFixtures.invitations(s.party.now_ms()))
	var lobby: PartyLobby = await _lobby()
	assert_int(lobby.invitation_rows()).is_equal(1)
	var row: Control = lobby.find_child("Invite_*", true, false) as Control
	assert_str((row.find_child("Text", true, false) as Label).text).contains("MeoXanh42")
	assert_object(row.find_child("TimeLeft", true, false)).is_not_null()
	# Now in another party: the accept is refused and the screen asks first.
	s.h.http.clear(PartyFixtures.ROUTE_MY_PARTY).reply(PartyFixtures.ROUTE_MY_PARTY, 200, PartyFixtures.party(s.self_id))
	lobby.close()
	lobby.open({"view": "invitations", "id": PartyFixtures.INVITATION_ID, "switch": true})
	await _frames(8)
	assert_bool(lobby.dialog().visible).is_true()
	assert_str(lobby.dialog().message_text()).is_equal(tr("ui.party.switch_party_confirm"))
	assert_int(s.h.http.count(PartyFixtures.ROUTE_LEAVE)).is_equal(0)


func test_an_expired_invitation_from_the_bell_says_so() -> void:
	await s.start(true)
	var lobby: PartyLobby = await _lobby({"view": "invitations", "id": Uuid.v7()})
	await _frames(4)
	assert_str(lobby.notice_text()).is_equal(tr("ui.party.error.PARTY_INVITATION_EXPIRED"))


# ---------- F102-AC-16, AC-18 ----------

func test_locale_switch_re_renders_at_once() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	var title: Label = lobby.find_child("Title", true, false) as Label
	assert_str(title.text).is_equal("Nhóm: Nhóm Sao Băng")
	Settings.set_locale("en-US", false)
	await _frames(2)
	assert_str(title.text).is_equal("Party: Nhóm Sao Băng")
	assert_str(lobby.start_hint_text()).is_equal("1 friend is not ready yet")


func test_wide_phone_gets_one_row_of_cards_and_big_bottom_buttons() -> void:
	await s.start()
	var vp: SubViewport = auto_free(SubViewport.new()) as SubViewport
	vp.size = Vector2i(1558, 720)
	add_child(vp)
	var lobby: PartyLobby = (load(LOBBY) as PackedScene).instantiate() as PartyLobby
	lobby.theme = (AppState.theme_service if AppState.theme_service != null else ThemeService.new(AppState.grade_band)).current_theme
	vp.add_child(lobby)
	lobby.service = s.party
	lobby.open({})
	await _frames(6)
	assert_bool(lobby.is_phone_layout()).is_true()
	var members: BoxContainer = lobby.find_child("Members", true, false) as BoxContainer
	assert_bool(members.vertical).is_false()
	var bottom: Control = lobby.find_child("BottomBar", true, false) as Control
	assert_float(bottom.get_global_rect().position.y).is_greater(members.get_global_rect().end.y)
	assert_float(lobby.ready_button().size.y).is_greater_equal(UiBuild.target())
	assert_bool(lobby.chat_slot().visible).is_false()
	lobby.queue_free()


func test_older_bands_change_who_may_join_from_the_settings() -> void:
	await s.start()
	var band: String = AppState.grade_band
	AppState.set_grade_band("g6_7")
	var lobby: PartyLobby = await _lobby()
	lobby.settings_button().pressed.emit()
	await _frames(2)
	var dlg: PartyDialog = lobby.dialog()
	var friends_chip: Button = dlg.find_child("Chip_friends", true, false) as Button
	assert_object(friends_chip).is_not_null()
	assert_object(dlg.find_child("Chip_members", true, false)).is_not_null()
	friends_chip.button_pressed = true
	s.h.http.reply(PartyFixtures.ROUTE_SETTINGS, 200, PartyFixtures.party(s.self_id))
	dlg.button(&"save").pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_SETTINGS)).is_equal(1)
	assert_dict(s.h.http.last(PartyFixtures.ROUTE_SETTINGS).body).is_equal({"visibility": "friends"})
	AppState.set_grade_band(band)


# ---------- F119-AC-01 (party chat in the lobby) ----------

## F119-AC-01, TC-CHAT-017: the lobby mounts the always-on party chat at once (channel of the snapshot), with the other
## members for mentions / the players list; nothing in it turns the chat off.
func test_lobby_shows_the_party_chat_under_the_activity() -> void:
	await s.start()
	var lobby: PartyLobby = await _lobby()
	await _frames(10)
	var dock: ChatDock = lobby.chat_dock()
	assert_object(dock).is_not_null()
	assert_bool(lobby.chat_slot().visible).is_true()
	assert_bool(dock.is_visible_in_tree()).is_true()
	assert_str(dock.context_id).is_equal(PartyFixtures.PARTY_ID)
	assert_str(dock.current_channel).is_equal(ChatFixtures.PARTY_CHANNEL)
	assert_int(dock.members.size()).is_equal(2)
	for m: Dictionary in dock.members:
		assert_str(String(m["user_id"])).is_not_equal(s.self_id)
	assert_int(dock.list().entries.size()).is_equal(5)
	lobby.close()
	assert_str(dock.current_channel).is_empty()
