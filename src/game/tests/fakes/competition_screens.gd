class_name CompetitionScreens
extends RefCounted
## TEST-ONLY (AGENTS R17): the F109 arena states built the same way for the grade-band matrix and the review captures,
## on the contract fixtures (CompetitionFixtures, ChatFixtures) — no network, no server state:
## `arena` (Thi đấu tab), `leaderboard_goal` / `leaderboard_top` / `leaderboard_ranked` (one per server visibility),
## `history`, `history_detail`, `tournaments`, `tournament_bracket` (tree for lớp 6–9, path for lớp 1–5), `duel`,
## `queue`, `queue_timeout`, `ready_check`, `battle_question`, `battle_recorded`, `battle_reveal`, `battle_team`,
## `battle_reconnect`, `result_win`, `result_loss`, `result_quiz_goal`, `result_quiz_rank`, `result_cancelled`; the server
## deltas of 2026-11-27: `duel_incoming` (a friend's duel prompt on the HUD), `result_encouragement` (loss streak line with
## the subject / topic), `tournament_error` (`409 TOURNAMENT_FULL` on sign-up).

const VIEWS: PackedStringArray = [
	"arena", "leaderboard_goal", "leaderboard_top", "leaderboard_ranked", "history", "history_detail", "tournaments",
	"tournament_bracket", "duel", "queue", "queue_timeout", "ready_check", "battle_question", "battle_recorded",
	"battle_reveal", "battle_team", "battle_reconnect", "result_win", "result_loss", "result_quiz_goal",
	"result_quiz_rank", "result_cancelled", "duel_incoming", "result_encouragement", "tournament_error",
]
## A topic of the listTopics fixture (Phép nhân) for the duel / encouragement views.
const TOPIC_MULTIPLY: String = "0193b0a0-0000-7000-8000-000000000611"
const FRAMES: int = 12


## Scripts the arena + chat routes for the signed-in student of `services` (match chat open, party for the duel).
static func prepare(services: ServiceLocator, http: FakeHttp, team: bool = false) -> String:
	var self_id: String = CompetitionPlayer.self_id(AppState)
	CompetitionFixtures.script(http, self_id)
	ChatScreens.prepare(services, http)
	var body: Dictionary = ChatFixtures.channels(self_id)
	for c: Variant in body["data"] as Array:
		(c as Dictionary)["status"] = "open"
	http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200, body)
	# The sample messages in the channel the dock shows first (team in a team battle, else the match).
	var history: Dictionary = ChatFixtures.history(self_id)
	for m: Variant in history["data"] as Array:
		(m as Dictionary)["channel_id"] = ChatFixtures.TEAM_CHANNEL if team else ChatFixtures.MATCH_CHANNEL
	http.clear(ChatFixtures.ROUTE_MESSAGES).reply(ChatFixtures.ROUTE_MESSAGES, 200, history)
	PartyFixtures.script(http, self_id, true)
	services.matchmaking.clear()
	services.battle.clear()
	services.leaderboards.clear()
	services.tournaments.clear()
	services.match_history.clear()
	return self_id


