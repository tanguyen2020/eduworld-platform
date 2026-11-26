extends GdUnitTestSuite
## T007 (F102-AC-01, F102-AC-02 service part, F102-AC-04, F102-AC-05; T012/T020): PartyService on a RealtimeHarness
## (FakeHttp + FakeWebSocketPeer, TEST-ONLY) with the party contract fixtures of tools/gen_fixtures.mjs:
## - `createParty` sends one Idempotency-Key per intent, the same key when the intent is retried after a network
##   failure, a new key for a new intent; the created party is in the store;
## - `acceptPartyInvitation` → `409 PARTY_ALREADY_IN_PARTY` sends nothing else (no silent leave) and no notice; the
##   confirmed switch sends `leaveParty` then a new accept;
## - every party Problem becomes a friendly sentence key, never the raw code; unknown codes the generic sentence;
## - the friends of the invite picker are the server list as it came (no filter by grade, school or class);
## - start: `412 VERSION_MISMATCH` reloads the party and is not re-sent; `409 PARTY_ACTIVITY_NOT_ELIGIBLE` keeps the
##   members named by the server; leader writes (kick, transfer, disband, settings) go to their routes.

var h: RealtimeHarness = null
var svc: PartyService = null
var notices: Array[String] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	svc = PartyService.new(h.api, h.client)
	svc.store.self_id = PartyFixtures.LEADER
	notices.clear()
	svc.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))


func after_test() -> void:
	h.client.unregister_handler(PartyService.PREFIX, svc.handle_message)
	h.dispose()


func _key(route: String) -> String:
	return String(h.http.last(route).headers.get("idempotency-key", ""))


# ---------- F102-AC-01 ----------

func test_create_party_keeps_the_key_while_the_intent_is_retried() -> void:
	h.http.fail(PartyFixtures.ROUTE_CREATE)
	var first: ApiResult = await svc.create_party()
	assert_bool(first.ok).is_false()
	assert_bool(first.network_error).is_true()
	var key_1: String = _key(PartyFixtures.ROUTE_CREATE)
	assert_bool(Uuid.is_valid(key_1)).is_true()
	# Every automatic retry of ApiClient used the same key.
	for rec: FakeHttp.Recorded in h.http.find(PartyFixtures.ROUTE_CREATE):
		assert_str(String(rec.headers["idempotency-key"])).is_equal(key_1)
	h.http.clear(PartyFixtures.ROUTE_CREATE).reply(PartyFixtures.ROUTE_CREATE, 201, PartyFixtures.party_alone())
	var second: ApiResult = await svc.create_party()
	assert_bool(second.ok).is_true()
	assert_str(_key(PartyFixtures.ROUTE_CREATE)).is_equal(key_1)
	assert_bool(svc.store.has_party()).is_true()
	assert_bool(svc.store.is_leader()).is_true()
	var body: Dictionary = h.http.last(PartyFixtures.ROUTE_CREATE).body
	assert_str(String(body["party_type"])).is_equal("friend")
	assert_int(String(body["name"]).length()).is_between(1, PartyService.NAME_MAX_LENGTH)
	# A new intent (after the party is gone) gets a new key.
	svc.store.close(PartyStore.CLOSED_LEFT)
	await svc.create_party()
	assert_str(_key(PartyFixtures.ROUTE_CREATE)).is_not_equal(key_1)


func test_create_is_refused_while_in_a_party_without_a_request() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	var result: ApiResult = await svc.create_party()
	assert_str(result.code).is_equal(PartyService.CODE_ALREADY_IN_PARTY)
	assert_int(h.http.count(PartyFixtures.ROUTE_CREATE)).is_equal(0)


func test_load_my_party_404_means_no_party_without_a_notice() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	h.http.reply(PartyFixtures.ROUTE_MY_PARTY, 404, AuthHarness.problem(404, "PARTY_NOT_FOUND"))
	var result: ApiResult = await svc.load_my_party()
	assert_int(result.status).is_equal(404)
	assert_bool(svc.store.has_party()).is_false()
	assert_array(notices).is_empty()


# ---------- F102-AC-04 ----------

