extends GdUnitTestSuite
## T041 (F111-AC-31; AGENTS R23, ADR-0065, ADR-0041): every F111 request goes through ApiClient with the session tenant
## header; `403 GAME_ACCESS_STUDENT_ONLY` (staff) ends the session through the F061 flow (`access_denied`); a `404` of an
## event, a club or an invitation closes that part and reads the lists again without details; `422 VALIDATION_FAILED` /
## `428` queue a `client_bug` telemetry event with the operation id only (no PII).

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	s.dispose()


func test_every_f111_request_carries_the_tenant_header() -> void:
	await s.services.social.list_suggestions()
	await s.services.clubs.load_lists("g6_7")
	await s.services.clubs.open_club(F111Fixtures.CLUB_OWN)
	await s.services.live_events.load_list()
	await s.services.live_events.get_detail(F111Fixtures.EVENT_FESTIVAL)
	await s.services.season.load_track()
	await s.services.season.load_history()
	await s.services.quest.load_type("daily")
	var checked: int = 0
	for rec: FakeHttp.Recorded in s.h.http.requests:
		if rec.path.begins_with("/social/club") or rec.path.begins_with("/social/friend-suggestions") \
				or rec.path.begins_with("/game/events") or rec.path.begins_with("/game/season-track") \
				or rec.path.begins_with("/game/quests"):
			assert_str(String(rec.headers.get("x-tenant-id", ""))).override_failure_message(rec.path).is_equal(AuthHarness.TENANT_ID)
			assert_str(String(rec.headers.get("authorization", ""))).starts_with("Bearer ")
			checked += 1
	assert_int(checked).is_greater_equal(12)


func test_staff_refused_goes_to_the_f061_flow() -> void:
	var denied: Array[String] = []
	s.services.access_denied_handler = func(code: String) -> void: denied.append(code)
	for svc: GameApiService in [s.services.clubs, s.services.live_events, s.services.season]:
		if not svc.access_denied.is_connected(s.services._on_access_denied):
			svc.access_denied.connect(s.services._on_access_denied)
	for route: String in [F111Fixtures.ROUTE_EVENTS, F111Fixtures.ROUTE_SEASON, F111Fixtures.ROUTE_CLUBS_MINE]:
		s.h.http.clear(route).reply(route, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	await s.services.live_events.load_list()
	await s.services.season.load_track()
	await s.services.clubs.load_lists("g6_7")
	assert_array(denied).is_equal(["GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY"])


func test_a_404_closes_the_part_without_details() -> void:
	var closed: Array[String] = []
	s.services.live_events.detail_closed.connect(func(id: String, _k: String) -> void: closed.append(id))
	s.services.clubs.page_closed.connect(func(id: String) -> void: closed.append(id))
	s.h.http.clear(F111Fixtures.ROUTE_EVENT).reply(F111Fixtures.ROUTE_EVENT, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.live_events.get_detail(F111Fixtures.EVENT_FESTIVAL)
	s.h.http.clear(F111Fixtures.ROUTE_CLUB).reply(F111Fixtures.ROUTE_CLUB, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.clubs.open_club(F111Fixtures.CLUB_OWN)
	assert_array(closed).contains_exactly([F111Fixtures.EVENT_FESTIVAL, F111Fixtures.CLUB_OWN])
	await s.services.clubs.load_lists("g6_7")
	var inv: ClubInvitationModel = s.services.clubs.store.invitations[0]
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_INVITATION).reply(F111Fixtures.ROUTE_ACCEPT_INVITATION, 404,
			AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.clubs.accept_invitation(inv)
	for i: ClubInvitationModel in s.services.clubs.store.invitations:
		assert_str(i.invitation_id).is_not_equal(inv.invitation_id)


func test_validation_failed_and_428_are_client_bug_events_without_personal_data() -> void:
	var telemetry: TelemetryService = s.services.telemetry
	await s.services.live_events.load_list()
	for c: Array in [[F111Fixtures.ROUTE_EVENT_JOIN, 428, "PRECONDITION_REQUIRED", "joinGameEvent"],
			[F111Fixtures.ROUTE_CLUB_CREATE, 422, "VALIDATION_FAILED", "createClub"]]:
		telemetry.clear()
		s.h.http.clear(String(c[0])).reply(String(c[0]), int(c[1]), AuthHarness.problem(int(c[1]), String(c[2])))
		if String(c[3]) == "joinGameEvent":
			await s.services.live_events.join(F111Fixtures.EVENT_FESTIVAL)
		else:
			await s.services.clubs.create({"club_type": "interest_group", "name": "Nhà thám hiểm"})
		assert_int(telemetry.queue.size()).override_failure_message(String(c[3])).is_equal(1)
		assert_str(String(telemetry.queue[0]["error_code"])).is_equal("client_bug")
		assert_str(String(telemetry.queue[0]["scene_code"])).is_equal(String(c[3]))
		var dump: String = JSON.stringify(telemetry.queue[0])
		for pii: String in ["Nhà thám hiểm", "MeoXanh42", s.self_id]:
			assert_str(dump).not_contains(pii)
