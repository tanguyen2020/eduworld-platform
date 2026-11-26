extends GdUnitTestSuite
## T008, T009 (F102-AC-02, F102-AC-03; TC-PTY-002; spec edge case "lời mời khi đang làm bài"): the invite picker lists
## the friends of `listFriends` as picture tiles (2D portrait + nickname + presence words) exactly as the server sent
## them — no class / grade / school group, no field to type a name — and one tap invites; the invitation toast shows the
## inviter, the member count and the time left from the server `expires_at`, hides by itself after 10 s or at expiry,
## accepts / declines in place, never pops during an exam / challenge / activity nor with popups off; expired
## invitations leave the list; the bell row "Xem" of a `party_invitation` notification opens the Party screen.

const PICKER: String = "res://scenes/social/InviteFriendPicker.tscn"
const TOAST: String = "res://scenes/social/InvitationToast.tscn"
const HUD: String = "res://scenes/ui/hud/MainHud.tscn"

var s: PartySceneHarness = null
var _popups: bool = true


func before_test() -> void:
	s = PartySceneHarness.new()
	await s.start()
	_popups = Settings.notification_popups
	Settings.notification_popups = true
	await s.party.load_my_party()


func after_test() -> void:
	Settings.notification_popups = _popups
	s.dispose()


func _frames(n: int = 6) -> void:
	for i: int in n:
		await get_tree().process_frame


func _picker() -> InviteFriendPicker:
	var picker: InviteFriendPicker = auto_free((load(PICKER) as PackedScene).instantiate()) as InviteFriendPicker
	add_child(picker)
	picker.open(s.party)
	await _frames()
	return picker


# ---------- F102-AC-02 ----------

func test_picker_lists_the_server_friends_as_tiles_without_groups_or_text_field() -> void:
	var picker: InviteFriendPicker = await _picker()
	assert_int(picker.tile_count()).is_equal(3)
	var texts: PackedStringArray = picker.tile_texts()
	assert_str(texts[0]).starts_with("CaHeoVui07")
	assert_str(texts[1]).starts_with("SaoBang12")
	assert_str(texts[2]).starts_with("ThoTrang03")
	assert_bool(picker.has_text_field()).is_false()
	for word: String in ["lớp", "Lớp", "khối", "Khối", "trường", "Trường", "class", "grade", "school"]:
		assert_str(" ".join(picker.tile_texts())).not_contains(word)
	# Each tile is a picture tile with the 2D portrait (no 3D), tall enough to touch.
	var tile: Button = picker.tile(PartyFixtures.NEWCOMER)
	assert_str(String(tile.theme_type_variation)).is_equal("PictureTile")
	assert_object(tile.find_child("Thumb", true, false)).is_not_null()
	assert_float(tile.size.y).is_greater_equal(UiBuild.target())
	assert_array(picker.find_children("*", "Node3D", true, false)).is_empty()
	# A friend already in the party shows it, and a tap does not invite.
	assert_str(picker.tile(PartyFixtures.MEMBER).text).contains(tr("ui.party.invite.in_party"))
	picker.tile(PartyFixtures.MEMBER).pressed.emit()
	await _frames(3)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_INVITE)).is_equal(0)


func test_one_tap_invites_and_server_refusals_are_friendly() -> void:
	var picker: InviteFriendPicker = await _picker()
	s.h.http.reply(PartyFixtures.ROUTE_INVITE, 403, AuthHarness.problem(403, "PARTY_POLICY_VIOLATION"))
	var notices: Array[String] = []
	s.party.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))
	picker.tile(PartyFixtures.NEWCOMER).pressed.emit()
	await _frames(6)
	assert_array(notices).is_equal(["ui.party.error.PARTY_POLICY_VIOLATION"])
	assert_bool(picker.tile(PartyFixtures.NEWCOMER).button_pressed).is_false()
	s.h.http.clear(PartyFixtures.ROUTE_INVITE).reply(PartyFixtures.ROUTE_INVITE, 201, PartyFixtures.openapi("PartyInvitation"))
	picker.tile(PartyFixtures.NEWCOMER).pressed.emit()
	await _frames(6)
	var tile: Button = picker.tile(PartyFixtures.NEWCOMER)
	assert_bool(tile.button_pressed).is_true()
	assert_str(tile.text).contains(tr("ui.party.invite.invited"))


