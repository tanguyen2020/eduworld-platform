extends GdUnitTestSuite
## T027, T029 (F112-AC-25, AC-27, AC-28; TC-UI-003, TC-I18N-006; F098-AC-09, F125): every F112 screen — the HUD chat on
## each kind of tab (Khu vực, Cả trường, a club, Bạn bè, a DM open and frozen), the search with hits and empty, "Quản lý
## club" (Sửa thông tin, Phó nhóm, Mục tiêu tuần, Sự kiện club) — is built for EVERY grade band at 1280×720, 1024×600 and
## the compact phones 844×390 / 740×360 (texts ≥ 12 CSS px, touch targets ≥ 40 CSS px) and BandLayoutProbe finds nothing
## too small, cut, outside the view or overlapping; every open chat tab keeps "≥ 3 tin" (ChatSpaceProbe); on a phone the
## windows are full-height overlays and the action row of a club form never scrolls away. The same in English and in the
## pseudo-locale (+35–40 %), which must fit without cuts.

var s: F112SceneHarness = null
var _autostart: bool = true
var _locale: String = ""


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	F112Matrix.seen.clear()
	s = F112SceneHarness.new()
	await s.start("g1_3", false)
	_locale = String(Settings.locale)


func after_test() -> void:
	for line: String in F112Matrix.seen:
		print("f112-matrix ", line)
	Settings.set_locale(_locale, false)
	s.dispose()
	AppState.set_compact(false)


func _assert_views(views: PackedStringArray) -> void:
	var failures: PackedStringArray = []
	for view: String in views:
		failures.append_array(await F112Matrix.run(self, s, view))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


func test_hub_tabs_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["hub_area", "hub_world", "hub_club", "hub_friends"]))


func test_dm_and_search_fit_every_band() -> void:
	await _assert_views(PackedStringArray(["hub_dm", "hub_dm_frozen", "search_hits", "search_empty"]))


func test_club_manage_fits_every_band() -> void:
	await _assert_views(PackedStringArray(["manage_edit", "manage_officers", "manage_goal", "manage_event"]))


func test_english_and_pseudo_locale_fit() -> void:
	var failures: PackedStringArray = []
	for locale: String in ["en-US", L10n.pseudo_locale()]:
		Settings.set_locale(locale, false)
		for view: String in ["hub_club", "hub_friends", "search_hits", "manage_event", "manage_officers"]:
			failures.append_array(await F112Matrix.run(self, s, view, {"1280x720": Vector2i(1280, 720),
				"740x360": Vector2i(1110, 540)}, PackedStringArray(["g1_3", "g8_9"])))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


func test_every_f112_key_has_vi_and_en() -> void:
	var missing: PackedStringArray = []
	var prefixes: PackedStringArray = ["ui.chat.hub.", "ui.chat.dm.", "ui.chat.search.", "ui.club.manage."]
	var vi: String = FileAccess.get_file_as_string("res://locale/vi.po")
	var en: String = FileAccess.get_file_as_string("res://locale/en.po")
	var found: int = 0
	for line: String in vi.split("\n"):
		if not line.begins_with("msgid \""):
			continue
		var key: String = line.trim_prefix("msgid \"").trim_suffix("\"")
		for p: String in prefixes:
			if key.begins_with(p):
				found += 1
				if not en.contains("msgid \"%s\"" % key):
					missing.append(key)
	assert_int(found).is_greater(40)
	assert_array(missing).is_empty()
