extends GdUnitTestSuite
## F123-AC-15, F123-AC-16, F123-AC-17, F123-AC-18, F123-AC-19 (T025; TC-I18N-005, TC-UI-003): the HUD bell shows the
## unread count in a `CountBadge` pill (words, not colour only); a realtime invitation or school work shows one short
## toast (bell icon + text + arrow, at least the touch target high, at most 3) that never blocks play, none with popups
## off or for level-ups; touching a toast / the bell are intents; the inbox lists the notifications newest first with the
## kind icon, the text in the current language, "Mới" on the unread ones (then marked read) and "Xem" only where this
## client has the screen ("Nhà trường giao" for school work); empty and error states; the Settings tab Thông báo switches
## the popups; every part uses the generated Theme variants and translation keys, and plays no sound. FakeHttp + contract
## fixtures (TEST-ONLY).

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const INBOX: String = "res://scenes/ui/notifications/NotificationInbox.tscn"
const SETTINGS_SCENE: String = "res://scenes/ui/settings/SettingsPanel.tscn"
const SETTINGS_PATH: String = "user://test_f123_settings.cfg"
const ROUTE_LIST: String = "GET /notifications"
const ROUTE_READ: String = "POST /notifications/*:read"
const NICK: String = "SaoBang12"

var h: AuthHarness = null
var services: ServiceLocator = null
var _popups: bool = true
var _path: String = ""
var _locale: String = ""


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	_popups = Settings.notification_popups
	_path = Settings.path
	_locale = String(Settings.locale)
	Settings.path = SETTINGS_PATH
	Settings.notification_popups = true
	Settings.set_locale("vi-VN", false)


func after_test() -> void:
	if services.realtime != null:
		services.realtime.unregister_handler(NotificationService.PREFIX, services.notifications.handle_message)
	AppState.services = null
	Settings.notification_popups = _popups
	Settings.set_locale(_locale, false)
	Settings.path = _path
	DirAccess.remove_absolute(SETTINGS_PATH)
	h.dispose()


static func _created(name: String) -> Dictionary:
	var payload: Dictionary = (RealtimeHarness.fixture("notification.item.created." + name)["payload"] as Dictionary)
	payload["notification_id"] = Uuid.v7()
	return payload


static func _row(kind: String, code: String, params: Dictionary, read: bool, action_kind: String) -> Dictionary:
	return {"notification_id": Uuid.v7(), "kind": kind, "message_code": code, "params": params, "read": read,
		"action": {"kind": action_kind, "id": Uuid.v7()}, "created_at": "2026-11-21T08:00:00Z"}


static func _page(items: Array, unread: int) -> Dictionary:
	return {"data": items, "page": {"next_cursor": null, "has_more": false, "limit": 20}, "unread_count": unread}


func _hud() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	return [runner, runner.scene() as CanvasLayer]


func _inbox() -> Control:
	var inbox: Control = auto_free((load(INBOX) as PackedScene).instantiate()) as Control
	add_child(inbox)
	return inbox


func _frames(n: int = 4) -> void:
	for i: int in n:
		await get_tree().process_frame


# ---------- HUD bell + toasts ----------

