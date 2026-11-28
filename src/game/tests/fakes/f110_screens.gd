class_name F110Screens
extends RefCounted
## TEST-ONLY (AGENTS R17): the F110 screens built the same way for the grade-band matrix, the compact polish probes and
## the review captures, on the contract fixtures (F110Fixtures, PartyFixtures, ChatFixtures) — no network, no server
## state. Views: `friends` (list + a friend's profile), `friends_empty`, `friends_requests`, `friends_find` (nearby players
## + a friend code), `friends_blocked`, `friends_block_confirm`, `boss_info`, `boss_lobby` (party ready list),
## `boss_question` (party, a phase with a question and the contribution chips), `boss_correct`, `boss_wrong`,
## `boss_retry`, `boss_reconnect` (a member reconnecting), `boss_solo`, `boss_leave`, `boss_result_defeated`,
## `boss_result_time_up`, `coop_step` (own role card, the other role, the board), `coop_assign` (leader's role choice).

const VIEWS: PackedStringArray = ["friends", "friends_empty", "friends_requests", "friends_find", "friends_blocked",
	"friends_block_confirm", "boss_info", "boss_lobby", "boss_question", "boss_correct", "boss_wrong", "boss_retry",
	"boss_reconnect", "boss_solo", "boss_leave", "boss_result_defeated", "boss_result_time_up", "coop_step", "coop_assign"]
const FRAMES: int = 10
## Review captures of tools/capture_screens.gd (`f110_<view>`; `hub_coop` = the Quest Tracker with the party strip,
## `game_menu` = the "Thêm" menu with its Bạn bè tile).
const CAPTURES: PackedStringArray = ["f110_friends", "f110_friends_empty", "f110_friends_requests", "f110_friends_find",
	"f110_friends_blocked", "f110_friends_block_confirm", "f110_boss_info", "f110_boss_lobby", "f110_boss_question",
	"f110_boss_correct", "f110_boss_wrong", "f110_boss_retry", "f110_boss_reconnect", "f110_boss_solo", "f110_boss_leave",
	"f110_boss_result_defeated", "f110_boss_result_time_up", "f110_coop_step", "f110_coop_assign", "f110_hub_coop",
	"f110_game_menu"]


## Coroutine (review captures): `view` over `world` on a layer above it; `hub_coop` (a tracked party quest with its
## strip on the HUD) or `game_menu` (the "Thêm" menu) on the world itself. Returns the world.
static func capture(world: Node, view: String, http: FakeHttp) -> Node:
	var services: ServiceLocator = AppState.get_services()
	if view == "hub_coop":
		services.quest.quests.append(QuestModel.from_dict(F110Fixtures.coop_quest()))
		services.quest.track(F110Fixtures.QUEST_ID)
		services.coop_quest.handle_message("quest.progress.updated", F110Fixtures.progress(0.75, false))
	elif view == "game_menu":
		EventBus.ui_requested.emit(&"game_menu", {})
	else:
		var layer: CanvasLayer = CanvasLayer.new()
		layer.name = "F110Layer"
		layer.layer = 20
		world.add_child(layer)
		var screen: Control = Control.new()
		screen.set_anchors_preset(Control.PRESET_FULL_RECT)
		screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
		layer.add_child(screen)
		await build(view, screen, services, http)
	await _frames(world, 20)
	return world


static func _self(_services: ServiceLocator) -> String:
	return CompetitionPlayer.self_id(AppState)


