extends GdUnitTestSuite
## F092-AC-03, F092-AC-04 (T008; GAME_NETWORKING §4.3–§4.4): heartbeat every `heartbeat_s`, silence of 2 × heartbeat
## → reconnect; close codes 4401 (new ticket now), 4408/abnormal (backoff then resume), 1012 after
## `system.server.draining` (wait `reconnect_after_ms`, quiet), 4429 (≥ 30 s), 4403 (stop, session ended for the
## AuthService), 4409 after `session.session.replaced` (stop, no reconnect) and 4409 without it (quiet), 4410 (curfew).

const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null


func before_test() -> void:
	h = RealtimeHarness.create()


func after_test() -> void:
	h.dispose()


func _tickets() -> int:
	return h.http.count(RealtimeHarness.ROUTE_TICKET)


func test_heartbeat_every_heartbeat_s_and_silence_reconnects() -> void:
	await h.connect_online()
	var peer: FakeWebSocketPeer = h.peer()
	h.advance_ms(14000)
	assert_int(peer.sent_of("system.heartbeat.ping").size()).is_equal(0)
	h.advance_ms(1000)
	assert_int(peer.sent_of("system.heartbeat.ping").size()).is_equal(1)
	assert_bool((peer.sent_of("system.heartbeat.ping")[0]["payload"] as Dictionary).has("client_time")).is_true()
	# A pong refreshes the silence timer and gives an RTT sample.
	var ping_time: int = int((peer.sent_of("system.heartbeat.ping")[0]["payload"] as Dictionary)["client_time"])
	h.advance_ms(40)
	h.push(h.envelope("system.heartbeat.pong", {"client_time": ping_time, "server_time": 1_763_600_015_020}))
	assert_int(h.client.stats.rtt_percentile(50.0)).is_equal(40)
	h.advance_ms(15000)
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	# Nothing received for 2 × 15 s → close and reconnect.
	h.advance_ms(15000)
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(peer.closed_by_client).is_true()


func test_4401_asks_a_new_ticket_at_once() -> void:
	await h.connect_online()
	h.peer().server_close(GameRealtimeClient.CLOSE_TICKET_INVALID)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.FETCHING_TICKET)
	await h.await_ticket()
	assert_int(_tickets()).is_equal(2)


func test_4408_and_abnormal_close_back_off_then_resume() -> void:
	for code: int in [GameRealtimeClient.CLOSE_HEARTBEAT_TIMEOUT, -1, 1006]:
		await h.connect_online()
		h.push(h.envelope("system.heartbeat.pong", {}))
		var seq: int = h.client.session.last_seq
		h.peer().server_close(code)
		h.client.tick()
		assert_int(h.client.state).override_failure_message(str(code)).is_equal(STATE.BACKOFF)
		assert_bool(h.client.is_reconnecting()).is_true()
		h.advance_ms(700)
		await h.await_ticket()
		h.open_with_welcome(1, "rt-new")
		assert_int(h.client.state).is_equal(STATE.RESUMING)
		var resume: Dictionary = h.peer().last_sent()
		assert_str(String(resume["type"])).is_equal("system.session.resume")
		assert_int(int((resume["payload"] as Dictionary)["last_seq"])).is_equal(seq)
		assert_str(String((resume["payload"] as Dictionary)["resume_token"])).is_equal("rt-example-opaque-0001")
		h.client.stop()


func test_1012_after_draining_waits_reconnect_after_ms_quietly() -> void:
	await h.connect_online()
	h.push(h.envelope("system.server.draining", {"reconnect_after_ms": 3500, "reason": "deploy"}))
	h.peer().server_close(GameRealtimeClient.CLOSE_SERVICE_RESTART)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(h.client.is_reconnecting()).is_false()
	h.advance_ms(3400)
	assert_int(_tickets()).is_equal(1)
	h.advance_ms(200)
	await h.await_ticket()
	assert_int(_tickets()).is_equal(2)


func test_4429_waits_at_least_30_seconds() -> void:
	await h.connect_online()
	h.peer().server_close(GameRealtimeClient.CLOSE_RATE_LIMITED)
	h.client.tick()
	h.advance_ms(29000)
	assert_int(_tickets()).is_equal(1)
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	h.advance_ms(1500)
	await h.await_ticket()
	assert_int(_tickets()).is_equal(2)


