extends GdUnitTestSuite
## F125-AC-01, F125-AC-03, F125-AC-05 (STUDENT_GAME_UX §2 compact breakpoint, DESIGN_SYSTEM §5.2, §9): CompactPolicy
## decides compact mode on the window short side in density-independent px (< 500), sets the content scale that makes
## the logical canvas `component.compact.logical_height` (540) tall — never below 1, never narrower than 16:9 — and floors
## the touch target and the smallest font; on the reference phones every band reaches 12 CSS px text and 40 CSS px
## touch targets, which the unscaled canvas (factor 1) does not.

const BASE: Vector2 = Vector2(1280, 720)


func test_tokens_and_setting() -> void:
	assert_int(UiTokens.COMPONENT_COMPACT_LOGICAL_HEIGHT).is_equal(540)
	assert_int(UiTokens.DENSITY_COMPACT_BUTTON_MIN_HEIGHT).is_greater_equal(56)
	assert_int(UiTokens.DENSITY_COMPACT_FONT_SMALL).is_equal(18)
	assert_float(CompactPolicy.short_side_limit()).is_equal(500.0)
	assert_vector(OrientationPolicy.base_size()).is_equal(BASE)


func test_short_side_below_500_density_independent_px_is_compact() -> void:
	# Phones held sideways (CSS px at density 1).
	assert_bool(CompactPolicy.is_compact(Vector2(844, 390))).is_true()
	assert_bool(CompactPolicy.is_compact(Vector2(740, 360))).is_true()
	assert_bool(CompactPolicy.is_compact(Vector2(999, 499))).is_true()
	# PC, tablet: never compact.
	assert_bool(CompactPolicy.is_compact(Vector2(1280, 720))).is_false()
	assert_bool(CompactPolicy.is_compact(Vector2(1024, 600))).is_false()
	assert_bool(CompactPolicy.is_compact(Vector2(1000, 500))).is_false()
	# Density independent: the same phone at 3× physical px is still 390 tall; a 2× tablet is 768 tall.
	assert_bool(CompactPolicy.is_compact(Vector2(2532, 1170), 3.0)).is_true()
	assert_bool(CompactPolicy.is_compact(Vector2(1688, 780), 2.0)).is_true()
	assert_bool(CompactPolicy.is_compact(Vector2(2048, 1536), 2.0)).is_false()
	assert_bool(CompactPolicy.is_compact(Vector2(1688, 780), 1.0)).is_false()
	# The phone held upright is compact too (the rotate prompt shows over it).
	assert_bool(CompactPolicy.is_compact(Vector2(390, 844))).is_true()
	assert_bool(CompactPolicy.is_compact(Vector2.ZERO)).is_false()


func test_an_on_screen_keyboard_never_switches_the_mode() -> void:
	# A tablet (1024×600) with a 300 px keyboard open reports a 1024×300 window: still not compact.
	assert_bool(CompactPolicy.is_compact(Vector2(1024, 300), 1.0, 300.0)).is_false()
	assert_bool(CompactPolicy.is_compact(Vector2(1024, 300), 1.0, 0.0)).is_true()
	# The phone stays compact with or without the keyboard, with the same factor.
	assert_bool(CompactPolicy.is_compact(Vector2(844, 200), 1.0, 190.0)).is_true()
	assert_float(CompactPolicy.content_scale(Vector2(844, 200), BASE, true)).is_equal_approx(
			CompactPolicy.content_scale(Vector2(844, 390), BASE, true), 0.0001)


func test_compact_canvas_is_540_logical_px_tall() -> void:
	for window: Vector2 in [Vector2(844, 390), Vector2(740, 360), Vector2(2532, 1170), Vector2(926, 428)]:
		var factor: float = CompactPolicy.content_scale(window, BASE, true)
		assert_float(factor).is_equal_approx(720.0 / 540.0, 0.0001)
		var view: Vector2 = CompactPolicy.logical_size(window, BASE, factor)
		assert_float(view.y).is_equal_approx(540.0, 0.01)
	assert_vector(CompactPolicy.logical_size(Vector2(844, 390), BASE, 720.0 / 540.0)).is_equal_approx(
			Vector2(1168.6, 540.0), Vector2(0.1, 0.01))
	assert_vector(CompactPolicy.logical_size(Vector2(740, 360), BASE, 720.0 / 540.0)).is_equal_approx(
			Vector2(1110.0, 540.0), Vector2(0.01, 0.01))


func test_outside_compact_the_factor_is_one() -> void:
	assert_float(CompactPolicy.content_scale(Vector2(1280, 720), BASE, false)).is_equal(1.0)
	assert_float(CompactPolicy.content_scale(Vector2(844, 390), BASE, false)).is_equal(1.0)
	assert_float(CompactPolicy.content_scale(Vector2.ZERO, BASE, true)).is_equal(1.0)


