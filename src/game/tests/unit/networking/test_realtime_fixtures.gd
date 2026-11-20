extends GdUnitTestSuite
## F092 T005, T006 (F092-AC-06, contracts §6): every realtime fixture generated from the AsyncAPI `examples`
## (tests/fixtures/contract/asyncapi, `node src/game/tools/gen_fixtures.mjs --check` keeps them in step) is accepted
## by the client: server messages pass the envelope and payload checks and parse into the typed models; client
## commands rebuild into a valid frame within `max_frame_bytes`.

const FIXTURES: String = "res://tests/fixtures/contract/asyncapi"
const CLIENT_TYPES: PackedStringArray = [
	"hub.instance.join", "hub.player.move", "hub.player.emote", "presence.status.set", "system.session.resume",
]


func _fixtures() -> PackedStringArray:
	var out: PackedStringArray = []
	for file: String in DirAccess.get_files_at(FIXTURES):
		if file.ends_with(".json"):
			out.append(file)
	return out


func test_every_server_fixture_passes_the_envelope_and_payload_checks() -> void:
	var server: int = 0
	for file: String in _fixtures():
		var text: String = FileAccess.get_file_as_string(FIXTURES.path_join(file))
		var data: Dictionary = JSON.parse_string(text) as Dictionary
		if String(data["type"]) in CLIENT_TYPES:
			continue
		server += 1
		var env: WsEnvelope = WsEnvelope.parse(text)
		assert_object(env).override_failure_message(file).is_not_null()
		assert_bool(WsMessageSchemas.is_known(env.type)).override_failure_message(file).is_true()
		assert_bool(WsMessageSchemas.validate(env.type, env.payload)).override_failure_message(file).is_true()
	assert_int(server).is_greater_equal(16)


func test_client_fixtures_rebuild_into_valid_frames() -> void:
	var client: int = 0
	for file: String in _fixtures():
		var data: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(FIXTURES.path_join(file))) as Dictionary
		if not String(data["type"]) in CLIENT_TYPES:
			continue
		client += 1
		var text: String = WsEnvelope.encode(String(data["type"]), String(data["id"]), data["payload"] as Dictionary, 1)
		assert_str(text).override_failure_message(file).is_not_empty()
	assert_int(client).is_equal(CLIENT_TYPES.size())


func test_hub_fixtures_parse_into_models() -> void:
	var assigned: Dictionary = RealtimeHarness.fixture("hub.instance.assigned.main_hub")
	var room: HubRoomState = HubRoomState.new()
	var players: Array = room.apply_assigned(assigned["payload"] as Dictionary, 1000)
	assert_int(players.size()).is_equal(1)
	var player: PublicPlayerModel = players[0][1] as PublicPlayerModel
	assert_str(player.nickname).is_equal("CuMeoChamChi12")
	assert_object(player.avatar()).is_not_null()
	var snapshot: Dictionary = RealtimeHarness.fixture("hub.snapshot.updated.delta")
	var eid: Array = []
	var sample: SnapshotSample = SnapshotSample.from_entity(((snapshot["payload"] as Dictionary)["entities"] as Array)[0], 1011, 5, eid)
	assert_int(int(eid[0])).is_equal(3)
	assert_str(sample.anim).is_equal("walk")
	var welcome: RealtimeSession = RealtimeSession.new()
	welcome.apply_welcome(RealtimeHarness.fixture("system.session.welcome.new_connection")["payload"] as Dictionary)
	assert_int(welcome.max_move_hz).is_equal(10)


func test_envelope_rejects_bad_frames() -> void:
	for bad: String in ["", "[]", "{\"v\":1}", JSON.stringify({"v": 1, "type": "a.b", "seq": 1, "payload": {}}),
			JSON.stringify({"v": 1, "type": "a.b.c", "seq": 0, "payload": {}}),
			JSON.stringify({"v": 1, "type": "a.b.c", "seq": 1.5, "payload": {}}),
			JSON.stringify({"v": 1, "type": "a.b.c", "seq": 1, "payload": []}),
			JSON.stringify({"v": 1, "type": "a.b.%s" % "c".repeat(70), "seq": 1, "payload": {}})]:
		assert_object(WsEnvelope.parse(bad)).override_failure_message(bad).is_null()
	assert_object(WsEnvelope.parse(JSON.stringify({"v": 1, "type": "a.b.c", "seq": 1, "payload": {}}), 10)).is_null()
	assert_str(WsEnvelope.encode("hub.player.move", "short", {}, 1)).is_empty()
