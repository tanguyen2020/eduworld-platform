extends GdUnitTestSuite
## T016 (F102-AC-11 client part): `SceneRouter.enter_party_session(scene_ref, room_id)` — after the 3-2-1 countdown the
## zone of the session is loaded like an area change (`area_entered` with `enter.room_id`, `party_session`), the room is
## remembered for a reconnect (F102-AC-15), a scene that is not in this build is refused with `area_change_failed`
## (BUNDLE_MISSING) and nothing is entered; leaving the session forgets the room. PartyService.enter_session() goes
## through it with the scene and room of `party.session.started`.

const ZONE: String = "res://scenes/world/zones/learning_zone_math/Zone.tscn"
const ROOM: String = "0193b0a0-0000-7000-8000-000000000201"

var entered: Array[Array] = []
var failed: Array[String] = []


func before_test() -> void:
	entered.clear()
	failed.clear()
	SceneRouter.party_room_id = ""
	SceneRouter.area_entered.connect(_on_entered)
	SceneRouter.area_change_failed.connect(_on_failed)


func after_test() -> void:
	SceneRouter.area_entered.disconnect(_on_entered)
	SceneRouter.area_change_failed.disconnect(_on_failed)
	SceneRouter.party_room_id = ""


func _on_entered(area_id: String, scene_ref: String, spawn: String, enter: Dictionary) -> void:
	entered.append([area_id, scene_ref, spawn, enter])


func _on_failed(_area_id: String, code: String, _missing: Array) -> void:
	failed.append(code)


func test_the_session_zone_is_entered_with_its_room() -> void:
	assert_int(SceneRouter.enter_party_session(ZONE, ROOM)).is_equal(OK)
	assert_int(entered.size()).is_equal(1)
	assert_str(String(entered[0][1])).is_equal(ZONE)
	assert_dict(entered[0][3] as Dictionary).is_equal({"room_id": ROOM, "party_session": true})
	assert_str(SceneRouter.party_room_id).is_equal(ROOM)
	SceneRouter.leave_party_session()
	assert_str(SceneRouter.party_room_id).is_empty()


func test_a_scene_missing_from_the_build_is_refused() -> void:
	assert_int(SceneRouter.enter_party_session("res://scenes/world/zones/nowhere/Zone.tscn", ROOM)).is_equal(ERR_FILE_NOT_FOUND)
	assert_array(entered).is_empty()
	assert_array(failed).is_equal([SceneRouter.CODE_BUNDLE_MISSING])
	assert_int(SceneRouter.enter_party_session(ZONE, "")).is_equal(ERR_INVALID_PARAMETER)
	assert_str(SceneRouter.party_room_id).is_empty()


func test_party_service_enters_the_session_of_the_started_message() -> void:
	var h: RealtimeHarness = RealtimeHarness.create()
	var svc: PartyService = PartyService.new(h.api, h.client)
	svc.store.self_id = PartyFixtures.LEADER
	assert_int(svc.enter_session()).is_equal(ERR_UNAVAILABLE)
	svc.store.apply_snapshot(PartyModel.from_rest(PartyFixtures.party_all_ready()))
	svc.handle_message("party.session.started", PartyFixtures.ws("session.started.quest"))
	assert_int(svc.enter_session()).is_equal(OK)
	assert_str(String(entered[0][1])).is_equal(ZONE)
	assert_str(String((entered[0][3] as Dictionary)["room_id"])).is_equal(ROOM)
	h.client.unregister_handler(PartyService.PREFIX, svc.handle_message)
	h.dispose()
