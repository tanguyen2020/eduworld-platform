extends GdUnitTestSuite
## F062-AC-05, F062-AC-06 (FR-GAME-010, NFR-A11Y-001, NFR-I18N-001, ADR-0047, ADR-0056, TC-UI-001, TC-UI-003,
## TC-I18N-004/005; T010): the HUD follows the grade band (Theme density from ThemeService; touch targets ≥ the
## UiTokens density height, 64 dp for `g1_3`; no minimap for `g1_3`), shows only the M-C2 components (future slots
## hidden), every string is a `ui.hub.*`/`ui.quest.*` key, a locale change re-renders it without restart, and the
## F062 scenes carry no theme override (lint_theme_overrides rule).

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const ThemeRules := preload("res://tools/lib/theme_rules.gd")
const I18nRules := preload("res://tools/lib/i18n_rules.gd")
const F062_SCENES: PackedStringArray = [
	"res://scenes/ui/hud/MainHud.tscn", "res://scenes/ui/hud/ContextActionButton.tscn", "res://scenes/ui/hud/Joystick.tscn",
	"res://scenes/ui/hud/BreakReminderDialog.tscn", "res://scenes/ui/hud/NoticeDialog.tscn",
	"res://scenes/quest/QuestJournal.tscn", "res://scenes/quest/AssignmentRow.tscn",
	"res://scenes/ui/settings/SettingsPanel.tscn", "res://scenes/ui/settings/QualitySettings.tscn",
	"res://scenes/bootstrap/QualityBenchmark.tscn", "res://scenes/world/WorldRoot.tscn",
	"res://scenes/world/zones/main_hub/Zone.tscn", "res://scenes/character/Player.tscn",
]

var _band: String = ""
var _locale: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	_locale = String(Settings.locale)
	AppState.start_ui(get_tree().root)


func after_test() -> void:
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)
	AppState.player = {}
	var none: Array[MyAssignmentModel] = []
	AppState.set_assignments(none)


func _hud() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	return [runner, runner.scene()]


func test_touch_targets_and_minimap_follow_the_grade_band() -> void:
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	for band: String in UiTokens.DENSITY_BANDS:
		AppState.set_grade_band(band)
		await runner.simulate_frames(1)
		assert_str(AppState.theme_service.band).is_equal(band)
		var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
		for name: String in ["AssignmentsChip", "JournalButton", "SettingsButton", "HomeButton", "ContextActionButton"]:
			var button: Button = hud.find_child(name, true, false) as Button
			assert_float(button.get_combined_minimum_size().y).override_failure_message("%s %s" % [band, name]) \
					.is_greater_equal(minimum)
		assert_bool(bool(hud.get("minimap_allowed"))).is_equal(band != "g1_3")
	assert_float(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT["g1_3"])).is_greater_equal(64.0)


func test_only_m_c2_components_are_visible() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	# F102: PartyChip is the PartyHud ("Nhóm" button) and is shown.
	assert_bool((hud.find_child("PartyChip", true, false) as Control).visible).is_true()
	for slot: String in ["XpBar", "CurrencyChip", "QuestTracker", "Minimap", "ChatDock"]:
		var node: Control = hud.find_child(slot, true, false) as Control
		assert_object(node).override_failure_message(slot).is_not_null()
		assert_bool(node.visible).override_failure_message(slot).is_false()
	for shown: String in ["PlayerBadge", "AssignmentsChip", "JournalButton", "SettingsButton", "HomeButton"]:
		assert_bool((hud.find_child(shown, true, false) as Control).visible).override_failure_message(shown).is_true()


func test_badge_and_chip_show_bootstrap_and_assignments() -> void:
	AppState.player = AuthHarness.bootstrap()
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	assert_str((hud.find_child("Nickname", true, false) as Label).text).is_equal("MeoXanh42")
	var chip: Button = hud.find_child("AssignmentsChip", true, false) as Button
	assert_str(chip.text).is_equal(tr("ui.hub.assignments"))
	assert_str(chip.text).not_contains("(")
	AppState.set_assignments(MyAssignmentModel.list_from(AssignmentFixtures.two()["data"] as Array))
	await runner.simulate_frames(1)
	assert_str(chip.text).is_equal(tr("ui.hub.assignments_count").format({"count": 2}))


func test_locale_change_updates_the_hud_without_restart() -> void:
	Settings.set_locale("vi-VN", false)
	AppState.set_assignments(MyAssignmentModel.list_from(AssignmentFixtures.two()["data"] as Array))
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	var chip: Button = hud.find_child("AssignmentsChip", true, false) as Button
	var home: Button = hud.find_child("HomeButton", true, false) as Button
	var vi_chip: String = chip.text
	var vi_home: String = home.tr(home.text)
	hud.call("show_banner", "ui.login.curfew_warning", {"minutes": 10})
	var banner: Label = hud.find_child("BannerLabel", true, false) as Label
	var vi_banner: String = banner.text
	assert_bool(Settings.set_locale("en-US", false)).is_true()
	await runner.simulate_frames(1)
	assert_str(chip.text).is_not_equal(vi_chip)
	assert_str(chip.text).is_equal(TranslationServer.translate("ui.hub.assignments_count").format({"count": 2}))
	assert_str(home.tr(home.text)).is_not_equal(vi_home)
	assert_str(banner.text).is_not_equal(vi_banner)
	assert_str(banner.text).contains("10")


