extends GdUnitTestSuite
## T020 (F110-AC-14, F110-AC-15, F110-AC-16; QUEST_SYSTEM §3.2, ADR-0016): `quest.roles.assigned` gives the student's
## role(s) and the other roles by nickname only (no card); "Chọn vai" is offered to the leader only while the server
## waits for the leader, and `403 PARTY_NOT_LEADER` is a friendly sentence; the panel shows exactly the cards the server
## returned (own + shared) and nothing is written to disk; "Chia sẻ dữ kiện" keeps its key when retried; a repeated
## `quest.clue.shared` never adds a card twice.

var s: F110SceneHarness = null
var coop: CoopQuestService = null
var notices: Array[String] = []


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	coop = s.services.coop_quest
	notices = []
	coop.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))


func after_test() -> void:
	s.dispose()


func test_roles_give_my_role_and_the_others_by_nickname_only() -> void:
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id))
	await s.settle(3)
	var mine: PackedStringArray = coop.store.my_roles(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	assert_array(mine).is_equal([F110Fixtures.ROLE_MEASURE])
	var roles: Array[RoleAssignmentModel] = coop.store.roles(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	assert_int(roles.size()).is_equal(2)
	var other: RoleAssignmentModel = roles[1]
	assert_bool(other.is_me).is_false()
	assert_str(other.nickname).is_equal(F110Fixtures.MATE_NICK)
	# The other member's card is not on this client.
	assert_object(coop.store.clue(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, F110Fixtures.ROLE_MAP)).is_null()
	# The role name comes from the own card the server sent.
	assert_str(L10n.pick(roles[0].title)).is_equal("Người đo")


func test_only_the_leader_waiting_for_the_choice_sees_assign_and_not_leader_is_friendly() -> void:
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id, PackedStringArray([F110Fixtures.ROLE_MEASURE]), true))
	await s.settle(2)
	assert_bool(coop.can_assign(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)).is_false()
	await s.with_party()
	assert_bool(coop.is_leader()).is_true()
	assert_bool(coop.can_assign(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)).is_true()
	s.push("quest.roles.assigned", F110Fixtures.roles(s.self_id))
	assert_bool(coop.can_assign(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)).is_false()
	s.h.http.clear(F110Fixtures.ROUTE_ROLES).reply(F110Fixtures.ROUTE_ROLES, 403, AuthHarness.problem(403, "PARTY_NOT_LEADER"))
	var a: Array[Dictionary] = [{"role": F110Fixtures.ROLE_MEASURE, "student_id": s.self_id}]
	var r: ApiResult = await coop.assign_roles(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, a)
	assert_bool(r.ok).is_false()
	assert_array(notices).contains(["ui.error.PARTY_NOT_LEADER"])
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_ROLES)
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_str(String(((rec.body["assignments"] as Array)[0] as Dictionary)["role"])).is_equal(F110Fixtures.ROLE_MEASURE)


func test_only_the_cards_the_server_returned_are_shown_and_nothing_is_written() -> void:
	var before: PackedStringArray = DirAccess.get_files_at("user://")
	await coop.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	var cards: Array[ClueCardModel] = coop.store.clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	assert_int(cards.size()).is_equal(1)
	assert_str(cards[0].role).is_equal(F110Fixtures.ROLE_MEASURE)
	assert_str(cards[0].body_text()).is_equal("Sợi dây dài 12 m.")
	assert_str(s.h.http.last(F110Fixtures.ROUTE_CLUES).path).is_equal("/game/quests/%s/steps/%s/clues" % [
		F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID])
	assert_array(DirAccess.get_files_at("user://")).is_equal(before)


func test_share_keeps_its_key_when_retried_and_puts_the_card_on_the_board() -> void:
	await coop.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	s.h.http.clear(F110Fixtures.ROUTE_SHARE).fail(F110Fixtures.ROUTE_SHARE)
	var failed: ApiResult = await coop.share(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, F110Fixtures.ROLE_MEASURE)
	assert_bool(failed.network_error).is_true()
	var key: String = String(s.h.http.last(F110Fixtures.ROUTE_SHARE).headers["idempotency-key"])
	s.h.http.clear(F110Fixtures.ROUTE_SHARE).reply(F110Fixtures.ROUTE_SHARE, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, true, "MeoXanh42")]))
	var ok: ApiResult = await coop.share(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, F110Fixtures.ROLE_MEASURE)
	assert_bool(ok.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_SHARE)
	assert_str(String(rec.headers["idempotency-key"])).is_equal(key)
	assert_str(rec.path).is_equal("/game/quests/%s/steps/%s/clues/%s:share" % [F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID,
		F110Fixtures.ROLE_MEASURE])
	assert_bool(coop.store.clue(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, F110Fixtures.ROLE_MEASURE).shared).is_true()


func test_a_repeated_clue_shared_never_adds_a_card_twice() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 200, F110Fixtures.clues(
			PackedStringArray([F110Fixtures.ROLE_MEASURE]), [F110Fixtures.clue(F110Fixtures.ROLE_MEASURE, false),
			F110Fixtures.clue(F110Fixtures.ROLE_MAP, true, F110Fixtures.MATE_NICK)]))
	s.push("quest.clue.shared", F110Fixtures.clue_shared(F110Fixtures.ROLE_MAP, F110Fixtures.MATE_NICK))
	await s.settle(3)
	var reads: int = s.h.http.count(F110Fixtures.ROUTE_CLUES)
	assert_int(coop.store.clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID).size()).is_equal(2)
	s.push("quest.clue.shared", F110Fixtures.clue_shared(F110Fixtures.ROLE_MAP, F110Fixtures.MATE_NICK))
	await s.settle(3)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_CLUES)).is_equal(reads)
	assert_int(coop.store.clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID).size()).is_equal(2)
	var board: SharedClueBoard = auto_free(SharedClueBoard.new()) as SharedClueBoard
	add_child(board)
	var doubled: Array[ClueCardModel] = coop.store.clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	doubled.append_array(coop.store.clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID))
	board.show_cards(doubled)
	assert_int(board.card_count()).is_equal(1)
	assert_array(board.roles()).is_equal([F110Fixtures.ROLE_MAP])


func test_a_gone_step_closes_the_panel_and_reads_the_quests_again() -> void:
	var closed: Array[String] = []
	coop.step_closed.connect(func(q: String, st: String) -> void: closed.append("%s|%s" % [q, st]))
	s.h.http.reply("GET /game/quests", 200, {"data": [], "page": {"has_more": false}})
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await coop.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	await s.settle(3)
	assert_array(closed).is_equal(["%s|%s" % [F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID]])
	assert_int(s.h.http.count("GET /game/quests")).is_greater_equal(1)
	assert_array(notices).is_empty()
