class_name ServiceLocator
extends RefCounted
## Domain services of the signed-in session (GODOT_ARCHITECTURE §4: `RefCounted` services created by the
## ServiceLocator held in AppState after bootstrap, receiving ApiClient through their constructor so tests inject
## fakes). UI and gameplay scripts reach them through `AppState.services`, never ApiClient directly (G-4, R1).

var assignments: AssignmentService = null
var play_time: PlayTimeService = null
var telemetry: TelemetryService = null
## Activity player (F063): attempts, formula SVG cache and item media (RAM).
var learning: LearningService = null
var math_svg: MathSvgCache = null
var media: MediaLoader = null
var video_files: VideoFileCache = null
## F133 T037/T038: public zone packs (preinstalled → LAN → CDN, SHA-256, LRU) and the off-hours prefetch.
var content: ContentDownloader = null
var prefetch: PrefetchScheduler = null
## Timed exams of the Exam Zone (F064): sessions, attempt, server countdown, answers queued in RAM.
var exam: ExamService = null
## In-game notifications (F123): REST inbox + realtime `notification.*`, bell badge, toasts (and the F064 grouped
## "pending school work" toast of the first assignments refresh).
var notifications: NotificationService = null
## Player profile, nickname, avatar and saved outfits (F070); character catalog cache + ownership view (F070/F075).
var profile: ProfileService = null
var inventory: InventoryService = null
## Hub multiplayer and own presence over the realtime connection (F092).
var realtime: GameRealtimeClient = null
var hub: HubSync = null
var presence: PresenceService = null
## The student's party (F102): REST + realtime `party.*`, PartyStore read by the lobby, the HUD and the invitation toast.
var party: PartyService = null
## Gameplay chat (F119): REST + realtime `chat.*`, ChatStore read by the ChatDock of the party lobby / match screens;
## block and mute (SocialService, shared with the friends UI) and reports (ReportService, MODERATION §5).
var chat: ChatService = null
var social: SocialService = null
var reports: ReportService = null
## Competition arena (F109): modes, queue, ready check, duels (MatchmakingService); the match on screen (BattleClient);
## boards + tiers (LeaderboardService); tournaments + grade challenges (TournamentService); match history (MatchHistoryService).
var matchmaking: MatchmakingService = null
var battle: BattleClient = null
var leaderboards: LeaderboardService = null
var tournaments: TournamentService = null
var match_history: MatchHistoryService = null
## World of the student: areas, exploration, chapters, level sessions, discoveries, NPC dialogues (F071).
var world: WorldService = null
## Quests of the student: list, delivery, accept/abandon, step reports, learning steps, realtime progress (F072).
var quest: QuestService = null
## F069: progression (XP, level, S07 queue), achievements + collections + badge showcase, solo challenges.
var progression: ProgressionService = null
var achievements: AchievementService = null
var challenge: ChallengeClient = null
## F110: boss battles (solo + party, `boss.*`), party quests (shared progress, roles, clue cards); friends live in
## `social` (SocialService + FriendsStore) and their presence comes through `presence`.
var boss: BossClient = null
var coop_quest: CoopQuestService = null
## F111: "Tham gia bạn" (HubSync + SceneRouter.request_area), clubs (REST `/social/clubs*`), the Event center (live-ops
## events `/game/events*` + realtime `event.*`) and the free season track (`/game/season-track*`, `season.track.updated`).
var join_friend: JoinFriendService = null
var clubs: ClubService = null
var live_events: LiveEventService = null
var season: SeasonTrackService = null
## `(code: String) -> void` for session-ending refusals; default `AuthService.handle_access_denied` (F062-AC-12/23).
var access_denied_handler: Callable = Callable()


