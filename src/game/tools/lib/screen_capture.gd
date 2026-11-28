extends Node
## Worker of tools/capture_screens.gd (F096): builds each student screen of the journey on the contract fixtures and
## fakes of res://tests/fakes (FakeHttp, FakeClock — TEST-ONLY data, AGENTS R17: no network, no server state) and
## saves the window to `<screen>-<tag>-<size>-<lang>-<band>.png`. Loaded at run time so the autoloads exist when it
## compiles. Nothing here ships (tools/* is excluded from every export preset).

const SETTLE_FRAMES: int = 10
const SettingsScript := preload("res://scripts/core/settings.gd")
const SETTINGS_PATH: String = "user://capture_screens_settings.cfg"
## F113: landscape windows only (STUDENT_GAME_UX §2) plus the phone held upright (390×844), which shows the rotate
## prompt instead of the game. An unknown size is an error (never a silent fallback).
const SIZES: Dictionary = {
	"1280x720": Vector2i(1280, 720), "1024x600": Vector2i(1024, 600), "844x390": Vector2i(844, 390),
	"740x360": Vector2i(740, 360),
	"390x844": Vector2i(390, 844),
}
## Screen name → builder method. Order follows the student journey (STUDENT_GAME_UX §11).
const SCREENS: PackedStringArray = [
	"splash", "update_required", "update_required_web", "update_required_teacher", "login", "login_busy", "login_busy_cancel", "login_error_busy", "resume", "curfew", "character",
	"hub", "hub_onboarding", "hub_onboarding_step2", "hub_offline", "journal", "activity", "activity_selected",
	"activity_feedback", "activity_incorrect", "activity_hint", "activity_multiple", "activity_true_false",
	"activity_fill_blank", "activity_numeric", "activity_matching", "activity_ordering", "activity_drag_drop",
	"result", "world_map", "settings", "about",
	# F114: the Settings window per tab, the logout confirmation over it.
	"settings_graphics", "settings_sound", "settings_access", "settings_info", "settings_logout",
	# F069: progression, economy, achievements, solo challenge.
	"hub_progress", "game_menu", "level_up", "level_up_multi", "journey", "inventory", "shop", "shop_confirm", "shop_need_more",
	"history", "achievements", "collection", "showcase", "challenge_board", "challenge_locked", "challenge_no_tries",
	"challenge_hud", "challenge_result_failed", "challenge_result_succeeded",
	# F098: tap-to-answer (grading, right, not yet right + right answer from the server, two-tap confirmation).
	"activity_tap_grading", "activity_tap_correct", "activity_tap_incorrect", "activity_tap_confirm",
	# F099: media players (video playing with a caption, video question, audio lesson, simulation).
	"activity_video", "activity_video_cue", "activity_audio", "activity_simulation",
	# F113: a window taller than wide (capture with --sizes=390x844): the rotate prompt covers the game.
	"rotate_prompt",
	# F123: in-game notifications — bell count + toasts on the HUD, the inbox (rows / empty), the Settings tab.
	"hub_notifications", "notifications", "notifications_empty", "settings_notifications",
	# F102: the Party screen (leader, member with a reconnecting leader, alone + invitation), the invite picker, the
	# activity choice, the leave confirmation, the 3-2-1 countdown and the HUD party strip + invitation popup.
	"party", "party_member", "party_alone", "party_invite", "party_activity", "party_confirm", "party_countdown",
	"hub_party",
	# F119: the party lobby with its chat open (phone overlay / PC column) and the chat dock of a team battle in each state.
	"party_chat", "chat_team", "chat_menu", "chat_report", "chat_thanks", "chat_picker", "chat_blocked", "chat_muted",
	"chat_closed", "chat_care", "chat_mini",
	# F109: the Competition Arena (modes, leaderboards per server visibility, history, tournaments + bracket, duel), the
	# HUD queue bar / ready check and the battle screen (question, answer recorded, reveal, team, reconnect, results).
	"arena", "arena_leaderboard_goal", "arena_leaderboard_top", "arena_leaderboard_ranked", "arena_history",
	"arena_history_detail", "arena_tournaments", "arena_tournament_bracket", "arena_duel", "arena_queue",
	"arena_queue_timeout", "arena_ready_check", "arena_battle_question", "arena_battle_recorded", "arena_battle_reveal",
	"arena_battle_team", "arena_battle_reconnect", "arena_result_win", "arena_result_loss", "arena_result_quiz_goal",
	"arena_result_quiz_rank", "arena_result_cancelled",
	# F109 server deltas (2026-11-27): a friend's duel prompt on the HUD, the loss-streak encouragement, a tournament problem with its own sentence (`409 TOURNAMENT_FULL`). F110: F110Screens.CAPTURES. F111: F111Screens.CAPTURES. F112: F112Screens.CAPTURES (HUD chat, search, Quản lý club).
	"arena_duel_incoming", "arena_result_encouragement", "arena_tournament_error",
]
## Sample look of the local player (catalog codes of the F070 fixture; display only).
const SAMPLE_AVATAR: Dictionary = {
	"body_type": "kid_a", "skin_tone": "tone_03", "hair": {"item_id": "hair_bob_01", "color": "hair_col_02"},
	"outfit": {"item_id": "outfit_school_01"}, "accessories": [{"slot": "back", "item_id": "acc_backpack_owl"}],
	"emote_slots": ["emote_wave", "emote_clap"],
}