func test_bell_shows_the_unread_count_in_a_pill() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var bell: Button = hud.find_child("NotificationBell", true, false) as Button
	var pill: Label = bell.find_child("Count", true, false) as Label
	assert_str(bell.text).is_empty()
	assert_object(bell.icon).is_not_null()
	assert_bool(pill.visible).is_false()
	var square: float = bell.custom_minimum_size.x
	h.http.reply(ROUTE_LIST, 200, _page([_row("assignment_new", "assignment.new", {"title": "Bài 1"}, false,
			"open_assignment")], 3))
	await services.notifications.load_unread()
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	assert_bool(pill.visible).is_true()
	assert_str(pill.text).is_equal("3")
	assert_str(String(pill.theme_type_variation)).is_equal("CountBadge")
	assert_str(bell.tooltip_text).is_equal(tr("ui.notification.bell_unread").format({"count": 3}))
	# The pill sits beside the icon inside the (wider) button, never over it or outside it.
	assert_float(bell.custom_minimum_size.x).is_greater(square)
	assert_bool(Rect2(Vector2.ZERO, bell.size).encloses(Rect2(pill.position, pill.size))).is_true()
	var icon_right: float = float(bell.get_theme_stylebox(&"normal").get_margin(SIDE_LEFT)) \
			+ float(bell.get_theme_constant(&"icon_max_width"))
	assert_float(pill.position.x).is_greater_equal(icon_right)
	services.notifications.handle_message(NotificationService.MSG_CREATED, _created("friend_request"))
	await (r[0] as GdUnitSceneRunner).simulate_frames(1)
	assert_str(pill.text).is_equal("4")
	services.notifications.reset()
	await (r[0] as GdUnitSceneRunner).simulate_frames(1)
	assert_bool(pill.visible).is_false()
	assert_float(bell.custom_minimum_size.x).is_equal(square)


func test_invitation_shows_one_short_toast_that_does_not_block_play() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	services.notifications.handle_message(NotificationService.MSG_CREATED, _created("friend_request"))
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	var texts: PackedStringArray = stack.call("texts") as PackedStringArray
	assert_int(texts.size()).is_equal(1)
	assert_str(texts[0]).is_equal(tr("ui.notification.friend.request_received").format({"nickname": NICK}))
	var toast: PanelContainer = stack.get_child(0) as PanelContainer
	assert_str(String(toast.theme_type_variation)).is_equal("HudPanel")
	assert_object((toast.find_child("Icon", true, false) as TextureRect).texture).is_not_null()
	assert_object((toast.find_child("Open", true, false) as TextureRect).texture).is_not_null()
	assert_float(toast.size.y + 0.5).is_greater_equal(UiBuild.target())
	# Not modal: the stack itself lets the touches through to the world.
	assert_int(stack.mouse_filter).is_equal(Control.MOUSE_FILTER_IGNORE)
	assert_int(hud.find_children("*", "AudioStreamPlayer", true, false).size()).is_equal(0)


func test_level_up_and_popups_off_only_change_the_badge() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	var level: Dictionary = _created("friend_request")
	level["kind"] = "level_reached"
	level["message_code"] = "progression.level_reached"
	level["params"] = {"level": "4"}
	services.notifications.handle_message(NotificationService.MSG_CREATED, level)
	Settings.notification_popups = false
	services.notifications.handle_message(NotificationService.MSG_CREATED, _created("assignment_new"))
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	assert_int((stack.call("texts") as PackedStringArray).size()).is_equal(0)
	var pill: Label = hud.find_child("Count", true, false) as Label
	assert_str(pill.text).is_equal("2")


func test_at_most_three_toasts_at_once() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	for i: int in 5:
		services.notifications.handle_message(NotificationService.MSG_CREATED, _created("assignment_new"))
	await (r[0] as GdUnitSceneRunner).simulate_frames(2)
	assert_int((stack.call("texts") as PackedStringArray).size()).is_equal(3)


func test_touching_a_toast_or_the_bell_are_intents() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var touched: Array[NotificationModel] = []
	var bells: Array[int] = [0]
	hud.connect("notification_pressed", func(item: NotificationModel) -> void: touched.append(item))
	hud.connect("notifications_pressed", func() -> void: bells[0] += 1)
	services.notifications.handle_message(NotificationService.MSG_CREATED, _created("assignment_new"))
	await (r[0] as GdUnitSceneRunner).simulate_frames(1)
	var stack: Control = hud.find_child("ToastStack", true, false) as Control
	stack.call("press", 0)
	assert_int(touched.size()).is_equal(1)
	assert_str(touched[0].kind).is_equal("assignment_new")
	assert_int((stack.call("texts") as PackedStringArray).size()).is_equal(0)
	(hud.find_child("NotificationBell", true, false) as Button).pressed.emit()
	assert_int(bells[0]).is_equal(1)


