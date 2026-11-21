extends GdUnitTestSuite
## F064-AC-22, F064-AC-23 (T025): the HUD 🔔 shows the badge of NotificationService and the toasts (at most 3 at once);
## touching a toast or the bell asks for the "Nhà trường giao" tab and clears the badge; nothing refreshes by itself —
## no timer, no refresh while the app is in the background, no push outside the game. FakeHttp (TEST-ONLY).

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const ROUTE: String = "GET /learning/my-assignments"

var h: AuthHarness = null
var services: ServiceLocator = null
var pressed: int = 0


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	pressed = 0


func after_test() -> void:
	AppState.services = null
	var none: Array[MyAssignmentModel] = []
	AppState.set_assignments(none)
	h.dispose()


func _hud() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	hud.connect("notifications_pressed", func() -> void: pressed += 1)
	return [runner, hud]


func _items(count: int, offset: int = 0) -> Array:
	var out: Array = []
	for i: int in count:
		var item: Dictionary = AssignmentFixtures.item_sample(offset + i)
		item["status"] = "not_started"
		out.append(item)
	return out


func test_badge_and_toast_follow_the_service() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var bell: Button = hud.find_child("NotificationBell", true, false) as Button
	assert_bool(bell.visible).is_true()
	assert_str(bell.text).is_equal(tr("ui.notification.bell"))
	h.http.reply(ROUTE, 200, AssignmentFixtures.page(_items(2)))
	await services.assignments.refresh()
	await (r[0] as GdUnitSceneRunner).simulate_frames(1)
	assert_str(bell.text).is_equal(tr("ui.notification.bell_count").format({"count": 2}))
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	var texts: PackedStringArray = stack.call("texts") as PackedStringArray
	assert_int(texts.size()).is_equal(1)
	assert_str(texts[0]).is_equal(tr("ui.notification.assignments_pending").format({"count": 2}))


func test_at_most_three_toasts_at_once() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	for i: int in 5:
		services.notifications.toast_requested.emit("ui.notification.assignment_new", {"title": "Bài %d" % i})
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	var texts: PackedStringArray = stack.call("texts") as PackedStringArray
	assert_int(texts.size()).is_equal(3)
	assert_str(texts[2]).is_equal(tr("ui.notification.assignment_new").format({"title": "Bài 4"}))


func test_touching_a_toast_or_the_bell_asks_for_the_tab() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	h.http.reply(ROUTE, 200, AssignmentFixtures.page(_items(2)))
	await services.assignments.refresh()
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	stack.call("press", 0)
	assert_int(pressed).is_equal(1)
	assert_int((stack.call("texts") as PackedStringArray).size()).is_equal(0)
	(hud.find_child("NotificationBell", true, false) as Button).pressed.emit()
	assert_int(pressed).is_equal(2)


func test_opening_the_tab_clears_the_badge() -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.page(_items(2)))
	await services.assignments.refresh()
	var intents: Array[Array] = []
	var record: Callable = func(screen: StringName, args: Dictionary) -> void: intents.append([screen, args])
	EventBus.ui_requested.connect(record)
	services.notifications.open_assigned()
	EventBus.ui_requested.disconnect(record)
	assert_int(services.notifications.badge).is_equal(0)
	assert_array(intents).is_equal([[&"quest_journal", {"tab": "assigned"}]])


func test_nothing_refreshes_by_itself_even_in_the_background() -> void:
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	var before: int = h.http.count(ROUTE)
	hud.propagate_notification(Node.NOTIFICATION_APPLICATION_PAUSED)
	hud.propagate_notification(Node.NOTIFICATION_APPLICATION_FOCUS_OUT)
	for i: int in 10:
		h.clock.advance(60.0)
		await runner.simulate_frames(5)
	assert_int(h.http.count(ROUTE)).is_equal(before)
	# No timer drives notifications, and nothing is sent outside the game (no push / OS notification API).
	assert_int(hud.find_children("*", "Timer", true, false).size()).is_equal(0)
	var source: String = FileAccess.get_file_as_string("res://scripts/notifications/notification_service.gd")
	for api: String in ["OS.", "Timer", "create_timer", "JavaScriptBridge", "request_permission"]:
		assert_str(source).override_failure_message(api).not_contains(api)
