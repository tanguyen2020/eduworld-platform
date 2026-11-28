extends GdUnitTestSuite
## T031 (F110-AC-28; NFR-A11Y-001, ADR-0047, AGENTS R32): the F110 screens take their style from the generated Theme only
## (no runtime theme override, no Theme resource of their own — ThemeRules, the rule of tools/lint_theme_overrides.gd),
## the touch targets follow the band density (≥ 48 dp, lớp 1–3 ≥ 64 dp; checked in every band by test_f110_band_matrix)
## and every state — online, ready, contribution, right / wrong — has an icon AND words, never a colour alone.

const ThemeRules := preload("res://tools/lib/theme_rules.gd")
const SCENE_DIRS: PackedStringArray = ["res://scenes/challenge/boss", "res://scenes/social", "res://scenes/quest"]

var s: F110SceneHarness = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func test_no_theme_override_in_the_f110_scenes() -> void:
	var problems: PackedStringArray = []
	var allow: PackedStringArray = ThemeRules.load_allowlist()
	for dir: String in SCENE_DIRS:
		for f: String in DirAccess.get_files_at(dir):
			var path: String = dir.path_join(f)
			if f.ends_with(".gd"):
				problems.append_array(ThemeRules.check_script(path, FileAccess.get_file_as_string(path)))
			elif f.ends_with(".tscn"):
				problems.append_array(ThemeRules.check_scene(path, FileAccess.get_file_as_string(path), allow))
	assert_array(problems).override_failure_message("\n".join(problems)).is_empty()


func test_buttons_follow_the_band_touch_target() -> void:
	var screen: FriendsScreen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.open({})
	await s.settle(4)
	var target: float = UiBuild.target()
	assert_float(target).is_greater_equal(64.0)
	for node: Node in screen.find_children("*", "Button", true, false):
		var b: Button = node as Button
		if b.is_visible_in_tree():
			assert_float(maxf(b.size.y, b.get_combined_minimum_size().y)).override_failure_message(b.name).is_greater_equal(target - 0.5)
	screen.close()


func test_every_state_has_an_icon_and_words() -> void:
	var f: FriendModel = FriendModel.from_card(F110Fixtures.card(F110Fixtures.FRIEND_ONLINE, "CaHeoVui07"))
	for presence: String in FriendModel.PRESENCES:
		f.set_presence(presence)
		assert_bool(ResourceLoader.exists(UiBuild.ICONS % f.presence_icon())).is_true()
		assert_bool(L10n.has_key(f.presence_key())).override_failure_message(f.presence_key()).is_true()
	var list: ContributionList = auto_free(ContributionList.new()) as ContributionList
	add_child(list)
	var members: Array[BossMemberModel] = BossMemberModel.list_from([F110Fixtures.member(s.self_id, "MeoXanh42", true, true, true),
		F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK, false)])
	for lobby: bool in [true, false]:
		list.show_members(members, s.self_id, 2, lobby, true)
		for id: String in [s.self_id, F110Fixtures.MATE]:
			var chip: PanelContainer = list.chip(id)
			assert_object((chip.find_child("StateIcon", true, false) as TextureRect).texture).is_not_null()
			assert_str((chip.find_child("State", true, false) as Label).text).is_not_empty()
	var strip: PartyProgressStrip = auto_free(PartyProgressStrip.new()) as PartyProgressStrip
	add_child(strip)
	strip.show_progress(PartyQuestProgressModel.from_dict("q", {"step_id": "s", "collective_progress": 0.5, "my_contribution_met": true}))
	assert_str(strip.mine_text()).is_not_empty()
	assert_bool(L10n.has_key("ui.boss.hit") and L10n.has_key("ui.boss.strike_back")).is_true()
