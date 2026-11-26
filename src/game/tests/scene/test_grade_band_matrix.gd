extends GdUnitTestSuite
## F098-AC-09 (owner "ma trận test khối 1–9"; STUDENT_GAME_UX §4.3, DESIGN_SYSTEM §5.2, LEARNING_EXPERIENCE §8,
## NFR-A11Y-001): every student screen of the matrix is built for EVERY grade band (g1_3, g4_5, g6_7, g8_9) at the three
## reference landscape windows (1280×720, 1024×600, 844×390 → logical 1280×720, 1280×750, 1558×720 — F113:
## landscape only, STUDENT_GAME_UX §2; windows taller than wide show the rotate prompt, checked alone) inside a
## SubViewport of that size with the band's Theme, and BandLayoutProbe checks touch targets, the smallest font, answer tiles and that
## nothing is cut off or outside the view. Screens: S01 login, Hub HUD, activity (single choice, true/false, matching,
## numeric), feedback (grading, right, not yet right), result, S02 character creation, wardrobe and shop (F069), level
## up, world map; F123: the HUD with the bell count and toasts, the notification inbox (rows, empty) and the Settings
## tab Thông báo. FakeHttp + contract fixtures (TEST-ONLY).

const HOST: String = "res://scenes/learning/ActivityHost.tscn"
const SETTLE_FRAMES: int = 6

var s: F069SceneHarness = null
## What each build showed to the probe (printed, for the evidence table).
var _seen: PackedStringArray = []
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	for line: String in _seen:
		print("band-matrix ", line)
	_seen.clear()
	s.h.http.delay_frames = 1
	s.dispose()


## Builds `build` (a Callable returning the screen root, coroutine allowed) for every band × window, probes it and
## returns every problem found.
## F099-AC-07: `empty` also checks that the activity board has no empty band taller than 25% of the view.
## F113: `sizes` defaults to the landscape windows (BandLayoutProbe.SIZES); the rotate prompt passes PORTRAIT_SIZES.
func _matrix(label: String, build: Callable, empty: bool = false, sizes: Dictionary = BandLayoutProbe.SIZES
		) -> PackedStringArray:
	var failures: PackedStringArray = []
	for band: String in UiTokens.DENSITY_BANDS:
		for window: String in sizes.keys():
			AppState.set_grade_band(band)
			var vp: SubViewport = SubViewport.new()
			vp.size = sizes[window] as Vector2i
			vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
			add_child(vp)
			var root: Node = await build.call(vp, band)
			if root == null:
				failures.append("%s %s %s: not built" % [label, band, window])
			else:
				for i: int in SETTLE_FRAMES:
					await await_idle_frame()
				failures.append_array(BandLayoutProbe.check(root, vp, band, "%s %s" % [label, window]))
				var band_note: String = ""
				if empty:
					failures.append_array(BandLayoutProbe.empty_bands(root, vp, "%s %s %s" % [label, band, window]))
					band_note = ", largest empty band %.0f%%" % (BandLayoutProbe.last_largest_band * 100.0)
				for track: Node in root.find_children("*", "MediaTrack", true, false):
					var t: MediaTrack = track as MediaTrack
					if t.is_visible_in_tree() and t.size.y + 0.5 < float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band]):
						failures.append("%s %s %s: media bar %.0f px < touch target" % [label, band, window, t.size.y])
				if BandLayoutProbe.last_texts == 0:
					failures.append("%s %s %s: no visible text checked" % [label, band, window])
				_seen.append("%s %s %s: %d buttons, %d texts, %d tiles%s" % [label, band, window,
						BandLayoutProbe.last_buttons, BandLayoutProbe.last_texts, BandLayoutProbe.last_tiles, band_note])
			vp.queue_free()
			await await_idle_frame()
	return failures


## Adds `node` to the viewport with the band Theme (a SubViewport does not inherit the window Theme).
func _mount(vp: SubViewport, node: Node) -> Node:
	var theme: Theme = AppState.theme_service.current_theme
	if node is Control:
		(node as Control).theme = theme
	else:
		for child: Node in node.get_children():
			if child is Control:
				(child as Control).theme = theme
	vp.add_child(node)
	return node


