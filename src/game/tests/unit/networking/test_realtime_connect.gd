extends GdUnitTestSuite
## F092-AC-01, F092-AC-02 (T007; GAME_NETWORKING §4.1, AGENTS R23): with a game session the client asks
## `createRealtimeTicket` with `X-Tenant-Id`, opens the ticket `url` over `wss` with subprotocol `eduworld.v1`, waits
## for `system.session.welcome`, keeps resume token, heartbeat and server time and is Online. A ticket refused with
## `403 GAME_ACCESS_STUDENT_ONLY` / `TENANT_MISMATCH` stops the client without retrying.

const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null


func before_test() -> void:
	h = RealtimeHarness.create()


func after_test() -> void:
	h.dispose()


func test_ticket_then_wss_with_subprotocol_then_welcome_is_online() -> void:
	h.reply_ticket()
	h.client.start(RealtimeHarness.SESSION_ID)
	assert_int(h.client.state).is_equal(STATE.FETCHING_TICKET)
	await h.await_ticket()
	var req: FakeHttp.Recorded = h.http.last(RealtimeHarness.ROUTE_TICKET)
	assert_str(req.path).is_equal("/game/sessions/%s/realtime-tickets" % RealtimeHarness.SESSION_ID)
	assert_str(String(req.headers.get("x-tenant-id", ""))).is_equal(RealtimeHarness.TENANT_ID)
	assert_int(h.client.state).is_equal(STATE.CONNECTING)
	var peer: FakeWebSocketPeer = h.peer()
	assert_str(peer.url).starts_with(RealtimeHarness.WS_URL + "?ticket=")
	assert_array(Array(peer.protocols)).contains_exactly(["eduworld.v1"])
	# The access token never travels over the socket (ADR-0008): only the one-time ticket in the URL.
	assert_bool(peer.url.contains(RealtimeHarness.ACCESS)).is_false()
	h.open_with_welcome(1, "rt-token-A")
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_str(h.client.session.resume_token).is_equal("rt-token-A")
	assert_int(h.client.session.heartbeat_s).is_equal(15)
	assert_int(h.client.session.timeout_s).is_equal(45)
	assert_int(h.client.session.max_frame_bytes).is_equal(16384)
	assert_int(h.client.session.last_seq).is_equal(1)
	# Server clock: offset from server_time (FakeClock monotonic + offset = server ms).
	assert_bool(h.client.server_now_ms() == 1_763_600_000_000).override_failure_message(str(h.client.server_now_ms())).is_true()
	assert_array(h.states).contains_exactly([STATE.FETCHING_TICKET, STATE.CONNECTING, STATE.WELCOMED, STATE.ONLINE])


func test_wrong_subprotocol_is_not_accepted() -> void:
	h.reply_ticket()
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	h.peer().open("other.v9")
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(h.peer().closed_by_client).is_true()


func test_student_only_refusal_stops_without_retry() -> void:
	var stopped: Array[String] = []
	h.client.access_stopped.connect(func(code: String) -> void: stopped.append(code))
	h.http.reply(RealtimeHarness.ROUTE_TICKET, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.STOPPED)
	assert_array(stopped).contains_exactly(["GAME_ACCESS_STUDENT_ONLY"])
	h.advance_ms(60000)
	assert_int(h.http.count(RealtimeHarness.ROUTE_TICKET)).is_equal(1)
	assert_bool(h.peers.is_empty()).is_true()


func test_tenant_mismatch_stops_without_retry() -> void:
	h.http.reply(RealtimeHarness.ROUTE_TICKET, 403, AuthHarness.problem(403, "TENANT_MISMATCH"))
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.STOPPED)
	h.advance_ms(60000)
	assert_int(h.http.count(RealtimeHarness.ROUTE_TICKET)).is_equal(1)


func test_ticket_network_error_backs_off_then_retries() -> void:
	h.http.fail(RealtimeHarness.ROUTE_TICKET)
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	h.reply_ticket()
	h.advance_ms(16000)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.CONNECTING)


func test_no_welcome_within_ten_seconds_backs_off() -> void:
	h.reply_ticket()
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	h.advance_ms(GameRealtimeClient.CONNECT_TIMEOUT_MS + 1)
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
