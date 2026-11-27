extends GdUnitTestSuite
## T029, T032, T033, T034, T038 (F109-AC-17…AC-21, AC-23…AC-25): the arena tabs on the contract fixtures —
## - Bảng xếp hạng: one chip per board the server returns; `grade_goal` → GradeGoalBar (collective progress, the
##   student's part, helpers only when the server gives them; no rank, no nickname of another student);
##   `top_n_percentile` → the rows + "nhóm 25%"; `ranked` → top + around me, "Đang cập nhật" when stale; rows show a
##   nickname and a portrait only; a 404 drops the tab with "Bảng này hiện không mở"; an empty list → empty state;
## - Lịch sử: cards with encouraging outcomes, the filter by mode, the detail with nicknames only and the review;
## - Giải đấu: list, detail with Đăng ký / Check-in by the server window, the bracket tree for lớp 6–9 and the simple
##   path for lớp 1–5 — nicknames only.

var s: CompetitionSceneHarness = null
var screen: ArenaScreen = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = CompetitionSceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	if screen != null:
		screen.free()
		screen = null
	s.dispose()


func _open(tab: String) -> void:
	screen = (load("res://scenes/competition/ArenaScreen.tscn") as PackedScene).instantiate() as ArenaScreen
	screen.services = s.services
	add_child(screen)
	await screen.open({"tab": tab})
	for i: int in 10:
		await await_idle_frame()


func _board(visibility: String) -> void:
	s.h.http.clear(CompetitionFixtures.ROUTE_BOARD).reply(CompetitionFixtures.ROUTE_BOARD, 200, CompetitionFixtures.board(visibility, s.self_id))


func test_grade_goal_board_has_no_rank_and_no_other_nickname() -> void:
	_board("grade_goal")
	await _open(ArenaScreen.TAB_LEADERBOARD)
	var view: LeaderboardView = screen.leaderboard()
	assert_int(view.board_chips().size()).is_equal(3)
	var bar: GradeGoalBar = view.grade_goal_bar()
	assert_object(bar).is_not_null()
	assert_float(bar.bar_value()).is_equal_approx(0.64, 0.001)
	assert_bool(bar.helpers_visible()).is_true()
	assert_int(view.rows().size()).is_equal(0)
	var words: String = " ".join(view.visible_texts())
	for nick: String in ["CaHeoVui07", "GauNau15", "SaoBang09"]:
		assert_str(words).not_contains(nick)
	assert_object(RegEx.create_from_string("#\\d").search(words)).is_null()


func test_grade_goal_helpers_hidden_below_the_cell_size() -> void:
	var page: Dictionary = CompetitionFixtures.board("grade_goal", s.self_id)
	(page["grade_goal"] as Dictionary)["contributors_count"] = null
	s.h.http.clear(CompetitionFixtures.ROUTE_BOARD).reply(CompetitionFixtures.ROUTE_BOARD, 200, page)
	await _open(ArenaScreen.TAB_LEADERBOARD)
	var bar: GradeGoalBar = screen.leaderboard().grade_goal_bar()
	assert_bool(bar.helpers_visible()).is_false()
	for t: String in bar.texts():
		assert_str(t).not_contains(" 0 ")


func test_top_n_percentile_rows_and_bucket() -> void:
	_board("top_n_percentile")
	await _open(ArenaScreen.TAB_LEADERBOARD)
	var view: LeaderboardView = screen.leaderboard()
	assert_int(view.rows().size()).is_equal(3)
	assert_str(view.percentile_text()).is_equal(tr("ui.leaderboard.percentile").format({"bucket": 25}))


func test_ranked_rows_show_nickname_and_avatar_only_and_stale_tag() -> void:
	_board("ranked")
	await _open(ArenaScreen.TAB_LEADERBOARD)
	var view: LeaderboardView = screen.leaderboard()
	assert_int(view.rows().size()).is_equal(4)
	assert_bool(view.stale_visible()).is_true()
	for row: Node in view.rows():
		for node: Node in row.find_children("*", "Label", true, false):
			var t: String = (node as Label).text
			for pii: String in ["sample", "Trường", "Lớp", "3A"]:
				assert_str(t).not_contains(pii)
		assert_int(row.find_children("*", "AvatarThumb", true, false).size()).is_equal(1)
	assert_str(" ".join(view.visible_texts())).contains(tr("ui.leaderboard.me"))


