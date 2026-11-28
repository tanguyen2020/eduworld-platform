class_name F111Screens
extends RefCounted
## TEST-ONLY (AGENTS R17): the F111 screens built the same way for the grade-band matrix, the compact probes and the
## review captures, on the contract fixtures (F111Fixtures, F110Fixtures) — no network, no server state. Views:
## `friends_join` (a friend's profile with "Tham gia bạn" grey + its reason), `friends_suggest` ("Tìm bạn" with "Vừa chơi
## cùng"), `clubs` (the club tab; the school policy is set to allow "Tạo club" — the caller restores it),
## `club_page` (owner: members), `club_goals`, `club_requests`, `club_readonly` (a locked
## club), `club_create` (the form), `club_invite` (the picker after a send), `events_today`, `events_done` (every daily
## quest done), `events_week`, `events_list`, `event_detail`, `event_ended`, `events_season`, `events_no_season`.

const VIEWS: PackedStringArray = ["friends_join", "friends_suggest", "clubs", "club_page", "club_goals", "club_requests",
	"club_readonly", "club_create", "club_invite", "events_today", "events_done", "events_week", "events_list",
	"event_detail", "event_ended", "events_season", "events_no_season"]
const FRAMES: int = 10
## Review captures of tools/capture_screens.gd (`f111_<view>`; `f111_hub_events` = the HUD with the calendar button and
## its dot, `f111_game_menu` = the "Thêm" menu with the "Sự kiện" tile).
const CAPTURES: PackedStringArray = ["f111_friends_join", "f111_friends_suggest", "f111_clubs", "f111_club_page",
	"f111_club_goals", "f111_club_requests", "f111_club_readonly", "f111_club_create", "f111_club_invite", "f111_events_today",
	"f111_events_done", "f111_events_week", "f111_events_list", "f111_event_detail", "f111_event_ended", "f111_events_season",
	"f111_events_no_season", "f111_hub_events", "f111_game_menu"]


## Coroutine (review captures): `view` over `world` on a layer above it, or the HUD / the "Thêm" menu of the world.
static func capture(world: Node, view: String, http: FakeHttp) -> Node:
	var services: ServiceLocator = AppState.get_services()
	_script(services, http)
	if view == "hub_events":
		await services.live_events.load_list()
	elif view == "game_menu":
		EventBus.ui_requested.emit(&"game_menu", {})
	else:
		var layer: CanvasLayer = CanvasLayer.new()
		layer.name = "F111Layer"
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
	var self_id: String = CompetitionPlayer.self_id(AppState)
	F110Fixtures.script(http, self_id, services.boss.now_ms())
	F111Fixtures.script(http, self_id, services.live_events.now_unix())