var _harness: AuthHarness = null
var _settings: Node = null
var _band: String = "g1_3"


var _args: PackedStringArray = []


func _arg(name: String, fallback: String) -> String:
	for arg: String in _args:
		if arg.begins_with("--%s=" % name):
			return arg.trim_prefix("--%s=" % name)
	return fallback


## Coroutine: captures the screens selected by `args` (see tools/capture_screens.gd); returns the exit code.
func run(args: PackedStringArray) -> int:
	_args = args
	var out_dir: String = _arg("out", ProjectSettings.globalize_path("user://capture_screens"))
	var tag: String = _arg("tag", "after")
	_band = _arg("band", "g1_3")
	var only: PackedStringArray = _arg("screens", "").split(",", false)
	var sizes: PackedStringArray = _arg("sizes", "1280x720").split(",", false)
	var locales: PackedStringArray = _arg("locales", "vi-VN").split(",", false)
	DirAccess.make_dir_recursive_absolute(out_dir)
	DirAccess.remove_absolute(SETTINGS_PATH)
	_settings = SettingsScript.new() as Node
	_settings.call("load_from", SETTINGS_PATH)
	_settings.set("quality_preset", QualityPolicy.MEDIUM)
	QualityManager.settings = _settings
	QualityManager.apply_preset(QualityPolicy.MEDIUM, false)
	AuthService.autostart = false
	# F097: the density Theme of the band (as Boot does), so g6_7 captures show the g6_7 sizes.
	if AppState.theme_service == null:
		AppState.theme_service = ThemeService.new(_band)
	AppState.theme_service.attach(get_tree().root)
	var count: int = 0
	for screen: String in SCREENS + F110Screens.CAPTURES + F111Screens.CAPTURES + F112Screens.CAPTURES:
		if not only.is_empty() and not screen in only:
			continue
		for size_name: String in sizes:
			for locale: String in locales:
				if not SIZES.has(size_name):
					printerr("capture_screens: unknown size %s (known: %s)" % [size_name, ", ".join(SIZES.keys())])
					return 2
				var size: Vector2i = SIZES[size_name] as Vector2i
				DisplayServer.window_set_size(size)
				get_tree().root.size = size
				AppState.set_grade_band(_band)
				Settings.set_locale(locale, false)
				var node: Node = null
				if screen.begins_with("f112_"):
					node = await F112Screens.capture(await _world(), screen.trim_prefix("f112_"), _harness.http)
				elif screen.begins_with("f111_"):
					node = await F111Screens.capture(await _world(), screen.trim_prefix("f111_"), _harness.http)
				elif screen.begins_with("f110_"):
					node = await F110Screens.capture(await _world(), screen.trim_prefix("f110_"), _harness.http)
				elif screen.begins_with("arena"):
					# F109: every arena screen is one CompetitionScreens view.
					node = await _arena_world("arena" if screen == "arena" else screen.trim_prefix("arena_"))
				elif screen.begins_with("update_required"):
					node = _update_required(screen)
				else:
					node = await call("_screen_" + screen) as Node
				if node == null:
					printerr("capture_screens: %s could not be built" % screen)
					continue
				for i: int in SETTLE_FRAMES:
					await get_tree().process_frame
				await RenderingServer.frame_post_draw
				var image: Image = get_tree().root.get_texture().get_image()
				var file: String = "%s-%s-%s-%s-%s.png" % [screen, tag, size_name, locale.substr(0, 2), _band]
				image.save_png(out_dir.path_join(file))
				count += 1
				node.queue_free()
				await _reset()
	print("capture_screens: %d images (%s, %s) in %s" % [count, tag, RenderingServer.get_current_rendering_method(),
			out_dir])
	_settings.free()
	DirAccess.remove_absolute(SETTINGS_PATH)
	return 0


func _reset() -> void:
	await get_tree().process_frame
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray())
	if _harness != null:
		_harness.dispose()
		_harness = null
	AppState.player = {}
	AuthService.form.reset_for_new_user()
	await get_tree().process_frame


