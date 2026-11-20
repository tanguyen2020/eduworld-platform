extends GdUnitTestSuite
## F092-AC-12, F092-AC-14 (T017; GAME_NETWORKING §5.2, SC-003): render time = server now − delay; lerp position and
## yaw between the samples around it; extrapolation at most 250 ms then still; the delay grows with jitter up to
## 200 ms; mean display error ≤ 0.3 m with 20 % of the snapshots lost and 80 ms jitter; the server correction is
## blended in 200 ms (no teleport).


func _sample(ms: int, p: Vector3, yaw: float = 0.0, anim: String = "walk") -> SnapshotSample:
	var s: SnapshotSample = SnapshotSample.new()
	s.server_ms = ms
	s.tick = ms / 100
	s.p = p
	s.yaw = yaw
	s.anim = anim
	s.has_p = true
	s.has_yaw = true
	s.has_anim = true
	return s


func test_lerp_between_the_two_samples_around_render_time() -> void:
	var samples: Array = [_sample(1000, Vector3.ZERO, 0.0, "idle"), _sample(1100, Vector3(1, 0, 0), PI / 2.0, "walk")]
	var mid: Dictionary = Interpolator.sample_at(samples, 1050)
	assert_bool((mid["p"] as Vector3).is_equal_approx(Vector3(0.5, 0, 0))).is_true()
	assert_float(float(mid["yaw"])).is_equal_approx(PI / 4.0, 0.0001)
	assert_str(String(mid["anim"])).is_equal("idle")
	assert_bool(bool(mid["extrapolated"])).is_false()
	# Yaw takes the short way round (−170° → 170° passes 180°).
	var wrap: Array = [_sample(0, Vector3.ZERO, deg_to_rad(-170.0)), _sample(100, Vector3.ZERO, deg_to_rad(170.0))]
	assert_float(absf(float(Interpolator.sample_at(wrap, 50)["yaw"]))).is_equal_approx(PI, 0.001)


func test_extrapolates_at_most_250_ms_then_stands_still() -> void:
	var samples: Array = [_sample(1000, Vector3.ZERO), _sample(1100, Vector3(0.5, 0, 0))]
	var ahead: Dictionary = Interpolator.sample_at(samples, 1200)
	assert_bool((ahead["p"] as Vector3).is_equal_approx(Vector3(1.0, 0, 0))).is_true()
	assert_bool(bool(ahead["extrapolated"])).is_true()
	var capped: Dictionary = Interpolator.sample_at(samples, 1100 + 250)
	var far: Dictionary = Interpolator.sample_at(samples, 1100 + 2000)
	assert_bool((far["p"] as Vector3).is_equal_approx(capped["p"] as Vector3)).is_true()
	assert_float((far["p"] as Vector3).x).is_equal_approx(0.5 + 5.0 * 0.25, 0.0001)


func test_delay_grows_with_jitter_up_to_the_ceiling() -> void:
	var ip: Interpolator = Interpolator.new()
	assert_int(ip.delay_ms).is_equal(Interpolator.BASE_DELAY_MS)
	for i: int in 50:
		ip.observe(i * 100 + 20, i * 100)
	assert_int(ip.delay_ms).is_equal(Interpolator.BASE_DELAY_MS)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 7
	for i: int in 200:
		ip.observe(i * 100 + rng.randi_range(0, 80), i * 100)
	assert_int(ip.delay_ms).is_greater(Interpolator.BASE_DELAY_MS)
	assert_int(ip.delay_ms).is_less_equal(Interpolator.MAX_DELAY_MS)
	for i: int in 200:
		ip.observe(i * 100 + rng.randi_range(0, 1000), i * 100)
	assert_int(ip.delay_ms).is_equal(Interpolator.MAX_DELAY_MS)


func test_buffer_keeps_one_second_and_fills_delta_fields() -> void:
	var buffer: SnapshotBuffer = SnapshotBuffer.new()
	buffer.push(3, _sample(0, Vector3(1, 0, 0), 0.5, "run"))
	var delta: SnapshotSample = SnapshotSample.new()
	delta.server_ms = 100
	delta.tick = 1
	delta.p = Vector3(2, 0, 0)
	delta.has_p = true
	buffer.push(3, delta)
	assert_float(buffer.latest(3).yaw).is_equal_approx(0.5, 0.0001)
	assert_str(buffer.latest(3).anim).is_equal("run")
	for i: int in range(2, 40):
		buffer.push(3, _sample(i * 100, Vector3(i, 0, 0)))
	var list: Array = buffer.samples(3)
	assert_int((list[list.size() - 1] as SnapshotSample).server_ms - (list[0] as SnapshotSample).server_ms).is_less_equal(SnapshotBuffer.KEEP_MS)
	# Older or duplicate samples are ignored.
	buffer.push(3, _sample(500, Vector3(99, 0, 0)))
	assert_float(buffer.latest(3).p.x).is_equal_approx(39.0, 0.0001)


## SC-003: a player running a 6 m circle at 4 m/s, snapshots at 10 Hz, 20 % lost, 0–80 ms arrival jitter; the display
## error at render time is ≤ 0.3 m on average.
func test_mean_error_with_loss_and_jitter() -> void:
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 92
	var ip: Interpolator = Interpolator.new()
	var buffer: SnapshotBuffer = SnapshotBuffer.new()
	var truth: Callable = func(ms: int) -> Vector3:
		var angle: float = float(ms) / 1000.0 * 4.0 / 6.0
		return Vector3(cos(angle) * 6.0, 0, sin(angle) * 6.0)
	var arrivals: Array = []
	for tick: int in 300:
		var server_ms: int = tick * 100
		if rng.randf() < 0.2:
			continue
		arrivals.append([server_ms + rng.randi_range(0, 80), server_ms])
	arrivals.sort_custom(func(a: Array, b: Array) -> bool: return int(a[0]) < int(b[0]))
	var next: int = 0
	var total: float = 0.0
	var count: int = 0
	for now: int in range(2000, 29000, 16):
		while next < arrivals.size() and int(arrivals[next][0]) <= now:
			var s_ms: int = int(arrivals[next][1])
			ip.observe(int(arrivals[next][0]), s_ms)
			buffer.push(1, _sample(s_ms, truth.call(s_ms)))
			next += 1
		var render: int = ip.render_time(now)
		var shown: Dictionary = Interpolator.sample_at(buffer.samples(1), render)
		total += (shown["p"] as Vector3).distance_to(truth.call(render) as Vector3)
		count += 1
	var mean: float = total / float(count)
	assert_float(mean).override_failure_message("mean error %.3f m" % mean).is_less_equal(0.3)


func test_correction_blends_in_200_ms() -> void:
	var c: PositionCorrection = PositionCorrection.new()
	var position: Vector3 = Vector3(10, 0, 0)
	c.start(position, Vector3(0, 0, 0))
	var first: Vector3 = c.step(1.0 / 60.0)
	# No teleport: the first frame moves only a small part of the 10 m.
	assert_float(first.length()).is_less(1.0)
	position += first
	var elapsed: float = 1.0 / 60.0
	while c.is_active():
		position += c.step(1.0 / 60.0)
		elapsed += 1.0 / 60.0
	assert_bool(position.is_equal_approx(Vector3.ZERO)).is_true()
	assert_float(elapsed).is_less_equal(PositionCorrection.BLEND_SEC + 1.0 / 60.0 + 0.0001)
