extends GdUnitTestSuite
## T021, T025 (F112-AC-18…AC-23; TC-SOC-004, TC-SOC-006, TC-SOC-007): the owner's tools — `updateClub` with only the
## changed fields and `If-Match` = the version read last; `412` → `getClub` again + "Thông tin club vừa thay đổi…"; `451`
## → the general moderation sentence (nothing echoed); `422` → the field; `409 CLUB_NOT_APPROVED`; `403` → the tools
## close and the page is read again; members of every page; `updateClubMemberRole` officer / member, never the owner;
## goals only with the three accepted metrics and a target in the contract bounds, `Idempotency-Key` kept when retried
## after a network failure (one goal); events without `internal_tournament`, never in the past, ISO UTC times.

var s: F112SceneHarness = null
var manage: ClubManageService = null
var revoked: Array[String] = []
var saved: Array[String] = []


func before_test() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	manage = s.services.club_manage
	revoked.clear()
	saved.clear()
	manage.tools_revoked.connect(func(id: String) -> void: revoked.append(id))
	manage.saved.connect(func(what: String) -> void: saved.append(what))
	await manage.load_club(F111Fixtures.CLUB_OWN)


func after_test() -> void:
	s.dispose()


func test_only_the_owner_of_an_active_club_manages() -> void:
	var own: ClubModel = ClubModel.from_dict(F111Fixtures.club(F111Fixtures.CLUB_OWN, "A", "interest_group", "active", "owner"))
	assert_bool(ClubManageService.can_manage(own)).is_true()
	for spec: Array in [["active", "officer"], ["active", "member"], ["pending_approval", "owner"], ["locked", "owner"],
			["archived", "owner"]]:
		var c: ClubModel = ClubModel.from_dict(F111Fixtures.club(F111Fixtures.CLUB_OWN, "A", "interest_group",
				String(spec[0]), String(spec[1])))
		assert_bool(ClubManageService.can_manage(c)).override_failure_message(str(spec)).is_false()


func test_load_reads_club_members_and_accepted_metrics() -> void:
	assert_str(manage.club.club_id).is_equal(F111Fixtures.CLUB_OWN)
	assert_int(manage.club.version).is_equal(1)
	assert_int(manage.members.size()).is_greater(0)
	assert_array(Array(manage.accepted_metrics)).contains(["activities_completed"])


func test_save_sends_only_changes_with_if_match() -> void:
	var r: ApiResult = await manage.save_club({"name": manage.club.name, "description": "Mô tả mới", "icon_id": manage.club.icon_id})
	assert_bool(r.ok).is_true()
	var req: FakeHttp.Recorded = s.h.http.last(F112Fixtures.ROUTE_CLUB_PATCH)
	assert_array(req.body.keys()).is_equal(["description"])
	assert_str(String(req.headers.get("if-match", ""))).is_equal("\"1\"")
	assert_array(saved).is_equal(["club"])
	# Nothing changed → nothing sent.
	var before: int = s.h.http.count(F112Fixtures.ROUTE_CLUB_PATCH)
	await manage.save_club({"name": manage.club.name, "description": manage.club.description, "icon_id": manage.club.icon_id})
	assert_int(s.h.http.count(F112Fixtures.ROUTE_CLUB_PATCH)).is_equal(before)


func test_412_reloads_and_asks_to_check_again() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_CLUB_PATCH).reply(F112Fixtures.ROUTE_CLUB_PATCH, 412,
			F112Fixtures.openapi("updateClub.412"))
	var newer: Dictionary = F111Fixtures.club(F111Fixtures.CLUB_OWN, "Tên mới hơn", "interest_group", "active", "owner", 6)
	newer["version"] = 2
	s.h.http.clear(F112Fixtures.ROUTE_CLUB).reply(F112Fixtures.ROUTE_CLUB, 200, newer)
	var gets: int = s.h.http.count(F112Fixtures.ROUTE_CLUB)
	await manage.save_club({"name": "Tên của em"})
	assert_int(s.h.http.count(F112Fixtures.ROUTE_CLUB)).is_greater(gets)
	assert_str(manage.form_error).is_equal(ClubManageService.KEY_CONFLICT)
	assert_str(manage.club.name).is_equal("Tên mới hơn")
	assert_int(manage.club.version).is_equal(2)


func test_moderation_validation_and_approval_problems() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_CLUB_PATCH).reply(F112Fixtures.ROUTE_CLUB_PATCH, 451,
			F112Fixtures.openapi("updateClub.451"))
	await manage.save_club({"name": "Tên lạ lắm"})
	assert_str(manage.form_error).is_equal(ClubManageService.KEY_BLOCKED)
	assert_str(TranslationServer.translate(manage.form_error)).not_contains("Tên lạ lắm")
	s.h.http.clear(F112Fixtures.ROUTE_GOAL_CREATE).reply(F112Fixtures.ROUTE_GOAL_CREATE, 422,
			F112Fixtures.openapi("createClubGoal.422"))
	await manage.create_goal({"metric": "activities_completed", "target": 30})
	assert_str(String(manage.field_errors.get("metric", ""))).is_equal(ClubManageService.KEY_FIELD)
	s.h.http.clear(F112Fixtures.ROUTE_EVENT_CREATE).reply(F112Fixtures.ROUTE_EVENT_CREATE, 409,
			AuthHarness.problem(409, "CLUB_NOT_APPROVED"))
	await manage.create_event(_event(), s.now_sec)
	assert_str(manage.form_error).is_equal(ClubManageService.KEY_NOT_APPROVED)


