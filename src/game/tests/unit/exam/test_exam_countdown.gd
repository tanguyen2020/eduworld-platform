extends GdUnitTestSuite
## F064-AC-03, F064-AC-04 (TC-EXM-002; T006): the countdown is `deadline_at − server now`, the server now being the
## server time of the attempt plus the monotonic time elapsed since it was received (a changed device clock does not
## matter); the shown seconds are within 1 s of the server; the 5 and 1 minute warnings fire once each (none for a
## threshold already passed when the runner opens); `g1_3` shows a sand glass without seconds. Pure functions.

var countdown: ExamCountdown = null


func before_test() -> void:
	countdown = ExamCountdown.new()
	countdown.sync(ExamFixtures.DEADLINE_AT, ExamFixtures.SERVER_TIME, 1_000_000, 0.0)


func test_remaining_is_the_deadline_minus_the_server_now() -> void:
	assert_int(countdown.remaining_ms(1_000_000)).is_equal(900_000)
	assert_int(countdown.remaining_ms(1_010_500)).is_equal(889_500)
	assert_int(countdown.remaining_ms(1_000_000 + 900_000)).is_equal(0)
	assert_int(countdown.remaining_ms(1_000_000 + 999_999)).is_equal(0)
	assert_int(ExamCountdown.remaining_between(10_000, 4_000)).is_equal(6_000)
	assert_int(ExamCountdown.remaining_between(10_000, 14_000)).is_equal(0)


func test_server_offset_is_kept_when_the_device_clock_is_wrong() -> void:
	# The device clock is a day late: only the server time and the monotonic clock count.
	var skewed: ExamCountdown = ExamCountdown.new()
	skewed.sync(ExamFixtures.DEADLINE_AT, ExamFixtures.SERVER_TIME, 5_000, 1_763_000_000.0 - 86_400.0)
	assert_int(skewed.remaining_ms(5_000 + 60_000)).is_equal(840_000)
	assert_int(skewed.server_now_ms(5_000)).is_equal(ExamCountdown.iso_to_ms(ExamFixtures.SERVER_TIME))


func test_without_server_time_the_device_clock_is_the_fallback() -> void:
	var c: ExamCountdown = ExamCountdown.new()
	var server_unix: float = float(ExamCountdown.iso_to_ms(ExamFixtures.SERVER_TIME)) / 1000.0
	c.sync(ExamFixtures.DEADLINE_AT, "", 2_000, server_unix)
	assert_int(c.remaining_ms(2_000)).is_equal(900_000)


func test_shown_seconds_are_within_one_second_of_the_server() -> void:
	for elapsed_ms: int in [0, 1, 250, 999, 1_000, 59_001, 123_456, 899_999]:
		var left: int = countdown.remaining_ms(1_000_000 + elapsed_ms)
		var shown_sec: int = ExamCountdown.display_seconds(left)
		var skew: float = float(shown_sec) - float(left) / 1000.0
		assert_float(skew).is_greater_equal(0.0)
		assert_float(skew).is_less(1.0)


func test_clock_text() -> void:
	assert_str(ExamCountdown.clock_text(889_500)).is_equal("14:50")
	assert_str(ExamCountdown.clock_text(59_001)).is_equal("01:00")
	assert_str(ExamCountdown.clock_text(0)).is_equal("00:00")
	assert_str(ExamCountdown.clock_text(3_723_000)).is_equal("1:02:03")


func test_each_warning_fires_once() -> void:
	countdown.prime(1_000_000)
	assert_array(countdown.due_warnings(1_000_000)).is_empty()
	# 10 minutes elapsed: 5 left → the 5-minute warning, once.
	assert_array(countdown.due_warnings(1_600_000)).is_equal([5])
	assert_array(countdown.due_warnings(1_601_000)).is_empty()
	assert_array(countdown.due_warnings(1_700_000)).is_empty()
	# 14 minutes elapsed: 1 left → the 1-minute warning, once.
	assert_array(countdown.due_warnings(1_840_000)).is_equal([1])
	assert_array(countdown.due_warnings(1_850_000)).is_empty()
	assert_array(countdown.due_warnings(1_900_000)).is_empty()


func test_warning_level_is_steady_not_blinking() -> void:
	assert_int(ExamCountdown.warning_level(300_001)).is_equal(0)
	assert_int(ExamCountdown.warning_level(300_000)).is_equal(1)
	assert_int(ExamCountdown.warning_level(120_000)).is_equal(1)
	assert_int(ExamCountdown.warning_level(60_000)).is_equal(2)
	# The level only depends on the time left: consecutive seconds give the same level (no alternation).
	for ms: int in range(59_000, 0, -1_000):
		assert_int(ExamCountdown.warning_level(ms)).is_equal(2)


func test_resume_after_a_threshold_does_not_replay_it() -> void:
	# Opened with 3 minutes left: the 5-minute warning is already past and never shown.
	countdown.prime(1_000_000 + 720_000)
	assert_array(countdown.due_warnings(1_000_000 + 721_000)).is_empty()
	assert_array(countdown.due_warnings(1_000_000 + 840_000)).is_equal([1])


func test_skipping_both_thresholds_at_once_reports_only_the_last() -> void:
	countdown.prime(1_000_000)
	assert_array(countdown.due_warnings(1_000_000 + 850_000)).is_equal([1])


func test_g1_3_shows_a_sand_glass_without_seconds() -> void:
	assert_bool(ExamCountdown.shows_seconds("g1_3")).is_false()
	for band: String in ["g4_5", "g6_7", "g8_9"]:
		assert_bool(ExamCountdown.shows_seconds(band)).is_true()
	assert_float(ExamCountdown.sand_fraction(450_000, 900_000)).is_equal_approx(0.5, 0.0001)
	assert_float(ExamCountdown.sand_fraction(0, 900_000)).is_equal(0.0)
	assert_float(ExamCountdown.sand_fraction(1_000_000, 900_000)).is_equal(1.0)
	assert_float(ExamCountdown.sand_fraction(10, 0)).is_equal(0.0)


func test_iso_timestamps_with_fraction_and_offset() -> void:
	var base: int = ExamCountdown.iso_to_ms("2026-11-20T01:30:00Z")
	assert_int(ExamCountdown.iso_to_ms("2026-11-20T01:30:00.250Z")).is_equal(base + 250)
	assert_int(ExamCountdown.iso_to_ms("2026-11-20T01:30:00.123456789Z")).is_equal(base + 123)
	assert_int(ExamCountdown.iso_to_ms("2026-11-20T08:30:00+07:00")).is_equal(base)
	assert_int(ExamCountdown.iso_to_ms("")).is_equal(0)
	assert_int(ExamCountdown.iso_to_ms("not a time")).is_equal(0)
