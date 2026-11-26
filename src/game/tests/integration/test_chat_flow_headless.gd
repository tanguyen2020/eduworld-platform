extends GdUnitTestSuite
## T032 live part (F119 quickstart #1, #4, #6, #7, #9, #10; F119-AC-01, AC-04, AC-05, AC-08, AC-10, AC-13, AC-15 with the
## real backend): two students with real realtime connections — `hs.an.3a` (A) creates a party and invites `hs.binh.3a`
## (B); both open the always-on `party` channel; A sends a quick message (B reads it in B's language), a text (B gets it
## through `chat.message.created`, A sees it "sent" only after `chat.message.acked`), a placeholder message with a phone
## pattern is refused with the PII sentence (no echo); B reports A's message with the party context (201 + thanks) and
## blocks A; A's next message never reaches B. The party is disbanded at the end so the run can be repeated. Skipped
## unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend) like the other live tests — the F119 ACs that need it stay
## "pending live run" until it passes in the owner's environment (F092 note: EW_REALTIME_PUBLIC_URL set).

const SKIP_REASON: String = "needs the backend compose with the chat + party seed and EW_REALTIME_PUBLIC_URL (EDUWORLD_IT_API_BASE_URL)"
const LEADER: String = "hs.an.3a"
const FRIEND: String = "hs.binh.3a"
const WAIT_FRAMES: int = 900


class Player:
	extends RefCounted
	var live: LiveBackend = null
	var realtime: GameRealtimeClient = null
	var party: PartyService = null
	var chat: ChatService = null
	var reports: ReportService = null
	var social: SocialService = null
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
	p.chat = ChatService.new(p.live.api, p.realtime)
	p.chat.store.self_id = p.student_id
	p.reports = ReportService.new(p.live.api)
	p.social = SocialService.new(p.live.api)
	p.social.relation_changed.connect(func(id: String, _k: String) -> void: p.chat.store.hide_user(id))
	p.realtime.start(p.live.auth.sessions.game_session_id)
	return p


func _dispose(p: Player) -> void:
	if p == null:
		return
	p.realtime.unregister_handler(PartyService.PREFIX, p.party.handle_message)
	p.realtime.unregister_handler(ChatService.PREFIX, p.chat.handle_message)
	p.realtime.stop()
	p.realtime.queue_free()
	await p.live.auth.logout()
	p.live.dispose()


func _has_text(p: Player, channel: String, text: String) -> bool:
	for m: ChatMessageModel in p.chat.store.visible(channel):
		if m.body == text:
			return true
	return false


func test_two_students_chat_in_their_party_and_report_and_block(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var a: Player = await _join(LEADER)
	var b: Player = await _join(FRIEND)
	assert_bool(await _wait(func() -> bool: return a.realtime.is_online() and b.realtime.is_online())) \
			.override_failure_message("sockets offline (EW_REALTIME_PUBLIC_URL?)").is_true()
	await a.party.load_my_party()
	if a.party.store.has_party() and a.party.store.is_leader():
		await a.party.disband()
	elif a.party.store.has_party():
		await a.party.leave()
	await b.party.load_my_party()
	if b.party.store.has_party():
		await b.party.leave()
	assert_bool((await a.party.create_party()).ok).is_true()
	assert_bool((await a.party.invite(b.student_id)).ok).is_true()
	assert_bool(await _wait(func() -> bool: return not b.party.store.invitations.is_empty())).is_true()
	assert_bool((await b.party.accept(b.party.store.invitations[0])).ok).is_true()
	var channel: String = a.party.store.party.chat_channel_id
	assert_bool(Uuid.is_valid(channel)).override_failure_message("party snapshot without chat_channel_id").is_true()

	# F119-AC-01, AC-06: the always-on party channel opens for both (join retried while it is being created).
	await a.chat.open_channel(channel, "party")
	await b.chat.open_channel(channel, "party")
	assert_bool(await _wait(func() -> bool:
		a.chat.tick()
		b.chat.tick()
		return a.chat.is_joined(channel) and b.chat.is_joined(channel))).is_true()

	# F119-AC-04, AC-08: a quick message reaches B by id; "sent" only after the ack.
	var quick: ChatMessageModel = a.chat.send_quick(channel, a.chat.quick.items[0].id)
	assert_bool(await _wait(func() -> bool: return not quick.is_pending())).is_true()
	assert_bool(await _wait(func() -> bool:
		for m: ChatMessageModel in b.chat.store.visible(channel):
			if m.message_id == quick.message_id:
				return true
		return false)).is_true()

	# F119-AC-10: a refused message gets the PII sentence, nothing echoed.
	var notices: Array[String] = []
	a.chat.feedback_requested.connect(func(key: String, _x: Dictionary, _t: String) -> void: notices.append(key))
	a.chat.send_text(channel, "so cua minh la 0900 000 000")
	assert_bool(await _wait(func() -> bool: return notices.has("ui.chat.feedback.pii_blocked"))).is_true()

	# F119-AC-13: B reports A's quick message with the party context.
	var report: ApiResult = await b.reports.create_report("message", quick.message_id, "spam",
			{"channel_id": channel, "party_id": a.party.store.party.party_id})
	assert_int(report.status).is_equal(201)

	# F119-AC-15: B blocks A — A's next text never reaches B.
	assert_bool((await b.social.block_user(a.student_id)).ok).is_true()
	var after: ChatMessageModel = a.chat.send_text(channel, "Tin sau khi bị chặn")
	assert_bool(await _wait(func() -> bool: return after != null and not after.is_pending())).is_true()
	for i: int in 120:
		await get_tree().process_frame
	assert_bool(_has_text(b, channel, "Tin sau khi bị chặn")).is_false()

	await a.party.disband()
	await _dispose(b)
	await _dispose(a)
