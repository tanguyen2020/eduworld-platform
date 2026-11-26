extends GdUnitTestSuite
## F099-AC-08…F099-AC-13 (STUDENT_GAME_UX_LEARNING §5, §6, §8): the friendly media players. Video and audio have a main
## button with icon + words that follows the player ("Phát" / "Tạm dừng" / "Xem lại", "Nghe" / "Tạm dừng" /
## "Nghe lại"), "Lùi 10 giây" through the usual seek rules, a big "Phát" on the paused picture; the MediaTrack is as tall
## as the band's touch target with chapter notches and question discs (?/✓); captions sit in the readable CaptionCard
## with the speaker written as words and the caption button says its state in words; the coach line follows the
## player; questions use the board look; the simulation shows its goals with a box + words, numbered steps, icon
## buttons and "Lượt thử: n/m". Scripted fakes (TEST-ONLY), no decoding.

var v: VideoRendererHarness = null
var a: AudioRendererHarness = null
var sim: SimulationRenderer = null
var _band: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	AppState.start_ui(get_tree().root)


func after_test() -> void:
	if v != null:
		v.dispose()
		v = null
	if a != null:
		a.dispose()
		a = null
	if sim != null:
		sim.teardown()
		sim.free()
		sim = null
	QualityManager.suspend_reasons.clear()
	AppState.set_grade_band(_band)


func _video(band: String = "g1_3", branched: bool = false) -> void:
	AppState.set_grade_band(band)
	v = await VideoRendererHarness.create(self, VideoFixtures.attempt(branched))
	v.renderer.theme = AppState.theme_service.current_theme
	v.renderer.size = Vector2(1232, 520)
	for i: int in 4:
		await await_idle_frame()


func _main() -> Button:
	return v.renderer.get_node("%PlayPause") as Button


# ---------- video: buttons (AC-08) ----------

func test_video_main_button_follows_the_player() -> void:
	await _video()
	assert_bool(v.playback.playing).is_true()
	assert_str(_main().text).is_equal(VideoLessonRenderer.PAUSE_KEY)
	assert_object(_main().icon).is_same(VideoLessonRenderer.ICON_PAUSE)
	assert_bool((v.renderer.get_node("%BigPlay") as Control).visible).is_false()
	v.renderer.toggle_play()
	assert_str(_main().text).is_equal(VideoLessonRenderer.PLAY_KEY)
	assert_object(_main().icon).is_same(VideoLessonRenderer.ICON_PLAY)
	var big: Button = v.renderer.get_node("%BigPlay") as Button
	assert_bool(big.visible).is_true()
	assert_str(big.text).is_equal(VideoLessonRenderer.PLAY_KEY)
	assert_object(big.icon).is_not_null()
	big.pressed.emit()
	assert_bool(v.playback.playing).is_true()


func test_video_replay_after_the_end() -> void:
	await _video("g4_5", true)
	v.playback.finish()
	assert_bool(v.renderer.ended).is_true()
	assert_str(_main().text).is_equal(VideoLessonRenderer.REPLAY_KEY)
	assert_str(v.renderer.coach_hint()).is_equal(VideoLessonRenderer.COACH_DONE_KEY)
	v.renderer.toggle_play()
	assert_int(v.playback.pos).is_equal(0)
	assert_bool(v.playback.playing).is_true()


func test_rewind_goes_back_ten_seconds_through_the_seek_rules() -> void:
	await _video()
	v.play_for(15000)
	var reports: int = v.reports.size()
	(v.renderer.get_node("%Rewind") as Button).pressed.emit()
	assert_int(v.playback.pos).is_equal(15000 - VideoLessonRenderer.REWIND_MS)
	assert_int(v.reports.size()).is_greater(reports)
	assert_str((v.renderer.get_node("%Rewind") as Button).text).is_equal("ui.media.back10")
	assert_object((v.renderer.get_node("%Rewind") as Button).icon).is_not_null()


func test_video_buttons_and_bar_reach_the_band_touch_target() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		await _video(band)
		var min_h: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
		for name: String in ["%PlayPause", "%Rewind", "%Captions", "%Chapters"]:
			var b: Control = v.renderer.get_node(name) as Control
			assert_float(b.size.y + 0.5).override_failure_message("%s %s" % [band, name]).is_greater_equal(min_h)
		assert_float((v.renderer.get_node("%Seek") as Control).size.y + 0.5).is_greater_equal(min_h)
		v.dispose()
		v = null


# ---------- video: progress bar (AC-09) ----------

func test_bar_marks_chapters_and_questions_by_shape() -> void:
	await _video()
	var track: MediaTrack = v.renderer.get_node("%Seek") as MediaTrack
	assert_bool(50000.0 in track.chapters).is_true()
	assert_int(track.markers.size()).is_equal(1)
	assert_float(float(track.markers[0]["at"])).is_equal(30000.0)
	assert_bool(bool(track.markers[0]["done"])).is_false()
	v.play_for(31000)
	assert_bool((v.renderer.get_node("%Overlay") as Control).visible).is_true()
	v.renderer.question.select_index(1)
	v.renderer.show_result(ResponseResultModel.from_dict(LearningFixtures.response_result_correct()))
	v.renderer.advance()
	assert_bool(bool(track.markers[0]["done"])).is_true()
	# The chapter now playing is named in words on the chapter button.
	assert_int((v.renderer.get_node("%Chapters") as OptionButton).selected).is_equal(0)


