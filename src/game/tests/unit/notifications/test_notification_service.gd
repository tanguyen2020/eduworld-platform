extends GdUnitTestSuite
## F123-AC-15, F123-AC-16 (service part), F123-AC-17, F123-AC-18 (T024; F064-AC-21 kept): NotificationService on a
## RealtimeHarness (FakeHttp + FakeWebSocketPeer, TEST-ONLY) with the contract fixtures of tools/gen_fixtures.mjs
## (`openapi/listNotifications.200.json`, `asyncapi/notification.item.created.*`):
## - entering the world / a failed resume / a degraded `notification` service → `listNotifications?unread=true`, the badge
##   is the server `unread_count`, no toast for old notifications;
## - `notification.item.created` → badge + 1 at once, one toast for invitations and school work, none for level-ups,
##   achievements, report outcomes, with popups off or during an exam / a challenge; a replayed id is ignored;
## - rows seen → WS `notification.item.mark_read` (≤ 100 ids), REST `markNotificationRead` when the socket is down;
##   "Đã xem hết" → `markAllNotificationsRead`;
## - opening an item → marked read + `EventBus.ui_requested` of its action (assignment → "Nhà trường giao"); no screen
##   in this client yet (friend requests) → false;
## - texts from `ui.notification.<message_code>` + params, else title/body, else a generic sentence; params bounded and
##   never logged (R14); the first assignments refresh of a session gives one grouped local toast (F064-AC-21).

const ROUTE_LIST: String = "GET /notifications"
const ROUTE_READ: String = "POST /notifications/*:read"
const ROUTE_READ_ALL: String = "POST /notifications:read-all"
const ROUTE_ASSIGNMENTS: String = "GET /learning/my-assignments"
const FIXTURES: String = "res://tests/fixtures/contract/openapi"
const NICK: String = "SaoBang12"


class SettingsStub:
	extends RefCounted
	var notification_popups: bool = true


var h: RealtimeHarness = null
var svc: NotificationService = null
var stub: SettingsStub = null
var toasts: Array[NotificationModel] = []
var badges: Array[int] = []
var quiet: bool = false


func before_test() -> void:
	h = RealtimeHarness.create()
	stub = SettingsStub.new()
	quiet = false
	svc = NotificationService.new(h.api, h.client, stub)
	svc.quiet_check = func() -> bool: return quiet
	toasts.clear()
	badges.clear()
	svc.toast_requested.connect(func(item: NotificationModel) -> void: toasts.append(item))
	svc.badge_changed.connect(func(n: int) -> void: badges.append(n))


func after_test() -> void:
	h.client.unregister_handler(NotificationService.PREFIX, svc.handle_message)
	h.dispose()


static func _list_fixture() -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/listNotifications.200.json" % FIXTURES)) as Dictionary
			).duplicate(true)


static func _item(kind: String, code: String, params: Dictionary = {}, read: bool = false, at: String = "") -> Dictionary:
	var action: Dictionary = {"kind": "open_assignment", "id": Uuid.v7()}
	match kind:
		"friend_request":
			action["kind"] = "open_friend_requests"
		"level_reached":
			action["kind"] = "open_profile"
	return {"notification_id": Uuid.v7(), "kind": kind, "message_code": code, "params": params, "read": read,
		"action": action, "created_at": at if not at.is_empty() else "2026-11-21T08:00:00Z"}


static func _page(items: Array, unread: int, next: String = "") -> Dictionary:
	return {"data": items, "page": {"next_cursor": next if not next.is_empty() else null, "has_more": not next.is_empty(),
		"limit": 20}, "unread_count": unread}


## The AsyncAPI example of `name` with a fresh notification id (a replay keeps the id).
func _created(name: String, fresh_id: bool = true) -> Dictionary:
	var env: Dictionary = RealtimeHarness.fixture("notification.item.created." + name)
	if fresh_id:
		(env["payload"] as Dictionary)["notification_id"] = Uuid.v7()
	return env


func _deliver(env: Dictionary) -> void:
	h.next_seq += 1
	env["seq"] = h.next_seq
	env["ts"] = h.client.server_now_ms()
	h.push(env)