## A signed-in student on FakeHttp with the services wired to it (AppState.services).
func _sign_in(first_time: bool = false) -> void:
	_harness = AuthHarness.create()
	await _harness.sign_in()
	if first_time:
		ProfileFixtures.script_first_time(_harness.http)
	else:
		ProfileFixtures.script_ready(_harness.http)
	_harness.http.reply(WorldFixtures.ROUTE_AREAS, 200, WorldFixtures.areas())
	_harness.http.reply(WorldFixtures.ROUTE_EXPLORATION, 200, WorldFixtures.exploration())
	_harness.http.reply("GET /learning/my-assignments", 200, AssignmentFixtures.two())
	_harness.http.reply("GET /game/quests", 200, {"data": [], "page": {"has_more": false}})
	F069Fixtures.script_all(_harness.http, _band)
	# F123: two unread notifications (bell count) and the inbox sample.
	NotificationFixtures.script(_harness.http)
	# F102: no party by default (the HUD shows the "Nhóm" button); the party screens script their own party.
	PartyFixtures.script(_harness.http, "", true)
	AppState.services = ServiceLocator.new(_harness.api, AppState, _harness.clock)
	AppState.player = AuthHarness.bootstrap()
	AppState.set_grade_band(_band)
	# F119: the chat fixtures (party channel history, quick messages, stickers on the fixture CDN).
	ChatScreens.prepare(AppState.get_services(), _harness.http)


func _add(path: String) -> Node:
	var node: Node = (load(path) as PackedScene).instantiate()
	get_tree().root.add_child(node)
	return node


func _screen_splash() -> Node:
	return _add("res://scenes/bootstrap/Splash.tscn")


## F133 T023: "Cần cập nhật" — update_required (store page), _web (reload the page), _teacher (ask the teacher).
func _update_required(screen: String) -> Node:
	AuthService.min_client_version = "1.3.0"
	var node: Node = _add("res://scenes/bootstrap/UpdateRequired.tscn")
	node.call("show_action", {"update_required_web": UpdateService.Action.RELOAD_PAGE,
		"update_required_teacher": UpdateService.Action.ASK_TEACHER}.get(screen, UpdateService.Action.OPEN_STORE))
	return node


func _screen_login() -> Node:
	return _add("res://scenes/auth/Login.tscn")


func _screen_login_busy() -> Node:
	AuthService.form.begin_submit("hs.an.3a")
	return _add("res://scenes/auth/Login.tscn")


## S01 §8 "hủy được sau 10 s": the cancel button of a slow sign-in.
func _screen_login_busy_cancel() -> Node:
	# Pretends the sign-in started 11 s ago (the button appears after 10 s).
	AuthService.form.begin_submit("hs.an.3a")
	var node: Node = _add("res://scenes/auth/Login.tscn")
	await get_tree().process_frame
	node.set("busy_since_msec", int(node.get("busy_since_msec")) - 11000)
	return node


func _screen_login_error_busy() -> Node:
	AuthService.form.begin_submit("hs.an.3a")
	AuthService.form.end_submit("UNAVAILABLE")
	return _add("res://scenes/auth/Login.tscn")


func _screen_resume() -> Node:
	return _add("res://scenes/auth/ResumeSplash.tscn")


func _screen_curfew() -> Node:
	AuthService.reopen_text = "06:00"
	return _add("res://scenes/auth/CurfewNotice.tscn")


func _screen_character() -> Node:
	await _sign_in(true)
	var node: Node = _add("res://scenes/character/CharacterCreation.tscn")
	for i: int in 60:
		await get_tree().process_frame
		var model: CharacterCreationModel = node.get("model") as CharacterCreationModel
		if model != null and model.state != CharacterCreationModel.State.LOADING:
			break
	return node


func _world(onboarding: bool = false) -> Node:
	await _sign_in()
	# Device flag of Mây's guide in RAM only (never saved by the capture).
	Settings.onboarding_hub_seen = not onboarding
	var world: Node = _add("res://scenes/world/WorldRoot.tscn")
	for i: int in 30:
		await get_tree().process_frame
	var profile: ProfileService = AppState.get_services().profile
	if profile.profile == null:
		await profile.fetch_profile()
	if profile.profile != null:
		profile.profile.avatar_config = AvatarConfigModel.from_dict(SAMPLE_AVATAR)
	var zone: ZoneRoot = world.get("zone") as ZoneRoot
	if zone != null and zone.player != null:
		var rig: AvatarBuilder = zone.player.get_node_or_null("AvatarRig") as AvatarBuilder
		if rig != null:
			rig.apply(AvatarConfigModel.from_dict(SAMPLE_AVATAR))
	# The equipped emotes of the sample look (the HUD emote button is enabled as for a student with emotes).
	var wheel: EmoteWheel = world.find_child("EmoteWheel", true, false) as EmoteWheel
	if wheel != null:
		wheel.set_emotes(AvatarConfigModel.from_dict(SAMPLE_AVATAR).emote_slots)
	return world


