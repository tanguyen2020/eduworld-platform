extends GdUnitTestSuite
## T028, T031, T034, T035, T037, T039, T043 (F109-AC-16…AC-18, AC-21…AC-24, AC-26, AC-28, AC-29; TC-TEN-001,
## TC-TEN-024): LeaderboardService, TournamentService, MatchHistoryService on FakeHttp with the F109 fixtures —
## - boards are exactly the server list; a 404 page drops its board; only server ids are sent back; there is no refresh
##   timer and no "bị vượt qua" message anywhere in the arena code (NFR-SAFE-003);
## - tiers from `getMyRatings`, a `competition.tier.updated` reloads them;
## - tournaments: list / detail, register keyed, check-in only inside the server window, tournament problems become
##   friendly sentences; `tournament.match.scheduled` raises the toast; a `tournament_update` notification opens the
##   tournament tab (F123 toast);
## - history: `mode` filter + cursor, detail 404 → "Không tìm thấy" and nothing kept; report (keyed, reason of the
##   enum, match context) and block of an opponent.

var h: RealtimeHarness = null
var lb: LeaderboardService = null
var ts: TournamentService = null
var mh: MatchHistoryService = null
var notices: Array[String] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	lb = LeaderboardService.new(h.api, h.client)
	ts = TournamentService.new(h.api, h.client)
	mh = MatchHistoryService.new(h.api, ReportService.new(h.api), SocialService.new(h.api))
	notices.clear()
	ts.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	mh.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	CompetitionFixtures.script(h.http)


func after_test() -> void:
	h.client.unregister_handler(LeaderboardService.PREFIX, lb.handle_message)
	h.client.unregister_handler(TournamentService.PREFIX, ts.handle_message)
	h.dispose()


# ---------- leaderboards ----------

func test_boards_are_the_server_list_and_ids_are_sent_back_as_given() -> void:
	assert_bool((await lb.load_boards()).ok).is_true()
	assert_int(lb.boards.size()).is_equal(3)
	var b: LeaderboardSummaryModel = lb.boards[1]
	await lb.load_page(b)
	var rec: FakeHttp.Recorded = h.http.last(CompetitionFixtures.ROUTE_BOARD)
	assert_str(rec.path).is_equal("/leaderboards/%s" % b.leaderboard_id)
	assert_str(rec.query).contains("scope_id=%s" % b.scope_id)
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(RealtimeHarness.TENANT_ID)
	assert_object(lb.page(b.leaderboard_id)).is_not_null()


