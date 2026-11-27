extends GdUnitTestSuite
## T018, T021 (F109-AC-05…AC-11; TC-CMP-003…007, ADR-0016, AGENTS R5): BattleClient on a RealtimeHarness with the
## AsyncAPI fixtures:
## - the snapshot builds the whole view (scores, progress, deadline, question without answer key);
## - one tap = exactly one `battle.answer.submit` (match, question instance, raw response, reference elapsed time, no
##   score), the input locks at once; a second answer is not sent;
## - `ANSWER_LATE` / `ANSWER_ALREADY_SUBMITTED` / `MATCH_NOT_ACTIVE` keep the lock and are never re-sent;
## - verdict / points only from `battle.answer.evaluated`, scores from `battle.question.revealed`;
## - an older snapshot is dropped; a reconnect asks `battle.state.request` and rebuilds from the snapshot only;
## - status / pause / resume (replay of the current question) / end;
## - the server deltas of 2026-11-27: `speed_bonus_enabled` (countdown / snapshot), `primary_user_ids` (team battle),
##   `chat_channel_ids` (kept by a later snapshot without them), options keyed `id` (an `option_id` alone still works);
##   a server without them reads safely (no speed hint, no primary marker, no chat ids).

const SELF_ID: String = "0193b0a0-0000-7000-8000-0000000000a1"

var h: RealtimeHarness = null
var bc: BattleClient = null
var notices: Array[String] = []
var locked: int = 0


func before_test() -> void:
	h = RealtimeHarness.create()
	await h.connect_online()
	bc = BattleClient.new(h.client)
	notices.clear()
	locked = 0
	bc.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	bc.answer_locked.connect(func() -> void: locked += 1)
	bc.start(CompetitionFixtures.MATCH_ID, "battle_1v1")
	bc.self_id = SELF_ID
	await get_tree().process_frame


func after_test() -> void:
	h.client.unregister_handler(BattleClient.PREFIX, bc.handle_message)
	h.dispose()


func _snapshot(name: String = "question_1v1") -> Dictionary:
	var p: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", name)
	bc.handle_message("battle.state.snapshot", p)
	return p


func test_start_asks_for_the_full_state() -> void:
	var sent: Array[Dictionary] = h.peer().sent_of(BattleClient.CMD_STATE)
	assert_int(sent.size()).is_equal(1)
	assert_str(String((sent[0]["payload"] as Dictionary)["match_id"])).is_equal(CompetitionFixtures.MATCH_ID)


func test_snapshot_builds_the_view_from_the_server() -> void:
	_snapshot()
	var v: BattleView = bc.view
	assert_str(v.state).is_equal(BattleView.STATE_QUESTION)
	assert_int(v.question_count).is_equal(3)
	assert_int(v.participants.size()).is_equal(2)
	assert_str(v.question.type).is_equal("single_choice")
	# The pinned rule of this match (server): the speed hint is on; the match chat id is the server's.
	assert_bool(v.speed_bonus_enabled).is_true()
	assert_dict(v.chat_channel_ids).is_equal({"match": "5bc7f014-e25a-5bc9-b357-7c1090ed139b"})
	assert_array(v.primary_user_ids).is_empty()
	var options: Array = v.question.render_payload["options"] as Array
	assert_str(String((options[1] as Dictionary)["id"])).is_equal("o_b")
	assert_bool(bc.input_locked).is_false()


