extends GdUnitTestSuite
## T020 (F111-AC-11, F111-AC-14; ADR-0016, ADR-0049): "Nhận" of a reached milestone is `claimEventMilestone` with one
## Idempotency-Key kept when retried; the milestone reads "Đã nhận" ONLY after the server answered `RewardClaimResult` and
## the rewards shown are the server's; `409 EVENT_MILESTONE_NOT_REACHED` reads the detail again; `event.progress.updated`
## replaces the points and the daily points left with the server values (the same message twice adds nothing) and marks
## the milestone just reached with a static badge; `event.community_goal.updated` replaces the bar.

var s: F111SceneHarness = null
var live: LiveEventService = null
var rewards: Array = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	live = s.services.live_events
	rewards.clear()
	live.reward_claimed.connect(func(list: Array) -> void: rewards.append(list))
	await live.load_list()
	await live.get_detail(F111Fixtures.EVENT_FESTIVAL)


func after_test() -> void:
	s.dispose()


func _m(id: String) -> EventMilestoneModel:
	return live.store.event(F111Fixtures.EVENT_FESTIVAL).milestone(id)


func test_claim_keeps_its_key_and_marks_claimed_only_after_the_server() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_MILESTONE_CLAIM).fail(F111Fixtures.ROUTE_MILESTONE_CLAIM)
	await live.claim_milestone(F111Fixtures.EVENT_FESTIVAL, "m1")
	assert_bool(_m("m1").claimed).is_false()
	assert_array(rewards).is_empty()
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_MILESTONE_CLAIM).headers["idempotency-key"])
	s.h.http.clear(F111Fixtures.ROUTE_MILESTONE_CLAIM).reply(F111Fixtures.ROUTE_MILESTONE_CLAIM, 200,
			F111Fixtures.claim_result([F111Fixtures.reward_xp(50), F111Fixtures.reward_item("Đèn lồng trăng", "Moon lantern")]))
	var r: ApiResult = await live.claim_milestone(F111Fixtures.EVENT_FESTIVAL, "m1")
	assert_bool(r.ok).is_true()
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_MILESTONE_CLAIM).headers["idempotency-key"])).is_equal(key)
	assert_str(s.h.http.last(F111Fixtures.ROUTE_MILESTONE_CLAIM).path).contains("/milestones/m1:claim")
	assert_bool(_m("m1").claimed).is_true()
	assert_int(rewards.size()).is_equal(1)
	assert_int((rewards[0] as Array).size()).is_equal(2)
	assert_str(((rewards[0] as Array)[1] as RewardModel).display_text()).is_equal("Đèn lồng trăng")


func test_not_reached_reads_the_detail_again() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_MILESTONE_CLAIM).reply(F111Fixtures.ROUTE_MILESTONE_CLAIM, 409,
			AuthHarness.problem(409, "EVENT_MILESTONE_NOT_REACHED"))
	var details: int = s.h.http.count(F111Fixtures.ROUTE_EVENT)
	await live.claim_milestone(F111Fixtures.EVENT_FESTIVAL, "m2")
	assert_bool(_m("m2").claimed).is_false()
	assert_int(s.h.http.count(F111Fixtures.ROUTE_EVENT)).is_greater(details)


func test_progress_replaces_values_and_never_adds() -> void:
	var msg: Dictionary = F111Fixtures.ws("event.progress.updated", {"event_id": F111Fixtures.EVENT_FESTIVAL,
		"event_run_id": F111Fixtures.RUN_FESTIVAL, "delta": 200, "event_points": 320, "daily_points_remaining": 0,
		"milestones_reached": ["m2"]})
	s.push("event.progress.updated", msg)
	s.push("event.progress.updated", msg)
	var e: LiveEventModel = live.store.event(F111Fixtures.EVENT_FESTIVAL)
	assert_int(e.event_points).is_equal(320)
	assert_int(e.daily_points_remaining).is_equal(0)
	assert_bool(_m("m2").reached).is_true()
	assert_bool(_m("m2").just_reached).is_true()
	assert_bool(_m("m2").claimed).is_false()


func test_community_goal_update_replaces_the_bar() -> void:
	s.push("event.community_goal.updated", F111Fixtures.ws("event.community_goal.updated", {"goal_id": F111Fixtures.GOAL_ID,
		"event_run_id": F111Fixtures.RUN_FESTIVAL, "target": 10000, "progress": 7600, "milestone_reached": 75}))
	var g: CommunityGoalModel = live.store.event(F111Fixtures.EVENT_FESTIVAL).community_goal
	assert_int(g.progress).is_equal(7600)
	assert_bool(75 in g.milestones_reached).is_true()
	var fresh: ApiResult = await live.get_goal(F111Fixtures.GOAL_ID)
	assert_bool(fresh.ok).is_true()
	assert_int(live.store.event(F111Fixtures.EVENT_FESTIVAL).community_goal.progress).is_equal(6200)
