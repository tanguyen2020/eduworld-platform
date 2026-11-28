class_name F112Screens
extends RefCounted
## TEST-ONLY (AGENTS R17): the F112 screens built the same way for the grade-band matrix and the review captures, on the
## contract fixtures (F112Fixtures) — no network, no server state. Views: `hub_area`, `hub_world`, `hub_club` (a club tab
## with messages), `hub_friends` ("Bạn bè"), `hub_dm` (a DM opened with "Nhắn tin"), `hub_dm_frozen`, `search_hits`,
## `search_empty`, `manage_edit`, `manage_officers`, `manage_goal`, `manage_event` (the owner's "Quản lý club").

const VIEWS: PackedStringArray = ["hub_area", "hub_world", "hub_club", "hub_friends", "hub_dm", "hub_dm_frozen",
	"search_hits", "search_empty", "manage_edit", "manage_officers", "manage_goal", "manage_event"]
## Review captures of tools/capture_screens.gd (`f112_<view>`; `f112_hud` = the HUD with the chat button and its badges).
const CAPTURES: PackedStringArray = ["f112_hub_area", "f112_hub_world", "f112_hub_club", "f112_hub_friends", "f112_hub_dm",
	"f112_hub_dm_frozen", "f112_search_hits", "f112_search_empty", "f112_manage_edit", "f112_manage_officers",
	"f112_manage_goal", "f112_manage_event", "f112_hud"]
const FRAMES: int = 10


## Builds the F112 services on `services` (the session ones of AppState) when they are missing (review captures).
static func ensure_services(services: ServiceLocator) -> void:
	if services.chat_hub == null:
		services.chat_hub = ChatHubService.new(services.chat, services.clubs, services.social, services.hub, AppState)
	if services.chat_search == null:
		services.chat_search = ChatSearchService.new(services.chat.api, services.telemetry)
	if services.club_manage == null:
		services.club_manage = ClubManageService.new(services.chat.api, services.telemetry, services.clubs)


## Coroutine (review captures): `view` over `world` on a layer above it, or the HUD with the chat badge.
static func capture(world: Node, view: String, http: FakeHttp) -> Node:
	var services: ServiceLocator = AppState.get_services()
	ensure_services(services)
	_script(services, http)
	await services.social.load_friends()
	await services.clubs.load_lists(AppState.grade_band)
	if view == "hud":
		await services.chat_hub.refresh()
		services.chat_hub.tick(Time.get_ticks_msec() + 5000)
	else:
		var layer: CanvasLayer = CanvasLayer.new()
		layer.name = "F112Layer"
		layer.layer = 20
		world.add_child(layer)
		var screen: Control = Control.new()
		screen.set_anchors_preset(Control.PRESET_FULL_RECT)
		screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
		layer.add_child(screen)
		await build(view, screen, services, http)
	await _frames(world, 20)
	return world


static func _script(services: ServiceLocator, http: FakeHttp) -> void:
	ChatFixtures.script(http, services.chat.store.self_id)
	F111Fixtures.script(http, CompetitionPlayer.self_id(AppState), int(Time.get_unix_time_from_system()))
	F112Fixtures.script(http)
	var page: Dictionary = ChatFixtures.history(services.chat.store.self_id)
	http.clear(ChatFixtures.ROUTE_MESSAGES).reply(ChatFixtures.ROUTE_MESSAGES, 200, page)


static func _frames(node: Node, n: int = FRAMES) -> void:
	for i: int in n:
		await node.get_tree().process_frame


## Coroutine: builds `view` in `parent` (a full-screen Control) and returns the screen root.
static func build(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> Control:
	ensure_services(services)
	_script(services, http)
	if view.begins_with("manage"):
		return await _manage(view, parent, services)
	var hub: ChatHub = (load("res://scenes/chat/ChatHub.tscn") as PackedScene).instantiate() as ChatHub
	hub.services = services
	parent.add_child(hub)
	var tab: String = {"hub_area": "area", "hub_world": "world", "hub_friends": "friends"}.get(view,
			"club:%s" % F111Fixtures.CLUB_SCHOOL)
	await hub.open({"tab": tab})
	await _frames(hub, 8)
	match view:
		"hub_dm", "hub_dm_frozen":
			await services.chat_hub.open_dm(F112Fixtures.FRIEND_DM)
			await _frames(hub, 6)
			if view == "hub_dm_frozen":
				services.chat.handle_message("chat.channel.membership_changed", {"channel_id": F112Fixtures.DM_KNOWN,
					"channel_type": "direct", "op": "status_changed", "status": "frozen"})
		"search_hits", "search_empty":
			if view == "search_empty":
				http.clear(F112Fixtures.ROUTE_SEARCH).reply(F112Fixtures.ROUTE_SEARCH, 200, {"data": [],
					"page": {"has_more": false}})
			hub.search_button().pressed.emit()
			await _frames(hub, 2)
			hub.search_panel().set_query("đi boss")
			await hub.search_panel().submit()
	# The history example of the contract on the channel on screen (the fake answers one page for every channel).
	if not hub.dock().current_channel.is_empty() and not view.begins_with("search"):
		_fill(services, hub.dock().current_channel)
	await _frames(hub, 6)
	return hub


static func _fill(services: ServiceLocator, channel_id: String) -> void:
	var page: Dictionary = ChatFixtures.history(services.chat.store.self_id)
	var msgs: Array[ChatMessageModel] = []
	var ch: ChatChannelModel = services.chat.store.channel(channel_id)
	var direct: bool = ch != null and ch.channel_type == ChatChannelModel.TYPE_DIRECT
	for raw: Variant in page["data"] as Array:
		var d: Dictionary = (raw as Dictionary).duplicate(true)
		# A DM holds only the student and the friend.
		var sender: String = String((d.get("sender", {}) as Dictionary).get("user_id", ""))
		if direct and sender != services.chat.store.self_id and sender != F112Fixtures.FRIEND_DM:
			continue
		d["channel_id"] = channel_id
		var m: ChatMessageModel = ChatMessageModel.from_dict(d)
		if m != null:
			msgs.append(m)
	msgs.sort_custom(func(a: ChatMessageModel, b: ChatMessageModel) -> bool: return a.seq < b.seq)
	services.chat.store.load_history(channel_id, msgs, false)


static func _manage(view: String, parent: Control, services: ServiceLocator) -> Control:
	var screen: FriendsScreen = (load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate() as FriendsScreen
	screen.services = services
	parent.add_child(screen)
	screen.open({"tab": FriendsScreen.TAB_CLUBS, "id": F111Fixtures.CLUB_OWN})
	await _frames(screen, 16)
	screen.club_view().page_view().action_button(ClubPage.ACT_MANAGE).pressed.emit()
	await _frames(screen, 10)
	var manage: ClubManage = screen.club_manage()
	var section: String = {"manage_officers": ClubManage.SECTION_OFFICERS, "manage_goal": ClubManage.SECTION_GOAL,
		"manage_event": ClubManage.SECTION_EVENT}.get(view, ClubManage.SECTION_EDIT)
	manage.select_section(section)
	if view == "manage_event":
		manage.set_event({"title": "Luyện tập chung", "description": "Cả nhóm cùng luyện Toán nhé", "day_offset": 2,
			"hour": 9, "minute": 0, "length": 60})
	if view == "manage_goal":
		manage.set_target(30)
		manage.set_goal_title("Cả club hoàn thành 30 bài")
	await _frames(screen, 6)
	return screen
