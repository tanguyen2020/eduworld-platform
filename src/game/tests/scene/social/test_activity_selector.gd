extends GdUnitTestSuite
## T015 (F102-AC-10; TC-PTY-005): the leader's activity list comes from `listPartyEligibleActivities`; an activity every
## member is eligible for is chosen and sent with the state_version on screen; one that only some members are eligible
## for shows a lock and "{nickname} chưa đủ điều kiện: {reason}" (the requirement of the activity, never the friend's
## grade or school) and is not sent; grade bands g1_3 / g4_5 get at most 4 big picture cards, g6_7 / g8_9 a dropdown.

const SELECTOR: String = "res://scenes/social/ActivitySelector.tscn"

var s: PartySceneHarness = null


func before_test() -> void:
	s = PartySceneHarness.new()
	await s.start()
	await s.party.load_my_party()


func after_test() -> void:
	s.dispose()


func _frames(n: int = 6) -> void:
	for i: int in n:
		await get_tree().process_frame


func _selector(band: String) -> ActivitySelector:
	var sel: ActivitySelector = auto_free((load(SELECTOR) as PackedScene).instantiate()) as ActivitySelector
	add_child(sel)
	sel.open(s.party, band)
	await _frames()
	return sel


func test_young_bands_choose_from_big_picture_cards() -> void:
	for band: String in ["g1_3", "g4_5"]:
		var sel: ActivitySelector = await _selector(band)
		assert_bool(sel.uses_cards()).is_true()
		assert_int(sel.option_count()).is_between(3, ActivitySelector.MAX_CARDS)
		var quest: Button = sel.option("math_g3_multiplication")
		assert_str(String(quest.theme_type_variation)).is_equal("PictureTile")
		assert_object(quest.find_child("Picture", true, false)).is_not_null()
		assert_str(quest.text).contains("Phép nhân ở Vương quốc Toán")
		assert_bool(sel.dropdown_button().visible).is_false()
		sel.close()


func test_an_activity_not_everyone_can_join_warns_and_is_not_sent() -> void:
	var sel: ActivitySelector = await _selector("g1_3")
	var warnings: PackedStringArray = sel.warning_texts()
	assert_int(warnings.size()).is_equal(1)
	assert_str(warnings[0]).contains("GauNau15")
	assert_str(warnings[0]).contains(tr("ui.party.reason.grade_not_allowed").format({"grades": "4"}))
	sel.option("math_fraction_dragon").pressed.emit()
	await _frames(4)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_ACTIVITY)).is_equal(0)
	assert_bool(sel.visible).is_true()


func test_choosing_an_eligible_card_sends_it_with_the_version_and_closes() -> void:
	var sel: ActivitySelector = await _selector("g1_3")
	var chosen: Dictionary = PartyFixtures.party(s.self_id)
	chosen["current_activity"] = {"activity_type": "explore", "activity_ref": "learning_zone.math"}
	chosen["state_version"] = 8
	s.h.http.reply(PartyFixtures.ROUTE_ACTIVITY, 200, chosen)
	sel.option("learning_zone.math").pressed.emit()
	await _frames(6)
	var body: Dictionary = s.h.http.last(PartyFixtures.ROUTE_ACTIVITY).body
	assert_str(String(body["activity_type"])).is_equal("explore")
	assert_str(String(body["activity_ref"])).is_equal("learning_zone.math")
	assert_int(int(body["expected_version"])).is_equal(7)
	assert_bool(sel.visible).is_false()
	assert_str(s.party.store.party.activity_id).is_equal("learning_zone.math")
	# The title of the eligible list is shown for the chosen activity.
	assert_str(L10n.pick(s.party.store.party.activity_title)).is_equal("Cùng khám phá Vương quốc Toán")


func test_older_bands_use_a_dropdown_of_every_activity() -> void:
	for band: String in ["g6_7", "g8_9"]:
		var sel: ActivitySelector = await _selector(band)
		assert_bool(sel.uses_cards()).is_false()
		assert_bool(sel.dropdown_button().visible).is_true()
		var list: Control = sel.find_child("Options", true, false) as Control
		assert_bool(list.visible).is_false()
		sel.dropdown_button().pressed.emit()
		assert_bool(list.visible).is_true()
		assert_int(sel.option_count()).is_equal(3)
		# Starts on the activity the party plays now.
		assert_str(sel.dropdown_button().text).is_equal("Phép nhân ở Vương quốc Toán")
		assert_bool(sel.option("math_g3_multiplication").button_pressed).is_true()
		assert_bool(sel.confirm_button().disabled).is_false()
		sel.option("learning_zone.math").pressed.emit()
		assert_str(sel.dropdown_button().text).is_equal("Cùng khám phá Vương quốc Toán")
		assert_bool(sel.option("learning_zone.math").button_pressed).is_true()
		assert_bool(sel.option("math_g3_multiplication").button_pressed).is_false()
		# The ineligible boss cannot be picked.
		sel.option("math_fraction_dragon").pressed.emit()
		assert_bool(sel.option("math_fraction_dragon").button_pressed).is_false()
		assert_str(sel.selected.activity_ref).is_equal("learning_zone.math")
		sel.close()