## Coroutine: builds `view` in `parent` (a full-screen Control) and returns the screen root.
static func build(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> Control:
	var self_id: String = prepare(services, http, view == "battle_team")
	if view.begins_with("battle_") or view.begins_with("result_"):
		return await _battle(view, parent, services, self_id)
	if view in ["queue", "queue_timeout", "ready_check", "duel_incoming"]:
		return await _hud(view, parent, services, self_id)
	var tab: String = ArenaScreen.TAB_PLAY
	if view.begins_with("leaderboard_"):
		tab = ArenaScreen.TAB_LEADERBOARD
		var vis: String = {"leaderboard_goal": "grade_goal", "leaderboard_top": "top_n_percentile",
			"leaderboard_ranked": "ranked"}[view]
		http.clear(CompetitionFixtures.ROUTE_BOARD).reply(CompetitionFixtures.ROUTE_BOARD, 200, CompetitionFixtures.board(vis, self_id))
		var lists: Array = CompetitionFixtures.openapi("listLeaderboards.200")["data"] as Array
		# Lớp 1–3 see only the grade goal (server filter); lớp 4–9 boards open on "Đấu trường trường".
		var shown: Array = [lists[0]] if vis == "grade_goal" else [lists[1], lists[2]]
		http.clear(CompetitionFixtures.ROUTE_BOARDS).reply(CompetitionFixtures.ROUTE_BOARDS, 200, {"data": shown})
	elif view.begins_with("history"):
		tab = ArenaScreen.TAB_HISTORY
	elif view.begins_with("tournament"):
		tab = ArenaScreen.TAB_TOURNAMENTS
		if view == "tournament_error":
			# Not signed up yet and the last places just went (the server says TOURNAMENT_FULL).
			var open: Dictionary = CompetitionFixtures.openapi("getTournament.200", self_id)
			open.erase("my_entry")
			open["entry_count"] = open["max_entries"]
			http.clear(CompetitionFixtures.ROUTE_TOURNAMENT).reply(CompetitionFixtures.ROUTE_TOURNAMENT, 200, open)
		if view == "tournament_bracket":
			var detail: Dictionary = CompetitionFixtures.openapi("getTournament.200", self_id)
			detail["status"] = "in_progress"
			http.clear(CompetitionFixtures.ROUTE_TOURNAMENT).reply(CompetitionFixtures.ROUTE_TOURNAMENT, 200, detail)
	var screen: ArenaScreen = (load("res://scenes/competition/ArenaScreen.tscn") as PackedScene).instantiate() as ArenaScreen
	screen.services = services
	parent.add_child(screen)
	await screen.open({"tab": tab})
	await _frames(parent, FRAMES)
	match view:
		"history_detail":
			await screen.history().open_detail(CompetitionFixtures.MATCH_ID)
		"tournament_bracket":
			await screen.tournaments().open_detail(CompetitionFixtures.TOURNAMENT_ID)
		"tournament_error":
			await screen.tournaments().open_detail(CompetitionFixtures.TOURNAMENT_ID)
			await _frames(parent, 6)
			http.clear(CompetitionFixtures.ROUTE_REGISTER).reply(CompetitionFixtures.ROUTE_REGISTER, 409,
					AuthHarness.problem(409, "TOURNAMENT_FULL"))
			await services.tournaments.register(CompetitionFixtures.TOURNAMENT_ID)
		"duel":
			screen.duel_button().pressed.emit()
			await _frames(parent, FRAMES)
			if not screen.duel_dialog().friend_chips().is_empty():
				(screen.duel_dialog().friend_chips()[0] as Button).pressed.emit()
	await _frames(parent, 6)
	return screen


static func _hud(view: String, parent: Control, services: ServiceLocator, self_id: String) -> Control:
	var hud: ArenaHud = ArenaHud.new()
	parent.add_child(hud)
	hud.bind(services.matchmaking, services.tournaments)
	var mm: MatchmakingService = services.matchmaking
	if view == "duel_incoming":
		await mm.load_subjects()
		await mm.load_topics(CompetitionFixtures.MATH)
		var duel: Dictionary = CompetitionFixtures.ws("competition.duel.received", "friend_duel", self_id)
		duel["expires_at"] = mm.now_ms() + 45000
		duel["topic_id"] = TOPIC_MULTIPLY
		var card: Dictionary = duel["challenger"] as Dictionary
		card["level"] = 7
		card["avatar_config"] = PartyMemberCard.DEFAULT_AVATAR.duplicate(true)
		mm.handle_competition_message("competition.duel.received", duel)
		hud.advance(0.1)
		await _frames(parent, FRAMES)
		return hud
	mm.last_search = {"mode": "battle_1v1", "subject_id": CompetitionFixtures.MATH, "topic_id": ""}
	mm.handle_message("matchmaking.ticket.updated", CompetitionFixtures.ws("matchmaking.ticket.updated", "queued", self_id))
	if view == "queue":
		mm.handle_message("matchmaking.ticket.updated", CompetitionFixtures.ws("matchmaking.ticket.updated", "widened", self_id))
	elif view == "queue_timeout":
		mm.handle_message("matchmaking.ticket.updated", CompetitionFixtures.ws("matchmaking.ticket.updated", "expired", self_id))
	else:
		var p: Dictionary = CompetitionFixtures.ws("matchmaking.proposal.created", "battle_1v1", self_id)
		p["deadline_at"] = mm.now_ms() + 12000
		mm.handle_message("matchmaking.proposal.created", p)
	hud.advance(0.1)
	await _frames(parent, FRAMES)
	return hud


static func _battle(view: String, parent: Control, services: ServiceLocator, self_id: String) -> Control:
	var team: bool = view == "battle_team"
	var quiz: bool = view.begins_with("result_quiz")
	var match_id: String = CompetitionFixtures.TEAM_MATCH_ID if team else (CompetitionFixtures.QUIZ_MATCH_ID if quiz else CompetitionFixtures.MATCH_ID)
	var mode: String = "team_battle" if team else ("timed_quiz" if quiz else "battle_1v1")
	var arena: BattleArena = (load("res://scenes/competition/BattleArena.tscn") as PackedScene).instantiate() as BattleArena
	arena.services = services
	parent.add_child(arena)
	arena.open({"match_id": match_id, "mode": mode})
	await _frames(parent, 6)
	var bc: BattleClient = services.battle
	var snap: Dictionary = CompetitionFixtures.ws("battle.state.snapshot", "team_battle" if team else "question_1v1", self_id)
	snap["match_id"] = match_id
	snap["mode"] = mode
	snap["deadline_at"] = bc.now_ms() + 12000
	# Team battle: the fixture's primary players (the student for team 1, CaHeoVui07 for team 2) are kept.
	# The server's chat ids name the channels of the chat fixtures (their sample messages show in the dock).
	snap["chat_channel_ids"] = {"match": ChatFixtures.MATCH_CHANNEL, "team": ChatFixtures.TEAM_CHANNEL} if team \
			else {"match": ChatFixtures.MATCH_CHANNEL}
	bc.handle_message("battle.state.snapshot", snap)
	await _frames(parent, 6)
	match view:
		"battle_recorded":
			arena.choose(1)
			bc.input_locked = true
			bc.answer_locked.emit()
		"battle_reveal":
			arena.choose(1)
			bc.handle_message("battle.answer.evaluated", CompetitionFixtures.ws("battle.answer.evaluated", "correct_bonus", self_id))
		"battle_reconnect":
			var st: Dictionary = CompetitionFixtures.ws("battle.participant.status_changed", "opponent_disconnected", self_id)
			st["grace_until"] = bc.now_ms() + 20000
			bc.handle_message("battle.participant.status_changed", st)
		"result_win", "result_loss", "result_cancelled", "result_quiz_goal", "result_quiz_rank", "result_encouragement":
			services.matchmaking.last_search = {"mode": mode, "subject_id": CompetitionFixtures.MATH, "topic_id": ""}
			var name: String = {"result_win": "win", "result_loss": "loss", "result_cancelled": "cancelled",
				"result_quiz_goal": "timed_quiz_grade_goal", "result_quiz_rank": "timed_quiz_rank",
				"result_encouragement": "loss_streak_encouragement"}[view]
			var ended: Dictionary = CompetitionFixtures.ws("battle.match.ended", name, self_id)
			ended["match_id"] = match_id
			if view == "result_encouragement":
				# This student lost the match (the third in a row): the server sent them the encouragement.
				for r: Variant in ended["results"] as Array:
					var row: Dictionary = r as Dictionary
					var mine: bool = String(row["user_id"]) == self_id
					row["outcome"] = "loss" if mine else "win"
					row["score"] = 350 if mine else 420
					row["correct_count"] = 2 if mine else 3
					row["competition_points"] = 6 if mine else 12
				(ended["encouragement"] as Dictionary)["topic_id"] = TOPIC_MULTIPLY
			bc.handle_message("battle.match.ended", ended)
	arena.advance(0.1)
	await _frames(parent, FRAMES)
	return arena


static func _frames(node: Node, n: int) -> void:
	var tree: SceneTree = node.get_tree() if node.is_inside_tree() else Engine.get_main_loop() as SceneTree
	for i: int in n:
		await tree.process_frame
