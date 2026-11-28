extends GdUnitTestSuite
## T030 (F111-AC-20, F111-AC-24; SOCIAL_SYSTEM §6, AGENTS R16, ADR-0050): a club page shows `getClub`, the members of
## `listClubMembers` by nickname, portrait and club role only (no score, mastery, other member's contribution, real name
## or school), the club goal as a group bar + "Em đã góp {n}" and the calendar; "Rời club" asks once then calls
## `leaveClub`; "Chat club" opens the ChatDock on the club's `chat_channel_id` when the club is active; a `locked` /
## `archived` club is view only (no join, invite or chat); `404` closes the page with a general sentence.

const FORBIDDEN: PackedStringArray = ["điểm", "mastery", "thành thạo", "trường", "school", "khối", "grade", "lớp"]

var s: F111SceneHarness = null
var screen: FriendsScreen = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)


func after_test() -> void:
	if screen.visible:
		screen.close()
	s.dispose()


func _open(club_id: String) -> ClubPage:
	screen.open({"tab": "clubs", "id": club_id})
	await s.settle(12)
	return screen.club_view().page_view()


func test_members_goals_and_calendar_show_only_public_words() -> void:
	var page: ClubPage = await _open(F111Fixtures.CLUB_OWN)
	page.select_section(ClubPage.SECTION_MEMBERS)
	await s.settle(2)
	var members: PackedStringArray = page.section_texts()
	assert_array(Array(members)).contains(["MeoXanh42", F111Fixtures.FRIEND_NICK, "GauNau15",
		TranslationServer.translate("ui.club.role.officer")])
	page.select_section(ClubPage.SECTION_GOALS)
	await s.settle(2)
	var goals: PackedStringArray = page.section_texts()
	assert_array(Array(goals)).contains(["Cả club hoàn thành 30 bài", "12/30",
		TranslationServer.translate("ui.club.goal.mine").format({"n": "3"})])
	page.select_section(ClubPage.SECTION_EVENTS)
	await s.settle(2)
	assert_array(Array(page.section_texts())).contains(["Luyện tập Toán cuối tuần"])
	var all: String = " ".join(page.all_texts()).to_lower()
	for w: String in FORBIDDEN:
		assert_bool(all.contains(w)).override_failure_message("'%s' on the club page" % w).is_false()


func test_leave_asks_once_then_leaves() -> void:
	var page: ClubPage = await _open(F111Fixtures.CLUB_SCHOOL)
	page.action_button(ClubPage.ACT_LEAVE).pressed.emit()
	await s.settle(2)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_LEAVE)).is_equal(0)
	assert_bool(screen.confirm_sheet().visible).is_true()
	screen.confirm_sheet().chosen.emit(FriendsScreen.ACT_CONFIRM)
	await s.settle(6)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_LEAVE)).is_equal(1)
	assert_bool(screen.club_view().page_view().visible).is_false()


func test_chat_club_opens_the_dock_on_the_club_channel() -> void:
	var page: ClubPage = await _open(F111Fixtures.CLUB_OWN)
	var chat: Button = page.action_button(ClubPage.ACT_CHAT)
	assert_bool(chat.visible).is_true()
	chat.pressed.emit()
	await s.settle(4)
	var dock: ChatDock = screen.club_chat_dock()
	assert_object(dock).is_not_null()
	assert_str(String(dock.context_kind)).is_equal("club")
	assert_array(Array(dock.channel_ids)).contains([F111Fixtures.CLUB_CHANNEL])


func test_locked_club_is_view_only() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 200,
			F111Fixtures.club(F111Fixtures.CLUB_LOCKED, "CLB Cờ vua", "interest_group", "locked", "member", 8))
	var page: ClubPage = await _open(F111Fixtures.CLUB_LOCKED)
	assert_bool(page.read_only_visible()).is_true()
	for act: StringName in [ClubPage.ACT_CHAT, ClubPage.ACT_INVITE, ClubPage.ACT_JOIN]:
		assert_bool(page.action_button(act).visible).override_failure_message(String(act)).is_false()


func test_gone_club_closes_with_a_general_sentence() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 404, AuthHarness.problem(404, "NOT_FOUND"))
	var page: ClubPage = await _open(F111Fixtures.CLUB_OWN)
	assert_bool(page.visible).is_false()
	assert_str(screen.notice_text()).is_equal(TranslationServer.translate("ui.club.gone"))