func test_empty_friend_list_shows_where_friends_come_from() -> void:
	s.h.http.clear(PartyFixtures.ROUTE_FRIENDS).reply(PartyFixtures.ROUTE_FRIENDS, 200,
			{"data": [], "page": {"next_cursor": null, "has_more": false, "limit": 100}})
	var picker: InviteFriendPicker = await _picker()
	assert_int(picker.tile_count()).is_equal(0)
	assert_bool((picker.find_child("Empty", true, false) as Control).visible).is_true()


# ---------- F102-AC-03 ----------

func _toast() -> InvitationToast:
	var toast: InvitationToast = auto_free((load(TOAST) as PackedScene).instantiate()) as InvitationToast
	add_child(toast)
	return toast


func test_toast_counts_down_from_the_server_expiry_and_hides_after_ten_seconds() -> void:
	var toast: InvitationToast = _toast()
	var inv: InvitationModel = InvitationModel.from_dict(PartyFixtures.invitation_ws(s.party.now_ms(), 300))
	toast.present(inv, s.party)
	await _frames(1)
	assert_bool(toast.visible).is_true()
	assert_str(toast.title_text()).contains("MeoXanh42")
	assert_str(toast.time_text()).contains("5:00")
	s.h.clock.advance(61.0)
	toast.advance(1.0)
	assert_str(toast.time_text()).contains("3:59")
	toast.advance(8.0)
	assert_bool(toast.visible).is_true()
	toast.advance(1.5)
	assert_bool(toast.visible).is_false()
	assert_str(String(toast.accept_button().theme_type_variation)).is_empty()
	assert_str(String(toast.decline_button().theme_type_variation)).is_equal("SecondaryButton")


func test_toast_hides_at_the_server_expiry_and_the_invitation_leaves_the_list() -> void:
	var payload: Dictionary = PartyFixtures.invitation_ws(s.party.now_ms(), 5)
	s.push("party.invitation.received", payload)
	assert_int(s.party.store.invitations.size()).is_equal(1)
	var toast: InvitationToast = _toast()
	toast.present(s.party.store.invitations[0], s.party)
	s.h.clock.advance(6.0)
	toast.advance(0.1)
	assert_bool(toast.visible).is_false()
	s.party.tick()
	assert_int(s.party.store.invitations.size()).is_equal(0)


func test_accept_and_decline_from_the_toast() -> void:
	s.h.http.clear(PartyFixtures.ROUTE_MY_PARTY).reply(PartyFixtures.ROUTE_MY_PARTY, 404, AuthHarness.problem(404, "PARTY_NOT_FOUND"))
	await s.party.load_my_party()
	s.push("party.invitation.received", PartyFixtures.invitation_ws(s.party.now_ms()))
	var toast: InvitationToast = _toast()
	var opened: Array[Dictionary] = []
	toast.open_requested.connect(func(args: Dictionary) -> void: opened.append(args))
	toast.present(s.party.store.invitations[0], s.party)
	s.h.http.reply(PartyFixtures.ROUTE_ACCEPT, 200, PartyFixtures.party(PartyFixtures.NEWCOMER))
	toast.accept_button().pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_ACCEPT)).is_equal(1)
	assert_array(opened).is_equal([{}])
	assert_bool(toast.visible).is_false()
	assert_bool(s.party.store.has_party()).is_true()
	# Decline of another invitation.
	var other: Dictionary = PartyFixtures.invitation_ws(s.party.now_ms())
	other["invitation_id"] = Uuid.v7()
	s.push("party.invitation.received", other)
	toast.present(s.party.store.invitation(String(other["invitation_id"])), s.party)
	s.h.http.reply(PartyFixtures.ROUTE_REJECT, 204)
	toast.decline_button().pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_REJECT)).is_equal(1)
	assert_object(s.party.store.invitation(String(other["invitation_id"]))).is_null()


