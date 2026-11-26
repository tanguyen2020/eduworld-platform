extends GdUnitTestSuite
## F063-AC-04 (T010): question text keeps its Vietnamese diacritics with the Theme font, item images are loaded from
## their media URL, `g1_3` plays the stem voice-over automatically and, when the sound is off (Voice bus muted) or
## there is no recording, always shows the caption with a muted icon.

const VOICE_SCENE: String = "res://scenes/learning/VoiceOverPlayer.tscn"
const VIETNAMESE: String = "Bạn Nam có 3 túi, mỗi túi 6 quả cam. Nam có tất cả bao nhiêu quả cam?"

var _voice_bus: int = -1


func before_test() -> void:
	_voice_bus = AudioServer.get_bus_index(&"Voice")


func after_test() -> void:
	AudioServer.set_bus_mute(_voice_bus, false)


func _tone() -> AudioStreamWAV:
	var wav: AudioStreamWAV = AudioStreamWAV.new()
	wav.format = AudioStreamWAV.FORMAT_8_BITS
	wav.mix_rate = 8000
	var data: PackedByteArray = PackedByteArray()
	data.resize(8000)
	wav.data = data
	return wav


func _voice() -> PanelContainer:
	var v: PanelContainer = auto_free((load(VOICE_SCENE) as PackedScene).instantiate()) as PanelContainer
	add_child(v)
	return v


func test_stem_shows_vietnamese_text_with_the_theme_font() -> void:
	var view: RichContentView = auto_free((load("res://scenes/learning/RichContentView.tscn") as PackedScene).instantiate()) as RichContentView
	add_child(view)
	var q: QuestionRenderModel = LearningFixtures.question("single_choice")
	var ctx: RendererContext = RendererContext.new()
	view.show_content(q.stem(), ctx, q)
	var label: Label = view.get_child(0) as Label
	assert_str(label.text).is_equal(VIETNAMESE)
	var font: Font = label.get_theme_font("font")
	for c: String in "ạăâđêôơưốềủữ":
		assert_bool(font.has_char(c.unicode_at(0))).override_failure_message(c).is_true()


func test_item_image_is_loaded_from_its_media_url() -> void:
	var http: FakeHttp = FakeHttp.new()
	var png: Image = Image.create(8, 4, false, Image.FORMAT_RGBA8)
	png.fill(Color.WHITE)
	http.reply_bytes("GET https://cdn.example.invalid/media/*", 200, png.save_png_to_buffer())
	var ctx: RendererContext = RendererContext.new()
	ctx.media = MediaLoader.new(http)
	var q: QuestionRendererBase = auto_free(QuestionRendererRegistry.instantiate("image_based")) as QuestionRendererBase
	add_child(q)
	q.setup(LearningFixtures.question("image_based"), ctx)
	for i: int in 10:
		await await_idle_frame()
	var image: TextureRect = q.get("image") as TextureRect
	assert_object(image.texture).is_not_null()
	assert_int(image.texture.get_width()).is_equal(8)
	assert_str(http.requests[0].url).contains("/bags.png")
	assert_bool(http.requests[0].headers.has("authorization")).is_false()


func test_g1_3_plays_the_voice_over_automatically() -> void:
	var v: PanelContainer = _voice()
	v.set("always_caption", true)
	v.call("present", VIETNAMESE, _tone(), true)
	assert_bool(bool(v.call("is_playing"))).is_true()
	assert_bool(bool(v.call("is_caption_visible"))).is_true()
	assert_str(String(v.call("icon_name"))).is_equal("sound_on")


func test_older_bands_do_not_autoplay_but_can_replay() -> void:
	var v: PanelContainer = _voice()
	v.call("present", VIETNAMESE, _tone(), false)
	assert_bool(bool(v.call("is_playing"))).is_false()
	assert_bool(bool(v.call("is_caption_visible"))).is_false()
	v.call("replay")
	assert_bool(bool(v.call("is_playing"))).is_true()


func test_muted_sound_always_shows_caption_and_muted_icon() -> void:
	AudioServer.set_bus_mute(_voice_bus, true)
	var v: PanelContainer = _voice()
	v.call("present", VIETNAMESE, _tone(), true)
	assert_bool(bool(v.call("is_playing"))).is_false()
	assert_bool(bool(v.call("is_caption_visible"))).is_true()
	assert_str(String(v.call("icon_name"))).is_equal("sound_off")


func test_no_recording_shows_caption() -> void:
	var v: PanelContainer = _voice()
	v.call("present", VIETNAMESE, null, true)
	assert_bool(bool(v.call("is_caption_visible"))).is_true()
	assert_str(String(v.call("icon_name"))).is_equal("sound_off")


## F097-AC-04 (F096-3): when the renderer shows the stem in its question card, the voice-over never repeats it as a
## caption — in any band, muted or not, with or without a recording — and still plays the clip.
func test_stem_on_screen_never_repeats_the_stem_as_caption() -> void:
	AudioServer.set_bus_mute(_voice_bus, true)
	var v: PanelContainer = _voice()
	v.set("always_caption", true)
	v.set("stem_on_screen", true)
	v.call("present", VIETNAMESE, null, true)
	assert_bool(bool(v.call("is_caption_visible"))).is_false()
	assert_bool(v.visible).is_false()
	AudioServer.set_bus_mute(_voice_bus, false)
	v.call("present", VIETNAMESE, _tone(), true)
	assert_bool(v.visible).is_false()
	assert_bool(bool(v.call("is_playing"))).is_true()