func _scene(path: String) -> Node:
	return (load(path) as PackedScene).instantiate()


func _assert_none(failures: PackedStringArray) -> void:
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


# ---------- builders ----------

func _activity(vp: SubViewport, band: String, types: PackedStringArray, result: Dictionary = {},
		tap: int = -1, grading: bool = false) -> Node:
	var host: Control = _mount(vp, _scene(HOST)) as Control
	host.set("services", s.services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	s.h.http.delay_frames = 1
	s.h.http.clear(LearningFixtures.ROUTE_START)
	s.h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt_with_types(types))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 3:
		await await_idle_frame()
	if tap >= 0:
		s.h.http.clear(LearningFixtures.ROUTE_RESPOND)
		s.h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, result)
		if grading:
			s.h.http.delay_frames = 100000
		(host.get("renderer") as ActivityRendererBase).select_index(tap)
		for i: int in 10:
			await await_idle_frame()
	return host


## F099-AC-15: the activity player on a media attempt; the playback is swapped for the scripted fakes (TEST-ONLY) and
## moved to `at_ms` (a video at its question opens the question on the board).
func _media(vp: SubViewport, band: String, attempt: Dictionary, at_ms: int = -1) -> Node:
	s.h.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/vi.vtt*", 200,
			VideoFixtures.VTT_VI.to_utf8_buffer())
	s.h.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/en.vtt*", 200,
			VideoFixtures.VTT_EN.to_utf8_buffer())
	s.services.video_files.downloader = VideoFixtures.fake_download
	var host: Control = _mount(vp, _scene(HOST)) as Control
	host.set("services", s.services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	s.h.http.delay_frames = 1
	s.h.http.clear(LearningFixtures.ROUTE_START)
	s.h.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 3:
		await await_idle_frame()
	var r: ActivityRendererBase = host.get("renderer") as ActivityRendererBase
	if r is VideoLessonRenderer:
		var v: VideoLessonRenderer = r as VideoLessonRenderer
		var fake: FakeVideoPlayback = FakeVideoPlayback.new()
		v.playback.pause()
		v.playback = fake
		fake.pos = maxi(at_ms, 0)
		fake.playing = true
		v._process(0.016)
	elif r is AudioLessonRenderer:
		var a: AudioLessonRenderer = r as AudioLessonRenderer
		var fake_audio: FakeAudioPlayback = FakeAudioPlayback.new()
		a.playback.pause()
		a.playback = fake_audio
		fake_audio.pos = maxi(at_ms, 0)
		(a.get_node("%Toast") as Control).visible = false
		a._refresh()
	return host


func _correct_with_answer() -> Dictionary:
	var r: Dictionary = LearningFixtures.response_result()
	r["correct_response"] = {"option_id": "o_b"}
	return r


# ---------- tests ----------

func test_login_fits_every_band() -> void:
	AuthService.form.load_memory(true, "", false)
	AuthService.form.end_submit("")
	_assert_none(await _matrix("login", func(vp: SubViewport, _band: String) -> Node:
		return _mount(vp, _scene("res://scenes/auth/Login.tscn"))))


func test_hub_hud_fits_every_band() -> void:
	await s.services.progression.refresh()
	await s.services.inventory.load_wallet()
	_assert_none(await _matrix("hud", func(vp: SubViewport, _band: String) -> Node:
		return _mount(vp, _scene("res://scenes/ui/hud/MainHud.tscn"))))


func test_activity_single_choice_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]))
	_assert_none(await _matrix("activity", build, true))


func test_activity_true_false_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["true_false"]))
	_assert_none(await _matrix("true_false", build, true))


func test_activity_matching_fits_every_band() -> void:
	_assert_none(await _matrix("matching", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["matching"]))))


func test_activity_numeric_fits_every_band() -> void:
	_assert_none(await _matrix("numeric", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["numeric"]))))


func test_feedback_grading_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]),
				LearningFixtures.response_result_correct(), 1, true)
	_assert_none(await _matrix("tap_grading", build, true))