func test_already_in_party_asks_first_then_leave_and_accept() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	h.http.reply(PartyFixtures.ROUTE_INVITATIONS, 200, PartyFixtures.invitations(svc.now_ms()))
	await svc.load_invitations()
	var inv: InvitationModel = svc.store.invitations[0]
	h.http.reply(PartyFixtures.ROUTE_ACCEPT, 409, AuthHarness.problem(409, "PARTY_ALREADY_IN_PARTY"))
	var refused: ApiResult = await svc.accept(inv)
	assert_str(refused.code).is_equal(PartyService.CODE_ALREADY_IN_PARTY)
	assert_int(h.http.count(PartyFixtures.ROUTE_LEAVE)).is_equal(0)
	assert_array(notices).is_empty()
	assert_bool(svc.store.has_party()).is_true()
	var refused_key: String = _key(PartyFixtures.ROUTE_ACCEPT)
	# The student confirmed "Rời nhóm hiện tại và tham gia?".
	h.http.reply(PartyFixtures.ROUTE_LEAVE, 204)
	h.http.clear(PartyFixtures.ROUTE_ACCEPT).reply(PartyFixtures.ROUTE_ACCEPT, 200, PartyFixtures.party(PartyFixtures.NEWCOMER))
	var joined: ApiResult = await svc.switch_and_accept(inv)
	assert_bool(joined.ok).is_true()
	var order: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.requests:
		if rec.path.ends_with(":leave") or rec.path.ends_with(":accept"):
			order.append(rec.path.get_slice(":", 1))
	assert_array(Array(order)).is_equal(["accept", "leave", "accept"])
	assert_str(_key(PartyFixtures.ROUTE_ACCEPT)).is_not_equal(refused_key)
	assert_bool(svc.store.has_party()).is_true()
	assert_int(svc.store.invitations.size()).is_equal(0)


func test_expired_invitation_leaves_the_list_with_a_friendly_sentence() -> void:
	h.http.reply(PartyFixtures.ROUTE_INVITATIONS, 200, PartyFixtures.invitations(svc.now_ms()))
	await svc.load_invitations()
	h.http.reply(PartyFixtures.ROUTE_ACCEPT, 410, AuthHarness.problem(410, "PARTY_INVITATION_EXPIRED"))
	await svc.accept(svc.store.invitations[0])
	assert_int(svc.store.invitations.size()).is_equal(0)
	assert_array(notices).is_equal(["ui.party.error.PARTY_INVITATION_EXPIRED"])


# ---------- F102-AC-05 ----------

func test_every_party_problem_has_a_friendly_sentence_without_the_code() -> void:
	for code: String in ["PARTY_FULL", "PARTY_INVITATION_EXPIRED", "PARTY_INVITE_RATE_LIMITED", "PARTY_INVITE_NOT_ALLOWED",
			"PARTY_POLICY_VIOLATION", "PARTY_NOT_LEADER", "PARTY_ACTIVITY_NOT_ELIGIBLE", "PARTY_NOT_READY"]:
		var key: String = PartyService.error_key(code)
		assert_str(key).is_equal("ui.party.error." + code)
		for locale: String in ["vi", "en"]:
			TranslationServer.set_locale(locale)
			var text: String = tr(key)
			assert_str(text).override_failure_message("%s %s missing" % [locale, key]).is_not_equal(key)
			assert_str(text).not_contains(code)
	TranslationServer.set_locale("vi")
	assert_str(tr("ui.party.error.PARTY_FULL")).is_equal("Nhóm đã đủ người")
	assert_str(tr("ui.party.error.PARTY_INVITATION_EXPIRED")).is_equal("Lời mời đã hết hạn")
	assert_str(PartyService.error_key("SOMETHING_NEW")).is_equal(PartyService.KEY_GENERIC_ERROR)


func test_invite_refused_by_the_server_shows_the_sentence_of_its_code() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	h.http.reply(PartyFixtures.ROUTE_INVITE, 403, AuthHarness.problem(403, "PARTY_INVITE_NOT_ALLOWED"))
	var refused: ApiResult = await svc.invite(PartyFixtures.NEWCOMER)
	assert_bool(refused.ok).is_false()
	assert_array(notices).is_equal(["ui.party.error.PARTY_INVITE_NOT_ALLOWED"])
	assert_bool(svc.invited.has(PartyFixtures.NEWCOMER)).is_false()
	h.http.clear(PartyFixtures.ROUTE_INVITE).reply(PartyFixtures.ROUTE_INVITE, 403,
			AuthHarness.problem(403, "PARTY_POLICY_VIOLATION"))
	await svc.invite(PartyFixtures.NEWCOMER)
	assert_str(notices.back()).is_equal("ui.party.error.PARTY_POLICY_VIOLATION")
	h.http.clear(PartyFixtures.ROUTE_INVITE).reply(PartyFixtures.ROUTE_INVITE, 201, PartyFixtures.openapi("PartyInvitation"))
	var ok: ApiResult = await svc.invite(PartyFixtures.NEWCOMER)
	assert_bool(ok.ok).is_true()
	assert_bool(svc.invited.has(PartyFixtures.NEWCOMER)).is_true()
	assert_str(String(h.http.last(PartyFixtures.ROUTE_INVITE).body["invitee_id"])).is_equal(PartyFixtures.NEWCOMER)
	assert_bool(Uuid.is_valid(_key(PartyFixtures.ROUTE_INVITE))).is_true()


# ---------- F102-AC-02 (service part) ----------