# ---------- F123-AC-15: badge from the unread page ----------

func test_entering_loads_the_unread_page_and_the_badge_without_toasts() -> void:
	h.http.reply(ROUTE_LIST, 200, _list_fixture())
	var result: ApiResult = await svc.load_unread()
	assert_bool(result.ok).is_true()
	var req: FakeHttp.Recorded = h.http.last(ROUTE_LIST)
	assert_str(req.query).contains("unread=true")
	assert_str(req.query).contains("limit=20")
	assert_int(svc.badge).is_equal(1)
	assert_int(badges.back()).is_equal(1)
	assert_int(svc.items.size()).is_equal(1)
	var first: NotificationModel = svc.items[0]
	assert_str(first.kind).is_equal("friend_request")
	assert_str(first.action_kind).is_equal("open_friend_requests")
	assert_bool(first.read).is_false()
	assert_int(first.created_at_ms).is_greater(0)
	# Old notifications never pop a burst of toasts (spec edge case).
	assert_array(toasts).is_empty()


func test_unread_count_counts_every_page() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "Bài 1"})], 37, "c2"))
	await svc.load_unread()
	assert_int(svc.badge).is_equal(37)
	assert_str(NotificationService.badge_text(svc.badge)).is_equal("37")
	assert_str(NotificationService.badge_text(120)).is_equal("99+")
	assert_str(NotificationService.badge_text(0)).is_empty()


func test_a_failed_load_keeps_the_badge() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([], 2))
	await svc.load_unread()
	h.http.reply(ROUTE_LIST, 503, AuthHarness.problem(503, "UNAVAILABLE"))
	var result: ApiResult = await svc.load_unread()
	assert_bool(result.ok).is_false()
	assert_int(svc.badge).is_equal(2)


func test_failed_resume_and_degraded_service_reload_over_rest() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, null, h.clock, h.client)
	h.http.reply(ROUTE_LIST, 200, _page([], 4))
	h.http.reply(ProfileFixtures.ROUTE_PROFILE, 200, ProfileFixtures.profile_ready())
	services.resync()
	for i: int in 10:
		await get_tree().process_frame
	assert_int(h.http.count(ROUTE_LIST)).is_greater_equal(1)
	assert_int(services.notifications.badge).is_equal(4)
	var before: int = h.http.count(ROUTE_LIST)
	h.client.service_degraded.emit(PackedStringArray(["notification"]))
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count(ROUTE_LIST)).is_greater(before)
	h.client.unregister_handler(NotificationService.PREFIX, services.notifications.handle_message)
	services.hub.dispose()


# ---------- F123-AC-15/16: realtime ----------

func test_realtime_invitation_raises_the_badge_and_pops_one_toast() -> void:
	await h.connect_online()
	_deliver(_created("friend_request"))
	assert_int(svc.badge).is_equal(1)
	assert_int(toasts.size()).is_equal(1)
	assert_str(toasts[0].kind).is_equal("friend_request")
	assert_str(toasts[0].display_text()).is_equal(tr("ui.notification.friend.request_received").format({"nickname": NICK}))
	assert_object(svc.items[0]).is_same(toasts[0])


func test_school_work_pops_a_toast_progression_does_not() -> void:
	await h.connect_online()
	_deliver(_created("assignment_new"))
	var level: Dictionary = _created("friend_request")
	var payload: Dictionary = level["payload"] as Dictionary
	payload["kind"] = "level_reached"
	payload["message_code"] = "progression.level_reached"
	payload["params"] = {"level": "5"}
	payload["action"] = {"kind": "open_profile"}
	_deliver(level)
	var achievement: Dictionary = _created("friend_request")
	(achievement["payload"] as Dictionary)["kind"] = "achievement_unlocked"
	(achievement["payload"] as Dictionary)["message_code"] = "achievement.unlocked"
	_deliver(achievement)
	assert_int(svc.badge).is_equal(3)
	assert_int(toasts.size()).is_equal(1)
	assert_str(toasts[0].kind).is_equal("assignment_new")
	assert_str(toasts[0].display_text()).is_equal(tr("ui.notification.assignment.new").format(
			{"title": "Phép nhân trong phạm vi 100"}))


