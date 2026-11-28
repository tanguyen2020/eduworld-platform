extends GdUnitTestSuite
## T011 (F111-AC-01, F111-AC-03; SOCIAL_SYSTEM §4.2, MULTIPLAYER_GAMEPLAY §6): "Tham gia bạn" reads the latest `area_id`
## of `presence.friend.updated`; a friend in the area on screen → one `hub.instance.join` with `follow_user_id`; a friend
## in another area → `SceneRouter.request_area(area_id, {follow_user_id})` first, then the hub join of the new zone with
## `follow_user_id`; `hub.instance.assigned` → the student is placed and the toast "Em đã đến chỗ {nickname}" is asked.
## The REST `joinFriendInWorld` is never called.

var s: F111SceneHarness = null
var join: JoinFriendService = null
var notices: Array[Array] = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	join = s.services.join_friend
	notices.clear()
	join.notice_requested.connect(func(key: String, args: Dictionary) -> void: notices.append([key, args]))
	await s.services.social.load_friends()
	s.ack_last(HubSync.TYPE_JOIN)
	await s.settle(2)


func after_test() -> void:
	s.dispose()


func _friend_in(area_id: String, status: String = "online") -> void:
	s.push("presence.friend.updated", F110Fixtures.ws("presence.friend.updated", {"user_id": F111Fixtures.FRIEND_ONLINE,
		"status": status, "activity": "idle", "area_id": area_id}))


func test_same_area_sends_one_follow_join_and_says_where_the_student_is() -> void:
	_friend_in("main_hub")
	var before: int = s.joins().size()
	assert_bool(await join.join(F111Fixtures.FRIEND_ONLINE)).is_true()
	assert_int(s.joins().size()).is_equal(before + 1)
	assert_dict(s.joins().back()).contains_key_value("follow_user_id", F111Fixtures.FRIEND_ONLINE)
	assert_dict(s.joins().back()).contains_key_value("area_id", "main_hub")
	assert_array(s.areas).is_empty()
	assert_bool(join.is_busy()).is_true()
	s.assign_last("main_hub")
	await s.settle(2)
	assert_bool(join.is_busy()).is_false()
	assert_str(s.services.hub.room.room_id).is_equal("hub:main_hub:02")
	assert_array(notices).contains([["ui.friends.joined", {"nickname": F111Fixtures.FRIEND_NICK}]])
	assert_int(s.h.http.count("POST /game/world/friends/*")).is_equal(0)


func test_another_area_changes_zone_first_then_joins_next_to_the_friend() -> void:
	_friend_in("learning_zone.math")
	assert_bool(await join.join(F111Fixtures.FRIEND_ONLINE)).is_true()
	await s.settle(2)
	assert_int(s.areas.size()).is_equal(1)
	assert_str(String(s.areas[0]["area_id"])).is_equal("learning_zone.math")
	assert_str(String((s.areas[0]["args"] as Dictionary)["follow_user_id"])).is_equal(F111Fixtures.FRIEND_ONLINE)
	var last: Dictionary = s.joins().back()
	assert_str(String(last["area_id"])).is_equal("learning_zone.math")
	assert_str(String(last.get("follow_user_id", ""))).is_equal(F111Fixtures.FRIEND_ONLINE)
	s.assign_last("learning_zone.math")
	await s.settle(2)
	assert_array(notices).contains([["ui.friends.joined", {"nickname": F111Fixtures.FRIEND_NICK}]])
	assert_int(s.h.http.count("POST /game/world/friends/*")).is_equal(0)


func test_the_latest_presence_area_is_used() -> void:
	_friend_in("learning_zone.math")
	_friend_in("main_hub")
	assert_bool(await join.join(F111Fixtures.FRIEND_ONLINE)).is_true()
	assert_array(s.areas).is_empty()
	assert_str(String(s.joins().back().get("follow_user_id", ""))).is_equal(F111Fixtures.FRIEND_ONLINE)
