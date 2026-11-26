extends GdUnitTestSuite
## F069 S09 achievements, collections, badge showcase (T032, T033; F069-AC-21…AC-24): a hidden achievement shows only "?"
## and the vague hint even when the response carries its title; unlocked ones say "Đã nhận" with the server's rewards and
## no claim button exists; a collection draws missing items with the same picture greyed plus "Chưa có" and "Cách nhận",
## g1_3 as a sticker book of bigger pictures; the showcase keeps only owned badges, the server's slot count, and saves
## through `updateAvatar`.

const SCREEN: String = "res://scenes/achievement/AchievementCollectionScreen.tscn"

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func _open(tab: String, band: String = "g1_3") -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(SCREEN)
	await runner.simulate_frames(1)
	var screen: Control = runner.scene() as Control
	screen.call("open", {"tab": tab})
	await runner.simulate_frames(8)
	return [runner, screen]


func test_hidden_achievement_shows_only_a_question_mark() -> void:
	var r: Array = await _open("achievements", "g6_7")
	var screen: Control = r[1] as Control
	var texts: PackedStringArray = screen.call("row_texts") as PackedStringArray
	assert_array(Array(texts)).contains(["?", "ui.achievement.hidden_hint", "Nhà thám hiểm Vương quốc Số", "7/10"])
	for t: String in texts:
		assert_str(t).not_contains("Bí mật của Mây")
		assert_str(t).not_contains("Cloudy")
	var hidden: AchievementModel = null
	for a: AchievementModel in s.services.achievements.achievements:
		if a.achievement_id == F069Fixtures.HIDDEN_ACHIEVEMENT:
			hidden = a
	assert_bool(hidden.hidden).is_true()
	assert_dict(hidden.title).is_empty()
	assert_float(hidden.progress_target).is_equal(0.0)


func test_unlocked_says_received_and_no_claim_button() -> void:
	var r: Array = await _open("achievements", "g6_7")
	var screen: Control = r[1] as Control
	assert_array(Array(screen.call("row_texts"))).contains(["ui.achievement.received", "ui.achievement.tier.gold"])
	for b: Node in screen.find_children("*", "Button", true, false):
		var text: String = tr((b as Button).text).to_lower()
		assert_str(text).not_contains("nhận thưởng")
		assert_str(text).not_contains("claim")


func test_collection_greys_missing_items_with_how_to_obtain() -> void:
	var r: Array = await _open("collections", "g6_7")
	var screen: Control = r[1] as Control
	await screen.call("open_collection", F069Fixtures.COLLECTION_ID)
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	var owned: Control = screen.find_child("Item_pet_00", true, false) as Control
	var missing: Control = screen.find_child("Item_pet_05", true, false) as Control
	assert_object((owned.find_child("Thumb", true, false) as ItemThumb).material).is_null()
	var thumb: ItemThumb = missing.find_child("Thumb", true, false) as ItemThumb
	assert_bool(thumb.greyed).is_true()
	assert_object(thumb.material).is_same(ItemThumb.grey_material())
	assert_str((missing.find_child("Status", true, false) as Label).text).is_equal("ui.collection.missing")
	assert_str((missing.find_child("HowToObtain", true, false) as Label).text).is_equal(
			tr("ui.collection.how_to_obtain_line").format({"how": "Hoàn thành chuỗi nhiệm vụ Rừng Ngôn Ngữ"}))
	assert_object(owned.find_child("HowToObtain", true, false)).is_null()


func test_g1_3_collection_is_a_sticker_book_of_bigger_pictures() -> void:
	var r: Array = await _open("collections", "g1_3")
	var screen: Control = r[1] as Control
	await screen.call("open_collection", F069Fixtures.COLLECTION_ID)
	var small_side: float = UiBuild.target()
	var thumb: ItemThumb = (screen.find_child("Item_pet_01", true, false) as Control).find_child("Thumb", true, false) as ItemThumb
	assert_float(thumb.custom_minimum_size.x).is_greater_equal(small_side * 2.5)
	# STUDENT_GAME_UX_SOCIAL §4: no showcase tab for g1_3.
	assert_object((screen.get("_tabs") as FilterChips).chip("showcase")).is_null()


func test_showcase_keeps_owned_badges_within_the_server_slots_and_saves() -> void:
	s.h.http.clear(F069Fixtures.ROUTE_INVENTORY)
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.badges())
	var r: Array = await _open("showcase", "g6_7")
	var screen: Control = r[1] as Control
	assert_str(s.h.http.last(F069Fixtures.ROUTE_INVENTORY).query).contains("kind=badge")
	for code: String in ["badge_explorer", "badge_friend", "badge_star", "badge_math", "badge_not_owned"]:
		screen.call("toggle_badge", code)
	var draft: PackedStringArray = screen.get("showcase_draft") as PackedStringArray
	assert_int(draft.size()).is_equal(s.services.achievements.badge_slots())
	assert_bool("badge_not_owned" in draft).is_false()
	var saved: AvatarConfigModel = s.services.profile.avatar_config().copy()
	saved.badge_showcase = draft
	s.h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, saved.to_dict())
	await screen.call("save_showcase")
	var put: FakeHttp.Recorded = s.h.http.last(ProfileFixtures.ROUTE_AVATAR)
	assert_array(put.body["badge_showcase"] as Array).contains_exactly(["badge_explorer", "badge_friend", "badge_star"])
	screen.call("toggle_badge", "badge_friend")
	assert_array(Array(screen.get("showcase_draft") as PackedStringArray)).contains_exactly(["badge_explorer", "badge_star"])