func test_popups_off_or_exam_give_the_badge_only() -> void:
	await h.connect_online()
	stub.notification_popups = false
	_deliver(_created("friend_request"))
	stub.notification_popups = true
	quiet = true
	_deliver(_created("assignment_new"))
	assert_int(svc.badge).is_equal(2)
	assert_array(toasts).is_empty()
	quiet = false
	_deliver(_created("assignment_new"))
	assert_int(toasts.size()).is_equal(1)


func test_a_replayed_notification_is_ignored() -> void:
	await h.connect_online()
	var env: Dictionary = _created("friend_request")
	_deliver(env.duplicate(true))
	_deliver(env.duplicate(true))
	assert_int(svc.badge).is_equal(1)
	assert_int(svc.items.size()).is_equal(1)
	assert_int(toasts.size()).is_equal(1)


func test_a_payload_breaking_the_contract_never_reaches_the_service() -> void:
	await h.connect_online()
	var bad: Dictionary = _created("friend_request")
	(bad["payload"] as Dictionary).erase("kind")
	_deliver(bad)
	var bad_id: Dictionary = _created("friend_request")
	(bad_id["payload"] as Dictionary)["notification_id"] = "not-a-uuid"
	_deliver(bad_id)
	assert_int(svc.badge).is_equal(0)
	assert_array(toasts).is_empty()


func test_exam_and_challenge_make_the_service_quiet() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, null, h.clock, h.client)
	assert_bool(services.notifications.is_quiet()).is_false()
	services.exam.attempt = AttemptModel.new()
	assert_bool(services.notifications.is_quiet()).is_true()
	services.exam.submitted = true
	assert_bool(services.notifications.is_quiet()).is_false()
	h.client.unregister_handler(NotificationService.PREFIX, services.notifications.handle_message)
	services.hub.dispose()


func test_no_sound_and_nothing_outside_the_game() -> void:
	var source: String = FileAccess.get_file_as_string("res://scripts/notifications/notification_service.gd")
	for api: String in ["AudioDirector", "AudioStreamPlayer", "OS.", "Timer", "create_timer", "JavaScriptBridge",
			"request_permission", "push_token", "fcm", "apns"]:
		assert_str(source).override_failure_message(api).not_contains(api)


# ---------- F123-AC-18: marking read and opening ----------

func test_seen_rows_go_by_websocket_in_one_command() -> void:
	await h.connect_online()
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "A"}),
			_item("friend_request", "friend.request_received", {"nickname": NICK}),
			_item("level_reached", "progression.level_reached", {"level": "3"}, true)], 2))
	await svc.refresh_inbox()
	assert_bool(svc.inbox_loaded).is_true()
	svc.mark_seen(svc.items)
	assert_int(svc.badge).is_equal(0)
	var sent: Array[Dictionary] = h.peer().sent_of(NotificationService.CMD_MARK_READ)
	assert_int(sent.size()).is_equal(1)
	var ids: Array = (sent[0]["payload"] as Dictionary)["notification_ids"] as Array
	assert_int(ids.size()).is_equal(2)
	assert_array(ids).contains([svc.items[0].notification_id, svc.items[1].notification_id])
	h.ack_last(NotificationService.CMD_MARK_READ)
	for i: int in 3:
		await get_tree().process_frame
	assert_int(h.http.count(ROUTE_READ)).is_equal(0)
	# Already read → nothing more is sent.
	assert_int(await svc.mark_seen(svc.items)).is_equal(0)


func test_more_than_one_hundred_ids_are_split() -> void:
	await h.connect_online()
	var items: Array[NotificationModel] = []
	for i: int in 130:
		var m: NotificationModel = NotificationModel.from_dict(_item("assignment_new", "assignment.new", {"title": "B"}))
		items.append(m)
	svc.unread_count = 130
	svc.mark_seen(items)
	h.ack_last(NotificationService.CMD_MARK_READ)
	for i: int in 3:
		await get_tree().process_frame
	h.ack_last(NotificationService.CMD_MARK_READ)
	for i: int in 3:
		await get_tree().process_frame
	var sent: Array[Dictionary] = h.peer().sent_of(NotificationService.CMD_MARK_READ)
	assert_int(sent.size()).is_equal(2)
	assert_int(((sent[0]["payload"] as Dictionary)["notification_ids"] as Array).size()).is_equal(100)
	assert_int(((sent[1]["payload"] as Dictionary)["notification_ids"] as Array).size()).is_equal(30)


