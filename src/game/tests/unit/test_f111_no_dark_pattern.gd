extends GdUnitTestSuite
## T039 (F111-AC-29; AGENTS R19, NFR-SAFE-003, FR-EVT-007): the Event center, the clubs and "Tìm bạn" have no
## second-by-second countdown, no blinking (no AnimationPlayer, no Tween, no per-frame timer of their own), no streak / day
## chain, no "chỉ còn…" wording, no push to play more, no out-of-game notification; the HUD dot is static and goes away once
## the Event center was opened (test_event_center_entry). No season premium / paid tier anywhere (ADR-0049).

const FILES: PackedStringArray = ["res://scenes/social/suggestion_group.gd", "res://scenes/social/club_tab.gd",
	"res://scenes/social/club_card.gd", "res://scenes/social/club_page.gd", "res://scenes/social/club_create_dialog.gd",
	"res://scenes/social/club_invite_picker.gd", "res://scenes/social/friends_clubs.gd",
	"res://scenes/ui/hud/event_center_button.gd", "res://scripts/liveops/live_event_service.gd",
	"res://scripts/liveops/season_track_service.gd", "res://scripts/liveops/live_event_store.gd",
	"res://scripts/liveops/liveops_time.gd", "res://scripts/social/club_service.gd"]
const CODE_FORBIDDEN: PackedStringArray = ["AnimationPlayer", "create_tween", "Tween.", "Timer.new", "duration(",
	"get_ticks_msec", "OS.request_permissions", "push_notification"]
const WORD_FORBIDDEN: PackedStringArray = ["streak", "chuỗi", "chỉ còn", "only ", "hurry", "nhanh lên", "đừng bỏ lỡ",
	"don't miss", "mua", "buy", "premium", "vip", "giây", "second"]


func _scene_files() -> PackedStringArray:
	var out: PackedStringArray = FILES.duplicate()
	for f: String in DirAccess.get_files_at("res://scenes/liveops"):
		if f.ends_with(".gd"):
			out.append("res://scenes/liveops".path_join(f))
	return out


func test_no_animation_timer_or_countdown_in_the_f111_code() -> void:
	for path: String in _scene_files():
		var text: String = FileAccess.get_file_as_string(path)
		assert_bool(text.is_empty()).override_failure_message(path).is_false()
		for needle: String in CODE_FORBIDDEN:
			assert_bool(text.contains(needle)).override_failure_message("%s: %s" % [path, needle]).is_false()
	# Only the notice banner counts down (seconds of reading time, not shown) — that is the screen's _process.
	assert_bool(FileAccess.get_file_as_string("res://scenes/liveops/event_center_screen.gd").contains("func _process")).is_true()


func test_no_fomo_streak_or_paid_words_in_the_f111_strings() -> void:
	var PoFileScript: GDScript = load("res://tools/lib/po_file.gd") as GDScript
	for locale: String in ["vi", "en"]:
		var po: Dictionary = PoFileScript.call("parse", FileAccess.get_file_as_string("res://locale/%s.po" % locale)) as Dictionary
		var checked: int = 0
		for key: Variant in po.keys():
			var k: String = String(key)
			if not (k.begins_with("ui.events.") or k.begins_with("ui.season.") or k.begins_with("ui.club.")
					or k.begins_with("ui.friends.suggest.") or k.begins_with("ui.friends.join")):
				continue
			checked += 1
			var text: String = String(po[key]).to_lower()
			for w: String in WORD_FORBIDDEN:
				assert_bool(text.contains(w)).override_failure_message("%s %s: '%s' in '%s'" % [locale, k, w, text]).is_false()
		assert_int(checked).is_greater(120)


func test_rest_suggestion_and_no_play_more() -> void:
	assert_str(TranslationServer.get_translation_object("vi").get_message("ui.events.all_done")).contains("Nghỉ ngơi")
	var tab: QuestDayTab = auto_free(QuestDayTab.new("daily")) as QuestDayTab
	add_child(tab)
	var done: Array[QuestModel] = []
	for d: Variant in F111Fixtures.daily(1_763_600_000, true)["data"] as Array:
		done.append(QuestModel.from_dict(d))
	tab.show_quests(done, 1_763_600_000)
	assert_bool(tab.all_done_visible()).is_true()
	assert_array(tab.go_buttons()).is_empty()