func _screen_hub() -> Node:
	return await _world()


func _screen_hub_onboarding() -> Node:
	return await _world(true)


func _screen_hub_onboarding_step2() -> Node:
	var world: Node = await _world(true)
	var hud: CanvasLayer = world.find_child("MainHud", true, false) as CanvasLayer
	(hud.call("onboarding") as OnboardingHint).next()
	return world


func _screen_hub_offline() -> Node:
	var world: Node = await _world()
	var hud: CanvasLayer = world.find_child("MainHud", true, false) as CanvasLayer
	hud.call("show_hub_notice", "ui.realtime.reconnecting")
	return world


func _open_panel(world: Node, screen_id: StringName, args: Dictionary = {}) -> void:
	EventBus.ui_requested.emit(screen_id, args)
	for i: int in 40:
		await get_tree().process_frame


func _screen_journal() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"quest_journal", {"tab": "assigned"})
	return world


func _screen_world_map() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"world_map")
	return world


func _screen_settings() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"settings")
	return world


## F114: the Settings window on one tab (older panels without tabs show their only page).
func _settings_tab(tab: String) -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"settings")
	var panel: Node = world.find_child("SettingsPanel", true, false)
	if panel != null and panel.has_method("show_tab"):
		panel.call("show_tab", tab)
	return world


func _screen_settings_graphics() -> Node:
	return await _settings_tab("graphics")


func _screen_settings_sound() -> Node:
	return await _settings_tab("sound")


func _screen_settings_access() -> Node:
	return await _settings_tab("access")


func _screen_settings_info() -> Node:
	return await _settings_tab("info")


## F114: "Đăng xuất" pressed → the confirmation over the Settings window (nested scrim).
func _screen_settings_logout() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"settings")
	var logout: Button = world.find_child("Logout", true, false) as Button
	if logout != null:
		logout.pressed.emit()
	for i: int in 20:
		await get_tree().process_frame
	return world


## F123: two notifications arrive while the student plays — an invitation and new school work (toasts + bell count).
func _screen_hub_notifications() -> Node:
	var world: Node = await _world()
	var notes: NotificationService = AppState.get_services().notifications
	notes.handle_message(NotificationService.MSG_CREATED, NotificationFixtures.created("friend_request"))
	notes.handle_message(NotificationService.MSG_CREATED, NotificationFixtures.created("assignment_new"))
	return world


## F123: the inbox from the bell (rows newest first, "Mới" on the unread ones).
func _screen_notifications() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"notification_inbox")
	return world


## F123: the inbox without any notification (Mây explains where they come from).
func _screen_notifications_empty() -> Node:
	var world: Node = await _world()
	NotificationFixtures.script(_harness.http, true)
	AppState.get_services().notifications.reset()
	await _open_panel(world, &"notification_inbox")
	return world


func _screen_settings_notifications() -> Node:
	return await _settings_tab("notifications")


# ---------- F102 party ----------

## The world with the sample party of the signed-in student (leader, or a member when `member`) loaded.
func _party_world(alone: bool = false, member: bool = false) -> Node:
	var world: Node = await _world()
	var party: PartyService = AppState.get_services().party
	party._refresh_self()
	PartyFixtures.script(_harness.http, party.store.self_id, alone)
	if member:
		var data: String = JSON.stringify(PartyFixtures.party()).replace(PartyFixtures.MEMBER, party.store.self_id)
		_harness.http.clear(PartyFixtures.ROUTE_MY_PARTY).reply(PartyFixtures.ROUTE_MY_PARTY, 200, JSON.parse_string(data))
	if alone:
		_harness.http.clear(PartyFixtures.ROUTE_INVITATIONS).reply(PartyFixtures.ROUTE_INVITATIONS, 200,
				PartyFixtures.invitations(party.now_ms()))
	await party.load_my_party()
	if not alone:
		await party.load_eligible()
	return world


func _party_lobby(world: Node) -> PartyLobby:
	await _open_panel(world, &"party")
	return world.find_child("PartyLobby", true, false) as PartyLobby


## Leader of "Nhóm Sao Băng": cards, the quest chosen, GauNau15 not ready yet (Start grey with the reason).
func _screen_party() -> Node:
	var world: Node = await _party_world()
	await _party_lobby(world)
	return world


## A member: the leader is reconnecting (banner, faded card) and GauNau15 is ready.
func _screen_party_member() -> Node:
	var world: Node = await _party_world(false, true)
	var party: PartyService = AppState.get_services().party
	await _party_lobby(world)
	party.handle_message("party.member.status_changed", {"party_id": PartyFixtures.PARTY_ID, "user_id": PartyFixtures.LEADER,
		"status": "disconnected"})
	party.handle_message("party.member.ready_changed", PartyFixtures.ws("member.ready_changed.ready"))
	return world


