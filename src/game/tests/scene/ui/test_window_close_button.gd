extends GdUnitTestSuite
## F125-AC-06, F125-AC-07 (owner 2026-11-27, F114-3; DESIGN_SYSTEM §5.1, STUDENT_GAME_UX_SOCIAL §10): every window built
## on the shared window frame — quest journal, wardrobe studio, world map, exam lobby, wardrobe/shop, achievements,
## solo challenge (board, result), journey, the "Thêm" menu, notification inbox, Settings, party, arena, F110 friends /
## boss card / interdependent step — has exactly one
## close button: the red `DangerButton` ✕ "Đóng" at the right end of its blue title bar at the top of the window, with
## the band touch target (the compact floor on a phone), and Esc/Back closes the window the same way as that button.
## Confirmation dialogs (neutral "Hủy"), the activity screen (guarded "Thoát") and full-frame screens (login) have no
## red close. FakeHttp + contract fixtures (TEST-ONLY).

const SETTLE_FRAMES: int = 8

var s: F069SceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	AppState.set_compact(false)
	s.h.http.delay_frames = 1
	s.dispose()


func _viewport(size: Vector2i) -> SubViewport:
	var vp: SubViewport = SubViewport.new()
	vp.size = size
	vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
	add_child(vp)
	return vp


func _mount(vp: SubViewport, path: String) -> Control:
	var node: Control = (load(path) as PackedScene).instantiate() as Control
	node.theme = AppState.theme_service.current_theme
	vp.add_child(node)
	return node


func _settle() -> void:
	for i: int in SETTLE_FRAMES:
		await await_idle_frame()


## Window name → builder (returns the screen root, shown and filled).
func _windows() -> Dictionary:
	return {
		"quest_journal": func(vp: SubViewport) -> Control:
			var j: Control = _mount(vp, "res://scenes/quest/QuestJournal.tscn")
			j.call("open", {"tab": "assigned"})
			return j,
		"world_map": func(vp: SubViewport) -> Control:
			var m: Control = _mount(vp, "res://scenes/world/WorldMap.tscn")
			m.show()
			m.call("render", WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), "g4_5",
					"main_hub"))
			return m,
		"wardrobe_studio": func(vp: SubViewport) -> Control:
			var w: Control = _mount(vp, "res://scenes/character/CustomizationStudio.tscn")
			w.call("open", {})
			return w,
		"exam_lobby": func(vp: SubViewport) -> Control:
			var e: Control = _mount(vp, "res://scenes/exam/ExamLobby.tscn")
			e.show()
			return e,
		"wardrobe": func(vp: SubViewport) -> Control:
			var i: Control = _mount(vp, "res://scenes/inventory/InventoryShopScreen.tscn")
			i.call("open", {"tab": "wardrobe"})
			return i,
		"shop": func(vp: SubViewport) -> Control:
			var i: Control = _mount(vp, "res://scenes/inventory/InventoryShopScreen.tscn")
			i.call("open", {"tab": "shop"})
			return i,
		"achievements": func(vp: SubViewport) -> Control:
			var a: Control = _mount(vp, "res://scenes/achievement/AchievementCollectionScreen.tscn")
			a.call("open", {})
			return a,
		"challenge_board": func(vp: SubViewport) -> Control:
			var c: Control = _mount(vp, "res://scenes/challenge/ChallengeBoard.tscn")
			c.call("open", {})
			return c,
		"challenge_result": func(vp: SubViewport) -> Control:
			var r: Control = _mount(vp, "res://scenes/challenge/ChallengeResult.tscn")
			r.call("open", {})
			return r,
		"journey": func(vp: SubViewport) -> Control:
			var j: Control = _mount(vp, "res://scenes/progression/JourneyScreen.tscn")
			j.call("open", {})
			return j,
		"game_menu": func(vp: SubViewport) -> Control:
			var g: Control = _mount(vp, "res://scenes/ui/hud/GameMenu.tscn")
			g.call("open", {})
			return g,
		"notifications": func(vp: SubViewport) -> Control:
			NotificationFixtures.script(s.h.http)
			var n: Control = _mount(vp, "res://scenes/ui/notifications/NotificationInbox.tscn")
			n.call("open", {})
			return n,
		"settings": func(vp: SubViewport) -> Control:
			var p: Control = _mount(vp, "res://scenes/ui/settings/SettingsPanel.tscn")
			p.call("open", {})
			return p,
		"party": func(vp: SubViewport) -> Control:
			var party: PartyService = s.services.party
			party.clear()
			party._refresh_self()
			PartyFixtures.script(s.h.http, party.store.self_id, false)
			ChatScreens.prepare(s.services, s.h.http)
			var lobby: PartyLobby = _mount(vp, "res://scenes/social/PartyLobby.tscn") as PartyLobby
			lobby.service = party
			lobby.open({})
			return lobby,
		"arena": func(vp: SubViewport) -> Control:
			var root: Control = Control.new()
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.size = Vector2(vp.size)
			return await CompetitionScreens.build("arena", root, s.services, s.h.http),
		# F110: Bạn bè, the boss card, the interdependent step.
		"friends": func(vp: SubViewport) -> Control:
			var root: Control = Control.new()
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.size = Vector2(vp.size)
			return await F110Screens.build("friends", root, s.services, s.h.http),
		"boss_info": func(vp: SubViewport) -> Control:
			var root: Control = Control.new()
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.size = Vector2(vp.size)
			return await F110Screens.build("boss_info", root, s.services, s.h.http),
		"coop_step": func(vp: SubViewport) -> Control:
			var root: Control = Control.new()
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.size = Vector2(vp.size)
			return await F110Screens.build("coop_step", root, s.services, s.h.http),
	}


