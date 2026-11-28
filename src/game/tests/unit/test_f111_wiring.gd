extends GdUnitTestSuite
## T009 (F111 plan "tích hợp"): AppState reads `policies.social` of the bootstrap (absent → no "Tạo club"); the
## ServiceLocator builds JoinFriendService, ClubService, LiveEventService and SeasonTrackService and clears them on logout;
## the realtime dispatcher hands `event.*` to LiveEventService and `season.track.updated` to SeasonTrackService; the
## club / event deep links of a notification (`open_club`, `open_event`) open the club tab of Bạn bè and the Event center
## (UiScreenRegistry `event_center`); an array query (`status=live,upcoming`) is sent as repeated keys (OpenAPI form).

var rt: RealtimeHarness = null
var h: AuthHarness = null
var _previous: ServiceLocator = null


func before_test() -> void:
	_previous = AppState.services
	h = AuthHarness.create()
	await h.sign_in()
	rt = RealtimeHarness.create()


func after_test() -> void:
	AppState.services = _previous
	AppState.school_policies = {}
	AppState.social_policy = SocialPolicyModel.from_policies({})
	rt.dispose()
	h.dispose()


func test_bootstrap_social_policy() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock, rt.client)
	AppState.services = services
	var b: Dictionary = AuthHarness.bootstrap()
	b["policies"] = {"social": {"student_club_creation": "requires_approval", "club_creation_eligible": true,
		"cross_school_enabled": false}}
	AppState.set_player(b)
	assert_bool(AppState.social_policy.can_show_create()).is_true()
	AppState.clear_player()
	assert_bool(AppState.social_policy.can_show_create()).is_false()
	b["policies"] = {}
	AppState.set_player(b)
	assert_str(AppState.social_policy.student_club_creation).is_equal("disabled")
	AppState.clear_player()


func test_services_are_built_dispatched_and_cleared() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock, rt.client)
	assert_object(services.join_friend).is_not_null()
	assert_object(services.clubs).is_not_null()
	assert_object(services.live_events).is_not_null()
	assert_object(services.season).is_not_null()
	await rt.connect_online()
	var now: int = int(rt.client.server_now_ms() / 1000)
	h.http.reply(F111Fixtures.ROUTE_EVENTS, 200, F111Fixtures.events(now))
	h.http.reply(F111Fixtures.ROUTE_SEASON, 200, F111Fixtures.season())
	await services.live_events.load_list()
	await services.season.load_track()
	rt.push(rt.envelope("event.progress.updated", F111Fixtures.ws("event.progress.updated", {
		"event_id": F111Fixtures.EVENT_FESTIVAL, "event_run_id": F111Fixtures.RUN_FESTIVAL, "event_points": 140,
		"daily_points_remaining": 60, "milestones_reached": []})))
	assert_int(services.live_events.store.event(F111Fixtures.EVENT_FESTIVAL).event_points).is_equal(140)
	rt.push(rt.envelope("season.track.updated", F111Fixtures.ws("season.track.updated", {"season_id": F111Fixtures.SEASON_ID,
		"points": 330, "tier": 4, "daily_points_remaining": 20})))
	assert_int(services.season.store.track.points).is_equal(330)
	services.end_session()
	assert_bool(services.live_events.store.events.is_empty()).is_true()
	assert_object(services.season.store.track).is_null()
	assert_bool(services.clubs.store.mine.is_empty()).is_true()


func test_deep_links_open_the_club_and_the_event_center() -> void:
	assert_bool(UiScreenRegistry.has(&"event_center")).is_true()
	var n: NotificationService = NotificationService.new(h.api)
	var opened: Array = []
	var sink: Callable = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	EventBus.ui_requested.connect(sink)
	var item: NotificationModel = NotificationModel.from_dict(
			F111Fixtures.asyncapi("notification.item.created.club_invitation")["payload"])
	assert_bool(n.open(item)).is_true()
	assert_str(String(opened[0][0])).is_equal("friends")
	assert_str(String((opened[0][1] as Dictionary).get("tab", ""))).is_equal("clubs")
	assert_str(String((opened[0][1] as Dictionary).get("id", ""))).is_equal(F111Fixtures.CLUB_SCHOOL)
	var ev: Dictionary = (F111Fixtures.asyncapi("notification.item.created.club_approved")["payload"] as Dictionary).duplicate(true)
	ev["action"] = {"kind": "open_event", "id": F111Fixtures.EVENT_FESTIVAL}
	assert_bool(n.open(NotificationModel.from_dict(ev))).is_true()
	EventBus.ui_requested.disconnect(sink)
	assert_str(String(opened[1][0])).is_equal("event_center")
	assert_str(String((opened[1][1] as Dictionary).get("id", ""))).is_equal(F111Fixtures.EVENT_FESTIVAL)


func test_array_query_is_sent_as_repeated_keys() -> void:
	h.http.reply(F111Fixtures.ROUTE_EVENTS, 200, F111Fixtures.page([]))
	await h.api.send(ApiRequest.make_get("/game/events", {"status": ["live", "upcoming"], "limit": 100}))
	assert_str(h.http.last(F111Fixtures.ROUTE_EVENTS).query).is_equal("status=live&status=upcoming&limit=100")
