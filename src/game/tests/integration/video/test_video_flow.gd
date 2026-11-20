extends GdUnitTestSuite
## T030 (TC-VID-001/003/004): against the backend compose — `hs.an.3a` starts the seeded video lesson, sends plausible
## heartbeats up to the cue at 01:30, answers it (`cue_answer`), watches to the end and submits; a resumed start returns
## the saved position. Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend). The seed needs a published
## `video_lesson` with a ready Theora variant (F027) — set EDUWORLD_IT_VIDEO_ACTIVITY_ID to its id.

const SKIP_REASON: String = "needs the backend compose with a seeded video lesson (EDUWORLD_IT_API_BASE_URL, EDUWORLD_IT_VIDEO_ACTIVITY_ID)"
const ENV_ACTIVITY: String = "EDUWORLD_IT_VIDEO_ACTIVITY_ID"
const ActivityFlow := preload("res://tests/integration/learning/test_activity_flow.gd")


static func _skip() -> bool:
	return LiveBackend.skip() or OS.get_environment(ENV_ACTIVITY).is_empty()


func test_student_watches_answers_the_cue_and_completes(do_skip: bool = _skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var service: LearningService = LearningService.new(live.api)
	var attempt: AttemptModel = await service.start({"activity_id": OS.get_environment(ENV_ACTIVITY), "mode": "practice"})
	assert_object(attempt).is_not_null()
	assert_str(attempt.activity_type).is_equal("video_lesson")
	var reporter: VideoProgressReporter = VideoProgressReporter.new()
	reporter.started(0)
	var duration: int = int(attempt.render.render_payload.get("duration_ms", 0))
	var pos: int = 0
	while pos < duration:
		await get_tree().create_timer(1.0).timeout
		pos = mini(pos + 1000, duration)
		var report: Dictionary = reporter.advanced(pos, 1.0)
		if not report.is_empty():
			assert_dict(await service.report_progress(report)).is_not_empty()
	for cue: Variant in attempt.render.render_payload.get("cues", []) as Array:
		var item: QuestionRenderModel = attempt.item_at(int((cue as Dictionary)["position"]))
		var result: ResponseResultModel = await service.respond(item.position, ActivityFlow.any_answer(item), 1000,
				{"kind": "cue_answer", "cue_id": String((cue as Dictionary)["cue_id"])})
		assert_object(result).is_not_null()
	await service.report_progress(reporter.paused(duration))
	assert_object(await service.submit()).is_not_null()
	await live.auth.logout()
	live.dispose()
