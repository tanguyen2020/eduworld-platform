extends GdUnitTestSuite
## F062-AC-19 (NFR-COMP-001, ADR-0057, TC-PERF-018; T025): the Main Hub greybox stays inside `scene_budget_hub`
## (Mobile/Web column, the stricter one) with the CI tolerance `perf_budget_tolerance_ci`, and the HUD inside
## `ui_cpu_budget` (≤ 150 Controls, ≤ 3 ms per update on mobile). F062-AC-20 (TC-PERF-020): loading the Hub scenes is
## measured here as a CI guard (≤ `load_time` 20 s); the cold-start → Hub measurement on DEV-AND-MID1 is a lab run
## (MainHubBenchmark.tscn with a renderer).

const BENCHMARK: String = "res://tests/perf/MainHubBenchmark.tscn"
const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")
## CONFIG_DEFAULTS `load_time`: cold start → Hub ≤ 20 s.
const LOAD_TIME_MS: int = 20000


func test_hub_scene_is_within_the_budget() -> void:
	var runner: GdUnitSceneRunner = scene_runner(BENCHMARK)
	await runner.simulate_frames(2)
	var bench: Node = runner.scene()
	var stats: Dictionary = BenchmarkScript.scene_stats(bench.get("zone") as Node)
	prints("main_hub static budget:", JSON.stringify(stats))
	assert_int(int(stats["triangles"])).is_less_equal(int(BenchmarkScript.BUDGET_TRIANGLES * BenchmarkScript.TOLERANCE))
	assert_int(int(stats["draw_calls"])).is_less_equal(int(BenchmarkScript.BUDGET_DRAW_CALLS * BenchmarkScript.TOLERANCE))
	assert_int(int(stats["dynamic_bodies"])).is_less_equal(BenchmarkScript.BUDGET_DYNAMIC_BODIES)
	assert_int(int(stats["triangles"])).is_greater(0)


func test_hud_is_within_the_ui_budget() -> void:
	var runner: GdUnitSceneRunner = scene_runner(BENCHMARK)
	await runner.simulate_frames(2)
	var bench: Node = runner.scene()
	var hud: Dictionary = BenchmarkScript.hud_stats(bench.get("hud") as CanvasLayer)
	prints("main_hub HUD budget:", JSON.stringify(hud))
	assert_int(int(hud["controls"])).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
	assert_float(float(hud["update_ms"])).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)


func test_hub_loads_within_the_load_time_budget() -> void:
	var runner: GdUnitSceneRunner = scene_runner(BENCHMARK)
	await runner.simulate_frames(1)
	var load_ms: int = int(runner.scene().get("load_ms"))
	prints("main_hub load_ms:", load_ms)
	assert_int(load_ms).is_less_equal(LOAD_TIME_MS)
