extends GdUnitTestSuite
## F113-AC-03, F113-AC-05: pure rules of the rotate prompt (OrientationPolicy).


func test_taller_than_wide_is_portrait_square_is_landscape() -> void:
	assert_bool(OrientationPolicy.is_portrait(Vector2(390, 844))).is_true()
	assert_bool(OrientationPolicy.is_portrait(Vector2(1280, 2770))).is_true()
	assert_bool(OrientationPolicy.is_portrait(Vector2(844, 390))).is_false()
	assert_bool(OrientationPolicy.is_portrait(Vector2(1280, 720))).is_false()
	assert_bool(OrientationPolicy.is_portrait(Vector2(800, 800))).is_false()
	assert_bool(OrientationPolicy.is_portrait(Vector2(800, 801))).is_true()


func test_base_short_side_is_the_canvas_height() -> void:
	assert_float(OrientationPolicy.base_short()).is_equal(720.0)


func test_prompt_scale_follows_the_short_side() -> void:
	# 390×844 window → logical 1280×2770 (stretch expand on 1280×720): the prompt is laid out on 720×1558.
	assert_float(OrientationPolicy.prompt_scale(Vector2(1280, 2770), 720.0)).is_equal_approx(1280.0 / 720.0, 0.0001)
	# 720×1280 window → logical 1280×2276.
	assert_float(OrientationPolicy.prompt_scale(Vector2(1280, 2276), 720.0)).is_equal_approx(1280.0 / 720.0, 0.0001)
	# Never smaller than the canvas, never a zero/negative scale.
	assert_float(OrientationPolicy.prompt_scale(Vector2(600, 900), 720.0)).is_equal(1.0)
	assert_float(OrientationPolicy.prompt_scale(Vector2.ZERO, 720.0)).is_equal(1.0)
	assert_float(OrientationPolicy.prompt_scale(Vector2(1280, 2770), 0.0)).is_equal(1.0)


func test_prompt_content_scale_lays_the_prompt_on_the_base_short_side() -> void:
	var base: Vector2 = OrientationPolicy.base_size()
	assert_vector(base).is_equal(Vector2(1280, 720))
	# A phone held upright (390×844 → logical 1280×2770): factor 16/9 → prompt canvas 720×1558, on screen 390 px wide
	# for 720 logical px (0.54 per px, as the landscape canvas on the same phone held sideways: 844 / 1558).
	var upright: float = OrientationPolicy.prompt_content_scale(Vector2(390, 844), base)
	assert_float(upright).is_equal_approx(1280.0 / 720.0, 0.0001)
	var final_scale: float = minf(390.0 / 1280.0, 844.0 / 720.0) * upright
	assert_float(final_scale).is_equal_approx(390.0 / 720.0, 0.0001)
	assert_float(final_scale).is_equal_approx(844.0 / 1558.0, 0.001)
	assert_float(OrientationPolicy.prompt_content_scale(Vector2(720, 1280), base)).is_equal_approx(1280.0 / 720.0, 0.0001)
	assert_float(OrientationPolicy.prompt_content_scale(Vector2(768, 1024), base)).is_equal_approx(1280.0 / 720.0, 0.0001)
	# Landscape windows never need it; degenerate sizes give 1.
	assert_float(OrientationPolicy.prompt_content_scale(Vector2(1280, 720), base)).is_equal(1.0)
	assert_float(OrientationPolicy.prompt_content_scale(Vector2(844, 390), base)).is_equal(1.0)
	assert_float(OrientationPolicy.prompt_content_scale(Vector2.ZERO, base)).is_equal(1.0)