## Alone: Mây, "Tạo nhóm và mời bạn" and a pending invitation.
func _screen_party_alone() -> Node:
	var world: Node = await _party_world(true)
	await _party_lobby(world)
	return world


func _screen_party_invite() -> Node:
	var world: Node = await _party_world()
	var lobby: PartyLobby = await _party_lobby(world)
	lobby.picker().open(AppState.get_services().party)
	for i: int in 20:
		await get_tree().process_frame
	return world


func _screen_party_activity() -> Node:
	var world: Node = await _party_world()
	var lobby: PartyLobby = await _party_lobby(world)
	lobby.selector().open(AppState.get_services().party, _band)
	for i: int in 20:
		await get_tree().process_frame
	if not lobby.selector().uses_cards():
		lobby.selector().dropdown_button().pressed.emit()
	return world


func _screen_party_confirm() -> Node:
	var world: Node = await _party_world()
	var lobby: PartyLobby = await _party_lobby(world)
	lobby.leave_button().pressed.emit()
	return world


func _screen_party_countdown() -> Node:
	var world: Node = await _party_world()
	var lobby: PartyLobby = await _party_lobby(world)
	lobby.start_countdown()
	return world


## The HUD in a party (member strip) with an invitation popup from another party.
func _screen_hub_party() -> Node:
	var world: Node = await _party_world()
	var party: PartyService = AppState.get_services().party
	var hud: CanvasLayer = world.find_child("MainHud", true, false) as CanvasLayer
	var invitation: Dictionary = PartyFixtures.invitation_ws(party.now_ms())
	(invitation["inviter"] as Dictionary)["nickname"] = "SaoBang12"
	hud.call("show_invitation", InvitationModel.from_dict(invitation))
	return world


# ---------- F119 chat ----------

## The party lobby with the party chat: open in the activity column on a PC, opened from "Chat nhóm" on a phone.
func _screen_party_chat() -> Node:
	var world: Node = await _party_world()
	var lobby: PartyLobby = await _party_lobby(world)
	if lobby.chat_button().visible:
		lobby.chat_button().pressed.emit()
	for i: int in 20:
		await get_tree().process_frame
	return world


## The world with the chat dock of a team battle in one state (where the match / boss screens embed it).
func _chat_world(view: String) -> Node:
	var world: Node = await _world()
	var layer: CanvasLayer = CanvasLayer.new()
	layer.name = "ChatLayer"
	layer.layer = 20
	world.add_child(layer)
	var screen: Control = Control.new()
	screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
	layer.add_child(screen)
	await ChatScreens.build(view, screen, AppState.get_services(), _harness.http)
	for i: int in 20:
		await get_tree().process_frame
	return world


func _screen_chat_team() -> Node:
	return await _chat_world("team")


func _screen_chat_menu() -> Node:
	return await _chat_world("menu")


func _screen_chat_report() -> Node:
	return await _chat_world("report")


func _screen_chat_thanks() -> Node:
	return await _chat_world("thanks")


func _screen_chat_picker() -> Node:
	return await _chat_world("picker")


func _screen_chat_blocked() -> Node:
	return await _chat_world("blocked")


func _screen_chat_muted() -> Node:
	return await _chat_world("muted")


func _screen_chat_closed() -> Node:
	return await _chat_world("closed")


func _screen_chat_care() -> Node:
	return await _chat_world("care")


func _screen_chat_mini() -> Node:
	return await _chat_world("mini")


# ---------- F109 arena ----------

## The world with one arena state (CompetitionScreens) on a layer above it: the arena window / HUD queue bar / ready
## check over the world, the battle screen covering it.
func _arena_world(view: String) -> Node:
	var world: Node = await _world()
	var layer: CanvasLayer = CanvasLayer.new()
	layer.name = "ArenaLayer"
	layer.layer = 20
	world.add_child(layer)
	var screen: Control = Control.new()
	screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
	layer.add_child(screen)
	await CompetitionScreens.build(view, screen, AppState.get_services(), _harness.http)
	for i: int in 20:
		await get_tree().process_frame
	return world


func _screen_about() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"settings")
	var panel: Node = world.find_child("SettingsPanel", true, false)
	if panel != null:
		panel.call("show_licenses")
	return world


