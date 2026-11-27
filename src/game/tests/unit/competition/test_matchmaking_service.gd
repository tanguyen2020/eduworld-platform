extends GdUnitTestSuite
## T012–T014, T041, T043 (F109-AC-02, AC-03, AC-04, AC-27, AC-28; TC-MM-001, TC-CMP-002, TC-CMP-010, TC-TEN-001):
## MatchmakingService on a RealtimeHarness (FakeHttp + FakeWebSocketPeer, TEST-ONLY) with the F109 fixtures:
## - `createMatchmakingTicket` sends mode / subject / topic (+ `matchmaking_type = topic`) with one Idempotency-Key per
##   intent, the same key while the intent is retried, and `X-Tenant-Id` of the session;
## - `matchmaking.ticket.updated` drives the ticket; cancel goes over WS, over REST when the socket is down;
## - the ready check: proposal → accept over WS (REST fallback), `requeued` back to the queue, `match.assigned` →
##   `match_assigned`;
## - `409 MM_TICKET_ALREADY_ACTIVE` reads the held ticket again, a timeout ends the queue with "timeout", a lost
##   ticket after a reconnect says "Hàng chờ đã kết thúc";
## - duels: `createDuel` keyed, `410 DUEL_EXPIRED` and `SOCIAL_SCOPE_VIOLATION` are friendly sentences (no reason);
## - server deltas 2026-11-27: `match.assigned` carries the match chat ids; a 409 `ticket_id` reopens that ticket;
##   `competition.duel.received` raises `duel_received` with the challenger card (not when quiet / expired).

var h: RealtimeHarness = null
var mm: MatchmakingService = null
var notices: Array[String] = []
var assigned: Array[String] = []
var ended: Array[String] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	await h.connect_online()
	mm = MatchmakingService.new(h.api, h.client)
	notices.clear()
	assigned.clear()
	ended.clear()
	mm.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	mm.match_assigned.connect(func(id: String, mode: String) -> void: assigned.append("%s|%s" % [id, mode]))
	mm.queue_ended.connect(func(reason: String) -> void: ended.append(reason))
	CompetitionFixtures.script(h.http)


func after_test() -> void:
	h.client.unregister_handler(MatchmakingService.PREFIX, mm.handle_message)
	h.client.unregister_handler(MatchmakingService.PREFIX_COMPETITION, mm.handle_competition_message)
	h.dispose()


func _key(route: String) -> String:
	return String(h.http.last(route).headers.get("idempotency-key", ""))


func _ticket_created() -> void:
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_CREATE).reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 201,
			CompetitionFixtures.openapi("createMatchmakingTicket.201"))


func test_modes_come_from_the_server_only() -> void:
	var result: ApiResult = await mm.load_modes()
	assert_bool(result.ok).is_true()
	var codes: Array[String] = []
	for m: ArenaModeModel in mm.modes:
		codes.append(m.mode)
	assert_array(codes).is_equal(["battle_1v1", "team_battle", "coop_challenge"])
	await mm.load_subjects()
	assert_str(mm.subject_name(CompetitionFixtures.MATH)).is_not_empty()
	assert_array(Array(mm.subjects_of("battle_1v1"))).contains([CompetitionFixtures.MATH])


func test_find_match_keeps_its_key_while_retried_and_sends_the_tenant() -> void:
	await mm.load_modes()
	h.http.fail(CompetitionFixtures.ROUTE_TICKET_CREATE)
	var first: ApiResult = await mm.find_match("battle_1v1", CompetitionFixtures.MATH, "0193b0a0-0000-7000-8000-000000000611")
	assert_bool(first.network_error).is_true()
	var key_1: String = _key(CompetitionFixtures.ROUTE_TICKET_CREATE)
	assert_bool(Uuid.is_valid(key_1)).is_true()
	_ticket_created()
	var second: ApiResult = await mm.find_match("battle_1v1", CompetitionFixtures.MATH, "0193b0a0-0000-7000-8000-000000000611")
	assert_bool(second.ok).is_true()
	assert_str(_key(CompetitionFixtures.ROUTE_TICKET_CREATE)).is_equal(key_1)
	var rec: FakeHttp.Recorded = h.http.last(CompetitionFixtures.ROUTE_TICKET_CREATE)
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(RealtimeHarness.TENANT_ID)
	assert_str(String(rec.body["mode"])).is_equal("battle_1v1")
	assert_str(String(rec.body["matchmaking_type"])).is_equal("topic")
	assert_bool(rec.body.has("score")).is_false()
	assert_bool(mm.in_queue()).is_true()
	# A new intent after the queue ended gets a new key.
	mm._end_queue("cancelled")
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	assert_str(_key(CompetitionFixtures.ROUTE_TICKET_CREATE)).is_not_equal(key_1)