func test_feedback_right_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]),
				LearningFixtures.response_result_correct(), 1)
	_assert_none(await _matrix("tap_correct", build, true))


func test_feedback_not_yet_right_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]), _correct_with_answer(), 0)
	_assert_none(await _matrix("tap_incorrect", build, true))


## F099-AC-15: video playing with its caption card, the question inside the video, an audio lesson, a simulation.
func test_video_player_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _media(vp, band, VideoFixtures.attempt(), 6000)
	_assert_none(await _matrix("video", build))


func test_video_question_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _media(vp, band, VideoFixtures.attempt(), 30000)
	_assert_none(await _matrix("video_cue", build))


func test_audio_player_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _media(vp, band, AudioFixtures.attempt(2, 1), 12000)
	_assert_none(await _matrix("audio", build))


func test_simulation_fits_every_band() -> void:
	var build: Callable = func(vp: SubViewport, band: String) -> Node:
		return await _media(vp, band, SimulationFixtures.attempt())
	_assert_none(await _matrix("simulation", build))


func test_result_fits_every_band() -> void:
	_assert_none(await _matrix("result", func(vp: SubViewport, band: String) -> Node:
		var view: Control = _mount(vp, _scene("res://scenes/learning/ActivityResult.tscn")) as Control
		view.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
		view.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), band,
				[{"kind": "xp", "amount": 40}, {"kind": "badge"}])
		return view))


func test_character_creation_fits_every_band() -> void:
	_assert_none(await _matrix("character", func(vp: SubViewport, _band: String) -> Node:
		# A first-time student (no avatar, no nickname yet): S02 opens on its first step.
		ProfileFixtures.script_first_time(s.h.http)
		s.services.profile.profile = null
		var screen: Control = _mount(vp, _scene("res://scenes/character/CharacterCreation.tscn")) as Control
		for i: int in 40:
			await await_idle_frame()
			var model: CharacterCreationModel = screen.get("model") as CharacterCreationModel
			if model != null and model.state != CharacterCreationModel.State.LOADING:
				break
		var loaded: CharacterCreationModel = screen.get("model") as CharacterCreationModel
		assert_int(loaded.state).override_failure_message("S02 not ready").is_equal(CharacterCreationModel.State.READY)
		return screen))


func test_wardrobe_fits_every_band() -> void:
	_assert_none(await _matrix("wardrobe", func(vp: SubViewport, _band: String) -> Node:
		var screen: Control = _mount(vp, _scene("res://scenes/inventory/InventoryShopScreen.tscn")) as Control
		screen.call("open", {"tab": "wardrobe"})
		for i: int in 8:
			await await_idle_frame()
		screen.call("select_entry", 1)
		return screen))


func test_shop_fits_every_band() -> void:
	_assert_none(await _matrix("shop", func(vp: SubViewport, _band: String) -> Node:
		var screen: Control = _mount(vp, _scene("res://scenes/inventory/InventoryShopScreen.tscn")) as Control
		screen.call("open", {"tab": "shop"})
		for i: int in 8:
			await await_idle_frame()
		screen.call("select_entry", 0)
		return screen))


func test_level_up_fits_every_band() -> void:
	_assert_none(await _matrix("level_up", func(vp: SubViewport, band: String) -> Node:
		var modal: Control = _mount(vp, _scene("res://scenes/progression/LevelUpModal.tscn")) as Control
		var list: Array[LevelUpModel] = [LevelUpModel.from_dict(F069Fixtures.level_up(3, 4, true))]
		modal.call("present", LevelUpModel.merge(list), ProgressionModel.from_dict(F069Fixtures.progression(4, 20, 140)),
				band)
		await await_idle_frame()
		modal.call("skip_effect")
		return modal))


func test_world_map_fits_every_band() -> void:
	_assert_none(await _matrix("world_map", func(vp: SubViewport, band: String) -> Node:
		var map: Control = _mount(vp, _scene("res://scenes/world/WorldMap.tscn")) as Control
		map.show()
		map.call("render", WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), band, "main_hub"))
		return map))


