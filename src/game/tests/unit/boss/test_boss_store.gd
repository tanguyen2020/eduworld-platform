extends GdUnitTestSuite
## T010 (F110-AC-02, F110-AC-03, F110-AC-09; PARTY_SYSTEM §6, GAME_NETWORKING §4.2): a party session with
## `room_type = boss` routes to the boss arena on the encounter (not a zone scene); once the arena is on screen the
## client says `boss.encounter.ready` (`ready = true`); `boss.encounter.updated` follows `state_version` (older /
## equal dropped, a gap reads `getBossEncounter`); the phase timer follows the server deadline on the server clock
## (≤ 1 s); without connection no answer is sent, and back online the encounter is read again, ready is said again and
## the open question of this student comes back with the same `question_instance_id`.

var s: F110SceneHarness = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	SceneRouter.party_room_id = ""
	s.dispose()


func test_party_session_with_room_type_boss_routes_to_the_boss_arena() -> void:
	await s.with_party()
	var routed: Array[String] = []
	s.services.party.boss_router = func(encounter: String, visual: String) -> Error:
		routed.append("%s|%s" % [encounter, visual])
		return OK
	var started: Dictionary = PartyFixtures.as_self(RealtimeHarness.fixture("party.session.started.quest")["payload"], s.self_id) as Dictionary
	started["room_type"] = "boss"
	started["room_id"] = F110Fixtures.ENCOUNTER_ID
	started["scene_ref"] = "boss/fraction_dragon"
	s.push("party.session.started", started)
	assert_int(s.services.party.enter_session()).is_equal(OK)
	assert_array(routed).is_equal(["%s|boss/fraction_dragon" % F110Fixtures.ENCOUNTER_ID])


func test_scene_router_opens_the_boss_arena_screen_on_the_encounter() -> void:
	var opened: Array = []
	var sink: Callable = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	EventBus.ui_requested.connect(sink)
	assert_int(SceneRouter.enter_boss_encounter(F110Fixtures.ENCOUNTER_ID, "boss/fraction_dragon")).is_equal(OK)
	assert_int(SceneRouter.enter_boss_encounter("not-a-uuid")).is_equal(ERR_INVALID_PARAMETER)
	EventBus.ui_requested.disconnect(sink)
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("boss_arena")
	assert_str(String((opened[0][1] as Dictionary)["encounter_id"])).is_equal(F110Fixtures.ENCOUNTER_ID)
	assert_str(String((opened[0][1] as Dictionary)["mode"])).is_equal("party")
	assert_bool(UiScreenRegistry.has(&"boss_arena")).is_true()


func test_entering_reads_the_encounter_then_says_ready() -> void:
	var ready: Array = []
	var waiter: Callable = func() -> void: ready.append(await s.services.boss.enter(F110Fixtures.ENCOUNTER_ID))
	waiter.call()
	await s.settle(4)
	var sent: Array[Dictionary] = s.sent("boss.encounter.ready")
	assert_int(sent.size()).is_equal(1)
	var p: Dictionary = sent[0]["payload"] as Dictionary
	assert_str(String(p["encounter_id"])).is_equal(F110Fixtures.ENCOUNTER_ID)
	assert_bool(bool(p["ready"])).is_true()
	assert_int(s.h.http.count(F110Fixtures.ROUTE_ENCOUNTER)).is_equal(1)
	s.ack_last("boss.encounter.ready")
	await s.settle(2)
	assert_str(String(AppState.blocking_activity)).is_equal("boss")


func test_updates_follow_state_version_and_a_gap_reads_the_encounter_again() -> void:
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	var store: BossStore = s.services.boss.store
	assert_bool(store.apply_message("boss.encounter.updated", F110Fixtures.updated(2, 80.0))).is_true()
	assert_float(store.encounter.energy_remaining_pct).is_equal(80.0)
	# The same or an older version is dropped (energy never goes back up from a stale copy).
	assert_bool(store.apply_message("boss.encounter.updated", F110Fixtures.updated(2, 95.0))).is_false()
	assert_bool(store.apply_message("boss.encounter.updated", F110Fixtures.updated(1, 99.0))).is_false()
	assert_float(store.encounter.energy_remaining_pct).is_equal(80.0)
	var reads: int = s.h.http.count(F110Fixtures.ROUTE_ENCOUNTER)
	assert_bool(store.apply_message("boss.encounter.updated", F110Fixtures.updated(5, 10.0))).is_false()
	await s.settle(4)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_ENCOUNTER)).is_equal(reads + 1)
	# Another encounter's message is ignored.
	var other: Dictionary = F110Fixtures.updated(3, 1.0)
	other["encounter_id"] = F110Fixtures.OTHER_ENCOUNTER_ID
	assert_bool(store.apply_message("boss.encounter.updated", other)).is_false()


