class_name F112SceneHarness
extends F111SceneHarness
## TEST-ONLY (AGENTS R17): the F111 harness (signed-in student, FakeHttp, online RealtimeHarness, HubSync in
## `main_hub`) with the chat rebuilt on the realtime harness (joins / leaves / sends are real `chat.*` commands on the fake
## socket) and the F112 services: ChatHubService (tabs, DMs, badge), ChatSearchService, ClubManageService; every F112
## route answered from the contract fixtures (F112Fixtures). The clubs and friends of the student are loaded.

func start(band: String = "g1_3", online: bool = true) -> void:
	await super(band, online)
	var old: ChatService = services.chat
	old.clear()
	services.chat = ChatService.new(h.api, rt.client, AppState, services.telemetry)
	services.chat.assets.cdn_base_url = ChatFixtures.CDN
	services.chat.assets.use_disk = false
	services.chat._refresh_self()
	ChatFixtures.script(h.http, services.chat.store.self_id)
	F112Fixtures.script(h.http)
	services.chat_hub = ChatHubService.new(services.chat, services.clubs, services.social, services.hub, AppState)
	services.chat_search = ChatSearchService.new(h.api, services.telemetry)
	services.club_manage = ClubManageService.new(h.api, services.telemetry, services.clubs)
	await services.social.load_friends()
	await services.clubs.load_lists(band)


## Delivers a realtime `chat.*` message to the chat (as the dispatcher would).
func push_chat(type: String, payload: Dictionary) -> void:
	services.chat.handle_message(type, payload)


func dispose() -> void:
	if services != null and services.chat != null and rt != null:
		rt.client.unregister_handler(ChatService.PREFIX, services.chat.handle_message)
		services.chat_hub.clear()
		services.chat_search.clear()
		services.club_manage.clear()
	super()
