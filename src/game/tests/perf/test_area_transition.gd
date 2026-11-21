extends GdUnitTestSuite
## F071-AC-12 CI guard (T047, TC-PERF-020): loading and placing every greybox zone (ZoneLoader.instantiate + add to the
## tree) stays inside `area_transition_max` (SceneRouter.AREA_TRANSITION_MAX_SEC) headless. The measurement on a
## Mid-tier lab device (DEV-PC-MID / DEV-AND-MID1, ≥ 95 % of gate transitions) is a lab run with the real art (OQ-16) and
## is reported with the quickstart evidence, not here.

const ZoneRules := preload("res://tools/lib/zone_rules.gd")


func test_every_zone_loads_within_the_transition_budget() -> void:
	var budget_ms: int = int(SceneRouter.AREA_TRANSITION_MAX_SEC * 1000.0)
	for path: String in ZoneRules.zone_paths():
		var started: int = Time.get_ticks_msec()
		var zone: ZoneRoot = ZoneLoader.instantiate(path)
		zone.interactive = false
		add_child(zone)
		await get_tree().process_frame
		var elapsed: int = Time.get_ticks_msec() - started
		prints("area transition", zone.area_id, elapsed, "ms")
		assert_int(elapsed).override_failure_message(path).is_less_equal(budget_ms)
		assert_bool(ZoneLoader.check_budget(zone.area_id, elapsed)).is_true()
		zone.queue_free()
		await get_tree().process_frame
