extends GdUnitTestSuite
## F092-AC-05, F092-AC-06 (T009; GAME_NETWORKING §4.2, §4.4, §9): resume sends `last_seq`, replayed duplicates are
## dropped, the resumed connection keeps its token; `resume_failed` → resync (getGameProfile + hub join) then Online;
## `send_command` pairs `ref` with a UUIDv7 `id`, times out after `client_command_timeout`; `hub.player.move` does
## not wait; unknown types are ignored and a payload of the wrong type never reaches a service.

const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null
var seen: Array[String] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	seen.clear()
	h.client.register_handler("hub", func(type: String, _payload: Dictionary, _env: WsEnvelope) -> void: seen.append(type))


func after_test() -> void:
	h.dispose()


func _drop_and_reconnect() -> void:
	h.peer().server_close(-1)
	h.client.tick()
	h.advance_ms(700)
	await h.await_ticket()


func test_resume_replays_once_and_keeps_the_token() -> void:
	await h.connect_online()
	h.push(h.envelope("hub.player.left", {"room_id": "r1", "eid": 1}))
	h.push(h.envelope("hub.player.left", {"room_id": "r1", "eid": 2}))
	var last: int = h.client.session.last_seq
	await _drop_and_reconnect()
	h.open_with_welcome(1, "rt-token-of-new-connection")
	var resume: Dictionary = h.peer().last_sent()
	assert_int(int((resume["payload"] as Dictionary)["last_seq"])).is_equal(last)
	# Replay: one already processed (dropped) and one new.
	h.push({"v": 1, "type": "hub.player.left", "seq": last, "ts": 1, "payload": {"room_id": "r1", "eid": 2}})
	h.push({"v": 1, "type": "hub.player.left", "seq": last + 1, "ts": 1, "payload": {"room_id": "r1", "eid": 3}})
	h.push({"v": 1, "type": "system.session.resumed", "seq": last + 2, "ts": 1, "payload": {"replayed": 1}})
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_array(seen).contains_exactly(["hub.player.left", "hub.player.left", "hub.player.left"])
	assert_str(h.client.session.resume_token).is_equal("rt-example-opaque-0001")
	assert_int(h.client.stats.resume_ok).is_equal(1)


func test_resume_failed_resyncs_profile_and_joins_the_area_again() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, null, h.clock, h.client)
	h.http.reply(ProfileFixtures.ROUTE_PROFILE, 200, ProfileFixtures.profile_ready())
	await h.connect_online()
	services.hub.enter_area("main_hub", "main_hub.default")
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")
	assert_bool(services.hub.room.is_joined()).is_true()
	await _drop_and_reconnect()
	h.open_with_welcome(1, "rt-fresh")
	h.push({"v": 1, "type": "system.session.resume_failed", "seq": 2, "ts": 1, "payload": {"reason": "expired"}})
	h.next_seq = 2
	assert_int(h.client.state).is_equal(STATE.RESYNC)
	assert_bool(services.hub.room.is_joined()).is_false()
	for i: int in 10:
		await get_tree().process_frame
	assert_int(h.http.count(ProfileFixtures.ROUTE_PROFILE)).is_equal(1)
	var joins: Array[Dictionary] = h.peer().sent_of("hub.instance.join")
	assert_int(joins.size()).is_equal(1)
	assert_str(String((joins[0]["payload"] as Dictionary)["area_id"])).is_equal("main_hub")
	h.ack_last("hub.instance.join")
	for i: int in 3:
		await get_tree().process_frame
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_str(h.client.session.resume_token).is_equal("rt-fresh")
	# The new numbering applies after a failed resume (low seq values are processed again).
	h.push(h.envelope("hub.player.left", {"room_id": "x", "eid": 1}))
	assert_int(h.client.session.last_seq).is_equal(h.next_seq)
	assert_bool(h.next_seq < 10).is_true()
	services.hub.dispose()


