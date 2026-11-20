extends GdUnitTestSuite
## F063-AC-06, SC-003 (TC-LRN-009; T011): an answer sent while offline stays in the RAM queue PendingWrites, the
## service reports `offline` (banner `ui.activity_player.offline`), and once the network is back the answer is
## re-sent in order with the same `Idempotency-Key`. Nothing of the attempt is written to `user://`.

var h: AuthHarness = null
var service: LearningService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = LearningService.new(h.api, null, h.clock)
	h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.fresh_attempt())
	await service.start({"activity_id": LearningFixtures.ACTIVITY_ID})


func after_test() -> void:
	h.dispose()


## Files of user:// with their modification time (engine logs and shader caches excluded).
static func _user_files(dir_path: String = "user://") -> PackedStringArray:
	var out: PackedStringArray = []
	if dir_path.get_file() in ["logs", "shader_cache", "vulkan"]:
		return out
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return out
	for f: String in dir.get_files():
		out.append(dir_path.path_join(f) + ":" + str(FileAccess.get_modified_time(dir_path.path_join(f))))
	for d: String in dir.get_directories():
		out.append_array(_user_files(dir_path.path_join(d)))
	return out


func test_offline_answers_are_queued_then_flushed_in_order_with_their_keys() -> void:
	var offline_events: Array[bool] = []
	service.offline_changed.connect(func(v: bool) -> void: offline_events.append(v))
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	assert_object(await service.respond(1, {"option_id": "o_b"}, 10)).is_null()
	assert_object(await service.respond(2, {"option_ids": ["o_a"]}, 10)).is_null()
	assert_bool(service.offline).is_true()
	assert_int(service.pending.size()).is_equal(2)
	# The second answer is queued behind the first without being sent (order kept).
	var keys: PackedStringArray = service.pending.keys()
	assert_int(keys.size()).is_equal(2)
	assert_str(String(h.http.last(LearningFixtures.ROUTE_RESPOND).headers["idempotency-key"])).is_equal(keys[0])
	var sent_before: int = h.http.count(LearningFixtures.ROUTE_RESPOND)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	var results: Array[ResponseResultModel] = await service.flush_pending()
	assert_array(results).has_size(2)
	assert_bool(service.offline).is_false()
	assert_int(service.pending.size()).is_equal(0)
	var replay: Array[FakeHttp.Recorded] = h.http.find(LearningFixtures.ROUTE_RESPOND).slice(sent_before)
	assert_array(replay).has_size(2)
	assert_int(int(replay[0].body["position"])).is_equal(1)
	assert_int(int(replay[1].body["position"])).is_equal(2)
	assert_str(String(replay[0].headers["idempotency-key"])).is_equal(keys[0])
	assert_str(String(replay[1].headers["idempotency-key"])).is_equal(keys[1])
	assert_array(offline_events).is_equal([true, false])


func test_queue_keeps_the_order_while_still_offline() -> void:
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	await service.respond(1, {"option_id": "o_b"}, 10)
	assert_array(await service.flush_pending()).is_empty()
	assert_int(service.pending.size()).is_equal(1)
	assert_bool(service.offline).is_true()


func test_nothing_is_written_to_disk() -> void:
	var before: PackedStringArray = _user_files()
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	await service.respond(5, {"text": "offline-answer-unique-4410"}, 10)
	await service.flush_pending()
	assert_array(Array(_user_files())).contains_exactly_in_any_order(Array(before))


func test_unkeyed_writes_are_refused() -> void:
	var queue: PendingWrites = PendingWrites.new()
	assert_bool(queue.enqueue(ApiRequest.make_post("/x", {}), 0)).is_false()
	var keyed: ApiRequest = ApiRequest.make_post("/x", {})
	keyed.idempotency_key = Uuid.v7()
	assert_bool(queue.enqueue(keyed, 0)).is_true()
	assert_bool(queue.enqueue(keyed, 1)).is_true()
	assert_int(queue.size()).is_equal(1)


func test_submit_waits_for_the_queued_answers() -> void:
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	await service.respond(1, {"option_id": "o_b"}, 10)
	assert_object(await service.submit()).is_null()
	assert_int(h.http.count(LearningFixtures.ROUTE_SUBMIT)).is_equal(0)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	assert_object(await service.submit()).is_not_null()
	assert_int(service.pending.size()).is_equal(0)
