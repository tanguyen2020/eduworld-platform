extends GdUnitTestSuite
## T013 (F111-AC-04, F111-AC-05; AC-SOC-005, AGENTS R16): a refused follow — `ROOM_FULL` → `hub.instance.join` again
## without `follow_user_id` for that area + "Khu vực của bạn đã đầy"; `PERMISSION_DENIED` / `NOT_FOUND` → an ordinary
## instance + "Chưa thể tham gia bạn lúc này" — never a room, an area or a reason in the words; `403 AREA_LOCKED` on the
## zone change → the student stays, no hub join is sent (the world shows the unlock conditions, F071); while a join is
## on its way the button is locked (no second command).

var s: F111SceneHarness = null
var join: JoinFriendService = null
var notices: Array[Array] = []
var failed: Array[String] = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	join = s.services.join_friend
	notices.clear()
	failed.clear()
	join.notice_requested.connect(func(key: String, args: Dictionary) -> void: notices.append([key, args]))
	join.join_failed.connect(func(code: String) -> void: failed.append(code))
	await s.services.social.load_friends()
	s.ack_last(HubSync.TYPE_JOIN)
	await s.settle(2)


func after_test() -> void:
	s.dispose()


func _friend_in(area_id: String) -> void:
	s.push("presence.friend.updated", F110Fixtures.ws("presence.friend.updated", {"user_id": F111Fixtures.FRIEND_ONLINE,
		"status": "online", "activity": "idle", "area_id": area_id}))


func _shown_words() -> String:
	var out: PackedStringArray = []
	for n: Array in notices:
		out.append(TranslationServer.translate(String(n[0])).format(n[1] as Dictionary))
	return " ".join(out)


func test_room_full_joins_the_area_without_follow_and_says_it_is_full() -> void:
	_friend_in("main_hub")
	await join.join(F111Fixtures.FRIEND_ONLINE)
	var count: int = s.joins().size()
	s.ack_last(HubSync.TYPE_JOIN, false, "ROOM_FULL")
	await s.settle(3)
	assert_int(s.joins().size()).is_equal(count + 1)
	assert_bool(s.joins().back().has("follow_user_id")).is_false()
	assert_str(String(s.joins().back()["area_id"])).is_equal("main_hub")
	assert_array(notices).contains([["ui.friends.join_room_full", {}]])
	assert_array(failed).contains(["ROOM_FULL"])
	assert_bool(join.is_busy()).is_false()


func test_denied_or_gone_says_a_general_sentence_without_any_place() -> void:
	for code: String in ["PERMISSION_DENIED", "NOT_FOUND"]:
		notices.clear()
		_friend_in("main_hub")
		await join.join(F111Fixtures.FRIEND_ONLINE)
		s.ack_last(HubSync.TYPE_JOIN, false, code)
		await s.settle(3)
		assert_bool(s.joins().back().has("follow_user_id")).override_failure_message(code).is_false()
		assert_array(notices).contains([["ui.friends.join_not_now", {}]])
		var words: String = _shown_words()
		for leak: String in ["main_hub", "hub:", "room", code]:
			assert_bool(words.contains(leak)).override_failure_message("%s: %s" % [code, leak]).is_false()
		s.ack_last(HubSync.TYPE_JOIN)
		await s.settle(2)


func test_area_locked_keeps_the_student_and_sends_no_join() -> void:
	_friend_in("challenge_mountain")
	s.area_result = FAILED
	var count: int = s.joins().size()
	assert_bool(await join.join(F111Fixtures.FRIEND_ONLINE)).is_true()
	await s.settle(2)
	assert_int(s.areas.size()).is_equal(1)
	assert_int(s.joins().size()).is_equal(count)
	assert_str(s.current_area).is_equal("main_hub")
	assert_bool(join.is_busy()).is_false()
	assert_array(failed).contains(["AREA_LOCKED"])


func test_the_button_is_locked_while_a_join_is_on_its_way() -> void:
	_friend_in("main_hub")
	await join.join(F111Fixtures.FRIEND_ONLINE)
	var count: int = s.joins().size()
	assert_bool(join.is_busy()).is_true()
	assert_bool(await join.join(F111Fixtures.FRIEND_ONLINE)).is_false()
	assert_int(s.joins().size()).is_equal(count)
	assert_str(String(join.availability(F111Fixtures.FRIEND_ONLINE))).is_equal("pending")