## The red closes of `root` (DangerButton labelled "Đóng"), visible ones only.
static func _red_closes(root: Node) -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in root.find_children("*", "Button", true, false):
		var b: Button = node as Button
		if b.is_visible_in_tree() and b.theme_type_variation == &"DangerButton" and b.text == "ui.common.close":
			out.append(b)
	return out


static func _title_bar_of(close: Control) -> Control:
	var p: Node = close.get_parent()
	while p != null and not (p is PanelContainer and (p as Control).theme_type_variation == &"WindowTitleBar"):
		p = p.get_parent()
	return p as Control


static func _window_of(close: Control) -> Control:
	var p: Node = close.get_parent()
	while p != null and not (p is PanelContainer and p.name == "Window"):
		p = p.get_parent()
	return p as Control


func _check_window(name: String, screen: Control, compact: bool) -> PackedStringArray:
	var out: PackedStringArray = []
	var closes: Array[Button] = _red_closes(screen)
	if closes.size() != 1:
		return PackedStringArray(["%s: %d red close buttons (want 1)" % [name, closes.size()]])
	var close: Button = closes[0]
	if close.name != "Close":
		out.append("%s: close named %s" % [name, close.name])
	if close.icon == null or not close.icon.resource_path.ends_with("cross.svg"):
		out.append("%s: close icon is not ✕" % name)
	var bar: Control = _title_bar_of(close)
	if bar == null:
		out.append("%s: close is not in a WindowTitleBar" % name)
		return out
	for other: Node in bar.find_children("*", "Control", true, false):
		var c: Control = other as Control
		if c != close and c.is_visible_in_tree() and c.get_global_rect().position.x >= close.get_global_rect().end.x:
			out.append("%s: %s is right of the close button" % [name, c.name])
	var bar_rect: Rect2 = bar.get_global_rect()
	var right_pad: float = bar.get_theme_stylebox(&"panel").get_margin(SIDE_RIGHT)
	if absf(close.get_global_rect().end.x - (bar_rect.end.x - right_pad)) > 1.0:
		out.append("%s: close not at the right end of the title bar (%.0f vs %.0f)" % [name,
				close.get_global_rect().end.x, bar_rect.end.x - right_pad])
	var win: Control = _window_of(close)
	if win == null:
		out.append("%s: close outside a Window frame" % name)
	else:
		var top_pad: float = win.get_theme_stylebox(&"panel").get_margin(SIDE_TOP)
		if bar_rect.position.y - win.get_global_rect().position.y > top_pad + 1.0:
			out.append("%s: title bar not at the top of the window" % name)
	var target: float = CompactPolicy.button_target(UiBuild.band(), compact)
	if close.size.y + 0.5 < target:
		out.append("%s: close %.0f px < touch target %.0f (compact %s)" % [name, close.size.y, target, compact])
	return out


