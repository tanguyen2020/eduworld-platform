extends GdUnitTestSuite
## F111-AC-28 (owner "ma trận test khối 1–9", F098-AC-09, F125; STUDENT_GAME_UX §2, §4.3, DESIGN_SYSTEM §5.2, §9): every F111
## screen — "Tham gia bạn" on a friend's profile, "Vừa chơi cùng", the club tab, a club page (members, goals, requests,
## a locked club), the create form, the invite picker, the Event center (Hôm nay, all done, Tuần này, Sự kiện, an event's
## detail, an ended event, Mùa, no season) — is built for EVERY grade band at 1280×720, 1024×600 and the compact phones
## 844×390 / 740×360 (logical 1168×540 / 1110×540, texts ≥ 12 CSS px, touch targets ≥ 40 CSS px) and BandLayoutProbe
## finds nothing too small, cut, outside the view or overlapping; the main action of a view is on screen at rest.

var s: F111SceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	F111Matrix.seen.clear()
	s = F111SceneHarness.new()
	await s.start("g1_3", false)


func after_test() -> void:
	for line: String in F111Matrix.seen:
		print("f111-matrix ", line)
	s.dispose()
	AppState.set_compact(false)


func _assert_views(views: PackedStringArray) -> void:
	var failures: PackedStringArray = []
	for view: String in views:
		failures.append_array(await F111Matrix.run(self, s, view))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


func test_friends_join_and_suggestions_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["friends_join", "friends_suggest"]))


func test_club_screens_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["clubs", "club_page", "club_goals", "club_requests", "club_readonly"]))


func test_club_dialogs_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["club_create", "club_invite"]))


func test_event_center_fits_every_band() -> void:
	await _assert_views(PackedStringArray(["events_today", "events_done", "events_week", "events_list"]))


func test_event_detail_and_season_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["event_detail", "event_ended", "events_season", "events_no_season"]))
