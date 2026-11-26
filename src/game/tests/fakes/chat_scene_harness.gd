class_name ChatSceneHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a signed-in student (AuthHarness on FakeHttp) with the session services in AppState and the
## chat routes scripted from the contract fixtures (ChatFixtures), for the F119 scene tests, the grade-band matrix and
## the review captures. The realtime client is the offline autoload: sends go over the REST fallback; realtime messages
## are delivered with `push()` (the dispatcher handler of ChatService). The chat CDN is `https://cdn.example.invalid`.

var h: AuthHarness = null
var services: ServiceLocator = null
var chat: ChatService = null
var self_id: String = ""
var _hosts: Array[Node] = []


func start() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	chat = services.chat
	chat.assets.cdn_base_url = ChatFixtures.CDN
	chat.assets.use_disk = false
	chat._refresh_self()
	self_id = chat.store.self_id
	ChatFixtures.script(h.http, self_id)


## A ChatDock in `parent` showing the party channel (others: CaHeoVui07, GauNau15), settled.
func party_dock(parent: Node, size: Vector2 = Vector2(560, 520)) -> ChatDock:
	var dock: ChatDock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	dock.set_anchors_preset(Control.PRESET_TOP_LEFT)
	dock.size = size
	parent.add_child(dock)
	dock.size = size
	dock.bind_services(chat, services.social, services.reports)
	dock.show_party(ChatFixtures.PARTY_ID, ChatFixtures.PARTY_CHANNEL, others())
	await settle(dock)
	return dock


## A ChatDock in a team battle: tabs "Đội" (team channel) and "Cả trận" (match channel).
func match_dock(parent: Node, size: Vector2 = Vector2(560, 520)) -> ChatDock:
	var dock: ChatDock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	dock.set_anchors_preset(Control.PRESET_TOP_LEFT)
	parent.add_child(dock)
	dock.size = size
	dock.bind_services(chat, services.social, services.reports)
	dock.show_match(ChatFixtures.MATCH_ID, ChatFixtures.MATCH_CHANNEL, ChatFixtures.TEAM_CHANNEL, others())
	await settle(dock)
	return dock


static func others() -> Array[Dictionary]:
	return [{"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}, {"user_id": ChatFixtures.THIRD, "nickname": "GauNau15"}]


## Lets the coroutines (REST replies one frame later) and the layout settle.
func settle(node: Node, frames: int = 12) -> void:
	var tree: SceneTree = node.get_tree() if node != null and node.is_inside_tree() else Engine.get_main_loop() as SceneTree
	for i: int in frames:
		await tree.process_frame


## Delivers the realtime message `type` with `payload` (AsyncAPI shape) to ChatService.
func push(type: String, payload: Dictionary) -> void:
	chat.handle_message(type, payload)


## Pushes the AsyncAPI example `name` (e.g. "chat.message.created.text").
func push_example(name: String) -> void:
	var env: Dictionary = ChatFixtures.envelope(name, self_id)
	chat.handle_message(String(env["type"]), env["payload"] as Dictionary)


func now_ms() -> int:
	return chat.now_ms()


func dispose() -> void:
	if chat != null:
		chat.clear()
		if services != null and services.realtime != null:
			services.realtime.unregister_handler(ChatService.PREFIX, chat.handle_message)
	if services != null and services.party != null and services.realtime != null:
		services.realtime.unregister_handler(PartyService.PREFIX, services.party.handle_message)
	AppState.services = null
	if h != null:
		h.dispose()