## Esc (ui_cancel) closes the window like its ✕ Đóng (the `closed` signal, or the window hidden).
func _esc_closes(name: String, vp: SubViewport, screen: Control) -> String:
	var fired: Array[int] = [0]
	if screen.has_signal("closed"):
		screen.connect("closed", func() -> void: fired[0] += 1)
	var esc: InputEventAction = InputEventAction.new()
	esc.action = &"ui_cancel"
	esc.pressed = true
	vp.push_input(esc)
	for i: int in 3:
		await await_idle_frame()
	if fired[0] == 0 and screen.is_visible_in_tree():
		return "%s: Esc did not close the window" % name
	return ""


func test_every_window_has_one_red_close_at_the_right_of_its_title_bar() -> void:
	var failures: PackedStringArray = []
	var windows: Dictionary = _windows()
	for compact: bool in [false, true]:
		AppState.set_compact(compact)
		var size: Vector2i = BandLayoutProbe.SIZES["844x390" if compact else "1280x720"] as Vector2i
		for name: String in windows.keys():
			var vp: SubViewport = _viewport(size)
			var screen: Control = await (windows[name] as Callable).call(vp)
			await _settle()
			failures.append_array(_check_window(name, screen, compact))
			vp.queue_free()
			await await_idle_frame()
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


func test_esc_and_the_close_button_close_every_window_the_same_way() -> void:
	var failures: PackedStringArray = []
	var windows: Dictionary = _windows()
	for name: String in windows.keys():
		# Esc / Back.
		var vp: SubViewport = _viewport(Vector2i(1280, 720))
		var screen: Control = await (windows[name] as Callable).call(vp)
		await _settle()
		var esc: String = await _esc_closes(name, vp, screen)
		if not esc.is_empty():
			failures.append(esc)
		vp.queue_free()
		await await_idle_frame()
		# The ✕ Đóng button.
		vp = _viewport(Vector2i(1280, 720))
		screen = await (windows[name] as Callable).call(vp)
		await _settle()
		var fired: Array[int] = [0]
		if screen.has_signal("closed"):
			screen.connect("closed", func() -> void: fired[0] += 1)
		var closes: Array[Button] = _red_closes(screen)
		if closes.size() == 1:
			closes[0].pressed.emit()
			for i: int in 3:
				await await_idle_frame()
			if fired[0] == 0 and screen.is_visible_in_tree():
				failures.append("%s: ✕ Đóng did not close the window" % name)
		vp.queue_free()
		await await_idle_frame()
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


## F125-AC-07: confirmations keep the neutral "Hủy", the activity keeps its guarded "Thoát", login has no close.
func test_dialogs_activity_and_full_frame_screens_have_no_red_close() -> void:
	var vp: SubViewport = _viewport(Vector2i(1280, 720))
	# Logout confirmation over Settings.
	var settings: Control = _mount(vp, "res://scenes/ui/settings/SettingsPanel.tscn")
	settings.call("open", {})
	await _settle()
	(settings.find_child("Logout", true, false) as Button).pressed.emit()
	await _settle()
	var confirm: Control = settings.find_child("LogoutConfirm", true, false) as Control
	assert_object(confirm).is_not_null()
	assert_bool(confirm.is_visible_in_tree()).is_true()
	assert_array(_red_closes(confirm)).is_empty()
	assert_object(confirm.find_child("Cancel", true, false)).is_not_null()
	assert_str((confirm.find_child("Cancel", true, false) as Button).theme_type_variation).is_equal("SecondaryButton")
	# Shop purchase confirmation.
	var shop: Control = _mount(vp, "res://scenes/inventory/InventoryShopScreen.tscn")
	shop.call("open", {"tab": "shop"})
	await _settle()
	shop.call("select_entry", 0)
	await _settle()
	shop.call("ask_purchase")
	await _settle()
	var dialog: Control = shop.find_child("PurchaseDialog", true, false) as Control
	if dialog != null:
		assert_array(_red_closes(dialog)).is_empty()
	# Login (full frame) and the activity player (guarded Thoát).
	var login: Control = _mount(vp, "res://scenes/auth/Login.tscn")
	await _settle()
	assert_array(_red_closes(login)).is_empty()
	var host: Control = _mount(vp, "res://scenes/learning/ActivityHost.tscn")
	host.set("services", s.services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", "g4_5")
	s.h.http.clear(LearningFixtures.ROUTE_START)
	s.h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt_with_types(PackedStringArray(
			["single_choice"])))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	await _settle()
	assert_array(_red_closes(host)).is_empty()
	vp.queue_free()
