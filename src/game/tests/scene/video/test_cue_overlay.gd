extends GdUnitTestSuite
## F065-AC-17, F065-AC-18, F065-AC-19, F065-AC-21 (TC-VID-001/002; T028): the video stops at the cue time, opens the
## F063 question renderer with no skip, sends `{kind: cue_answer, cue_id, position, response}` and plays on after the
## feedback; heartbeats go out while playing and on pause; the seek bar refuses the unwatched part; at the end a pending
## required cue brings the video back to it, otherwise the renderer asks to submit.

var h: VideoRendererHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null
	QualityManager.suspend_reasons.clear()


func test_cue_stops_the_video_and_sends_a_cue_answer() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	assert_bool(h.renderer.wants_action_bar()).is_false()
	h.play_for(29000)
	assert_bool(h.playback.playing).is_true()
	assert_int(h.reports.size()).is_greater_equal(2)
	h.play_for(2000)
	assert_bool(h.playback.playing).is_false()
	assert_bool((h.renderer.get_node("%Overlay") as Control).visible).is_true()
	assert_bool(h.renderer.wants_action_bar()).is_true()
	assert_array(h.shown).is_equal([1])
	assert_object(h.renderer.question).is_not_null()
	assert_int(h.renderer.find_children("*Skip*", "Button", true, false).size()).is_equal(0)
	h.renderer.select_index(1)
	assert_bool(h.renderer.is_answer_complete()).is_true()
	assert_bool(h.renderer.check()).is_true()
	var sub: Dictionary = h.submissions[0]
	assert_str(String(sub["kind"])).is_equal("cue_answer")
	assert_str(String(sub["cue_id"])).is_equal("q1")
	assert_int(int(sub["position"])).is_equal(1)
	assert_dict(sub["response"] as Dictionary).is_equal({"option_id": "o_b"})
	var right: ResponseResultModel = ResponseResultModel.from_dict(LearningFixtures.response_result_correct())
	h.renderer.show_result(right)
	assert_bool(h.renderer.advance()).is_true()
	assert_bool((h.renderer.get_node("%Overlay") as Control).visible).is_false()
	assert_bool(h.playback.playing).is_true()
	assert_bool("q1" in h.renderer.pending_cues).is_false()


func test_heartbeats_and_seek_lock() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	h.play_for(10000)
	var last: Dictionary = h.reports[h.reports.size() - 1]
	assert_int(int(last["position_ms"])).is_equal(10000)
	assert_array(last["watched_ranges"] as Array).is_equal([{"start_ms": 0, "end_ms": 10000}])
	h.renderer.toggle_play()
	assert_bool(h.playback.playing).is_false()
	var reached: int = h.renderer.request_seek(90000)
	assert_int(reached).is_equal(10000)
	assert_bool((h.renderer.get_node("%Toast") as Label).visible).is_true()
	assert_str((h.renderer.get_node("%Toast") as Label).text).is_equal("ui.video.seek_locked")
	assert_int(h.renderer.request_seek(4000)).is_equal(4000)


func test_end_returns_to_a_pending_required_cue_then_submits() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	# The cue was answered wrong earlier: the server still lists it as pending.
	h.renderer.answered_cues.append("q1")
	h.renderer.apply_progress({"watched_ranges": [{"start_ms": 0, "end_ms": 100000}], "pending_cue_ids": ["q1"]})
	h.playback.playing = true
	h.playback.finish()
	assert_int(h.finished).is_equal(0)
	assert_int(h.playback.pos).is_equal(30000)
	assert_bool((h.renderer.get_node("%Overlay") as Control).visible).is_true()
	h.renderer.apply_progress({"pending_cue_ids": []})
	h.renderer.ended = true
	assert_bool(h.renderer.advance()).is_false()
	assert_int(h.finished).is_equal(1)


func test_end_without_pending_cue_submits() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt())
	h.renderer.apply_progress({"pending_cue_ids": []})
	h.playback.playing = true
	h.playback.finish()
	assert_int(h.finished).is_equal(1)
	assert_bool(h.renderer.is_last()).is_true()
	var last: Dictionary = h.reports[h.reports.size() - 1]
	assert_int(int(last["position_ms"])).is_equal(100000)
