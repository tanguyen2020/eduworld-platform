extends GdUnitTestSuite
## F073-AC-23, F073-AC-24 (T034): MiniGameHost shows the countdown from `deadline_at` and `server_time` (local clock
## offset, never paused offline), queues an answer lost to the network and re-sends it with the SAME Idempotency-Key,
## and in grades 1–5 every touchable control is at least 64 px, each verdict comes with a sound and an icon + text
## (colour is never the only signal) and mouse/touch (button press) and keyboard (1–6, Enter) both play. Also the
## embedded path: a quest/assignment `minigame` attempt in the activity player, from first answer to the result screen.
## FakeHttp/FakeClock with the F073 fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/minigames/MiniGameHost.tscn"
const ACTIVITY_HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"
## `deadline_at` − `server_time` of the non-timed fixtures (02:08:00 − 02:05:00).
const FIXTURE_TIME_MS: int = 180000

var h: AuthHarness = null
var services: ServiceLocator = null
var host: MiniGameHost = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _open(type: String, band: String = "g4_5", data: Dictionary = {}) -> MiniGameHost:
	host = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as MiniGameHost
	host.services = services
	host.grade_band = band
	add_child(host)
	h.http.reply(MiniGameFixtures.ROUTE_START, 201, data if not data.is_empty() else MiniGameFixtures.session(type))
	await host.open({"minigame_config_id": MiniGameFixtures.CONFIG_ID})
	return host


func _scene() -> MiniGameQuestionScene:
	return host.scene as MiniGameQuestionScene


## Waits until the host is no longer waiting for the server (FakeHttp answers after a frame, retries included).
func _settle(max_frames: int = 240) -> void:
	for i: int in max_frames:
		await await_idle_frame()
		if not host.state in [MiniGameHost.State.AWAITING_RESULT, MiniGameHost.State.SUBMITTING] or host.get("_submit_waiting"):
			return


func _keys(route: String) -> Array[String]:
	var out: Array[String] = []
	for rec: FakeHttp.Recorded in h.http.find(route):
		out.append(String(rec.headers.get("idempotency-key", "")))
	return out


# ---------- F073-AC-23: server clock ----------

func test_countdown_follows_the_server_clock_offset() -> void:
	await _open("matching")
	assert_int(_scene().remaining_ms).is_equal(FIXTURE_TIME_MS)
	var clock: Label = host.find_child("Clock", true, false) as Label
	assert_bool(clock.visible).is_true()
	assert_str(clock.text).is_equal("⏳ 03:00")
	h.clock.advance(30.0)
	host.tick_clock()
	assert_int(_scene().remaining_ms).is_equal(FIXTURE_TIME_MS - 30000)
	assert_str(clock.text).is_equal("⏳ 02:30")


func test_countdown_ignores_the_device_wall_clock_and_uses_server_time() -> void:
	# The device clock is far from the server's: only the elapsed local time counts, from `server_time`.
	var data: Dictionary = MiniGameFixtures.session("ordering")
	data["server_time"] = "2026-11-20T02:07:00Z"
	await _open("ordering", "g4_5", data)
	assert_int(_scene().remaining_ms).is_equal(60000)


func test_countdown_keeps_running_while_offline() -> void:
	await _open("matching")
	var t0: float = h.clock.unix_time()
	h.http.fail(MiniGameFixtures.ROUTE_RESPOND)
	MiniGameFixtures.answer(_scene())
	(_scene().find_child("Done", true, false) as Button).pressed.emit()
	await _settle()
	assert_bool(services.learning.offline).is_true()
	h.clock.advance(45.0)
	host.tick_clock()
	var expected: int = FIXTURE_TIME_MS - int(round((h.clock.unix_time() - t0) * 1000.0))
	assert_int(_scene().remaining_ms).is_equal(expected)
	assert_int(_scene().remaining_ms).is_less(FIXTURE_TIME_MS - 45000 + 1)
	assert_bool((host.find_child("Offline", true, false) as Label).visible).is_true()


func test_time_up_locks_the_game_and_submits() -> void:
	await _open("matching")
	h.http.reply(MiniGameFixtures.ROUTE_SUBMIT, 200, MiniGameFixtures.submitted("matching", 0, false))
	h.clock.advance(181.0)
	await host.tick_clock()
	await _settle()
	assert_bool(_scene().locked).is_true()
	assert_int(h.http.count(MiniGameFixtures.ROUTE_SUBMIT)).is_equal(1)
	assert_int(host.state).is_equal(MiniGameHost.State.FINISHED)