func _activity_host(types: PackedStringArray = []) -> Control:
	await _sign_in()
	var host: Control = (load("res://scenes/learning/ActivityHost.tscn") as PackedScene).instantiate() as Control
	host.set("services", AppState.services)
	host.set("client_version", AuthHarness.VERSION)
	host.set("grade_band", _band)
	get_tree().root.add_child(host)
	var attempt: Dictionary = LearningFixtures.fresh_attempt() if types.is_empty() \
			else LearningFixtures.attempt_with_types(types)
	_harness.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	_harness.http.reply(LearningFixtures.ROUTE_HINT, 200, LearningFixtures.hint())
	_harness.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	_harness.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	_harness.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 60:
		await get_tree().process_frame
	return host


func _screen_activity() -> Node:
	return await _activity_host()


func _screen_activity_feedback() -> Node:
	var host: Control = await _activity_host()
	var service: LearningService = host.get("service") as LearningService
	await _select(host, 1)
	if service != null:
		host.call("_apply_result", ResponseResultModel.from_dict(LearningFixtures.response_result_correct()))
	return host


func _screen_result() -> Node:
	var host: Control = await _activity_host()
	var service: LearningService = host.get("service") as LearningService
	service.last_rewards = [{"kind": "xp", "amount": 40}, {"kind": "badge"}]
	host.call("_show_results", false)
	for i: int in 60:
		await get_tree().process_frame
	# Let the celebration finish (≤ CelebrationBanner.ANIMATION_MAX_SEC) so the stars are drawn.
	await get_tree().create_timer(CelebrationBanner.ANIMATION_MAX_SEC + 0.2).timeout
	return host


## F097: the student picked the second answer (before "Kiểm tra").
func _screen_activity_selected() -> Node:
	var host: Control = await _activity_host()
	await _select(host, 1)
	return host


func _select(host: Control, index: int) -> void:
	var renderer: ActivityRendererBase = host.get("renderer") as ActivityRendererBase
	if renderer != null:
		renderer.select_index(index)
	await get_tree().process_frame


## F097: a wrong answer with the option feedback and the explanation (kind wording, no penalty).
func _screen_activity_incorrect() -> Node:
	var host: Control = await _activity_host()
	await _select(host, 0)
	host.call("_apply_result", ResponseResultModel.from_dict(LearningFixtures.response_result()))
	for i: int in 10:
		await get_tree().process_frame
	return host


## F097: a hint revealed by Mây.
func _screen_activity_hint() -> Node:
	var host: Control = await _activity_host()
	await host.call("_on_hint_requested")
	for i: int in 10:
		await get_tree().process_frame
	return host


func _one_type(type: String) -> Node:
	return await _activity_host(PackedStringArray([type]))


func _screen_activity_multiple() -> Node:
	var host: Control = await _one_type("multiple_choice")
	await _select(host, 0)
	await _select(host, 2)
	return host


func _screen_activity_true_false() -> Node:
	return await _one_type("true_false")


func _screen_activity_fill_blank() -> Node:
	return await _one_type("fill_in_blank")


func _screen_activity_numeric() -> Node:
	return await _one_type("numeric")


func _screen_activity_matching() -> Node:
	return await _one_type("matching")


func _screen_activity_ordering() -> Node:
	return await _one_type("ordering")


func _screen_activity_drag_drop() -> Node:
	return await _one_type("drag_drop")


# ---------- F069: progression, economy, achievements, solo challenge ----------

func _screen_hub_progress() -> Node:
	var world: Node = await _world()
	for i: int in 20:
		await get_tree().process_frame
	return world


func _screen_game_menu() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"game_menu")
	return world


func _level_up(from_level: int, to_level: int) -> Node:
	var world: Node = await _world()
	for i: int in 10:
		await get_tree().process_frame
	var service: ProgressionService = AppState.get_services().progression
	_harness.http.clear(F069Fixtures.ROUTE_PROGRESSION)
	_harness.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(to_level, 20, 140))
	for level: int in range(from_level, to_level):
		service.handle_message(ProgressionService.MSG_LEVEL_REACHED, F069Fixtures.level_up(level, level + 1, level == from_level))
	for i: int in 20:
		await get_tree().process_frame
	var modal: Control = world.find_child("LevelUpModal", true, false) as Control
	if modal != null:
		modal.call("skip_effect")
	return world


func _screen_level_up() -> Node:
	return await _level_up(3, 4)


func _screen_level_up_multi() -> Node:
	return await _level_up(3, 5)


func _screen_journey() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"journey")
	return world


func _inventory(tab: String, select: int = -1) -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"inventory", {"tab": tab})
	var screen: Control = world.find_child("InventoryShopScreen", true, false) as Control
	if screen != null and select >= 0:
		screen.call("select_entry", select)
		for i: int in 10:
			await get_tree().process_frame
	return world


func _screen_inventory() -> Node:
	return await _inventory("wardrobe", 1)


func _screen_shop() -> Node:
	return await _inventory("shop", 0)