func test_ticket_updates_and_cancel_over_ws_then_rest() -> void:
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	mm.handle_message("matchmaking.ticket.updated", CompetitionFixtures.ws("matchmaking.ticket.updated", "widened"))
	assert_bool(mm.ticket.search_widened).is_true()
	assert_bool(mm.ticket.bot_offer_available).is_true()
	assert_int(mm.ticket.wait_ms).is_equal(65000)
	var done: Array[bool] = []
	var job: Callable = func() -> void: done.append(await mm.cancel())
	job.call()
	await get_tree().process_frame
	assert_int(h.peer().sent_of(MatchmakingService.CMD_CANCEL).size()).is_equal(1)
	h.ack_last(MatchmakingService.CMD_CANCEL)
	await get_tree().process_frame
	assert_bool(done[0]).is_true()
	assert_object(mm.ticket).is_null()
	assert_array(ended).contains(["cancelled"])
	# Socket down: REST fallback `cancelMatchmakingTicket`.
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	h.client.stop()
	h.http.reply(CompetitionFixtures.ROUTE_TICKET_DELETE, 204)
	assert_bool(await mm.cancel()).is_true()
	assert_int(h.http.count(CompetitionFixtures.ROUTE_TICKET_DELETE)).is_equal(1)


func test_ready_check_accept_requeue_and_assignment() -> void:
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	mm.handle_message("matchmaking.proposal.created", CompetitionFixtures.ws("matchmaking.proposal.created", "battle_1v1"))
	assert_object(mm.proposal).is_not_null()
	assert_int(mm.proposal.participants.size()).is_equal(2)
	var job: Callable = func() -> void: await mm.accept_proposal()
	job.call()
	await get_tree().process_frame
	var sent: Array[Dictionary] = h.peer().sent_of(MatchmakingService.CMD_ACCEPT)
	assert_int(sent.size()).is_equal(1)
	assert_str(String((sent[0]["payload"] as Dictionary)["proposal_id"])).is_equal(CompetitionFixtures.MATCH_ID)
	h.ack_last(MatchmakingService.CMD_ACCEPT)
	await get_tree().process_frame
	assert_bool(mm.proposal.accepted).is_true()
	mm.handle_message("matchmaking.proposal.updated", CompetitionFixtures.ws("matchmaking.proposal.updated", "one_accepted"))
	assert_int(mm.proposal.accepted_count).is_equal(1)
	# Someone else declined: back in the queue (F109-AC-03).
	mm.handle_message("matchmaking.proposal.cancelled", CompetitionFixtures.ws("matchmaking.proposal.cancelled", "requeued"))
	assert_object(mm.proposal).is_null()
	assert_bool(mm.in_queue()).is_true()
	assert_bool(mm.ticket.requeued).is_true()
	mm.handle_message("matchmaking.match.assigned", CompetitionFixtures.ws("matchmaking.match.assigned", "battle_1v1"))
	assert_array(assigned).is_equal(["%s|battle_1v1" % CompetitionFixtures.MATCH_ID])
	assert_object(mm.ticket).is_null()
	# The server's match chat channel comes with the assignment (no team channel in 1v1).
	assert_dict(mm.chat_channels_for(CompetitionFixtures.MATCH_ID)).is_equal({"match": "5bc7f014-e25a-5bc9-b357-7c1090ed139b"})
	assert_dict(mm.chat_channels_for(CompetitionFixtures.TEAM_MATCH_ID)).is_empty()


func test_team_assignment_carries_the_match_and_own_team_channels() -> void:
	mm.handle_message("matchmaking.match.assigned", CompetitionFixtures.ws("matchmaking.match.assigned", "team_battle"))
	assert_array(assigned).is_equal(["%s|team_battle" % CompetitionFixtures.TEAM_MATCH_ID])
	assert_dict(mm.chat_channels_for(CompetitionFixtures.TEAM_MATCH_ID)).is_equal({
		"match": "4cf706cf-d9ed-54c4-ada6-d38bbb9140cf", "team": "a8bf5cd0-c9b5-5c6f-916f-392612711d25"})
	# A robot match afterwards has no ids from the assignment (the battle snapshot brings them).
	mm.enter_match(CompetitionFixtures.MATCH_ID, "battle_1v1")
	assert_dict(mm.chat_channels_for(CompetitionFixtures.MATCH_ID)).is_empty()


