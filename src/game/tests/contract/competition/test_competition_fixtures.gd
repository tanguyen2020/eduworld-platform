extends GdUnitTestSuite
## T009 (F109-AC-15, AC-20, ADR-0016): every F109 contract fixture of tools/gen_fixtures.mjs parses into its typed
## model, no fixture or model carries a raw rating / RD / rating delta, a real name or a school, and a question never
## carries an answer key before `battle.answer.evaluated`. A rating / PII field a server bug might add is dropped by the
## models (the screens cannot show what the model does not hold).

const OPENAPI_FILES: PackedStringArray = [
	"listCompetitionModes.200", "listSubjects.200", "listTopics.200", "createMatchmakingTicket.201",
	"getMatchmakingTicket.200", "startBotMatch.201", "getMyRatings.200", "listMyMatches.200", "getMatch.200",
	"listLeaderboards.200", "getLeaderboard.200.grade_goal", "getLeaderboard.200.top_n_percentile",
	"getLeaderboard.200.ranked", "listTournaments.200", "getTournament.200", "registerTournamentEntry.201",
	"checkInTournamentEntry.200", "getTournamentBracket.200", "listMyChallenges.200", "joinChallenge.200",
	"createDuel.201", "acceptDuel.200",
]
const PII_KEYS: PackedStringArray = ["display_name", "full_name", "real_name", "school_name", "class_name", "student_code"]


func _all_fixtures() -> Array:
	var out: Array = []
	for name: String in OPENAPI_FILES:
		out.append(CompetitionFixtures.openapi(name))
	var dir: String = RealtimeHarness.FIXTURES
	for file: String in DirAccess.get_files_at(dir):
		for prefix: String in ["matchmaking.", "battle.", "competition.", "tournament."]:
			if file.begins_with(prefix):
				out.append(JSON.parse_string(FileAccess.get_file_as_string("%s/%s" % [dir, file])))
	return out


static func _keys_in(value: Variant, keys: PackedStringArray) -> PackedStringArray:
	var found: PackedStringArray = []
	if value is Dictionary:
		for k: Variant in (value as Dictionary).keys():
			if String(k) in keys:
				found.append(String(k))
			found.append_array(_keys_in((value as Dictionary)[k], keys))
	elif value is Array:
		for v: Variant in value as Array:
			found.append_array(_keys_in(v, keys))
	return found


func test_no_fixture_has_a_raw_rating_or_personal_data() -> void:
	var fixtures: Array = _all_fixtures()
	assert_int(fixtures.size()).is_greater(40)
	for f: Variant in fixtures:
		assert_array(TierView.find_raw_rating(f)).is_empty()
		assert_array(_keys_in(f, PII_KEYS)).is_empty()


func test_modes_ticket_tiers_and_duels_parse() -> void:
	var modes: Array[ArenaModeModel] = ArenaModeModel.list_from(CompetitionFixtures.openapi("listCompetitionModes.200")["data"])
	assert_int(modes.size()).is_equal(3)
	assert_bool(modes[0].allows_topic()).is_true()
	assert_bool(modes[2].ranked).is_false()
	var t: MatchmakingTicketModel = MatchmakingTicketModel.from_dict(CompetitionFixtures.openapi("getMatchmakingTicket.200"))
	assert_int(t.wait_ms).is_equal(24000)
	assert_bool(t.is_active()).is_true()
	var tiers: Array[TierView] = TierView.list_from(CompetitionFixtures.openapi("getMyRatings.200")["data"])
	assert_int(tiers.size()).is_equal(2)
	assert_str(tiers[0].label()).is_equal("Bạc II")
	assert_bool(tiers[1].provisional).is_true()
	assert_str(tiers[1].label()).contains("2/5")
	var duel: DuelModel = DuelModel.from_dict(CompetitionFixtures.openapi("acceptDuel.200"))
	assert_bool(Uuid.is_valid(duel.match_id)).is_true()


func test_a_raw_rating_added_by_a_server_bug_never_reaches_a_model() -> void:
	var rating: Dictionary = (CompetitionFixtures.openapi("getMyRatings.200")["data"] as Array)[0]
	rating["rating"] = 1523.4
	rating["rd"] = 61.0
	var tier: TierView = TierView.from_dict(rating)
	assert_str(tier.label()).not_contains("1523")
	assert_bool("rating" in tier.get_property_list().map(func(p: Dictionary) -> String: return String(p["name"]))).is_false()
	var match: Dictionary = CompetitionFixtures.openapi("getMatch.200")
	((match["participants"] as Array)[0] as Dictionary)["rating_delta"] = 14
	var record: MatchRecord = MatchRecord.from_dict(match)
	for p: Dictionary in record.participants:
		assert_bool(p.has("rating_delta")).is_false()
	var ended: Dictionary = CompetitionFixtures.ws("battle.match.ended", "win")
	((ended["results"] as Array)[0] as Dictionary)["rating_delta"] = 14
	var s: MatchSummary = MatchSummary.from_dict(ended)
	assert_bool(s.results[0].has("rating_delta")).is_false()


