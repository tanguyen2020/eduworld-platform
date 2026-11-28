extends GdUnitTestSuite
## T040 (F111-AC-30; AGENTS R1, R16, ADR-0064): other players are only nickname / avatar / level / club role in the F111
## models; the club, event, season and suggestion stores stay in RAM (no `user://` file is written by F111 code) and are
## cleared on logout or `session.session.replaced`; the F111 scene scripts only call public methods of services — never
## ApiClient / RealtimeClient / HTTPRequest / WebSocketPeer, never a private `_method` of a service.

const SCENE_FILES: PackedStringArray = ["res://scenes/social/suggestion_group.gd", "res://scenes/social/club_tab.gd",
	"res://scenes/social/club_card.gd", "res://scenes/social/club_page.gd", "res://scenes/social/club_create_dialog.gd",
	"res://scenes/social/club_invite_picker.gd", "res://scenes/social/friends_clubs.gd",
	"res://scenes/social/find_friends_tab.gd", "res://scenes/social/friend_profile_panel.gd",
	"res://scenes/social/friends_screen.gd", "res://scenes/ui/hud/event_center_button.gd",
	"res://scenes/liveops/event_center_screen.gd", "res://scenes/liveops/event_card.gd",
	"res://scenes/liveops/event_detail_panel.gd", "res://scenes/liveops/community_goal_bar.gd",
	"res://scenes/liveops/quest_day_tab.gd", "res://scenes/liveops/season_tab.gd", "res://scenes/liveops/season_track_view.gd"]
const SERVICE_FILES: PackedStringArray = ["res://scripts/social/club_service.gd", "res://scripts/social/club_store.gd",
	"res://scripts/social/club_form.gd", "res://scripts/social/join_availability.gd",
	"res://scripts/multiplayer/join_friend_service.gd", "res://scripts/liveops/live_event_service.gd",
	"res://scripts/liveops/live_event_store.gd", "res://scripts/liveops/season_track_service.gd",
	"res://scripts/liveops/season_track_store.gd", "res://scripts/services/keyed_write_service.gd",
	"res://scripts/quest/quest_go.gd"]
const FORBIDDEN_FIELDS: PackedStringArray = ["full_name", "real_name", "school", "school_id", "school_name", "grade",
	"grade_level", "class_id", "email", "score", "mastery", "owner_id", "requester_school"]

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	s.dispose()


func test_models_of_other_players_have_no_personal_or_learning_field() -> void:
	for obj: Object in [FriendSuggestionModel.new(), ClubModel.new(), ClubMemberModel.new(), ClubInvitationModel.new(),
			ClubJoinRequestModel.new(), ClubGoalModel.new(), CommunityGoalModel.new(), LiveEventModel.new()]:
		for p: Dictionary in obj.get_property_list():
			assert_bool(String(p["name"]) in FORBIDDEN_FIELDS) \
					.override_failure_message("%s.%s" % [obj.get_script().resource_path, p["name"]]).is_false()


func test_scene_scripts_call_no_network_client_and_no_private_service_method() -> void:
	var private_call: RegEx = RegEx.create_from_string(
			"(clubs|live|season_service|social|join_friend|services\\.[a-z_]+)\\._[a-z]")
	for path: String in SCENE_FILES:
		var text: String = FileAccess.get_file_as_string(path)
		assert_bool(text.is_empty()).override_failure_message("missing %s" % path).is_false()
		for name: String in ["ApiClient", "RealtimeClient", "HTTPRequest", "WebSocketPeer", "send_request", "send_keyed"]:
			assert_bool(RegEx.create_from_string("(?<![A-Za-z0-9_])%s(?![A-Za-z0-9_])" % name).search(text) == null) \
					.override_failure_message("%s uses %s" % [path, name]).is_true()
		assert_object(private_call.search(text)).override_failure_message("%s calls a private service method" % path).is_null()


func test_f111_code_writes_nothing_to_disk() -> void:
	for path: String in SCENE_FILES + SERVICE_FILES:
		var text: String = FileAccess.get_file_as_string(path)
		for needle: String in ["user://", "FileAccess.open", "ConfigFile", "store_string", "save_png"]:
			assert_bool(text.contains(needle)).override_failure_message("%s: %s" % [path, needle]).is_false()


func test_logout_and_session_replaced_clear_every_f111_store() -> void:
	await s.services.clubs.load_lists("g6_7")
	await s.services.clubs.open_club(F111Fixtures.CLUB_OWN)
	await s.services.live_events.load_list()
	await s.services.season.load_track()
	await s.services.social.list_suggestions()
	assert_bool(s.services.clubs.store.mine.is_empty()).is_false()
	assert_bool(s.services.social.store.suggestions.is_empty()).is_false()
	# `session.session.replaced` / logout → AuthService → AppState.clear_player → ServiceLocator.end_session.
	s.services.end_session()
	assert_bool(s.services.clubs.store.mine.is_empty()).is_true()
	assert_object(s.services.clubs.store.page).is_null()
	assert_bool(s.services.clubs.store.invitations.is_empty()).is_true()
	assert_bool(s.services.live_events.store.events.is_empty()).is_true()
	assert_object(s.services.season.store.track).is_null()
	assert_bool(s.services.social.store.suggestions.is_empty()).is_true()
	assert_bool(s.services.join_friend.is_busy()).is_false()
