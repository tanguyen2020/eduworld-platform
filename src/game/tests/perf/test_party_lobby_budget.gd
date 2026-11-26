extends GdUnitTestSuite
## T029 (F102 SC-002/SC-003 CI guard; STUDENT_GAME_UX_SOCIAL §5 "Hiệu năng", AC-UX-104; PERFORMANCE_BUDGET `ui_cpu_budget`):
## the Party screen with 4 members draws no 3D at all (2D portraits only), keeps the visible Controls inside the screen
## UI budget, and a friend's Ready change redraws only that card within the UI update budget. The FPS of a 4-member
## lobby on the reference mid-range Android device (SC-003, ≥ 30 FPS) is a lab measurement, recorded in the quickstart
## when it is run.

const LOBBY: String = "res://scenes/social/PartyLobby.tscn"
const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")

var s: PartySceneHarness = null


func before_test() -> void:
	s = PartySceneHarness.new()
	await s.start()
	var four: Dictionary = PartyFixtures.party(s.self_id)
	var extra: Dictionary = ((four["members"] as Array)[1] as Dictionary).duplicate(true)
	extra["student_id"] = PartyFixtures.NEWCOMER
	extra["nickname"] = "SaoBang12"
	(four["members"] as Array).append(extra)
	s.h.http.clear(PartyFixtures.ROUTE_MY_PARTY).reply(PartyFixtures.ROUTE_MY_PARTY, 200, four)


func after_test() -> void:
	s.dispose()


func _visible_controls(root: Node) -> int:
	var n: int = 0
	for c: Node in root.find_children("*", "Control", true, false):
		if (c as Control).is_visible_in_tree():
			n += 1
	return n


func test_four_member_lobby_is_2d_and_within_the_ui_budget() -> void:
	var lobby: PartyLobby = auto_free((load(LOBBY) as PackedScene).instantiate()) as PartyLobby
	add_child(lobby)
	lobby.service = s.party
	lobby.open({})
	for i: int in 8:
		await get_tree().process_frame
	assert_int(lobby.card_count()).is_equal(4)
	assert_array(lobby.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(lobby.find_children("*", "SubViewport", true, false)).is_empty()
	var controls: int = _visible_controls(lobby)
	var started: int = Time.get_ticks_usec()
	s.push("party.member.ready_changed", PartyFixtures.ws("member.ready_changed.ready"))
	var update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	prints("party_lobby budget:", JSON.stringify({"controls": controls, "ready_update_ms": update_ms}))
	assert_int(controls).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
	assert_float(update_ms).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)
