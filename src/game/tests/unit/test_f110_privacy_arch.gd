extends GdUnitTestSuite
## T032 (F110-AC-29; AGENTS R1, R16, ADR-0064): other players are only nickname / avatar / level / level title in the
## F110 models; friends, clue cards and the boss state stay in RAM (no `user://` file is written by F110 code) and are
## cleared on logout or `session.session.replaced`; the F110 scene scripts only call public methods of services —
## never ApiClient / RealtimeClient, never a private `_method` of a service.

const SCENE_FILES: PackedStringArray = [
	"res://scenes/challenge/boss/boss_arena.gd", "res://scenes/challenge/boss/boss_info_card.gd",
	"res://scenes/challenge/boss/boss_result.gd", "res://scenes/challenge/boss/contribution_list.gd",
	"res://scenes/challenge/boss/question_panel_host.gd", "res://scenes/challenge/boss/boss_energy_bar.gd",
	"res://scenes/challenge/boss/phase_timer.gd", "res://scenes/challenge/boss/boss_figure.gd",
	"res://scenes/quest/interdependent_step_panel.gd", "res://scenes/quest/clue_card.gd",
	"res://scenes/quest/shared_clue_board.gd", "res://scenes/quest/role_assign_dialog.gd",
	"res://scenes/quest/party_progress_strip.gd", "res://scenes/social/friends_screen.gd",
	"res://scenes/social/friend_row.gd", "res://scenes/social/friend_list_view.gd",
	"res://scenes/social/friend_profile_panel.gd", "res://scenes/social/friend_request_list.gd",
	"res://scenes/social/find_friends_tab.gd", "res://scenes/social/friend_code_widget.gd",
	"res://scenes/social/block_list.gd", "res://scenes/social/friends_ui.gd",
]
const SERVICE_FILES: PackedStringArray = [
	"res://scripts/challenge/boss_client.gd", "res://scripts/challenge/boss_store.gd",
	"res://scripts/quest/coop_quest_service.gd", "res://scripts/quest/coop_quest_store.gd",
	"res://scripts/social/social_service.gd", "res://scripts/social/friends_store.gd",
]
const FORBIDDEN_FIELDS: PackedStringArray = ["full_name", "real_name", "school", "school_id", "school_name", "grade",
	"grade_level", "class_id", "email", "score", "mastery"]

var s: F110SceneHarness = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	s.dispose()


func test_models_of_other_players_have_no_personal_or_learning_field() -> void:
	for obj: Object in [FriendModel.new(), FriendRequestModel.new(), RelationModel.new(), BossMemberModel.new(),
			RoleAssignmentModel.new(), ClueCardModel.new()]:
		for p: Dictionary in obj.get_property_list():
			assert_bool(String(p["name"]) in FORBIDDEN_FIELDS) \
					.override_failure_message("%s.%s" % [obj.get_script().resource_path, p["name"]]).is_false()


func test_scene_scripts_call_no_network_client_and_no_private_service_method() -> void:
	var private_call: RegEx = RegEx.create_from_string("(client|social|coop|services\\.[a-z_]+|boss)\\._[a-z]")
	for path: String in SCENE_FILES:
		var text: String = FileAccess.get_file_as_string(path)
		assert_bool(text.is_empty()).override_failure_message("missing %s" % path).is_false()
		for name: String in ["ApiClient", "RealtimeClient", "HTTPRequest", "WebSocketPeer"]:
			assert_bool(RegEx.create_from_string("(?<![A-Za-z0-9_])%s(?![A-Za-z0-9_])" % name).search(text) == null) \
					.override_failure_message("%s uses %s" % [path, name]).is_true()
		assert_object(private_call.search(text)).override_failure_message("%s calls a private service method" % path).is_null()


func test_f110_code_writes_nothing_to_disk() -> void:
	for path: String in SCENE_FILES + SERVICE_FILES:
		var text: String = FileAccess.get_file_as_string(path)
		for needle: String in ["user://", "FileAccess.open", "ConfigFile", "store_string", "save_png"]:
			assert_bool(text.contains(needle)).override_failure_message("%s: %s" % [path, needle]).is_false()


func test_logout_and_session_replaced_clear_every_f110_store() -> void:
	await s.services.social.load_friends()
	await s.services.social.create_code()
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	s.services.coop_quest.handle_message("quest.roles.assigned", F110Fixtures.roles(s.self_id))
	s.services.presence.hidden = true
	assert_bool(s.services.social.store.friends.is_empty()).is_false()
	# `session.session.replaced` / logout → AuthService → AppState.clear_player → ServiceLocator.end_session.
	s.services.end_session()
	assert_bool(s.services.social.store.friends.is_empty()).is_true()
	assert_object(s.services.social.store.code).is_null()
	assert_object(s.services.boss.store.encounter).is_null()
	assert_bool(s.services.coop_quest.store.is_empty()).is_true()
	assert_bool(s.services.presence.hidden).is_false()
	assert_str(String(AppState.blocking_activity)).is_equal("")


func test_a_boss_phase_only_raises_the_bell_count() -> void:
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	assert_bool(s.services.boss_running()).is_true()
	assert_bool(s.services.notifications.is_quiet()).is_true()
	assert_bool(s.services.party.should_popup(null)).is_false()
	s.services.boss.store.clear()
	assert_bool(s.services.notifications.is_quiet()).is_false()