func test_f062_scenes_use_keys_and_no_theme_override() -> void:
	var allowlist: PackedStringArray = ThemeRules.load_allowlist()
	for path: String in F062_SCENES:
		var text: String = FileAccess.get_file_as_string(path)
		assert_array(Array(ThemeRules.check_scene(path, text, allowlist))).override_failure_message(path).is_empty()
		assert_array(Array(I18nRules.hardcoded_text(path, text))).override_failure_message(path).is_empty()
	for key: String in ["ui.hub.assignments", "ui.hub.assignments_count", "ui.hub.home", "ui.hub.settings",
			"ui.hub.journal", "ui.hub.action.read", "ui.quest.tab.assigned", "ui.common.break_reminder"]:
		for locale: String in ["vi-VN", "en-US"]:
			var message: String = String(TranslationServer.get_translation_object(locale).get_message(key))
			assert_str(message).override_failure_message("%s %s" % [locale, key]).is_not_empty()


## F096-AC-09: every HUD button has an icon of design/icons/game beside a translated label, no PC key hint in the label.
func test_buttons_have_icons_and_plain_labels() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	for name: String in ["AssignmentsChip", "JournalButton", "NotificationBell", "SettingsButton", "WardrobeButton",
			"MapButton", "HomeButton", "NextStepButton"]:
		var button: Button = hud.find_child(name, true, false) as Button
		assert_object(button.icon).override_failure_message(name).is_not_null()
		assert_str(button.icon.resource_path).override_failure_message(name).starts_with("res://resources/ui/icons/")
	for locale: String in ["vi-VN", "en-US"]:
		Settings.set_locale(locale, false)
		await (r[0] as GdUnitSceneRunner).simulate_frames(1)
		var journal: Button = hud.find_child("JournalButton", true, false) as Button
		assert_str(tr(journal.text)).not_contains("(")
		assert_str(tr(journal.text)).not_contains("ui.")


## F096-AC-11: "Việc tiếp theo" points at school work when there is some, else at the World Map, and asks the owner to
## open it.
func test_next_step_card_suggests_and_opens() -> void:
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	var card: Button = hud.find_child("NextStepButton", true, false) as Button
	assert_bool(card.is_visible_in_tree()).is_true()
	assert_str(card.text).is_equal(tr("ui.next_step.explore"))
	var asked: Array = []
	hud.connect("next_step_pressed", func(id: StringName, args: Dictionary) -> void: asked.append([id, args]))
	card.pressed.emit()
	assert_str(String((asked[0] as Array)[0])).is_equal("world_map")
	var list: Array[MyAssignmentModel] = []
	for item: Variant in AssignmentFixtures.two()["data"] as Array:
		list.append(MyAssignmentModel.from_dict(item as Dictionary))
	AppState.set_assignments(list)
	await runner.simulate_frames(1)
	assert_str(card.text).is_equal(tr("ui.next_step.assignments").format({"count": list.size()}))
	card.pressed.emit()
	assert_str(String((asked[1] as Array)[0])).is_equal("quest_journal")
	assert_str(String(((asked[1] as Array)[1] as Dictionary)["tab"])).is_equal("assigned")


## F096-AC-10: Mây's guide opens on demand, rings the HUD control of each step, "Bỏ qua" closes it and remembers the
## device; the ring does not pulse with reduced motion.
func test_onboarding_guides_skips_and_remembers() -> void:
	var seen: bool = Settings.onboarding_hub_seen
	var reduce: bool = Settings.reduce_motion
	Settings.reduce_motion = false
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	var hint: OnboardingHint = hud.call("onboarding") as OnboardingHint
	assert_bool(hint.visible).is_false()
	hud.call("start_onboarding")
	await runner.simulate_frames(2)
	assert_bool(hint.visible).is_true()
	assert_str((hint.find_child("StepText", true, false) as Label).text).is_equal(tr("ui.onboarding.step_welcome"))
	assert_object((hint.find_child("StepIcon", true, false) as TextureRect).texture).is_not_null()
	(hint.find_child("Next", true, false) as Button).pressed.emit()
	await runner.simulate_frames(2)
	var chip: Control = hud.find_child("AssignmentsChip", true, false) as Control
	var ring: Rect2 = hint.target_rect()
	ring.position += hint.get_global_rect().position
	assert_bool(ring.encloses(chip.get_global_rect())).is_true()
	assert_bool(hint.pulsing()).is_true()
	Settings.reduce_motion = true
	(hint.find_child("Next", true, false) as Button).pressed.emit()
	await runner.simulate_frames(1)
	assert_bool(hint.pulsing()).is_false()
	assert_str((hint.find_child("Next", true, false) as Button).text).is_equal(tr("ui.onboarding.start"))
	hint.skip()
	assert_bool(hint.visible).is_false()
	assert_bool(Settings.onboarding_hub_seen).is_true()
	Settings.onboarding_hub_seen = seen
	Settings.reduce_motion = reduce