func test_empty_list_and_a_closed_board() -> void:
	h.http.clear(CompetitionFixtures.ROUTE_BOARDS).reply(CompetitionFixtures.ROUTE_BOARDS, 200, {"data": []})
	await lb.load_boards()
	assert_bool(lb.boards_loaded and lb.boards.is_empty()).is_true()
	h.http.clear(CompetitionFixtures.ROUTE_BOARDS).reply(CompetitionFixtures.ROUTE_BOARDS, 200,
			CompetitionFixtures.openapi("listLeaderboards.200"))
	await lb.load_boards()
	var closed: Array[String] = []
	lb.board_closed.connect(func(id: String) -> void: closed.append(id))
	h.http.clear(CompetitionFixtures.ROUTE_BOARD).reply(CompetitionFixtures.ROUTE_BOARD, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await lb.load_page(lb.boards[0])
	assert_array(closed).is_equal([CompetitionFixtures.LB_GOAL])
	assert_int(lb.boards.size()).is_equal(2)


func test_tiers_and_tier_update() -> void:
	await lb.load_tiers()
	assert_str(lb.tier_for("battle_1v1").label()).is_equal("Bạc II")
	var before: int = h.http.count(CompetitionFixtures.ROUTE_RATINGS)
	lb.handle_message("competition.tier.updated", CompetitionFixtures.ws("competition.tier.updated", "silver"))
	await get_tree().process_frame
	await get_tree().process_frame
	assert_int(h.http.count(CompetitionFixtures.ROUTE_RATINGS)).is_equal(before + 1)


func test_no_refresh_timer_and_no_overtaken_message_in_the_arena_code() -> void:
	for path: String in ["res://scripts/competition/leaderboard_service.gd", "res://scenes/competition/leaderboard_view.gd",
			"res://scenes/competition/arena_screen.gd"]:
		var src: String = FileAccess.get_file_as_string(path)
		assert_str(src).not_contains("Timer.new")
		assert_str(src).not_contains("create_timer")
	var po: String = FileAccess.get_file_as_string("res://locale/vi.po")
	for line: String in po.split("\n"):
		if line.begins_with("msgid \"ui.leaderboard.") or line.begins_with("msgid \"ui.arena."):
			assert_str(line).not_contains("overtaken")
	assert_str(po).not_contains("vượt qua em")


# ---------- tournaments ----------

func test_list_detail_register_and_check_in_window() -> void:
	await ts.load_list()
	assert_int(ts.tournaments.size()).is_equal(1)
	h.http.reply(CompetitionFixtures.ROUTE_REGISTER, 201, CompetitionFixtures.openapi("registerTournamentEntry.201"))
	assert_bool((await ts.register(CompetitionFixtures.TOURNAMENT_ID)).ok).is_true()
	assert_bool(Uuid.is_valid(String(h.http.last(CompetitionFixtures.ROUTE_REGISTER).headers.get("idempotency-key", "")))).is_true()
	var t: TournamentModel = ts.tournament(CompetitionFixtures.TOURNAMENT_ID)
	assert_bool(t.registered()).is_true()
	# The check-in button follows the server window only.
	assert_bool(t.can_check_in(t.check_in_opens_ms - 1000)).is_false()
	assert_bool(t.can_check_in(t.check_in_opens_ms + 1000)).is_true()
	assert_bool(t.can_check_in(t.starts_ms + 1000)).is_false()
	h.http.reply(CompetitionFixtures.ROUTE_CHECK_IN, 200, CompetitionFixtures.openapi("checkInTournamentEntry.200"))
	assert_bool((await ts.check_in(t)).ok).is_true()
	assert_bool(ts.tournament(CompetitionFixtures.TOURNAMENT_ID).checked_in()).is_true()


func test_tournament_problems_are_friendly_sentences() -> void:
	await ts.load_detail(CompetitionFixtures.TOURNAMENT_ID)
	for pair: Array in [[409, "CONFLICT", "ui.tournament.error.full"], [403, "POLICY_DENIED", "ui.tournament.error.not_eligible"],
			[422, "INVALID_STATE_TRANSITION", "ui.tournament.error.window_closed"]]:
		h.http.clear(CompetitionFixtures.ROUTE_REGISTER).reply(CompetitionFixtures.ROUTE_REGISTER, int(pair[0]),
				AuthHarness.problem(int(pair[0]), String(pair[1])))
		await ts.register(CompetitionFixtures.TOURNAMENT_ID)
		assert_str(notices[notices.size() - 1]).is_equal(String(pair[2]))
	h.http.clear(CompetitionFixtures.ROUTE_TOURNAMENT).reply(CompetitionFixtures.ROUTE_TOURNAMENT, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await ts.load_detail(CompetitionFixtures.TOURNAMENT_ID)
	assert_object(ts.details.get(CompetitionFixtures.TOURNAMENT_ID)).is_null()
	assert_str(notices[notices.size() - 1]).is_equal("ui.arena.error.NOT_FOUND")


func test_match_notice_raises_the_toast_and_notification_opens_the_tab() -> void:
	var toasts: Array[String] = []
	ts.toast_requested.connect(func(key: String, _a: Dictionary, id: String) -> void: toasts.append("%s|%s" % [key, id]))
	ts.handle_message("tournament.match.scheduled", CompetitionFixtures.ws("tournament.match.scheduled", "ten_minutes"))
	assert_array(toasts).is_equal(["ui.tournament.toast.match_soon|%s" % CompetitionFixtures.TOURNAMENT_ID])
	var notes: NotificationService = NotificationService.new(h.api, null)
	var item: NotificationModel = NotificationModel.from_dict(CompetitionFixtures.ws("notification.item.created", "tournament_update"))
	assert_bool(notes.should_popup(item)).is_true()
	assert_str(String(notes.target_screen(item))).is_equal("arena")


func test_grade_challenge_join_is_keyed() -> void:
	await ts.load_challenges()
	assert_int(ts.challenges.size()).is_equal(2)
	h.http.reply(CompetitionFixtures.ROUTE_JOIN, 200, CompetitionFixtures.openapi("joinChallenge.200"))
	var r: ApiResult = await ts.join_challenge(CompetitionFixtures.QUIZ_CHALLENGE)
	assert_str(DataRead.str_of(r.body(), "match_id")).is_equal(CompetitionFixtures.QUIZ_MATCH_ID)
	assert_bool(Uuid.is_valid(String(h.http.last(CompetitionFixtures.ROUTE_JOIN).headers.get("idempotency-key", "")))).is_true()


# ---------- history ----------

func test_history_filter_cursor_and_detail() -> void:
	await mh.load_history("battle_1v1")
	assert_str(h.http.last(CompetitionFixtures.ROUTE_MATCHES).query).contains("mode=battle_1v1")
	assert_int(mh.matches.size()).is_equal(2)
	assert_bool(mh.has_more()).is_true()
	await mh.load_history("", true)
	assert_str(h.http.last(CompetitionFixtures.ROUTE_MATCHES).query).contains("cursor=")
	assert_int(mh.matches.size()).is_equal(4)
	await mh.load_detail(CompetitionFixtures.MATCH_ID)
	var m: MatchRecord = mh.details[CompetitionFixtures.MATCH_ID] as MatchRecord
	assert_int(m.review.size()).is_equal(3)
	assert_int(m.others(CompetitionFixtures.SELF_SAMPLE).size()).is_equal(1)


func test_detail_of_another_students_match_shows_nothing() -> void:
	h.http.clear(CompetitionFixtures.ROUTE_MATCH).reply(CompetitionFixtures.ROUTE_MATCH, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await mh.load_detail(CompetitionFixtures.MATCH_ID)
	assert_bool(mh.details.has(CompetitionFixtures.MATCH_ID)).is_false()
	assert_array(notices).is_equal(["ui.arena.error.NOT_FOUND"])


func test_report_and_block_an_opponent() -> void:
	h.http.reply(CompetitionFixtures.ROUTE_REPORT, 201, {"report_id": "0193b0a0-0000-7000-8000-00000000c101", "status": "open"})
	var r: ApiResult = await mh.report_opponent(CompetitionFixtures.OPPONENT, "rude_language", CompetitionFixtures.MATCH_ID)
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(CompetitionFixtures.ROUTE_REPORT)
	assert_str(String(rec.body["target_type"])).is_equal("user")
	assert_str(String((rec.body["context"] as Dictionary)["match_id"])).is_equal(CompetitionFixtures.MATCH_ID)
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_array(notices).contains(["ui.arena.safety.reported"])
	h.http.reply(CompetitionFixtures.ROUTE_BLOCK, 204)
	assert_bool((await mh.block_opponent(CompetitionFixtures.OPPONENT)).ok).is_true()
	assert_array(notices).contains(["ui.arena.safety.blocked"])
	# A reason outside the enum is never sent.
	var before: int = h.http.count(CompetitionFixtures.ROUTE_REPORT)
	await mh.report_opponent(CompetitionFixtures.OPPONENT, "made_up_reason")
	assert_int(h.http.count(CompetitionFixtures.ROUTE_REPORT)).is_equal(before)