## F113-AC-05: a window taller than wide (720×1280, 390×844) shows only the rotate prompt; it covers the view and its
## texts keep the band sizes (on the device the window content scale lays it out on the base short side — see
## test_orientation_guard.gd; here the SubViewport has the logical size of the unscaled window, the hardest case).
func test_rotate_prompt_fits_every_band() -> void:
	var covers: PackedStringArray = []
	var build: Callable = func(vp: SubViewport, _band: String) -> Node:
		var guard: Node = (load("res://scripts/core/orientation_guard.gd") as GDScript).new() as Node
		guard.set("follow_window", false)
		guard.set("set_paused", func(_value: bool) -> void: pass)
		guard.set("is_paused", func() -> bool: return false)
		guard.set("lock_landscape", func() -> void: pass)
		guard.set("set_content_scale", func(_value: float) -> void: pass)
		guard.set("get_content_scale", func() -> float: return 1.0)
		vp.add_child(guard)
		guard.call("apply", Vector2(vp.size))
		for i: int in 2:
			await await_idle_frame()
		var prompt: Control = guard.call("prompt") as Control
		if not prompt.size.is_equal_approx(Vector2(vp.size)):
			covers.append("prompt %s does not cover %s" % [prompt.size, vp.size])
		return guard
	_assert_none(await _matrix("rotate_prompt", build, false, BandLayoutProbe.PORTRAIT_SIZES))
	assert_array(covers).override_failure_message("\n".join(covers)).is_empty()


## F114-AC-14: S15 Cài đặt — every tab, the licences page and the logout confirmation fit every band × window
## (controls ≥ the band's touch target, text ≥ its smallest font, nothing cut or outside the window).
func _settings(vp: SubViewport, view: String) -> Node:
	var panel: Control = _mount(vp, _scene("res://scenes/ui/settings/SettingsPanel.tscn")) as Control
	panel.call("open")
	match view:
		"licenses":
			panel.call("show_tab", "info")
			panel.call("show_licenses")
		"logout":
			(panel.find_child("Logout", true, false) as Button).pressed.emit()
		_:
			panel.call("show_tab", view)
	return panel


func test_settings_fits_every_band() -> void:
	for view: String in ["language", "graphics", "sound", "access", "notifications", "info", "licenses", "logout"]:
		var failures: PackedStringArray = await _matrix("settings_" + view, func(vp: SubViewport, _band: String) -> Node:
			return _settings(vp, view))
		# The engine's licence texts are shown verbatim (F096-AC-04): their separator rules (80 '*' / '-' / '=') are
		# longer than a line and wrap; every other check of the page still applies.
		var kept: PackedStringArray = []
		for line: String in failures:
			var word: String = line.get_slice("\"", 1)
			if not (line.contains("LicenseText: word cut across lines")
					and word.replace("*", "").replace("-", "").replace("=", "").is_empty()):
				kept.append(line)
		_assert_none(kept)


## F114 edge case: the largest text size (+2) still fits; long pages scroll inside the window, the tab column is not cut.
func test_settings_fits_every_band_with_the_largest_text() -> void:
	AppState.theme_service.set_font_scale(2)
	var failures: PackedStringArray = []
	for view: String in ["language", "sound", "access", "notifications", "logout"]:
		failures.append_array(await _matrix("settings_text+2_" + view, func(vp: SubViewport, _band: String) -> Node:
			return _settings(vp, view)))
	AppState.theme_service.set_font_scale(0)
	_assert_none(failures)


## F123-AC-19: the HUD with a two-digit bell count and three toasts (an invitation, school work, results).
func test_hub_with_notifications_fits_every_band() -> void:
	await s.services.progression.refresh()
	await s.services.inventory.load_wallet()
	_assert_none(await _matrix("hud_notifications", func(vp: SubViewport, _band: String) -> Node:
		s.services.notifications.reset()
		s.h.http.reply(NotificationFixtures.ROUTE_LIST, 200, NotificationFixtures.page(NotificationFixtures.sample_items(),
				12))
		var hud: Node = _mount(vp, _scene("res://scenes/ui/hud/MainHud.tscn"))
		await s.services.notifications.load_unread()
		for name: String in ["friend_request", "assignment_new", "assignment_new"]:
			s.services.notifications.handle_message(NotificationService.MSG_CREATED, NotificationFixtures.created(name))
		return hud))