func test_offline_marks_read_over_rest() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "A"})], 1))
	h.http.reply(ROUTE_READ, 200, _item("assignment_new", "assignment.new", {}, true))
	await svc.refresh_inbox()
	var id: String = svc.items[0].notification_id
	assert_int(await svc.mark_seen(svc.items)).is_equal(1)
	assert_int(h.http.count(ROUTE_READ)).is_equal(1)
	assert_str(h.http.last(ROUTE_READ).path).is_equal("/notifications/%s:read" % id)
	assert_int(svc.badge).is_equal(0)


func test_mark_all_read() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "A"})], 9, "c2"))
	h.http.reply(ROUTE_READ_ALL, 200, {"unread_count": 0})
	await svc.refresh_inbox()
	assert_int(svc.badge).is_equal(9)
	var result: ApiResult = await svc.mark_all_read()
	assert_bool(result.ok).is_true()
	assert_int(svc.badge).is_equal(0)
	assert_bool(svc.items[0].read).is_true()


func test_load_more_follows_the_cursor() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "A"})], 2, "c2"))
	await svc.refresh_inbox()
	h.http.reply(ROUTE_LIST, 200, _page([_item("friend_request", "friend.request_received", {"nickname": NICK},
			false, "2026-11-20T08:00:00Z")], 2))
	await svc.load_more()
	assert_str(h.http.last(ROUTE_LIST).query).contains("cursor=c2")
	assert_int(svc.items.size()).is_equal(2)
	assert_str(svc.page_cursor).is_empty()


func test_opening_an_assignment_marks_it_read_and_asks_for_the_tab() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([_item("assignment_new", "assignment.new", {"title": "A"})], 1))
	h.http.reply(ROUTE_READ, 200, {})
	await svc.refresh_inbox()
	var intents: Array[Array] = []
	var record: Callable = func(screen: StringName, args: Dictionary) -> void: intents.append([screen, args])
	EventBus.ui_requested.connect(record)
	var opened: bool = svc.open(svc.items[0])
	EventBus.ui_requested.disconnect(record)
	assert_bool(opened).is_true()
	assert_array(intents).is_equal([[&"quest_journal", {"tab": "assigned"}]])
	assert_bool(svc.items[0].read).is_true()
	assert_int(svc.badge).is_equal(0)


func test_an_item_without_a_screen_in_this_client_only_gets_read() -> void:
	h.http.reply(ROUTE_LIST, 200, _list_fixture())
	h.http.reply(ROUTE_READ, 200, {})
	await svc.refresh_inbox()
	var friend: NotificationModel = svc.items[0]
	assert_bool(svc.can_open(friend)).is_false()
	var intents: Array = []
	var record: Callable = func(screen: StringName, _args: Dictionary) -> void: intents.append(screen)
	EventBus.ui_requested.connect(record)
	assert_bool(svc.open(friend)).is_false()
	EventBus.ui_requested.disconnect(record)
	assert_array(intents).is_empty()
	assert_bool(friend.read).is_true()
	var level: NotificationModel = NotificationModel.from_dict(_item("level_reached", "progression.level_reached"))
	assert_str(String(svc.target_screen(level))).is_equal("journey")


# ---------- F123-AC-17: texts ----------