func test_accept_falls_back_to_rest_when_the_socket_is_down() -> void:
	mm.handle_message("matchmaking.proposal.created", CompetitionFixtures.ws("matchmaking.proposal.created", "battle_1v1"))
	h.client.stop()
	h.http.reply(CompetitionFixtures.ROUTE_ACCEPT, 204)
	assert_bool(await mm.accept_proposal()).is_true()
	assert_int(h.http.count(CompetitionFixtures.ROUTE_ACCEPT)).is_equal(1)


func test_already_active_reads_the_held_ticket_and_timeout_suggests_another_mode() -> void:
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_CREATE).reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 409,
			_active_without_id())
	var before: int = h.http.count(CompetitionFixtures.ROUTE_TICKET_GET)
	var result: ApiResult = await mm.find_match("team_battle", CompetitionFixtures.MATH)
	assert_str(result.code).is_equal("MM_TICKET_ALREADY_ACTIVE")
	assert_int(h.http.count(CompetitionFixtures.ROUTE_TICKET_GET)).is_equal(before + 1)
	assert_bool(mm.in_queue()).is_true()
	assert_array(notices).is_empty()
	mm.handle_message("matchmaking.ticket.updated", CompetitionFixtures.ws("matchmaking.ticket.updated", "expired"))
	assert_array(ended).contains(["timeout"])
	assert_object(mm.ticket).is_null()


func test_already_active_with_the_problem_ticket_id_reopens_that_ticket() -> void:
	# Nothing held in RAM (the app was opened again): the problem's `ticket_id` is read.
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_CREATE).reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 409,
			AuthHarness.problem(409, "MM_TICKET_ALREADY_ACTIVE", {"ticket_id": CompetitionFixtures.TICKET_ID}))
	var result: ApiResult = await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	assert_str(result.problem.ticket_id).is_equal(CompetitionFixtures.TICKET_ID)
	assert_str(h.http.last(CompetitionFixtures.ROUTE_TICKET_GET).url).contains(CompetitionFixtures.TICKET_ID)
	assert_bool(mm.in_queue()).is_true()
	assert_str(mm.ticket.ticket_id).is_equal(CompetitionFixtures.TICKET_ID)
	assert_array(notices).is_empty()


## `409 MM_TICKET_ALREADY_ACTIVE` without `ticket_id` (the held ticket is only another party member's).
func _active_without_id() -> Dictionary:
	var body: Dictionary = AuthHarness.problem(409, "MM_TICKET_ALREADY_ACTIVE")
	body.erase("ticket_id")
	return body


func test_already_active_without_a_held_ticket_says_so() -> void:
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_CREATE).reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 409,
			_active_without_id())
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	assert_array(notices).is_equal(["ui.arena.error.MM_TICKET_ALREADY_ACTIVE"])


func test_policy_denied_is_a_friendly_sentence() -> void:
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_CREATE).reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 403,
			AuthHarness.problem(403, "POLICY_DENIED"))
	await mm.find_match("team_battle", CompetitionFixtures.MATH)
	assert_array(notices).is_equal(["ui.arena.error.POLICY_DENIED"])
	assert_bool(L10n.has_key(notices[0])).is_true()


