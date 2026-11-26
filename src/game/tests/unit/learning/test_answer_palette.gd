extends GdUnitTestSuite
## F099-AC-01 (STUDENT_GAME_UX_LEARNING §4.1a, DESIGN_SYSTEM §5.1): the six candy colour sets of the answer tiles have
## no green and no orange — the hue of every accent is at least MIN_HUE_DEG away from the right (green) and
## not-yet-right (orange) state colours, so an unchosen tile can never pass for a graded one. Reads the generated
## UiTokens (source: design/game-tokens/component.json).

const MIN_HUE_DEG: float = 30.0


static func _hue_gap_deg(a: Color, b: Color) -> float:
	var d: float = absf(a.h - b.h) * 360.0
	return minf(d, 360.0 - d)


func test_candy_accents_are_far_from_the_state_hues() -> void:
	var states: Array[Color] = [UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BORDER, UiTokens.COMPONENT_ANSWER_TILE_TRY_BORDER,
		UiTokens.COMPONENT_ANSWER_TILE_TAG_CORRECT_BG, UiTokens.COMPONENT_ANSWER_TILE_TAG_TRY_BG]
	assert_int(AnswerBadge.ACCENTS.size()).is_equal(6)
	for i: int in AnswerBadge.ACCENTS.size():
		for state: Color in states:
			var gap: float = _hue_gap_deg(AnswerBadge.ACCENTS[i], state)
			assert_float(gap).override_failure_message("accent %d is %.0f° from state %s" % [i + 1, gap, state.to_html()]) \
					.is_greater_equal(MIN_HUE_DEG)


func test_candy_backgrounds_are_not_the_state_backgrounds() -> void:
	var bgs: Array[Color] = [UiTokens.COMPONENT_ANSWER_TILE_BG_1, UiTokens.COMPONENT_ANSWER_TILE_BG_2,
		UiTokens.COMPONENT_ANSWER_TILE_BG_3, UiTokens.COMPONENT_ANSWER_TILE_BG_4, UiTokens.COMPONENT_ANSWER_TILE_BG_5,
		UiTokens.COMPONENT_ANSWER_TILE_BG_6]
	for i: int in bgs.size():
		assert_bool(bgs[i].is_equal_approx(UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BG)).is_false()
		assert_bool(bgs[i].is_equal_approx(UiTokens.COMPONENT_ANSWER_TILE_TRY_BG)).is_false()


func test_rest_look_is_neutral() -> void:
	# The resting tile is white with a grey outline and an ink number disc: low saturation, never a state colour.
	for c: Color in [UiTokens.COMPONENT_ANSWER_TILE_REST_BORDER, UiTokens.COMPONENT_ANSWER_TILE_REST_ACCENT]:
		assert_float(c.s).is_less(0.35)
	assert_bool(UiTokens.COMPONENT_ANSWER_TILE_REST_BG.is_equal_approx(UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BG)).is_false()
	assert_bool(UiTokens.COMPONENT_ANSWER_TILE_REST_BG.is_equal_approx(UiTokens.COMPONENT_ANSWER_TILE_TRY_BG)).is_false()
