extends GdUnitTestSuite
## F092-AC-21, F092-AC-22 (CI part), F092-AC-23 (CI part) — T024 (NFR-COMP-001, PERFORMANCE_BUDGET §5, §10, §13;
## TC-PERF-018): 40 simulated players in one hub instance (39 others + the local one). The closest `quality_presets`
## players are drawn in full (10 / 20, PC 30 / 40), the rest as impostors; players beyond 30 m animate at a lower rate;
## the script time of the hub sync + interpolation per frame stays inside "game logic + script ≤ 6 ms"; the uplink of
## one client stays inside `hub_sync_bandwidth` (≤ 3 KB/s); players coming and going reuse the pool. FPS p10, the
## downlink and the 60-minute memory soak need the lab devices (lab_hub40_protocol.md).

## PERFORMANCE_BUDGET §2: game logic + script ≤ 6 ms per frame on a mid-range mobile.
const SCRIPT_BUDGET_MS: float = 6.0
## CONFIG_DEFAULTS `hub_sync_bandwidth`: ≤ 3 KB/s up.
const UPLINK_BUDGET_BYTES_PER_SEC: float = 3072.0
const OTHERS: int = 39

var h: RealtimeHarness = null
var hub: HubSync = null
var sync: RemotePlayerSync = null
var viewer: Node3D = null


func before_test() -> void:
	h = RealtimeHarness.create()
	hub = HubSync.new(h.client)
	sync = auto_free(RemotePlayerSync.new()) as RemotePlayerSync
	add_child(sync)
	viewer = auto_free(Node3D.new()) as Node3D
	add_child(viewer)
	sync.viewer = viewer
	sync.bind(hub)


func after_test() -> void:
	sync.unbind()
	hub.dispose()
	h.dispose()


## Instance with 39 others spread 2–60 m from the viewer.
func _fill() -> void:
	await h.connect_online()
	hub.enter_area("main_hub")
	h.ack_last("hub.instance.join")
	var assigned: Dictionary = RealtimeHarness.fixture("hub.instance.assigned.main_hub")
	var template: Dictionary = ((assigned["payload"] as Dictionary)["players"] as Array)[0] as Dictionary
	var players: Array = []
	for i: int in OTHERS:
		var entry: Dictionary = template.duplicate(true)
		entry["eid"] = 100 + i
		(entry["player"] as Dictionary)["user_id"] = "7d1e2f3a-4b5c-4d6e-9f0a-%012d" % i
		(entry["player"] as Dictionary)["nickname"] = "Ban%02d" % i
		var d: float = 2.0 + float(i) * 1.5
		entry["state"] = {"eid": 100 + i, "p": [d, 0, 0], "r": 0.0, "a": "walk"}
		players.append(entry)
	(assigned["payload"] as Dictionary)["players"] = players
	h.next_seq += 1
	assigned["seq"] = h.next_seq
	h.push(assigned)


func _snapshot(tick: int) -> void:
	var entities: Array = []
	for i: int in OTHERS:
		var d: float = 2.0 + float(i) * 1.5
		entities.append({"eid": 100 + i, "p": [d + sin(float(tick) * 0.1), 0, float(tick % 10) * 0.1], "r": 0.1 * tick, "a": "run"})
	h.next_seq += 1
	h.peer().push({"v": 1, "type": "hub.snapshot.updated", "seq": h.next_seq, "ts": h.client.server_now_ms(),
			"payload": {"room_id": "hub:main_hub:01", "tick": tick, "entities": entities}})


func test_full_characters_follow_the_quality_preset() -> void:
	await _fill()
	assert_int(sync.shown_count()).is_equal(OTHERS)
	for case: Array in [["low", false, 10], ["medium", false, 20], ["medium", true, 30], ["high", true, 39]]:
		sync.quality_preset = String(case[0])
		sync.is_pc = bool(case[1])
		sync.update_lod()
		var full: int = 0
		var impostors: int = 0
		for rp: Variant in sync.active.values():
			match (rp as RemotePlayer).lod:
				RemotePlayer.Lod.IMPOSTOR:
					impostors += 1
				_:
					full += 1
		assert_int(full).override_failure_message(str(case)).is_equal(int(case[2]))
		assert_int(impostors).is_equal(OTHERS - int(case[2]))
	# Closest are full; beyond 30 m the animation is reduced; impostors hide the avatar.
	sync.quality_preset = "high"
	sync.update_lod()
	assert_int((sync.player_for(100) as RemotePlayer).lod).is_equal(RemotePlayer.Lod.FULL)
	assert_int((sync.player_for(100 + OTHERS - 1) as RemotePlayer).lod).is_equal(RemotePlayer.Lod.FAR)
	sync.quality_preset = "low"
	sync.update_lod()
	var far_one: RemotePlayer = sync.player_for(100 + OTHERS - 1)
	assert_int(far_one.lod).is_equal(RemotePlayer.Lod.IMPOSTOR)
	assert_bool(far_one.avatar.visible).is_false()
	assert_bool(far_one.impostor.visible).is_true()


func test_script_time_per_frame_with_40_players() -> void:
	await _fill()
	sync.quality_preset = "medium"
	sync.is_pc = false
	var times: Array[float] = []
	for frame: int in 180:
		var started: int = Time.get_ticks_usec()
		if frame % 6 == 0:
			_snapshot(frame / 6)
		h.client.tick()
		hub.update_local(Vector3(float(frame) * 0.05, 0, 0), 0.0, "run", 1_000_000 + frame * 16)
		sync._process(1.0 / 60.0)
		times.append(float(Time.get_ticks_usec() - started) / 1000.0)
		h.clock.advance(1.0 / 60.0)
	times.sort()
	var median: float = times[times.size() / 2]
	prints("hub40 script ms: median %.3f p90 %.3f" % [median, times[int(times.size() * 0.9)]])
	assert_float(median).is_less_equal(SCRIPT_BUDGET_MS)


func test_uplink_stays_inside_the_bandwidth_budget() -> void:
	await _fill()
	var before: int = h.client.stats.bytes_out
	# 10 s of running at 60 FPS (worst case: always moving and turning).
	for frame: int in 600:
		hub.update_local(Vector3(float(frame) * 0.09, 0, float(frame % 50) * 0.05), float(frame) * 0.05, "run",
				2_000_000 + frame * 16)
		if frame % 60 == 0:
			h.clock.advance(1.0)
			h.client.tick()
	var per_sec: float = float(h.client.stats.bytes_out - before) / 9.6
	prints("hub40 uplink bytes/s:", per_sec)
	assert_float(per_sec).is_less_equal(UPLINK_BUDGET_BYTES_PER_SEC)


func test_pool_is_reused_while_players_come_and_go() -> void:
	await _fill()
	var created: int = sync.created
	for round_no: int in 200:
		var eid: int = 100 + (round_no % OTHERS)
		h.push(h.envelope("hub.player.left", {"room_id": "hub:main_hub:01", "eid": eid}))
		(sync.player_for(eid) as RemotePlayer)._process(0.4)
		var j: Dictionary = RealtimeHarness.fixture("hub.player.joined.joined")
		(j["payload"] as Dictionary)["eid"] = eid
		h.next_seq += 1
		j["seq"] = h.next_seq
		h.push(j)
	assert_int(sync.created).is_equal(created)
	assert_int(sync.created).is_less_equal(RemotePlayerSync.CAPACITY)
	assert_int(sync.shown_count()).is_equal(OTHERS)