func _screen_shop_confirm() -> Node:
	var world: Node = await _inventory("shop", 0)
	var screen: Control = world.find_child("InventoryShopScreen", true, false) as Control
	screen.call("ask_purchase")
	for i: int in 10:
		await get_tree().process_frame
	return world


func _screen_shop_need_more() -> Node:
	return await _inventory("shop", 1)


func _screen_history() -> Node:
	return await _inventory("history")


func _achievements(tab: String) -> Node:
	var world: Node = await _world()
	if tab == "showcase":
		_harness.http.clear(F069Fixtures.ROUTE_INVENTORY)
		_harness.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.badges())
	await _open_panel(world, &"achievements", {"tab": tab})
	return world


func _screen_achievements() -> Node:
	return await _achievements("achievements")


func _screen_collection() -> Node:
	var world: Node = await _achievements("collections")
	var screen: Control = world.find_child("AchievementCollectionScreen", true, false) as Control
	await screen.call("open_collection", F069Fixtures.COLLECTION_ID)
	for i: int in 10:
		await get_tree().process_frame
	return world


func _screen_showcase() -> Node:
	var world: Node = await _achievements("showcase")
	var screen: Control = world.find_child("AchievementCollectionScreen", true, false) as Control
	screen.call("toggle_badge", "badge_explorer")
	for i: int in 5:
		await get_tree().process_frame
	return world


func _screen_challenge_board() -> Node:
	var world: Node = await _world()
	await _open_panel(world, &"challenge_board")
	return world


func _screen_challenge_locked() -> Node:
	var world: Node = await _world()
	_harness.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	_harness.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail(_band, 3, "", true),
			{"Date": F069Fixtures.SERVER_DATE})
	await _open_panel(world, &"challenge_board")
	return world


func _screen_challenge_no_tries() -> Node:
	var world: Node = await _world()
	_harness.http.clear(F069Fixtures.ROUTE_CHALLENGES)
	_harness.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	_harness.http.reply(F069Fixtures.ROUTE_CHALLENGES, 200, F069Fixtures.challenges(_band, 0))
	_harness.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail(_band, 0, "2026-11-26T07:42:00Z"),
			{"Date": F069Fixtures.SERVER_DATE})
	await _open_panel(world, &"challenge_board")
	return world


func _run_challenge(world: Node) -> ChallengeClient:
	var client: ChallengeClient = AppState.get_services().challenge
	client.intent_sink = func(_screen: StringName, _args: Dictionary) -> void: pass
	await client.list()
	await client.get_challenge(F069Fixtures.CHALLENGE_TIME)
	_harness.http.reply(F069Fixtures.ROUTE_START, 201, F069Fixtures.attempt("in_progress", 2, 2),
			{"Date": F069Fixtures.SERVER_DATE})
	_harness.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_1, 2),
			{"Date": F069Fixtures.SERVER_DATE})
	await client.start(F069Fixtures.CHALLENGE_TIME, "hard")
	for i: int in 10:
		await get_tree().process_frame
	return client


func _screen_challenge_hud() -> Node:
	var world: Node = await _world()
	await _run_challenge(world)
	return world


func _challenge_result(status: String) -> Node:
	var world: Node = await _world()
	var client: ChallengeClient = await _run_challenge(world)
	client.intent_sink = Callable()
	var rewards: Array = [{"kind": "xp", "amount": 60}, {"kind": "currency", "amount": 30}] if status == "succeeded" \
			else [{"kind": "xp", "amount": 10}]
	client.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID, "status": status,
		"activities_completed": 6 if status == "succeeded" else 4, "rewards": rewards})
	# The activity player of the run closes: the held result opens (ChallengeClient.on_activity_closed).
	client.on_activity_closed()
	for i: int in 40:
		await get_tree().process_frame
	return world


func _screen_challenge_result_failed() -> Node:
	return await _challenge_result("failed")


func _screen_challenge_result_succeeded() -> Node:
	return await _challenge_result("succeeded")


# ---------- F098: tap-to-answer ----------

## F098: the tapped tile while the server grades ("Đang chấm…", the other tiles locked). The fake server never
## answers during the capture.
func _screen_activity_tap_grading() -> Node:
	var host: Control = await _activity_host()
	_harness.http.delay_frames = 1000000
	await _select(host, 1)
	for i: int in 5:
		await get_tree().process_frame
	return host


## F098: one tap on the right answer: green tile with a tick and "Đúng rồi!", Mây happy, "Tiếp".
func _screen_activity_tap_correct() -> Node:
	var host: Control = await _activity_host()
	await _select(host, 1)
	for i: int in 20:
		await get_tree().process_frame
	return host