func _init(rest: RestApiClient = null, app_state: Node = null, clock: ClientClock = null,
		realtime_client: GameRealtimeClient = null) -> void:
	telemetry = TelemetryService.new(rest, clock)
	assignments = AssignmentService.new(rest, app_state, telemetry)
	play_time = PlayTimeService.new()
	play_time.break_reminder_shown.connect(_on_break_reminder_shown)
	assignments.access_denied.connect(_on_access_denied)
	learning = LearningService.new(rest, telemetry, clock)
	learning.access_denied.connect(_on_access_denied)
	exam = ExamService.new(rest, clock)
	exam.access_denied.connect(_on_access_denied)
	var transport: HttpTransport = rest.transport if rest != null else null
	math_svg = MathSvgCache.new(transport, _cdn_base_url())
	math_svg.failure_handler = func(_reason: String) -> void: learning.report_math_failed()
	media = MediaLoader.new(transport)
	video_files = VideoFileCache.new(rest)
	video_files.telemetry = telemetry
	_init_content(rest)
	profile = ProfileService.new(rest, app_state)
	profile.access_denied.connect(_on_access_denied)
	realtime = realtime_client if realtime_client != null else _realtime_autoload()
	inventory = InventoryService.new(rest, realtime, telemetry)
	inventory.profile = profile
	inventory.access_denied.connect(_on_access_denied)
	progression = ProgressionService.new(rest, app_state, realtime, telemetry)
	progression.access_denied.connect(_on_access_denied)
	progression.level_reached.connect(func(_level: int) -> void: inventory.load_wallet())
	achievements = AchievementService.new(rest, profile, telemetry)
	achievements.access_denied.connect(_on_access_denied)
	challenge = ChallengeClient.new(rest, app_state, clock, realtime, learning, telemetry)
	challenge.access_denied.connect(_on_access_denied)
	challenge.challenge_finished.connect(func(_a: ChallengeAttemptModel) -> void: inventory.load_wallet())
	hub = HubSync.new(realtime)
	presence = PresenceService.new(realtime, clock)
	world = WorldService.new(rest, app_state, realtime)
	world.access_denied.connect(_on_access_denied)
	quest = QuestService.new(rest, app_state, realtime)
	quest.access_denied.connect(_on_access_denied)
	notifications = NotificationService.new(rest, realtime, _autoload("Settings"), telemetry)
	notifications.bind(assignments)
	notifications.access_denied.connect(_on_access_denied)
	# F123-AC-16: badge only (no toast) during a timed exam or a solo challenge.
	notifications.quiet_check = func() -> bool: return exam.in_progress() or challenge.is_running() or boss_running()
	# F102: the party invitation popup is the InvitationToast (accept / decline in place); the inbox keeps the row.
	notifications.delegated_popup_kinds = PackedStringArray(["party_invitation"])
	party = PartyService.new(rest, realtime, app_state, _autoload("Settings"), telemetry)
	party.access_denied.connect(_on_access_denied)
	# F102 edge case: an invitation during an exam, a challenge or the activity player only raises the bell count.
	# F110 edge case: an invitation during a boss phase only raises the bell count too.
	party.quiet_check = func() -> bool: return exam.in_progress() or challenge.is_running() or boss_running() or presence.activity_status == PresenceService.IN_ACTIVITY
	chat = ChatService.new(rest, realtime, app_state, telemetry, _cdn_base_url())
	chat.access_denied.connect(_on_access_denied)
	social = SocialService.new(rest, telemetry, realtime)
	social.access_denied.connect(_on_access_denied)
	social.party = party
	social.now_source = party.now_ms
	presence.friend_presence_received.connect(func(user_id: String, status: String, activity: String, area_id: String) -> void:
		social.store.apply_presence(user_id, status, activity, area_id))
	# F119-AC-15: once the server confirmed a block / mute, the player's messages leave the chat at once.
	social.relation_changed.connect(func(user_id: String, _kind: String) -> void: chat.store.hide_user(user_id))
	reports = ReportService.new(rest, telemetry)
	reports.access_denied.connect(_on_access_denied)
	boss = BossClient.new(rest, app_state, clock, realtime, telemetry)
	boss.access_denied.connect(_on_access_denied)
	coop_quest = CoopQuestService.new(rest, realtime, quest, app_state, telemetry)
	coop_quest.access_denied.connect(_on_access_denied)
	coop_quest.party_store = party.store
	_init_f111(rest)
	matchmaking = MatchmakingService.new(rest, realtime, app_state, telemetry)
	matchmaking.access_denied.connect(_on_access_denied)
	battle = BattleClient.new(realtime, app_state)
	# F109: a friend's duel prompt never pops during an exam, a challenge, the activity player or a match, nor with the
	# popups off (the same rule as the party invitation popup) — the duel then ends by itself after 60 s.
	matchmaking.quiet_check = func() -> bool: return not party.should_popup(null) or not battle.match_id.is_empty()
	leaderboards = LeaderboardService.new(rest, realtime, telemetry)
	leaderboards.access_denied.connect(_on_access_denied)
	tournaments = TournamentService.new(rest, realtime, telemetry)
	tournaments.access_denied.connect(_on_access_denied)
	match_history = MatchHistoryService.new(rest, reports, social, telemetry)
	match_history.access_denied.connect(_on_access_denied)
	if realtime != null:
		realtime.resync_required.connect(resync)
		if app_state != null:
			app_state.set("net_stats", realtime.stats)


