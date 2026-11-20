extends GdUnitTestSuite
## F065-AC-15, F065-AC-16, F065-AC-23 (T027): the variant follows the preset (480p on Low/Web, 720p otherwise) and
## drops to 480p after two stalls longer than 3 s at the same position; subtitles default to the UI language, cycle
## vi → en → off and are drawn in the Theme caption panel; opening the player suspends the world for "video".

var h: VideoRendererHarness = null
var _preset: String = ""
var _locale: String = ""


func before_test() -> void:
	_preset = QualityManager.preset
	_locale = TranslationServer.get_locale()
	TranslationServer.set_locale("vi")


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null
	QualityManager.preset = _preset
	QualityManager.suspend_reasons.clear()
	TranslationServer.set_locale(_locale)


func test_variant_follows_the_preset() -> void:
	var media: Array[Dictionary] = []
	for m: Variant in VideoFixtures.media():
		media.append(m as Dictionary)
	assert_str(String(VideoLessonRenderer.pick_variant(media, "", QualityPolicy.LOW, false, false)["variant"])).is_equal("ogv_480")
	assert_str(String(VideoLessonRenderer.pick_variant(media, "", QualityPolicy.MEDIUM, false, false)["variant"])).is_equal("ogv_720")
	assert_str(String(VideoLessonRenderer.pick_variant(media, "", QualityPolicy.HIGH, true, false)["variant"])).is_equal("ogv_480")
	assert_str(String(VideoLessonRenderer.pick_variant(media, "", QualityPolicy.HIGH, false, true)["variant"])).is_equal("ogv_480")
	assert_str(String(VideoLessonRenderer.pick_variant(media, "water", QualityPolicy.HIGH, false, false)["segment_id"])).is_equal("water")


func test_two_stalls_drop_to_480p_at_the_same_position() -> void:
	QualityManager.preset = QualityPolicy.HIGH
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	assert_str(h.renderer.variant).is_equal("ogv_720")
	assert_bool(QualityManager.suspend_reasons.has(&"video")).is_true()
	h.play_for(12000)
	h.playback.stall = true
	for i: int in 8:
		h.renderer._process(1.0)
	for i: int in 10:
		await await_idle_frame()
	assert_bool(h.renderer.downgraded).is_true()
	assert_str(h.renderer.variant).is_equal("ogv_480")
	assert_str(h.playback.loaded[h.playback.loaded.size() - 1]).ends_with("ogv_480.ogv")
	assert_int(h.playback.pos).is_equal(12000)
	h.renderer.teardown()
	assert_bool(QualityManager.suspend_reasons.has(&"video")).is_false()


func test_subtitles_default_to_ui_language_and_cycle() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	for i: int in 10:
		await await_idle_frame()
	assert_str(h.renderer.subtitle_language).is_equal("vi")
	h.play_for(1000)
	var line: Label = h.renderer.get_node("%Subtitle") as Label
	assert_str(line.text).is_equal("Xin chào các em!")
	assert_str(String((h.renderer.get_node("%SubtitlePanel") as PanelContainer).theme_type_variation)).is_equal("VoiceCaption")
	await h.renderer.cycle_subtitles()
	assert_str(h.renderer.subtitle_language).is_equal("en")
	h.play_for(1000)
	assert_str(line.text).is_equal("Hello everyone!")
	await h.renderer.cycle_subtitles()
	assert_str(h.renderer.subtitle_language).is_empty()
	assert_bool((h.renderer.get_node("%SubtitlePanel") as Control).visible).is_false()
