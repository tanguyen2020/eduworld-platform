extends GdUnitTestSuite
## T037 (F111-AC-27; NFR-I18N-001, NFR-I18N-003, FR-EVT-008, ADR-0056): every F111 string is a `ui.events.*`,
## `ui.season.*`, `ui.club.*`, `ui.friends.*` or `ui.error.<CODE>` key with a Vietnamese and an English translation; a
## `LocalizedName` / `TenantLocalizedName` is picked by language (a school event without English shows Vietnamese); dates
## are ICT in the locale format; switching vi ↔ en re-renders at once; the pseudo-locale (+35–40 %) still fits the longest
## F111 strings (`ui.events.all_done`, `ui.friends.join_unavailable.*`) in every band at every reference window.

const PoFileScript := preload("res://tools/lib/po_file.gd")
const FILES: PackedStringArray = [
	"res://scripts/social/join_availability.gd", "res://scripts/multiplayer/join_friend_service.gd",
	"res://scripts/social/models/friend_suggestion_model.gd", "res://scripts/social/models/club_model.gd",
	"res://scripts/social/models/club_member_model.gd", "res://scripts/social/models/club_goal_model.gd",
	"res://scripts/social/club_service.gd", "res://scripts/social/club_form.gd", "res://scripts/liveops/live_event_service.gd",
	"res://scripts/liveops/season_track_service.gd", "res://scripts/liveops/models/live_event_model.gd",
	"res://scenes/social/suggestion_group.gd", "res://scenes/social/club_tab.gd", "res://scenes/social/club_card.gd",
	"res://scenes/social/club_page.gd", "res://scenes/social/club_create_dialog.gd", "res://scenes/social/club_invite_picker.gd",
	"res://scenes/social/friends_clubs.gd", "res://scenes/liveops/event_center_screen.gd", "res://scenes/liveops/event_card.gd",
	"res://scenes/liveops/event_detail_panel.gd", "res://scenes/liveops/community_goal_bar.gd",
	"res://scenes/liveops/quest_day_tab.gd", "res://scenes/liveops/season_tab.gd", "res://scenes/liveops/season_track_view.gd",
	"res://scenes/ui/hud/event_center_button.gd",
]

var s: F111SceneHarness = null
var _locale: String = ""


func before_test() -> void:
	_locale = String(Settings.locale)
	s = F111SceneHarness.new()
	await s.start("g1_3", false)


func after_test() -> void:
	Settings.set_locale(_locale, false)
	s.dispose()
	AppState.set_compact(false)


func test_every_f111_key_has_vi_and_en() -> void:
	var vi: Dictionary = PoFileScript.parse(FileAccess.get_file_as_string("res://locale/vi.po"))
	var en: Dictionary = PoFileScript.parse(FileAccess.get_file_as_string("res://locale/en.po"))
	var re: RegEx = RegEx.create_from_string("\"(ui\\.[a-z0-9_]+(?:\\.[A-Za-z0-9_]+)+)\"")
	var checked: int = 0
	var missing: PackedStringArray = []
	for path: String in FILES:
		for m: RegExMatch in re.search_all(FileAccess.get_file_as_string(path)):
			var key: String = m.get_string(1)
			if key.ends_with(".") or key.contains("%"):
				continue
			if not String(vi.get(key, "")).is_empty() and not String(en.get(key, "")).is_empty():
				checked += 1
			else:
				missing.append("%s: %s" % [path, key])
	assert_array(missing).is_empty()
	assert_int(checked).is_greater(80)
	# Keys built from data (`ui.club.type.<type>`, `ui.events.quest_status.<status>`…) exist for every value.
	for t: String in ClubModel.TYPES:
		assert_bool(vi.has("ui.club.type.%s" % t) and en.has("ui.club.type.%s" % t)).override_failure_message(t).is_true()
	for st: String in ClubModel.STATUSES:
		assert_bool(vi.has("ui.club.status.%s" % st) and en.has("ui.club.status.%s" % st)).is_true()
	for r: String in ClubModel.ROLES:
		assert_bool(vi.has("ui.club.role.%s" % r) and en.has("ui.club.role.%s" % r)).is_true()
	for st: String in QuestDayTab.STATUS_ICONS.keys():
		assert_bool(vi.has("ui.events.quest_status.%s" % st)).is_true()
	for reason: StringName in JoinAvailability.REASONS:
		assert_bool(vi.has(JoinAvailability.reason_key(reason)) and en.has(JoinAvailability.reason_key(reason))).is_true()
	for code: String in ["CLUB_FULL", "CLUB_LIMIT_REACHED", "CLUB_NOT_APPROVED", "CHAT_CONTENT_BLOCKED", "EVENT_NOT_AVAILABLE",
			"EVENT_ENDED", "EVENT_MILESTONE_NOT_REACHED", "SEASON_TIER_NOT_REACHED", "SOCIAL_SCOPE_VIOLATION", "PERMISSION_DENIED"]:
		assert_bool(vi.has("ui.error.%s" % code) and en.has("ui.error.%s" % code)).override_failure_message(code).is_true()


func test_switching_vi_en_rerenders_and_picks_names() -> void:
	var root: Control = Control.new()
	add_child(root)
	root.size = Vector2(1280, 720)
	var screen: EventCenterScreen = await F111Screens.build("events_list", root, s.services, s.h.http) as EventCenterScreen
	assert_array(Array(screen.card(F111Fixtures.EVENT_FESTIVAL).texts())).contains(["Lễ hội Trăng Rằm"])
	Settings.set_locale("en-US", false)
	await get_tree().process_frame
	assert_array(Array(screen.card(F111Fixtures.EVENT_FESTIVAL).texts())).contains(["Mid-Autumn Festival"])
	assert_array(Array(screen.card(F111Fixtures.EVENT_SCHOOL).texts())).contains(["Tuần lễ đọc sách"])
	assert_str(tr(screen.tab_button("events").text)).is_equal("Events")
	# ICT dates in the locale format.
	var at: int = LiveOpsTime.unix_of("2026-11-28T17:30:00Z")
	assert_str(LiveOpsTime.ict_date(at)).is_equal(LocaleFormat.date(Time.get_datetime_dict_from_unix_time(at + 7 * 3600)))
	screen.close()
	root.free()


func test_the_pseudo_locale_fits_the_longest_f111_strings() -> void:
	Settings.set_locale(L10n.pseudo_locale(), false)
	var failures: PackedStringArray = []
	for view: String in ["friends_join", "events_done", "event_detail", "club_page"]:
		failures.append_array(await F111Matrix.run(self, s, view))
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()