func test_a_closed_board_and_an_empty_list() -> void:
	# The first board is closed by the school (404), the next one opens.
	s.h.http.clear(CompetitionFixtures.ROUTE_BOARD).reply(CompetitionFixtures.ROUTE_BOARD, 404, AuthHarness.problem(404, "NOT_FOUND"))
	s.h.http.reply(CompetitionFixtures.ROUTE_BOARD, 200, CompetitionFixtures.board("top_n_percentile", s.self_id))
	await _open(ArenaScreen.TAB_LEADERBOARD)
	var view: LeaderboardView = screen.leaderboard()
	assert_str(view.notice_text()).is_equal(tr("ui.leaderboard.closed"))
	assert_int(view.board_chips().size()).is_equal(2)
	s.h.http.clear(CompetitionFixtures.ROUTE_BOARDS).reply(CompetitionFixtures.ROUTE_BOARDS, 200, {"data": []})
	await view.refresh()
	assert_bool(view.empty_visible()).is_true()


func test_history_cards_filter_and_detail() -> void:
	await _open(ArenaScreen.TAB_HISTORY)
	var view: MatchHistoryView = screen.history()
	assert_int(view.cards().size()).is_equal(2)
	assert_int(view.filter_chips().size()).is_equal(4)
	(view.filter_chips()[1] as Button).pressed.emit()
	for i: int in 6:
		await await_idle_frame()
	assert_str(s.h.http.last(CompetitionFixtures.ROUTE_MATCHES).query).contains("mode=battle_1v1")
	await view.open_detail(CompetitionFixtures.MATCH_ID)
	assert_bool(view.detail_visible()).is_true()
	var words: String = " ".join(view.detail_texts())
	assert_str(words).contains("CaHeoVui07")
	assert_str(words).contains("6 × 3")
	assert_object(view.safety()).is_not_null()
	assert_int(view.safety().player_count()).is_equal(1)


func test_history_detail_404_shows_not_found_and_nothing_else() -> void:
	await _open(ArenaScreen.TAB_HISTORY)
	var view: MatchHistoryView = screen.history()
	s.h.http.clear(CompetitionFixtures.ROUTE_MATCH).reply(CompetitionFixtures.ROUTE_MATCH, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await view.open_detail(CompetitionFixtures.MATCH_ID)
	assert_bool(view.detail_visible()).is_false()
	assert_str(view.notice_text()).is_equal(tr("ui.arena.error.NOT_FOUND"))


func test_tournament_list_detail_and_buttons() -> void:
	await _open(ArenaScreen.TAB_TOURNAMENTS)
	var view: TournamentView = screen.tournaments()
	assert_int(view.tournament_cards().size()).is_equal(1)
	assert_int(view.challenge_cards().size()).is_equal(1)
	await view.open_detail(CompetitionFixtures.TOURNAMENT_ID)
	assert_bool(view.detail_visible()).is_true()
	assert_object(view.action_button("Withdraw")).is_not_null()
	var check: Button = view.action_button("CheckIn")
	assert_object(check).is_not_null()
	assert_bool(check.disabled).is_true()
	assert_object(view.action_button("Register")).is_null()


func _bracket_detail(band: String) -> TournamentView:
	AppState.set_grade_band(band)
	var detail: Dictionary = CompetitionFixtures.openapi("getTournament.200", s.self_id)
	detail["status"] = "in_progress"
	s.h.http.clear(CompetitionFixtures.ROUTE_TOURNAMENT).reply(CompetitionFixtures.ROUTE_TOURNAMENT, 200, detail)
	await _open(ArenaScreen.TAB_TOURNAMENTS)
	var view: TournamentView = screen.tournaments()
	await view.open_detail(CompetitionFixtures.TOURNAMENT_ID)
	for i: int in 4:
		await await_idle_frame()
	return view


func test_bracket_tree_for_grade_6_9_and_path_for_grade_1_5() -> void:
	var view: TournamentView = await _bracket_detail("g6_7")
	assert_object(view.bracket_tree()).is_not_null()
	assert_object(view.bracket_path()).is_null()
	assert_int(view.bracket_tree().node_count()).is_equal(3)
	screen.free()
	screen = null
	view = await _bracket_detail("g1_3")
	assert_object(view.bracket_path()).is_not_null()
	assert_object(view.bracket_tree()).is_null()
	assert_int(view.bracket_path().step_count()).is_equal(2)
	var words: String = " ".join(view.detail_texts())
	assert_str(words).contains("CaHeoVui07")
	assert_str(words).not_contains("0193b0a0")