## Coroutine: builds `view` in `parent` (a full-screen Control) and returns the screen root.
static func build(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> Control:
	var self_id: String = _self(services)
	services.party._refresh_self()
	F110Fixtures.script(http, self_id, services.boss.now_ms())
	ChatScreens.prepare(services, http)
	if view.begins_with("friends"):
		return await _friends(view, parent, services, http)
	if view == "boss_info":
		var card: BossInfoCard = (load("res://scenes/challenge/boss/BossInfoCard.tscn") as PackedScene).instantiate() as BossInfoCard
		card.services = services
		parent.add_child(card)
		await card.open({"area_id": "math_kingdom"})
		await _frames(card)
		return card
	if view.begins_with("boss_"):
		return await _boss(view, parent, services, http, self_id)
	return await _coop(view, parent, services, http, self_id)


static func _frames(node: Node, n: int = FRAMES) -> void:
	for i: int in n:
		await node.get_tree().process_frame


# ---------- friends ----------

static func _friends(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> Control:
	var self_id: String = _self(services)
	if view == "friends_empty":
		http.clear(F110Fixtures.ROUTE_FRIENDS).reply(F110Fixtures.ROUTE_FRIENDS, 200, {"data": [], "page": {"has_more": false, "limit": 100}})
	if view == "friends_blocked":
		http.clear(F110Fixtures.ROUTE_BLOCKS).reply(F110Fixtures.ROUTE_BLOCKS, 200, {"data": [
			F110Fixtures.relation(F110Fixtures.STRANGER, "BaoGio88")], "page": {"has_more": false, "limit": 100}})
		http.clear(F110Fixtures.ROUTE_MUTES).reply(F110Fixtures.ROUTE_MUTES, 200, {"data": [
			F110Fixtures.relation(F110Fixtures.FRIEND_OFFLINE, "ThoTrang03")], "page": {"has_more": false, "limit": 100}})
	# "Ở gần đây": two players of the hub instance (nickname + look only).
	var room: HubRoomState = services.hub.room
	room.players.clear()
	var look: Dictionary = ((F110Fixtures.friends()["data"] as Array)[1] as Dictionary)["avatar_config"] as Dictionary
	for pair: Array in [[11, F110Fixtures.STRANGER, "BuomXanh09"], [12, "0193b0a0-0000-7000-8000-0000000000b8", "KhiCon21"]]:
		room.players[pair[0]] = PublicPlayerModel.from_dict({"user_id": pair[1], "nickname": pair[2], "avatar_config": look,
			"level": 5})
	var screen: FriendsScreen = (load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate() as FriendsScreen
	screen.services = services
	parent.add_child(screen)
	var tab: String = {"friends_requests": FriendsScreen.TAB_REQUESTS, "friends_find": FriendsScreen.TAB_FIND,
		"friends_blocked": FriendsScreen.TAB_BLOCKED}.get(view, FriendsScreen.TAB_FRIENDS)
	screen.open({"tab": tab})
	await _frames(screen, 12)
	match view:
		"friends", "friends_block_confirm":
			screen.select_friend(F110Fixtures.FRIEND_ONLINE)
			await _frames(screen, 4)
			if view == "friends_block_confirm":
				screen.profile().button(FriendProfilePanel.ACT_BLOCK).pressed.emit()
		"friends_find":
			await services.social.create_code()
			screen.find_view().code_widget.type_text("ab12cd")
	await _frames(screen, 6)
	room.players.clear()
	return screen


# ---------- boss ----------

static func _boss(view: String, parent: Control, services: ServiceLocator, http: FakeHttp, self_id: String) -> Control:
	var party: bool = view != "boss_solo"
	var now: int = services.boss.now_ms()
	var status: String = "lobby" if view == "boss_lobby" else "phase_active"
	var e: Dictionary = F110Fixtures.encounter(self_id, "party" if party else "solo", status, 1, now)
	if party:
		await _with_party(services, http, self_id)
	http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200, e)
	var arena: BossArena = (load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate() as BossArena
	arena.services = services
	parent.add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": "party" if party else "solo",
		"title": "Rồng Phân Số" if TranslationServer.get_locale().begins_with("vi") else "Fraction Dragon"})
	var boss: BossClient = services.boss
	if view != "boss_lobby":
		var members: Array = [F110Fixtures.member(self_id, "MeoXanh42", true, true, false),
			F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK, true, view != "boss_reconnect", true),
			F110Fixtures.member(F110Fixtures.THIRD, F110Fixtures.THIRD_NICK, true, true, false)] if party \
				else [F110Fixtures.member(self_id, "MeoXanh42", true, true, false)]
		boss.handle_message("boss.phase.started", F110Fixtures.phase_started(0, view == "boss_retry", now + 95_000))
		boss.handle_message("boss.encounter.updated", F110Fixtures.updated(2, 52.0, members))
		boss.handle_message("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1"], now + 95_000))
	match view:
		"boss_correct", "boss_wrong":
			boss.handle_message("boss.answer.evaluated", F110Fixtures.evaluated("bq_p1_1", view == "boss_correct"))
			boss.handle_message("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_2"], now + 95_000))
		"boss_leave":
			arena.ask_leave()
		"boss_result_defeated":
			boss.handle_message("boss.encounter.ended", F110Fixtures.ended("defeated"))
		"boss_result_time_up":
			boss.handle_message("boss.encounter.ended", F110Fixtures.ended("time_up", 1, false))
	await _frames(arena)
	return arena


static func _with_party(services: ServiceLocator, http: FakeHttp, self_id: String) -> void:
	PartyFixtures.script(http, self_id, false)
	await services.party.load_my_party()


# ---------- co-op quest ----------

static func _coop(view: String, parent: Control, services: ServiceLocator, http: FakeHttp, self_id: String) -> Control:
	await _with_party(services, http, self_id)
	services.quest.quests.clear()
	services.quest.quests.append(QuestModel.from_dict(F110Fixtures.coop_quest()))
	var coop: CoopQuestService = services.coop_quest
	coop.handle_message("quest.roles.assigned", F110Fixtures.roles(self_id, PackedStringArray([F110Fixtures.ROLE_MEASURE]),
			view == "coop_assign"))
	http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false),
			F110Fixtures.clue(F110Fixtures.ROLE_MAP, true, F110Fixtures.MATE_NICK)]))
	var panel: InterdependentStepPanel = (load("res://scenes/quest/InterdependentStepPanel.tscn") as PackedScene).instantiate() \
			as InterdependentStepPanel
	panel.services = services
	parent.add_child(panel)
	await panel.open({"quest_id": F110Fixtures.QUEST_ID, "step_id": F110Fixtures.STEP_ID})
	await _frames(panel)
	if view == "coop_assign" and panel.assign_button().visible:
		panel.assign_button().pressed.emit()
		await _frames(panel, 4)
	return panel
