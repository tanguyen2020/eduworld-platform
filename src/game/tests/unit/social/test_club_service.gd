extends GdUnitTestSuite
## T029 (F111-AC-19, F111-AC-21; SOCIAL_SYSTEM §6, ADR-0054, ADR-0095): `listClubs` `scope = mine` and (not for lớp 1–3)
## `scope = discoverable`, plus the student's pending invitations (`listMyClubInvitations`); a legacy `study_group` club is
## never kept; "Xin vào" is `requestJoinClub` and an invitation is accepted with `acceptClubInvitation` — each with one
## Idempotency-Key kept when retried — or declined; `409 CLUB_FULL`, `409 CLUB_LIMIT_REACHED`, `409 CLUB_NOT_APPROVED`,
## `403 SOCIAL_SCOPE_VIOLATION` become their friendly `ui.error.<CODE>` sentence.

var s: F111SceneHarness = null
var clubs: ClubService = null
var notices: Array[String] = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")
	clubs = s.services.clubs
	notices.clear()
	clubs.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))


func after_test() -> void:
	s.dispose()


func test_lists_mine_discoverable_and_invitations_without_study_groups() -> void:
	var r: ApiResult = await clubs.load_lists("g6_7")
	assert_bool(r.ok).is_true()
	assert_int(clubs.store.mine.size()).is_equal(4)
	assert_int(clubs.store.discoverable.size()).is_equal(1)
	assert_int(clubs.store.invitations.size()).is_equal(1)
	for c: ClubModel in clubs.store.mine + clubs.store.discoverable:
		assert_str(c.club_type).is_not_equal("study_group")
	assert_str(s.h.http.last(F111Fixtures.ROUTE_INVITATIONS).query).contains("status=pending")


func test_g1_3_does_not_ask_for_discoverable_clubs() -> void:
	await clubs.load_lists("g1_3")
	for rec: FakeHttp.Recorded in s.h.http.find(F111Fixtures.ROUTE_CLUBS):
		assert_bool(rec.query.contains("discoverable")).is_false()
	assert_bool(clubs.store.discoverable.is_empty()).is_true()
	assert_int(clubs.store.invitations.size()).is_equal(1)


func test_request_join_keeps_its_key_and_shows_waiting() -> void:
	await clubs.load_lists("g6_7")
	s.h.http.clear(F111Fixtures.ROUTE_REQUEST_JOIN).fail(F111Fixtures.ROUTE_REQUEST_JOIN)
	await clubs.request_join(F111Fixtures.CLUB_DISCOVER)
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_REQUEST_JOIN).headers["idempotency-key"])
	assert_bool(clubs.store.is_requested(F111Fixtures.CLUB_DISCOVER)).is_false()
	s.h.http.clear(F111Fixtures.ROUTE_REQUEST_JOIN).reply(F111Fixtures.ROUTE_REQUEST_JOIN, 201,
			F111Fixtures.join_request(F111Fixtures.REQUEST_ID, "MeoXanh42", s.self_id))
	var ok: ApiResult = await clubs.request_join(F111Fixtures.CLUB_DISCOVER)
	assert_bool(ok.ok).is_true()
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_REQUEST_JOIN).headers["idempotency-key"])).is_equal(key)
	assert_bool(clubs.store.is_requested(F111Fixtures.CLUB_DISCOVER)).is_true()
	assert_array(notices).contains(["ui.club.request_sent"])


func test_accept_and_decline_an_invitation() -> void:
	await clubs.load_lists("g6_7")
	var inv: ClubInvitationModel = clubs.store.invitations[0]
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_INVITATION).fail(F111Fixtures.ROUTE_ACCEPT_INVITATION)
	await clubs.accept_invitation(inv)
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_ACCEPT_INVITATION).headers["idempotency-key"])
	s.h.http.clear(F111Fixtures.ROUTE_ACCEPT_INVITATION).reply(F111Fixtures.ROUTE_ACCEPT_INVITATION, 200,
			F111Fixtures.member(s.self_id, "MeoXanh42"))
	# The server no longer lists the accepted invitation.
	s.h.http.clear(F111Fixtures.ROUTE_INVITATIONS).reply(F111Fixtures.ROUTE_INVITATIONS, 200, F111Fixtures.page([]))
	assert_bool((await clubs.accept_invitation(inv)).ok).is_true()
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_ACCEPT_INVITATION).headers["idempotency-key"])).is_equal(key)
	assert_bool(clubs.store.invitations.is_empty()).is_true()
	s.h.http.clear(F111Fixtures.ROUTE_INVITATIONS).reply(F111Fixtures.ROUTE_INVITATIONS, 200, F111Fixtures.invitations())
	await clubs.load_lists("g6_7")
	assert_bool((await clubs.decline_invitation(clubs.store.invitations[0])).ok).is_true()
	assert_int(s.h.http.count(F111Fixtures.ROUTE_DECLINE_INVITATION)).is_equal(1)


func test_problems_become_friendly_sentences() -> void:
	await clubs.load_lists("g6_7")
	for c: Array in [[409, "CLUB_FULL"], [409, "CLUB_LIMIT_REACHED"], [409, "CLUB_NOT_APPROVED"], [403, "SOCIAL_SCOPE_VIOLATION"]]:
		s.h.http.clear(F111Fixtures.ROUTE_REQUEST_JOIN).reply(F111Fixtures.ROUTE_REQUEST_JOIN, int(c[0]),
				AuthHarness.problem(int(c[0]), String(c[1])))
		await clubs.request_join(F111Fixtures.CLUB_DISCOVER)
		assert_str(notices.back()).is_equal("ui.error.%s" % c[1])
		assert_bool(L10n.has_key(notices.back())).is_true()
