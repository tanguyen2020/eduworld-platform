extends GdUnitTestSuite
## F060-AC-14 (NFR-I18N-001, TC-I18N-004 frame part, SC-002): changing the language calls
## TranslationServer.set_locale, emits EventBus.locale_changed and updates the open screen without restart.

const GALLERY: String = "res://scenes/ui/debug/UiGallery.tscn"

var _locale: String = ""


func before_test() -> void:
	_locale = String(Settings.locale)


func after_test() -> void:
	Settings.set_locale(_locale, false)


func test_switch_updates_open_screen_within_a_second() -> void:
	Settings.set_locale("vi-VN", false)
	var runner: GdUnitSceneRunner = scene_runner(GALLERY)
	await runner.simulate_frames(3)
	var gallery: Node = runner.scene()
	var title: Label = gallery.get_node("Scroll/Content/Title") as Label
	var sample: Label = gallery.get_node("%FormatSample") as Label
	var vi_title: String = title.tr(title.text)
	var vi_sample: String = sample.text
	assert_str(vi_sample).contains("1.234,5")
	var events: Array[String] = []
	var on_locale: Callable = func(locale: String) -> void: events.append(locale)
	EventBus.locale_changed.connect(on_locale)
	var started: int = Time.get_ticks_msec()
	assert_bool(Settings.set_locale("en-US", false)).is_true()
	await runner.simulate_frames(1)
	var elapsed: int = Time.get_ticks_msec() - started
	EventBus.locale_changed.disconnect(on_locale)
	assert_array(events).is_equal(["en-US"])
	assert_str(TranslationServer.get_locale()).starts_with("en")
	assert_str(title.tr(title.text)).is_not_equal(vi_title)
	assert_str(sample.text).contains("1,234.5")
	assert_str(sample.text).contains("Nov 15, 2026")
	assert_object(runner.scene()).is_same(gallery)
	assert_int(elapsed).is_less(1000)
