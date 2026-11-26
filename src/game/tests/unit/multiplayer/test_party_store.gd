extends GdUnitTestSuite
## T013 (F102-AC-07, F102-AC-08; data-model §3): PartyStore applies the realtime `party.*` messages (AsyncAPI example
## fixtures through the RealtimeClient dispatcher of a RealtimeHarness, TEST-ONLY) in `state_version` order — an old or
## repeated version is dropped, a gap reloads the party with `getParty` and the change is not applied; Ready / presence
## changes touch one member (`member_updated`); Ready is sent with `party.member.ready_set` when the socket takes
## commands, the card changes only once the server acked it, and the REST fallback `setPartyReady` is used while the
## socket is down; leader transfer, activity change (Ready reset), kick of this student and disband are applied.

var h: RealtimeHarness = null
var svc: PartyService = null
var updated: Array[String] = []
var closed: Array[StringName] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	svc = PartyService.new(h.api, h.client)
	svc.store.self_id = PartyFixtures.LEADER
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	updated.clear()
	closed.clear()
	svc.store.member_updated.connect(func(id: String) -> void: updated.append(id))
	svc.store.party_closed.connect(func(reason: StringName) -> void: closed.append(reason))


func after_test() -> void:
	h.client.unregister_handler(PartyService.PREFIX, svc.handle_message)
	h.dispose()


func _push(name: String, edit: Dictionary = {}) -> void:
	var env: Dictionary = RealtimeHarness.fixture("party." + name)
	(env["payload"] as Dictionary).merge(edit, true)
	h.next_seq += 1
	env["seq"] = h.next_seq
	env["ts"] = h.client.server_now_ms()
	h.push(env)


# ---------- F102-AC-07 ----------

func test_versioned_messages_apply_in_order_and_old_ones_are_dropped() -> void:
	await h.connect_online()
	_push("member.joined.joined") # state_version 8 (held 7)
	assert_int(svc.store.party.members.size()).is_equal(4)
	assert_int(svc.store.party.state_version).is_equal(8)
	assert_int(svc.store.party.member(PartyFixtures.NEWCOMER).level).is_equal(9)
	# The same version again (resume replay) and an older one change nothing.
	_push("member.left.left") # state_version 8 again
	assert_object(svc.store.party.member(PartyFixtures.WAITING)).is_not_null()
	_push("member.left.left", {"state_version": 9})
	assert_object(svc.store.party.member(PartyFixtures.WAITING)).is_null()
	assert_int(svc.store.party.state_version).is_equal(9)


func test_a_version_gap_reloads_the_party_without_applying_the_change() -> void:
	await h.connect_online()
	var reloaded: Dictionary = PartyFixtures.party()
	reloaded["state_version"] = 12
	h.http.reply(PartyFixtures.ROUTE_GET_PARTY, 200, reloaded)
	_push("member.joined.joined", {"state_version": 11})
	assert_object(svc.store.party.member(PartyFixtures.NEWCOMER)).is_null()
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count(PartyFixtures.ROUTE_GET_PARTY)).is_equal(1)
	assert_int(svc.store.party.state_version).is_equal(12)


func test_lobby_changes_touch_one_member_only() -> void:
	await h.connect_online()
	_push("member.ready_changed.ready") # GauNau15 ready
	assert_bool(svc.store.party.member(PartyFixtures.WAITING).ready).is_true()
	assert_array(updated).is_equal([PartyFixtures.WAITING])
	_push("member.status_changed.disconnected") # CaHeoVui07 drops
	var dropped: PartyMemberModel = svc.store.party.member(PartyFixtures.MEMBER)
	assert_bool(dropped.is_disconnected()).is_true()
	assert_bool(dropped.ready).is_false()
	assert_array(updated).is_equal([PartyFixtures.WAITING, PartyFixtures.MEMBER])
	# A message of another party is ignored.
	_push("member.ready_changed.ready", {"party_id": Uuid.v7(), "user_id": PartyFixtures.MEMBER})
	assert_array(updated).is_equal([PartyFixtures.WAITING, PartyFixtures.MEMBER])


func test_activity_change_resets_every_ready_and_keeps_the_reasons() -> void:
	await h.connect_online()
	var changed: Array[PartyModel] = []
	svc.store.activity_changed.connect(func(p: PartyModel) -> void: changed.append(p))
	_push("activity.selected.boss_ineligible")
	assert_int(changed.size()).is_equal(1)
	assert_str(svc.store.party.activity_kind).is_equal("boss")
	assert_int(svc.store.party.not_ready_count()).is_equal(3)
	assert_int(svc.store.ineligible.size()).is_equal(1)
	assert_str(String(svc.store.ineligible[0]["requirement"])).is_equal("grade_levels:4")