func test_bar_seek_keeps_the_watched_only_rule() -> void:
	await _video()
	v.play_for(5000)
	var track: MediaTrack = v.renderer.get_node("%Seek") as MediaTrack
	track.release_at(90000.0)
	assert_int(v.playback.pos).is_less_equal(5000)
	assert_str((v.renderer.get_node("%Toast") as Label).text).is_equal(VideoLessonRenderer.SEEK_LOCKED_KEY)
	assert_str(v.renderer.coach_hint()).is_equal(VideoLessonRenderer.COACH_SEEK_LOCKED_KEY)
	assert_float(track.value).is_less_equal(5000.0)


# ---------- video: captions (AC-10) ----------

func test_caption_card_writes_the_speaker_in_words() -> void:
	await _video("g4_5")
	v.play_for(6000)
	var panel: PanelContainer = v.renderer.get_node("%SubtitlePanel") as PanelContainer
	var line: Label = v.renderer.get_node("%Subtitle") as Label
	assert_bool(panel.visible).is_true()
	assert_str(String(panel.theme_type_variation)).is_equal("CaptionCard")
	assert_str(line.text).is_equal("Cô: Hôm nay mình học\nphép nhân.")
	assert_int(line.get_theme_font_size(&"font_size")).is_equal(int(UiTokens.DENSITY_FONT_STEM["g4_5"]))
	assert_bool(line.get_theme_color(&"font_color").is_equal_approx(UiTokens.COMPONENT_CAPTION_TEXT)).is_true()
	var box: StyleBoxFlat = panel.get_theme_stylebox(&"panel") as StyleBoxFlat
	assert_bool(box.bg_color.is_equal_approx(UiTokens.COMPONENT_CAPTION_BG)).is_true()
	var captions: Button = v.renderer.get_node("%Captions") as Button
	assert_str(captions.text).contains("VI")
	await v.renderer.cycle_subtitles()
	await v.renderer.cycle_subtitles()
	assert_str(captions.text).contains(tr("ui.media.off"))


func test_cue_without_speaker_keeps_its_words() -> void:
	var p: VttParser = VttParser.new()
	assert_bool(p.parse(VideoFixtures.VTT_VI)).is_true()
	assert_str(p.caption_at(1000)).is_equal("Xin chào các em!")
	assert_str(p.caption_at(6000)).is_equal("Cô: Hôm nay mình học\nphép nhân.")
	assert_str(p.text_at(6000)).is_equal("Hôm nay mình học\nphép nhân.")


# ---------- video: coach and questions (AC-11, AC-12) ----------

func test_video_coach_follows_the_player() -> void:
	await _video()
	var changed: Array[int] = [0]
	v.renderer.coach_hint_changed.connect(func() -> void: changed[0] += 1)
	assert_str(v.renderer.coach_hint()).is_equal(VideoLessonRenderer.COACH_WATCH_QUESTIONS_KEY)
	v.renderer.toggle_play()
	assert_str(v.renderer.coach_hint()).is_equal(VideoLessonRenderer.COACH_START_KEY)
	v.renderer.toggle_play()
	v.play_for(31000)
	assert_str(v.renderer.coach_hint()).is_empty()
	assert_int(changed[0]).is_greater_equal(3)
	# The question takes the stage on the board background; the bar and buttons step aside.
	assert_str(String((v.renderer.get_node("%Overlay") as PanelContainer).theme_type_variation)).is_equal("ActivityBoard")
	assert_bool((v.renderer.get_node("TrackRow") as Control).visible).is_false()
	assert_object(v.renderer.question.card).is_not_null()


func test_branch_prompt_uses_answer_tiles_and_a_coach_line() -> void:
	await _video("g4_5", true)
	v.play_for(21000)
	assert_bool((v.renderer.get_node("%Branch") as Control).visible).is_true()
	assert_str(v.renderer.coach_hint()).is_equal(VideoLessonRenderer.COACH_BRANCH_KEY)
	for b: Node in (v.renderer.get_node("%BranchOptions") as Node).get_children():
		assert_str(String((b as Button).theme_type_variation)).starts_with("AnswerTile")


