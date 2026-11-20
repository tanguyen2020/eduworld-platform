extends GdUnitTestSuite
## F066-AC-18, F066-AC-19 (TC-AUD-001; T035): "Lượt nghe: n/max", a new play is asked to the server (`play_started`)
## and starts only once accepted, at the limit the play button greys out with `ui.audio.plays_exhausted` and answering
## stays possible; a timed question pauses the audio; speed 0.75×/1× only when allowed; the transcript follows
## `show_transcript`; the world music is ducked while the lesson is open.

var h: AudioRendererHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null
	QualityManager.suspend_reasons.clear()


func test_play_waits_for_the_server_and_counts() -> void:
	h = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 0))
	assert_bool(AudioDirector.music_ducked()).is_true()
	var plays: Label = h.renderer.get_node("%Plays") as Label
	assert_bool(plays.visible).is_true()
	assert_str(plays.text).is_equal(tr("ui.audio.plays_left").format({"used": 0, "max": 2}))
	h.renderer.press_play()
	assert_bool(h.playback.playing).is_false()
	assert_bool((h.reports.back() as Dictionary).get("play_started", false)).is_true()
	assert_int(int((h.reports.back() as Dictionary)["position_ms"])).is_equal(0)
	assert_bool((h.renderer.get_node("%Play") as Button).disabled).is_true()
	h.accept(1)
	assert_bool(h.playback.playing).is_true()
	assert_int(h.playback.pos).is_equal(0)
	assert_str(plays.text).is_equal(tr("ui.audio.plays_left").format({"used": 1, "max": 2}))
	# Pause and resume inside the play are not plays.
	h.play_for(5000)
	var sent: int = h.reports.size()
	h.renderer.press_play()
	h.renderer.press_play()
	assert_bool(h.playback.playing).is_true()
	for r: Dictionary in h.reports.slice(sent):
		assert_bool(r.has("play_started")).is_false()


func test_exhausted_plays_grey_out_and_answers_stay_possible() -> void:
	h = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 1))
	h.renderer.press_play()
	h.accept(2)
	h.playback.finish()
	await await_idle_frame()
	# After the end the first unanswered question opens; the plays are used.
	assert_bool((h.renderer.get_node("%Overlay") as Control).visible).is_true()
	assert_bool(h.renderer.wants_action_bar()).is_true()
	var play: Button = h.renderer.get_node("%Play") as Button
	assert_bool(play.disabled).is_true()
	assert_bool((h.renderer.get_node("%Exhausted") as Control).visible).is_true()
	assert_str((h.renderer.get_node("%Exhausted") as Label).text).is_equal("ui.audio.plays_exhausted")
	# A third request refused by the server (another device used the play) keeps the audio silent.
	(h.renderer.get_node("%Overlay") as Control).visible = false
	h.renderer.plays.exhausted = false
	h.renderer.press_play()
	h.renderer.progress_refused(AudioPlayTracker.CODE_LIMIT)
	assert_bool(h.playback.playing).is_false()
	assert_bool(play.disabled).is_true()
	assert_str((h.renderer.get_node("%Toast") as Label).text).is_equal("ui.audio.plays_exhausted")


func test_timed_question_pauses_the_audio() -> void:
	h = await AudioRendererHarness.create(self, AudioFixtures.attempt(null, 0))
	assert_bool((h.renderer.get_node("%Plays") as Label).visible).is_false()
	h.renderer.press_play()
	h.accept(1, null)
	h.play_for(25000)
	assert_bool(h.playback.playing).is_false()
	assert_int(h.playback.pos).is_greater_equal(20000)
	assert_bool((h.renderer.get_node("%Overlay") as Control).visible).is_true()
	assert_int(h.renderer.current_position()).is_equal(1)


func test_speed_and_transcript_follow_the_version() -> void:
	h = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 0, "always", true))
	var speed: Button = h.renderer.get_node("%Speed") as Button
	assert_bool(speed.visible).is_true()
	h.renderer.toggle_speed()
	assert_float(h.playback.speed).is_equal(0.75)
	h.renderer.toggle_speed()
	assert_float(h.playback.speed).is_equal(1.0)
	assert_bool((h.renderer.get_node("%Transcript") as Button).visible).is_true()
	await h.renderer.toggle_transcript()
	assert_bool((h.renderer.get_node("%TranscriptPanel") as Control).visible).is_true()
	assert_str((h.renderer.get_node("%TranscriptText") as Label).text).is_equal(AudioFixtures.TRANSCRIPT)
	h.dispose()
	h = await AudioRendererHarness.create(self, AudioFixtures.attempt(2, 0, "never", false))
	assert_bool((h.renderer.get_node("%Speed") as Button).visible).is_false()
	assert_bool((h.renderer.get_node("%Transcript") as Button).visible).is_false()
	h.renderer.teardown()
	assert_bool(AudioDirector.music_ducked()).is_false()
