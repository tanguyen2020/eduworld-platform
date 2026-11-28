extends GdUnitTestSuite
## T030 (F110-AC-27; NFR-I18N-001, NFR-I18N-003, ADR-0056): every F110 string is a `ui.boss.*`, `ui.quest.coop.*`,
## `ui.quest.clue.*`, `ui.friends.*`, `ui.friendcode.*` or `ui.error.<CODE>` key with a Vietnamese and an English
## translation; a `LocalizedName` / `LocalizedText` is picked by language; switching vi ↔ en re-renders at once; the
## pseudo-locale (+35–40 %) still fits — `ui.boss.need_contribution` and `ui.friendcode.instructions` (the longest ones)
## are never cut — in every band at every reference window.

const PoFileScript := preload("res://tools/lib/po_file.gd")
const PREFIXES: PackedStringArray = ["ui.boss.", "ui.quest.coop", "ui.quest.clue.", "ui.friends.", "ui.friendcode.",
	"ui.error.", "ui.menu.friends"]
const FILES: PackedStringArray = ["res://scripts/challenge/boss_client.gd", "res://scripts/quest/coop_quest_service.gd",
	"res://scripts/social/social_service.gd", "res://scripts/social/models/friend_model.gd",
	"res://scripts/social/models/friend_request_model.gd", "res://scenes/challenge/boss/boss_arena.gd",
	"res://scenes/challenge/boss/boss_info_card.gd", "res://scenes/challenge/boss/boss_result.gd",
	"res://scenes/challenge/boss/contribution_list.gd", "res://scenes/challenge/boss/question_panel_host.gd",
	"res://scenes/challenge/boss/boss_energy_bar.gd", "res://scenes/challenge/boss/phase_timer.gd",
	"res://scenes/quest/interdependent_step_panel.gd", "res://scenes/quest/clue_card.gd",
	"res://scenes/quest/shared_clue_board.gd", "res://scenes/quest/role_assign_dialog.gd",
	"res://scenes/quest/party_progress_strip.gd", "res://scenes/social/friends_screen.gd",
	"res://scenes/social/friend_row.gd", "res://scenes/social/friend_list_view.gd",
	"res://scenes/social/friend_profile_panel.gd", "res://scenes/social/friend_request_list.gd",
	"res://scenes/social/find_friends_tab.gd", "res://scenes/social/friend_code_widget.gd", "res://scenes/social/block_list.gd"]

var s: F110SceneHarness = null
var _locale: String = ""


func before_test() -> void:
	_locale = String(Settings.locale)
	s = F110SceneHarness.new()
	await s.start("g1_3", false)


func after_test() -> void:
	Settings.set_locale(_locale, false)
	s.dispose()
	AppState.set_compact(false)


func test_every_f110_key_has_vi_and_en() -> void:
	var vi: Dictionary = PoFileScript.parse(FileAccess.get_file_as_string("res://locale/vi.po"))
	var en: Dictionary = PoFileScript.parse(FileAccess.get_file_as_string("res://locale/en.po"))
	var re: RegEx = RegEx.create_from_string("\"(ui\\.[a-z0-9_]+(?:\\.[A-Za-z0-9_]+)+)\"")
	var checked: int = 0
	var missing: PackedStringArray = []
	for path: String in FILES:
		for m: RegExMatch in re.search_all(FileAccess.get_file_as_string(path)):
			var key: String = m.get_string(1)
			if not String(vi.get(key, "")).is_empty() and not String(en.get(key, "")).is_empty():
				checked += 1
			else:
				missing.append("%s: %s" % [path, key])
	assert_array(missing).is_empty()
	assert_int(checked).is_greater(100)
	# The F110 keys live in their namespaces.
	for row: String in ["ui.boss.need_contribution", "ui.friendcode.instructions", "ui.quest.clue.share",
			"ui.quest.coop.title", "ui.friends.request_pending", "ui.error.FRIEND_CODE_INVALID"]:
		assert_bool(vi.has(row) and en.has(row)).is_true()


func test_switching_vi_en_rerenders_the_screens_at_once() -> void:
	var root: Control = Control.new()
	add_child(root)
	root.size = Vector2(1280, 720)
	var screen: FriendsScreen = await F110Screens.build("friends", root, s.services, s.h.http) as FriendsScreen
	var vi_tab: String = screen.tab_button(FriendsScreen.TAB_FRIENDS).text
	Settings.set_locale("en-US", false)
	await get_tree().process_frame
	var en_tab: String = screen.tab_button(FriendsScreen.TAB_FRIENDS).text
	assert_str(en_tab).is_not_equal(vi_tab)
	assert_str(en_tab).starts_with("Friends")
	# LocalizedName picked by language (the boss title of the card).
	var b: BossSummary = BossSummary.from_dict(F110Fixtures.boss())
	assert_str(b.display_title("en")).is_equal("Fraction Dragon")
	assert_str(b.display_title("vi")).is_equal("Rồng Phân Số")
	screen.close()
	root.free()


func test_the_pseudo_locale_fits_the_longest_f110_strings_in_every_band() -> void:
	Settings.set_locale(L10n.pseudo_locale(), false)
	var failures: PackedStringArray = []
	for view: String in ["friends_find", "boss_question", "boss_result_time_up", "coop_step"]:
		failures.append_array(await F110Matrix.run(self, s, view))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()
