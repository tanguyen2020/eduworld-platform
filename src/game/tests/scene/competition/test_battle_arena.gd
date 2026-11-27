extends GdUnitTestSuite
## T017, T019, T020, T022, T023, T026, T027 (F109-AC-05, AC-06, AC-08…AC-16; TC-CMP-003…007, TC-PERF-018; F119-AC-01…03,
## AC-14): the battle screen on the contract fixtures —
## - the snapshot draws the header (both nicknames, server scores, "Câu 1/3") and the question with the F063 renderer;
## - a tap sends one `battle.answer.submit`, every tile locks and "Đã ghi nhận" shows; ✓ / ✗ in words and the points only
##   after `battle.answer.evaluated`; "Thưởng tốc độ" only when `time_bonus > 0`; no speed hint while
##   `speed_bonus_enabled` is false;
## - lớp 1–3: no number of seconds, answer tiles ≥ 64 dp; lớp 4–9: the seconds;
## - team battle: team scores, "Lượt chính của em" from `primary_user_ids`;
## - "Đối thủ đang kết nối lại…" until the grace, the pause overlay; the 3D world is suspended while open;
## - the match chat is always there (no off switch), shrinks to its 2-line strip while a question is on screen;
## - the result: encouraging outcome, points, tier in words (never a rating number), cancelled = "Trận đã hủy", the
##   Báo cáo / Chặn row for the opponent (never the robot).

var s: CompetitionSceneHarness = null
var arena: BattleArena = null
var shown: int = 0
var hidden: int = 0
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = CompetitionSceneHarness.new()
	await s.start("g4_5")
	shown = 0
	hidden = 0
	EventBus.question_shown.connect(_on_shown)
	EventBus.question_hidden.connect(_on_hidden)


func after_test() -> void:
	EventBus.question_shown.disconnect(_on_shown)
	EventBus.question_hidden.disconnect(_on_hidden)
	if arena != null:
		if arena.visible:
			arena.close()
		arena.free()
		arena = null
	s.dispose()


func _on_shown() -> void:
	shown += 1


func _on_hidden() -> void:
	hidden += 1


func _open(match_id: String = CompetitionFixtures.MATCH_ID, mode: String = "battle_1v1") -> void:
	arena = (load("res://scenes/competition/BattleArena.tscn") as PackedScene).instantiate() as BattleArena
	arena.services = s.services
	arena.theme = AppState.theme_service.current_theme
	add_child(arena)
	arena.size = Vector2(1280, 720)
	arena.open({"match_id": match_id, "mode": mode})
	for i: int in 6:
		await await_idle_frame()


func _frames(n: int = 4) -> void:
	for i: int in n:
		await await_idle_frame()


func _question_snapshot() -> void:
	var snap: Dictionary = s.ws("battle.state.snapshot", "question_1v1")
	snap["deadline_at"] = s.services.battle.now_ms() + 15000
	s.push("battle.state.snapshot", snap)
	await _frames()


func _tiles() -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in arena.question_area().find_children("*", "Button", true, false):
		if (node as Button).theme_type_variation.begins_with("AnswerTile"):
			out.append(node as Button)
	return out


func test_snapshot_draws_header_and_question_and_suspends_the_world() -> void:
	await _open()
	assert_bool(QualityManager.suspend_reasons.has(BattleArena.SUSPEND_REASON)).is_true()
	await _question_snapshot()
	assert_str(arena.header().progress_text()).is_equal(tr("ui.arena.battle.progress").format({"n": 1, "total": 3}))
	var sides: String = " ".join(arena.header().side_texts())
	assert_str(sides).contains("CaHeoVui07")
	assert_int(_tiles().size()).is_equal(4)
	assert_bool(arena.question_on()).is_true()
	assert_int(shown).is_equal(1)
	assert_str(arena.header().seconds_text()).is_not_empty()


