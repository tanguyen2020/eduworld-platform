extends GdUnitTestSuite
## T014 (F110-AC-11, TC-PERF-018 CI guard; STUDENT_GAME_UX_CORE S06 §13, PERFORMANCE_BUDGET §7): the party boss screen
## (4 members) suspends the 3D world while open (resumed on close), draws no 3D node / SubViewport of its own (a fixed
## 2D stage: no camera to move, no joystick — the HUD layer is covered), the question panel is a Control, the light-ray
## particles stay ≤ 200 (High), fewer at Medium and none at Low, the visible Controls stay inside the screen UI budget,
## a server message is applied within the UI update budget and nothing is polled. The FPS on the reference mid-range
## Android device (SC-004) is a lab measurement recorded in the quickstart when it is run.

const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")

var s: F110SceneHarness = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	await s.with_party()


func after_test() -> void:
	QualityManager.apply_preset(QualityPolicy.MEDIUM, false)
	s.dispose()


func _visible_controls(root: Node) -> int:
	var n: int = 0
	for c: Node in root.find_children("*", "Control", true, false):
		if (c as Control).is_visible_in_tree():
			n += 1
	return n


func test_boss_screen_is_2d_light_and_event_driven() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200,
			F110Fixtures.encounter(s.self_id, "party", "phase_active", 1))
	var arena: BossArena = (load("res://scenes/challenge/boss/BossArena.tscn") as PackedScene).instantiate() as BossArena
	arena.services = s.services
	add_child(arena)
	await arena.open({"encounter_id": F110Fixtures.ENCOUNTER_ID, "mode": "party"})
	var members: Array = [F110Fixtures.member(s.self_id, "MeoXanh42"), F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK),
		F110Fixtures.member(F110Fixtures.THIRD, F110Fixtures.THIRD_NICK), F110Fixtures.member(F110Fixtures.STRANGER, "BuomXanh09")]
	s.push("boss.phase.started", F110Fixtures.phase_started(0, false, s.services.boss.now_ms() + 90_000))
	s.push("boss.encounter.updated", F110Fixtures.updated(2, 80.0, members))
	s.push("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1"]))
	for i: int in 30:
		await get_tree().process_frame
	assert_bool(QualityManager.suspend_reasons.has(BossArena.SUSPEND_REASON)).is_true()
	assert_array(arena.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(arena.find_children("*", "SubViewport", true, false)).is_empty()
	assert_array(arena.find_children("*", "Camera2D", true, false)).is_empty()
	assert_bool(arena.question_panel() is Control).is_true()
	for id: String in [s.self_id, F110Fixtures.MATE, F110Fixtures.THIRD, F110Fixtures.STRANGER]:
		assert_object(arena.members().chip(id)).is_not_null()
	var controls: int = _visible_controls(arena)
	var requests: int = s.h.http.requests.size()
	for i: int in 120:
		await get_tree().process_frame
	assert_int(s.h.http.requests.size()).override_failure_message("REST polled during a phase").is_equal(requests)
	var started: int = Time.get_ticks_usec()
	s.push("boss.encounter.updated", F110Fixtures.updated(3, 60.0, members))
	var update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	prints("boss_arena budget:", JSON.stringify({"controls": controls, "update_ms": update_ms}))
	assert_int(controls).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
	assert_float(update_ms).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)
	# Particles by preset: ≤ 200, none at Low.
	for preset: String in [QualityPolicy.LOW, QualityPolicy.MEDIUM, QualityPolicy.HIGH]:
		QualityManager.apply_preset(preset, false)
		var rays: CPUParticles2D = arena.figure().rays
		assert_int(rays.amount).is_less_equal(200)
		assert_bool(rays.visible).is_equal(preset != QualityPolicy.LOW)
	arena.close()
	assert_bool(QualityManager.suspend_reasons.has(BossArena.SUSPEND_REASON)).is_false()
	arena.free()
