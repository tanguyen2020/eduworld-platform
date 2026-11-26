extends GdUnitTestSuite
## T026 (F102-AC-19, F102-AC-20; AGENTS R1, R16, ADR-0064): the party data stays in memory and leaves with the student —
## `ServiceLocator.end_session()` (logout, `session.session.replaced` → AuthService) and a new session clear PartyStore
## and AppState.party; no party script writes to disk; the party scenes call only PartyService / read PartyStore (no
## ApiClient, ApiRequest, RealtimeClient or command in `scenes/**` party files); the cards show nickname, level,
## presence and Ready only.

const PARTY_SCRIPTS: PackedStringArray = [
	"res://scripts/multiplayer/party_service.gd", "res://scripts/multiplayer/party_store.gd",
	"res://scripts/multiplayer/models/party_model.gd", "res://scripts/multiplayer/models/party_member_model.gd",
	"res://scripts/multiplayer/models/invitation_model.gd", "res://scripts/multiplayer/models/eligible_activity_model.gd",
]
const PARTY_SCENES: PackedStringArray = [
	"res://scenes/social/party_lobby.gd", "res://scenes/social/party_member_card.gd",
	"res://scenes/social/invite_friend_picker.gd", "res://scenes/social/activity_selector.gd",
	"res://scenes/social/invitation_toast.gd", "res://scenes/social/party_dialog.gd", "res://scenes/social/party_forms.gd",
	"res://scenes/social/party_lobby_dialogs.gd",
	"res://scenes/ui/hud/party_hud.gd",
]
## Network entry points a scene script must never touch (GODOT_ARCHITECTURE §6).
const FORBIDDEN_IN_SCENES: PackedStringArray = [
	"ApiClient", "RestApiClient", "ApiRequest", "RealtimeClient", "send_command", "send_request", "HTTPRequest",
	"WebSocketPeer",
]


func _code(path: String) -> String:
	var lines: PackedStringArray = []
	for line: String in FileAccess.get_file_as_string(path).split("\n"):
		var stripped: String = line.strip_edges()
		if stripped.begins_with("#"):
			continue
		var hash_at: int = line.find("##")
		lines.append(line.substr(0, hash_at) if hash_at >= 0 else line)
	return "\n".join(lines)


func test_logout_and_takeover_clear_the_party() -> void:
	var h: AuthHarness = AuthHarness.create()
	var services: ServiceLocator = ServiceLocator.new(h.api, null, h.clock)
	services.party.store.self_id = PartyFixtures.LEADER
	services.party.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	services.party.store.set_invitations(InvitationModel.list_from(PartyFixtures.invitations()["data"]))
	services.party.friends.append(PartyMemberModel.from_card((PartyFixtures.friends()["data"] as Array)[0]))
	services.end_session()
	assert_bool(services.party.store.has_party()).is_false()
	assert_int(services.party.store.invitations.size()).is_equal(0)
	assert_int(services.party.friends.size()).is_equal(0)
	assert_str(services.party.store.self_id).is_empty()
	# A new session (next student) starts with nothing either.
	services.party.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	services.start_session({})
	assert_bool(services.party.store.has_party()).is_false()
	h.dispose()


func test_app_state_follows_the_store_and_is_cleared() -> void:
	var h: AuthHarness = AuthHarness.create()
	var holder: Node = auto_free(Node.new()) as Node
	holder.set_script(load("res://scripts/core/app_state.gd"))
	var svc: PartyService = PartyService.new(h.api, null, holder)
	svc.store.self_id = PartyFixtures.LEADER
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	assert_object(holder.get("party")).is_not_null()
	svc.clear()
	assert_object(holder.get("party")).is_null()
	h.dispose()


func test_no_party_script_writes_to_disk() -> void:
	for path: String in PARTY_SCRIPTS + PARTY_SCENES:
		var code: String = _code(path)
		for forbidden: String in ["user://", "FileAccess", "ConfigFile", "DirAccess", "ResourceSaver"]:
			assert_str(code).override_failure_message("%s uses %s" % [path, forbidden]).not_contains(forbidden)


func test_party_scenes_only_use_the_service_and_the_store() -> void:
	for path: String in PARTY_SCENES:
		var code: String = _code(path)
		for forbidden: String in FORBIDDEN_IN_SCENES:
			assert_str(code).override_failure_message("%s references %s (AGENTS R1)" % [path, forbidden]) \
					.not_contains(forbidden)


func test_cards_show_only_public_fields() -> void:
	var data: Dictionary = PartyFixtures.party()
	var first: Dictionary = (data["members"] as Array)[1] as Dictionary
	first["real_name"] = "Nguyen Van B"
	first["school_name"] = "Truong Tieu hoc X"
	first["grade_level"] = 3
	var p: PartyModel = PartyModel.from_rest(data)
	var card: PartyMemberCard = auto_free(PartyMemberCard.new()) as PartyMemberCard
	add_child(card)
	var m: PartyMemberModel = p.member(PartyFixtures.MEMBER)
	m.level = 10
	card.bind(m, false, false)
	var text: String = " ".join(card.texts())
	assert_str(text).contains("CaHeoVui07")
	assert_str(text).contains("10")
	for leaked: String in ["Nguyen", "Truong", "Tieu hoc", "3A"]:
		assert_str(text).not_contains(leaked)