func test_a_window_narrower_than_16_9_keeps_a_16_9_canvas_and_the_factor_never_drops_below_one() -> void:
	var window: Vector2 = Vector2(600, 480)
	var factor: float = CompactPolicy.content_scale(window, BASE, true)
	var view: Vector2 = CompactPolicy.logical_size(window, BASE, factor)
	assert_float(view.x).is_greater_equal(960.0 - 0.01)
	assert_float(view.y).is_greater_equal(540.0 - 0.01)
	assert_float(factor).is_greater_equal(1.0)
	for w: Vector2 in [Vector2(480, 480), Vector2(499, 300), Vector2(900, 450)]:
		assert_float(CompactPolicy.content_scale(w, BASE, true)).is_greater_equal(1.0)


func test_compact_floors_never_lower_a_band() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var regular: float = CompactPolicy.button_target(band, false)
		assert_float(regular).is_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band]))
		assert_float(CompactPolicy.button_target(band, true)).is_equal(
				maxf(regular, float(UiTokens.DENSITY_COMPACT_BUTTON_MIN_HEIGHT)))
		assert_int(CompactPolicy.font_floor(band, false)).is_equal(int(UiTokens.DENSITY_FONT_SMALL[band]))
		assert_int(CompactPolicy.font_floor(band, true)).is_greater_equal(int(UiTokens.DENSITY_FONT_SMALL[band]))
		assert_int(CompactPolicy.font_floor(band, true)).is_greater_equal(UiTokens.DENSITY_COMPACT_FONT_SMALL)
	# g1_3 keeps its 64 and its 24.
	assert_float(CompactPolicy.button_target("g1_3", true)).is_equal(64.0)
	assert_int(CompactPolicy.font_floor("g1_3", true)).is_equal(24)


## F125-AC-03 on paper: at the reference phones every band reaches 12 CSS px text and 40 CSS px touch targets in compact
## mode; the unscaled canvas (factor 1, the layout before F125) does not (negative check).
func test_physical_floors_hold_on_the_reference_phones_and_fail_unscaled() -> void:
	for window: Vector2 in [Vector2(844, 390), Vector2(740, 360)]:
		var px: float = CompactPolicy.physical_scale(window, BASE, CompactPolicy.content_scale(window, BASE, true))
		for band: String in UiTokens.DENSITY_BANDS:
			assert_float(float(CompactPolicy.font_floor(band, true)) * px + 0.05).is_greater_equal(12.0)
			assert_float(CompactPolicy.button_target(band, true) * px + 0.05).is_greater_equal(40.0)
	var unscaled: float = CompactPolicy.physical_scale(Vector2(844, 390), BASE, 1.0)
	assert_float(unscaled).is_equal_approx(390.0 / 720.0, 0.0001)
	for band: String in UiTokens.DENSITY_BANDS:
		var font_ok: bool = float(CompactPolicy.font_floor(band, false)) * unscaled >= 12.0
		var touch_ok: bool = CompactPolicy.button_target(band, false) * unscaled >= 40.0
		assert_bool(font_ok and touch_ok).override_failure_message("%s passes unscaled" % band).is_false()


## The matrix windows (BandLayoutProbe) are the logical canvases CompactPolicy gives (no drift between the two).
func test_matrix_sizes_follow_the_policy() -> void:
	for name: String in BandLayoutProbe.WINDOWS.keys():
		var window: Vector2 = BandLayoutProbe.WINDOWS[name] as Vector2
		var compact: bool = CompactPolicy.is_compact(window)
		assert_bool(BandLayoutProbe.compact_of(name)).is_equal(compact)
		var view: Vector2 = CompactPolicy.logical_size(window, BASE, CompactPolicy.content_scale(window, BASE, compact))
		assert_vector(Vector2(BandLayoutProbe.SIZES[name] as Vector2i)).is_equal_approx(view.floor(), Vector2(1, 1))
	assert_bool(BandLayoutProbe.compact_of("844x390")).is_true()
	assert_bool(BandLayoutProbe.compact_of("740x360")).is_true()
	assert_bool(BandLayoutProbe.compact_of("1024x600")).is_false()
	assert_float(BandLayoutProbe.css_scale_of("844x390")).is_equal_approx(390.0 / 540.0, 0.0001)
	assert_float(BandLayoutProbe.css_scale_of("740x360")).is_equal_approx(360.0 / 540.0, 0.0001)
	assert_float(BandLayoutProbe.css_scale_of("844x390-1x")).is_equal_approx(844.0 / 1558.0, 0.0001)
