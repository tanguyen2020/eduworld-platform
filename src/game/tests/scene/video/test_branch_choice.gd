extends GdUnitTestSuite
## F065-AC-24 (T035): at the time of a branch point the video stops and shows the prompt with its options; a choice
## sends `{kind: branch_choice, branch_point_id, option_id}`; the segment named by `next_segment_id` plays next; the end
## of the ending segment asks to submit.

var h: VideoRendererHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null
	QualityManager.suspend_reasons.clear()


func test_branch_prompt_choice_and_segment() -> void:
	h = await VideoRendererHarness.create(self, VideoFixtures.attempt(true))
	h.play_for(21000)
	var panel: Control = h.renderer.get_node("%Branch") as Control
	assert_bool(panel.visible).is_true()
	assert_bool(h.playback.playing).is_false()
	assert_str((h.renderer.get_node("%BranchPrompt") as Label).text).is_equal("Vịt đi đâu?")
	var buttons: Array[Node] = h.renderer.get_node("%BranchOptions").get_children()
	assert_int(buttons.size()).is_equal(2)
	assert_str((buttons[0] as Button).text).is_equal("Xuống nước")
	(buttons[0] as Button).pressed.emit()
	assert_dict(h.submissions[0]).is_equal({"kind": "branch_choice", "branch_point_id": "bp1", "option_id": "swim"})
	var result: ResponseResultModel = ResponseResultModel.from_dict({"accepted": true, "next_segment_id": "water"})
	h.renderer.show_result(result)
	for i: int in 5:
		await await_idle_frame()
	assert_bool(panel.visible).is_false()
	assert_str(h.renderer.segment_id).is_equal("water")
	assert_str(h.playback.loaded[h.playback.loaded.size() - 1]).ends_with("water_480.ogv")
	assert_int(h.playback.pos).is_equal(0)
	assert_bool(h.playback.playing).is_true()
	# No prompt again inside the segment; its end asks to submit.
	h.play_for(30000)
	assert_bool(panel.visible).is_false()
	h.playback.finish()
	assert_int(h.finished).is_equal(1)
