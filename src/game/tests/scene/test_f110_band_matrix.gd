extends GdUnitTestSuite
## F110-AC-28 (owner "ma trận test khối 1–9", F098-AC-09, F125; STUDENT_GAME_UX §2, §4.3, DESIGN_SYSTEM §5.2, §9):
## every F110 screen — Bạn bè (list + profile, empty, Lời mời, Tìm bạn + friend code, Đã chặn, the block confirmation),
## the boss card, the boss arena (party lobby, a question with the contributions, right / wrong feedback, a retry, a
## reconnecting member, solo, the leave confirmation, the results) and the `interdependent` step (+ the leader's role
## choice) — is built for EVERY grade band at 1280×720, 1024×600 and the compact phones 844×390 / 740×360 (logical
## 1168×540 / 1110×540, texts ≥ 12 CSS px, touch targets ≥ 40 CSS px) and BandLayoutProbe finds nothing too small, cut,
## outside the view or overlapping; the result's button row never covers the outcome. FakeHttp + contract fixtures.

var s: F110SceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	F110Matrix.seen.clear()
	s = F110SceneHarness.new()
	await s.start("g1_3", false)


func after_test() -> void:
	for line: String in F110Matrix.seen:
		print("f110-matrix ", line)
	s.dispose()
	AppState.set_compact(false)


func _assert_views(views: PackedStringArray) -> void:
	var failures: PackedStringArray = []
	for view: String in views:
		failures.append_array(await F110Matrix.run(self, s, view))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


func test_friends_screens_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["friends", "friends_empty", "friends_requests", "friends_find", "friends_blocked",
		"friends_block_confirm"]))


func test_boss_screens_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["boss_info", "boss_lobby", "boss_question", "boss_correct", "boss_wrong",
		"boss_retry", "boss_reconnect", "boss_solo", "boss_leave"]))


func test_boss_results_fit_every_band_and_the_buttons_never_cover_the_outcome() -> void:
	await _assert_views(PackedStringArray(["boss_result_defeated", "boss_result_time_up"]))


func test_coop_step_fits_every_band() -> void:
	await _assert_views(PackedStringArray(["coop_step", "coop_assign"]))