func test_accept_while_in_another_party_opens_the_switch_question() -> void:
	s.push("party.invitation.received", PartyFixtures.invitation_ws(s.party.now_ms()))
	var toast: InvitationToast = _toast()
	var opened: Array[Dictionary] = []
	toast.open_requested.connect(func(args: Dictionary) -> void: opened.append(args))
	toast.present(s.party.store.invitations[0], s.party)
	s.h.http.reply(PartyFixtures.ROUTE_ACCEPT, 409, AuthHarness.problem(409, "PARTY_ALREADY_IN_PARTY"))
	toast.accept_button().pressed.emit()
	await _frames(6)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_LEAVE)).is_equal(0)
	assert_str(String(opened[0]["view"])).is_equal("invitations")
	assert_bool(bool(opened[0]["switch"])).is_true()


func test_hud_pops_the_toast_only_outside_exams_and_with_popups_on() -> void:
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var toast: InvitationToast = hud.call("invitation_toast") as InvitationToast
	var payload: Dictionary = PartyFixtures.invitation_ws(s.party.now_ms())
	s.push("party.invitation.received", payload)
	await runner.simulate_frames(1)
	assert_bool(toast.visible).is_true()
	var box: Rect2 = toast.get_global_rect()
	assert_float(box.end.x).is_less_equal(hud.get_viewport().get_visible_rect().size.x)
	toast.dismiss()
	# During an exam / a challenge / the activity player: only the bell (F123 inbox row), no popup.
	s.party.quiet_check = func() -> bool: return true
	var quiet: Dictionary = PartyFixtures.invitation_ws(s.party.now_ms())
	quiet["invitation_id"] = Uuid.v7()
	s.push("party.invitation.received", quiet)
	await runner.simulate_frames(1)
	assert_bool(toast.visible).is_false()
	# Popups off in Settings.
	s.party.quiet_check = Callable()
	Settings.notification_popups = false
	var off: Dictionary = PartyFixtures.invitation_ws(s.party.now_ms())
	off["invitation_id"] = Uuid.v7()
	s.push("party.invitation.received", off)
	await runner.simulate_frames(1)
	assert_bool(toast.visible).is_false()
	# The generic notification toast is not shown for party invitations (the invitation toast is the popup).
	assert_bool(s.services.notifications.should_popup(NotificationModel.from_dict({
		"notification_id": Uuid.v7(), "kind": "party_invitation", "message_code": "party.invitation_received",
		"created_at": "2026-11-21T08:00:00Z", "read": false}))).is_false()


func test_bell_row_of_a_party_invitation_opens_the_party_screen() -> void:
	var intents: Array = []
	var record: Callable = func(screen: StringName, args: Dictionary) -> void: intents.append([screen, args])
	EventBus.ui_requested.connect(record)
	var item: NotificationModel = NotificationModel.from_dict({"notification_id": Uuid.v7(), "kind": "party_invitation",
		"message_code": "party.invitation_received", "params": {"nickname": "MeoXanh42"}, "read": true,
		"action": {"kind": "open_party_invitation", "id": PartyFixtures.INVITATION_ID}, "created_at": "2026-11-21T08:00:00Z"})
	assert_bool(s.services.notifications.can_open(item)).is_true()
	assert_bool(s.services.notifications.open(item)).is_true()
	EventBus.ui_requested.disconnect(record)
	assert_array(intents).is_equal([[&"party", {"view": "invitations", "id": PartyFixtures.INVITATION_ID}]])
	assert_bool(UiScreenRegistry.has(&"party")).is_true()