func test_a_lost_ticket_after_reconnect_ends_the_queue() -> void:
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	h.http.clear(CompetitionFixtures.ROUTE_TICKET_GET).reply(CompetitionFixtures.ROUTE_TICKET_GET, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await mm.refresh_ticket()
	assert_array(ended).contains(["lost"])
	assert_array(notices).contains([MatchmakingService.KEY_QUEUE_LOST])


func test_robot_match_is_keyed_and_opens_the_match() -> void:
	_ticket_created()
	await mm.find_match("battle_1v1", CompetitionFixtures.MATH)
	h.http.reply(CompetitionFixtures.ROUTE_BOT, 201, CompetitionFixtures.openapi("startBotMatch.201"))
	var result: ApiResult = await mm.start_bot_match()
	assert_bool(result.ok).is_true()
	assert_bool(Uuid.is_valid(_key(CompetitionFixtures.ROUTE_BOT))).is_true()
	assert_int(assigned.size()).is_equal(1)
	assert_object(mm.ticket).is_null()


func test_duel_create_accept_expired_and_scope() -> void:
	h.http.reply(CompetitionFixtures.ROUTE_DUEL_CREATE, 201, CompetitionFixtures.openapi("createDuel.201"))
	var sent: ApiResult = await mm.challenge_friend(CompetitionFixtures.OPPONENT, CompetitionFixtures.MATH)
	assert_bool(sent.ok).is_true()
	assert_bool(Uuid.is_valid(_key(CompetitionFixtures.ROUTE_DUEL_CREATE))).is_true()
	assert_str(String(h.http.last(CompetitionFixtures.ROUTE_DUEL_CREATE).body["opponent_id"])).is_equal(CompetitionFixtures.OPPONENT)
	var duel: DuelModel = DuelModel.from_dict(CompetitionFixtures.openapi("createDuel.201"))
	h.http.reply(CompetitionFixtures.ROUTE_DUEL_ACCEPT, 200, CompetitionFixtures.openapi("acceptDuel.200"))
	assert_bool((await mm.accept_duel(duel)).ok).is_true()
	assert_int(assigned.size()).is_equal(1)
	h.http.clear(CompetitionFixtures.ROUTE_DUEL_ACCEPT).reply(CompetitionFixtures.ROUTE_DUEL_ACCEPT, 410,
			AuthHarness.problem(410, "DUEL_EXPIRED"))
	var gone: ApiResult = await mm.accept_duel(duel)
	assert_str(CompetitionText.error_key(gone.code)).is_equal("ui.arena.error.DUEL_EXPIRED")
	h.http.clear(CompetitionFixtures.ROUTE_DUEL_CREATE).reply(CompetitionFixtures.ROUTE_DUEL_CREATE, 403,
			AuthHarness.problem(403, "SOCIAL_SCOPE_VIOLATION"))
	var scope: ApiResult = await mm.challenge_friend(CompetitionFixtures.OPPONENT, CompetitionFixtures.MATH)
	assert_str(CompetitionText.error_key(scope.code)).is_equal("ui.arena.error.SOCIAL_SCOPE_VIOLATION")
	# The general sentence never says why (friendship, grade band, block).
	var text: String = TranslationServer.translate("ui.arena.error.SOCIAL_SCOPE_VIOLATION").to_lower()
	for word: String in ["khối", "chặn", "bạn bè", "block", "grade"]:
		assert_str(text).not_contains(word)


func test_a_friends_duel_raises_the_prompt_with_the_public_card_only() -> void:
	var got: Array[DuelModel] = []
	mm.duel_received.connect(func(d: DuelModel) -> void: got.append(d))
	var payload: Dictionary = CompetitionFixtures.ws("competition.duel.received", "friend_duel")
	payload["expires_at"] = mm.now_ms() + 60000
	# Through the realtime dispatcher, as the server sends it.
	h.push(h.envelope("competition.duel.received", payload))
	await get_tree().process_frame
	assert_int(got.size()).is_equal(1)
	var duel: DuelModel = got[0]
	assert_str(duel.duel_id).is_equal(String(payload["duel_id"]))
	assert_str(duel.other_nickname).is_equal("CaHeoVui07")
	assert_str(duel.challenger_id).is_equal(CompetitionFixtures.OPPONENT)
	assert_str(duel.subject_id).is_equal(CompetitionFixtures.MATH)
	assert_bool(duel.is_pending()).is_true()
	assert_int(duel.seconds_left(mm.now_ms())).is_between(59, 60)
	assert_object(mm.incoming_duel).is_same(duel)
	# Declining answers the server and forgets the prompt.
	h.http.reply(CompetitionFixtures.ROUTE_DUEL_DECLINE, 200, CompetitionFixtures.openapi("createDuel.201"))
	await mm.decline_duel(duel)
	assert_int(h.http.count(CompetitionFixtures.ROUTE_DUEL_DECLINE)).is_equal(1)
	assert_object(mm.incoming_duel).is_null()


func test_a_duel_is_not_shown_when_quiet_expired_or_malformed() -> void:
	var got: Array[DuelModel] = []
	mm.duel_received.connect(func(d: DuelModel) -> void: got.append(d))
	var payload: Dictionary = CompetitionFixtures.ws("competition.duel.received", "friend_duel")
	payload["expires_at"] = mm.now_ms() + 60000
	mm.quiet_check = func() -> bool: return true
	mm.handle_competition_message("competition.duel.received", payload)
	mm.quiet_check = Callable()
	var late: Dictionary = payload.duplicate(true)
	late["expires_at"] = mm.now_ms() - 1000
	mm.handle_competition_message("competition.duel.received", late)
	var no_card: Dictionary = payload.duplicate(true)
	no_card.erase("challenger")
	mm.handle_competition_message("competition.duel.received", no_card)
	# Another competition message is not a duel.
	mm.handle_competition_message("competition.tier.updated", payload)
	assert_array(got).is_empty()
	assert_object(mm.incoming_duel).is_null()
