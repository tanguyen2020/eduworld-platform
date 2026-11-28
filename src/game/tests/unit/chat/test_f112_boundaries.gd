extends GdUnitTestSuite
## T031, T032 (F112-AC-30, AC-31; TC-CHAT-010, TC-AUTH-014): the F112 code only talks REST `/chat/*` + `/social/*` through
## the ApiClient and the realtime client — no NATS, no Valkey, no moderation list or rule, no chat branch on the grade
## band, no BBCode on text from a student or the server; the F112 screens show only nickname / avatar / level / club role
## of other players; the DM map, the search and the club tools live in RAM and are cleared on logout / takeover;
## `403 GAME_ACCESS_STUDENT_ONLY` goes to the F061 flow; a `404` club closes the tools.

const F112_FILES: PackedStringArray = [
	"res://scripts/chat/chat_hub_state.gd", "res://scripts/chat/chat_hub_service.gd", "res://scripts/chat/direct_peer_map.gd",
	"res://scripts/chat/highlight_text.gd", "res://scripts/chat/chat_search_service.gd",
	"res://scripts/chat/models/persistent_channel_tab.gd", "res://scripts/social/club_manage_service.gd",
	"res://scripts/social/club_form_options.gd", "res://scenes/chat/chat_hub.gd", "res://scenes/chat/chat_hub_button.gd",
	"res://scenes/chat/chat_friends_list.gd", "res://scenes/chat/chat_search_panel.gd", "res://scenes/social/club_manage.gd",
]
const FORBIDDEN: PackedStringArray = ["nats", "valkey", "redis", "grade_band", "grade_level", "bbcode_enabled = true",
	"append_text(", "parse_bbcode(", "FileAccess.open(", "ConfigFile", "user://"]

var s: F112SceneHarness = null


func after_test() -> void:
	if s != null:
		s.dispose()
		s = null


func _code(path: String) -> String:
	var text: String = FileAccess.get_file_as_string(path)
	var lines: PackedStringArray = []
	for line: String in text.split("\n"):
		var t: String = line.strip_edges()
		if t.begins_with("#"):
			continue
		lines.append(line)
	return "\n".join(lines)


func test_no_broker_no_grade_branch_no_bbcode_no_disk() -> void:
	for path: String in F112_FILES:
		assert_bool(FileAccess.file_exists(path)).override_failure_message(path).is_true()
		var code: String = _code(path).to_lower()
		for w: String in FORBIDDEN:
			assert_bool(code.contains(w.to_lower())).override_failure_message("%s uses %s" % [path, w]).is_false()


func test_scenes_never_call_the_network() -> void:
	for path: String in F112_FILES:
		if not path.begins_with("res://scenes/"):
			continue
		var code: String = _code(path)
		for w: String in ["ApiRequest", "send_request(", "send_command(", "RestApiClient", "HttpTransport"]:
			assert_bool(code.contains(w)).override_failure_message("%s calls %s" % [path, w]).is_false()


func test_screens_show_only_public_cards() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	var root: Control = auto_free(Control.new()) as Control
	add_child(root)
	root.size = Vector2(1280, 720)
	var all: PackedStringArray = []
	for view: String in ["hub_friends", "search_hits", "manage_officers"]:
		var screen: Control = await F112Screens.build(view, root, s.services, s.h.http)
		for node: Node in screen.find_children("*", "Label", true, false):
			if (node as Label).is_visible_in_tree():
				all.append(TranslationServer.translate((node as Label).text))
		if screen.has_method("close"):
			screen.call("close")
	var text: String = " ".join(all).to_lower()
	for w: String in ["họ tên", "full_name", "trường th", "school_name", "khối 3", "grade", "lớp 3a", "mastery", "@example"]:
		assert_bool(text.contains(w)).override_failure_message("'%s' on an F112 screen" % w).is_false()


func test_ram_only_and_cleared_on_logout() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	await s.services.chat_hub.refresh()
	await s.services.chat_hub.open_dm(F112Fixtures.FRIEND_NEW)
	await s.services.chat_search.search("di boss")
	await s.services.club_manage.load_club(F111Fixtures.CLUB_OWN)
	assert_bool(s.services.chat_hub.state.tabs.is_empty()).is_false()
	s.services.end_session()
	assert_bool(s.services.chat_hub.state.tabs.is_empty()).is_true()
	assert_str(s.services.chat_hub.state.peers.channel_of(F112Fixtures.FRIEND_NEW)).is_empty()
	assert_bool(s.services.chat_search.hits.is_empty()).is_true()
	assert_object(s.services.club_manage.club).is_null()


func test_student_only_refusal_goes_to_f061_and_404_closes_tools() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	var denied: Array[String] = []
	s.services.club_manage.access_denied.connect(func(code: String) -> void: denied.append(code))
	s.h.http.clear(F112Fixtures.ROUTE_CLUB).reply(F112Fixtures.ROUTE_CLUB, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	await s.services.club_manage.load_club(F111Fixtures.CLUB_OWN)
	assert_array(denied).is_equal(["GAME_ACCESS_STUDENT_ONLY"])
	var revoked: Array[String] = []
	s.services.club_manage.tools_revoked.connect(func(id: String) -> void: revoked.append(id))
	s.h.http.clear(F112Fixtures.ROUTE_CLUB).reply(F112Fixtures.ROUTE_CLUB, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.club_manage.load_club(F111Fixtures.CLUB_OWN)
	assert_array(revoked).is_equal([F111Fixtures.CLUB_OWN])
