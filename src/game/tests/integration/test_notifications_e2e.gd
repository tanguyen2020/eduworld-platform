extends GdUnitTestSuite
## T026 (F123-AC-01, F123-AC-04, F123-AC-15, F123-AC-16; quickstart #1): against the backend compose, `hs.binh.3a` is in
## the game with a real realtime connection; `hs.an.3a` sends a friend request (F095 `sendFriendRequest`) and the
## receiving client gets `notification.item.created` (`kind = friend_request`, the sender's nickname only) — one toast
## and the bell badge + 1 — then the request is declined again so the run can be repeated (a pending request from an
## earlier run is declined first). Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend), like the other live
## integration tests.

const SKIP_REASON: String = "needs the backend compose with the F095 + F123 seed (EDUWORLD_IT_API_BASE_URL)"
const RECEIVER: String = "hs.binh.3a"
const SENDER: String = "hs.an.3a"
const PATH_REQUESTS: String = "/social/friend-requests"
const PATH_REJECT: String = "/social/friend-requests/%s:reject"
## Frames waited for the socket to come online and for the notification (SC-001: ≤ 2 s p95; generous for CI).
const WAIT_FRAMES: int = 900


func _wait(condition: Callable) -> bool:
	for i: int in WAIT_FRAMES:
		if bool(condition.call()):
			return true
		await get_tree().process_frame
	return bool(condition.call())


func _post(live: LiveBackend, path: String, body: Variant = null) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post(path, body)
	req.idempotency_key = Uuid.v4()
	return await live.api.send(req)


## Declines every pending request of `requester_id` to the signed-in receiver (left over by an earlier run).
func _decline_from(receiver: LiveBackend, requester_id: String) -> void:
	var incoming: ApiResult = await receiver.api.send(ApiRequest.make_get(PATH_REQUESTS, {"direction": "incoming"}))
	if not incoming.ok:
		return
	for raw: Variant in DataRead.arr_of(incoming.body(), "data"):
		var row: Dictionary = raw as Dictionary
		var requester: Dictionary = DataRead.dict_of(row, "requester")
		if DataRead.str_of(requester, "user_id") == requester_id or DataRead.str_of(requester, "student_id") == requester_id:
			await _post(receiver, PATH_REJECT % DataRead.str_of(row, "request_id"))


func test_a_friend_request_reaches_the_receiving_client(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var receiver: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await receiver.auth.login(RECEIVER, LiveBackend.student_password())
	var receiver_id: String = DataRead.str_of(DataRead.dict_of(AppState.player, "user"), "user_id")
	assert_bool(Uuid.is_valid(receiver_id)).override_failure_message("receiver bootstrap without user_id").is_true()
	var realtime: GameRealtimeClient = GameRealtimeClient.new()
	add_child(realtime)
	realtime.configure(receiver.api, receiver.api.clock)
	var notes: NotificationService = NotificationService.new(receiver.api, realtime)
	assert_bool((await notes.load_unread()).ok).is_true()
	var before: int = notes.badge
	var toasts: Array[NotificationModel] = []
	notes.toast_requested.connect(func(item: NotificationModel) -> void: toasts.append(item))
	realtime.start(receiver.auth.sessions.game_session_id)
	assert_bool(await _wait(func() -> bool: return realtime.is_online())).override_failure_message("socket offline") \
			.is_true()

	var sender: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await sender.auth.login(SENDER, LiveBackend.student_password())
	var sender_id: String = DataRead.str_of(DataRead.dict_of(AppState.player, "user"), "user_id")
	var sent: ApiResult = await _post(sender, PATH_REQUESTS, {"addressee_id": receiver_id})
	if sent.status == 409:
		await _decline_from(receiver, sender_id)
		sent = await _post(sender, PATH_REQUESTS, {"addressee_id": receiver_id})
	assert_int(sent.status).override_failure_message("sendFriendRequest %d %s" % [sent.status, sent.code]).is_equal(201)

	assert_bool(await _wait(func() -> bool: return not toasts.is_empty())).override_failure_message("no notification") \
			.is_true()
	var item: NotificationModel = toasts[0]
	assert_str(item.kind).is_equal("friend_request")
	assert_str(item.message_code).is_equal("friend.request_received")
	assert_array(item.params.keys()).contains_exactly(["nickname"])
	assert_int(notes.badge).is_equal(before + 1)
	assert_str(item.display_text()).contains(String(item.params["nickname"]))

	await _post(receiver, PATH_REJECT % DataRead.str_of(sent.body(), "request_id"))
	realtime.unregister_handler(NotificationService.PREFIX, notes.handle_message)
	realtime.stop()
	realtime.queue_free()
	await sender.auth.logout()
	sender.dispose()
	await receiver.auth.logout()
	receiver.dispose()
