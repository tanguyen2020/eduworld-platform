extends GdUnitTestSuite
## T012 (F111-AC-02; STUDENT_GAME_UX_SOCIAL §6, MULTIPLAYER_GAMEPLAY §5): "Tham gia bạn" is on only when the friend is
## `online` / `away` with an `area_id` and not in an exam, and the student is not offline, busy (exam, boss, challenge —
## `AppState.blocking_activity`) or already joining; otherwise the button is grey WITH the reason in words. The client
## check is only a hint (the server decides). T015: the profile panel shows the button and its reason.

func _friend(status: String, area: String = "main_hub", activity: String = "idle") -> FriendModel:
	var f: FriendModel = FriendModel.from_card(F110Fixtures.card(F111Fixtures.FRIEND_ONLINE, "CaHeoVui07", status))
	f.area_id = area
	f.activity = activity
	return f


func test_reasons_of_the_friend() -> void:
	var me: Dictionary = {}
	assert_str(String(JoinAvailability.evaluate(_friend("online"), me))).is_empty()
	assert_str(String(JoinAvailability.evaluate(_friend("away"), me))).is_empty()
	# "Ẩn trạng thái" is seen as offline without an area.
	assert_str(String(JoinAvailability.evaluate(_friend("offline", ""), me))).is_equal("offline")
	assert_str(String(JoinAvailability.evaluate(_friend("in_match", "arena"), me))).is_equal("in_match")
	assert_str(String(JoinAvailability.evaluate(_friend("online", ""), me))).is_equal("no_area")
	assert_str(String(JoinAvailability.evaluate(_friend("in_activity", "exam_zone", "exam"), me))).is_equal("exam")
	assert_str(String(JoinAvailability.evaluate(_friend("in_activity", "learning_zone.math", "learning"), me))) \
			.is_equal("in_activity")
	assert_str(String(JoinAvailability.evaluate(null, me))).is_equal("offline")


func test_reasons_of_the_student() -> void:
	var f: FriendModel = _friend("online")
	assert_str(String(JoinAvailability.evaluate(f, {"offline": true}))).is_equal("self_offline")
	assert_str(String(JoinAvailability.evaluate(f, {"blocking": &"boss"}))).is_equal("busy")
	assert_str(String(JoinAvailability.evaluate(f, {"exam": true}))).is_equal("busy")
	assert_str(String(JoinAvailability.evaluate(f, {"pending": true}))).is_equal("pending")


func test_every_reason_has_words() -> void:
	for reason: StringName in JoinAvailability.REASONS:
		assert_bool(L10n.has_key(JoinAvailability.reason_key(reason))).override_failure_message(String(reason)).is_true()


func test_profile_panel_shows_the_button_grey_with_its_reason() -> void:
	var panel: FriendProfilePanel = auto_free(FriendProfilePanel.new()) as FriendProfilePanel
	add_child(panel)
	panel.show_friend(_friend("in_match", "arena"), false)
	panel.set_join_state(&"in_match")
	var b: Button = panel.button(FriendProfilePanel.ACT_JOIN)
	assert_bool(b.visible).is_true()
	assert_bool(b.disabled).is_true()
	assert_str(panel.join_reason_text()).is_equal(TranslationServer.translate("ui.friends.join_unavailable.in_match"))
	panel.set_join_state(&"")
	assert_bool(b.disabled).is_false()
	assert_str(panel.join_reason_text()).is_empty()