func test_4403_stops_and_reports_the_reason() -> void:
	var ended: Array = []
	h.client.session_ended_received.connect(func(reason: String, at: int) -> void: ended.append([reason, at]))
	await h.connect_online()
	var msg: Dictionary = RealtimeHarness.fixture("session.session.ended.account_locked")
	msg["seq"] = 50
	h.push(msg)
	h.peer().server_close(GameRealtimeClient.CLOSE_REVOKED)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.STOPPED)
	assert_array(ended).contains_exactly([["account_locked", 0]])
	assert_str(h.client.session.resume_token).is_empty()
	h.advance_ms(120000)
	assert_int(_tickets()).is_equal(1)


func test_4409_after_replaced_stops_and_never_reconnects() -> void:
	var replaced: Array[int] = []
	h.client.session_replaced_received.connect(func() -> void: replaced.append(1))
	await h.connect_online()
	var msg: Dictionary = RealtimeHarness.fixture("session.session.replaced.takeover")
	msg["seq"] = 60
	h.push(msg)
	h.peer().server_close(GameRealtimeClient.CLOSE_REPLACED)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.STOPPED)
	assert_int(replaced.size()).is_equal(1)
	h.advance_ms(120000)
	assert_int(_tickets()).is_equal(1)


func test_4409_without_message_is_silent() -> void:
	var replaced: Array[int] = []
	h.client.session_replaced_received.connect(func() -> void: replaced.append(1))
	await h.connect_online()
	h.peer().server_close(GameRealtimeClient.CLOSE_REPLACED)
	h.client.tick()
	assert_int(replaced.size()).is_equal(0)
	assert_bool(h.client.is_reconnecting()).is_false()
	assert_int(h.client.state).is_not_equal(STATE.STOPPED)


func test_4410_curfew_stops_with_reopen_time() -> void:
	var ended: Array = []
	h.client.session_ended_received.connect(func(reason: String, at: int) -> void: ended.append([reason, at]))
	await h.connect_online()
	var msg: Dictionary = RealtimeHarness.fixture("session.session.ended.curfew")
	msg["seq"] = 70
	h.push(msg)
	h.peer().server_close(GameRealtimeClient.CLOSE_SESSION_ENDED)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.STOPPED)
	assert_str(String(ended[0][0])).is_equal("curfew")
	assert_int(int(ended[0][1])).is_equal(int((msg["payload"] as Dictionary)["reopen_at"]))


## AuthService path (F092-AC-04, AC-09): Online stops the REST heartbeat; replaced → ReplacedNotice, tokens and the
## stored session erased, the realtime client disconnected (no reconnect).
func test_auth_service_replaced_and_expired() -> void:
	var auth: AuthHarness = AuthHarness.create()
	var rt: GameRealtimeClient = GameRealtimeClient.new()
	rt.auto_poll = false
	var peers: Array[FakeWebSocketPeer] = []
	rt.configure(auth.api, auth.clock, func() -> WsPeer:
		var p: FakeWebSocketPeer = FakeWebSocketPeer.new()
		peers.append(p)
		return p)
	auth.auth.attach_realtime(rt)
	auth.http.reply(RealtimeHarness.ROUTE_TICKET, 201, {"ticket": RealtimeHarness.TICKET,
			"expires_at": "2026-11-21T08:00:01Z", "url": RealtimeHarness.WS_URL, "protocol_version": 1})
	await auth.sign_in()
	for i: int in 10:
		await get_tree().process_frame
	assert_int(peers.size()).is_equal(1)
	peers[0].open()
	var welcome: Dictionary = RealtimeHarness.fixture("system.session.welcome.new_connection")
	peers[0].push(welcome)
	rt.tick()
	assert_int(rt.state).is_equal(STATE.ONLINE)
	# Online → the REST heartbeat is not sent (F092-AC-09).
	assert_bool(auth.auth.sessions.realtime_online).is_true()
	var replaced: Dictionary = RealtimeHarness.fixture("session.session.replaced.takeover")
	replaced["seq"] = 2
	peers[0].push(replaced)
	peers[0].server_close(GameRealtimeClient.CLOSE_REPLACED)
	rt.tick()
	assert_int(auth.last_state()).is_equal(GameStateMachine.State.REPLACED_NOTICE)
	assert_bool(auth.auth.tokens.has_access()).is_false()
	assert_dict(auth.stored_record()).is_empty()
	assert_int(rt.state).is_equal(STATE.DISCONNECTED)
	rt.free()
	auth.dispose()
