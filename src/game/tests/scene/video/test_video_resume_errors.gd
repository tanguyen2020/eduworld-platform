extends GdUnitTestSuite
## F065-AC-20, F065-AC-22 (TC-VID-003; T029): a resumed attempt asks "Xem tiếp từ mm:ss?" and seeks there when the
## student agrees; a refused heartbeat (VIDEO_SEEK_NOT_ALLOWED) sends the video back to the watched frontier; start with
## `422 VIDEO_NOT_READY` shows `ui.video.not_ready` and no renderer.

var h: VideoRendererHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null
	QualityManager.suspend_reasons.clear()


func test_resume_prompt_seeks_to_the_saved_position() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt(false, 192000))
	var prompt: Control = h.renderer.get_node("%Resume") as Control
	assert_bool(prompt.visible).is_true()
	assert_str((h.renderer.get_node("%ResumeLabel") as Label).text).is_equal(tr("ui.video.resume_prompt").format({"time": "03:12"}))
	assert_bool(h.playback.playing).is_false()
	(h.renderer.get_node("%ResumeYes") as Button).pressed.emit()
	for i: int in 5:
		await await_idle_frame()
	assert_bool(prompt.visible).is_false()
	assert_int(h.playback.pos).is_equal(192000)
	assert_bool(h.playback.playing).is_true()


func test_refused_seek_goes_back_to_the_frontier() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	h.renderer.apply_progress({"watched_ranges": [{"start_ms": 0, "end_ms": 25000}], "pending_cue_ids": ["q1"]})
	h.playback.pos = 60000
	h.renderer.progress_refused(LearningService.CODE_SEEK_NOT_ALLOWED)
	assert_int(h.playback.pos).is_equal(25000)
	assert_str((h.renderer.get_node("%Toast") as Label).text).is_equal("ui.video.seek_locked")


func test_video_not_ready_shows_its_message() -> void:
	var auth: AuthHarness = AuthHarness.create()
	await auth.sign_in()
	var host: Control = auto_free((load("res://scenes/learning/ActivityHost.tscn") as PackedScene).instantiate()) as Control
	host.set("services", ServiceLocator.new(auth.api, AppState, auth.clock))
	host.set("client_version", "1.2.3")
	add_child(host)
	auth.http.reply(LearningFixtures.ROUTE_START, 422, LearningFixtures.problem(422, "VIDEO_NOT_READY"))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	assert_object(host.get("renderer")).is_null()
	assert_str((host.get_node("%StateLabel") as Label).text).is_equal(tr("ui.video.not_ready"))
	assert_bool((host.get_node("%Retry") as Button).visible).is_true()
	host.call("close")
	auth.dispose()