func test_one_tap_sends_exactly_one_answer_and_locks() -> void:
	_snapshot()
	var job: Callable = func() -> void: await bc.submit_answer({"option_id": "o_b"})
	job.call()
	await get_tree().process_frame
	assert_int(locked).is_equal(1)
	assert_bool(bc.input_locked).is_true()
	# A second tap while the first is on its way sends nothing.
	assert_bool(await bc.submit_answer({"option_id": "o_c"})).is_false()
	var sent: Array[Dictionary] = h.peer().sent_of(BattleClient.CMD_SUBMIT)
	assert_int(sent.size()).is_equal(1)
	var payload: Dictionary = sent[0]["payload"] as Dictionary
	assert_str(String(payload["match_id"])).is_equal(CompetitionFixtures.MATCH_ID)
	assert_str(String(payload["question_instance_id"])).is_equal("qi_501_0")
	assert_dict(payload["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_bool(payload.has("client_elapsed_ms")).is_true()
	for forbidden: String in ["score", "points", "is_correct", "time_bonus"]:
		assert_bool(payload.has(forbidden)).is_false()
	h.ack_last(BattleClient.CMD_SUBMIT)
	await get_tree().process_frame
	assert_bool(bc.view.my_answer_locked).is_true()
	# No verdict before the server says so.
	assert_object(bc.view.evaluation).is_null()


func test_refusals_keep_the_lock_and_are_not_resent() -> void:
	for code: String in ["ANSWER_LATE", "ANSWER_ALREADY_SUBMITTED", "MATCH_NOT_ACTIVE"]:
		_snapshot()
		bc.view.state_version = 0
		var before: int = h.peer().sent_of(BattleClient.CMD_SUBMIT).size()
		var job: Callable = func() -> void: await bc.submit_answer({"option_id": "o_a"})
		job.call()
		await get_tree().process_frame
		h.ack_last(BattleClient.CMD_SUBMIT, false, code)
		await get_tree().process_frame
		assert_bool(bc.input_locked).override_failure_message(code).is_true()
		assert_str(notices[notices.size() - 1]).is_equal("ui.arena.error.%s" % code)
		assert_bool(await bc.submit_answer({"option_id": "o_a"})).is_false()
		assert_int(h.peer().sent_of(BattleClient.CMD_SUBMIT).size()).is_equal(before + 1)
		bc.view = null


func test_verdict_and_scores_come_only_from_the_server() -> void:
	_snapshot()
	var evals: Array[AnswerEvaluation] = []
	bc.answer_evaluated.connect(func(e: AnswerEvaluation) -> void: evals.append(e))
	bc.handle_message("battle.answer.recorded", CompetitionFixtures.ws("battle.answer.recorded", "opponent"))
	assert_bool(bc.view.player(CompetitionFixtures.OPPONENT).answered).is_true()
	bc.handle_message("battle.answer.evaluated", CompetitionFixtures.ws("battle.answer.evaluated", "correct_bonus"))
	assert_int(evals.size()).is_equal(1)
	assert_int(evals[0].time_bonus).is_equal(50)
	assert_int(evals[0].points).is_equal(150)
	bc.handle_message("battle.question.revealed", CompetitionFixtures.ws("battle.question.revealed", "battle_1v1"))
	assert_str(bc.view.state).is_equal(BattleView.STATE_REVEAL)
	assert_int(bc.view.player(SELF_ID).score).is_equal(150)
	assert_int(bc.view.player(CompetitionFixtures.OPPONENT).score).is_equal(0)


func test_an_older_snapshot_is_dropped_and_a_reconnect_rebuilds_from_the_snapshot() -> void:
	_snapshot()
	var old: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", "question_1v1")
	old["state_version"] = 3
	old["question_index"] = 2
	bc.handle_message("battle.state.snapshot", old)
	assert_int(bc.view.state_version).is_equal(7)
	assert_int(bc.view.question_index).is_equal(0)
	# The connection drops and comes back: the client asks for the state, the local view is replaced by the snapshot.
	var before: int = h.peer().sent_of(BattleClient.CMD_STATE).size()
	bc.view.player(SELF_ID).score = 999
	# The connection was coming back (Backoff → Online): the client asks for the state again.
	bc._was_reconnecting = true
	bc._on_connection_state_changed(0)
	await get_tree().process_frame
	assert_int(h.peer().sent_of(BattleClient.CMD_STATE).size()).is_equal(before + 1)
	var fresh: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", "question_1v1")
	fresh["state_version"] = 9
	fresh["my_answer_locked"] = true
	bc.handle_message("battle.state.snapshot", fresh)
	assert_int(bc.view.player(SELF_ID).score).is_equal(0)
	assert_bool(bc.input_locked).is_true()


func test_status_pause_resume_replay_and_end() -> void:
	_snapshot()
	var statuses: Array[String] = []
	bc.player_status_changed.connect(func(id: String, on: bool, _g: int) -> void: statuses.append("%s|%s" % [id, on]))
	bc.handle_message("battle.participant.status_changed", CompetitionFixtures.ws("battle.participant.status_changed", "opponent_disconnected"))
	assert_bool(bc.view.player(CompetitionFixtures.OPPONENT).connected).is_false()
	assert_int(bc.view.player(CompetitionFixtures.OPPONENT).grace_until).is_greater(0)
	bc.handle_message("battle.match.paused", CompetitionFixtures.ws("battle.match.paused", "server_failover"))
	assert_str(bc.view.state).is_equal(BattleView.STATE_PAUSED)
	bc.input_locked = true
	bc.handle_message("battle.match.resumed", CompetitionFixtures.ws("battle.match.resumed", "replay"))
	assert_str(bc.view.state).is_equal(BattleView.STATE_QUESTION)
	assert_bool(bc.input_locked).is_false()
	var ends: Array[MatchSummary] = []
	bc.match_ended.connect(func(s: MatchSummary) -> void: ends.append(s))
	bc.handle_message("battle.match.ended", CompetitionFixtures.ws("battle.match.ended", "win"))
	assert_int(ends.size()).is_equal(1)
	assert_str(String(ends[0].result_of(SELF_ID)["outcome"])).is_equal("win")
	assert_bool(bc.view.is_over()).is_true()
	assert_array(statuses).contains(["%s|false" % CompetitionFixtures.OPPONENT])


func test_messages_of_another_match_are_ignored() -> void:
	_snapshot()
	var other: Dictionary = CompetitionFixtures.ws("battle.match.ended", "team_win")
	bc.handle_message("battle.match.ended", other)
	assert_bool(bc.view.is_over()).is_false()


func test_team_snapshot_reads_primary_players_speed_rule_and_both_chat_channels() -> void:
	bc.match_id = CompetitionFixtures.TEAM_MATCH_ID
	bc.handle_message("battle.state.snapshot", CompetitionFixtures.ws("battle.state.snapshot", "team_battle"))
	assert_bool(bc.view.speed_bonus_enabled).is_false()
	assert_array(bc.view.primary_user_ids).is_equal(PackedStringArray([SELF_ID, CompetitionFixtures.OPPONENT]))
	assert_bool(bc.is_primary()).is_true()
	assert_dict(bc.view.chat_channel_ids).is_equal({"match": "4cf706cf-d9ed-54c4-ada6-d38bbb9140cf",
		"team": "a8bf5cd0-c9b5-5c6f-916f-392612711d25"})
	assert_int(bc.view.team_score(1)).is_equal(390)
	assert_int(bc.my_team()).is_equal(1)
	# The next question names other primary players (the student now helps).
	var shown: Dictionary = CompetitionFixtures.ws("battle.question.shown", "team_battle")
	shown["state_version"] = bc.view.state_version + 1
	bc.handle_message("battle.question.shown", shown)
	assert_bool(bc.is_primary()).is_equal(SELF_ID in DataRead.strings_of(shown, "primary_user_ids", 8))


func test_countdown_carries_the_speed_rule() -> void:
	var countdown: Dictionary = CompetitionFixtures.ws("battle.countdown.started", "battle_1v1")
	bc.handle_message("battle.countdown.started", countdown)
	assert_str(bc.view.state).is_equal(BattleView.STATE_COUNTDOWN)
	assert_bool(bc.view.speed_bonus_enabled).is_true()
	countdown["speed_bonus_enabled"] = false
	bc.view.state_version = 0
	bc.handle_message("battle.countdown.started", countdown)
	assert_bool(bc.view.speed_bonus_enabled).is_false()


func test_a_server_without_the_new_fields_reads_safely() -> void:
	_snapshot()
	var bare: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", "question_1v1")
	for key: String in ["speed_bonus_enabled", "chat_channel_ids", "primary_user_ids"]:
		bare.erase(key)
	bare["state_version"] = 8
	# An older server: options carry the deprecated `option_id` only.
	for o: Variant in (bare["current_question"] as Dictionary)["options"] as Array:
		(o as Dictionary).erase("id")
	bc.handle_message("battle.state.snapshot", bare)
	assert_bool(bc.view.speed_bonus_enabled).is_false()
	assert_array(bc.view.primary_user_ids).is_empty()
	# The chat ids are fixed for the match: the ones of the earlier snapshot stay.
	assert_str(String(bc.view.chat_channel_ids.get("match", ""))).is_equal("5bc7f014-e25a-5bc9-b357-7c1090ed139b")
	var options: Array = bc.view.question.render_payload["options"] as Array
	assert_str(String((options[0] as Dictionary)["id"])).is_equal("o_a")
	# A fresh match with no ids at all: nothing is invented.
	bc.start(CompetitionFixtures.MATCH_ID, "battle_1v1")
	bc.handle_message("battle.state.snapshot", bare)
	assert_dict(bc.view.chat_channel_ids).is_empty()
