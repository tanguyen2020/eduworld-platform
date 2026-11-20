extends GdUnitTestSuite
## T016 (F063-AC-01/05/08/11, TC-TEN-001, TC-LRN-002): against the backend compose — `hs.an.3a` opens the first
## activity assigned to 3A, answers every item with a well-formed response of its type (built through the answer
## models, never knowing the key), submits, and loads the review; `hs.dung.7a` (other tenant) asking for that attempt
## gets `404` and no data. Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend). Needs F030/F032 + F041 seed.

const SKIP_REASON: String = "needs the backend compose with F030/F032/F041 (EDUWORLD_IT_API_BASE_URL)"


## A well-formed answer for any item (first choices / everything placed) — shape only, never the right answer.
static func any_answer(item: QuestionRenderModel) -> Dictionary:
	var model: AnswerModel = null
	match item.type:
		"single_choice":
			model = SingleChoiceAnswer.new()
			model.setup(item.render_payload)
			(model as SingleChoiceAnswer).select_index(0)
		"multiple_choice":
			model = MultipleChoiceAnswer.new()
			model.setup(item.render_payload)
			(model as MultipleChoiceAnswer).select_index(0)
		"true_false":
			model = TrueFalseAnswer.new()
			model.setup(item.render_payload)
			var tf: TrueFalseAnswer = model as TrueFalseAnswer
			if tf.has_statements():
				for id: String in tf.statements:
					tf.set_statement(id, true)
			else:
				tf.set_value(true)
		"fill_in_blank":
			model = FillInBlankAnswer.new()
			model.setup(item.render_payload)
			for id: String in (model as FillInBlankAnswer).blank_ids():
				(model as FillInBlankAnswer).set_blank(id, "1")
		"short_answer":
			model = ShortAnswerAnswer.new()
			model.setup(item.render_payload)
			(model as ShortAnswerAnswer).set_text("a")
		"numeric":
			model = NumericAnswer.new()
			model.setup(item.render_payload)
			(model as NumericAnswer).set_text("1")
		"matching":
			model = MatchingAnswer.new()
			model.setup(item.render_payload)
			var m: MatchingAnswer = model as MatchingAnswer
			for i: int in mini(m.left.size(), m.right.size()):
				m.pair(m.left[i], m.right[i])
		"ordering":
			model = OrderingAnswer.new()
			model.setup(item.render_payload)
			(model as OrderingAnswer).move_to(0, 0)
		"drag_drop":
			model = DragDropAnswer.new()
			model.setup(item.render_payload)
			var d: DragDropAnswer = model as DragDropAnswer
			for id: String in d.draggables:
				for zone: String in d.zones:
					if d.place(id, zone):
						break
		"image_based":
			model = ImageBasedAnswer.new()
			model.setup(item.render_payload)
			(model as ImageBasedAnswer).add_point(Vector2.ONE)
		"interactive":
			model = InteractiveAnswer.new()
			model.setup(item.render_payload)
			(model as InteractiveAnswer).set_state("value", 0)
	return model.to_response() if model != null else {}


func test_student_completes_an_assigned_activity_and_others_cannot_read_it(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var assignments: AssignmentService = AssignmentService.new(live.api)
	assert_bool(await assignments.refresh()).is_true()
	var intent: Dictionary = {}
	assignments.intent_sink = func(_screen: StringName, args: Dictionary) -> void: intent = args
	for a: MyAssignmentModel in assignments.assignments:
		if a.content_kind == MyAssignmentModel.KIND_ACTIVITY and a.status in ["not_started", "in_progress"]:
			assignments.start(a)
			break
	assert_dict(intent).is_not_empty()
	var service: LearningService = LearningService.new(live.api)
	var attempt: AttemptModel = await service.start(intent)
	assert_object(attempt).is_not_null()
	for item: QuestionRenderModel in attempt.items:
		if attempt.saved.has(item.position):
			continue
		var result: ResponseResultModel = await service.respond(item.position, any_answer(item), 1000)
		assert_object(result).override_failure_message("%s %s" % [item.type, service.last_error_code]).is_not_null()
	var submitted: AttemptModel = await service.submit()
	assert_object(submitted).is_not_null()
	var attempt_id: String = submitted.attempt_id
	await live.auth.logout()
	live.dispose()
	var other: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await other.auth.login("hs.dung.7a", LiveBackend.student_password())
	var intruder: LearningService = LearningService.new(other.api)
	assert_object(await intruder.resume(attempt_id)).is_null()
	assert_str(intruder.last_error_code).is_equal("NOT_FOUND")
	assert_object(intruder.attempt).is_null()
	await other.auth.logout()
	other.dispose()
