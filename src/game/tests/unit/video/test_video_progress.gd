extends GdUnitTestSuite
## F065-AC-17, F065-AC-19 (TC-VID-002; T026): watched ranges merge (≤ 64) and lock the seek at the frontier under
## watched_only/backward_only until completed; heartbeats every `heartbeat_seconds` and on pause, with an increasing
## `client_seq` (unix ms) and the cumulative ranges, so a lost heartbeat loses nothing; the service posts them unkeyed.


func test_ranges_merge_and_bound() -> void:
	var r: WatchedRanges = WatchedRanges.new()
	r.duration_ms = 100000
	r.add(0, 5000)
	r.add(4000, 9000)
	r.add(20000, 30000)
	r.add(95000, 200000)
	assert_int(r.ranges.size()).is_equal(3)
	assert_that(r.ranges[0]).is_equal(Vector2i(0, 9000))
	assert_int(r.frontier()).is_equal(100000)
	for i: int in 100:
		r.add(i * 1000 + 300, i * 1000 + 600)
	assert_int(r.ranges.size()).is_less_equal(WatchedRanges.MAX_RANGES)
	r.load_from([{"start_ms": 0, "end_ms": 40000}])
	assert_int(r.watched_ms()).is_equal(40000)
	assert_array(r.to_payload()).is_equal([{"start_ms": 0, "end_ms": 40000}])


func test_seek_lock() -> void:
	var r: WatchedRanges = WatchedRanges.new()
	r.add(0, 40000)
	assert_int(r.clamp_seek(90000, WatchedRanges.SEEK_WATCHED_ONLY, false)).is_equal(40000)
	assert_int(r.clamp_seek(10000, WatchedRanges.SEEK_WATCHED_ONLY, false)).is_equal(10000)
	assert_int(r.clamp_seek(90000, WatchedRanges.SEEK_BACKWARD_ONLY, false)).is_equal(40000)
	assert_int(r.clamp_seek(90000, WatchedRanges.SEEK_FREE, false)).is_equal(90000)
	assert_int(r.clamp_seek(90000, WatchedRanges.SEEK_WATCHED_ONLY, true)).is_equal(90000)
	assert_bool(r.is_locked_beyond(90000, WatchedRanges.SEEK_WATCHED_ONLY, false)).is_true()


func test_heartbeats_are_periodic_and_cumulative() -> void:
	var rep: VideoProgressReporter = VideoProgressReporter.new()
	var now: Array[int] = [1_000_000]
	rep.now_ms = func() -> int: return now[0]
	rep.heartbeat_sec = 10.0
	rep.started(0)
	assert_dict(rep.advanced(5000, 5.0)).is_empty()
	var first: Dictionary = rep.advanced(10000, 5.0)
	assert_int(int(first["client_seq"])).is_equal(1_000_000)
	assert_int(int(first["position_ms"])).is_equal(10000)
	assert_array(first["watched_ranges"] as Array).is_equal([{"start_ms": 0, "end_ms": 10000}])
	# Same clock: the next sequence still increases.
	var paused: Dictionary = rep.paused(12000)
	assert_int(int(paused["client_seq"])).is_equal(1_000_001)
	assert_array(paused["watched_ranges"] as Array).is_equal([{"start_ms": 0, "end_ms": 12000}])
	rep.started(30000)
	now[0] = 2_000_000
	var after: Dictionary = rep.advanced(41000, 11.0)
	assert_int(int(after["client_seq"])).is_equal(2_000_000)
	assert_array(after["watched_ranges"] as Array).is_equal([{"start_ms": 0, "end_ms": 12000}, {"start_ms": 30000, "end_ms": 41000}])
	assert_float(float(after["playback_rate"])).is_equal(1.0)


func test_service_posts_heartbeats_without_key_and_reports_refusals() -> void:
	var h: AuthHarness = AuthHarness.create()
	await h.sign_in()
	var service: LearningService = LearningService.new(h.api, null, h.clock)
	h.http.reply(LearningFixtures.ROUTE_START, 201, VideoFixtures.attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})
	var route: String = "POST /learning/activity-attempts/*/progress"
	h.http.reply(route, 200, {"position_ms": 10000, "watched_ratio": 0.1, "completed": false,
		"watched_ranges": [{"start_ms": 0, "end_ms": 10000}], "pending_cue_ids": ["q1"]})
	var st: Dictionary = await service.report_progress({"client_seq": 5, "position_ms": 10000,
		"watched_ranges": [{"start_ms": 0, "end_ms": 10000}], "playback_rate": 1.0})
	assert_float(float(st["watched_ratio"])).is_equal(0.1)
	var rec: FakeHttp.Recorded = h.http.last(route)
	assert_bool(rec.headers.has("idempotency-key")).is_false()
	assert_int(int(rec.body["client_seq"])).is_equal(5)
	h.http.reply(route, 422, LearningFixtures.problem(422, "VIDEO_SEEK_NOT_ALLOWED"))
	assert_dict(await service.report_progress({"client_seq": 6, "position_ms": 99000, "watched_ranges": []})).is_empty()
	assert_str(service.last_error_code).is_equal(LearningService.CODE_SEEK_NOT_ALLOWED)
	assert_object(service.attempt).is_not_null()
	# Embedded question and branch choice bodies (F065-AC-18, AC-24).
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await service.respond(1, {"option_id": "o_a"}, 1200, {"kind": "cue_answer", "cue_id": "q1"})
	var cue: Dictionary = h.http.last(LearningFixtures.ROUTE_RESPOND).body
	assert_str(String(cue["kind"])).is_equal("cue_answer")
	assert_str(String(cue["cue_id"])).is_equal("q1")
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, {"accepted": true, "next_segment_id": "water", "hints_used": 0})
	var res: ResponseResultModel = await service.choose_branch("bp1", "swim")
	assert_str(res.next_segment_id).is_equal("water")
	var branch: FakeHttp.Recorded = h.http.last(LearningFixtures.ROUTE_RESPOND)
	assert_dict(branch.body).is_equal({"kind": "branch_choice", "branch_point_id": "bp1", "option_id": "swim"})
	assert_bool(Uuid.is_v7(String(branch.headers["idempotency-key"]))).is_true()
	h.dispose()
