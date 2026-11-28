class_name F110SceneHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a signed-in student (AuthHarness on FakeHttp) with the session services in AppState, the F110
## routes scripted from F110Fixtures, the party routes from PartyFixtures and the chat routes from ChatScreens, for the
## F110 unit / scene tests, the grade-band matrix and the review captures. BossClient, CoopQuestService, SocialService
## (its notification handler) and PresenceService run on a RealtimeHarness that is online (FakeWebSocketPeer): commands
## are really sent and can be acked / refused with `ack_last`. `dispose()` restores AppState.

var h: AuthHarness = null
var rt: RealtimeHarness = null
var services: ServiceLocator = null
var self_id: String = ""
var _previous: ServiceLocator = null
var _band: String = ""
var _locale: String = ""


func start(band: String = "g1_3", online: bool = true) -> void:
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
	_unregister()
	rt = RealtimeHarness.create()
	if online:
		await rt.connect_online()
	services.boss = BossClient.new(h.api, AppState, h.clock, rt.client, services.telemetry)
	services.coop_quest = CoopQuestService.new(h.api, rt.client, services.quest, AppState, services.telemetry)
	services.coop_quest.party_store = services.party.store
	services.presence = PresenceService.new(rt.client, h.clock)
	services.presence.friend_presence_received.connect(func(user_id: String, status: String, activity: String,
			area_id: String) -> void: services.social.store.apply_presence(user_id, status, activity, area_id))
	services.party._refresh_self()
	PartyFixtures.script(h.http, self_id, true)
	ChatScreens.prepare(services, h.http)
	F110Fixtures.script(h.http, self_id, rt.client.server_now_ms())


## The party of the fixtures with the signed-in student as leader, loaded in PartyService.
func with_party() -> void:
	PartyFixtures.script(h.http, self_id, false)
	await services.party.load_my_party()


## Delivers a realtime message (AsyncAPI shape) to the F110 services, as the dispatcher would.
func push(type: String, payload: Dictionary) -> void:
	if type.begins_with("boss."):
		services.boss.handle_message(type, payload)
	elif type.begins_with("quest."):
		services.coop_quest.handle_message(type, payload)
	elif type.begins_with("presence."):
		services.presence.handle_message(type, payload)
	elif type.begins_with("party."):
		services.party.handle_message(type, payload)
	elif type.begins_with("notification."):
		services.social.handle_notification(type, payload)


## Answers the last realtime command of `type` (ok, or the failure `code`).
func ack_last(type: String, ok: bool = true, code: String = "") -> void:
	rt.ack_last(type, ok, code)


func sent(type: String) -> Array[Dictionary]:
	return rt.peer().sent_of(type) if rt != null and rt.peer() != null else [] as Array[Dictionary]


## Coroutine: a few frames for the coroutines (FakeHttp answers after one frame).
func settle(frames: int = 4) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	for i: int in frames:
		await tree.process_frame


func dispose() -> void:
	if services != null:
		services.chat.clear()
		services.boss.clear()
		services.coop_quest.clear()
		services.social.clear()
		_unregister()
		if rt != null:
			rt.client.unregister_handler(BossClient.PREFIX, services.boss.handle_message)
			rt.client.unregister_handler(CoopQuestService.PREFIX, services.coop_quest.handle_message)
			rt.client.unregister_handler(PresenceService.PREFIX, services.presence.handle_message)
	if rt != null:
		rt.dispose()
	if h != null:
		h.dispose()
	AppState.services = _previous
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)
	AppState.end_blocking(BossStore.BLOCKING_ACTIVITY)
	QualityManager.resume_world(BossArena.SUSPEND_REASON)


## The services built on the autoload client registered their handlers there; they are replaced here.
func _unregister() -> void:
	var auto: GameRealtimeClient = services.realtime
	if auto == null:
		return
	auto.unregister_handler(BossClient.PREFIX, services.boss.handle_message)
	auto.unregister_handler(CoopQuestService.PREFIX, services.coop_quest.handle_message)
	auto.unregister_handler(PresenceService.PREFIX, services.presence.handle_message)
	auto.unregister_handler("notification", services.social.handle_notification)
