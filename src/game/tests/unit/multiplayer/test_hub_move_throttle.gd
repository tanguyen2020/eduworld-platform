extends GdUnitTestSuite
## F092-AC-10, F092-AC-11, F092-AC-16, F092-AC-17 (T016; GAME_NETWORKING §5.1): `hub.instance.join` on entering an
## area (again after a fresh connection); `hub.instance.assigned` gives room, spawn, self_eid and the players;
## `hub.player.move` at most `client_input_rate` (10 Hz), only after Δposition ≥ 2 cm, Δyaw ≥ 2° or a new animation,
## 1 Hz keepalive, position quantised to 1 cm and yaw to 1°; emotes limited by `emote_rate` with friendly codes;
## `hub.instance.migrated` keeps the scene; join errors are reported by code.

var h: RealtimeHarness = null
var hub: HubSync = null


func before_test() -> void:
	h = RealtimeHarness.create()
	hub = HubSync.new(h.client)


func after_test() -> void:
	hub.dispose()
	h.dispose()


func _moves() -> Array[Dictionary]:
	return h.peer().sent_of("hub.player.move")


func _joined() -> void:
	await h.connect_online()
	hub.enter_area("main_hub", "main_hub.default")
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")


func test_join_on_enter_and_assigned_room() -> void:
	var rooms: Array[HubRoomState] = []
	var joined: Array[int] = []
	hub.room_assigned.connect(func(r: HubRoomState) -> void: rooms.append(r))
	hub.remote_player_joined.connect(func(eid: int, _p: PublicPlayerModel, _s: SnapshotSample) -> void: joined.append(eid))
	hub.enter_area("main_hub", "main_hub.default")
	await h.connect_online()
	var join: Dictionary = h.peer().sent_of("hub.instance.join")[0]
	assert_dict(join["payload"] as Dictionary).is_equal({"area_id": "main_hub", "spawn_point": "main_hub.default"})
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")
	assert_int(rooms.size()).is_equal(1)
	assert_str(hub.room.room_id).is_equal("hub:main_hub:01")
	assert_int(hub.room.self_eid).is_equal(7)
	assert_bool(hub.room.spawn.is_equal_approx(Vector3(0, 0, 4))).is_true()
	assert_array(joined).contains_exactly([3])


func test_move_rate_thresholds_quantisation_and_keepalive() -> void:
	await _joined()
	var t: int = 10_000
	assert_bool(hub.update_local(Vector3(1.234, 0, 2.006), 0.0, "walk", t)).is_true()
	var first: Dictionary = _moves()[0]["payload"] as Dictionary
	assert_str(String(first["room_id"])).is_equal("hub:main_hub:01")
	assert_int(int(first["client_seq"])).is_equal(1)
	assert_float(float((first["p"] as Array)[0])).is_equal_approx(1.23, 0.0001)
	assert_float(float((first["p"] as Array)[2])).is_equal_approx(2.01, 0.0001)
	assert_str(String(first["a"])).is_equal("walk")
	# Faster than 10 Hz is never sent.
	assert_bool(hub.update_local(Vector3(2, 0, 2), 0.0, "walk", t + 50)).is_false()
	# Below 2 cm and 2° and the same animation: nothing (until the 1 Hz keepalive).
	assert_bool(hub.update_local(Vector3(1.24, 0, 2.01), deg_to_rad(1.0), "walk", t + 200)).is_false()
	assert_bool(hub.update_local(Vector3(1.26, 0, 2.01), 0.0, "walk", t + 300)).is_true()
	assert_bool(hub.update_local(Vector3(1.26, 0, 2.01), deg_to_rad(3.0), "walk", t + 400)).is_true()
	var turned: Dictionary = _moves()[2]["payload"] as Dictionary
	assert_float(float(turned["r"])).is_equal_approx(deg_to_rad(3.0), 0.0001)
	assert_bool(hub.update_local(Vector3(1.26, 0, 2.01), deg_to_rad(3.0), "idle", t + 500)).is_true()
	assert_bool(hub.update_local(Vector3(1.26, 0, 2.01), deg_to_rad(3.0), "idle", t + 1400)).is_false()
	assert_bool(hub.update_local(Vector3(1.26, 0, 2.01), deg_to_rad(3.0), "idle", t + 1500)).is_true()
	assert_int(_moves().size()).is_equal(5)
	# Running for 10 s at 60 FPS sends at most 10 per second.
	var start: int = _moves().size()
	for frame: int in 600:
		hub.update_local(Vector3(frame * 0.08, 0, 0), 0.0, "run", t + 2000 + frame * 16)
	assert_int(_moves().size() - start).is_less_equal(101)
	assert_int(_moves().size() - start).is_greater(80)


func test_server_max_move_hz_lowers_the_rate() -> void:
	await _joined()
	h.client.session.max_move_hz = 5
	var sent: int = 0
	for frame: int in 60:
		if hub.update_local(Vector3(frame * 0.1, 0, 0), 0.0, "run", 50_000 + frame * 16):
			sent += 1
	assert_int(sent).is_less_equal(5)