func test_leaderboard_pages_follow_the_server_visibility() -> void:
	var goal: LeaderboardPageModel = LeaderboardPageModel.from_dict(CompetitionFixtures.board("grade_goal"))
	assert_str(goal.visibility).is_equal(LeaderboardPageModel.VIS_GRADE_GOAL)
	assert_array(goal.entries).is_empty()
	assert_int(int(goal.grade_goal["contributors_count"])).is_equal(38)
	var top: LeaderboardPageModel = LeaderboardPageModel.from_dict(CompetitionFixtures.board("top_n_percentile"))
	assert_int(int(top.me["percentile_bucket"])).is_equal(25)
	assert_object(top.my_entry()).is_null()
	var ranked: LeaderboardPageModel = LeaderboardPageModel.from_dict(CompetitionFixtures.board("ranked"))
	assert_bool(ranked.stale).is_true()
	assert_int(ranked.extra_around().size()).is_equal(1)
	# A grade-goal board never lists students even if a server bug sends rows (ADR-0095).
	var bad: Dictionary = CompetitionFixtures.board("grade_goal")
	bad["entries"] = CompetitionFixtures.board("ranked")["entries"]
	assert_array(LeaderboardPageModel.from_dict(bad).entries).is_empty()


func test_tournaments_bracket_and_challenges_parse() -> void:
	var list: Array[TournamentModel] = TournamentModel.list_from(CompetitionFixtures.openapi("listTournaments.200")["data"])
	assert_int(list.size()).is_equal(1)
	assert_bool(list[0].can_register()).is_true()
	var detail: TournamentModel = TournamentModel.from_dict(CompetitionFixtures.openapi("getTournament.200"))
	assert_bool(detail.registered()).is_true()
	var b: BracketModel = BracketModel.from_dict(CompetitionFixtures.openapi("getTournamentBracket.200"))
	assert_int(b.rounds()).is_equal(2)
	assert_str(b.name_of(String(b.nodes[0]["entry_a"]))).is_equal("MeoXanh42")
	assert_int(b.path_of(b.entry_of_player(CompetitionFixtures.SELF_SAMPLE)).size()).is_equal(2)
	var challenges: Array[GradeChallengeModel] = GradeChallengeModel.list_from(CompetitionFixtures.openapi("listMyChallenges.200")["data"])
	assert_int(challenges.size()).is_equal(2)
	assert_bool(challenges[0].is_timed_quiz()).is_true()


func test_battle_question_has_no_answer_key_before_the_evaluation() -> void:
	var snap: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", "question_1v1")
	var v: BattleView = BattleView.from_snapshot(snap)
	assert_object(v.question).is_not_null()
	assert_str(v.question.type).is_equal("single_choice")
	assert_array(QuestionRenderModel.find_answer_keys(v.question.render_payload)).is_empty()
	assert_str(String(((v.question.render_payload["options"] as Array)[1] as Dictionary)["id"])).is_equal("o_b")
	var shown: Dictionary = CompetitionFixtures.ws("battle.question.shown", "single_choice")
	assert_bool(_keys_in(shown, PackedStringArray(["correct_response"])).is_empty()).is_true()
	var evaluated: Dictionary = CompetitionFixtures.ws("battle.answer.evaluated", "correct_bonus")
	assert_dict(AnswerEvaluation.from_dict(evaluated).correct_response).is_equal({"option_id": "o_b"})
	# An answer key slipped into the question is stripped before any renderer sees it (R6).
	var leaked: Dictionary = (snap["current_question"] as Dictionary).duplicate(true)
	leaked["correct_option_id"] = "o_b"
	assert_bool(BattleView.question_from(leaked, 0).render_payload.has("correct_option_id")).is_false()


func test_every_match_end_shape_parses() -> void:
	for name: String in ["win", "loss", "draw", "cancelled", "team_win", "timed_quiz_grade_goal", "timed_quiz_rank"]:
		var s: MatchSummary = MatchSummary.from_dict(CompetitionFixtures.ws("battle.match.ended", name))
		assert_object(s).override_failure_message(name).is_not_null()
	assert_bool(MatchSummary.from_dict(CompetitionFixtures.ws("battle.match.ended", "cancelled")).is_void()).is_true()
	var quiz: MatchSummary = MatchSummary.from_dict(CompetitionFixtures.ws("battle.match.ended", "timed_quiz_grade_goal"))
	assert_array(quiz.top).is_empty()
	assert_int(int(quiz.grade_goal["contributors_count"])).is_equal(41)
	var rank: MatchSummary = MatchSummary.from_dict(CompetitionFixtures.ws("battle.match.ended", "timed_quiz_rank"))
	assert_int(rank.my_rank).is_equal(3)
	assert_dict(rank.grade_goal).is_empty()
