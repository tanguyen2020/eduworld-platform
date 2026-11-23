extends GdUnitTestSuite
## T042 (NFR-COMP-001, ADR-0057; PERFORMANCE_BUDGET §9 `ui_cpu_budget`): every mini-game scene stays inside the UI
## budget of a mid-range device — at most 150 Controls with an item on screen, a feedback/timer update within 3 ms
## (mobile column, with the CI tolerance of the Main Hub benchmark) and an item shown within one frame of the lowest
## preset (33.3 ms). Headless CI guard; the on-device measurement (DEV-AND-MID1, Low/Medium presets) is a lab run.

const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")
## PERFORMANCE_BUDGET §5: one frame at 30 FPS (Low preset).
const FRAME_BUDGET_MS: float = 33.3
const ITERATIONS: int = 50
const PRESENT_RUNS: int = 5


func _ctx() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	ctx.grade_band = "g1_3"
	return ctx


func test_every_scene_is_within_the_ui_budget() -> void:
	for type: String in MiniGameFixtures.CORE_TYPES + MiniGameFixtures.EXTENDED_TYPES:
		var s: MiniGameSessionModel = MiniGameFixtures.model(type)
		var scene: MiniGameBase = auto_free(MiniGameRegistry.instantiate(type)) as MiniGameBase
		add_child(scene)
		scene.setup(s.config, s, _ctx())
		# Warm-up: the first item of a type loads its F063 question scene from disk (a loading cost, not a frame cost).
		scene.present_item(s.items()[0])
		await await_idle_frame()
		# Best of PRESENT_RUNS presentations: a shared CI/dev machine under load inflates one sample, the best one is the cost
		# of the scene itself.
		var present_ms: float = INF
		for run: int in PRESENT_RUNS:
			var t0: int = Time.get_ticks_usec()
			scene.present_item(s.items()[0])
			present_ms = minf(present_ms, (Time.get_ticks_usec() - t0) / 1000.0)
			await await_idle_frame()
		var controls: int = scene.find_children("*", "Control", true, false).size() + 1
		var result: ResponseResultModel = ResponseResultModel.from_dict(MiniGameFixtures.result(1, true))
		var t0: int = Time.get_ticks_usec()
		for i: int in ITERATIONS:
			scene.update_timer(60000 - i * 1000)
			scene.show_feedback(result)
		var update_ms: float = (Time.get_ticks_usec() - t0) / 1000.0 / ITERATIONS
		prints("minigame budget:", type, JSON.stringify({"controls": controls, "present_ms": present_ms, "update_ms": update_ms}))
		assert_int(controls).override_failure_message(type).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
		assert_float(update_ms).override_failure_message(type).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)
		assert_float(present_ms).override_failure_message(type).is_less_equal(FRAME_BUDGET_MS * BenchmarkScript.TOLERANCE)
		scene.teardown()