func test_leader_transfer_kick_of_self_and_disband() -> void:
	await h.connect_online()
	var leaders: Array[String] = []
	svc.store.leader_changed.connect(func(_from: String, to: String, _reason: String) -> void: leaders.append(to))
	_push("leader.transferred.leader_disconnected")
	assert_array(leaders).is_equal([PartyFixtures.MEMBER])
	assert_bool(svc.store.is_leader()).is_false()
	assert_bool(svc.store.party.member(PartyFixtures.MEMBER).is_leader()).is_true()
	_push("member.left.kicked", {"user_id": PartyFixtures.LEADER})
	assert_array(closed).is_equal([PartyStore.CLOSED_KICKED])
	assert_bool(svc.store.has_party()).is_false()
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	_push("party.disbanded.leader_disbanded")
	assert_array(closed).is_equal([PartyStore.CLOSED_KICKED, PartyStore.CLOSED_DISBANDED])


func test_session_started_and_ended_follow_the_server() -> void:
	await h.connect_online()
	var ended: Array[Dictionary] = []
	svc.store.session_ended.connect(func(info: Dictionary) -> void: ended.append(info))
	_push("session.started.quest")
	assert_bool(svc.store.party.is_in_activity()).is_true()
	assert_str(String(svc.store.session["room_id"])).is_equal("0193b0a0-0000-7000-8000-000000000201")
	_push("session.ended.completed")
	assert_bool(svc.store.party.is_in_activity()).is_false()
	assert_dict(ended[0]).is_equal({"outcome": "completed", "reason": "completed"})


# ---------- F102-AC-08 ----------

func test_ready_goes_over_the_socket_and_shows_only_after_the_ack() -> void:
	await h.connect_online()
	svc.store.me().ready = false
	svc.set_ready(true)
	await get_tree().process_frame
	var sent: Array[Dictionary] = h.peer().sent_of(PartyService.CMD_READY)
	assert_int(sent.size()).is_equal(1)
	assert_dict((sent[0]["payload"]) as Dictionary).is_equal({"party_id": PartyFixtures.PARTY_ID, "ready": true})
	# Sent but not confirmed yet: the card still says "Chưa sẵn sàng".
	assert_bool(svc.store.me().ready).is_false()
	h.ack_last(PartyService.CMD_READY)
	await get_tree().process_frame
	assert_bool(svc.store.me().ready).is_true()
	assert_int(h.http.count(PartyFixtures.ROUTE_READY)).is_equal(0)


func test_ready_refused_by_the_server_does_not_change_the_card() -> void:
	await h.connect_online()
	svc.store.me().ready = false
	svc.set_ready(true)
	await get_tree().process_frame
	h.ack_last(PartyService.CMD_READY, false, "RATE_LIMITED")
	await get_tree().process_frame
	assert_bool(svc.store.me().ready).is_false()


func test_ready_falls_back_to_rest_when_the_socket_cannot_take_commands() -> void:
	# Never connected: the socket takes no command; REST `setPartyReady` (204) confirms.
	svc.store.me().ready = false
	h.http.reply(PartyFixtures.ROUTE_READY, 204)
	var ok: bool = await svc.set_ready(true)
	assert_bool(ok).is_true()
	assert_bool(svc.store.me().ready).is_true()
	assert_bool(bool(h.http.last(PartyFixtures.ROUTE_READY).body["ready"])).is_true()


func test_activity_select_sends_the_version_and_a_conflict_reloads() -> void:
	await h.connect_online()
	h.http.reply(PartyFixtures.ROUTE_GET_PARTY, 200, PartyFixtures.party())
	var activity: EligibleActivityModel = EligibleActivityModel.list_from(PartyFixtures.eligible()["data"])[1]
	svc.select_activity(activity)
	await get_tree().process_frame
	var sent: Dictionary = h.peer().sent_of(PartyService.CMD_SELECT)[0]["payload"] as Dictionary
	assert_int(int(sent["expected_version"])).is_equal(7)
	assert_dict(sent["activity"] as Dictionary).is_equal({"kind": "explore", "id": "learning_zone.math"})
	h.ack_last(PartyService.CMD_SELECT, false, "CONFLICT")
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count(PartyFixtures.ROUTE_GET_PARTY)).is_equal(1)