func test_nothing_refreshes_by_itself_even_in_the_background() -> void:
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var hud: CanvasLayer = r[1] as CanvasLayer
	var before: int = h.http.count(ROUTE_LIST)
	hud.propagate_notification(Node.NOTIFICATION_APPLICATION_PAUSED)
	for i: int in 10:
		h.clock.advance(60.0)
		await runner.simulate_frames(5)
	assert_int(h.http.count(ROUTE_LIST)).is_equal(before)
	assert_int(hud.find_children("*", "Timer", true, false).size()).is_equal(0)


# ---------- inbox ----------

func test_inbox_lists_marks_read_and_opens_school_work() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([
		_row("friend_request", "friend.request_received", {"nickname": NICK}, false, "open_friend_requests"),
		_row("assignment_new", "assignment.new", {"title": "Phép nhân bảng 3"}, false, "open_assignment"),
		_row("level_reached", "progression.level_reached", {"level": "3"}, true, "open_profile"),
	], 2))
	h.http.reply(ROUTE_READ, 200, {})
	var inbox: Control = _inbox()
	inbox.call("open")
	await _frames(8)
	assert_int(int(inbox.call("row_count"))).is_equal(3)
	var texts: PackedStringArray = inbox.call("row_texts") as PackedStringArray
	assert_str(texts[0]).is_equal(tr("ui.notification.friend.request_received").format({"nickname": NICK}))
	assert_str(texts[1]).is_equal(tr("ui.notification.assignment.new").format({"title": "Phép nhân bảng 3"}))
	assert_str(texts[2]).is_equal(tr("ui.notification.progression.level_reached").format({"level": "3"}))
	# "Mới" (words) on the two unread rows; they are marked read (REST: no socket in this harness).
	assert_bool(bool(inbox.call("is_marked_new", 0))).is_true()
	assert_bool(bool(inbox.call("is_marked_new", 1))).is_true()
	assert_bool(bool(inbox.call("is_marked_new", 2))).is_false()
	assert_int(h.http.count(ROUTE_READ)).is_equal(2)
	assert_int(services.notifications.badge).is_equal(0)
	# "Xem" only where this client has the screen: school work and the level (Hành trình), not friend requests (F095).
	assert_object(inbox.call("open_button", 0)).is_null()
	var open_work: Button = inbox.call("open_button", 1) as Button
	assert_object(open_work).is_not_null()
	assert_object(open_work.icon).is_not_null()
	assert_float(open_work.custom_minimum_size.y + 0.5).is_greater_equal(UiBuild.target())
	var intents: Array[Array] = []
	var record: Callable = func(screen: StringName, args: Dictionary) -> void: intents.append([screen, args])
	EventBus.ui_requested.connect(record)
	open_work.pressed.emit()
	EventBus.ui_requested.disconnect(record)
	assert_array(intents).is_equal([[&"quest_journal", {"tab": "assigned"}]])


func test_inbox_window_follows_the_tier_one_style() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([], 0))
	var inbox: Control = _inbox()
	inbox.call("open")
	await _frames(4)
	var title: Label = inbox.find_child("Title", true, false) as Label
	assert_str(String(title.theme_type_variation)).is_equal("WindowTitleLabel")
	assert_bool(title.uppercase).is_true()
	assert_str(String((inbox.find_child("TitleBar", true, false) as Control).theme_type_variation)).is_equal(
			"WindowTitleBar")
	var close: Button = inbox.find_child("Close", true, false) as Button
	assert_str(String(close.theme_type_variation)).is_equal("DangerButton")
	assert_str(close.icon.resource_path).ends_with("cross.svg")
	assert_object(inbox.find_child("Scrim", true, false)).is_not_null()
	var closed: Array[int] = [0]
	inbox.connect("closed", func() -> void: closed[0] += 1)
	close.pressed.emit()
	assert_bool(inbox.visible).is_false()
	assert_int(closed[0]).is_equal(1)