# ---------- F073-AC-23: queue and re-send with the same key ----------

func test_answer_lost_offline_is_resent_with_the_same_key() -> void:
	await _open("matching")
	h.http.fail(MiniGameFixtures.ROUTE_RESPOND)
	MiniGameFixtures.answer(_scene())
	(_scene().find_child("Done", true, false) as Button).pressed.emit()
	await _settle()
	assert_bool(services.learning.offline).is_true()
	assert_int(services.learning.pending.size()).is_equal(1)
	assert_int(host.state).is_equal(MiniGameHost.State.FEEDBACK)
	assert_int(_scene().effects_played).is_equal(0)
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true, 10, 1))
	await host.retry_pending()
	var keys: Array[String] = _keys(MiniGameFixtures.ROUTE_RESPOND)
	assert_int(keys.size()).is_greater_equal(2)
	for key: String in keys:
		assert_str(key).is_equal(keys[0])
	assert_bool(Uuid.is_v7(keys[0])).is_true()
	assert_bool(services.learning.offline).is_false()
	assert_int(_scene().effects_played).is_equal(1)
	assert_str((h.http.last(MiniGameFixtures.ROUTE_RESPOND).body["response"] as Dictionary).keys()[0]).is_equal("pairs")


func test_each_answer_gets_its_own_key_and_no_score_is_sent() -> void:
	await _open("multiple_choice")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true, 10, 1))
	host.select_index(1)
	await _settle()
	host.press_primary()
	await await_idle_frame()
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(2, false))
	MiniGameFixtures.answer(_scene())
	(_scene().find_child("Done", true, false) as Button).pressed.emit()
	await _settle()
	var keys: Array[String] = _keys(MiniGameFixtures.ROUTE_RESPOND)
	assert_int(keys.size()).is_equal(2)
	assert_str(keys[0]).is_not_equal(keys[1])
	for rec: FakeHttp.Recorded in h.http.find(MiniGameFixtures.ROUTE_RESPOND):
		for forbidden: String in ["score", "is_correct", "game_points", "streak", "points"]:
			assert_bool(rec.body.has(forbidden)).override_failure_message(forbidden).is_false()


func test_hud_shows_the_server_points_and_streak_then_the_final_game_score() -> void:
	await _open("true_false")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true, 12, 1))
	MiniGameFixtures.answer(_scene())
	await _settle()
	assert_int(host.game_points).is_equal(12)
	host.press_primary()
	await await_idle_frame()
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(2, true, 15, 2))
	MiniGameFixtures.answer(_scene())
	await _settle()
	assert_int(host.streak).is_equal(2)
	assert_bool((host.find_child("Streak", true, false) as Label).visible).is_true()
	h.http.reply(MiniGameFixtures.ROUTE_SUBMIT, 200, MiniGameFixtures.submitted("true_false", 40, true))
	host.press_primary()
	await _settle()
	assert_int(host.state).is_equal(MiniGameHost.State.FINISHED)
	assert_int(host.summary.game_score).is_equal(40)
	assert_bool(host.summary.completion_met).is_true()
	assert_int(host.game_points).is_equal(40)
	assert_str((host.find_child("SummaryScore", true, false) as Label).text).contains("40")
	(host.find_child("Continue", true, false) as Button).pressed.emit()
	assert_bool(host.visible).is_false()


# ---------- F073-AC-24: grades 1–5 ----------

func test_grade_1_to_5_touch_targets_are_at_least_64_px_on_every_scene() -> void:
	for type: String in MiniGameFixtures.CORE_TYPES + MiniGameFixtures.EXTENDED_TYPES:
		await _open(type, "g1_3")
		await await_idle_frame()
		assert_bool(_scene().uses_large_targets()).is_true()
		assert_bool(_scene().touch_targets_ok(host)).override_failure_message(type).is_true()
		host.close()


func test_grade_4_5_band_also_gets_64_px() -> void:
	await _open("ordering", "g4_5")
	await await_idle_frame()
	assert_bool(_scene().touch_targets_ok(host)).is_true()


