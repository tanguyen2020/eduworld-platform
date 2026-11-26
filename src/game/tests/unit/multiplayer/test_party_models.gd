extends GdUnitTestSuite
## T004 (F102 data-model §1, §3; F102-AC-06, F102-AC-19, AGENTS R16): every party fixture generated from the contract
## examples (tools/gen_fixtures.mjs: OpenAPI `Party`, `PartyInvitation`, `PartyEligibleActivity`, `PlayerCard`, the
## response examples and the AsyncAPI `party.*` examples) parses into the typed models; unknown fields (a real name, a
## school, a grade) are never copied; a payload without a valid id is dropped; realtime server fixtures pass the
## envelope and payload checks of WsMessageSchemas.

const ASYNC_DIR: String = "res://tests/fixtures/contract/asyncapi"


func test_rest_party_parses_with_members_roles_and_version() -> void:
	var p: PartyModel = PartyModel.from_rest(PartyFixtures.party())
	assert_object(p).is_not_null()
	assert_str(p.party_id).is_equal(PartyFixtures.PARTY_ID)
	assert_str(p.name).is_equal("Nhóm Sao Băng")
	assert_int(p.state_version).is_equal(7)
	assert_int(p.members.size()).is_equal(3)
	assert_str(p.leader().nickname).is_equal("MeoXanh42")
	assert_bool(p.member(PartyFixtures.LEADER).is_leader()).is_true()
	assert_bool(p.member(PartyFixtures.MEMBER).is_leader()).is_false()
	assert_bool(p.member(PartyFixtures.WAITING).ready).is_false()
	assert_str(p.activity_kind).is_equal("quest")
	assert_str(p.activity_id).is_equal("math_g3_multiplication")
	assert_int(p.not_ready_count()).is_equal(1)
	# REST members carry no level: unknown (0), never invented.
	assert_int(p.member(PartyFixtures.MEMBER).level).is_equal(0)
	assert_object(p.member(PartyFixtures.MEMBER).avatar()).is_not_null()
	# Generated schema sample parses too.
	assert_object(PartyModel.from_rest(PartyFixtures.openapi("Party"))).is_not_null()


func test_snapshot_parses_public_players_with_level() -> void:
	var p: PartyModel = PartyModel.from_snapshot(PartyFixtures.ws("state.snapshot.lobby"))
	assert_object(p).is_not_null()
	assert_int(p.members.size()).is_equal(3)
	assert_int(p.member(PartyFixtures.LEADER).level).is_equal(12)
	assert_str(p.member(PartyFixtures.MEMBER).presence).is_equal("online")
	assert_dict(p.activity_title).is_equal({DataRead.PLAIN_TEXT_KEY: "Phép nhân ở Vương quốc Toán"})
	var running: PartyModel = PartyModel.from_snapshot(PartyFixtures.ws("state.snapshot.in_activity"))
	assert_bool(running.is_in_activity()).is_true()
	assert_str(running.room_id).is_equal("0193b0a0-0000-7000-8000-000000000201")


func test_unknown_fields_never_reach_the_models() -> void:
	var data: Dictionary = PartyFixtures.party()
	var first: Dictionary = (data["members"] as Array)[0] as Dictionary
	first["real_name"] = "Nguyen Van A"
	first["school_name"] = "Truong A"
	first["grade_level"] = 3
	first["class_name"] = "3A"
	var m: PartyMemberModel = PartyModel.from_rest(data).members[0]
	var props: PackedStringArray = []
	for prop: Dictionary in m.get_property_list():
		props.append(String(prop["name"]))
	for forbidden: String in ["real_name", "school_name", "grade_level", "class_name"]:
		assert_bool(forbidden in props).override_failure_message(forbidden).is_false()
	assert_str(JSON.stringify(m.avatar_config)).not_contains("Nguyen")