func test_coach_words_are_kind() -> void:
	var keys: PackedStringArray = [VideoLessonRenderer.COACH_START_KEY, VideoLessonRenderer.COACH_WATCH_KEY,
		VideoLessonRenderer.COACH_WATCH_QUESTIONS_KEY, VideoLessonRenderer.COACH_SEEK_LOCKED_KEY,
		VideoLessonRenderer.COACH_DONE_KEY, VideoLessonRenderer.COACH_BRANCH_KEY, AudioLessonRenderer.COACH_START_KEY,
		AudioLessonRenderer.COACH_LISTEN_KEY, AudioLessonRenderer.COACH_PLAYS_DONE_KEY, AudioLessonRenderer.COACH_AGAIN_KEY,
		SimulationRenderer.COACH_START_KEY, SimulationRenderer.COACH_CHECKING_KEY, SimulationRenderer.COACH_REACHED_KEY,
		SimulationRenderer.COACH_NOT_REACHED_KEY, SimulationRenderer.COACH_RUNS_DONE_KEY]
	for key: String in keys:
		for locale: String in ["vi-VN", "en-US"]:
			var words: String = String(TranslationServer.get_translation_object(locale).get_message(key)).to_lower()
			assert_str(words).override_failure_message("%s %s" % [locale, key]).is_not_empty()
			for bad: String in ["sai rồi", "phạt", "giây nữa", "nhanh lên", "wrong", "penalty", "hurry", "seconds left"]:
				assert_str(words).override_failure_message("%s %s %s" % [locale, key, bad]).not_contains(bad)


# ---------- audio (AC-08, AC-09, AC-11) ----------

func test_audio_main_button_card_and_coach() -> void:
	AppState.set_grade_band("g1_3")
	a = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 0))
	a.renderer.theme = AppState.theme_service.current_theme
	var play: Button = a.renderer.get_node("%Play") as Button
	assert_str(play.text).is_equal(AudioLessonRenderer.LISTEN_KEY)
	assert_str(a.renderer.coach_hint()).is_equal(AudioLessonRenderer.COACH_START_KEY)
	assert_bool((a.renderer.get_node("%AudioCard") as Control).visible).is_true()
	assert_str((a.renderer.get_node("%SegmentTitle") as Label).text).is_equal("Chào hỏi")
	a.renderer.press_play()
	a.accept(1)
	assert_str(play.text).is_equal(AudioLessonRenderer.PAUSE_KEY)
	assert_str(a.renderer.coach_hint()).is_equal(AudioLessonRenderer.COACH_LISTEN_KEY)
	var track: MediaTrack = a.renderer.get_node("%Seek") as MediaTrack
	assert_int(track.markers.size()).is_equal(1)
	assert_float(float(track.markers[0]["at"])).is_equal(20000.0)
	a.play_for(5000)
	(a.renderer.get_node("%Rewind") as Button).pressed.emit()
	assert_int(a.playback.pos).is_equal(0)
	a.play_for(21000)
	assert_bool((a.renderer.get_node("%Overlay") as Control).visible).is_true()
	assert_str(String((a.renderer.get_node("%Overlay") as PanelContainer).theme_type_variation)).is_equal("ActivityBoard")
	assert_str(a.renderer.coach_hint()).is_empty()


func test_audio_listen_again_and_plays_done() -> void:
	a = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 1))
	var play: Button = a.renderer.get_node("%Play") as Button
	assert_str(play.text).is_equal(AudioLessonRenderer.LISTEN_AGAIN_KEY)
	assert_str(a.renderer.coach_hint()).is_equal(AudioLessonRenderer.COACH_AGAIN_KEY)
	a.dispose()
	a = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 2))
	assert_bool((a.renderer.get_node("%Play") as Button).disabled).is_true()
	assert_str(a.renderer.coach_hint()).is_equal(AudioLessonRenderer.COACH_PLAYS_DONE_KEY)


# ---------- simulation (AC-11, AC-13) ----------

func test_simulation_goals_steps_buttons_and_coach() -> void:
	sim = (load("res://scenes/learning/activity_renderers/simulation/SimulationRenderer.tscn") as PackedScene).instantiate() \
			as SimulationRenderer
	add_child(sim)
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	sim.setup(AttemptModel.from_dict(SimulationFixtures.attempt()), ctx)
	var goal: CheckBox = (sim.get_node("%Goals") as Control).get_child(0) as CheckBox
	# Readable, not greyed out: the box shape and the words carry the state.
	assert_bool(goal.disabled).is_false()
	assert_str(goal.text).is_equal("Làm cho đòn bẩy cân bằng")
	assert_str(((sim.get_node("%Steps") as Control).get_child(0) as Label).text).is_equal("1. Đặt hai vật lên đòn bẩy")
	assert_object((sim.get_node("%Release") as Button).icon).is_not_null()
	assert_object((sim.get_node("%Retry") as Button).icon).is_not_null()
	assert_str((sim.get_node("%Runs") as Label).text).is_equal(tr(SimulationRenderer.RUNS_KEY).format({"used": 0, "max": 3}))
	assert_str(sim.coach_hint()).is_equal(SimulationRenderer.COACH_START_KEY)
	sim.release()
	assert_str(sim.coach_hint()).is_equal(SimulationRenderer.COACH_CHECKING_KEY)
	assert_bool(goal.button_pressed).is_false()
	sim.show_run_result(SimulationFixtures.result(false, 1))
	assert_str(sim.coach_hint()).is_equal(SimulationRenderer.COACH_NOT_REACHED_KEY)
	sim.retry()
	sim.release()
	sim.show_run_result(SimulationFixtures.result(true, 2))
	assert_bool(goal.button_pressed).is_true()
	assert_str(sim.coach_hint()).is_equal(SimulationRenderer.COACH_REACHED_KEY)
