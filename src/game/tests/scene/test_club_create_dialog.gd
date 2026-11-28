extends GdUnitTestSuite
## T031 (F111-AC-22, F111-AC-23; SOCIAL_SYSTEM §6.1, AGENTS R15, ADR-0041): "Tạo club" shows only when
## `policies.social.student_club_creation` is not `disabled` and `club_creation_eligible` is true; the form offers only
## `interest_group` / `competition_team`, a name of 2–64 characters, a description of at most 500 (counter, typing past it
## is blocked) and an icon of the library; `createClub` carries an Idempotency-Key; `pending_approval` → "Club đang chờ
## nhà trường duyệt", `active` → the club page; `451 CHAT_CONTENT_BLOCKED` → a general moderation sentence (no word
## echoed) and the form stays to be fixed; `403 PERMISSION_DENIED` → "Trường em chưa mở tạo club" and the button goes.

var s: F111SceneHarness = null
var screen: FriendsScreen = null
var _policy: SocialPolicyModel = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")
	_policy = AppState.social_policy
	AppState.social_policy = SocialPolicyModel.from_policies({"social": {"student_club_creation": "requires_approval",
		"club_creation_eligible": true}})


func after_test() -> void:
	AppState.social_policy = _policy
	if screen != null and is_instance_valid(screen) and screen.visible:
		screen.close()
	screen = null
	s.dispose()


func _open() -> ClubTab:
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)
	screen.open({"tab": "clubs"})
	await s.settle(10)
	return screen.club_view()


func test_button_follows_the_policy() -> void:
	for p: Array in [["disabled", true, false], ["requires_approval", false, false], ["auto", true, true],
			["requires_approval", true, true]]:
		AppState.social_policy = SocialPolicyModel.from_policies({"social": {"student_club_creation": p[0],
			"club_creation_eligible": p[1]}})
		var tab: ClubTab = await _open()
		assert_bool(tab.create_button().visible).override_failure_message(str(p)).is_equal(bool(p[2]))
		screen.close()


func test_form_limits_and_types() -> void:
	var tab: ClubTab = await _open()
	tab.create_button().pressed.emit()
	await s.settle(2)
	var form: ClubCreateDialog = screen.club_form()
	assert_bool(form.visible).is_true()
	assert_array(Array(form.type_ids())).is_equal(["interest_group", "competition_team"])
	form.set_name_text("A")
	assert_bool(form.submit_button().disabled).is_true()
	assert_str(form.error_text()).is_equal(TranslationServer.translate("ui.club.form.name_short"))
	form.set_name_text("x".repeat(80))
	assert_int(form.name_text().length()).is_equal(64)
	form.set_description_text("y".repeat(520))
	assert_int(form.description_text().length()).is_equal(500)
	assert_str(form.counter_text()).is_equal("500/500")
	assert_int(form.icon_ids().size()).is_greater_equal(4)
	form.set_name_text("Nhà thám hiểm")
	assert_bool(form.submit_button().disabled).is_false()


func test_create_pending_then_blocked_then_disabled() -> void:
	var tab: ClubTab = await _open()
	tab.create_button().pressed.emit()
	await s.settle(2)
	var form: ClubCreateDialog = screen.club_form()
	form.set_name_text("Đội Robot")
	form.choose_type("competition_team")
	form.submit_button().pressed.emit()
	await s.settle(6)
	var sent: FakeHttp.Recorded = s.h.http.last(F111Fixtures.ROUTE_CLUB_CREATE)
	assert_str(String(sent.headers.get("idempotency-key", ""))).is_not_empty()
	assert_str(String(sent.body["club_type"])).is_equal("competition_team")
	assert_str(String(sent.body["name"])).is_equal("Đội Robot")
	assert_bool(form.visible).is_false()
	assert_str(screen.notice_text()).is_equal(TranslationServer.translate("ui.club.created_pending"))
	# 451: a general sentence, nothing of the text echoed, the form stays.
	tab.create_button().pressed.emit()
	await s.settle(2)
	form.set_name_text("Tên placeholder_blocked")
	s.h.http.clear(F111Fixtures.ROUTE_CLUB_CREATE).reply(F111Fixtures.ROUTE_CLUB_CREATE, 451,
			AuthHarness.problem(451, "CHAT_CONTENT_BLOCKED"))
	form.submit_button().pressed.emit()
	await s.settle(6)
	assert_bool(form.visible).is_true()
	assert_str(form.error_text()).is_equal(TranslationServer.translate("ui.club.form.blocked"))
	assert_bool(form.error_text().contains("placeholder_blocked")).is_false()
	assert_str(form.name_text()).is_equal("Tên placeholder_blocked")
	# 403: the school closed club creation → the button goes.
	s.h.http.clear(F111Fixtures.ROUTE_CLUB_CREATE).reply(F111Fixtures.ROUTE_CLUB_CREATE, 403,
			AuthHarness.problem(403, "PERMISSION_DENIED"))
	form.set_name_text("Nhóm mới")
	form.submit_button().pressed.emit()
	await s.settle(6)
	assert_bool(form.visible).is_false()
	assert_str(screen.notice_text()).is_equal(TranslationServer.translate("ui.club.not_enabled"))
	assert_bool(tab.create_button().visible).is_false()


func test_active_club_opens_its_page() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_CLUB_CREATE).reply(F111Fixtures.ROUTE_CLUB_CREATE, 201,
			F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner", 1))
	var tab: ClubTab = await _open()
	tab.create_button().pressed.emit()
	await s.settle(2)
	screen.club_form().set_name_text("Nhà thám hiểm nhí")
	screen.club_form().submit_button().pressed.emit()
	await s.settle(10)
	assert_bool(tab.page_view().is_visible_in_tree()).is_true()
	assert_str(tab.page_view().club_id).is_equal(F111Fixtures.CLUB_OWN)