func test_verdicts_come_with_sound_and_icon_and_text() -> void:
	await _open("true_false", "g1_3")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true))
	MiniGameFixtures.answer(_scene())
	await _settle()
	var scene: MiniGameQuestionScene = _scene()
	assert_str(scene.last_icon).is_equal("✓")
	assert_int(scene.sounds_played).is_equal(1)
	assert_str((scene.find_child("EffectText", true, false) as Label).text).is_equal("ui.feedback.correct")
	assert_str((scene.find_child("EffectIcon", true, false) as Label).text).is_equal("✓")
	var sfx: AudioStreamPlayer = scene.find_child("FeedbackSfx", true, false) as AudioStreamPlayer
	assert_object(sfx).is_not_null()
	assert_object(sfx.stream).is_same(MiniGameFeedbackSfx.stream(MiniGameFeedbackSfx.KIND_CORRECT))
	host.press_primary()
	await await_idle_frame()
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(2, false))
	MiniGameFixtures.answer(_scene())
	await _settle()
	assert_str(scene.last_icon).is_equal("✗")
	assert_int(scene.sounds_played).is_equal(2)
	assert_str((scene.find_child("EffectText", true, false) as Label).text).is_equal("ui.feedback.incorrect")
	assert_object(sfx.stream).is_same(MiniGameFeedbackSfx.stream(MiniGameFeedbackSfx.KIND_INCORRECT))


func test_keyboard_number_and_enter_play_the_game() -> void:
	await _open("multiple_choice")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true))
	var key: InputEventKey = InputEventKey.new()
	key.keycode = KEY_3
	key.pressed = true
	host._unhandled_input(key)
	await _settle()
	assert_dict(h.http.last(MiniGameFixtures.ROUTE_RESPOND).body["response"] as Dictionary).is_equal({"option_id": "o_c"})
	assert_int(host.state).is_equal(MiniGameHost.State.FEEDBACK)
	var enter: InputEventKey = InputEventKey.new()
	enter.keycode = KEY_ENTER
	enter.pressed = true
	host._unhandled_input(enter)
	await await_idle_frame()
	assert_int(host.index).is_equal(1)
	assert_int(host.state).is_equal(MiniGameHost.State.PLAYING)


func test_mouse_or_touch_press_answers_through_the_buttons() -> void:
	await _open("ordering")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true))
	var rows: Dictionary = _scene().question.get("rows") as Dictionary
	((rows["i_2"] as HBoxContainer).get_child(1) as Button).pressed.emit()
	(_scene().find_child("Done", true, false) as Button).pressed.emit()
	await _settle()
	assert_dict(h.http.last(MiniGameFixtures.ROUTE_RESPOND).body["response"] as Dictionary).is_equal(
			{"order": ["i_2", "i_1", "i_3", "i_4"]})


# ---------- quest step / assignment: inside the activity player ----------

func test_minigame_activity_runs_inside_the_activity_player_to_its_result() -> void:
	var activity_host: Control = auto_free((load(ACTIVITY_HOST_SCENE) as PackedScene).instantiate()) as Control
	activity_host.set("services", services)
	activity_host.set("client_version", "1.2.3")
	activity_host.set("grade_band", "g4_5")
	add_child(activity_host)
	# ActivityHost runs its own countdown on the device wall clock: the embedded play here has no deadline.
	var data: Dictionary = MiniGameFixtures.attempt("ordering")
	data.erase("deadline_at")
	h.http.reply(LearningFixtures.ROUTE_START, 201, data)
	await activity_host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "assignment"})
	var renderer: MinigameActivityRenderer = activity_host.get("renderer") as MinigameActivityRenderer
	assert_object(renderer).is_not_null()
	host = renderer.host
	assert_bool(host.embedded).is_true()
	assert_bool((activity_host.find_child("Primary", true, false) as Button).visible).is_false()
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true, 10, 1))
	MiniGameFixtures.answer(_scene())
	assert_bool(renderer.check()).is_true()
	await _settle()
	assert_int(host.state).is_equal(MiniGameHost.State.FEEDBACK)
	h.http.reply(MiniGameFixtures.ROUTE_SUBMIT, 200, MiniGameFixtures.submitted("ordering", 10, true))
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	host.press_primary()
	await _settle()
	assert_int(host.state).is_equal(MiniGameHost.State.FINISHED)
	(host.find_child("Continue", true, false) as Button).pressed.emit()
	for i: int in 60:
		await await_idle_frame()
		if int(activity_host.get("state")) == 6:
			break
	assert_int(int(activity_host.get("state"))).is_equal(6)
	assert_int(h.http.count(MiniGameFixtures.ROUTE_SUBMIT)).is_equal(1)
