extends GdUnitTestSuite
## T028 (F102 quickstart #1–#9, TC-PTY-001…006; F102-AC-01, AC-03, AC-04, AC-07, AC-08, AC-10, AC-11 live part): against
## the backend compose with the party seed, two students each with a real realtime connection: `hs.an.3a` (A) creates a
## party and invites `hs.binh.3a` (B, a friend); B receives `party.invitation.received` and accepts (a party B was left
## in by an earlier run is left first); A sees B join (`party.member.joined`) and B's Ready (`party.member.ready_changed`)
## within the SC-002 budget; A chooses an activity every member is eligible for and starts; both receive
## `party.session.started` with the same room. The party is disbanded at the end so the run can be repeated. Skipped
## unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend), like the other live integration tests — the F102 ACs that need
## it stay "pending live run" until it passes in the owner's environment (F092 note: EW_REALTIME_PUBLIC_URL set).

const SKIP_REASON: String = "needs the backend compose with the party seed and EW_REALTIME_PUBLIC_URL (EDUWORLD_IT_API_BASE_URL)"
const LEADER: String = "hs.an.3a"
const FRIEND: String = "hs.binh.3a"
## Frames waited for a socket / a message (SC-002: ≤ 1 s p95 for Ready; generous for CI machines).
const WAIT_FRAMES: int = 900
## SC-002 / AC-UX-104: a friend's Ready shows within 1 s on a normal network.
const READY_BUDGET_MS: int = 1000


class Player:
	extends RefCounted
	var live: LiveBackend = null
	var realtime: GameRealtimeClient = null
	var party: PartyService = null
	var student_id: String = ""


func _wait(condition: Callable) -> bool:
	for i: int in WAIT_FRAMES:
		if bool(condition.call()):
			return true
		await get_tree().process_frame
	return bool(condition.call())


func _join(username: String) -> Player:
	var p: Player = Player.new()
	p.live = LiveBackend.create(self, Uuid.v4())
	await p.live.auth.login(username, LiveBackend.student_password())
	p.student_id = DataRead.str_of(DataRead.dict_of(AppState.player, "student"), "student_id")
	p.realtime = GameRealtimeClient.new()
	add_child(p.realtime)
	p.realtime.configure(p.live.api, p.live.api.clock)
	p.party = PartyService.new(p.live.api, p.realtime)
	p.party.store.self_id = p.student_id
	p.realtime.start(p.live.auth.sessions.game_session_id)
	return p


func _leave_any(p: Player) -> void:
	await p.party.load_my_party()
	if p.party.store.has_party():
		if p.party.store.is_leader():
			await p.party.disband()
		else:
			await p.party.leave()


func _dispose(p: Player) -> void:
	if p == null:
		return
	p.realtime.unregister_handler(PartyService.PREFIX, p.party.handle_message)
	p.realtime.stop()
	p.realtime.queue_free()
	await p.live.auth.logout()
	p.live.dispose()


func test_two_students_make_a_party_get_ready_and_start(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var a: Player = await _join(LEADER)
	var b: Player = await _join(FRIEND)
	assert_bool(Uuid.is_valid(a.student_id) and Uuid.is_valid(b.student_id)).is_true()
	assert_bool(await _wait(func() -> bool: return a.realtime.is_online() and b.realtime.is_online())) \
			.override_failure_message("sockets offline (EW_REALTIME_PUBLIC_URL?)").is_true()
	await _leave_any(a)
	await _leave_any(b)

	# F102-AC-01: create (one key per intent).
	var created: ApiResult = await a.party.create_party()
	assert_bool(created.ok).override_failure_message("createParty %d %s" % [created.status, created.code]).is_true()
	assert_bool(a.party.store.is_leader()).is_true()

	# F102-AC-03: B receives the invitation in real time, then accepts it.
	var invited: ApiResult = await a.party.invite(b.student_id)
	assert_bool(invited.ok).override_failure_message("inviteToParty %d %s" % [invited.status, invited.code]).is_true()
	assert_bool(await _wait(func() -> bool: return not b.party.store.invitations.is_empty())) \
			.override_failure_message("no party.invitation.received").is_true()
	var inv: InvitationModel = b.party.store.invitations[0]
	assert_int(inv.seconds_left(b.party.now_ms())).is_greater(0)
	var accepted: ApiResult = await b.party.accept(inv)
	assert_bool(accepted.ok).override_failure_message("accept %d %s" % [accepted.status, accepted.code]).is_true()

	# F102-AC-07: A sees B join in order.
	assert_bool(await _wait(func() -> bool: return a.party.store.party.member(b.student_id) != null)) \
			.override_failure_message("no party.member.joined").is_true()

	# F102-AC-08 / SC-002: B's Ready reaches A within the budget.
	var sent_at: int = Time.get_ticks_msec()
	assert_bool(await b.party.set_ready(true)).is_true()
	assert_bool(await _wait(func() -> bool: return a.party.store.party.member(b.student_id).ready)).is_true()
	var ready_ms: int = Time.get_ticks_msec() - sent_at
	print("F102 live: Ready shown to the leader after %d ms" % ready_ms)
	assert_int(ready_ms).is_less_equal(READY_BUDGET_MS)

	# F102-AC-10: an activity everyone is eligible for, then everyone ready and start (AC-11).
	assert_bool((await a.party.load_eligible()).ok).is_true()
	var pick: EligibleActivityModel = null
	for act: EligibleActivityModel in a.party.eligible:
		if act.all_eligible and act.min_members <= 2:
			pick = act
			break
	assert_object(pick).override_failure_message("no activity every member is eligible for").is_not_null()
	assert_bool(await a.party.select_activity(pick)).is_true()
	assert_bool(await _wait(func() -> bool: return b.party.store.party.activity_id == pick.activity_ref)).is_true()
	await b.party.set_ready(true)
	await a.party.set_ready(true)
	assert_bool(await _wait(func() -> bool: return a.party.store.party.not_ready_count() == 0)).is_true()
	var started: ApiResult = await a.party.start_session()
	assert_bool(started.ok).override_failure_message("start %d %s" % [started.status, started.code]).is_true()
	assert_bool(await _wait(func() -> bool: return not b.party.store.session.is_empty())) \
			.override_failure_message("no party.session.started").is_true()
	assert_str(String(b.party.store.session["room_id"])).is_equal(String(a.party.store.session["room_id"]))

	await a.party.disband()
	await _dispose(b)
	await _dispose(a)
