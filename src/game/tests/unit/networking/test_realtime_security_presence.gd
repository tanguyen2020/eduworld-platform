extends GdUnitTestSuite
## F092-AC-07, F092-AC-09 (T010; ADR-0008, ADR-0064, AGENTS R14; MULTIPLAYER_GAMEPLAY §5): no ticket, token or resume
## token in the log or the telemetry; a release build refuses `ws://`; while Online the REST heartbeat stops (the tick
## still runs for the curfew checks); `presence.status.set` follows the activity (`online`, `in_activity`, `in_match`)
## and 5 minutes without input (`away`).

const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null


func before_test() -> void:
	h = RealtimeHarness.create()
	AppState.pending_telemetry.clear()


func after_test() -> void:
	h.dispose()


func _everything_logged() -> String:
	return JSON.stringify(Log.entries()) + JSON.stringify(AppState.pending_telemetry)


func test_no_ticket_token_or_resume_token_in_log_or_telemetry() -> void:
	Log.set_level_name("debug")
	await h.connect_online()
	h.push(h.envelope("system.heartbeat.pong", {}))
	h.peer().server_close(-1)
	h.client.tick()
	var all: String = _everything_logged()
	Log.set_level_name("info")
	assert_bool(all.contains(RealtimeHarness.TICKET)).is_false()
	assert_bool(all.contains(RealtimeHarness.ACCESS)).is_false()
	assert_bool(all.contains("rt-example-opaque-0001")).is_false()
	assert_bool(all.contains("realtime_state")).is_true()


func test_logger_redacts_ticket_fields_and_urls() -> void:
	var out: Dictionary = Log.redact({"ticket": "abc", "resume_token": "def", "url": "wss://x/realtime/v1?ticket=SECRET1&a=1"}) as Dictionary
	assert_str(String(out["ticket"])).is_equal(Log.REDACTED)
	assert_str(String(out["resume_token"])).is_equal(Log.REDACTED)
	assert_bool(String(out["url"]).contains("SECRET1")).is_false()


func test_release_build_refuses_ws_scheme() -> void:
	h.client.allow_insecure = false
	h.reply_ticket("ws://rt.test.invalid/realtime/v1")
	h.client.start(RealtimeHarness.SESSION_ID)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(h.peers.is_empty()).is_true()
	# Debug builds may use the local compose stack over ws://.
	h.client.allow_insecure = true
	h.advance_ms(700)
	await h.await_ticket()
	assert_int(h.client.state).is_equal(STATE.CONNECTING)


func test_rest_heartbeat_stops_while_online_but_the_tick_continues() -> void:
	var sessions: GameSessionService = GameSessionService.new(h.api, h.clock, "1.2.3", "windows")
	sessions.game_session_id = RealtimeHarness.SESSION_ID
	sessions.heartbeat_interval_sec = 1.0
	h.http.reply("POST /game/sessions/*:heartbeat", 204)
	var ticks: Array[int] = []
	sessions.heartbeat_ticked.connect(func() -> void: ticks.append(1))
	sessions.realtime_online = true
	sessions.start_heartbeat()
	for i: int in 8:
		await get_tree().process_frame
	assert_int(sessions.rest_beats).is_equal(0)
	assert_int(ticks.size()).is_greater(2)
	sessions.realtime_online = false
	for i: int in 8:
		await get_tree().process_frame
	assert_int(sessions.rest_beats).is_greater(0)
	sessions.stop_heartbeat()


func _statuses() -> Array:
	return h.peer().sent_of("presence.status.set").map(func(f: Dictionary) -> String:
		return String((f["payload"] as Dictionary)["status"]))


func test_presence_follows_activity_and_idle() -> void:
	var presence: PresenceService = PresenceService.new(h.client, h.clock)
	await h.connect_online()
	# The gateway starts the connection as online: nothing to send.
	assert_array(_statuses()).is_empty()
	presence.screen_opened(&"activity_host")
	assert_array(_statuses()).contains_exactly(["in_activity"])
	assert_str(String((h.peer().last_sent()["payload"] as Dictionary)["activity"])).is_equal("learning")
	presence.screen_opened(&"activity_host")
	assert_int(_statuses().size()).is_equal(1)
	presence.screen_closed(&"activity_host")
	presence.set_activity(PresenceService.IN_MATCH, "battle")
	assert_array(_statuses()).contains_exactly(["in_activity", "online", "in_match"])
	h.clock.advance(299.0)
	presence.tick()
	assert_int(_statuses().size()).is_equal(3)
	h.clock.advance(2.0)
	presence.tick()
	assert_array(_statuses()).contains_exactly(["in_activity", "online", "in_match", "away"])
	presence.touch()
	assert_array(_statuses()).contains_exactly(["in_activity", "online", "in_match", "away", "in_match"])
	# Never `offline`: only the gateway sets it.
	assert_bool("offline" in _statuses()).is_false()


func test_presence_is_sent_again_after_a_new_connection() -> void:
	var presence: PresenceService = PresenceService.new(h.client, h.clock)
	await h.connect_online()
	presence.set_activity(PresenceService.IN_ACTIVITY, "quest")
	assert_int(_statuses().size()).is_equal(1)
	h.peer().server_close(-1)
	h.client.tick()
	h.advance_ms(700)
	await h.await_ticket()
	h.open_with_welcome(1, "rt-x")
	h.push({"v": 1, "type": "system.session.resumed", "seq": h.client.session.last_seq + 1, "ts": 1, "payload": {}})
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_array(_statuses()).contains_exactly(["in_activity"])


## T030 (OQ pinning 2026-11-20): the release build connects with `TLSOptions.client(trusted_chain)` of the packaged CA
## bundle; without a bundle the engine CAs apply. (The refusal of a certificate outside the bundle needs a TLS server
## and the F133 bundle — lab/integration.)
func test_ca_bundle_becomes_the_trusted_chain() -> void:
	assert_object(GameRealtimeClient.load_tls_options("user://f092_missing_bundle.crt")).is_null()
	var crypto: Crypto = Crypto.new()
	var key: CryptoKey = crypto.generate_rsa(2048)
	var cert: X509Certificate = crypto.generate_self_signed_certificate(key, "CN=eduworld-test-ca,O=EduWorld,C=VN")
	cert.save("user://f092_bundle.crt")
	var tls: TLSOptions = GameRealtimeClient.load_tls_options("user://f092_bundle.crt")
	assert_object(tls).is_not_null()
	h.client.tls_options = tls
	await h.connect_online()
	assert_object(h.peer().tls).is_same(tls)
	DirAccess.remove_absolute("user://f092_bundle.crt")
