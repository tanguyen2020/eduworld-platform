extends GdUnitTestSuite
## F066-AC-22 (TC-SIM-003; T038): the action log keeps t_ms relative to the run and never decreasing, only actions of
## the template, ≤ 20 arguments and ≤ 500 actions; LearningService sends a run with an Idempotency-Key that stays the
## same when the run is sent again after a network loss.

var now: int = 1000


func _log() -> SimulationActionLog:
	var l: SimulationActionLog = SimulationActionLog.new(PackedStringArray(["place_mass", "release"]))
	l.now_msec = func() -> int: return now
	return l


func test_relative_times_and_template_actions() -> void:
	var l: SimulationActionLog = _log()
	now = 5000
	l.start()
	now = 5400
	assert_bool(l.record("place_mass", {"mass_index": 0, "side": "left", "slot": 2})).is_true()
	assert_bool(l.record("teleport", {})).is_false()
	now = 6100
	assert_bool(l.record("release")).is_true()
	var p: Array = l.to_payload()
	assert_int(p.size()).is_equal(2)
	assert_int(int(p[0]["t_ms"])).is_equal(400)
	assert_int(int(p[1]["t_ms"])).is_equal(1100)
	var many: Dictionary = {}
	for i: int in 21:
		many["a%d" % i] = i
	assert_bool(l.record("place_mass", many)).is_false()
	var run: Dictionary = l.run_input("physics.lever", 1, 42, {"arm_length_m": 1})
	assert_dict(run).contains_keys(["template_code", "template_version", "seed", "parameters", "action_log"])
	assert_bool(run.has("client_outcome")).is_false()


func test_limit_of_500_actions() -> void:
	var l: SimulationActionLog = _log()
	l.start()
	for i: int in 500:
		assert_bool(l.record("release")).is_true()
	assert_bool(l.record("release")).is_false()
	assert_bool(l.full).is_true()
	assert_int(l.to_payload().size()).is_equal(500)


func test_run_keeps_its_key_after_a_network_loss() -> void:
	var h: AuthHarness = AuthHarness.create()
	await h.sign_in()
	var service: LearningService = LearningService.new(h.api, TelemetryService.new(h.api, h.clock), h.clock)
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	var route: String = "POST /learning/activity-attempts/*/simulation-runs"
	var run: Dictionary = {"template_code": "physics.lever", "template_version": 1, "seed": 42, "parameters": {}, "action_log": []}
	h.http.fail(route)
	assert_dict(await service.submit_simulation_run(run)).is_empty()
	assert_bool(service.offline).is_true()
	var first: String = String(h.http.last(route).headers.get("idempotency-key", ""))
	h.http.reply(route, 200, {"run_id": "0f660000-0000-4000-8000-0000000000aa", "verified": true, "goal_met": true,
		"verification_status": "verified", "goals_met": [{"goal_id": "balance", "met": true}], "runs_used": 1, "max_runs": 3})
	var result: Dictionary = await service.submit_simulation_run(run)
	assert_bool(bool(result["goal_met"])).is_true()
	assert_str(String(h.http.last(route).headers.get("idempotency-key", ""))).is_equal(first)
	h.http.reply(route, 422, LearningFixtures.problem(422, "SIMULATION_LOG_INVALID"))
	assert_dict(await service.submit_simulation_run({"template_code": "x", "action_log": [1]})).is_empty()
	assert_str(service.last_error_code).is_equal(LearningService.CODE_LOG_INVALID)
	assert_str(String(h.http.last(route).headers.get("idempotency-key", ""))).is_not_equal(first)
	h.dispose()
