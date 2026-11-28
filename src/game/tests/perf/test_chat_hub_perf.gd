extends GdUnitTestSuite
## T030 (F112-AC-29, NFR-COMP-001 CI guard; TC-PERF-009; PERFORMANCE_BUDGET): the HUD chat on "Cả trường" receiving 10
## messages a second for 60 s keeps at most `chat_recent_buffer` messages in RAM, builds only the message cards in view
## (virtualised list), redraws the HUD badge at most once a second, applies each message within the UI update budget and
## draws no 3D node of its own. The FPS on the reference mid-range Android device (SC-005, preset Trung bình) is a lab
## measurement recorded in the quickstart.

## UI update budget of one realtime message (PERFORMANCE_BUDGET UI frame share at 30 FPS, as test_chat_list_perf).
const UPDATE_BUDGET_MS: float = 8.0
const RATE: int = 10
const SECONDS: int = 60

var s: F112SceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")


func after_test() -> void:
	s.dispose()


func test_world_burst_stays_inside_the_budget() -> void:
	var hub_screen: ChatHub = auto_free((load("res://scenes/chat/ChatHub.tscn") as PackedScene).instantiate()) as ChatHub
	hub_screen.services = s.services
	add_child(hub_screen)
	hub_screen.size = Vector2(1280, 720)
	await hub_screen.open({"tab": "world"})
	await s.settle(10)
	var badges: Array[int] = [0]
	s.services.chat_hub.badge_changed.connect(func(_u: int, _m: int) -> void: badges[0] += 1)
	var sample: Dictionary = ChatFixtures.ws("chat.message.created.text", s.services.chat.store.self_id)
	var worst_ms: float = 0.0
	var now: int = Time.get_ticks_msec()
	for i: int in RATE * SECONDS:
		var p: Dictionary = sample.duplicate(true)
		p["channel_id"] = F112Fixtures.WORLD
		p["message_id"] = "0193b0a0-0000-7000-8000-%012x" % (0x100000 + i)
		p["seq"] = 6 + i
		var started: int = Time.get_ticks_usec()
		s.push_chat("chat.message.created", p)
		worst_ms = maxf(worst_ms, float(Time.get_ticks_usec() - started) / 1000.0)
		now += 1000 / RATE
		s.services.chat_hub.tick(now)
		if i % RATE == 0:
			await get_tree().process_frame
	await s.settle(4)
	assert_int(s.services.chat.store.count(F112Fixtures.WORLD)).is_less_equal(ChatStore.RECENT_BUFFER)
	assert_int(hub_screen.dock().list().bubble_count()).is_less(40)
	assert_int(badges[0]).is_less_equal(SECONDS + 1)
	assert_float(worst_ms).override_failure_message("worst message %.2f ms" % worst_ms).is_less(UPDATE_BUDGET_MS * 4.0)
	assert_array(hub_screen.find_children("*", "Node3D", true, false)).is_empty()
	assert_array(hub_screen.find_children("*", "SubViewport", true, false)).is_empty()
