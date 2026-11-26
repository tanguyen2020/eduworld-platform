extends GdUnitTestSuite
## T026 (F119-AC-21, TC-PERF-009; PERFORMANCE_BUDGET `ui_cpu_budget`, STUDENT_GAME_UX_SOCIAL §7 "Hiệu năng máy tầm trung"):
## 500 messages into one channel keep at most `chat_recent_buffer` in memory, the virtualised list holds only the
## bubbles in view (a pool far below the message count), a new message costs one bubble bind within the UI update
## budget, and typing / receipt redraws are coalesced to ≤ 1 per second. The FPS on the reference mid-range device is a
## lab measurement recorded in the quickstart when it is run.

const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")
const MESSAGES: int = 500

var s: ChatSceneHarness = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	# After the sign-in (the bootstrap sets the band of the sample student).
	AppState.set_grade_band("g4_5")


func after_test() -> void:
	s.dispose()


func _created(seq: int) -> Dictionary:
	var p: Dictionary = ChatFixtures.ws("chat.message.created.text")
	p["message_id"] = Uuid.v4()
	p["seq"] = seq
	p["body"] = "Tin số %d của nhóm mình" % seq
	return p


func test_five_hundred_messages_stay_within_the_buffer_and_the_pool() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self, Vector2(560, 520))) as ChatDock
	for i: int in range(16, 16 + MESSAGES):
		s.push("chat.message.created", _created(i))
	await s.settle(dock, 6)
	assert_int(s.chat.store.count(ChatFixtures.PARTY_CHANNEL)).is_equal(ChatStore.RECENT_BUFFER)
	assert_int(dock.list().entries.size()).is_equal(ChatStore.RECENT_BUFFER)
	var pool: int = dock.list().pool_size()
	assert_int(pool).is_less(30)
	var started: int = Time.get_ticks_usec()
	s.push("chat.message.created", _created(16 + MESSAGES))
	var update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0
	await s.settle(dock, 2)
	# Scrolling to the oldest messages re-binds bubbles of the same pool.
	dock.list().scroll_vertical = 0
	await s.settle(dock, 3)
	assert_int(dock.list().pool_size()).is_less(30)
	prints("chat_list budget:", JSON.stringify({"messages": MESSAGES, "kept": s.chat.store.count(ChatFixtures.PARTY_CHANNEL),
		"pool": pool, "new_message_ms": update_ms}))
	assert_float(update_ms).is_less_equal(BenchmarkScript.BUDGET_HUD_MS * BenchmarkScript.TOLERANCE * 4.0)


func test_typing_redraws_are_coalesced_to_once_a_second() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	await s.settle(dock, 70)
	for i: int in 20:
		s.push("chat.typing.updated", {"channel_id": ChatFixtures.PARTY_CHANNEL, "typing": [
			{"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07%d" % i}]})
	assert_str(dock.typing_text()).is_empty()
	dock._process(ChatDock.COALESCE_SEC + 0.01)
	assert_str(dock.typing_text()).contains("CaHeoVui0719")
