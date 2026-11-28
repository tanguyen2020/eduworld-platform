extends GdUnitTestSuite
## T032 (F111-AC-19; SOCIAL_SYSTEM §6, STUDENT_GAME_UX_SOCIAL §6): the "Club" tab of Bạn bè shows the pending invitations
## ("{nickname} mời em vào {club}" with Đồng ý / Từ chối), "Club của em" and — not for lớp 1–3 — "Club đề xuất"; a club
## card has its name, icon, type in words, "{n}/{max} thành viên" and its state in words (`pending_approval` → "Đang chờ
## nhà trường duyệt"); a suggested club has "Xin vào" (then "Đang chờ duyệt"). A notification deep link opens the club.

var s: F111SceneHarness = null
var screen: FriendsScreen = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	if screen != null and is_instance_valid(screen) and screen.visible:
		screen.close()
	screen = null
	s.dispose()


func _open(args: Dictionary = {"tab": "clubs"}) -> void:
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)
	screen.open(args)
	await s.settle(10)


func test_cards_invitations_and_states_in_words() -> void:
	await _open()
	var tab: ClubTab = screen.club_view()
	assert_bool(tab.is_visible_in_tree()).is_true()
	assert_array(Array(tab.section_keys())).is_equal(["ui.club.invitations", "ui.club.mine", "ui.club.discover"])
	var inv: PackedStringArray = tab.invitation_texts()
	assert_array(Array(inv)).contains([TranslationServer.translate("ui.club.invitation_line").format(
			{"nickname": "CaHeoVui07", "club": "CLB Âm nhạc"})])
	var pending: PackedStringArray = tab.card(F111Fixtures.CLUB_PENDING).texts()
	assert_array(Array(pending)).contains(["Đội Robot", TranslationServer.translate("ui.club.type.competition_team"),
		TranslationServer.translate("ui.club.status.pending_approval")])
	var school: PackedStringArray = tab.card(F111Fixtures.CLUB_SCHOOL).texts()
	assert_array(Array(school)).contains([TranslationServer.translate("ui.club.members_count").format({"count": "24", "max": "50"})])
	assert_object(tab.card(F111Fixtures.CLUB_STUDY)).is_null()
	var join: Button = tab.card(F111Fixtures.CLUB_DISCOVER).join_button()
	assert_bool(join.visible).is_true()
	join.pressed.emit()
	await s.settle(4)
	assert_array(Array(tab.card(F111Fixtures.CLUB_DISCOVER).texts())).contains([TranslationServer.translate("ui.club.request_pending")])


func test_g1_3_sees_only_its_clubs_and_invitations() -> void:
	AppState.set_grade_band("g1_3")
	await _open()
	var tab: ClubTab = screen.club_view()
	assert_array(Array(tab.section_keys())).is_equal(["ui.club.invitations", "ui.club.mine"])
	assert_bool(tab.create_button().visible).is_false()


func test_deep_link_opens_the_club_page() -> void:
	await _open({"tab": "clubs", "id": F111Fixtures.CLUB_OWN})
	await s.settle(6)
	var page: ClubPage = screen.club_view().page_view()
	assert_bool(page.is_visible_in_tree()).is_true()
	assert_str(page.club_id).is_equal(F111Fixtures.CLUB_OWN)
