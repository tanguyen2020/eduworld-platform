class_name F111SceneHarness
extends F110SceneHarness
## TEST-ONLY (AGENTS R17): the F110 harness (a signed-in student on FakeHttp, the session services in AppState, an online
## RealtimeHarness) plus the F111 routes of F111Fixtures and the F111 services rebuilt on the realtime harness:
## HubSync + JoinFriendService ("Tham gia bạn"; the zone change is a scripted `area_requester`), LiveEventService and
## SeasonTrackService (`event.*`, `season.track.updated`). `areas` records the zone changes asked; `area_result` is the
## answer of the next one (OK, or FAILED: the requester returns `area_code` like SceneRouter emitting `area_change_failed`).

var now_sec: int = 0
var areas: Array[Dictionary] = []
var area_result: Error = OK
## Code of a refused zone change (SceneRouter `area_change_failed`).
var area_code: String = "AREA_LOCKED"
## Area the world shows (the current zone of JoinFriendService).
var current_area: String = "main_hub"


func start(band: String = "g1_3", online: bool = true) -> void:
	await super(band, online)
	now_sec = int(rt.client.server_now_ms() / 1000)
	services.social.now_source = func() -> int: return now_sec * 1000
	services.hub = HubSync.new(rt.client)
	services.live_events = LiveEventService.new(h.api, rt.client, services.telemetry)
	services.live_events.now_source = func() -> int: return now_sec
	services.season = SeasonTrackService.new(h.api, rt.client, services.telemetry)
	services.join_friend = JoinFriendService.new(services.hub, services.social)
	services.join_friend.area_requester = _request_area
	services.join_friend.current_area = func() -> String: return current_area
	services.join_friend.self_state = func() -> Dictionary: return {"offline": not rt.client.is_online()}
	services.hub.enter_area(current_area, "")
	F111Fixtures.script(h.http, self_id, now_sec)


## Scripted SceneRouter.request_area: records the call; OK → the world joins the hub of the new area with the follow id
## (as WorldRoot does on `area_entered`).
func _request_area(area_id: String, args: Dictionary) -> String:
	areas.append({"area_id": area_id, "args": args.duplicate()})
	await (Engine.get_main_loop() as SceneTree).process_frame
	if area_result != OK:
		return area_code
	current_area = area_id
	services.hub.enter_area(area_id, "", String(args.get("follow_user_id", "")))
	return ""


## Delivers a realtime message (AsyncAPI shape) as the dispatcher would; `hub.*` / `system.*` go through the client.
func push(type: String, payload: Dictionary) -> void:
	if type.begins_with("event."):
		services.live_events.handle_message(type, payload)
	elif type.begins_with("season."):
		services.season.handle_message(type, payload)
	else:
		super(type, payload)


## Server message through the realtime client (dispatcher, schemas) with the next seq.
func push_ws(type: String, payload: Dictionary, ref: String = "") -> void:
	var env: Dictionary = rt.envelope(type, payload, ref)
	rt.push(env)


## The `hub.instance.join` commands sent so far (their payloads).
func joins() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for env: Dictionary in sent(HubSync.TYPE_JOIN):
		out.append(env["payload"] as Dictionary)
	return out


## `hub.instance.assigned` for the last join (room of `area_id` with one other player).
func assign_last(area_id: String, friend_id: String = F111Fixtures.FRIEND_ONLINE, nick: String = F111Fixtures.FRIEND_NICK) -> void:
	var payload: Dictionary = (RealtimeHarness.fixture("hub.instance.assigned.main_hub")["payload"] as Dictionary).duplicate(true)
	payload["area_id"] = area_id
	payload["room_id"] = "hub:%s:02" % area_id
	var players: Array = payload["players"] as Array
	((players[0] as Dictionary)["player"] as Dictionary)["user_id"] = friend_id
	((players[0] as Dictionary)["player"] as Dictionary)["nickname"] = nick
	ack_last(HubSync.TYPE_JOIN)
	push_ws("hub.instance.assigned", payload)


func dispose() -> void:
	if services != null:
		if services.live_events != null:
			services.live_events.clear()
			if rt != null:
				rt.client.unregister_handler(LiveEventService.PREFIX, services.live_events.handle_message)
		if services.season != null:
			services.season.clear()
			if rt != null:
				rt.client.unregister_handler(SeasonTrackService.PREFIX, services.season.handle_message)
		if services.hub != null:
			services.hub.dispose()
	super()