func test_invalid_payloads_are_dropped() -> void:
	assert_object(PartyModel.from_rest({"name": "x"})).is_null()
	assert_object(PartyModel.from_rest("bad")).is_null()
	assert_object(PartyMemberModel.from_rest({"student_id": "nope", "nickname": "A"})).is_null()
	assert_object(PartyMemberModel.from_rest({"student_id": PartyFixtures.LEADER, "nickname": "x".repeat(33)})).is_null()
	assert_object(InvitationModel.from_dict({"invitation_id": PartyFixtures.INVITATION_ID})).is_null()
	assert_object(EligibleActivityModel.from_dict({"activity_type": "unknown", "activity_ref": "a"})).is_null()
	# A party keeps at most 8 members (platform cap) and no duplicate.
	var data: Dictionary = PartyFixtures.party()
	var members: Array = data["members"] as Array
	members.append((members[0] as Dictionary).duplicate(true))
	assert_int(PartyModel.from_rest(data).members.size()).is_equal(3)


func test_invitations_parse_from_rest_and_realtime() -> void:
	var list: Array[InvitationModel] = InvitationModel.list_from(PartyFixtures.invitations()["data"])
	assert_int(list.size()).is_equal(1)
	assert_str(list[0].inviter_nickname).is_equal("MeoXanh42")
	assert_str(list[0].party_name).is_equal("Nhóm Sao Băng")
	assert_int(list[0].expires_at_ms).is_greater(0)
	var ws: InvitationModel = InvitationModel.from_dict(PartyFixtures.ws("invitation.received.friend"))
	assert_int(ws.member_count).is_equal(3)
	assert_int(ws.seconds_left(ws.expires_at_ms - 61_000)).is_equal(61)
	assert_bool(ws.is_expired(ws.expires_at_ms)).is_true()
	assert_bool(ws.is_expired(ws.expires_at_ms - 1)).is_false()
	assert_object(InvitationModel.from_dict(PartyFixtures.openapi("PartyInvitation"))).is_not_null()


func test_eligible_activities_and_friend_cards_parse() -> void:
	var list: Array[EligibleActivityModel] = EligibleActivityModel.list_from(PartyFixtures.eligible()["data"])
	assert_int(list.size()).is_equal(3)
	assert_bool(list[0].all_eligible).is_true()
	var boss: EligibleActivityModel = list[2]
	assert_bool(boss.all_eligible).is_false()
	assert_int(boss.ineligible().size()).is_equal(1)
	assert_str(String(boss.ineligible()[0]["reason_code"])).is_equal("grade_not_allowed")
	assert_object(EligibleActivityModel.from_dict(PartyFixtures.openapi("PartyEligibleActivity"))).is_not_null()
	var friends: Array = PartyFixtures.friends()["data"] as Array
	var f: PartyMemberModel = PartyMemberModel.from_card(friends[0])
	assert_str(f.nickname).is_equal("CaHeoVui07")
	assert_int(f.level).is_equal(10)
	assert_str(PartyMemberModel.from_card(friends[2]).presence).is_equal("offline")
	assert_object(PartyMemberModel.from_card(PartyFixtures.openapi("PlayerCard"))).is_not_null()


func test_reason_words_use_the_activity_requirement_only() -> void:
	var text: String = EligibleActivityModel.reason_text("grade_not_allowed", "grade_levels:4")
	assert_str(text).contains("4")
	assert_str(EligibleActivityModel.reason_text("grade_not_allowed", "")).is_equal(tr("ui.party.reason.grade_other"))
	assert_str(EligibleActivityModel.reason_text("something_new", "")).is_equal(tr("ui.party.reason.generic"))


func test_every_party_server_fixture_passes_the_message_checks() -> void:
	var count: int = 0
	for file: String in DirAccess.get_files_at(ASYNC_DIR):
		if not file.begins_with("party."):
			continue
		count += 1
		var env: WsEnvelope = WsEnvelope.parse(FileAccess.get_file_as_string(ASYNC_DIR.path_join(file)))
		assert_object(env).override_failure_message(file).is_not_null()
		assert_bool(WsMessageSchemas.is_known(env.type)).override_failure_message(file).is_true()
		assert_bool(WsMessageSchemas.validate(env.type, env.payload)).override_failure_message(file).is_true()
	assert_int(count).is_greater_equal(16)