## Applies the bootstrap of a new session (policies, product telemetry switch) and drops the previous data.
func start_session(bootstrap: Dictionary) -> void:
	assignments.clear()
	notifications.reset()
	party.clear()
	chat.clear()
	reports.clear()
	_clear_f110()
	_clear_competition()
	play_time.configure(bootstrap.get("policies", {}) as Dictionary)
	var flags: Variant = bootstrap.get("feature_flags", {})
	# CONFIG_DEFAULTS `analytics_product_telemetry_enabled` (default on) as published in the bootstrap feature flags.
	telemetry.product_telemetry_enabled = not (flags is Dictionary
			and (flags as Dictionary).get("product_telemetry_enabled") is bool
			and not bool((flags as Dictionary)["product_telemetry_enabled"]))


## Session ended (logout, expiry, tenant change): drops the list and the unsent telemetry so nothing of this student
## is ever sent with the next student's token or tenant.
func end_session() -> void:
	assignments.clear()
	notifications.reset()
	party.clear()
	chat.clear()
	reports.clear()
	_clear_f110()
	_clear_competition()
	learning.clear()
	exam.clear()
	media.clear()
	profile.clear()
	world.clear()
	inventory.clear()
	progression.clear()
	achievements.clear()
	challenge.clear()
	play_time.configure({})
	telemetry.clear()
	hub.leave_area()


## F092-AC-05: the resume failed — read the profile again over REST, join the current area again, then Online.
func resync() -> void:
	await profile.fetch_profile()
	progression.refresh()
	inventory.load_wallet()
	notifications.load_unread()
	party.resync()
	chat.resync()
	if challenge.is_running():
		challenge.resume(challenge.attempt().challenge_attempt_id)
	boss.resync()
	if not hub.desired_area.is_empty():
		await hub.join()
	if realtime != null:
		realtime.finish_resync()


## F110: a boss encounter of this student is on (Lobby … Result): popups wait (bell count only, spec edge case).
func boss_running() -> bool:
	return boss != null and boss.store.encounter != null and not boss.store.encounter.is_over()


## F110-AC-29: nothing of the previous student's boss, party quest, friends, requests, code or hidden switch is kept.
## F111-AC-30: nor the clubs, the live-ops events, the season track or a "Tham gia bạn" on its way.
func _clear_f110() -> void:
	boss.clear()
	coop_quest.clear()
	social.clear()
	presence.clear()
	join_friend.clear()
	clubs.clear()
	live_events.clear()
	season.clear()


## F111 services (GODOT_ARCHITECTURE §4): built with the ApiClient / RealtimeClient; session refusals go to F061.
func _init_f111(rest: RestApiClient) -> void:
	join_friend = JoinFriendService.new(hub, social)
	clubs = ClubService.new(rest, telemetry)
	clubs.access_denied.connect(_on_access_denied)
	live_events = LiveEventService.new(rest, realtime, telemetry)
	live_events.access_denied.connect(_on_access_denied)
	season = SeasonTrackService.new(rest, realtime, telemetry)
	season.access_denied.connect(_on_access_denied)


## F109: nothing of the previous student's queue, match, boards or tournaments is kept.
func _clear_competition() -> void:
	matchmaking.clear()
	battle.clear()
	leaderboards.clear()
	tournaments.clear()
	match_history.clear()


static func _realtime_autoload() -> GameRealtimeClient:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null("RealtimeClient") as GameRealtimeClient if tree != null else null


static func _autoload(node_name: String) -> Node:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null(node_name) if tree != null else null


## F133: ContentDownloader + PrefetchScheduler from Config (device.cfg, bundles CDN, release public key). The prefetch
## timer only runs on shared Android devices (PrefetchScheduler.attach).
func _init_content(rest: RestApiClient) -> void:
	var config: Node = _autoload("Config")
	var device: DeviceConfig = config.get("device") as DeviceConfig if config != null else null
	content = ContentDownloader.new(device, telemetry, rest)
	content.probe = BandwidthProbe.new()
	prefetch = PrefetchScheduler.new(content.device, content, telemetry)
	if config != null:
		prefetch.client_version = String(config.get("client_version"))
		prefetch.bundles_base_url = String(config.get("bundles_base_url"))
		prefetch.public_key = String(config.get("manifest_public_key"))
	prefetch.attach(rest)


static func _cdn_base_url() -> String:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var config: Node = tree.root.get_node_or_null("Config") if tree != null else null
	return String(config.get("cdn_base_url")) if config != null else ""


func _on_break_reminder_shown(reminder_no: int, minutes_played: int) -> void:
	telemetry.queue_break_reminder(reminder_no, minutes_played)


func _on_access_denied(code: String) -> void:
	if access_denied_handler.is_valid():
		access_denied_handler.call(code)
		return
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var auth: Node = tree.root.get_node_or_null("AuthService") if tree != null else null
	if auth != null:
		auth.call("handle_access_denied", code)