func test_inbox_empty_and_error_states() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([], 0))
	var inbox: Control = _inbox()
	inbox.call("open")
	await _frames(6)
	var empty: Control = inbox.find_child("Empty", true, false) as Control
	assert_bool(empty.visible).is_true()
	assert_object(empty.find_child("Mascot", true, false)).is_not_null()
	inbox.call("close")
	# A refusal that is not retried (a 503 would be retried with backoff by ApiClient).
	h.http.reply(ROUTE_LIST, 404, AuthHarness.problem(404, "NOT_FOUND"))
	inbox.call("open")
	await _frames(6)
	var retry: Button = inbox.find_child("Retry", true, false) as Button
	assert_bool(retry.visible).is_true()
	assert_str((inbox.find_child("StateLabel", true, false) as Label).text).is_equal(
			"ui.notification.inbox.load_failed")
	assert_bool(empty.visible).is_false()


func test_inbox_follows_the_language() -> void:
	h.http.reply(ROUTE_LIST, 200, _page([
		_row("assignment_new", "assignment.new", {"title": "Phép nhân bảng 3"}, true, "open_assignment")], 0))
	var inbox: Control = _inbox()
	inbox.call("open")
	await _frames(6)
	Settings.set_locale("en-US", false)
	EventBus.locale_changed.emit("en-US")
	await _frames(2)
	var texts: PackedStringArray = inbox.call("row_texts") as PackedStringArray
	assert_str(texts[0]).is_equal("Your school gave you a new task: Phép nhân bảng 3")


# ---------- Settings: popup switch ----------

func test_settings_tab_switches_the_popups() -> void:
	var panel: Control = auto_free((load(SETTINGS_SCENE) as PackedScene).instantiate()) as Control
	add_child(panel)
	panel.call("open")
	panel.call("show_tab", "notifications")
	var tab: Button = panel.call("tab_button", "notifications") as Button
	assert_object(tab.icon).is_not_null()
	assert_str(tab.text).is_equal("ui.settings.tab.notifications")
	assert_bool((panel.call("page", "notifications") as Control).visible).is_true()
	var switch: ToggleSwitch = panel.find_child("NotificationPopups", true, false) as ToggleSwitch
	assert_bool(switch.button_pressed).is_true()
	assert_str(switch.text).is_equal("ui.settings.on")
	switch.button_pressed = false
	assert_bool(Settings.notification_popups).is_false()
	assert_str(switch.text).is_equal("ui.settings.off")
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(SETTINGS_PATH)).is_equal(OK)
	assert_bool(bool(cfg.get_value("ui", "notification_popups", true))).is_false()
	assert_bool(services.notifications.popups_enabled()).is_false()


# ---------- theme, keys ----------

func test_parts_use_generated_theme_variants_and_keys() -> void:
	var theme: Theme = load("res://resources/ui/theme/eduworld_theme_g4_5.tres") as Theme
	for variant: StringName in [&"CountBadge", &"HudPanel", &"QuestCard", &"WindowTitleBar", &"DangerButton"]:
		assert_bool(theme.get_type_variation_base(variant) != &"").override_failure_message(String(variant)).is_true()
	assert_bool(theme.has_stylebox(&"normal", &"CountBadge")).is_true()
	for path: String in ["res://scenes/ui/notifications/NotificationToast.tscn", "res://scenes/ui/hud/NotificationBell.tscn",
			"res://scenes/ui/notifications/notification_inbox.gd", "res://scenes/ui/hud/notification_bell.gd",
			"res://scenes/ui/hud/toast_stack.gd"]:
		var source: String = FileAccess.get_file_as_string(path)
		for banned: String in ["add_theme_color_override", "add_theme_font_size_override", "add_theme_stylebox_override",
				"Color(", "AudioStreamPlayer"]:
			assert_str(source).override_failure_message("%s %s" % [path, banned]).not_contains(banned)
