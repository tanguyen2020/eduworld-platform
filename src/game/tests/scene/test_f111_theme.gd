extends GdUnitTestSuite
## T038 (F111-AC-28; NFR-A11Y-001, ADR-0047, AGENTS R32): the F111 screens take their style from the generated Theme only
## (no runtime theme override, no Theme of their own — ThemeRules, the rule of tools/lint_theme_overrides.gd); the touch
## targets follow the band density (≥ 48 dp, lớp 1–3 ≥ 64 dp; every band in test_f111_band_matrix); presence, waiting
## for approval, claimed, the time left and every state have words (and an icon), never a colour alone.

const ThemeRules := preload("res://tools/lib/theme_rules.gd")
const FILES: PackedStringArray = ["res://scenes/social/suggestion_group.gd", "res://scenes/social/club_tab.gd",
	"res://scenes/social/club_card.gd", "res://scenes/social/club_page.gd", "res://scenes/social/club_create_dialog.gd",
	"res://scenes/social/club_invite_picker.gd", "res://scenes/social/friends_clubs.gd",
	"res://scenes/social/friend_profile_panel.gd", "res://scenes/ui/hud/event_center_button.gd"]

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func test_no_theme_override_in_the_f111_scenes() -> void:
	var problems: PackedStringArray = []
	var allow: PackedStringArray = ThemeRules.load_allowlist()
	var paths: PackedStringArray = FILES.duplicate()
	for f: String in DirAccess.get_files_at("res://scenes/liveops"):
		if f.ends_with(".gd") or f.ends_with(".tscn"):
			paths.append("res://scenes/liveops".path_join(f))
	for path: String in paths:
		if path.ends_with(".gd"):
			problems.append_array(ThemeRules.check_script(path, FileAccess.get_file_as_string(path)))
		else:
			problems.append_array(ThemeRules.check_scene(path, FileAccess.get_file_as_string(path), allow))
	assert_array(problems).override_failure_message("\n".join(problems)).is_empty()
	assert_int(paths.size()).is_greater(15)


func test_event_center_buttons_follow_the_band_touch_target() -> void:
	var root: Control = Control.new()
	add_child(root)
	root.size = Vector2(1280, 720)
	var screen: EventCenterScreen = await F111Screens.build("events_today", root, s.services, s.h.http) as EventCenterScreen
	var target: float = UiBuild.target()
	assert_float(target).is_greater_equal(64.0)
	var n: int = 0
	for node: Node in screen.find_children("*", "Button", true, false):
		var b: Button = node as Button
		if b.is_visible_in_tree():
			n += 1
			assert_float(maxf(b.size.y, b.get_combined_minimum_size().y)).override_failure_message(b.name).is_greater_equal(target - 0.5)
	assert_int(n).is_greater(3)
	screen.close()
	root.free()


func test_every_state_has_words_and_an_icon() -> void:
	for st: String in ClubModel.STATUSES:
		var c: ClubModel = ClubModel.from_dict(F111Fixtures.club(F111Fixtures.CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", st))
		assert_bool(L10n.has_key(c.status_key())).override_failure_message(st).is_true()
	for st: String in QuestDayTab.STATUS_ICONS.keys():
		assert_bool(ResourceLoader.exists(UiBuild.ICONS % String(QuestDayTab.STATUS_ICONS[st]))).is_true()
	for kind: String in FriendSuggestionModel.CONTEXT_VIEW.keys():
		var view: Array = FriendSuggestionModel.CONTEXT_VIEW[kind] as Array
		assert_bool(ResourceLoader.exists(UiBuild.ICONS % String(view[0])) and L10n.has_key(String(view[1]))).is_true()
	for icon: String in ["calendar", "clock", "check", "lock", "gift", "pause", "heart", "checkbox_on", "checkbox_off"]:
		assert_bool(ResourceLoader.exists(UiBuild.ICONS % icon)).override_failure_message(icon).is_true()
	# A milestone row says its state in words: claimed / not yet / just reached.
	for key: String in ["ui.events.claimed", "ui.events.not_reached", "ui.events.just_reached", "ui.club.status.pending_approval",
			"ui.events.time_left_days", "ui.events.last_day", "ui.events.suspended"]:
		assert_bool(L10n.has_key(key)).override_failure_message(key).is_true()