func test_text_from_the_message_code_then_title_body_then_generic() -> void:
	var known: NotificationModel = NotificationModel.from_dict(_item("friend_request", "friend.request_received",
			{"nickname": NICK}))
	assert_str(known.display_text()).is_equal(tr("ui.notification.friend.request_received").format({"nickname": NICK}))
	assert_str(known.display_text()).contains(NICK)
	var unknown: Dictionary = _item("system", "system.unknown_code")
	unknown["title"] = {"vi": "Bảo trì tối nay", "en": "Maintenance tonight"}
	unknown["body"] = {"vi": "Game nghỉ lúc 22:00", "en": "The game pauses at 22:00"}
	var fallback: NotificationModel = NotificationModel.from_dict(unknown)
	assert_str(fallback.display_text()).contains(L10n.pick({"vi": "Bảo trì tối nay", "en": "Maintenance tonight"}))
	var bare: NotificationModel = NotificationModel.from_dict(_item("system", ""))
	assert_str(bare.display_text()).is_equal(tr("ui.notification.generic"))


func test_every_catalog_message_code_has_vi_and_en_texts() -> void:
	for key: String in NotificationModel.MESSAGE_KEYS:
		for locale: String in ["vi", "en"]:
			var translation: Translation = TranslationServer.get_translation_object(locale)
			assert_str(String(translation.get_message(key))).override_failure_message("%s %s" % [locale, key]) \
					.is_not_empty()


func test_params_are_bounded_and_never_logged() -> void:
	var raw: Dictionary = _item("friend_request", "friend.request_received")
	var params: Dictionary = {"nickname": NICK, "score": 9.5, "long": "x".repeat(400)}
	for i: int in 30:
		params["k%d" % i] = "v"
	raw["params"] = params
	var m: NotificationModel = NotificationModel.from_dict(raw)
	assert_bool(m.params.has("score")).is_false()
	assert_int(m.params.size()).is_less_equal(NotificationModel.MAX_PARAMS)
	if m.params.has("long"):
		assert_int(String(m.params["long"]).length()).is_equal(NotificationModel.MAX_PARAM_LEN)
	Log.clear()
	await h.connect_online()
	_deliver(_created("friend_request"))
	h.http.reply(ROUTE_LIST, 200, _list_fixture())
	await svc.load_unread()
	assert_str(JSON.stringify(Log.entries())).not_contains(NICK)


func test_reset_drops_everything_of_the_student() -> void:
	h.http.reply(ROUTE_LIST, 200, _list_fixture())
	await svc.load_unread()
	svc.reset()
	assert_int(svc.badge).is_equal(0)
	assert_array(svc.items).is_empty()
	assert_bool(svc.inbox_loaded).is_false()


# ---------- F064-AC-21 kept: the grouped pending-work toast of the first refresh ----------

func test_first_assignments_refresh_gives_one_grouped_local_toast() -> void:
	var assignments: AssignmentService = AssignmentService.new(h.api)
	svc.bind(assignments)
	var items: Array = []
	for i: int in 3:
		var a: Dictionary = AssignmentFixtures.item_sample(i)
		a["status"] = "not_started"
		items.append(a)
	h.http.reply(ROUTE_ASSIGNMENTS, 200, AssignmentFixtures.page(items))
	await assignments.refresh()
	assert_int(toasts.size()).is_equal(1)
	assert_bool(toasts[0].is_local()).is_true()
	assert_str(toasts[0].display_text()).is_equal(tr("ui.notification.assignments_pending").format({"count": 3}))
	# The badge stays the server unread count.
	assert_int(svc.badge).is_equal(0)
	await assignments.refresh()
	assert_int(toasts.size()).is_equal(1)
	# Its "open" goes to the tab without any server call.
	var intents: Array = []
	var record: Callable = func(screen: StringName, _args: Dictionary) -> void: intents.append(screen)
	EventBus.ui_requested.connect(record)
	assert_bool(svc.open(toasts[0])).is_true()
	EventBus.ui_requested.disconnect(record)
	assert_array(intents).is_equal([&"quest_journal"])
	assert_int(h.http.count(ROUTE_READ)).is_equal(0)


func test_session_end_through_the_service_locator() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock, h.client)
	h.http.reply(ROUTE_LIST, 200, _page([], 3))
	await services.notifications.load_unread()
	assert_int(services.notifications.badge).is_equal(3)
	services.end_session()
	assert_int(services.notifications.badge).is_equal(0)
	h.client.unregister_handler(NotificationService.PREFIX, services.notifications.handle_message)
	services.hub.dispose()
