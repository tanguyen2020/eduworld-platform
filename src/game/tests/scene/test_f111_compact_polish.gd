extends GdUnitTestSuite
## F111 compact polish (F125 compact mode, STUDENT_GAME_UX §2, DESIGN_SYSTEM §5.2): on a compact phone the F111 sub-pages
## give their content the screen — the club page header is one line ("Quay lại" and the actions as icon buttons that keep
## their words as tooltip and accessible name, the club type hidden, the name on the line of its tags) and the members sit
## two to a row; a club's page or an event's detail hides the window's tab row (it comes back with the lists); the season
## strip comes before its note; "Vừa chơi cùng" tiles are rows with the tap hint on the title line. Outside compact mode
## every screen keeps its full layout. (The band matrix checks the visible content heights, test_f111_band_matrix.)

var s: F111SceneHarness = null
var _autostart: bool = true
var _root: Control = null


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g1_3", false)
	_root = auto_free(Control.new()) as Control
	add_child(_root)
	_root.size = Vector2(1110, 540)


func after_test() -> void:
	AppState.set_compact(false)
	s.dispose()


func _build(view: String, band: String = "g1_3") -> Control:
	AppState.set_grade_band(band)
	_root.theme = AppState.theme_service.current_theme
	return await F111Screens.build(view, _root, s.services, s.h.http)


func _assert_named_icon(b: Button, key: String) -> void:
	assert_str(b.text).override_failure_message("%s shows words on compact" % b.name).is_empty()
	assert_str(b.tooltip_text).is_equal(key)
	assert_str(b.accessibility_name).is_equal(TranslationServer.translate(key))
	assert_object(b.icon).is_not_null()


func test_club_page_header_is_one_line_with_named_icon_buttons_on_compact() -> void:
	AppState.set_compact(true)
	var screen: FriendsScreen = await _build("club_page") as FriendsScreen
	var page: ClubPage = screen.club_view().page_view()
	_assert_named_icon(page.find_child("Back", true, false) as Button, "ui.club.back")
	_assert_named_icon(page.action_button(ClubPage.ACT_CHAT), "ui.club.chat")
	_assert_named_icon(page.action_button(ClubPage.ACT_INVITE), "ui.club.invite")
	assert_bool((page.find_child("ClubType", true, false) as Label).visible).is_false()
	assert_str(page.find_child("ClubName", true, false).get_parent().name).is_equal("Line")
	var grid: GridContainer = page.find_child("MemberGrid", true, false) as GridContainer
	assert_object(grid).is_not_null()
	assert_int(grid.columns).is_equal(ClubPage.COMPACT_MEMBER_COLUMNS)
	assert_bool(screen.tab_row_visible()).is_false()
	# One header line: Back, the name / tags and the actions share a row.
	var back_y: float = (page.find_child("Back", true, false) as Control).global_position.y
	var chat_y: float = page.action_button(ClubPage.ACT_CHAT).global_position.y
	assert_float(absf(back_y - chat_y)).is_less_equal(BandLayoutProbe.TOLERANCE_PX)
	# Back to the lists: the tab row comes back.
	(page.find_child("Back", true, false) as Button).pressed.emit()
	await s.settle(4)
	assert_bool(screen.tab_row_visible()).is_true()
	screen.close()


func test_club_page_keeps_its_full_header_outside_compact_mode() -> void:
	var screen: FriendsScreen = await _build("club_page") as FriendsScreen
	var page: ClubPage = screen.club_view().page_view()
	assert_str((page.find_child("Back", true, false) as Button).text).is_equal("ui.club.back")
	assert_str(page.action_button(ClubPage.ACT_CHAT).text).is_equal("ui.club.chat")
	assert_bool((page.find_child("ClubType", true, false) as Label).visible).is_true()
	assert_object(page.find_child("MemberGrid", true, false)).is_null()
	assert_bool(screen.tab_row_visible()).is_true()
	# The mode switching while the page is open re-lays it out (a phone turned).
	AppState.set_compact(true)
	await s.settle(4)
	assert_str((page.find_child("Back", true, false) as Button).text).is_empty()
	assert_bool(screen.tab_row_visible()).is_false()
	AppState.set_compact(false)
	await s.settle(4)
	assert_str((page.find_child("Back", true, false) as Button).text).is_equal("ui.club.back")
	assert_bool(screen.tab_row_visible()).is_true()
	screen.close()


func test_event_detail_hides_the_tabs_and_puts_points_on_the_title_line_on_compact() -> void:
	AppState.set_compact(true)
	var screen: EventCenterScreen = await _build("event_detail") as EventCenterScreen
	var detail: EventDetailPanel = screen.detail_view()
	_assert_named_icon(detail.back_button(), "ui.events.back")
	assert_bool(screen.tab_row_visible()).is_false()
	assert_str(detail.find_child("Points", true, false).get_parent().name).is_equal("TitleLine")
	detail.back_button().pressed.emit()
	await s.settle(4)
	assert_bool(screen.tab_row_visible()).is_true()
	screen.close()


func test_season_strip_comes_before_its_note_on_compact_only() -> void:
	AppState.set_compact(true)
	var screen: EventCenterScreen = await _build("events_season", "g6_7") as EventCenterScreen
	var season: SeasonTab = screen.season_view()
	var note: Control = season.find_child("Note", true, false) as Control
	assert_int(season.track_view().get_index()).is_less(note.get_index())
	AppState.set_compact(false)
	await s.settle(4)
	assert_int(note.get_index()).is_less(season.track_view().get_index())
	screen.close()


func test_recently_played_tiles_are_rows_with_the_hint_on_the_title_line_on_compact() -> void:
	AppState.set_compact(true)
	var screen: FriendsScreen = await _build("friends_suggest") as FriendsScreen
	var group: SuggestionGroup = screen.find_view().suggestion_group()
	assert_str(group.find_child("TapHint", true, false).get_parent().name).is_equal("TitleRow")
	var tile: Control = group.find_child("Suggestion", true, false) as Control
	assert_object(tile.find_child("NameLine", true, false)).is_not_null()
	assert_str(screen.find_view().find_child("NearbyNone", true, false).get_parent().name).is_equal("NearbyTitle")
	# Every word of a tile is still there (nickname, level, context).
	assert_int(tile.find_children("*", "Label", true, false).size()).is_greater_equal(3)
	screen.close()
