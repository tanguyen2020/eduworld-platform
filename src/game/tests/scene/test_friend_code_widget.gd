extends GdUnitTestSuite
## T026 (F110-AC-23; SOCIAL_SYSTEM §4, ADR-0041): "Tạo mã" calls `createFriendCode` and shows the code in big characters
## with a countdown of the server `expires_at` and "Đọc to" one character after the other; the code field upper-cases,
## keeps only A-Z / 0-9, at most 12 characters, enables "Gửi" only for `^[A-Z0-9]{6,12}$`, then `redeemFriendCode`;
## `404 FRIEND_CODE_INVALID` → "Mã không đúng hoặc đã hết hạn".

var s: F110SceneHarness = null
var screen: FriendsScreen = null
var widget: FriendCodeWidget = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g1_3")
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.open({"tab": FriendsScreen.TAB_FIND})
	await s.settle(4)
	widget = screen.find_view().code_widget


func after_test() -> void:
	if screen.visible:
		screen.close()
	s.dispose()


func test_the_field_upper_cases_filters_and_limits_the_code() -> void:
	widget.type_text("ab-12 cd!")
	assert_str(widget.field().text).is_equal("AB12CD")
	assert_bool(widget.send_button().disabled).is_false()
	widget.type_text("ab1")
	assert_str(widget.field().text).is_equal("AB1")
	assert_bool(widget.send_button().disabled).is_true()
	widget.type_text("abcdefghijklmnop")
	assert_str(widget.field().text).is_equal("ABCDEFGHIJKL")
	assert_int(widget.field().max_length).is_equal(12)
	assert_bool(widget.send_button().disabled).is_false()
	widget.type_text("éèà")
	assert_str(widget.field().text).is_empty()
	assert_bool(widget.send_button().disabled).is_true()


func test_send_redeems_the_normalized_code_and_an_invalid_code_is_friendly() -> void:
	widget.type_text("k7m2qx")
	widget.send_button().pressed.emit()
	await s.settle(3)
	assert_str(String(s.h.http.last(F110Fixtures.ROUTE_REDEEM).body["code"])).is_equal("K7M2QX")
	assert_bool(Uuid.is_valid(String(s.h.http.last(F110Fixtures.ROUTE_REDEEM).headers.get("idempotency-key", "")))).is_true()
	s.h.http.clear(F110Fixtures.ROUTE_REDEEM).reply(F110Fixtures.ROUTE_REDEEM, 404, AuthHarness.problem(404, "FRIEND_CODE_INVALID"))
	widget.type_text("ZZZZ99")
	widget.send_button().pressed.emit()
	await s.settle(3)
	assert_str(screen.notice_text()).is_equal(TranslationServer.translate("ui.error.FRIEND_CODE_INVALID"))
	assert_str(TranslationServer.translate("ui.error.FRIEND_CODE_INVALID")).is_equal("Mã không đúng hoặc đã hết hạn")


func test_create_shows_the_code_big_with_the_server_countdown_and_reads_it_aloud() -> void:
	widget.find_child("CreateCode", true, false).emit_signal("pressed")
	await s.settle(4)
	assert_object(widget.code).is_not_null()
	assert_str(widget.code.code).is_equal("K7M2QX")
	assert_str(widget.spelled()).is_equal("K 7 M 2 Q X")
	var chars: Node = widget.find_child("Chars", true, false)
	assert_int(chars.get_child_count()).is_equal(6)
	var now: int = s.services.social.now_ms()
	widget.tick(now)
	var countdown: Label = widget.find_child("Countdown", true, false) as Label
	assert_str(countdown.text).contains(ExamCountdown.clock_text(widget.code.left_ms(now)))
	widget.read_aloud()
	assert_int(widget.reading_index).is_equal(0)
	for i: int in 5:
		widget._process(FriendCodeWidget.READ_STEP_SEC + 0.01)
	assert_int(widget.reading_index).is_equal(5)
	widget._process(FriendCodeWidget.READ_STEP_SEC + 0.01)
	assert_int(widget.reading_index).is_equal(-1)
	# Past `expires_at` the code disappears and "Tạo mã" comes back.
	widget.tick(widget.code.expires_ms + 1)
	assert_object(widget.code).is_null()
