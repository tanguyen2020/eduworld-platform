extends GdUnitTestSuite
## T022 (F109-AC-12, TC-PERF-018 CI guard; STUDENT_GAME_UX_SOCIAL §8 "Hiệu năng", PERFORMANCE_BUDGET §7): the battle
## screen suspends the 3D world while open (resumed on close), draws no 3D node / SubViewport of its own (2D portraits,
## a static vector backdrop), keeps the visible Controls inside the screen UI budget, a server message (scores) is
## applied within the UI update budget, and nothing is polled: no REST call is made while 120 frames pass with a
## question on screen. The FPS on the reference mid-range Android device (SC-005) is a lab measurement recorded in the
## quickstart when it is run.

const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")

var s: CompetitionSceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = CompetitionSceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	s.dispose()


func _visible_controls(root: Node) -> int:
	var n: int = 0
	for c: Node in root.find_children("*", "Control", true, false):
		if (c as Control).is_visible_in_tree():
			n += 1
	return n


func test_battle_screen_is_2d_event_driven_and_within_the_ui_budget() -> void:
	var arena: BattleArena = (load("res://scenes/competition/BattleArena.tscn") as PackedScene).instantiate() as BattleArena
	arena.services = s.services
	add_child(arena)
	arena.open({"match_id": CompetitionFixtures.MATCH_ID, "mode": "battle_1v1"})
	var snap: Dictionary = s.ws("battle.state.snapshot", "question_1v1")
	snap["deadline_at"] = s.services.battle.now_ms() + 15000
	s.push("battle.state.snapshot", snap)
	for i: int in 40:
		await get_tree().process_frame
	assert_bool(QualityManager.suspend_reasons.has(BattleArena.SUSPEND_REASON)).is_true()
	assert_array(arena.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(arena.find_children("*", "SubViewport", true, false)).is_empty()
	var chat_controls: int = _visible_controls(arena.chat_dock())
	var controls: int = _visible_controls(arena) - chat_controls
	var requests: int = s.h.http.requests.size()
	for i: int in 120:
		await get_tree().process_frame
	assert_int(s.h.http.requests.size()).override_failure_message("REST polled during a question").is_equal(requests)
	# A message during the question (the opponent answered: header redrawn) stays inside the UI update budget.
	var started: int = Time.get_ticks_usec()
	s.push_example("battle.answer.recorded", "opponent")
	var update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	# The reveal also gives the chat its column back (F119 dock expanded once between two questions): reported only.
	started = Time.get_ticks_usec()
	s.push_example("battle.question.revealed", "battle_1v1")
	var reveal_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	prints("battle_arena budget:", JSON.stringify({"controls": controls, "chat_controls": chat_controls,
		"answer_update_ms": update_ms, "reveal_update_ms": reveal_ms}))
	assert_int(controls).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
	assert_float(update_ms).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)
	arena.close()
	assert_bool(QualityManager.suspend_reasons.has(BattleArena.SUSPEND_REASON)).is_false()
	arena.free()
