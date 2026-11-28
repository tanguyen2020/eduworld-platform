extends GdUnitTestSuite
## T010 (F111-AC-01, AC-03; F092 / F071 regression): `HubSync.enter_area(area_id, spawn_point, follow_user_id)` and
## `HubSync.follow(user_id)` (true when sent) send `hub.instance.join` with `follow_user_id` (AsyncAPI `HubInstanceJoin`) exactly once —
## a later join of the same area (a fresh connection, a resync) is an ordinary one; a refused follow is reported as
## `follow_failed(code)`, never as the generic `join_failed` hub notice; without a follow id nothing changed (F092).
## `SceneRouter.request_area(area_id, portal_id, world, args)` passes `args.follow_user_id` on to the `enter` of
## `area_entered` so the world joins the new zone next to the friend (F071 unchanged without args).

var h: RealtimeHarness = null
var hub: HubSync = null
var auth: AuthHarness = null

const FRIEND: String = "0193b0a0-0000-7000-8000-0000000000a2"


func before_test() -> void:
	h = RealtimeHarness.create()
	hub = HubSync.new(h.client)


func after_test() -> void:
	hub.dispose()
	h.dispose()
	if auth != null:
		auth.dispose()
		auth = null
	AppState.spawn_point_id = ""
	AppState.current_zone = ""


func _joins() -> Array[Dictionary]:
	return h.peer().sent_of("hub.instance.join")


func test_enter_area_with_a_follow_id_joins_next_to_the_friend_once() -> void:
	await h.connect_online()
	hub.enter_area("learning_zone.math", "", FRIEND)
	var first: Dictionary = _joins()[0]["payload"] as Dictionary
	assert_dict(first).is_equal({"area_id": "learning_zone.math", "follow_user_id": FRIEND})
	h.ack_last("hub.instance.join")
	await get_tree().process_frame
	# The follow id is spent: the next join of the area (a fresh connection / resync) is ordinary.
	hub.join()
	var again: Dictionary = _joins()[1]["payload"] as Dictionary
	assert_bool(again.has("follow_user_id")).is_false()


func test_follow_in_the_same_area_and_a_refusal_is_not_the_hub_notice() -> void:
	await h.connect_online()
	hub.enter_area("main_hub", "main_hub.default")
	h.ack_last("hub.instance.join")
	await get_tree().process_frame
	var generic: Array[String] = []
	var follow: Array[String] = []
	hub.join_failed.connect(func(code: String) -> void: generic.append(code))
	hub.follow_failed.connect(func(code: String) -> void: follow.append(code))
	assert_bool(hub.follow(FRIEND)).is_true()
	var sent: Dictionary = _joins()[1]["payload"] as Dictionary
	assert_dict(sent).is_equal({"area_id": "main_hub", "spawn_point": "main_hub.default", "follow_user_id": FRIEND})
	h.ack_last("hub.instance.join", false, "ROOM_FULL")
	await get_tree().process_frame
	await get_tree().process_frame
	assert_array(follow).contains_exactly(["ROOM_FULL"])
	assert_array(generic).is_empty()


func test_without_a_follow_id_the_join_is_unchanged() -> void:
	hub.enter_area("main_hub", "main_hub.default")
	await h.connect_online()
	assert_dict(_joins()[0]["payload"] as Dictionary).is_equal({"area_id": "main_hub", "spawn_point": "main_hub.default"})
	var generic: Array[String] = []
	hub.join_failed.connect(func(code: String) -> void: generic.append(code))
	h.ack_last("hub.instance.join", false, "ROOM_FULL")
	await get_tree().process_frame
	await get_tree().process_frame
	assert_array(generic).contains_exactly(["ROOM_FULL"])
	# A bad follow id is never sent.
	assert_bool(hub.follow("not-a-uuid")).is_false()


func test_request_area_passes_the_follow_id_to_the_world() -> void:
	auth = AuthHarness.create()
	await auth.sign_in()
	var store: WorldFixtures.FakeStore = auto_free(WorldFixtures.FakeStore.new()) as WorldFixtures.FakeStore
	var world: WorldService = WorldService.new(auth.api, store)
	auth.http.reply(WorldFixtures.ROUTE_AREAS, 200, WorldFixtures.areas())
	await world.fetch_areas()
	auth.http.reply(WorldFixtures.ROUTE_ENTER, 200, WorldFixtures.enter_math())
	var entered: Array[Dictionary] = []
	var on_enter: Callable = func(_id: String, _scene: String, _spawn: String, enter: Dictionary) -> void: entered.append(enter)
	SceneRouter.area_entered.connect(on_enter)
	var err: Error = await SceneRouter.request_area("learning_zone.math", "", world, {"follow_user_id": FRIEND})
	var plain: Error = await SceneRouter.request_area("learning_zone.math", "", world)
	SceneRouter.area_entered.disconnect(on_enter)
	assert_int(err).is_equal(OK)
	assert_int(plain).is_equal(OK)
	assert_str(String(entered[0].get("follow_user_id", ""))).is_equal(FRIEND)
	assert_bool(entered[1].has("follow_user_id")).is_false()
	# A bad id is not passed on.
	assert_bool(SceneRouter.follow_of({"follow_user_id": "nope"}).is_empty()).is_true()