func test_403_closes_the_tools_and_reads_the_page_again() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_CLUB_PATCH).reply(F112Fixtures.ROUTE_CLUB_PATCH, 403,
			AuthHarness.problem(403, "PERMISSION_DENIED"))
	var gets: int = s.h.http.count(F112Fixtures.ROUTE_CLUB)
	await manage.save_club({"description": "x"})
	assert_array(revoked).is_equal([F111Fixtures.CLUB_OWN])
	assert_str(manage.form_error).is_equal(ClubManageService.KEY_REVOKED)
	assert_int(s.h.http.count(F112Fixtures.ROUTE_CLUB)).is_greater(gets)


func test_role_officer_and_back_never_the_owner() -> void:
	var member: ClubMemberModel = null
	var owner: ClubMemberModel = null
	for m: ClubMemberModel in manage.members:
		if m.role == ClubModel.ROLE_MEMBER and member == null:
			member = m
		if m.role == ClubModel.ROLE_OWNER:
			owner = m
	assert_object(member).is_not_null()
	var answer: Dictionary = F112Fixtures.openapi("updateClubMemberRole.200")
	answer["user_id"] = member.user_id
	answer["role"] = "officer"
	s.h.http.clear(F112Fixtures.ROUTE_MEMBER_PATCH).reply(F112Fixtures.ROUTE_MEMBER_PATCH, 200, answer)
	await manage.set_role(member.user_id, true)
	var req: FakeHttp.Recorded = s.h.http.last(F112Fixtures.ROUTE_MEMBER_PATCH)
	assert_str(req.path).ends_with("/members/%s" % member.user_id)
	assert_str(String(req.body["role"])).is_equal("officer")
	assert_str(manage.member(member.user_id).role).is_equal("officer")
	answer["role"] = "member"
	s.h.http.clear(F112Fixtures.ROUTE_MEMBER_PATCH).reply(F112Fixtures.ROUTE_MEMBER_PATCH, 200, answer)
	await manage.set_role(member.user_id, false)
	assert_str(String(s.h.http.last(F112Fixtures.ROUTE_MEMBER_PATCH).body["role"])).is_equal("member")
	if owner != null:
		var before: int = s.h.http.count(F112Fixtures.ROUTE_MEMBER_PATCH)
		await manage.set_role(owner.user_id, false)
		assert_int(s.h.http.count(F112Fixtures.ROUTE_MEMBER_PATCH)).is_equal(before)


func test_goal_metrics_target_bounds_and_one_key_when_retried() -> void:
	assert_array(Array(ClubFormOptions.metrics())).is_equal(["activities_completed", "quests_completed", "discoveries_found"])
	assert_array(Array(ClubFormOptions.metrics(PackedStringArray(["quests_completed", "boss_defeated"])))).is_equal(
			["quests_completed"])
	for bad: Dictionary in [{"metric": "minutes_learning", "target": 10}, {"metric": "activities_completed", "target": 0},
			{"metric": "activities_completed", "target": 100001}]:
		assert_dict(ClubFormOptions.goal_body(bad)).is_empty()
	s.h.http.clear(F112Fixtures.ROUTE_GOAL_CREATE).fail(F112Fixtures.ROUTE_GOAL_CREATE).reply(F112Fixtures.ROUTE_GOAL_CREATE,
			201, F112Fixtures.openapi("createClubGoal.201"))
	var form: Dictionary = {"metric": "activities_completed", "target": 30, "title": "Cả club hoàn thành 30 bài"}
	# The network failure is retried (ApiClient) with the SAME key: one goal.
	var result: ApiResult = await manage.create_goal(form)
	assert_bool(result.ok).is_true()
	var reqs: Array[FakeHttp.Recorded] = s.h.http.find(F112Fixtures.ROUTE_GOAL_CREATE)
	assert_int(reqs.size()).is_greater_equal(2)
	var keys: Dictionary = {}
	for r: FakeHttp.Recorded in reqs:
		keys[String(r.headers.get("idempotency-key", ""))] = true
		assert_bool(r.body.has("subject_id")).is_false()
	assert_int(keys.size()).is_equal(1)
	assert_bool(saved.has("goal")).is_true()


func test_event_kinds_and_no_time_in_the_past() -> void:
	assert_array(Array(ClubFormOptions.event_kinds())).is_equal(["practice_session", "coop_quest", "boss_raid", "other"])
	var past: Dictionary = _event()
	past["starts_unix"] = s.now_sec - 60
	assert_str(ClubFormOptions.check_event(past, s.now_sec)).is_equal("ui.club.manage.event.past")
	var before: int = s.h.http.count(F112Fixtures.ROUTE_EVENT_CREATE)
	await manage.create_event(past, s.now_sec)
	assert_int(s.h.http.count(F112Fixtures.ROUTE_EVENT_CREATE)).is_equal(before)
	var r: ApiResult = await manage.create_event(_event(), s.now_sec)
	assert_bool(r.ok).is_true()
	var req: FakeHttp.Recorded = s.h.http.last(F112Fixtures.ROUTE_EVENT_CREATE)
	assert_str(String(req.body["event_kind"])).is_equal("practice_session")
	assert_bool(req.body.has("tournament_template_id")).is_false()
	assert_str(String(req.body["starts_at"])).ends_with("Z")
	assert_str(String(req.headers.get("idempotency-key", ""))).is_not_empty()
	assert_bool(saved.has("event")).is_true()


func _event() -> Dictionary:
	return {"title": "Luyện tập chung", "description": "Cả nhóm cùng luyện", "event_kind": "practice_session",
		"starts_unix": s.now_sec + 2 * 86400, "ends_unix": s.now_sec + 2 * 86400 + 3600}
