extends GdUnitTestSuite
## T022 (F102-AC-15; MULTIPLAYER_GAMEPLAY §9, ADR-0031): while the realtime connection comes back the student's party
## writes are locked (no command, no REST call) and the party stays readable; once the connection is online again
## PartyService reads `getMyParty` and, when the party is in a session this student belongs to, enters the same
## session room again (session router of the test). A REST call that finds no network locks the writes too.
## RealtimeHarness (FakeHttp + FakeWebSocketPeer + FakeClock, TEST-ONLY).

const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null
var svc: PartyService = null
var entered: Array[String] = []
var offline_events: Array[bool] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	svc = PartyService.new(h.api, h.client)
	svc.store.self_id = PartyFixtures.LEADER
	entered.clear()
	offline_events.clear()
	svc.session_router = func(scene_ref: String, room_id: String) -> Error:
		entered.append("%s|%s" % [scene_ref, room_id])
		return OK
	svc.connection_changed.connect(func(value: bool) -> void: offline_events.append(value))
	SceneRouter.party_room_id = ""


func after_test() -> void:
	SceneRouter.party_room_id = ""
	h.client.unregister_handler(PartyService.PREFIX, svc.handle_message)
	h.dispose()


func _drop_and_come_back() -> void:
	h.peer().server_close(1006)
	h.client.tick()


func _resume() -> void:
	h.advance_ms(700)
	await h.await_ticket()
	h.open_with_welcome(1, "rt-new")
	h.push({"v": 1, "type": "system.session.resumed", "seq": h.client.session.last_seq + 1, "ts": 1,
		"payload": {"replayed": 0}})
	for i: int in 6:
		await get_tree().process_frame


func test_offline_locks_writes_and_keeps_the_party_readable() -> void:
	await h.connect_online()
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	_drop_and_come_back()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(svc.is_offline()).is_true()
	assert_array(offline_events).contains([true])
	var before: int = h.http.requests.size()
	assert_bool(await svc.set_ready(false)).is_false()
	var activity: EligibleActivityModel = EligibleActivityModel.list_from(PartyFixtures.eligible()["data"])[0]
	assert_bool(await svc.select_activity(activity)).is_false()
	assert_int(h.http.requests.size()).is_equal(before)
	# The party is still there to look at.
	assert_bool(svc.store.has_party()).is_true()
	assert_int(svc.store.party.members.size()).is_equal(3)


func test_back_online_reads_my_party_and_unlocks() -> void:
	await h.connect_online()
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	var newer: Dictionary = PartyFixtures.party()
	newer["state_version"] = 9
	(((newer["members"] as Array)[2]) as Dictionary)["ready"] = true
	h.http.reply(PartyFixtures.ROUTE_MY_PARTY, 200, newer)
	h.http.reply(PartyFixtures.ROUTE_INVITATIONS, 200, {"data": []})
	_drop_and_come_back()
	await _resume()
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_bool(svc.is_offline()).is_false()
	assert_int(h.http.count(PartyFixtures.ROUTE_MY_PARTY)).is_greater_equal(1)
	assert_int(svc.store.party.state_version).is_equal(9)
	assert_bool(svc.store.party.member(PartyFixtures.WAITING).ready).is_true()
	assert_bool(offline_events.back()).is_false()
	assert_array(entered).is_empty()


func test_back_online_in_a_session_enters_the_same_room_again() -> void:
	await h.connect_online()
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party_all_ready()))
	svc.handle_message("party.session.started", PartyFixtures.ws("session.started.quest"))
	var running: Dictionary = PartyFixtures.party()
	running["status"] = "in_activity"
	running["state_version"] = 8
	running["current_room_id"] = "0193b0a0-0000-7000-8000-000000000201"
	h.http.reply(PartyFixtures.ROUTE_MY_PARTY, 200, running)
	h.http.reply(PartyFixtures.ROUTE_INVITATIONS, 200, {"data": []})
	_drop_and_come_back()
	await _resume()
	assert_array(entered).is_equal(["res://scenes/world/zones/learning_zone_math/Zone.tscn|0193b0a0-0000-7000-8000-000000000201"])
	# Already in that room (the world shows it): nothing to enter.
	entered.clear()
	SceneRouter.party_room_id = "0193b0a0-0000-7000-8000-000000000201"
	await svc.resync()
	assert_array(entered).is_empty()


func test_a_rest_network_failure_locks_writes_until_the_next_answer() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	h.http.fail(PartyFixtures.ROUTE_GET_PARTY)
	await svc.refresh_party()
	assert_bool(svc.is_offline()).is_true()
	h.http.clear(PartyFixtures.ROUTE_GET_PARTY).reply(PartyFixtures.ROUTE_GET_PARTY, 200, PartyFixtures.party())
	await svc.refresh_party()
	assert_bool(svc.is_offline()).is_false()
	assert_bool(svc.store.has_party()).is_true()