func test_one_tap_sends_one_answer_locks_and_shows_recorded() -> void:
	await _open()
	await _question_snapshot()
	arena.choose(1)
	await _frames()
	arena.choose(2)
	await _frames()
	var sent: Array[Dictionary] = s.rt.peer().sent_of(BattleClient.CMD_SUBMIT)
	assert_int(sent.size()).is_equal(1)
	assert_dict((sent[0]["payload"] as Dictionary)["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_str(arena.status_text()).is_equal(tr("ui.arena.battle.recorded"))
	assert_bool(arena.question.is_input_locked()).is_true()
	# No verdict before the server's evaluation.
	assert_bool(arena.reveal().visible).is_false()
	for b: Button in _tiles():
		assert_int(QuestionRendererBase.tile_verdict(b)).is_not_equal(AnswerBadge.Verdict.CORRECT)


func test_verdict_in_words_and_speed_bonus_only_when_sent() -> void:
	await _open()
	await _question_snapshot()
	s.push_example("battle.answer.evaluated", "no_bonus")
	await _frames()
	assert_str(arena.reveal().verdict_text()).is_equal(tr("ui.arena.reveal.correct"))
	assert_bool(arena.reveal().bonus_visible()).is_false()
	s.push_example("battle.answer.evaluated", "correct_bonus")
	await _frames()
	assert_bool(arena.reveal().bonus_visible()).is_true()
	s.push_example("battle.answer.evaluated", "incorrect")
	await _frames()
	assert_str(arena.reveal().verdict_text()).is_equal(tr("ui.arena.reveal.try"))
	s.push_example("battle.question.revealed", "battle_1v1")
	await _frames()
	assert_bool(arena.question_on()).is_false()
	assert_int(hidden).is_greater_equal(1)


func test_no_speed_hint_while_the_bonus_is_off() -> void:
	await _open()
	await _question_snapshot()
	# Every visible word of the match screen outside the chat (the chat's own "tin nhắn nhanh" is quick messages).
	var words: String = " ".join(arena.find_children("*", "Label", true, false).filter(
			func(n: Node) -> bool: return (n as Label).is_visible_in_tree() and not arena.chat_dock().is_ancestor_of(n)).map(
			func(n: Node) -> String: return tr((n as Label).text))).to_lower()
	for hint: String in ["nhanh", "tốc độ", "fast", "speed"]:
		assert_str(words).not_contains(hint)


func test_grade_1_3_has_no_seconds_and_big_tiles() -> void:
	AppState.set_grade_band("g1_3")
	await _open()
	await _question_snapshot()
	arena.advance(0.1)
	assert_bool(arena.header().seconds_visible()).is_false()
	for b: Button in _tiles():
		assert_float(b.size.y).is_greater_equal(64.0)


func test_team_battle_scores_and_primary_marker() -> void:
	await _open(CompetitionFixtures.TEAM_MATCH_ID, "team_battle")
	var snap: Dictionary = s.ws("battle.state.snapshot", "team_battle")
	snap["primary_user_ids"] = [s.self_id]
	s.push("battle.state.snapshot", snap)
	await _frames()
	var sides: String = " ".join(arena.header().side_texts())
	assert_str(sides).contains(tr("ui.arena.battle.my_team"))
	assert_str(sides).contains(tr("ui.arena.battle.score").format({"score": "390"}))
	assert_str(arena.header().role_text()).is_equal(tr("ui.arena.battle.role_primary"))
	# A teammate away is said in words (header + banner).
	assert_str(sides).contains(tr("ui.arena.battle.teammate_reconnecting"))
	arena.advance(0.1)
	assert_str(arena.banner_text()).contains("SaoBang09")


func test_opponent_reconnecting_banner_and_pause_overlay() -> void:
	await _open()
	await _question_snapshot()
	var st: Dictionary = s.ws("battle.participant.status_changed", "opponent_disconnected")
	st["grace_until"] = s.services.battle.now_ms() + 20000
	s.push("battle.participant.status_changed", st)
	arena.advance(0.1)
	assert_str(arena.banner_text()).contains("CaHeoVui07")
	s.push_example("battle.match.paused", "server_failover")
	await _frames()
	assert_bool(arena.pause_visible()).is_true()
	s.push_example("battle.match.resumed", "replay")
	await _frames()
	assert_bool(arena.pause_visible()).is_false()
	assert_bool(arena.question_on()).is_true()


func test_match_chat_is_always_there_and_shrinks_during_a_question() -> void:
	await _open()
	await _question_snapshot()
	await _frames(8)
	var dock: ChatDock = arena.chat_dock()
	assert_bool(dock.is_visible_in_tree()).is_true()
	assert_str(String(dock.context_kind)).is_equal("match")
	assert_int(dock.channel_ids.size()).is_greater_equal(1)
	assert_int(dock.mode).is_equal(ChatDock.Mode.MINI)
	# The 2-line strip sits beside the header (never over the answers), the question takes the whole width below.
	assert_bool(arena.strip_slot().is_ancestor_of(dock)).is_true()
	assert_bool(dock.get_global_rect().intersects(arena.question_area().get_global_rect())).is_false()
	for name: String in dock.control_names():
		assert_str(name.to_lower()).not_contains("disable")
		assert_str(name.to_lower()).not_contains("off")
	s.push_example("battle.question.revealed", "battle_1v1")
	await _frames()
	assert_int(dock.mode).is_not_equal(ChatDock.Mode.MINI)
	# Between questions the chat is the right column again.
	assert_bool(arena.chat_column().is_ancestor_of(dock)).is_true()
	assert_bool(arena.chat_column().visible).is_true()


func test_result_win_points_tier_in_words_and_safety_row() -> void:
	await _open()
	await _question_snapshot()
	s.services.matchmaking.last_search = {"mode": "battle_1v1", "subject_id": CompetitionFixtures.MATH, "topic_id": ""}
	s.push_example("battle.match.ended", "win")
	await _frames(12)
	var r: MatchResultView = arena.result_view()
	assert_bool(r.visible).is_true()
	assert_str(r.outcome_key()).is_equal("ui.arena.result.win")
	var text: String = " ".join(r.texts())
	assert_str(text).contains(tr("ui.arena.result.points").format({"points": "12"}))
	assert_str(r.tier_text()).contains("Bạc II")
	assert_object(RegEx.create_from_string("\\b1[0-9]{3}\\b").search(text + r.tier_text())).is_null()
	assert_int(r.safety().player_count()).is_equal(1)
	assert_object(r.safety().report_button(CompetitionFixtures.OPPONENT)).is_not_null()
	assert_bool(r.again_button().visible).is_true()
	assert_bool(arena.question_on()).is_false()


func test_result_loss_is_encouraging_and_cancelled_says_not_counted() -> void:
	await _open()
	await _question_snapshot()
	s.push_example("battle.match.ended", "loss")
	await _frames(8)
	assert_str(arena.result_view().outcome_key()).is_equal("ui.arena.result.not_won")
	var words: String = " ".join(arena.result_view().texts()).to_lower()
	for harsh: String in ["thua", "kém", "lose", "loser"]:
		assert_str(words).not_contains(harsh)
	arena.close()
	await _open()
	await _question_snapshot()
	s.push_example("battle.match.ended", "cancelled")
	await _frames(8)
	assert_str(arena.result_view().outcome_key()).is_equal("ui.arena.result.void")
	assert_bool(arena.result_view().again_button().visible).is_false()


func test_report_from_the_result_uses_the_f119_sheet_with_the_match_context() -> void:
	await _open()
	await _question_snapshot()
	s.push_example("battle.match.ended", "win")
	await _frames(8)
	var r: MatchResultView = arena.result_view()
	r.safety().report_button(CompetitionFixtures.OPPONENT).pressed.emit()
	await _frames()
	assert_str(String(r.dialogs().view)).is_equal("report")
	assert_str(String(r.dialogs().context["match_id"])).is_equal(CompetitionFixtures.MATCH_ID)
