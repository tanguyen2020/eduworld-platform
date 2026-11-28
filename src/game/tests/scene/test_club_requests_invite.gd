extends GdUnitTestSuite
## T035 (F111-AC-25, F111-AC-26; SOCIAL_SYSTEM §6.2, AC-SOC-004, AGENTS R16): an owner / officer sees the "Yêu cầu"
## section (`listClubJoinRequests`) with "Chấp nhận" (`acceptClubJoinRequest`, Idempotency-Key kept on retry) and "Từ
## chối" (`rejectClubJoinRequest`); a member does not; `403 PERMISSION_DENIED` hides the buttons and reads the page
## again. "Mời bạn" picks only from the friends list (no name typed), at most 50, `inviteToClub` with a key; a friend the
## server did not invite (scope, block) reads "Chưa mời được" with no reason.

var s: F111SceneHarness = null
var screen: FriendsScreen = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")
	await s.services.social.load_friends()
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)


func after_test() -> void:
	if screen.visible:
		screen.close()
	s.dispose()


func _open(club: Dictionary) -> ClubPage:
	s.h.http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 200, club)
	screen.open({"tab": "clubs", "id": String(club["club_id"])})
	await s.settle(12)
	return screen.club_view().page_view()


func test_requests_only_for_owner_and_officer() -> void:
	var page: ClubPage = await _open(F111Fixtures.club(F111Fixtures.CLUB_SCHOOL, "CLB Toán", "school_club", "active", "member"))
	assert_bool(page.section_button(ClubPage.SECTION_REQUESTS).visible).is_false()
	assert_int(s.h.http.count(F111Fixtures.ROUTE_CLUB_REQUESTS)).is_equal(0)
	page = await _open(F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "officer"))
	assert_bool(page.section_button(ClubPage.SECTION_REQUESTS).visible).is_true()


func test_accept_keeps_its_key_and_reject() -> void:
	var page: ClubPage = await _open(F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner"))
	page.select_section(ClubPage.SECTION_REQUESTS)
	await s.settle(2)
	assert_array(Array(page.section_texts())).contains([TranslationServer.translate("ui.club.request_line").format(
			{"nickname": "ThoTrang03"})])
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_REQUEST).fail(F111Fixtures.ROUTE_ACCEPT_REQUEST)
	page.request_button(F111Fixtures.REQUEST_ID, true).pressed.emit()
	await s.settle(6)
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_ACCEPT_REQUEST).headers["idempotency-key"])
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_REQUEST).reply(F111Fixtures.ROUTE_ACCEPT_REQUEST, 200,
			F111Fixtures.member(F111Fixtures.FRIEND_OFFLINE, "ThoTrang03"))
	page.request_button(F111Fixtures.REQUEST_ID, true).pressed.emit()
	await s.settle(6)
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_ACCEPT_REQUEST).headers["idempotency-key"])).is_equal(key)
	assert_object(page.request_button(F111Fixtures.REQUEST_ID, true)).is_null()
	page.request_button(F111Fixtures.REQUEST_ID_2, false).pressed.emit()
	await s.settle(6)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_REJECT_REQUEST)).is_equal(1)


func test_forbidden_hides_the_buttons_and_reads_the_page_again() -> void:
	var page: ClubPage = await _open(F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "officer"))
	page.select_section(ClubPage.SECTION_REQUESTS)
	await s.settle(2)
	var reads: int = s.h.http.count(F111Fixtures.ROUTE_CLUB)
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_REQUEST).reply(F111Fixtures.ROUTE_ACCEPT_REQUEST, 403, AuthHarness.problem(403, "PERMISSION_DENIED"))
	s.h.http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 200,
			F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "member"))
	page.request_button(F111Fixtures.REQUEST_ID, true).pressed.emit()
	await s.settle(12)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_CLUB)).is_greater(reads)
	assert_bool(page.section_button(ClubPage.SECTION_REQUESTS).visible).is_false()


func test_invite_only_friends_at_most_50_and_failed_ones_without_reason() -> void:
	var page: ClubPage = await _open(F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner"))
	page.action_button(ClubPage.ACT_INVITE).pressed.emit()
	await s.settle(2)
	var picker: ClubInvitePicker = screen.invite_picker()
	assert_bool(picker.visible).is_true()
	assert_array(Array(picker.candidate_ids())).contains_exactly([F111Fixtures.FRIEND_ONLINE, "0193b0a0-0000-7000-8000-0000000000a4",
		F111Fixtures.FRIEND_OFFLINE])
	assert_bool(picker.find_children("*", "LineEdit", true, false).is_empty()).is_true()
	assert_int(ClubInvitePicker.MAX_PICK).is_equal(50)
	picker.toggle(F111Fixtures.FRIEND_ONLINE)
	picker.toggle(F111Fixtures.FRIEND_OFFLINE)
	picker.send_button().pressed.emit()
	await s.settle(8)
	var sent: FakeHttp.Recorded = s.h.http.last(F111Fixtures.ROUTE_INVITE)
	assert_str(String(sent.headers.get("idempotency-key", ""))).is_not_empty()
	assert_array(sent.body["invitee_ids"] as Array).contains_exactly([F111Fixtures.FRIEND_ONLINE, F111Fixtures.FRIEND_OFFLINE])
	assert_str(picker.result_text(F111Fixtures.FRIEND_ONLINE)).is_equal(TranslationServer.translate("ui.club.invite_done"))
	assert_str(picker.result_text(F111Fixtures.FRIEND_OFFLINE)).is_equal(TranslationServer.translate("ui.club.invite_failed"))
	# A whole refusal (scope / block): every picked friend reads the same general words.
	s.h.http.clear(F111Fixtures.ROUTE_INVITE).reply(F111Fixtures.ROUTE_INVITE, 403, AuthHarness.problem(403, "SOCIAL_SCOPE_VIOLATION"))
	picker.toggle("0193b0a0-0000-7000-8000-0000000000a4")
	picker.send_button().pressed.emit()
	await s.settle(8)
	assert_str(picker.result_text("0193b0a0-0000-7000-8000-0000000000a4")).is_equal(TranslationServer.translate("ui.club.invite_failed"))
	for w: String in ["chặn", "block", "phạm vi", "scope", "trường"]:
		assert_bool(" ".join(picker.texts()).to_lower().contains(w)).override_failure_message(w).is_false()