## Coroutine: builds `view` in `parent` (a full-screen Control) and returns the screen root.
static func build(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> Control:
	_script(services, http)
	var now: int = services.live_events.now_unix()
	if view.begins_with("friends") or view.begins_with("club"):
		return await _friends(view, parent, services, http, now)
	return await _events(view, parent, services, http, now)


static func _frames(node: Node, n: int = FRAMES) -> void:
	for i: int in n:
		await node.get_tree().process_frame


static func _friends(view: String, parent: Control, services: ServiceLocator, http: FakeHttp, now: int) -> Control:
	AppState.social_policy = SocialPolicyModel.from_policies({"social": {"student_club_creation": "requires_approval",
		"club_creation_eligible": true}})
	match view:
		"club_page", "club_goals", "club_requests", "club_invite":
			http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 200,
					F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner", 6))
		"club_readonly":
			http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 200,
					F111Fixtures.club(F111Fixtures.CLUB_LOCKED, "CLB Cờ vua", "interest_group", "locked", "member", 8))
	var screen: FriendsScreen = (load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate() as FriendsScreen
	screen.services = services
	parent.add_child(screen)
	var args: Dictionary = {"tab": FriendsScreen.TAB_FRIENDS}
	if view == "friends_suggest":
		args = {"tab": FriendsScreen.TAB_FIND}
	elif view == "clubs" or view == "club_create":
		args = {"tab": FriendsScreen.TAB_CLUBS}
	elif view.begins_with("club_"):
		args = {"tab": FriendsScreen.TAB_CLUBS, "id": F111Fixtures.CLUB_LOCKED if view == "club_readonly" else F111Fixtures.CLUB_OWN}
	screen.open(args)
	await _frames(screen, 16)
	match view:
		"friends_join":
			services.join_friend.self_state = func() -> Dictionary: return {"offline": false}
			services.presence.handle_message("presence.friend.updated", F110Fixtures.ws("presence.friend.updated",
				{"user_id": F111Fixtures.FRIEND_ONLINE, "status": "in_match", "activity": "battle", "area_id": "arena"}))
			services.social.store.flush(Time.get_ticks_msec() + 5000)
			screen.select_friend(F111Fixtures.FRIEND_ONLINE)
		"club_goals":
			screen.club_view().page_view().select_section(ClubPage.SECTION_GOALS)
		"club_requests":
			screen.club_view().page_view().select_section(ClubPage.SECTION_REQUESTS)
		"club_create":
			screen.club_view().create_button().pressed.emit()
			await _frames(screen, 2)
			screen.club_form().set_name_text("Nhà thám hiểm")
			screen.club_form().set_description_text("Cùng nhau khám phá và học vui mỗi tuần")
		"club_invite":
			await services.social.load_friends()
			screen.club_view().page_view().action_button(ClubPage.ACT_INVITE).pressed.emit()
			await _frames(screen, 2)
			screen.invite_picker().show_result({"ok_ids": PackedStringArray([F111Fixtures.FRIEND_ONLINE]),
				"failed_ids": PackedStringArray([F111Fixtures.FRIEND_OFFLINE])})
	await _frames(screen, 6)
	return screen


static func _events(view: String, parent: Control, services: ServiceLocator, http: FakeHttp, now: int) -> Control:
	if view == "events_done":
		http.clear(F111Fixtures.ROUTE_QUESTS_DAILY).reply(F111Fixtures.ROUTE_QUESTS_DAILY, 200, F111Fixtures.daily(now, true))
	if view == "event_ended":
		var d: Dictionary = F111Fixtures.detail(now)
		d["status"] = "ended"
		(d["milestones"] as Array).append(F111Fixtures.milestone("m0", 50, true, true, [F111Fixtures.reward_xp(20)]))
		http.clear(F111Fixtures.ROUTE_EVENT).reply(F111Fixtures.ROUTE_EVENT, 200, d)
	if view == "event_detail":
		http.clear(F111Fixtures.ROUTE_EVENT).reply(F111Fixtures.ROUTE_EVENT, 200, F111Fixtures.detail(now, true,
				[{"grade_level": 7, "progress": 1800}, {"grade_level": 4, "progress": 2600}, {"grade_level": 5, "progress": 900}]))
	if view == "events_no_season":
		http.clear(F111Fixtures.ROUTE_SEASON).reply(F111Fixtures.ROUTE_SEASON, 404, AuthHarness.problem(404, "NOT_FOUND"))
	var screen: EventCenterScreen = (load("res://scenes/liveops/EventCenterScreen.tscn") as PackedScene).instantiate() \
			as EventCenterScreen
	screen.services = services
	parent.add_child(screen)
	var tab: String = {"events_today": "today", "events_done": "today", "events_week": "week", "events_list": "events",
		"events_season": "season", "events_no_season": "season"}.get(view, "events")
	var args: Dictionary = {"tab": tab}
	if view == "event_detail" or view == "event_ended":
		args = {"id": F111Fixtures.EVENT_FESTIVAL}
	await screen.open(args)
	await _frames(screen, 8)
	if view == "event_detail":
		services.live_events.handle_message("event.progress.updated", F111Fixtures.ws("event.progress.updated", {
			"event_id": F111Fixtures.EVENT_FESTIVAL, "event_run_id": F111Fixtures.RUN_FESTIVAL, "event_points": 320,
			"daily_points_remaining": 40, "milestones_reached": ["m2"]}))
	await _frames(screen, 4)
	return screen
