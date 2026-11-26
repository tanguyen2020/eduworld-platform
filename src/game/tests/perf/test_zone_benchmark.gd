extends GdUnitTestSuite
## F093-AC-03, F094-AC-04 (NFR-COMP-001, TC-PERF-018): every zone of art passes 3 and 4 stays inside `scene_budget_zone`
## (PERFORMANCE_BUDGET §7 row "Vùng môn học / khám phá": ≤ 120k triangles, ≤ 90 draw calls, ≤ 50 dynamic bodies,
## Mobile/Web column) with the CI tolerance `perf_budget_tolerance_ci`, measured headless by the static proxy of the
## benchmark (tests/perf/zone_benchmark.gd); the per-frame primitives / draw calls on the Mobile and Compatibility
## renderers are the renderer run of ZoneBenchmark.tscn (quickstart §5). Loading each zone stays within `load_time`.

const Bench := preload("res://tests/perf/zone_benchmark.gd")
const HubBench := preload("res://tests/perf/main_hub_benchmark.gd")
const ZONES: PackedStringArray = ["learning_zone.math", "learning_zone.vietnamese", "learning_zone.english",
	"learning_zone.science", "learning_zone.history", "practice_zone", "social_zone", "quest_zone.discovery_ocean",
	"challenge_mountain", "exam_zone", "arena"]
## CONFIG_DEFAULTS `load_time`: ≤ 20 s.
const LOAD_TIME_MS: int = 20000


func test_subject_zones_stay_in_the_zone_budget() -> void:
	for area: String in ZONES:
		var started: int = Time.get_ticks_msec()
		var zone: ZoneRoot = (load(Bench.zone_path(area)) as PackedScene).instantiate() as ZoneRoot
		zone.interactive = false  # no player / services; the renderer run of ZoneBenchmark.tscn includes the player
		add_child(zone)
		await get_tree().process_frame
		var load_ms: int = Time.get_ticks_msec() - started
		var stats: Dictionary = HubBench.scene_stats(zone)
		prints(area, "static budget:", JSON.stringify(stats), "load_ms:", load_ms)
		assert_int(int(stats["triangles"])).override_failure_message(area).is_less_equal(int(Bench.BUDGET_TRIANGLES * Bench.TOLERANCE))
		assert_int(int(stats["draw_calls"])).override_failure_message(area).is_less_equal(int(Bench.BUDGET_DRAW_CALLS * Bench.TOLERANCE))
		assert_int(int(stats["dynamic_bodies"])).override_failure_message(area).is_less_equal(Bench.BUDGET_DYNAMIC_BODIES)
		assert_int(int(stats["triangles"])).is_greater(12000)  # real art, not the F071 greybox (a dozen triangles)
		assert_int(load_ms).is_less_equal(LOAD_TIME_MS)
		zone.free()


func test_benchmark_scene_reads_the_area_argument() -> void:
	assert_str(Bench.zone_path("learning_zone.history")).is_equal("res://scenes/world/zones/learning_zone_history/Zone.tscn")
	var runner: GdUnitSceneRunner = scene_runner("res://tests/perf/ZoneBenchmark.tscn")
	await runner.simulate_frames(1)
	var bench: Node = runner.scene()
	assert_str(String(bench.get("area"))).is_equal("learning_zone.math")
	assert_object(bench.get("zone")).is_not_null()
