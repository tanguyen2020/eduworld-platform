extends GdUnitTestSuite
## T026 (F111-AC-16, F111-AC-18; ADR-0049, FR-GAM-001, STUDENT_GAME_UX_SOCIAL §9): the season strip is virtualised — only
## the tiers in view (plus a small margin) exist as nodes, scrolling re-binds them; there is no premium track, no price and
## no "buy a tier" anywhere; the tab says the unclaimed rewards are sent at the end of the season; lớp 1–3 see only the
## "Hôm nay" and "Sự kiện" tabs, the other bands all four.

const PAID_WORDS: PackedStringArray = ["giá", "mua", "premium", "trả phí", "price", "buy", "vip", "đ ", "vnd"]

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	s.dispose()


func test_the_strip_builds_only_the_tiers_in_view() -> void:
	await s.services.season.load_track()
	var tab: SeasonTab = auto_free(SeasonTab.new()) as SeasonTab
	add_child(tab)
	tab.size = Vector2(1100, 600)
	tab.show_track(s.services.season.store, "g6_7")
	await s.settle(3)
	var strip: SeasonTrackView = tab.track_view()
	assert_int(strip.tier_count()).is_equal(30)
	assert_int(strip.built_items()).is_less(16)
	assert_object(strip.item(1)).is_not_null()
	assert_object(strip.item(30)).is_null()
	strip.scroll_to(30)
	await s.settle(3)
	assert_object(strip.item(30)).is_not_null()
	assert_int(strip.built_items()).is_less(16)
	var all: String = " ".join(tab.texts()).to_lower()
	for w: String in PAID_WORDS:
		assert_bool(all.contains(w)).override_failure_message("'%s' on the season tab" % w).is_false()
	assert_bool(all.contains(TranslationServer.translate("ui.season.auto_grant").to_lower())).is_true()


func test_no_season_says_a_new_one_comes() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_SEASON).reply(F111Fixtures.ROUTE_SEASON, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.season.load_track()
	var tab: SeasonTab = auto_free(SeasonTab.new()) as SeasonTab
	add_child(tab)
	tab.show_track(s.services.season.store, "g6_7")
	assert_array(Array(tab.texts())).contains([TranslationServer.translate("ui.season.no_season")])
	assert_object(tab.track_view()).is_null()


func test_g1_3_has_two_tabs_other_bands_four() -> void:
	for band: String in ["g1_3", "g4_5", "g8_9"]:
		AppState.set_grade_band(band)
		var screen: EventCenterScreen = auto_free((load("res://scenes/liveops/EventCenterScreen.tscn") as PackedScene).instantiate()) as EventCenterScreen
		screen.services = s.services
		add_child(screen)
		await screen.open({})
		var want: Array = ["today", "events"] if band == "g1_3" else ["today", "week", "events", "season"]
		assert_array(Array(screen.allowed_tabs())).override_failure_message(band).is_equal(want)
		for id: String in ["today", "week", "events", "season"]:
			assert_bool(screen.tab_button(id).visible).override_failure_message("%s %s" % [band, id]).is_equal(id in want)
		assert_str(screen.tab).is_equal("today")
		screen.close()