## F123-AC-19: the notification inbox with its five sample rows ("Mới", "Xem") and empty (Mây's bubble).
func _inbox(vp: SubViewport, empty: bool) -> Node:
	s.services.notifications.reset()
	NotificationFixtures.script(s.h.http, empty)
	var inbox: Control = _mount(vp, _scene("res://scenes/ui/notifications/NotificationInbox.tscn")) as Control
	inbox.set("service", s.services.notifications)
	inbox.call("open")
	for i: int in 6:
		await await_idle_frame()
	return inbox


func test_notification_inbox_fits_every_band() -> void:
	_assert_none(await _matrix("notification_inbox", func(vp: SubViewport, _band: String) -> Node:
		return await _inbox(vp, false)))


func test_notification_inbox_empty_fits_every_band() -> void:
	_assert_none(await _matrix("notification_inbox_empty", func(vp: SubViewport, _band: String) -> Node:
		return await _inbox(vp, true)))


## The probe itself catches what it must (a guard against a silent matrix).
func test_probe_flags_small_buttons_small_fonts_and_overflow() -> void:
	AppState.set_grade_band("g1_3")
	var vp: SubViewport = auto_free(SubViewport.new()) as SubViewport
	vp.size = Vector2i(1280, 720)
	add_child(vp)
	var root: Control = Control.new()
	root.theme = AppState.theme_service.current_theme
	root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	vp.add_child(root)
	var small: Button = Button.new()
	small.text = "x"
	small.custom_minimum_size = Vector2(10, 10)
	root.add_child(small)
	var off: Label = Label.new()
	off.text = "off screen"
	off.position = Vector2(1400, 10)
	root.add_child(off)
	var tiny: Label = Label.new()
	tiny.text = "tiny"
	tiny.add_theme_font_size_override(&"font_size", 10)
	tiny.position = Vector2(10, 300)
	root.add_child(tiny)
	var cut: Label = Label.new()
	cut.text = "a long line that cannot fit"
	cut.clip_text = true
	cut.size = Vector2(30, 40)
	cut.position = Vector2(10, 400)
	root.add_child(cut)
	await await_idle_frame()
	await await_idle_frame()
	# The small button is held at its theme minimum height; force the probe to see a too-small one.
	small.size = Vector2(10, 10)
	var text: String = "\n".join(BandLayoutProbe.check(root, vp, "g1_3", "probe"))
	assert_str(text).contains("outside the view")
	assert_str(text).contains("font 10 px")
	assert_str(text).contains("trimmed text")


## F099-AC-07: the empty-band rule catches a board with a big hole (a guard against a silent check).
func test_probe_flags_an_empty_band() -> void:
	AppState.set_grade_band("g8_9")
	var vp: SubViewport = auto_free(SubViewport.new()) as SubViewport
	vp.size = Vector2i(1280, 720)
	add_child(vp)
	var board: PanelContainer = PanelContainer.new()
	board.theme = AppState.theme_service.current_theme
	board.theme_type_variation = &"ActivityBoard"
	board.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	vp.add_child(board)
	var box: VBoxContainer = VBoxContainer.new()
	board.add_child(box)
	var top: Label = Label.new()
	top.text = "top"
	box.add_child(top)
	var hole: Control = Control.new()
	hole.size_flags_vertical = Control.SIZE_EXPAND_FILL
	box.add_child(hole)
	var bottom: Button = Button.new()
	bottom.text = "bottom"
	box.add_child(bottom)
	await await_idle_frame()
	await await_idle_frame()
	var text: String = "
".join(BandLayoutProbe.empty_bands(board, vp, "probe"))
	assert_str(text).contains("empty band")
	# A filled board passes.
	hole.custom_minimum_size.y = 0.0
	hole.size_flags_vertical = Control.SIZE_FILL
	bottom.size_flags_vertical = Control.SIZE_EXPAND_FILL
	await await_idle_frame()
	await await_idle_frame()
	assert_array(BandLayoutProbe.empty_bands(board, vp, "probe")).is_empty()
