extends GdUnitTestSuite
## T017 (F111-AC-06, F111-AC-08; STUDENT_GAME_UX_SOCIAL §6, ADR-0095): the "Tìm bạn" tab has exactly three groups —
## "Ở gần đây" (F110), "Vừa chơi cùng" (`listFriendSuggestions`, the context as icon + words) and the friend code (F110);
## no "Bạn cùng lớp" group, no class / grade suggestion, no field to type a name; an empty group shows its empty state
## with the friend-code hint; a failing list hides the group and keeps "Ở gần đây" and the code; lớp 1–3 pick a player by
## tapping the big portrait.

var s: F111SceneHarness = null
var screen: FriendsScreen = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	if screen != null and is_instance_valid(screen) and screen.visible:
		screen.close()
	screen = null
	s.dispose()


func _open() -> void:
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)
	screen.open({"tab": FriendsScreen.TAB_FIND})
	await s.settle(8)


func test_three_groups_and_no_class_group_or_search_field() -> void:
	await _open()
	var tab: FindFriendsTab = screen.find_view()
	assert_array(tab.group_keys()).is_equal(["ui.friends.find.nearby", "ui.friends.suggest.title", "ui.friendcode.mine"])
	var group: SuggestionGroup = tab.suggestion_group()
	assert_bool(group.is_visible_in_tree()).is_true()
	assert_int(group.count()).is_equal(3)
	var card: Control = group.card(F111Fixtures.SUGGESTED_PARTY)
	assert_object(card).is_not_null()
	var words: PackedStringArray = group.card_texts(F111Fixtures.SUGGESTED_PARTY)
	assert_array(Array(words)).contains([TranslationServer.translate("ui.friends.suggest.context.party")])
	assert_array(Array(words)).contains(["SoiNho31"])
	assert_bool(tab.has_search_field()).is_false()
	var all: String = " ".join(screen.shown_texts()).to_lower()
	for word: String in ["lớp", "khối", "classmate", "grade", "bạn cùng lớp"]:
		assert_bool(all.contains(word)).override_failure_message("'%s' on Tìm bạn" % word).is_false()


func test_add_from_the_group_sends_a_friend_request() -> void:
	await _open()
	var group: SuggestionGroup = screen.find_view().suggestion_group()
	group.add_button(F111Fixtures.SUGGESTED_COOP).pressed.emit()
	await s.settle(4)
	assert_str(String(s.h.http.last(F110Fixtures.ROUTE_SEND).body["addressee_id"])).is_equal(F111Fixtures.SUGGESTED_COOP)


func test_empty_group_says_so_with_the_friend_code_hint() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 200, F111Fixtures.page([]))
	await _open()
	var group: SuggestionGroup = screen.find_view().suggestion_group()
	assert_bool(group.is_visible_in_tree()).is_true()
	assert_bool(group.empty_visible()).is_true()
	assert_str(group.empty_text()).is_equal(TranslationServer.translate("ui.friends.suggest.empty"))


func test_a_failing_list_hides_only_its_group() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 503, AuthHarness.problem(503, "UNAVAILABLE"))
	await _open()
	var tab: FindFriendsTab = screen.find_view()
	assert_bool(tab.suggestion_group().is_visible_in_tree()).is_false()
	assert_bool(tab.code_widget.is_visible_in_tree()).is_true()
	assert_array(tab.group_keys()).contains(["ui.friends.find.nearby"])


func test_g1_3_picks_by_tapping_the_big_portrait() -> void:
	AppState.set_grade_band("g1_3")
	await _open()
	var group: SuggestionGroup = screen.find_view().suggestion_group()
	var portrait: Button = group.portrait_button(F111Fixtures.SUGGESTED_MATCH)
	assert_object(portrait).is_not_null()
	assert_float(portrait.get_combined_minimum_size().y).is_greater_equal(UiBuild.target() * 1.5 - 0.5)
	portrait.pressed.emit()
	await s.settle(4)
	assert_str(String(s.h.http.last(F110Fixtures.ROUTE_SEND).body["addressee_id"])).is_equal(F111Fixtures.SUGGESTED_MATCH)
