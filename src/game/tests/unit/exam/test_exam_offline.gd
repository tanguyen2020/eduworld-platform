extends GdUnitTestSuite
## F064-AC-14 (TC-LRN-009; T017): while the network is down the runner shows the offline banner, the countdown keeps
## going on the synced server offset, each answer is kept in the RAM `PendingWrites` — only the latest one per
## position — and sent again (with `answered_at_client` on the server clock and the exam device token) once the
## network is back; nothing of the exam or the answers is written under `user://`. FakeHttp/FakeClock (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"
const MARKER: String = "o_offline_marker_f064"

var h: AuthHarness = null
var service: ExamService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = ExamService.new(h.api, h.clock)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _start() -> void:
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await service.start(ExamFixtures.SESSION_ID)


func test_answers_wait_in_ram_latest_per_position() -> void:
	await _start()
	var offline: Array[bool] = []
	service.offline_changed.connect(func(v: bool) -> void: offline.append(v))
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_bool(service.offline).is_true()
	await service.save_answer(1, {"option_id": "o_b"})
	await service.save_answer(3, {"value": true})
	assert_int(service.pending.size()).is_equal(2)
	# Only the first answer was tried; the others went straight to the queue.
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(1)
	assert_bool(service.is_answered(1)).is_true()
	assert_array(offline).is_equal([true])


func test_countdown_keeps_running_while_offline() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": "o_a"})
	var before: int = service.remaining_ms()
	h.clock.advance(120.0)
	assert_int(service.remaining_ms()).is_equal(before - 120_000)


func test_network_back_replays_the_latest_answers_with_their_time() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": "o_a"})
	h.clock.advance(30.0)
	await service.save_answer(1, {"option_id": "o_b"})
	await service.save_answer(3, {"value": true})
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	h.clock.advance(float(ExamService.OFFLINE_RETRY_MS) / 1000.0)
	service.tick()
	for i: int in 20:
		await await_idle_frame()
	assert_bool(service.offline).is_false()
	assert_int(service.pending.size()).is_equal(0)
	var saves: Array[FakeHttp.Recorded] = h.http.find(ExamFixtures.ROUTE_SAVE)
	assert_int(saves.size()).is_equal(3)
	var first_replay: FakeHttp.Recorded = saves[1]
	assert_str(first_replay.path).ends_with("/answers/1")
	assert_dict(first_replay.body["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_str(String(first_replay.headers.get("x-exam-device-token", ""))).is_equal(AuthHarness.DEVICE_ID)
	# The server clock at the time of the answer (server_time + 30 s elapsed), never the device clock.
	assert_str(String(first_replay.body.get("answered_at_client", ""))).is_equal("2026-11-20T01:30:30Z")
	assert_str(saves[2].path).ends_with("/answers/3")
	for key: String in ["score", "is_correct", "client_elapsed_ms"]:
		assert_bool(first_replay.body.has(key)).override_failure_message(key).is_false()


func test_no_retry_storm_while_still_offline() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": "o_a"})
	service.tick()
	service.tick()
	await await_idle_frame()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(1)


func test_nothing_is_written_to_user_storage() -> void:
	var before: PackedStringArray = _user_files("user://")
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": MARKER})
	await service.save_answer(5, {"text": MARKER})
	var after: PackedStringArray = _user_files("user://")
	for path: String in after:
		if not path in before:
			assert_str(FileAccess.get_file_as_string(path)).override_failure_message(path).not_contains(MARKER)
	for path: String in after:
		assert_str(FileAccess.get_file_as_string(path)).override_failure_message(path).not_contains(MARKER)


func test_runner_shows_the_offline_banner() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	var lobby: Control = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	lobby.set("grade_band", "g4_5")
	add_child(lobby)
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session())
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID})
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	var banner: Label = runner.find_child("Offline", true, false) as Label
	assert_bool(banner.visible).is_false()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	(runner.get("question") as QuestionRendererBase).select_index(0)
	for i: int in 10:
		await await_idle_frame()
	assert_bool(banner.visible).is_true()
	assert_str(tr(banner.text)).is_equal(tr("ui.exam.offline"))
	var clock_before: String = ((runner.find_child("Clock", true, false) as Control).call("time_text") as String)
	h.clock.advance(61.0)
	runner.call("_process", 1.0)
	assert_str((runner.find_child("Clock", true, false) as Control).call("time_text") as String).is_not_equal(clock_before)


static func _user_files(dir_path: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return out
	for file: String in dir.get_files():
		out.append(dir_path.path_join(file))
	for sub: String in dir.get_directories():
		out.append_array(_user_files(dir_path.path_join(sub)))
	return out
