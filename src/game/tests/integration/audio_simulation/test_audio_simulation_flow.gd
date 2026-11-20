extends GdUnitTestSuite
## T040 (TC-AUD-001, TC-SIM-001): against the backend compose — `hs.an.3a` starts the seeded listening lesson, uses
## its plays (`play_started`) until `AUDIO_PLAY_LIMIT_REACHED`, answers and submits; then starts the seeded lever
## simulation, sends a balanced run (verified, goal met) and submits. Skipped unless EDUWORLD_IT_API_BASE_URL is set
## (LiveBackend) and the seed ids are given (EDUWORLD_IT_AUDIO_ACTIVITY_ID, EDUWORLD_IT_SIMULATION_ACTIVITY_ID).

const SKIP_REASON: String = "needs the backend compose with a seeded audio lesson and lever simulation (EDUWORLD_IT_API_BASE_URL, EDUWORLD_IT_AUDIO_ACTIVITY_ID, EDUWORLD_IT_SIMULATION_ACTIVITY_ID)"
const ENV_AUDIO: String = "EDUWORLD_IT_AUDIO_ACTIVITY_ID"
const ENV_SIM: String = "EDUWORLD_IT_SIMULATION_ACTIVITY_ID"
const ActivityFlow := preload("res://tests/integration/learning/test_activity_flow.gd")


static func _skip() -> bool:
	return LiveBackend.skip() or OS.get_environment(ENV_AUDIO).is_empty() or OS.get_environment(ENV_SIM).is_empty()


func test_student_listens_and_balances_the_lever(do_skip: bool = _skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var service: LearningService = LearningService.new(live.api)
	var audio: AttemptModel = await service.start({"activity_id": OS.get_environment(ENV_AUDIO), "mode": "practice"})
	assert_str(audio.activity_type).is_equal("audio_lesson")
	var reporter: VideoProgressReporter = VideoProgressReporter.new()
	var max_plays: int = int((audio.render.render_payload.get("interaction", {}) as Dictionary).get("max_plays", 1))
	for i: int in max_plays + 1:
		var r: Dictionary = reporter.paused(0)
		r["play_started"] = true
		var st: Dictionary = await service.report_progress(r)
		if i < max_plays:
			assert_int(int(st.get("plays_used", 0))).is_equal(i + 1)
		else:
			assert_str(service.last_error_code).is_equal(LearningService.CODE_PLAY_LIMIT)
	for item: QuestionRenderModel in audio.items:
		assert_object(await service.respond(item.position, ActivityFlow.any_answer(item), 1000)).is_not_null()
	assert_object(await service.submit()).is_not_null()
	var sim: AttemptModel = await service.start({"activity_id": OS.get_environment(ENV_SIM), "mode": "practice"})
	assert_str(sim.activity_type).is_equal("simulation")
	var p: Dictionary = sim.render.render_payload
	var actions: SimulationActionLog = SimulationActionLog.new(PackedStringArray(["place_mass", "remove_mass", "release"]))
	actions.start()
	actions.record("place_mass", {"mass_index": 0, "side": "left", "slot": 2})
	actions.record("place_mass", {"mass_index": 1, "side": "right", "slot": 4})
	actions.record("release")
	var result: Dictionary = await service.submit_simulation_run(actions.run_input(String(p["template_code"]), int(p["template_version"]),
			int(p["seed"]), p["parameters"] as Dictionary))
	assert_bool(bool(result.get("goal_met", false))).is_true()
	for item: QuestionRenderModel in sim.items:
		await service.respond(item.position, ActivityFlow.any_answer(item), 1000, {"kind": "simulation_question"})
	assert_object(await service.submit()).is_not_null()
	await live.auth.logout()
