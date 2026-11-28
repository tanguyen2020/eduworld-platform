extends GdUnitTestSuite
## T022, T025 (F112-AC-18…AC-24, AC-26; TC-SOC-004, TC-SOC-006, TC-SOC-007, TC-EVT-009): "Quản lý club" is on the page of
## an active club the student owns only (never for an officer / member or a pending / locked club); it opens the four
## sections; "Phó nhóm" lists the members (nickname, role — nothing else) with one action per row except the owner's, each
## asked once; the edit form saves, a moderation refusal keeps the form with a general sentence; the goal form offers the
## three metrics by picture + words and a target with −/+; the event form offers four kinds (no internal tournament), a
## time in the past greys "Tạo sự kiện", times read as day + hour (no countdown); a success is a 2-second toast; one red
## "✕ Đóng".

var s: F112SceneHarness = null
var screen: FriendsScreen = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)


func after_test() -> void:
	if screen.visible:
		screen.close()
	s.dispose()


func _page(club_id: String) -> ClubPage:
	screen.open({"tab": "clubs", "id": club_id})
	await s.settle(14)
	return screen.club_view().page_view()


func _manage() -> ClubManage:
	var page: ClubPage = await _page(F111Fixtures.CLUB_OWN)
	page.action_button(ClubPage.ACT_MANAGE).pressed.emit()
	await s.settle(10)
	return screen.club_manage()


func test_manage_button_only_for_the_owner_of_an_active_club() -> void:
	var page: ClubPage = await _page(F111Fixtures.CLUB_OWN)
	assert_bool(page.action_button(ClubPage.ACT_MANAGE).visible).is_true()
	for spec: Array in [[F111Fixtures.CLUB_SCHOOL, "active", "member"], [F111Fixtures.CLUB_SCHOOL, "active", "officer"],
			[F111Fixtures.CLUB_PENDING, "pending_approval", "owner"], [F111Fixtures.CLUB_LOCKED, "locked", "owner"]]:
		s.h.http.clear(F112Fixtures.ROUTE_CLUB).reply(F112Fixtures.ROUTE_CLUB, 200, F111Fixtures.club(String(spec[0]), "Club",
				"interest_group", String(spec[1]), String(spec[2])))
		page = await _page(String(spec[0]))
		assert_bool(page.action_button(ClubPage.ACT_MANAGE).visible).override_failure_message(str(spec)).is_false()


func test_officers_list_one_action_per_row_never_the_owner() -> void:
	var manage: ClubManage = await _manage()
	assert_bool(manage.visible).is_true()
	manage.select_section(ClubManage.SECTION_OFFICERS)
	await s.settle(2)
	var owner_id: String = s.self_id
	assert_object(manage.role_button(owner_id)).is_null()
	var officer: Button = manage.role_button(F111Fixtures.FRIEND_ONLINE)
	assert_str(officer.text).is_equal("ui.club.manage.officer.remove")
	var member: Button = manage.role_button(F111Fixtures.MEMBER_OTHER)
	assert_str(member.text).is_equal("ui.club.manage.officer.make")
	member.pressed.emit()
	await s.settle(1)
	assert_bool(manage.confirm_visible()).is_true()
	var before: int = s.h.http.count(F112Fixtures.ROUTE_MEMBER_PATCH)
	await manage.confirm_role()
	await s.settle(2)
	assert_int(s.h.http.count(F112Fixtures.ROUTE_MEMBER_PATCH)).is_equal(before + 1)
	var all: String = " ".join(manage.all_texts()).to_lower()
	for w: String in ["trường", "school", "khối", "grade", "lớp", "điểm", "mastery"]:
		assert_bool(all.contains(w)).override_failure_message("'%s' in Phó nhóm" % w).is_false()


func test_edit_saves_and_moderation_keeps_the_form() -> void:
	var manage: ClubManage = await _manage()
	manage.description_field().text = "Giới thiệu mới của club"
	manage.description_field().text_changed.emit()
	assert_bool(manage.submit_button().disabled).is_false()
	manage.submit_button().pressed.emit()
	await s.settle(4)
	assert_str(manage.toast_text()).is_equal(TranslationServer.translate("ui.club.manage.saved.club"))
	s.h.http.clear(F112Fixtures.ROUTE_CLUB_PATCH).reply(F112Fixtures.ROUTE_CLUB_PATCH, 451,
			F112Fixtures.openapi("updateClub.451"))
	manage.name_field().text = "Tên thử nghiệm"
	manage.name_field().text_changed.emit("Tên thử nghiệm")
	manage.submit_button().pressed.emit()
	await s.settle(4)
	assert_str(manage.error_text()).is_equal(TranslationServer.translate("ui.club.form.blocked"))
	assert_str(manage.error_text()).not_contains("Tên thử nghiệm")
	assert_str(manage.name_field().text).is_equal("Tên thử nghiệm")
	# Typing past 500 characters is blocked.
	manage.description_field().text = "a".repeat(510)
	manage.description_field().text_changed.emit()
	assert_int(manage.description_field().text.length()).is_equal(ClubModel.DESCRIPTION_MAX)


func test_goal_form_metrics_target_and_toast() -> void:
	var manage: ClubManage = await _manage()
	manage.select_section(ClubManage.SECTION_GOAL)
	await s.settle(2)
	assert_array(Array(manage.metric_ids())).is_equal(["activities_completed", "quests_completed", "discoveries_found"])
	manage.set_target(0)
	assert_str(manage.target_text()).is_equal("1")
	manage.set_target(30)
	manage.submit_button().pressed.emit()
	await s.settle(4)
	assert_str(manage.toast_text()).is_equal(TranslationServer.translate("ui.club.manage.saved.goal"))
	var all: String = " ".join(manage.all_texts()).to_lower()
	for w: String in ["xếp hạng", "mua", "đóng góp của"]:
		assert_bool(all.contains(w)).override_failure_message("'%s'" % w).is_false()


func test_event_form_kinds_past_time_and_day_hour() -> void:
	var manage: ClubManage = await _manage()
	manage.now_source = func() -> int: return s.now_sec
	manage.select_section(ClubManage.SECTION_EVENT)
	await s.settle(2)
	assert_array(Array(manage.kind_ids())).is_equal(["practice_session", "coop_quest", "boss_raid", "other"])
	manage.set_event({"title": "Luyện tập chung", "day_offset": 0, "hour": 0, "minute": 0})
	assert_bool(manage.submit_button().disabled).is_true()
	manage.set_event({"day_offset": 5, "hour": 9, "minute": 0})
	assert_bool(manage.submit_button().disabled).is_false()
	var starts: int = int(manage.event_form()["starts_unix"])
	assert_str(manage.when_text()).contains(LiveOpsTime.ict_date(starts))
	assert_str(manage.when_text()).contains(LiveOpsTime.ict_time(starts))
	manage.submit_button().pressed.emit()
	await s.settle(4)
	assert_str(manage.toast_text()).override_failure_message("error: %s" % manage.error_text()).is_equal(
			TranslationServer.translate("ui.club.manage.saved.event"))
	await s.settle(1)
	# The toast is gone after 2 s.
	var until: int = Time.get_ticks_msec() + 2600
	while Time.get_ticks_msec() < until and not manage.toast_text().is_empty():
		await get_tree().process_frame
	assert_str(manage.toast_text()).is_empty()


func test_one_red_close() -> void:
	var manage: ClubManage = await _manage()
	var closes: Array[Node] = manage.find_children("Close", "Button", true, false)
	assert_int(closes.size()).is_equal(1)
	assert_str(String((closes[0] as Button).theme_type_variation)).is_equal("DangerButton")