func test_friends_are_the_server_list_as_it_came() -> void:
	h.http.reply(PartyFixtures.ROUTE_FRIENDS, 200, PartyFixtures.friends())
	var result: ApiResult = await svc.load_friends()
	assert_bool(result.ok).is_true()
	var names: PackedStringArray = []
	for f: PartyMemberModel in svc.friends:
		names.append(f.nickname)
	assert_array(Array(names)).is_equal(["CaHeoVui07", "SaoBang12", "ThoTrang03"])
	# The request asks for friends only; no grade, school or class parameter is ever sent.
	var query: String = h.http.last(PartyFixtures.ROUTE_FRIENDS).query
	for forbidden: String in ["grade", "school", "class"]:
		assert_str(query).not_contains(forbidden)


# ---------- start / leader writes ----------

func test_start_version_mismatch_reloads_and_is_not_resent() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party_all_ready()))
	h.http.reply(PartyFixtures.ROUTE_START, 412, AuthHarness.problem(412, "VERSION_MISMATCH"))
	h.http.reply(PartyFixtures.ROUTE_GET_PARTY, 200, PartyFixtures.party_all_ready())
	var result: ApiResult = await svc.start_session()
	assert_int(result.status).is_equal(412)
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count(PartyFixtures.ROUTE_START)).is_equal(1)
	assert_int(h.http.count(PartyFixtures.ROUTE_GET_PARTY)).is_equal(1)
	assert_int(int(h.http.last(PartyFixtures.ROUTE_START).body["expected_version"])).is_equal(7)
	assert_array(notices).contains(["ui.party.error.VERSION_MISMATCH"])


func test_start_not_eligible_keeps_the_members_named_by_the_server() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party_all_ready()))
	h.http.reply(PartyFixtures.ROUTE_START, 409, AuthHarness.problem(409, "PARTY_ACTIVITY_NOT_ELIGIBLE", {
		"ineligible_members": [{"student_id": PartyFixtures.WAITING, "reason_code": "grade_not_allowed",
			"requirement": "grade_levels:4"}]}))
	await svc.start_session()
	assert_int(svc.store.ineligible.size()).is_equal(1)
	assert_str(String(svc.store.ineligible[0]["student_id"])).is_equal(PartyFixtures.WAITING)
	assert_array(notices).is_equal(["ui.party.error.PARTY_ACTIVITY_NOT_ELIGIBLE"])


func test_start_success_records_the_session_from_the_answer() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party_all_ready()))
	h.http.reply(PartyFixtures.ROUTE_START, 201, PartyFixtures.session_start())
	var started: Array[Dictionary] = []
	svc.store.session_started.connect(func(info: Dictionary) -> void: started.append(info))
	var result: ApiResult = await svc.start_session()
	assert_bool(result.ok).is_true()
	assert_int(started.size()).is_equal(1)
	assert_str(String(started[0]["scene_ref"])).is_equal("res://scenes/world/zones/learning_zone_math/Zone.tscn")
	# The push of the same session does not start it twice.
	svc.handle_message("party.session.started", PartyFixtures.ws("session.started.quest"))
	assert_int(started.size()).is_equal(1)
	assert_bool(Uuid.is_valid(_key(PartyFixtures.ROUTE_START))).is_true()


func test_leader_writes_go_to_their_routes() -> void:
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party()))
	h.http.reply(PartyFixtures.ROUTE_KICK, 204)
	assert_bool((await svc.kick(PartyFixtures.WAITING)).ok).is_true()
	assert_object(svc.store.party.member(PartyFixtures.WAITING)).is_null()
	assert_str(h.http.last(PartyFixtures.ROUTE_KICK).path).contains(PartyFixtures.WAITING)
	var moved: Dictionary = PartyFixtures.party()
	moved["leader_id"] = PartyFixtures.MEMBER
	moved["state_version"] = 8
	h.http.reply(PartyFixtures.ROUTE_TRANSFER, 200, moved)
	assert_bool((await svc.transfer_leader(PartyFixtures.MEMBER)).ok).is_true()
	assert_str(String(h.http.last(PartyFixtures.ROUTE_TRANSFER).body["new_leader_id"])).is_equal(PartyFixtures.MEMBER)
	assert_bool(svc.store.is_leader()).is_false()
	h.http.reply(PartyFixtures.ROUTE_SETTINGS, 200, PartyFixtures.party())
	await svc.update_settings({"visibility": "friends"})
	var rec: FakeHttp.Recorded = h.http.last(PartyFixtures.ROUTE_SETTINGS)
	assert_str(String(rec.headers.get("content-type", ""))).contains("merge-patch")
	assert_str(String(rec.body["visibility"])).is_equal("friends")
	h.http.reply(PartyFixtures.ROUTE_DISBAND, 204)
	assert_bool((await svc.disband()).ok).is_true()
	assert_bool(svc.store.has_party()).is_false()
	assert_array(notices).contains(["ui.party.disbanded"])