func test_send_command_pairs_ref_and_completes_on_ack_or_failure() -> void:
	await h.connect_online()
	var results: Array[CommandResult] = []
	var run: Callable = func() -> void: results.append(await h.client.send_command("hub.instance.join", {"area_id": "main_hub"}))
	run.call()
	var sent: Dictionary = h.peer().last_sent()
	assert_bool(Uuid.is_v7(String(sent["id"]))).is_true()
	assert_int(int(sent["v"])).is_equal(1)
	assert_int(h.client.pending_count()).is_equal(1)
	# An ack for another id does not complete it.
	h.push(h.envelope("system.command.acked", {}, "0192f3a4-5b6c-7d8e-9f01-000000000000"))
	assert_int(results.size()).is_equal(0)
	h.push(h.envelope("system.command.acked", {"result": {}}, String(sent["id"])))
	assert_int(results.size()).is_equal(1)
	assert_bool(results[0].ok).is_true()
	run.call()
	h.ack_last("hub.instance.join", false, "ROOM_FULL")
	assert_str(results[1].code).is_equal("ROOM_FULL")


func test_command_times_out_after_client_command_timeout() -> void:
	await h.connect_online()
	var results: Array[CommandResult] = []
	var run: Callable = func() -> void: results.append(await h.client.send_command("hub.instance.join", {"area_id": "main_hub"}))
	run.call()
	h.push(h.envelope("system.heartbeat.pong", {}))
	h.advance_ms(9999)
	assert_int(results.size()).is_equal(0)
	h.advance_ms(2)
	assert_str(results[0].code).is_equal(CommandResult.CODE_TIMEOUT)
	assert_int(h.client.pending_count()).is_equal(0)


func test_move_is_fire_and_forget_and_commands_wait_for_online() -> void:
	await h.connect_online()
	assert_bool(h.client.send_fire_and_forget("hub.player.move", {"room_id": "r", "client_seq": 1, "p": [0, 0, 0], "r": 0.0})).is_true()
	assert_int(h.client.pending_count()).is_equal(0)
	h.peer().server_close(-1)
	h.client.tick()
	var r: CommandResult = await h.client.send_command("hub.player.emote", {"room_id": "r", "emote_id": "emote_wave"})
	assert_str(r.code).is_equal(CommandResult.CODE_OFFLINE)
	assert_bool(h.client.send_fire_and_forget("hub.player.move", {"room_id": "r", "client_seq": 2, "p": [0, 0, 0], "r": 0.0})).is_false()


func test_unknown_types_and_wrong_payloads_never_reach_a_service() -> void:
	await h.connect_online()
	h.push(h.envelope("hub.future.thing", {"x": 1}))
	h.push(h.envelope("mystery.thing.happened", {"x": 1}))
	# Wrong types: eid as text, missing room id, bad envelope.
	h.push(h.envelope("hub.player.left", {"room_id": "r1", "eid": "three"}))
	h.push(h.envelope("hub.player.left", {"eid": 3}))
	h.push(h.envelope("hub.instance.assigned", {"room_id": "r1", "area_id": "main_hub", "self_eid": 1, "spawn": [0, 0], "tick_rate_hz": 10}))
	h.peer().push_raw("not json")
	h.peer().push_raw(JSON.stringify({"v": 2, "type": "hub.player.left", "seq": 999, "payload": {}}))
	h.peer().push_raw(JSON.stringify({"v": 1, "type": "Hub.Player.Left", "seq": 1000, "payload": {}}))
	h.client.tick()
	assert_array(seen).contains_exactly(["hub.future.thing"])
	assert_int(h.client.state).is_equal(STATE.ONLINE)


func test_frames_larger_than_max_frame_bytes_are_not_sent() -> void:
	await h.connect_online()
	var big: String = "x".repeat(20000)
	assert_bool(h.client.send_fire_and_forget("hub.player.move", {"room_id": big})).is_false()
	assert_int(h.peer().sent_of("hub.player.move").size()).is_equal(0)
