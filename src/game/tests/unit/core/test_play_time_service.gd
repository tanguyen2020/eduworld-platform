extends GdUnitTestSuite
## F062-AC-21, F062-AC-22 (FR-GAME-012, CHILD_SAFETY, CONFIG_DEFAULTS `break_reminder`; T028): the break reminder
## threshold comes from the bootstrap policy, the reminder repeats every `break_reminder_minutes` of continuous play,
## never blocks (it is only a signal), and a reminder that falls due during a timed exam or a mandatory dialog waits
## until every hold is released. No policy → no reminder (no client-side default, R33).

var service: PlayTimeService = null
var shown: Array[Array] = []


func before_test() -> void:
	service = PlayTimeService.new()
	shown.clear()
	service.break_reminder_shown.connect(_on_shown)


func _on_shown(reminder_no: int, minutes_played: int) -> void:
	shown.append([reminder_no, minutes_played])


func _policy(minutes: Variant) -> Dictionary:
	return {"play_time": {"break_reminder_minutes": minutes, "access_window": "06:30-21:30 Asia/Ho_Chi_Minh"}}


func _play(seconds: float, step: float = 1.0) -> void:
	var left: float = seconds
	while left > 0.0:
		service.tick(minf(step, left))
		left -= step


func test_reminder_appears_once_at_the_server_threshold() -> void:
	service.configure(_policy(1))
	_play(59.0)
	assert_array(shown).is_empty()
	_play(1.0)
	assert_array(shown).is_equal([[1, 1]])
	_play(30.0)
	assert_int(shown.size()).is_equal(1)


func test_reminder_repeats_every_interval() -> void:
	service.configure(_policy(45))
	_play(45.0 * 60.0, 5.0)
	_play(45.0 * 60.0, 5.0)
	assert_array(shown).is_equal([[1, 45], [2, 90]])


func test_no_policy_means_no_reminder() -> void:
	for policy: Dictionary in [{}, {"play_time": {}}, _policy(0), _policy("30"), _policy(null)]:
		service.configure(policy)
		_play(3.0 * 3600.0, 60.0)
		assert_bool(service.is_enabled()).is_false()
	assert_array(shown).is_empty()


func test_reminder_waits_during_a_timed_exam_and_shows_after() -> void:
	service.configure(_policy(1))
	service.hold(&"exam_lobby")
	_play(90.0)
	assert_array(shown).is_empty()
	assert_bool(service.pending).is_true()
	service.release(&"exam_lobby")
	assert_array(shown).is_equal([[1, 1]])
	# The next reminder counts a full interval from the moment the deferred one was shown.
	_play(59.0)
	assert_int(shown.size()).is_equal(1)
	_play(1.0)
	assert_int(shown.size()).is_equal(2)


func test_all_holds_must_be_released() -> void:
	service.configure(_policy(1))
	service.hold(&"exam_lobby")
	service.hold(&"dialog")
	_play(61.0)
	service.release(&"dialog")
	assert_array(shown).is_empty()
	service.release(&"exam_lobby")
	assert_int(shown.size()).is_equal(1)


func test_reminder_count_is_capped_by_the_contract() -> void:
	service.configure(_policy(1))
	_play(30.0 * 60.0, 1.0)
	assert_int(shown.size()).is_equal(PlayTimeService.MAX_REMINDERS)


func test_new_session_restarts_the_count() -> void:
	service.configure(_policy(1))
	_play(50.0)
	service.configure(_policy(1))
	_play(50.0)
	assert_array(shown).is_empty()
