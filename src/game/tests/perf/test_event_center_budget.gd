extends GdUnitTestSuite
## T042 (F111-AC-32, NFR-COMP-001 CI guard; STUDENT_GAME_UX_SOCIAL §2, PERFORMANCE_BUDGET §7): the Event center suspends
## the 3D world behind it while open (resumed on close), draws no 3D node / SubViewport of its own; the season strip of 30
## (and 100) tiers builds only the tiles in view; a club page with 50 members and the Event center stay inside the screen
## UI control budget; a realtime update is applied within the UI update budget and nothing is polled. The FPS on the
## reference mid-range Android device (SC-005, preset Trung bình) is a lab measurement recorded in the quickstart.

const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	s.dispose()


func _visible_controls(root: Node) -> int:
	var n: int = 0
	for c: Node in root.find_children("*", "Control", true, false):
		if (c as Control).is_visible_in_tree():
			n += 1
	return n


func test_event_center_is_2d_virtualised_and_event_driven() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_SEASON).reply(F111Fixtures.ROUTE_SEASON, 200, F111Fixtures.season(100))
	var root: Control = Control.new()
	add_child(root)
	root.size = Vector2(1280, 720)
	var screen: EventCenterScreen = await F111Screens.build("events_season", root, s.services, s.h.http) as EventCenterScreen
	assert_bool(QualityManager.suspend_reasons.has(EventCenterScreen.SUSPEND_REASON)).is_true()
	assert_array(screen.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(screen.find_children("*", "SubViewport", true, false)).is_empty()
	var strip: SeasonTrackView = screen.season_view().track_view()
	assert_int(strip.tier_count()).is_equal(100)
	assert_int(strip.built_items()).is_less(16)
	var controls: int = _visible_controls(screen)
	var requests: int = s.h.http.requests.size()
	for i: int in 120:
		await get_tree().process_frame
	assert_int(s.h.http.requests.size()).override_failure_message("REST polled while open").is_equal(requests)
	var started: int = Time.get_ticks_usec()
	s.push("season.track.updated", F111Fixtures.ws("season.track.updated", {"season_id": F111Fixtures.SEASON_ID, "points": 900,
		"tier": 9, "daily_points_remaining": 10}))
	var update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	# The screen draws the update once, at the end of the frame (coalesced).
	await get_tree().process_frame
	assert_str(" ".join(screen.season_view().texts())).contains(TranslationServer.translate("ui.season.tier").format(
			{"tier": "9", "max": "100"}))
	prints("event_center budget:", JSON.stringify({"controls": controls, "update_ms": update_ms}))
	assert_int(controls).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS)
	assert_float(update_ms).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE)
	screen.close()
	assert_bool(QualityManager.suspend_reasons.has(EventCenterScreen.SUSPEND_REASON)).is_false()
	root.free()


func test_club_page_with_50_members_stays_in_budget() -> void:
	var many: Array = []
	for i: int in 50:
		many.append(F111Fixtures.member("0193b0a0-0000-7000-8000-0000000%05d" % (20000 + i), "Ban%02d" % i,
				"owner" if i == 0 else "member"))
	s.h.http.clear(F111Fixtures.ROUTE_CLUB_MEMBERS).reply(F111Fixtures.ROUTE_CLUB_MEMBERS, 200, F111Fixtures.page(many))
	var root: Control = Control.new()
	add_child(root)
	root.size = Vector2(1280, 720)
	var screen: FriendsScreen = await F111Screens.build("club_page", root, s.services, s.h.http) as FriendsScreen
	assert_int(s.services.clubs.store.members.size()).is_equal(50)
	var controls: int = _visible_controls(screen)
	prints("club_page budget:", JSON.stringify({"controls": controls}))
	assert_int(controls).is_less_equal(BenchmarkScript.BUDGET_HUD_CONTROLS * 3)
	screen.close()
	root.free()