## F098: one tap on a wrong answer: warm tile with a cross and kind words; the right answer is marked because the
## (fake) server sent `correct_response`; the option feedback and explanation in Mây's bubble.
func _screen_activity_tap_incorrect() -> Node:
	var host: Control = await _activity_host()
	var result: Dictionary = LearningFixtures.response_result()
	result["correct_response"] = {"option_id": "o_b"}
	_harness.http.clear(LearningFixtures.ROUTE_RESPOND)
	_harness.http.reply(LearningFixtures.ROUTE_RESPOND, 200, result)
	await _select(host, 0)
	for i: int in 20:
		await get_tree().process_frame
	return host


## F098: a band set to confirm with a second tap, after the first tap (tick + Mây "Chạm lần nữa…").
func _screen_activity_tap_confirm() -> Node:
	ProjectSettings.set_setting(InstantAnswerPolicy.SETTING, PackedStringArray([_band]))
	var host: Control = await _activity_host()
	await _select(host, 1)
	for i: int in 5:
		await get_tree().process_frame
	return host


# ---------- F099: media players ----------

## The activity player on a media attempt (`video_lesson`, `audio_lesson`, `simulation`). TEST-ONLY: the media files
## are placeholders (no decoding) and the playback is swapped for the scripted fakes of res://tests/fakes.
func _media_host(attempt: Dictionary) -> Control:
	await _sign_in()
	_harness.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/vi.vtt*", 200,
			VideoFixtures.VTT_VI.to_utf8_buffer())
	_harness.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/en.vtt*", 200,
			VideoFixtures.VTT_EN.to_utf8_buffer())
	_harness.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/transcript.txt*", 200,
			AudioFixtures.TRANSCRIPT.to_utf8_buffer())
	AppState.services.video_files.downloader = VideoFixtures.fake_download
	var host: Control = (load("res://scenes/learning/ActivityHost.tscn") as PackedScene).instantiate() as Control
	host.set("services", AppState.services)
	host.set("client_version", AuthHarness.VERSION)
	host.set("grade_band", _band)
	get_tree().root.add_child(host)
	_harness.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	_harness.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 60:
		await get_tree().process_frame
	return host


## Swaps the renderer playback for a scripted fake at `at_ms` and renders one step (the fake does not advance).
func _fake_video(host: Control, at_ms: int, playing: bool) -> VideoLessonRenderer:
	var r: VideoLessonRenderer = host.get("renderer") as VideoLessonRenderer
	if r == null:
		return null
	var fake: FakeVideoPlayback = FakeVideoPlayback.new()
	fake.playback_finished.connect(r._on_finished)
	r.playback.pause()
	r.playback = fake
	fake.pos = at_ms
	fake.playing = true
	r.reporter.ranges.load_from([{"start_ms": 0, "end_ms": maxi(at_ms, 45000)}])
	r._process(0.016)
	fake.playing = playing and not (r.get_node("%Overlay") as Control).visible
	if r.has_method("_refresh_controls"):
		r.call("_refresh_controls")
	return r


## F099: a video lesson playing, with the caption of the teacher and the chapter / question marks on the bar.
func _screen_activity_video() -> Node:
	var host: Control = await _media_host(VideoFixtures.attempt())
	_fake_video(host, 6000, true)
	for i: int in 10:
		await get_tree().process_frame
	return host


## F099: the video stopped at its question (question card + answer tiles of F097 on the board).
func _screen_activity_video_cue() -> Node:
	var host: Control = await _media_host(VideoFixtures.attempt())
	_fake_video(host, 30000, true)
	for i: int in 20:
		await get_tree().process_frame
	return host


## F099: an audio lesson paused in its second play (plays left, the question mark on the bar, the coach hint).
func _screen_activity_audio() -> Node:
	var host: Control = await _media_host(AudioFixtures.attempt(2, 1))
	var r: AudioLessonRenderer = host.get("renderer") as AudioLessonRenderer
	if r != null:
		var fake: FakeAudioPlayback = FakeAudioPlayback.new()
		fake.playback_finished.connect(r._on_finished)
		r.playback.pause()
		r.playback = fake
		fake.pos = 12000
		# The placeholder file cannot be decoded: drop its "not ready" toast (the fake plays instead).
		(r.get_node("%Toast") as Control).visible = false
		r._refresh()
	for i: int in 10:
		await get_tree().process_frame
	return host


## F099: a simulation with its goals, numbered steps and the release / redo buttons.
func _screen_activity_simulation() -> Node:
	var host: Control = await _media_host(SimulationFixtures.attempt())
	for i: int in 20:
		await get_tree().process_frame
	return host


## F113: the rotate prompt of OrientationGuard over the hub HUD of a signed-in student (the guard reacts to the window
## size by itself; capture with --sizes=390x844).
func _screen_rotate_prompt() -> Node:
	return await _world()
