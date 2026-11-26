extends GdUnitTestSuite
## F069 i18n, theme and performance (T036, T037, T038; F069-AC-25, AC-26, AC-27; NFR-I18N-001/003, NFR-A11Y-001,
## NFR-COMP-001): every F069 key exists in vi and en, a locale switch re-renders the HUD at once, numbers follow the
## locale, the pseudo-locale (+35 %) keeps the main buttons readable; the F069 scenes carry no theme override; the grid
## of 500 items creates only the visible cells + 1 buffer row and keeps that pool while scrolling; a panel opens in
## ≤ 200 ms.

const ThemeRules := preload("res://tools/lib/theme_rules.gd")
const I18nRules := preload("res://tools/lib/i18n_rules.gd")
const SCREENS: PackedStringArray = [
	"res://scenes/inventory/InventoryShopScreen.tscn", "res://scenes/achievement/AchievementCollectionScreen.tscn",
	"res://scenes/challenge/ChallengeBoard.tscn", "res://scenes/progression/JourneyScreen.tscn",
	"res://scenes/ui/hud/GameMenu.tscn",
]
const PREFIXES: PackedStringArray = ["ui.progression.", "ui.levelup.", "ui.journey.", "ui.inventory.", "ui.shop.",
	"ui.achievement.", "ui.collection.", "ui.challenge.", "ui.reward.", "ui.menu."]
## AC-27 / SC-003: a panel is usable (skeleton at least) within this budget.
const PANEL_OPEN_MAX_MS: int = 200

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func test_every_f069_key_exists_in_vi_and_en() -> void:
	var keys: PackedStringArray = []
	for dir: String in ["res://scripts", "res://scenes"]:
		for path: String in _scan(dir):
			for key: String in I18nRules.keys_in(FileAccess.get_file_as_string(path)):
				for prefix: String in PREFIXES:
					if key.begins_with(prefix) and not key in keys:
						keys.append(key)
	assert_int(keys.size()).is_greater(150)
	for locale: String in ["vi-VN", "en-US"]:
		var t: Translation = TranslationServer.get_translation_object(locale)
		for key: String in keys:
			assert_str(String(t.get_message(key))).override_failure_message("%s %s" % [locale, key]).is_not_empty()


func test_locale_switch_and_number_format() -> void:
	await s.services.progression.refresh()
	s.h.http.clear(F069Fixtures.ROUTE_WALLET)
	s.h.http.reply(F069Fixtures.ROUTE_WALLET, 200, F069Fixtures.wallet(1250))
	await s.services.inventory.load_wallet()
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/ui/hud/MainHud.tscn")
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var chip: CurrencyChip = hud.call("currency_chip") as CurrencyChip
	var vi_text: String = chip.text
	assert_str(vi_text).is_equal(tr("ui.reward.currency").format({"amount": LocaleFormat.number(1250)}))
	assert_str(vi_text).contains("Xu")
	Settings.set_locale("en-US", false)
	await runner.simulate_frames(1)
	assert_str(chip.text).contains("Coins")
	assert_str(chip.text).contains("1,250")
	assert_str(vi_text).not_contains("1,250")
	assert_str((hud.call("xp_bar") as XpBar).level_text()).starts_with("Level 3")
	assert_str((hud.call("xp_bar") as XpBar).level_text()).contains("Little Explorer")


func test_pseudo_locale_keeps_the_main_buttons_whole() -> void:
	if not L10n.is_supported(L10n.pseudo_locale()):
		return
	Settings.set_locale(L10n.pseudo_locale(), false)
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/inventory/InventoryShopScreen.tscn")
	await runner.simulate_frames(1)
	var screen: Control = runner.scene() as Control
	screen.call("open", {"tab": "shop"})
	await runner.simulate_frames(8)
	screen.call("select_entry", 0)
	await runner.simulate_frames(2)
	for name: String in ["Buy", "TryOn", "Close"]:
		var b: Button = screen.find_child(name, true, false) as Button
		assert_bool(b.clip_text).override_failure_message(name).is_false()
		assert_float(b.size.x).override_failure_message(name).is_greater_equal(b.get_combined_minimum_size().x - 0.5)


func test_f069_scenes_have_no_theme_override_and_no_hardcoded_text() -> void:
	var allowlist: PackedStringArray = ThemeRules.load_allowlist()
	for dir: String in ["res://scenes/progression", "res://scenes/inventory", "res://scenes/achievement",
			"res://scenes/challenge", "res://scenes/ui/hud", "res://scenes/ui/components"]:
		for path: String in _scan(dir):
			var text: String = FileAccess.get_file_as_string(path)
			if path.ends_with(".tscn"):
				assert_array(Array(ThemeRules.check_scene(path, text, allowlist))).override_failure_message(path).is_empty()
				assert_array(Array(I18nRules.hardcoded_text(path, text))).override_failure_message(path).is_empty()
			else:
				assert_array(Array(ThemeRules.check_script(path, text))).override_failure_message(path).is_empty()


func test_grid_of_500_items_keeps_a_small_pool_while_scrolling() -> void:
	s.h.http.clear(F069Fixtures.ROUTE_INVENTORY)
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.many_items(100), "c2"))
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.many_items(100), "c3"))
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.many_items(100), "c4"))
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.many_items(100), "c5"))
	s.h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.many_items(100)))
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/inventory/InventoryShopScreen.tscn")
	await runner.simulate_frames(1)
	var screen: Control = runner.scene() as Control
	screen.call("open", {"tab": "wardrobe"})
	await runner.simulate_frames(20)
	var grid: VirtualItemGrid = screen.call("grid") as VirtualItemGrid
	assert_int(grid.entries.size()).is_equal(500)
	var rows_visible: int = ceili(grid.size.y / (grid.cell_size.y + float(UiTokens.SIZE_SPACE_8))) + VirtualItemGrid.BUFFER_ROWS
	assert_int(grid.pool_size()).is_less_equal(rows_visible * grid.columns)
	assert_int(grid.pool_size()).is_less(60)
	var pool: int = grid.pool_size()
	for step: int in 10:
		grid.scroll_vertical += int(grid.cell_size.y * 4.0)
		await runner.simulate_frames(1)
		assert_int(grid.pool_size()).is_equal(pool)
	assert_int(screen.find_children("Cell*", "Button", true, false).size()).is_equal(pool)


func test_panels_open_within_200_ms() -> void:
	for path: String in SCREENS:
		var runner: GdUnitSceneRunner = scene_runner(path)
		await runner.simulate_frames(1)
		var screen: Control = runner.scene() as Control
		var started: int = Time.get_ticks_msec()
		screen.call("open", {})
		await get_tree().process_frame
		var elapsed: int = Time.get_ticks_msec() - started
		assert_bool(screen.visible).override_failure_message(path).is_true()
		assert_int(elapsed).override_failure_message(path).is_less_equal(PANEL_OPEN_MAX_MS)


static func _scan(dir: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var d: DirAccess = DirAccess.open(dir)
	if d == null:
		return out
	for f: String in d.get_files():
		if f.ends_with(".gd") or f.ends_with(".tscn"):
			out.append(dir.path_join(f))
	for sub: String in d.get_directories():
		out.append_array(_scan(dir.path_join(sub)))
	return out
