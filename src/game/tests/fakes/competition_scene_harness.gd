class_name CompetitionSceneHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a signed-in student (AuthHarness on FakeHttp) with the session services in AppState, the
## arena routes scripted from CompetitionFixtures and the chat routes from ChatFixtures, for the F109 scene tests, the
## grade-band matrix and the review captures. The matchmaking and battle services run on a RealtimeHarness that is
## online (FakeWebSocketPeer), so commands are really sent and can be acked / refused with `ack_last`.

var h: AuthHarness = null
var rt: RealtimeHarness = null
var services: ServiceLocator = null
var self_id: String = ""
var _previous: ServiceLocator = null
var _band: String = ""
var _locale: String = ""


## Coroutine: signs in, builds the services, puts the arena services on the online realtime harness.
func start(band: String = "g4_5") -> void:
	_previous = AppState.services
	_band = AppState.grade_band
	_locale = String(Settings.locale)
	Settings.set_locale("vi-VN", false)
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	AppState.player = AuthHarness.bootstrap()
	AppState.start_ui(Engine.get_main_loop().root as Window)
	AppState.set_grade_band(band)
	self_id = CompetitionPlayer.self_id(AppState)
	rt = RealtimeHarness.create()
	await rt.connect_online()
	_unregister()
	services.matchmaking = MatchmakingService.new(h.api, rt.client, AppState)
	services.battle = BattleClient.new(rt.client, AppState)
	CompetitionFixtures.script(h.http, self_id)
	ChatScreens.prepare(services, h.http)
	open_match_chat()


## The match channel of the chat fixtures is open (a running match; the fixture keeps it closed for F119's retention
## test).
func open_match_chat() -> void:
	var body: Dictionary = ChatFixtures.channels(self_id)
	for c: Variant in body["data"] as Array:
		(c as Dictionary)["status"] = "open"
	h.http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200, body)


## Delivers a realtime message (AsyncAPI shape) to the arena services.
func push(type: String, payload: Dictionary) -> void:
	if type.begins_with("matchmaking."):
		services.matchmaking.handle_message(type, payload)
	elif type.begins_with("battle."):
		services.battle.handle_message(type, payload)
	elif type.begins_with("tournament."):
		services.tournaments.handle_message(type, payload)
	elif type.begins_with("competition."):
		services.matchmaking.handle_competition_message(type, payload)


## The AsyncAPI example `<type>.<name>` for the signed-in student.
func ws(type: String, name: String) -> Dictionary:
	return CompetitionFixtures.ws(type, name, self_id)


func push_example(type: String, name: String) -> Dictionary:
	var p: Dictionary = ws(type, name)
	push(type, p)
	return p


func dispose() -> void:
	if services != null:
		services.chat.clear()
		_unregister()
		if services.matchmaking != null and rt != null:
			rt.client.unregister_handler(MatchmakingService.PREFIX, services.matchmaking.handle_message)
			rt.client.unregister_handler(MatchmakingService.PREFIX_COMPETITION, services.matchmaking.handle_competition_message)
		if services.battle != null and rt != null:
			rt.client.unregister_handler(BattleClient.PREFIX, services.battle.handle_message)
	if rt != null:
		rt.dispose()
	if h != null:
		h.dispose()
	AppState.services = _previous
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)
	for blocker: StringName in [&"battle", &"challenge", &"exam"]:
		AppState.end_blocking(blocker)
	QualityManager.resume_world(&"battle")


## The services built on the autoload client registered their handlers there; they are replaced here.
func _unregister() -> void:
	var auto: GameRealtimeClient = services.realtime
	if auto == null:
		return
	if services.matchmaking != null:
		auto.unregister_handler(MatchmakingService.PREFIX, services.matchmaking.handle_message)
		auto.unregister_handler(MatchmakingService.PREFIX_COMPETITION, services.matchmaking.handle_competition_message)
	if services.battle != null:
		auto.unregister_handler(BattleClient.PREFIX, services.battle.handle_message)
	auto.unregister_handler(LeaderboardService.PREFIX, services.leaderboards.handle_message)
	auto.unregister_handler(TournamentService.PREFIX, services.tournaments.handle_message)
