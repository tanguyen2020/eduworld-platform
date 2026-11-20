extends GdUnitTestSuite
## F092-AC-10, AC-12, AC-13, AC-15, AC-16, AC-17, AC-19, AC-23 (T018; GAME_NETWORKING §5.2, AGENTS R16): the players of
## `hub.instance.assigned`/`hub.player.joined` appear as pooled RemotePlayers built from `avatar_config` with their
## nickname only (a real name, school or class sent by mistake is never shown); snapshots move them through the
## Interpolator; they freeze while reconnecting; `hub.player.left`/`removed` fade them out in 0.3 s and the node goes
## back to the pool; appearance changes rebuild the avatar; emotes play; a migration keeps every node.

var h: RealtimeHarness = null
var hub: HubSync = null
var sync: RemotePlayerSync = null


func before_test() -> void:
	h = RealtimeHarness.create()
	hub = HubSync.new(h.client)
	sync = auto_free(RemotePlayerSync.new()) as RemotePlayerSync
	add_child(sync)
	sync.bind(hub)


func after_test() -> void:
	sync.unbind()
	hub.dispose()
	h.dispose()


func _joined() -> void:
	await h.connect_online()
	hub.enter_area("main_hub")
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")


func _all_texts(root: Node) -> String:
	var out: PackedStringArray = []
	for node: Node in root.find_children("*", "", true, false):
		if node is Label3D:
			out.append((node as Label3D).text)
		elif node is Label:
			out.append((node as Label).text)
	return " | ".join(out)


func test_assigned_players_are_built_with_avatar_and_nickname_only() -> void:
	await _joined()
	var rp: RemotePlayer = sync.player_for(3)
	assert_object(rp).is_not_null()
	assert_str(rp.nickname_label.text).is_equal("CuMeoChamChi12")
	assert_str(rp.avatar.config.hair_item_id).is_equal("hair_bob_01")
	assert_str(rp.avatar.config.skin_tone).is_equal("tone_03")
	assert_bool(rp.position.is_equal_approx(Vector3(2.5, 0, 1.25))).is_true()
	# The local player (self_eid) is never drawn as a remote.
	assert_object(sync.player_for(7)).is_null()


func test_extra_personal_fields_are_never_shown() -> void:
	await h.connect_online()
	hub.enter_area("main_hub")
	h.ack_last("hub.instance.join")
	h.push_fixture("hub.instance.assigned.main_hub")
	var joined: Dictionary = RealtimeHarness.fixture("hub.player.joined.joined")
	var player: Dictionary = (joined["payload"] as Dictionary)["player"] as Dictionary
	player["full_name"] = "Nguyễn Văn Bình"
	player["school_name"] = "TH Demo A"
	player["class_name"] = "3A"
	h.next_seq += 1
	joined["seq"] = h.next_seq
	h.push(joined)
	var texts: String = _all_texts(sync)
	assert_bool(texts.contains("GauTrucVuiVe07")).is_true()
	for secret: String in ["Nguyễn Văn Bình", "TH Demo A", "3A"]:
		assert_bool(texts.contains(secret)).override_failure_message(secret).is_false()
	var model: PublicPlayerModel = hub.room.players[9] as PublicPlayerModel
	assert_bool("full_name" in model.get_property_list().map(func(p: Dictionary) -> String: return String(p["name"]))).is_false()


func test_snapshots_move_players_and_freeze_while_reconnecting() -> void:
	await _joined()
	var rp: RemotePlayer = sync.player_for(3)
	var base: int = h.client.server_now_ms()
	for i: int in 5:
		h.push({"v": 1, "type": "hub.snapshot.updated", "seq": h.next_seq + 1 + i, "ts": base + i * 100,
				"payload": {"room_id": "hub:main_hub:01", "tick": 2000 + i, "entities": [{"eid": 3, "p": [i, 0, 0], "a": "run"}]}})
	h.next_seq += 5
	h.clock.advance(0.5)
	sync._process(0.016)
	assert_float(rp.position.x).is_greater(2.0)
	assert_str(String(rp.anim_driver.state)).is_equal("run")
	var frozen_at: Vector3 = rp.position
	h.peer().server_close(-1)
	h.client.tick()
	h.clock.advance(0.3)
	sync._process(0.016)
	assert_bool(rp.position.is_equal_approx(frozen_at)).is_true()


func test_left_and_removed_fade_out_and_return_to_the_pool() -> void:
	await _joined()
	h.push_fixture("hub.player.joined.joined")
	assert_int(sync.shown_count()).is_equal(2)
	var rp9: RemotePlayer = sync.player_for(9)
	h.push_fixture("hub.player.left.left")
	assert_bool(rp9.fading).is_true()
	rp9._process(0.15)
	assert_bool(rp9.visible).is_true()
	rp9._process(0.2)
	assert_bool(rp9.visible).is_false()
	assert_object(sync.player_for(9)).is_null()
	assert_int(sync.pool.size()).is_equal(1)
	# removed[] of a snapshot does the same.
	h.push(h.envelope("hub.snapshot.updated", {"room_id": "hub:main_hub:01", "tick": 99, "entities": [], "removed": [3]}))
	(sync.player_for(3) as RemotePlayer)._process(0.4)
	assert_int(sync.pool.size()).is_equal(2)
	# Players coming and going reuse the pool: no node beyond the ones created.
	var created: int = sync.created
	for round_no: int in 30:
		var j: Dictionary = RealtimeHarness.fixture("hub.player.joined.joined")
		((j["payload"] as Dictionary))["eid"] = 100 + round_no
		h.next_seq += 1
		j["seq"] = h.next_seq
		h.push(j)
		h.push(h.envelope("hub.player.left", {"room_id": "hub:main_hub:01", "eid": 100 + round_no}))
		(sync.player_for(100 + round_no) as RemotePlayer)._process(0.4)
	assert_int(sync.created).is_equal(created)


func test_appearance_change_rebuilds_the_avatar_and_emotes_play() -> void:
	await _joined()
	var rp: RemotePlayer = sync.player_for(3)
	h.push_fixture("hub.player.appearance_changed.outfit")
	assert_str(rp.avatar.config.outfit_item_id).is_equal("outfit_sport_01")
	assert_str(rp.avatar.config.hair_item_id).is_equal("hair_spiky_01")
	h.push_fixture("hub.player.emoted.wave")
	assert_str(String(rp.anim_driver.state)).is_equal(String(AnimDriver.EMOTE))
	assert_str(rp.anim_driver.emote_id).is_equal("emote_wave")


func test_migration_keeps_the_players() -> void:
	await _joined()
	var rp: RemotePlayer = sync.player_for(3)
	h.push_fixture("hub.instance.migrated.failover")
	assert_object(sync.player_for(3)).is_same(rp)
	assert_bool(rp.visible).is_true()


func test_only_players_sent_by_the_server_exist() -> void:
	await _joined()
	# A snapshot entity that never joined this room is not drawn (F092-AC-19).
	h.push(h.envelope("hub.snapshot.updated", {"room_id": "hub:main_hub:01", "tick": 5,
			"entities": [{"eid": 55, "p": [1, 0, 1]}]}))
	sync._process(0.016)
	assert_object(sync.player_for(55)).is_null()
	assert_int(sync.shown_count()).is_equal(1)