func test_phase_timer_follows_the_server_deadline_and_hides_seconds_for_young_grades() -> void:
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	var now: int = s.services.boss.now_ms()
	s.push("boss.phase.started", F110Fixtures.phase_started(0, false, now + 90_000))
	var left: int = s.services.boss.phase_left_ms()
	assert_int(absi(left - 90_000)).is_less_equal(1000)
	var timer: PhaseTimer = auto_free(PhaseTimer.new()) as PhaseTimer
	add_child(timer)
	timer.render(left, "g4_5", "0:p1")
	assert_str(timer.clock_text()).is_equal("01:30")
	timer.render(left, "g1_3", "0:p1")
	assert_str(timer.clock_text()).is_empty()
	assert_float(timer.bar_value()).is_greater(0.9)
	# A changed device wall clock changes nothing: the realtime offset / monotonic time is used.
	s.h.clock.unix += 3600.0
	assert_int(absi(s.services.boss.phase_left_ms() - left)).is_less_equal(1000)


func test_offline_locks_answers_and_back_online_restores_the_open_question() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200,
			F110Fixtures.encounter(s.self_id, "party", "phase_active", 3))
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	s.push("boss.question.assigned", F110Fixtures.question_assigned(["bq_p1_1"]))
	assert_str(s.services.boss.store.current_question().question_instance_id).is_equal("bq_p1_1")
	s.rt.peer().server_close(1006)
	s.rt.client.tick()
	assert_bool(s.services.boss.is_offline()).is_true()
	var sent: bool = await s.services.boss.submit({"option_id": "o_b"})
	assert_bool(sent).is_false()
	assert_int(s.sent("boss.answer.submit").size()).is_equal(0)
	# Back online (here: on another device the server keeps the question) — read again, ready again, same question.
	var e: Dictionary = F110Fixtures.encounter(s.self_id, "party", "phase_active", 4)
	e["my_questions"] = [F110Fixtures.rest_question("bq_p1_1")]
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200, e)
	var reads: int = s.h.http.count(F110Fixtures.ROUTE_ENCOUNTER)
	var readies: int = s.sent("boss.encounter.ready").size()
	s.rt.advance_ms(700)
	await s.rt.await_ticket()
	s.rt.open_with_welcome(1, "rt-new")
	s.rt.push({"v": 1, "type": "system.session.resumed", "seq": s.rt.client.session.last_seq + 1, "ts": 1,
		"payload": {"replayed": 0}})
	await s.settle(8)
	assert_bool(s.services.boss.is_offline()).is_false()
	assert_int(s.h.http.count(F110Fixtures.ROUTE_ENCOUNTER)).is_equal(reads + 1)
	assert_int(s.sent("boss.encounter.ready").size()).is_equal(readies + 1)
	var q: BossQuestionModel = s.services.boss.store.current_question()
	assert_object(q).is_not_null()
	assert_str(q.question_instance_id).is_equal("bq_p1_1")
	assert_int(s.services.boss.store.encounter.state_version).is_equal(4)


func test_a_disconnected_member_is_faded_with_words() -> void:
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	var list: ContributionList = auto_free(ContributionList.new()) as ContributionList
	add_child(list)
	var members: Array[BossMemberModel] = BossMemberModel.list_from([
		F110Fixtures.member(s.self_id, "MeoXanh42"), F110Fixtures.member(F110Fixtures.MATE, F110Fixtures.MATE_NICK, true, false)])
	list.show_members(members, s.self_id, 0, false, true)
	assert_str(String(list.chip(F110Fixtures.MATE).theme_type_variation)).is_equal("PartyMemberCardAway")
	assert_array(list.chip_texts(F110Fixtures.MATE)).contains([TranslationServer.translate("ui.boss.member.reconnecting")])