func test_no_move_before_the_room_or_while_reconnecting() -> void:
	await h.connect_online()
	assert_bool(hub.update_local(Vector3.ONE, 0.0, "idle", 1000)).is_false()
	hub.enter_area("main_hub")
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")
	h.peer().server_close(-1)
	h.client.tick()
	assert_bool(hub.update_local(Vector3.ONE, 0.0, "idle", 2000)).is_false()
	assert_bool(hub.is_frozen()).is_true()


func test_fresh_connection_joins_again_resume_does_not() -> void:
	await _joined()
	# Resume: the server restores the room subscriptions — no new join.
	h.peer().server_close(-1)
	h.client.tick()
	h.advance_ms(700)
	await h.await_ticket()
	h.open_with_welcome(1, "rt-2")
	h.push({"v": 1, "type": "system.session.resumed", "seq": h.client.session.last_seq + 1, "ts": 1, "payload": {}})
	assert_int(h.peer().sent_of("hub.instance.join").size()).is_equal(0)
	assert_bool(hub.room.is_joined()).is_true()
	# New session (no resume token): a fresh connection joins the area again.
	h.client.stop()
	await h.connect_online()
	assert_int(h.peer().sent_of("hub.instance.join").size()).is_equal(1)


func test_emote_rate_and_error_codes() -> void:
	await _joined()
	var failed: Array[String] = []
	hub.emote_failed.connect(func(code: String) -> void: failed.append(code))
	hub.emote("emote_wave", 100_000)
	assert_int(h.peer().sent_of("hub.player.emote").size()).is_equal(1)
	assert_dict(h.peer().sent_of("hub.player.emote")[0]["payload"] as Dictionary).is_equal(
			{"room_id": "hub:main_hub:01", "emote_id": "emote_wave"})
	hub.emote("emote_clap", 101_000)
	assert_int(h.peer().sent_of("hub.player.emote").size()).is_equal(1)
	assert_array(failed).contains_exactly(["RATE_LIMITED"])
	h.ack_last("hub.player.emote")
	hub.emote("emote_dance", 103_000)
	h.ack_last("hub.player.emote", false, "PERMISSION_DENIED")
	assert_array(failed).contains_exactly(["RATE_LIMITED", "PERMISSION_DENIED"])
	assert_str(HubSync.emote_notice_key("PERMISSION_DENIED")).is_equal("ui.hub.emote_not_owned")
	assert_str(HubSync.emote_notice_key("RATE_LIMITED")).is_equal("ui.hub.emote_rate_limited")


func test_join_errors_are_reported_by_code() -> void:
	await h.connect_online()
	var failed: Array[String] = []
	hub.join_failed.connect(func(code: String) -> void: failed.append(code))
	for code: String in ["ROOM_FULL", "PERMISSION_DENIED", "NOT_FOUND"]:
		hub.enter_area("practice_zone")
		h.ack_last("hub.instance.join", false, code)
	assert_array(failed).contains_exactly(["ROOM_FULL", "PERMISSION_DENIED", "NOT_FOUND"])
	assert_bool(hub.room.is_joined()).is_false()
	assert_str(HubSync.join_notice_key("ROOM_FULL")).is_equal("ui.hub.room_full")
	assert_str(HubSync.join_notice_key("PERMISSION_DENIED")).is_equal("ui.hub.join_denied")
	assert_str(HubSync.join_notice_key("NOT_FOUND")).is_equal("ui.hub.area_not_found")
	assert_str(HubSync.join_notice_key("INTERNAL")).is_equal("ui.error.generic")


func test_migrated_switches_the_room_without_leaving() -> void:
	await _joined()
	var left: Array[int] = []
	hub.remote_player_left.connect(func(eid: int) -> void: left.append(eid))
	h.push_fixture("hub.instance.migrated.failover")
	assert_str(hub.room.room_id).is_equal("hub:main_hub:02")
	assert_bool(hub.room.players.has(3)).is_true()
	assert_array(left).is_empty()
	# A late snapshot of the old room is ignored.
	var snap: Dictionary = h.envelope("hub.snapshot.updated", {"room_id": "hub:main_hub:01", "tick": 5,
			"entities": [{"eid": 3, "p": [9, 0, 9]}]})
	h.push(snap)
	assert_bool((hub.buffer.latest(3).p as Vector3).is_equal_approx(Vector3(9, 0, 9))).is_false()
	hub.update_local(Vector3(1, 0, 1), 0.0, "idle", 200_000)
	assert_str(String((_moves()[0]["payload"] as Dictionary)["room_id"])).is_equal("hub:main_hub:02")


func test_corrected_position_is_reported() -> void:
	await _joined()
	var corrections: Array[Vector3] = []
	hub.position_corrected.connect(func(p: Vector3) -> void: corrections.append(p))
	h.push_fixture("hub.player.corrected.speed")
	assert_int(corrections.size()).is_equal(1)
	assert_bool(corrections[0].is_equal_approx(Vector3(1.2, 0, 3.4))).is_true()
